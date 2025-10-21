import i18next from "i18next";
import Backend from "i18next-fs-backend";
import path from "node:path";

export async function initI18n(localesDir: string) {
  await i18next.use(Backend).init({
    backend: {
      loadPath: path.join(localesDir, "{{lng}}/{{ns}}.json"),
    },
    lng: "en",
    fallbackLng: "en",
    preload: ["en", "ja"],
    ns: ["common", "emails"],
    defaultNS: "common",
    interpolation: {
      escapeValue: false,
    },
    returnNull: false,
    returnEmptyString: true,
  });
}

export function t(key: string, options?: Record<string, unknown>): string {
  return String(i18next.t(key, options as any));
}
