import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const logoUrl = new URL("../public/bimi.svg", import.meta.url);

test("BIMI logo uses the restricted SVG Tiny PS profile", async () => {
  const svg = await readFile(logoUrl, "utf8");

  assert.match(svg, /<svg\b[^>]*\bversion="1\.2"/);
  assert.match(svg, /<svg\b[^>]*\bbaseProfile="tiny-ps"/);
  assert.match(svg, /<svg\b[^>]*\bwidth="96"/);
  assert.match(svg, /<svg\b[^>]*\bheight="96"/);
  assert.match(svg, /<title>Taskly<\/title>/);
  assert.match(svg, /<desc>[^<]+<\/desc>/);
  assert.doesNotMatch(svg, /<(?:script|animate|foreignObject)\b/i);
  assert.doesNotMatch(svg, /\b(?:href|xlink:href)=/i);
  assert.ok(Buffer.byteLength(svg) <= 32 * 1024);
});
