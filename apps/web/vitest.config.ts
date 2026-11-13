import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: { alias: { "@": fileURLToPath(new URL("./src", import.meta.url)) } },
  test: {
    include: ["src/**/*.vitest.ts"],
    environment: "node",
    coverage: {
      include: [
        "src/features/**/api.ts",
        "src/features/notifications/settings.ts",
        "src/features/tasks/list-model.ts",
        "src/features/tasks/task-revalidation.ts",
        "src/lib/api.ts",
        "src/lib/graphql.ts",
        "src/lib/auth-guard.ts",
        "src/lib/token-expiry.ts",
        "src/react/login/flow.ts",
        "src/react/login/helpers.ts",
      ],
      exclude: ["src/**/*.vitest.ts", "src/**/*.d.ts"],
      thresholds: { lines: 75 },
    },
  },
});
