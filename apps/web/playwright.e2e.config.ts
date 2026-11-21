import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  workers: 1,
  retries: 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    ...devices["Desktop Chrome"],
    baseURL: "http://localhost:4321",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "PUBLIC_API_URL=http://localhost:3063 ASTRO_DEV_BACKGROUND=0 npm run dev -- --host localhost --port 4321",
    url: "http://localhost:4321/login",
    // cf forwards SIGTERM to the Astro process it starts in a separate process group.
    gracefulShutdown: { signal: "SIGTERM", timeout: 10_000 },
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
