import { afterEach, expect, it, vi } from "vitest";

vi.mock("./api", () => ({ getApiBase: () => "https://api.example.test", webFetch: vi.fn() }));

import { webFetch } from "./api";
import { checkSession, currentPathWithQueryAndHash, loginRedirect, postLoginRedirect, redirectIfAuthed, requireAuth } from "./auth-guard";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
});

it("recognizes an authenticated session and sends cookies", async () => {
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: true, json: async () => ({ data: { me: { id: 1 } } }) } as Response);
  expect(await checkSession()).toBe(true);
  expect(webFetch).toHaveBeenCalledWith("https://api.example.test/graphql", expect.objectContaining({ method: "POST" }));
  expect(loginRedirect(true, "/home")).toBeNull();
});

it("treats rejected or invalid sessions as unauthenticated", async () => {
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: false, status: 401 } as Response);
  expect(await checkSession()).toBe(false);
  expect(loginRedirect(false, "/settings?tab=security#passkeys")).toBe("/login?next=%2Fsettings%3Ftab%3Dsecurity%23passkeys");
});

it("does not start a login loop when the session check is unavailable", async () => {
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: false, status: 429 } as Response);
  expect(await checkSession()).toBeNull();
  vi.mocked(webFetch).mockRejectedValueOnce(new Error("offline"));
  expect(await checkSession()).toBeNull();
  expect(loginRedirect(null, "/settings")).toBeNull();
});

it("accepts only a local post-login destination", () => {
  expect(postLoginRedirect(true, "?next=%2Ftasks%3Fq%3Da")).toBe("/tasks?q=a");
  expect(postLoginRedirect(true, "?next=%2F%2Fevil.example")).toBe("/home");
  expect(postLoginRedirect(true, "?next=%2F%5Cevil.example")).toBe("/home");
  expect(postLoginRedirect(false, "?next=%2Ftasks")).toBeNull();
});

it("rejects GraphQL errors and missing user ids", async () => {
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: true, json: async () => ({ errors: [{ message: "nope" }] }) } as Response);
  expect(await checkSession()).toBeNull();
  vi.mocked(webFetch).mockResolvedValueOnce({ ok: true, json: async () => ({ data: { me: null } }) } as Response);
  expect(await checkSession()).toBeNull();
});

it("builds the current browser path and performs guard redirects", async () => {
  const location = { pathname: "/settings", search: "?tab=security", hash: "#keys", replace: vi.fn() };
  vi.stubGlobal("location", location);
  expect(currentPathWithQueryAndHash()).toBe("/settings?tab=security#keys");

  vi.mocked(webFetch).mockResolvedValueOnce({ ok: false, status: 401 } as Response);
  requireAuth();
  await vi.waitFor(() => expect(location.replace).toHaveBeenCalledWith("/login?next=%2Fsettings%3Ftab%3Dsecurity%23keys"));

  vi.mocked(webFetch).mockResolvedValueOnce({ ok: true, json: async () => ({ data: { me: { id: 1 } } }) } as Response);
  redirectIfAuthed("/dashboard");
  await vi.waitFor(() => expect(location.replace).toHaveBeenCalledWith("/dashboard"));
  vi.unstubAllGlobals();
});
