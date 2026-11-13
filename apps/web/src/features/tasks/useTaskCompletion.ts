import * as React from "react";
import { setTaskCompleted, type Task } from "./api";
import { scheduleTaskCompletion } from "./state.mjs";

type PendingAction = {
  target: boolean;
  timeout: ReturnType<typeof setTimeout>;
  cancel: () => void;
};

export function useTaskCompletion(
  setItems: React.Dispatch<React.SetStateAction<Task[] | null>>,
  onError: (error: unknown) => void,
  pauseRevalidation?: () => () => void,
) {
  const [pending, setPending] = React.useState<Record<number, PendingAction>>(
    {},
  );
  const pendingTimers = React.useRef(new Set<ReturnType<typeof setTimeout>>());
  const refreshLeases = React.useRef(new Set<() => void>());

  React.useEffect(
    () => () => {
      for (const timer of pendingTimers.current) clearTimeout(timer);
      pendingTimers.current.clear();
      for (const release of refreshLeases.current) release();
      refreshLeases.current.clear();
    },
    [],
  );

  function toggleWithPending(task: Task) {
    if (task.is_completion_disabled) return;
    const existing = pending[task.id];
    if (existing) {
      existing.cancel();
      pendingTimers.current.delete(existing.timeout);
      setPending((current) => {
        const next = { ...current };
        delete next[task.id];
        return next;
      });
      return;
    }
    const resume = pauseRevalidation?.();
    const release = () => {
      if (refreshLeases.current.delete(release)) resume?.();
    };
    refreshLeases.current.add(release);
    const action = scheduleTaskCompletion({
      task,
      publish: (change: (items: Task[]) => Task[]) =>
        setItems((current) => change(current || [])),
      persist: setTaskCompleted,
      onError,
      onFinished: release,
      onSettled: () => {
        pendingTimers.current.delete(action.timeout);
        setPending((current) => {
          const next = { ...current };
          delete next[task.id];
          return next;
        });
      },
    });
    pendingTimers.current.add(action.timeout);
    setPending((current) => ({
      ...current,
      [task.id]: { target: !task.is_completed, ...action },
    }));
  }

  return { pending, toggleWithPending };
}
