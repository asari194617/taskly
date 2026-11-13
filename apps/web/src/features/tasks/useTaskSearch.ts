import * as React from "react";
import { searchTasks, type Task, type TaskCursor } from "./api";
import { normalizeTaskSearch } from "./state.mjs";
import { appendTaskPage } from "./list-model";
import {
  createTaskRefreshGuard,
  createTaskRevalidation,
} from "./task-revalidation";

export function useTaskSearch(loadFailed: string) {
  const [items, updateItems] = React.useState<Task[] | null>(null);
  const [nextCursor, setNextCursor] = React.useState<TaskCursor | null>(null);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [loadMoreError, setLoadMoreError] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [q, setQ] = React.useState("");
  const [qInput, setQInput] = React.useState("");
  const requestController = React.useRef<AbortController | null>(null);
  const loadingMoreRef = React.useRef(false);
  const itemsRef = React.useRef(items);
  itemsRef.current = items;
  const loadedCursors = React.useRef<(TaskCursor | undefined)[]>([]);
  const revalidation = React.useRef<ReturnType<
    typeof createTaskRevalidation
  > | null>(null);
  const refreshGuard = React.useRef<ReturnType<
    typeof createTaskRefreshGuard
  > | null>(null);
  if (!refreshGuard.current) {
    refreshGuard.current = createTaskRefreshGuard(() => revalidation.current);
  }
  const setItems = React.useCallback<
    React.Dispatch<React.SetStateAction<Task[] | null>>
  >((change) => {
    revalidation.current?.invalidate();
    updateItems(change);
  }, []);
  const pauseRevalidation = React.useCallback(
    () => refreshGuard.current!.pause(),
    [],
  );

  React.useEffect(() => {
    const ac = new AbortController();
    requestController.current = ac;
    loadingMoreRef.current = false;
    setLoadingMore(false);
    updateItems(null);
    loadedCursors.current = [];
    setNextCursor(null);
    setError(null);
    setLoadMoreError(false);
    const refresh = createTaskRevalidation({
      window,
      document,
      canRefresh: () =>
        itemsRef.current !== null &&
        !loadingMoreRef.current &&
        refreshGuard.current!.idle(),
      cursors: () => loadedCursors.current,
      fetchPage: (signal, cursor) => searchTasks(q, signal, cursor),
      publish: updateItems,
    });
    revalidation.current = refresh;
    void (async () => {
      try {
        const page = await searchTasks(q, ac.signal);
        if (ac.signal.aborted) return;
        loadedCursors.current = [undefined];
        updateItems(page.items);
        setNextCursor(page.nextCursor);
      } catch (cause: any) {
        if (ac.signal.aborted || cause?.name === "AbortError") return;
        setError(cause?.message || loadFailed);
      }
    })();
    return () => {
      ac.abort();
      refresh.stop();
      if (revalidation.current === refresh) revalidation.current = null;
    };
  }, [q, loadFailed]);

  const loadMore = React.useCallback(async () => {
    const ac = requestController.current;
    if (!ac || ac.signal.aborted || !nextCursor || loadingMoreRef.current)
      return;
    revalidation.current?.invalidate();
    loadingMoreRef.current = true;
    setLoadingMore(true);
    setLoadMoreError(false);
    try {
      const page = await searchTasks(q, ac.signal, nextCursor);
      if (ac.signal.aborted || requestController.current !== ac) return;
      loadedCursors.current.push(nextCursor);
      updateItems((current) => appendTaskPage(current, page.items));
      setNextCursor(page.nextCursor);
    } catch (cause: any) {
      if (!ac.signal.aborted && cause?.name !== "AbortError")
        setLoadMoreError(true);
    } finally {
      if (requestController.current === ac) {
        loadingMoreRef.current = false;
        setLoadingMore(false);
      }
    }
  }, [nextCursor, q]);

  React.useEffect(() => {
    const id = setTimeout(() => setQ(normalizeTaskSearch(qInput)), 500);
    return () => clearTimeout(id);
  }, [qInput]);

  return {
    items,
    setItems,
    pauseRevalidation,
    nextCursor,
    loadingMore,
    loadMoreError,
    error,
    setError,
    q,
    qInput,
    setQInput,
    submitSearch: () => setQ(normalizeTaskSearch(qInput)),
    loadMore,
  };
}
