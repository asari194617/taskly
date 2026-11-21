import { bindings, defineConfig } from "cf/config";

export default defineConfig(({ mode }) => ({
  worker: {
    name: "taskly-email-inbound-local",
    entrypoint: "./worker.js",
    compatibilityDate: "2025-11-03",
    workersDev: false,
    previewUrls: false,
    env: {
      TASK_IMAGES: mode === "r2"
        ? bindings.r2({
            name: "taskly-task-images-local",
            dev: {
              experimentalS3Credentials: {
                accessKeyId: "taskly-local-access-key",
                secretAccessKey: "taskly-local-secret-key",
              },
            },
          })
        : undefined,
    },
  },
}));
