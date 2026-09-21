import assert from "node:assert/strict";
import test from "node:test";

import { buildCanonicalUrl } from "../src/lib/canonical-url.mjs";

test("uses the configured site and normalizes the trailing slash", () => {
  assert.equal(
    buildCanonicalUrl(
      new URL("http://localhost:4321/login/password"),
      new URL("https://taskly.iy0306.com"),
    ),
    "https://taskly.iy0306.com/login/password/",
  );
});

test("does not include query parameters or fragments", () => {
  assert.equal(
    buildCanonicalUrl(
      new URL(
        "http://localhost:4321/login/passkey?email=user%40example.com#challenge",
      ),
      new URL("https://taskly.iy0306.com"),
    ),
    "https://taskly.iy0306.com/login/passkey/",
  );
});
