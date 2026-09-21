export function normalizeTaskSearch(value) {
  const trimmed = value.trim();
  return trimmed.length >= 2 ? trimmed : "";
}

export function updateTaskCompletion(items, taskId, completed) {
  return items.map((task) =>
    task.id === taskId ? { ...task, is_completed: completed } : task,
  );
}
