import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/graphql", () => ({ gql: vi.fn() }));

import { gql } from "@/lib/graphql";
import { createTask, deleteTask, reportTask, searchTasks, updateTask, setTaskCompleted, uploadTaskImage, TASK_IMAGE_MAX_BYTES } from "./api";
import { appendTaskPage, visibleTasks } from "./list-model";
import { runOptimisticMutation, scheduleTaskCompletion } from "./state.mjs";

const query = vi.mocked(gql);
type CompletionItem = { id: number; is_completed: boolean };

afterEach(() => {
  vi.clearAllMocks();
  vi.useRealTimers();
});

describe("task search and update", () => {
  it("uploads supported images directly with the signed R2 request", async () => {
    const request = vi.spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(new Response(JSON.stringify({
        key: "task-images/1/550e8400-e29b-41d4-a716-446655440000",
        upload_url: "https://r2.example.test/signed",
        headers: { "Content-Type": "image/png", "x-amz-meta-owner-id": "1" },
      }), { status: 200, headers: { "Content-Type": "application/json" } }))
      .mockResolvedValueOnce(new Response(null, { status: 200 }));
    const file = new File([new Uint8Array([1, 2, 3])], "proof.png", { type: "image/png" });

    await expect(uploadTaskImage(file)).resolves.toContain("550e8400");
    expect(request).toHaveBeenNthCalledWith(2, "https://r2.example.test/signed", expect.objectContaining({
      method: "PUT",
      body: file,
    }));
  });

  it("rejects oversized or unsafe images before requesting a URL", async () => {
    const request = vi.spyOn(globalThis, "fetch");
    const oversized = { type: "image/png", size: TASK_IMAGE_MAX_BYTES + 1 } as File;
    const svg = { type: "image/svg+xml", size: 10 } as File;
    await expect(uploadTaskImage(oversized)).rejects.toThrow("image_too_large");
    await expect(uploadTaskImage(svg)).rejects.toThrow("unsupported_image_type");
    expect(request).not.toHaveBeenCalled();
  });

  it("keeps the create, delete, and report request contracts", async () => {
    const input = {
      title: "report",
      subject_id: 7,
      deadline: "2026-09-22",
      is: true,
      scr: false,
    };
    query.mockResolvedValueOnce({ createTask: { id: 8, ...input } });
    expect(await createTask(input)).toMatchObject({ id: 8, title: "report" });
    expect(query).toHaveBeenLastCalledWith(
      expect.stringContaining("createTask"),
      input,
    );

    query.mockResolvedValueOnce({ deleteTask: true });
    await deleteTask(8);
    expect(query).toHaveBeenLastCalledWith(
      expect.stringContaining("deleteTask"),
      { id: 8 },
    );

    query.mockResolvedValueOnce({ createUserReport: { id: 5 } });
    await reportTask(8, "incorrect", "details");
    expect(query).toHaveBeenLastCalledWith(
      expect.stringContaining("createUserReport"),
      {
        task_id: 8,
        reason: "incorrect",
        comment: "details",
      },
    );
  });

  it("passes search terms and cancellation to the API", async () => {
    const signal = new AbortController().signal;
    const cursor = { cursor_deadline: "2026-09-22", cursor_id: "50" };
    query.mockResolvedValueOnce({ tasks: { items: [{ id: 1, title: "report" }], next_cursor: cursor } });
    expect(await searchTasks("report", signal)).toEqual({ items: [{ id: 1, title: "report" }], nextCursor: cursor });
    expect(query).toHaveBeenCalledWith(expect.stringContaining("next_cursor{ cursor_deadline cursor_id }"), { q: "report" }, { signal });
    query.mockResolvedValueOnce({ tasks: { items: [{ id: 2, title: "later" }], next_cursor: null } });
    expect(await searchTasks("report", signal, cursor)).toEqual({ items: [{ id: 2, title: "later" }], nextCursor: null });
    expect(query).toHaveBeenLastCalledWith(expect.stringContaining("cursor_deadline:$cursor_deadline"), { q: "report", ...cursor }, { signal });
  });

  it("shows expired title and description matches across search pages", async () => {
    const cursor = { cursor_deadline: "2026-09-20", cursor_id: "50" };
    const first = Array.from({ length: 50 }, (_, index) => ({
      id: index + 1,
      title: index === 0 ? "needle title" : "other",
      description: index === 1 ? "needle description" : null,
      deadline: "2026-09-20",
    }));
    const second = [{ id: 51, title: "needle today", deadline: "2026-09-21" }];
    query.mockResolvedValueOnce({ tasks: { items: first, next_cursor: cursor } });
    query.mockResolvedValueOnce({ tasks: { items: second, next_cursor: null } });

    const page1 = await searchTasks("needle");
    const page2 = await searchTasks("needle", undefined, page1.nextCursor!);
    const results = appendTaskPage(page1.items, page2.items);
    expect(results).toHaveLength(51);
    expect(visibleTasks(results, "needle", "all", new Date(2026, 8, 21)).map((task) => task.id))
      .toEqual(Array.from({ length: 51 }, (_, index) => index + 1));
    expect(visibleTasks(results, "", "all", new Date(2026, 8, 21)).map((task) => task.id))
      .toEqual([51]);
    expect(query).toHaveBeenLastCalledWith(
      expect.stringContaining("tasks(limit:15, q:$q"),
      { q: "needle", ...cursor },
      { signal: undefined },
    );
  });

  it("falls back only for the older completion schema and propagates other errors", async () => {
    query.mockRejectedValueOnce(new Error('Cannot query field "is_completed"'));
    query.mockResolvedValueOnce({ tasks: { items: [{ id: 2, title: "old" }] } });
    expect(await searchTasks("")).toEqual({ items: [{ id: 2, title: "old", is_completed: false }], nextCursor: null });
    expect(query).toHaveBeenLastCalledWith(expect.not.stringContaining("is_completed is_hidden"), { q: undefined }, { signal: undefined });
    query.mockRejectedValueOnce(new Error("API unavailable"));
    await expect(searchTasks("test")).rejects.toThrow("API unavailable");
  });

  it("sends edits and restores the previous list when the API fails", async () => {
    const before = [{ id: 1, title: "old" }];
    const after = [{ id: 1, title: "new" }];
    const published: typeof before[] = [];
    query.mockResolvedValueOnce({ updateTask: { id: 1 } });
    await runOptimisticMutation(before, after, (items: typeof before) => published.push(items), () => updateTask(1, { title: "new" }));
    expect(published).toEqual([after]);
    expect(query).toHaveBeenCalledWith(expect.stringContaining("updateTask"), { id: 1, title: "new" });
    query.mockRejectedValueOnce(new Error("save failed"));
    await expect(runOptimisticMutation(before, after, (items: typeof before) => published.push(items), () => updateTask(1, { title: "new" }))).rejects.toThrow("save failed");
    expect(published.slice(-2)).toEqual([after, before]);
  });
});

