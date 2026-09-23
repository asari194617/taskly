import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/graphql", () => ({ gql: vi.fn() }));

import { gql } from "@/lib/graphql";
import * as dayShift from "./admin-day-shift/api";
import * as users from "./admin-users/api";
import * as security from "./security/api";

const query = vi.mocked(gql);

afterEach(() => vi.clearAllMocks());

describe("day-shift admin API", () => {
  it("normalizes nullable list and index results", async () => {
    query
      .mockResolvedValueOnce({ adminUsersByIds: null })
      .mockResolvedValueOnce({ dayShiftIndex: null })
      .mockResolvedValueOnce({ adminDayshiftGroupIds: null })
      .mockResolvedValueOnce({ adminUsers: null });
    await expect(dayShift.loadUsersByIds([1, 2])).resolves.toEqual([]);
    await expect(dayShift.loadDayShiftIndex()).resolves.toBe(0);
    await expect(dayShift.loadGroupIds(4)).resolves.toEqual([]);
    await expect(dayShift.searchAdminUsers("")).resolves.toEqual([]);
    expect(query).toHaveBeenLastCalledWith(expect.stringContaining("adminUsers(q:$q)"), { q: undefined });
  });

  it("falls back to the legacy current-cycle query", async () => {
    query.mockRejectedValueOnce(new Error("old schema")).mockResolvedValueOnce({ adminDayshiftCurrentCycle: { id: 9 } });
    await expect(dayShift.loadCurrentCycleId()).resolves.toBe(9);
    expect(query.mock.calls[1][0]).toContain("adminDayshiftCurrentCycle");
    query.mockResolvedValueOnce({ dayShiftCurrentCycle: 5 });
    await expect(dayShift.loadCurrentCycleId()).resolves.toBe(5);
  });

  it("keeps every mutation contract", async () => {
    query.mockResolvedValue({});
    await dayShift.loadCycles(2, 25);
    await dayShift.loadGroups(3, 4, 10);
    await dayShift.createCycle("2026-01-01", "2026-03-31", "spring");
    await dayShift.deleteCycle(7);
    await dayShift.createGroup(3, [1, 2]);
    await dayShift.deleteGroup(3, 8);
    await dayShift.reorderGroups(3, [8, 9]);
    await dayShift.updateDayShiftIndex(2);
    await dayShift.setCurrentCycle(3);
    await dayShift.updateGroupMembers(8, [2, 4]);
    expect(query.mock.calls.map((call) => call[1])).toEqual([
      { page: 2, size: 25 }, { cid: 3, page: 4, size: 10 },
      { b: "2026-01-01", f: "2026-03-31", d: "spring" }, { id: 7 },
      { cid: 3, ids: [1, 2] }, { cid: 3, gid: 8 }, { cid: 3, ids: [8, 9] },
      { v: 2 }, { cid: 3 }, { gid: 8, ids: [2, 4] },
    ]);
  });
});

describe("user admin API", () => {
  it("chooses the correct role operation and normalizes devices", async () => {
    query.mockResolvedValueOnce({}).mockResolvedValueOnce({}).mockResolvedValueOnce({ adminUserDevices: null });
    await users.setBuiltInRole(4, "admin", true);
    await users.setBuiltInRole(4, "admin", false);
    await expect(users.loadDevices(4)).resolves.toEqual([]);
    expect(query.mock.calls[0][0]).toContain("adminUserAddRole");
    expect(query.mock.calls[1][0]).toContain("adminUserRemoveRole");
  });

  it("passes user, notification, and device inputs unchanged", async () => {
    query.mockResolvedValue({ adminUserBulkCreate: { created: 2, errors: [] } });
    await users.loadAdminUsers({ query: "a", sortBy: "email", sortDir: "asc", page: 2, pageSize: 20 });
    await users.blockUser(1, "spam"); await users.unblockUser(1);
    await users.updateUser(1, { email: "a@example.test", displayName: "A", language: "en", emailLogin: true });
    await users.createUser("b@example.test", "B", "ja");
    await expect(users.bulkCreateUsers("email\na@example.test")).resolves.toEqual({ created: 2, errors: [] });
    await users.setCustomRole(1, "reviewer", true);
    await users.loadNotificationSettings(1);
    await users.saveNotificationChannels(1, true, false);
    await users.saveNotificationType(1, "TASK", true);
    await users.updateDeviceEnabled(5, false); await users.deleteDevice(5);
    await users.createDevice(1, { platform: "web", name: "Chrome", payload: "{}", enabled: true });
    await users.deleteUser(1, false);
    expect(query).toHaveBeenCalledTimes(14);
    expect(query).toHaveBeenCalledWith(expect.stringContaining("adminUserCreateDevice"), { id: 1, pf: "web", name: "Chrome", payload: "{}", en: true });
  });
});

describe("security API", () => {
  it("returns setup values and serializes passkey registration", async () => {
    query
      .mockResolvedValueOnce({ totpSetup: { secret: "secret", otpauth: "otpauth://totp/test" } })
      .mockResolvedValueOnce({ recoveryGenerate: ["one"] })
      .mockResolvedValueOnce({ webauthnRegisterBegin: '{"challenge":"abc"}' })
      .mockResolvedValue({});
    await expect(security.beginTotpSetup()).resolves.toEqual({ secret: "secret", otpauth: "otpauth://totp/test" });
    await expect(security.generateRecoveryCodes()).resolves.toEqual(["one"]);
    await expect(security.beginPasskeyRegistration()).resolves.toEqual({ challenge: "abc" });
    await security.finishPasskeyRegistration({ id: 4 });
    expect(query).toHaveBeenLastCalledWith(expect.stringContaining("webauthnRegisterFinish"), { input: '{"id":4}' });
  });

  it("keeps settings mutation variables", async () => {
    query.mockResolvedValue({});
    await security.loadSecuritySettings(); await security.setEmailLogin(false);
    await security.activateTotpSetup("123456"); await security.disableTotpSetup();
    await security.setPassword(undefined, "new password");
    await security.renamePasskey(3, "Laptop"); await security.deletePasskey(3);
    expect(query).toHaveBeenCalledWith(expect.stringContaining("setPassword"), { current: undefined, next: "new password" });
    expect(query).toHaveBeenCalledTimes(7);
  });
});
