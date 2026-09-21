import assert from "node:assert/strict";
import test from "node:test";
import {
  normalizeTaskSearch,
  updateTaskCompletion,
} from "./state.mjs";

test("normalizeTaskSearch trims usable queries and suppresses short ones", () => {
  assert.equal(normalizeTaskSearch("  report  "), "report");
  assert.equal(normalizeTaskSearch(" a "), "");
  assert.equal(normalizeTaskSearch("   "), "");
});

test("updateTaskCompletion only changes the target task", () => {
  const tasks = [
    { id: 1, title: "first", is_completed: false },
    { id: 2, title: "second", is_completed: false },
  ];
  const updated = updateTaskCompletion(tasks, 2, true);

  assert.deepEqual(updated, [tasks[0], { ...tasks[1], is_completed: true }]);
  assert.notEqual(updated, tasks);
  assert.equal(updated[0], tasks[0]);
});
