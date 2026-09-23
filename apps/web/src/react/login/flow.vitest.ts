import { afterEach, beforeEach, expect, it, vi } from "vitest";

vi.mock("./helpers", () => ({ apiBase: "https://api.example.test", webFetch: vi.fn() }));

import { webFetch } from "./helpers";
import { readMailCodeStatus, requestMailCode, secondsUntilResend } from "./flow";

const values = new Map<string, string>();

beforeEach(() => {
  values.clear();
  vi.stubGlobal("sessionStorage", {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

it("requests a mail code and records its cooldown", async () => {
  vi.spyOn(Date, "now").mockReturnValue(10_000);
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: true } as Response);
  await expect(requestMailCode("user@example.test")).resolves.toEqual({ ok: true, at: 10_000 });
  expect(readMailCodeStatus("user@example.test")).toEqual({ ok: true, at: 10_000 });
  expect(secondsUntilResend("user@example.test", 40_001)).toBe(30);
  expect(secondsUntilResend("user@example.test", 70_000)).toBe(0);
});

it("records HTTP and network failures", async () => {
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: false, text: async () => "rate limited" } as Response);
  await expect(requestMailCode("user@example.test")).resolves.toEqual({ ok: false, error: "rate limited" });
  expect(readMailCodeStatus("user@example.test")).toEqual({ ok: false, error: "rate limited" });

  vi.mocked(webFetch).mockRejectedValueOnce(new Error("offline"));
  await expect(requestMailCode("other@example.test")).resolves.toEqual({ ok: false, error: "offline" });
  expect(await requestMailCode("")).toEqual({ ok: false, error: "メールアドレスが未入力です" });
  expect(readMailCodeStatus("")).toBeNull();
  expect(secondsUntilResend("")).toBe(0);
});
