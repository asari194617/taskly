import { defineConfig, devices } from "@playwright/test";

const port = process.env.PLAYWRIGHT_PORT || "4321";
process.env.PUBLIC_AUTH_OAUTH_START_URL ||= "http://localhost:3063/auth/oauth/start";

export default defineConfig({
  testDir: "./tests/browser",
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    ...devices["Desktop Chrome"],
    // WebAuthn accepts localhost as a relying-party domain, but not an IP address.
    baseURL: `http://localhost:${port}`,
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command: `PLAYWRIGHT_TEST=1 ASTRO_DEV_BACKGROUND=0 npm run dev -- --host localhost --port ${port}`,
    env: { PUBLIC_AUTH_OAUTH_START_URL: process.env.PUBLIC_AUTH_OAUTH_START_URL },
    url: `http://localhost:${port}/login`,
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
