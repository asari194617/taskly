import { afterEach, expect, it, vi } from "vitest";

vi.mock("../../lib/graphql", () => ({ gql: vi.fn() }));

import { gql } from "../../lib/graphql";
import { arrayToB64url, b64urlToArray, clearAuthTransaction, getEmailFromQuery, getNextParam, isAuthTransactionError, postLoginRedirect, readAuthTransaction, saveAuthTransaction, withNext } from "./helpers";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

it("redirects to the requested page after confirming the cookie session", async () => {
  const location = { search: "?next=%2Fsettings", replace: vi.fn() };
  vi.stubGlobal("location", location);
  vi.stubGlobal("window", { location });
  vi.mocked(gql).mockResolvedValueOnce({ me: { is_agreed_policies: true } });

  await postLoginRedirect();

  expect(gql).toHaveBeenCalledWith(
    "query{ me{ is_agreed_policies } }",
    undefined,
    { redirectOnUnauthorized: false },
  );
  expect(location.replace).toHaveBeenCalledWith("/settings");
});

it("keeps the login page open when the cookie session is unavailable", async () => {
  const location = { search: "?next=%2Fsettings", replace: vi.fn() };
  vi.stubGlobal("location", location);
  vi.stubGlobal("window", { location });
  vi.mocked(gql).mockRejectedValueOnce(new Error("unauthorized"));

  await expect(postLoginRedirect()).rejects.toThrow("ログイン状態を確認できませんでした");
  expect(location.replace).not.toHaveBeenCalled();
});

it("stores, validates, and clears auth transactions", () => {
  const values = new Map<string, string>();
  vi.stubGlobal("sessionStorage", {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value),
    removeItem: (key: string) => values.delete(key),
  });
  vi.spyOn(Date, "now").mockReturnValue(1_000);
  saveAuthTransaction("tx-1", " USER@Example.Test ", 60, ["password", "totp"]);
  expect(readAuthTransaction("user@example.test")).toMatchObject({ id: "tx-1", email: "user@example.test" });
  expect(readAuthTransaction("other@example.test")).toBeNull();
  saveAuthTransaction("tx-2", "user@example.test", -1, []);
  expect(readAuthTransaction("user@example.test")).toBeNull();
  clearAuthTransaction();
});

it("handles navigation helpers and auth errors", () => {
  vi.stubGlobal("location", { search: "?next=%2Fsettings%3Ftab%3Dsecurity&email=user%40example.test", origin: "https://taskly.example" });
  expect(getNextParam()).toBe("/settings?tab=security");
  expect(getEmailFromQuery()).toBe("user@example.test");
  expect(withNext("/login/password")).toBe("/login/password?next=%2Fsettings%3Ftab%3Dsecurity");
  expect(isAuthTransactionError("auth transaction expired")).toBe(true);
  expect(isAuthTransactionError("password required")).toBe(true);
  expect(isAuthTransactionError(new Error("password required"))).toBe(false);

  vi.stubGlobal("location", { search: "?next=%2F%2Fevil.example", origin: "https://taskly.example" });
  expect(getNextParam()).toBe("");
  expect(withNext("/login/password")).toBe("/login/password");
});

it("converts WebAuthn base64url values", () => {
  const bytes = new Uint8Array([0, 1, 2, 253, 254, 255]);
  const encoded = arrayToB64url(bytes);
  expect(encoded).toBe("AAEC_f7_");
  expect(Array.from(b64urlToArray(encoded))).toEqual(Array.from(bytes));
});

it("redirects users without current policy consent", async () => {
  const location = { search: "?next=%2Fsettings", replace: vi.fn() };
  vi.stubGlobal("location", location);
  vi.stubGlobal("window", { location });
  vi.mocked(gql).mockResolvedValueOnce({ me: { is_agreed_policies: false } });
  await postLoginRedirect();
  expect(location.replace).toHaveBeenCalledWith("/consent?next=%2Fsettings");
});
