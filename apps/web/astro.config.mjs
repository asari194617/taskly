import { defineConfig } from "astro/config";
import react from "@astrojs/react";

import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  // Preview deployments intentionally keep the production canonical origin by
  // default. Set SITE_URL explicitly only when a deployment should be indexed
  // under a different public origin.
  site: process.env.SITE_URL || "https://taskly.iy0306.com",
  server: { port: 4321 },
  devToolbar: { enabled: process.env.PLAYWRIGHT_TEST !== "1" },
  integrations: [react()],

  vite: {
    plugins: [tailwindcss()],
  },
});
