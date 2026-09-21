import { describe, expect, it } from "vitest";
import { appendTaskPage, taskRows, visibleTasks } from "./list-model";
import type { Task } from "./api";

const tasks: Task[] = [
  {
    id: 1,
    title: "past",
    subject_id: 1,
    deadline: "2026-09-20",
    is_completed: false,
  },
  {
    id: 2,
    title: "today",
    subject_id: 1,
    deadline: "2026-09-21",
    is_completed: false,
  },
  {
    id: 3,
    title: "later",
    subject_id: 1,
    deadline: "2026-09-22",
    is_completed: true,
  },
  { id: 4, title: "none", subject_id: 1, deadline: null, is_completed: false },
];

describe("task list presentation", () => {
  it("hides expired tasks until searching and applies the completion filter", () => {
    const now = new Date(2026, 8, 21, 12);
    expect(visibleTasks(tasks, "", "all", now).map((task) => task.id)).toEqual([
      2, 3, 4,
    ]);
    expect(
      visibleTasks(tasks, "past", "all", now).map((task) => task.id),
    ).toEqual([1, 2, 3, 4]);
    expect(
      visibleTasks(tasks, "", "completed", now).map((task) => task.id),
    ).toEqual([3]);
    expect(
      visibleTasks(tasks, "", "incomplete", now).map((task) => task.id),
    ).toEqual([2, 4]);
  });

  it("orders dated groups before undated tasks and emits one heading per group", () => {
    const rows = taskRows(
      [tasks[3], tasks[2], tasks[0], { ...tasks[0], id: 5 }],
      (date) => String(date.getDate()),
      "none",
    );
    expect(rows.map(({ task, heading }) => [task.id, heading])).toEqual([
      [1, "20"],
      [5, null],
      [3, "22"],
      [4, "none"],
    ]);
  });

  it("appends cursor results without duplicating an item already loaded", () => {
    expect(
      appendTaskPage([tasks[0], tasks[1]], [tasks[1], tasks[2]]).map(
        (task) => task.id,
      ),
    ).toEqual([1, 2, 3]);
  });
});
