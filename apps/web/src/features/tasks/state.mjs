export function normalizeTaskSearch(value) {
  const trimmed = value.trim();
  return trimmed.length >= 2 ? trimmed : "";
}

export function updateTaskCompletion(items, taskId, completed) {
  return items.map((task) =>
    task.id === taskId ? { ...task, is_completed: completed } : task,
  );
}

export async function runOptimisticMutation(snapshot, optimistic, publish, persist) {
  publish(optimistic);
  try {
    await persist();
  } catch (error) {
    publish(snapshot);
    throw error;
  }
}

export function scheduleTaskCompletion({ task, publish, persist, onError, onSettled, delay = 3000 }) {
  const target = !task.is_completed;
  publish((items) => updateTaskCompletion(items, task.id, target));
  let cancelled = false;
  const timeout = setTimeout(async () => {
    if (cancelled) return;
    onSettled();
    try {
      await persist(task.id, target);
    } catch (error) {
      publish((items) => updateTaskCompletion(items, task.id, !!task.is_completed));
      onError(error);
    }
  }, delay);
  return {
    timeout,
    cancel() {
      if (cancelled) return;
      cancelled = true;
      clearTimeout(timeout);
      publish((items) => updateTaskCompletion(items, task.id, !!task.is_completed));
    },
  };
}
