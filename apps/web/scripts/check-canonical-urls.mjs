import { readFileSync } from "node:fs";

const siteUrl = (process.env.SITE_URL || "https://taskly.iy0306.com").replace(
  /\/$/,
  "",
);

const pages = ["login", "login/password", "login/passkey"];

for (const page of pages) {
  const html = readFileSync(new URL(`../dist/${page}/index.html`, import.meta.url), "utf8");
  const expectedUrl = `${siteUrl}/${page}/`;
  const canonical = html.match(/<link rel="canonical" href="([^"]+)"/u)?.[1];
  const openGraphUrl = html.match(/<meta property="og:url" content="([^"]+)"/u)?.[1];

  if (canonical !== expectedUrl) {
    throw new Error(`${page}: expected canonical ${expectedUrl}, received ${canonical}`);
  }
  if (openGraphUrl !== expectedUrl) {
    throw new Error(`${page}: expected og:url ${expectedUrl}, received ${openGraphUrl}`);
  }
  if (html.includes("localhost")) {
    throw new Error(`${page}: production HTML contains localhost`);
  }
}

console.log("Canonical and Open Graph URLs are configured.");
