import { defineConfig } from "astro/config";
import react from "@astrojs/react";

import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  server: { port: 4321 },
  integrations: [react()],

  vite: {
    plugins: [tailwindcss()],
  },
});