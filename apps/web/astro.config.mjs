import { defineConfig } from "astro/config";
import react from "@astrojs/react";
import tailwind from "@astrojs/tailwind";

export default defineConfig({
  server: { port: 4321 },
  integrations: [react(), tailwind({ applyBaseStyles: false })],
});
