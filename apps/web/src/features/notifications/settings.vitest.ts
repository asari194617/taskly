import { expect, it, vi } from "vitest";
import { applyNotificationSetting, saveNotificationSetting } from "./settings";

it("saves notification method and type changes with the expected payload", async () => {
  const request = vi.fn().mockResolvedValue({ ok: true });
  await saveNotificationSetting("https://api.example.test", "notification-methods", { email: false }, request);
  await saveNotificationSetting("https://api.example.test", "notification-types", { TASK: false }, request);
  expect(request).toHaveBeenNthCalledWith(1, "https://api.example.test/settings/notification-methods", expect.objectContaining({ method: "POST", body: '{"email":false}' }));
  expect(request).toHaveBeenNthCalledWith(2, "https://api.example.test/settings/notification-types", expect.objectContaining({ body: '{"TASK":false}' }));
});

it("rolls back a failed HTTP response or network request", async () => {
  const changes: boolean[] = [];
  const publish = (value: boolean) => changes.push(value);
  const failedResponse = vi.fn().mockResolvedValue({ ok: false, status: 503 });
  await expect(applyNotificationSetting(true, false, publish, () =>
    saveNotificationSetting("/api", "notification-methods", { email: false }, failedResponse)
  )).rejects.toThrow("503");
  expect(changes).toEqual([false, true]);

  changes.length = 0;
  await expect(applyNotificationSetting(true, false, publish, async () => {
    throw new Error("offline");
  })).rejects.toThrow("offline");
  expect(changes).toEqual([false, true]);
});

it("keeps the new state after a successful save", async () => {
  const changes: boolean[] = [];
  await applyNotificationSetting(true, false, (value) => changes.push(value), async () => {});
  expect(changes).toEqual([false]);
});
