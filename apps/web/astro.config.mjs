import { defineConfig } from "astro/config";
import react from "@astrojs/react";
import tailwind from "@astrojs/tailwind";
import path from 'path';

export default defineConfig({
  server: { port: 4321 },
  integrations: [react(), tailwind({ applyBaseStyles: false })],
  vite: {
    resolve: {
      alias: {
        '@': path.resolve(new URL('./src', import.meta.url).pathname),
      }
    }
  }
});
