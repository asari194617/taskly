import { afterEach, expect, it, vi } from "vitest";

vi.mock("./api", () => ({ getApiBase: () => "https://api.example.test", webFetch: vi.fn() }));

import { webFetch } from "./api";
import { checkSession, loginRedirect, postLoginRedirect } from "./auth-guard";

afterEach(() => vi.clearAllMocks());

it("recognizes an authenticated session and sends cookies", async () => {
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: true, json: async () => ({ data: { me: { id: 1 } } }) } as Response);
  expect(await checkSession()).toBe(true);
  expect(webFetch).toHaveBeenCalledWith("https://api.example.test/graphql", expect.objectContaining({ method: "POST" }));
  expect(loginRedirect(true, "/home")).toBeNull();
});

it("treats rejected or invalid sessions as unauthenticated", async () => {
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: false } as Response);
  expect(await checkSession()).toBe(false);
  vi.mocked(webFetch).mockRejectedValueOnce(new Error("offline"));
  expect(await checkSession()).toBe(false);
  expect(loginRedirect(false, "/settings?tab=security#passkeys")).toBe("/login?next=%2Fsettings%3Ftab%3Dsecurity%23passkeys");
});

it("accepts only a local post-login destination", () => {
  expect(postLoginRedirect(true, "?next=%2Ftasks%3Fq%3Da")).toBe("/tasks?q=a");
  expect(postLoginRedirect(true, "?next=%2F%2Fevil.example")).toBe("/home");
  expect(postLoginRedirect(true, "?next=%2F%5Cevil.example")).toBe("/home");
  expect(postLoginRedirect(false, "?next=%2Ftasks")).toBeNull();
});
