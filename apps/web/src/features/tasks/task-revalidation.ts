import type { Task, TaskCursor, TaskPage } from "./api";

type BrowserWindow = Pick<
  Window,
  "addEventListener" | "removeEventListener" | "setInterval" | "clearInterval"
>;
type BrowserDocument = Pick<
  Document,
  "addEventListener" | "removeEventListener" | "visibilityState"
>;
type Options = {
  window: BrowserWindow;
  document: BrowserDocument;
  canRefresh: () => boolean;
  cursors: () => readonly (TaskCursor | undefined)[];
  fetchPage: (signal: AbortSignal, cursor?: TaskCursor) => Promise<TaskPage>;
  publish: (change: (items: Task[] | null) => Task[] | null) => void;
};

/** Refresh completion flags without replacing the user's current list or form. */
export function createTaskRevalidation(options: Options) {
  let stopped = false;
  let revision = 0;
  let request: AbortController | null = null;
  let interval: number | undefined;
  const visible = () => options.document.visibilityState === "visible";
  function invalidate() {
    revision++;
    request?.abort();
    request = null;
  }
  function clearInterval() {
    if (interval === undefined) return;
    options.window.clearInterval(interval);
    interval = undefined;
  }
  async function refresh() {
    if (stopped || !visible() || !options.canRefresh() || request) return;
    const controller = new AbortController();
    const startedAt = revision;
    request = controller;
    const current = () =>
      !stopped &&
      !controller.signal.aborted &&
      revision === startedAt &&
      visible() &&
      options.canRefresh();
    try {
      const completions = new Map<number, boolean>();
      for (const cursor of options.cursors()) {
        const page = await options.fetchPage(controller.signal, cursor);
        if (!current()) return;
        for (const task of page.items) {
          if (typeof task.is_completed === "boolean") {
            completions.set(task.id, task.is_completed);
          }
        }
      }
      if (!current()) return;
      options.publish((items) => {
        if (!current() || !items) return items;
        let changed = false;
        const next = items.map((task) => {
          const completion = completions.get(task.id);
          if (completion === undefined || task.is_completed === completion)
            return task;
          changed = true;
          return { ...task, is_completed: completion };
        });
        return changed ? next : items;
      });
    } catch {
      // Background failures leave the loaded list and explicit-action errors intact.
    } finally {
      if (request === controller) request = null;
    }
  }
  function onFocus() {
    void refresh();
  }
  function onVisibility() {
    if (visible()) {
      if (interval === undefined)
        interval = options.window.setInterval(onFocus, 30_000);
      void refresh();
    } else {
      clearInterval();
      invalidate();
    }
  }
  options.window.addEventListener("focus", onFocus);
  options.document.addEventListener("visibilitychange", onVisibility);
  if (visible()) interval = options.window.setInterval(onFocus, 30_000);
  return {
    invalidate,
    refresh,
    stop() {
      stopped = true;
      clearInterval();
      invalidate();
      options.window.removeEventListener("focus", onFocus);
      options.document.removeEventListener("visibilitychange", onVisibility);
    },
  };
}

/** Completion writes can span both the undo window and an in-flight mutation. */
export function createTaskRefreshGuard(
  current: () => Pick<
    ReturnType<typeof createTaskRevalidation>,
    "invalidate" | "refresh"
  > | null,
) {
  let pending = 0;
  return {
    idle: () => pending === 0,
    pause() {
      pending++;
      current()?.invalidate();
      let released = false;
      return () => {
        if (released) return;
        released = true;
        pending--;
        if (!pending) void current()?.refresh();
      };
    },
  };
}
