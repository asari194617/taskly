import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/browser",
  workers: 1,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    ...devices["Desktop Chrome"],
    baseURL: "http://127.0.0.1:4321",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "ASTRO_DEV_BACKGROUND=0 npm run dev -- --host 127.0.0.1",
    url: "http://127.0.0.1:4321/login",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
