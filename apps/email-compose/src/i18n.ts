import i18next, { type InitOptions } from "i18next";
import resourcesToBackend from "i18next-resources-to-backend";
import { initReactI18next } from "react-i18next";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import { readFile } from "node:fs/promises";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
let messagesDir = path.join(__dirname, "messages");
let initPromise: Promise<void> | null = null;

export async function initI18n(dir?: string) {
  if (dir) messagesDir = dir;
  if (!initPromise) {
    const options: InitOptions = {
      supportedLngs: ["en", "ja"],
      fallbackLng: "en",
      lng: undefined,
      preload: ["en", "ja"],
      ns: [
        "common",
        "login-code",
        "moderation-alert",
        "tasks-digest",
        "dayshift-skip",
        "day-shift",
      ],
      defaultNS: "common",
      interpolation: {
        escapeValue: false,
      },
      returnNull: false,
      returnEmptyString: true,
    };

    initPromise = i18next
      .use(initReactI18next)
      .use(
        resourcesToBackend(async (language: string, namespace: string) => {
          const filePath = path.join(
            messagesDir,
            language,
            `${namespace}.json`,
          );
          const raw = await readFile(filePath, "utf8");
          return JSON.parse(raw);
        }),
      )
      .init(options)
      .then(() => undefined);
  }
  await initPromise;
}

export { i18next };