describe("completion undo window", () => {
  it("cancels an optimistic change without calling the API", async () => {
    vi.useFakeTimers();
    let items = [{ id: 1, is_completed: false }];
    const publish = (change: (items: CompletionItem[]) => CompletionItem[]) => { items = change(items); };
    const persist = vi.fn();
    const action = scheduleTaskCompletion({ task: items[0], publish, persist, onError: vi.fn(), onSettled: vi.fn() });
    expect(items[0].is_completed).toBe(true);
    action.cancel();
    await vi.runAllTimersAsync();
    expect(items[0].is_completed).toBe(false);
    expect(persist).not.toHaveBeenCalled();
  });

  it("commits after the window and rolls back on API failure", async () => {
    vi.useFakeTimers();
    let items = [{ id: 1, is_completed: false }];
    const publish = (change: (items: CompletionItem[]) => CompletionItem[]) => { items = change(items); };
    const onError = vi.fn();
    query.mockRejectedValueOnce(new Error("offline"));
    scheduleTaskCompletion({ task: items[0], publish, persist: setTaskCompleted, onError, onSettled: vi.fn() });
    await vi.advanceTimersByTimeAsync(3000);
    expect(query).toHaveBeenCalledWith(expect.stringContaining("completeTask"), { id: 1 });
    expect(items[0].is_completed).toBe(false);
    expect(onError).toHaveBeenCalledWith(expect.objectContaining({ message: "offline" }));
  });

  it("keeps the completed state after a successful API response", async () => {
    vi.useFakeTimers();
    let items: CompletionItem[] = [{ id: 3, is_completed: false }];
    const publish = (change: (items: CompletionItem[]) => CompletionItem[]) => { items = change(items); };
    const onSettled = vi.fn();
    query.mockResolvedValueOnce({ completeTask: true });
    scheduleTaskCompletion({ task: items[0], publish, persist: setTaskCompleted, onError: vi.fn(), onSettled });
    await vi.advanceTimersByTimeAsync(2999);
    expect(query).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1);
    expect(items[0].is_completed).toBe(true);
    expect(onSettled).toHaveBeenCalledOnce();
  });
});
