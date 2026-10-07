import { expect, it } from "vitest";
import { legacyLoginURL, loginNext, oauthStartURL } from "./login-redirect";

it("passes a local destination through the OAuth start URL", () => {
  const url = oauthStartURL("https://api.example.test/auth/oauth/start", "?next=%2Fsettings%2Fsecurity%3Ftab%3Dpasskeys");
  expect(new URL(url!).searchParams.get("next")).toBe("/settings/security?tab=passkeys");
  expect(legacyLoginURL("?email=user%40example.test&next=%2Fsettings")).toBe("/login?next=%2Fsettings");
});

it("drops external and malformed destinations", () => {
  for (const next of ["https://evil.example", "//evil.example", "/\\evil.example", "/\r\nLocation: https://evil.example", "/login"]) {
    const search = `?next=${encodeURIComponent(next)}`;
    expect(loginNext(search)).toBe("");
    expect(legacyLoginURL(search)).toBe("/login");
  }
  expect(oauthStartURL("javascript:alert(1)", "")).toBeNull();
});
