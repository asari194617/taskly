import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { Task, TaskCursor, TaskPage } from "./api";
import {
  createTaskRefreshGuard,
  createTaskRevalidation,
} from "./task-revalidation";
import { scheduleTaskCompletion } from "./state.mjs";
const task = (id: number, completed = false): Task => ({
  id,
  title: `Task ${id}`,
  subject_id: 1,
  is_completed: completed,
});
const page = (...items: Task[]): TaskPage => ({ items, nextCursor: null });
function deferred<T>() {
  let resolve!: (value: T) => void, reject!: (cause: unknown) => void;
  const promise = new Promise<T>((ok, fail) => {
    resolve = ok;
    reject = fail;
  });
  return { promise, resolve, reject };
}
class TestDocument extends EventTarget {
  visibilityState: DocumentVisibilityState = "visible";
}
const cleanups: (() => void)[] = [];
function harness(visible = true) {
  const document = new TestDocument();
  document.visibilityState = visible ? "visible" : "hidden";
  const window = Object.assign(new EventTarget(), {
    setInterval: (callback: () => void, timeout: number) =>
      globalThis.setInterval(callback, timeout) as unknown as number,
    clearInterval: (id: number) => globalThis.clearInterval(id),
  });
  let items: Task[] | null = [task(1), task(2)],
    ready = true;
  let cursors: (TaskCursor | undefined)[] = [undefined];
  const fetchPage = vi
    .fn<(signal: AbortSignal, cursor?: TaskCursor) => Promise<TaskPage>>()
    .mockResolvedValue(page(task(1, true), task(2)));
  const publish = vi.fn((change: (value: Task[] | null) => Task[] | null) => {
    items = change(items);
  });
  let refresh: ReturnType<typeof createTaskRevalidation> | null = null;
  const guard = createTaskRefreshGuard(() => refresh);
  refresh = createTaskRevalidation({
    window,
    document,
    canRefresh: () => ready && guard.idle(),
    cursors: () => cursors,
    fetchPage,
    publish,
  });
  cleanups.push(() => refresh?.stop());
  return {
    window,
    document,
    fetchPage,
    publish,
    refresh,
    guard,
    get items() {
      return items;
    },
    set items(value) {
      items = value;
    },
    set ready(value: boolean) {
      ready = value;
    },
    set cursors(value: (TaskCursor | undefined)[]) {
      cursors = value;
    },
    focus: () => window.dispatchEvent(new Event("focus")),
    visibility(value: DocumentVisibilityState) {
      document.visibilityState = value;
      document.dispatchEvent(new Event("visibilitychange"));
    },
  };
}
beforeEach(() => vi.useFakeTimers());
afterEach(() => {
  for (const cleanup of cleanups.splice(0)) cleanup();
  vi.useRealTimers();
  vi.restoreAllMocks();
});
it("refreshes on focus and each visible 30-second interval without replacing unchanged tasks", async () => {
  const h = harness(),
    original = h.items!;
  expect(h.fetchPage).not.toHaveBeenCalled();
  h.focus();
  await vi.advanceTimersByTimeAsync(0);
  expect(h.items![0].is_completed).toBe(true);
  expect(h.items![1]).toBe(original[1]);
  await vi.advanceTimersByTimeAsync(29_999);
  expect(h.fetchPage).toHaveBeenCalledTimes(1);
  await vi.advanceTimersByTimeAsync(1);
  expect(h.fetchPage).toHaveBeenCalledTimes(2);
});
it("starts no hidden timers or focus requests and refreshes when shown", async () => {
  const h = harness(false);
  expect(vi.getTimerCount()).toBe(0);
  h.focus();
  await vi.advanceTimersByTimeAsync(60_000);
  expect(h.fetchPage).not.toHaveBeenCalled();
  h.visibility("visible");
  await vi.advanceTimersByTimeAsync(0);
  expect(h.fetchPage).toHaveBeenCalledTimes(1);
  expect(vi.getTimerCount()).toBe(1);
  h.visibility("visible");
  await vi.advanceTimersByTimeAsync(0);
  expect(vi.getTimerCount()).toBe(1);
  h.visibility("hidden");
  expect(vi.getTimerCount()).toBe(0);
  await vi.advanceTimersByTimeAsync(60_000);
  expect(h.fetchPage).toHaveBeenCalledTimes(2);
});
it("does not overlap focus, visibility and timer requests", async () => {
  const h = harness(),
    request = deferred<TaskPage>();
  h.fetchPage.mockReturnValue(request.promise);
  h.focus();
  h.focus();
  h.visibility("visible");
  await vi.advanceTimersByTimeAsync(30_000);
  expect(h.fetchPage).toHaveBeenCalledTimes(1);
  request.resolve(page(task(1, true)));
  await vi.advanceTimersByTimeAsync(0);
  h.focus();
  expect(h.fetchPage).toHaveBeenCalledTimes(2);
});
it("waits for initial loading and pagination to finish", async () => {
  const h = harness();
  h.ready = false;
  h.focus();
  await vi.advanceTimersByTimeAsync(30_000);
  expect(h.fetchPage).not.toHaveBeenCalled();
  h.ready = true;
  h.focus();
  await vi.advanceTimersByTimeAsync(0);
  expect(h.items![0].is_completed).toBe(true);
});
it("reads every loaded page but keeps order, membership and locally edited details", async () => {
  const h = harness(),
    second = { cursor_deadline: "later", cursor_id: "2" };
  h.cursors = [undefined, second];
  h.items![0].title = "Edited title";
  h.fetchPage
    .mockResolvedValueOnce(
      page({ ...task(1, true), title: "Server title" }, task(99, true)),
    )
    .mockResolvedValueOnce(page(task(2, true)));
  await h.refresh.refresh();
  expect(h.fetchPage.mock.calls.map(([, cursor]) => cursor)).toEqual([
    undefined,
    second,
  ]);
  expect(
    h.items?.map((item) => [item.id, item.title, item.is_completed]),
  ).toEqual([
    [1, "Edited title", true],
    [2, "Task 2", true],
  ]);
  expect(h.fetchPage.mock.calls[0][0]).toBe(h.fetchPage.mock.calls[1][0]);
});
it("keeps data on background failure and retries on next focus", async () => {
  const h = harness(),
    original = h.items;
  h.fetchPage.mockRejectedValueOnce(new Error("offline"));
  await h.refresh.refresh();
  expect(h.items).toBe(original);
  expect(h.publish).not.toHaveBeenCalled();
  await h.refresh.refresh();
  expect(h.items![0].is_completed).toBe(true);
});
it("does not publish a partial refresh if a later page fails", async () => {
  const h = harness(),
    original = h.items;
  h.cursors = [undefined, { cursor_deadline: "later", cursor_id: "2" }];
  h.fetchPage
    .mockResolvedValueOnce(page(task(1, true)))
    .mockRejectedValueOnce(new Error("offline"));
  await h.refresh.refresh();
  expect(h.items).toBe(original);
  expect(h.publish).not.toHaveBeenCalled();
});
it("aborts on hidden and rejects its late response", async () => {
  const h = harness(),
    request = deferred<TaskPage>(),
    original = h.items;
  h.fetchPage.mockReturnValueOnce(request.promise);
  h.focus();
  const signal = h.fetchPage.mock.calls[0][0];
  h.visibility("hidden");
  expect(signal.aborted).toBe(true);
  request.resolve(page(task(1, true)));
  await vi.advanceTimersByTimeAsync(0);
  expect(h.items).toBe(original);
  expect(h.publish).not.toHaveBeenCalled();
});
it("removes listeners, timers and in-flight requests on unmount", async () => {
  const h = harness(),
    request = deferred<TaskPage>();
  h.fetchPage.mockReturnValue(request.promise);
  h.focus();
  const signal = h.fetchPage.mock.calls[0][0];
  h.refresh.stop();
  expect(signal.aborted).toBe(true);
  expect(vi.getTimerCount()).toBe(0);
  h.focus();
  h.visibility("hidden");
  h.visibility("visible");
  await vi.advanceTimersByTimeAsync(60_000);
  expect(h.fetchPage).toHaveBeenCalledTimes(1);
  expect(vi.getTimerCount()).toBe(0);
  request.resolve(page(task(1, true)));
  await vi.advanceTimersByTimeAsync(0);
  expect(h.publish).not.toHaveBeenCalled();
});
it("leaves absent IDs, unknown flags, null state and unchanged objects alone", async () => {
  const h = harness(),
    original = h.items;
  h.fetchPage.mockResolvedValueOnce(
    page({ ...task(1), is_completed: undefined }),
  );
  await h.refresh.refresh();
  expect(h.items).toBe(original);
  h.items = null;
  await h.refresh.refresh();
  expect(h.items).toBeNull();
});
it("rejects deferred state publication after a local mutation invalidates a GET", async () => {
  const h = harness();
  let apply!: (items: Task[] | null) => Task[] | null;
  h.publish.mockImplementation((change) => {
    apply = change;
  });
  await h.refresh.refresh();
  h.refresh.invalidate();
  h.items = [task(1), task(2, true)];
  expect(apply(h.items)).toBe(h.items);
});
function completion(
  h: ReturnType<typeof harness>,
  id: number,
  persist: () => Promise<unknown>,
  onError = vi.fn(),
) {
  return scheduleTaskCompletion({
    task: h.items!.find((item) => item.id === id)!,
    publish: (change: (items: Task[]) => Task[]) => {
      h.refresh.invalidate();
      h.items = change(h.items!);
    },
    persist,
    onError,
    onSettled: vi.fn(),
    onFinished: h.guard.pause(),
  });
}
it("keeps refresh paused through undo and a pending network write", async () => {
  const h = harness(),
    pending = deferred<unknown>(),
    persist = vi.fn(() => pending.promise);
  completion(h, 1, persist);
  expect(h.items![0].is_completed).toBe(true);
  h.focus();
  await vi.advanceTimersByTimeAsync(2_999);
  expect(h.fetchPage).not.toHaveBeenCalled();
  expect(persist).not.toHaveBeenCalled();
  await vi.advanceTimersByTimeAsync(1);
  expect(persist).toHaveBeenCalledTimes(1);
  h.focus();
  await vi.advanceTimersByTimeAsync(30_000);
  expect(h.fetchPage).not.toHaveBeenCalled();
  pending.resolve(true);
  await vi.advanceTimersByTimeAsync(0);
  expect(h.fetchPage).toHaveBeenCalledTimes(1);
  expect(h.items![0].is_completed).toBe(true);
});
it("undo cancels the write and releases refresh only after rollback", async () => {
  const h = harness(),
    persist = vi.fn(),
    action = completion(h, 1, persist);
  h.fetchPage.mockResolvedValue(page(task(1)));
  action.cancel();
  action.cancel();
  await vi.advanceTimersByTimeAsync(30_000);
  expect(persist).not.toHaveBeenCalled();
  expect(h.items![0].is_completed).toBe(false);
  expect(h.guard.idle()).toBe(true);
});
it("waits for concurrent writes and ignores late undo after persistence starts", async () => {
  const h = harness(),
    first = deferred<unknown>(),
    second = deferred<unknown>();
  const a = completion(h, 1, () => first.promise);
  completion(h, 2, () => second.promise);
  await vi.advanceTimersByTimeAsync(3_000);
  a.cancel();
  expect(h.items![0].is_completed).toBe(true);
  expect(h.guard.idle()).toBe(false);
  first.resolve(true);
  await vi.advanceTimersByTimeAsync(0);
  h.focus();
  expect(h.fetchPage).not.toHaveBeenCalled();
  h.fetchPage.mockResolvedValue(page(task(1, true), task(2, true)));
  second.resolve(true);
  await vi.advanceTimersByTimeAsync(0);
  expect(h.fetchPage).toHaveBeenCalledTimes(1);
  expect(h.items?.every((item) => item.is_completed)).toBe(true);
});
it("rolls back failed persistence before resuming refresh", async () => {
  const h = harness(),
    pending = deferred<unknown>(),
    onError = vi.fn();
  completion(h, 1, () => pending.promise, onError);
  h.fetchPage.mockResolvedValue(page(task(1)));
  await vi.advanceTimersByTimeAsync(3_000);
  pending.reject(new Error("write failed"));
  await vi.advanceTimersByTimeAsync(0);
  expect(onError).toHaveBeenCalledTimes(1);
  expect(h.items![0].is_completed).toBe(false);
  expect(h.guard.idle()).toBe(true);
});
it("rejects a pre-mutation GET resolving later than a newer refresh", async () => {
  const h = harness(),
    old = deferred<TaskPage>(),
    write = deferred<unknown>();
  h.fetchPage.mockReturnValueOnce(old.promise);
  h.focus();
  const signal = h.fetchPage.mock.calls[0][0];
  completion(h, 1, () => write.promise);
  expect(signal.aborted).toBe(true);
  await vi.advanceTimersByTimeAsync(3_000);
  write.resolve(true);
  await vi.advanceTimersByTimeAsync(0);
  expect(h.items![0].is_completed).toBe(true);
  old.resolve(page(task(1)));
  await vi.advanceTimersByTimeAsync(0);
  expect(h.items![0].is_completed).toBe(true);
});
it("carries a pause across controller/query replacement and releases only once", () => {
  let current: { invalidate: () => void; refresh: () => Promise<void> } | null =
    { invalidate: vi.fn(), refresh: vi.fn().mockResolvedValue(undefined) };
  const guard = createTaskRefreshGuard(() => current),
    release = guard.pause();
  expect(guard.idle()).toBe(false);
  const refresh = vi.fn().mockResolvedValue(undefined);
  current = { invalidate: vi.fn(), refresh };
  release();
  release();
  expect(refresh).toHaveBeenCalledTimes(1);
  expect(guard.idle()).toBe(true);
  current = null;
  const detached = guard.pause();
  detached();
  expect(guard.idle()).toBe(true);
});
