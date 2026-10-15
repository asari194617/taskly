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

// Task images use signed URLs on the private bucket for both PUT and GET.
const taskImageOrigin = "https://taskly-task-images.5a245d21c227e6c29395ad786899eb1e.r2.cloudflarestorage.com";
const csp = headers.match(/^  Content-Security-Policy: (.+)$/m)?.[1] ?? "";
const sources = new Map(csp.split(";").map((directive) => {
  const [name, ...values] = directive.trim().split(/\s+/);
  return [name, values];
}));
for (const directive of ["connect-src", "img-src"]) {
  if (!sources.get(directive)?.includes(taskImageOrigin)) {
    throw new Error(`Task image bucket must be allowed by ${directive}`);
  }
}

console.log("Security headers are configured.");
