import * as React from "react";
import { searchTasks, type Task, type TaskCursor } from "./api";
import { normalizeTaskSearch } from "./state.mjs";
import { appendTaskPage } from "./list-model";

export function useTaskSearch(loadFailed: string) {
  const [items, setItems] = React.useState<Task[] | null>(null);
  const [nextCursor, setNextCursor] = React.useState<TaskCursor | null>(null);
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [loadMoreError, setLoadMoreError] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [q, setQ] = React.useState("");
  const [qInput, setQInput] = React.useState("");
  const requestController = React.useRef<AbortController | null>(null);
  const loadingMoreRef = React.useRef(false);

  React.useEffect(() => {
    const ac = new AbortController();
    requestController.current = ac;
    loadingMoreRef.current = false;
    setLoadingMore(false);
    setItems(null);
    setNextCursor(null);
    setError(null);
    setLoadMoreError(false);
    void (async () => {
      try {
        const page = await searchTasks(q, ac.signal);
        if (ac.signal.aborted) return;
        setItems(page.items);
        setNextCursor(page.nextCursor);
      } catch (cause: any) {
        if (ac.signal.aborted || cause?.name === "AbortError") return;
        setError(cause?.message || loadFailed);
      }
    })();
    return () => ac.abort();
  }, [q, loadFailed]);

  const loadMore = React.useCallback(async () => {
    const ac = requestController.current;
    if (!ac || ac.signal.aborted || !nextCursor || loadingMoreRef.current)
      return;
    loadingMoreRef.current = true;
    setLoadingMore(true);
    setLoadMoreError(false);
    try {
      const page = await searchTasks(q, ac.signal, nextCursor);
      if (ac.signal.aborted || requestController.current !== ac) return;
      setItems((current) => appendTaskPage(current, page.items));
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
