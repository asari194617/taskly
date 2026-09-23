import { afterEach, describe, expect, it, vi } from "vitest";

import { clearLegacySessionToken, getApiBase, webFetch } from "./api";
import { gql } from "./graphql";

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("web API transport", () => {
  it("clears the legacy token and sends cookie credentials with a platform header", async () => {
    const removeItem = vi.fn();
    vi.stubGlobal("localStorage", { removeItem });
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await webFetch("/health", { headers: { Accept: "application/json" } });

    expect(removeItem).toHaveBeenCalledWith("api_token");
    expect(fetchMock).toHaveBeenCalledWith("/health", expect.objectContaining({
      credentials: "include",
      headers: expect.any(Headers),
    }));
    const headers = fetchMock.mock.calls[0][1].headers as Headers;
    expect(headers.get("Accept")).toBe("application/json");
    expect(headers.get("X-Client-Platform")).toBe("web");
  });

  it("preserves an explicit platform and tolerates inaccessible storage", async () => {
    vi.stubGlobal("localStorage", { removeItem: () => { throw new Error("blocked"); } });
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(webFetch("/health", { headers: { "X-Client-Platform": "pwa" } })).resolves.toBeInstanceOf(Response);
    expect((fetchMock.mock.calls[0][1].headers as Headers).get("X-Client-Platform")).toBe("pwa");
    expect(() => clearLegacySessionToken()).not.toThrow();
  });

  it("uses the configured default API origin", () => {
    expect(getApiBase()).toBe("http://localhost:3063");
  });
});

describe("GraphQL client", () => {
  it("posts query variables and returns data", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: { viewer: { id: 3 } } }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(gql<{ viewer: { id: number } }>("query Viewer", { limit: 2 })).resolves.toEqual({ viewer: { id: 3 } });
    const [url, request] = fetchMock.mock.calls[0];
    expect(url).toBe("http://localhost:3063/graphql");
    expect(JSON.parse(request.body)).toEqual({ query: "query Viewer", variables: { limit: 2 } });
    expect(request.signal).toBeUndefined();
  });

  it("combines GraphQL errors and falls back to the HTTP status text", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ errors: [{ message: "first" }, { message: "second" }] }), { status: 200 }))
      .mockResolvedValueOnce(new Response("{}", { status: 503, statusText: "Unavailable" }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(gql("bad query", undefined, { redirectOnUnauthorized: false })).rejects.toThrow("first\nsecond");
    await expect(gql("bad query", undefined, { redirectOnUnauthorized: false })).rejects.toThrow("Unavailable");
  });

  it("reports unauthorized responses without browser globals", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ errors: [] }), {
      status: 401,
      statusText: "Unauthorized",
    })));
    await expect(gql("query Viewer")).rejects.toThrow("Unauthorized");
  });
});
