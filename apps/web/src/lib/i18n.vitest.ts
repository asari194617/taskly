import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { t } from "./i18n";

const japanese = /[\u3040-\u30ff\u3400-\u9fff]/;

function flatten(value: unknown): string[] {
  if (typeof value === "string") return [value];
  if (value && typeof value === "object") return Object.values(value).flatMap(flatten);
  return [];
}

describe("translated web screens", () => {
  it("keeps translated sections available in both languages", () => {
    for (const section of [
      "common", "settingsNotifications", "consent", "settingsBasic",
      "settingsDetail", "settingsApiKeys", "settingsIcal", "settingsMcp", "dateTimePicker",
    ] as const) {
      expect(Object.keys(t("en")[section])).toEqual(Object.keys(t("ja")[section]));
      expect(flatten(t("en")[section])).not.toEqual([]);
      expect(flatten(t("en")[section]).every((text) => !japanese.test(text))).toBe(true);
    }
  });

  it("keeps fixed Japanese text out of translated screen components", () => {
    for (const path of [
      "../react/settings/Notifications.tsx", "../react/pages/ConsentPage.tsx", "../react/Footer.tsx",
      "../react/settings/Basic.tsx", "../react/settings/ApiKeys.tsx",
      "../react/settings/Ical.tsx", "../react/settings/Mcp.tsx",
      "../components/ui/date-time-picker.tsx",
    ]) {
      const source = readFileSync(fileURLToPath(new URL(path, import.meta.url)), "utf8");
      expect(source.match(japanese), path).toBeNull();
    }
  });
});
