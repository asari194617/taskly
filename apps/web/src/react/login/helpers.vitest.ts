import { afterEach, expect, it, vi } from "vitest";

vi.mock("../../lib/graphql", () => ({ gql: vi.fn() }));

import { gql } from "../../lib/graphql";
import { postLoginRedirect } from "./helpers";

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
