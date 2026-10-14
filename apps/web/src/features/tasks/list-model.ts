import type { Task } from "./api";

export type CompletionFilter = "incomplete" | "completed" | "all";
export type TaskRow = { task: Task; heading: string | null };

export function appendTaskPage(current: Task[] | null, page: Task[]): Task[] {
  const seen = new Set(current?.map((task) => task.id));
  return [...(current ?? []), ...page.filter((task) => !seen.has(task.id))];
}

function startOfDay(date: Date): number {
  return new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
  ).getTime();
}

export function visibleTasks(
  items: Task[],
  query: string,
  filter: CompletionFilter,
  now = new Date(),
): Task[] {
  const today = startOfDay(now);
  return items.filter((task) => {
    const completed = !task.is_completion_disabled && task.is_completed;
    if (filter === "incomplete" && completed) return false;
    if (filter === "completed" && !completed) return false;
    return (
      !!query.trim() ||
      !task.deadline ||
      startOfDay(new Date(task.deadline)) >= today
    );
  });
}

export function taskRows(
  items: Task[],
  headingForDate: (date: Date) => string,
  noDeadline: string,
): TaskRow[] {
  const groups = new Map<number, { heading: string; items: Task[] }>();
  for (const task of items) {
    const date = task.deadline ? new Date(task.deadline) : null;
    const key = date ? startOfDay(date) : Number.MAX_SAFE_INTEGER;
    if (!groups.has(key))
      groups.set(key, {
        heading: date ? headingForDate(date) : noDeadline,
        items: [],
      });
    groups.get(key)!.items.push(task);
  }
  return [...groups.entries()]
    .sort(([a], [b]) => a - b)
    .flatMap(([, group]) =>
      group.items.map((task, index) => ({
        task,
        heading: index === 0 ? group.heading : null,
      })),
    );
}
