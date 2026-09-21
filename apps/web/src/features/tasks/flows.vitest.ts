import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/graphql", () => ({ gql: vi.fn() }));

import { gql } from "@/lib/graphql";
import { searchTasks, updateTask, setTaskCompleted } from "./api";
import { runOptimisticMutation, scheduleTaskCompletion } from "./state.mjs";

const query = vi.mocked(gql);
type CompletionItem = { id: number; is_completed: boolean };

afterEach(() => {
  vi.clearAllMocks();
  vi.useRealTimers();
});

describe("task search and update", () => {
  it("passes search terms and cancellation to the API", async () => {
    const signal = new AbortController().signal;
    const cursor = { cursor_deadline: "2026-09-22T00:00:00Z", cursor_id: "50" };
    query.mockResolvedValueOnce({ tasks: { items: [{ id: 1, title: "report" }], next_cursor: cursor } });
    expect(await searchTasks("report", signal)).toEqual({ items: [{ id: 1, title: "report" }], nextCursor: cursor });
    expect(query).toHaveBeenCalledWith(expect.stringContaining("next_cursor{ cursor_deadline cursor_id }"), { q: "report" }, { signal });
    query.mockResolvedValueOnce({ tasks: { items: [{ id: 2, title: "later" }], next_cursor: null } });
    expect(await searchTasks("report", signal, cursor)).toEqual({ items: [{ id: 2, title: "later" }], nextCursor: null });
    expect(query).toHaveBeenLastCalledWith(expect.stringContaining("cursor_deadline:$cursor_deadline"), { q: "report", ...cursor }, { signal });
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
