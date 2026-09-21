import { readFileSync } from "node:fs";

const headersFile = new URL("../public/_headers", import.meta.url);
const headers = readFileSync(headersFile, "utf8");

const requiredHeaders = [
  "Content-Security-Policy",
  "Cross-Origin-Opener-Policy",
  "Cross-Origin-Resource-Policy",
  "Permissions-Policy",
  "Referrer-Policy",
  "Strict-Transport-Security",
  "X-Content-Type-Options",
  "X-Frame-Options",
];

const requiredCspDirectives = [
  "base-uri 'self'",
  "connect-src 'self' https://api.taskly.iy0306.com",
  "form-action 'self'",
  "frame-ancestors 'none'",
  "object-src 'none'",
  "upgrade-insecure-requests",
];

for (const header of requiredHeaders) {
  if (!headers.includes(`  ${header}:`)) {
    throw new Error(`Missing required security header: ${header}`);
  }
}

for (const directive of requiredCspDirectives) {
  if (!headers.includes(directive)) {
    throw new Error(`Missing required CSP directive: ${directive}`);
  }
}

console.log("Security headers are configured.");
