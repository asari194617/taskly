import { t, type Lang } from "./i18n";

// DOMの多言語対応テキストを更新
export function updateI18nTexts() {
  if (typeof window === "undefined") return;

  const lang = (localStorage.getItem("display_lang") as Lang | null) || "ja";
  const dict = t(lang);

  // data-i18n-key属性を持つ要素を探して更新
  document.querySelectorAll("[data-i18n-key]").forEach((el) => {
    const key = el.getAttribute("data-i18n-key");
    if (!key) return;

    const keys = key.split(".");
    let value: any = dict;
    for (const k of keys) {
      value = value?.[k];
    }

    if (typeof value === "string") {
      el.textContent = value;
    }
  });
}

// localStorageの変更を監視
export function watchLanguageChange() {
  if (typeof window === "undefined") return;

  window.addEventListener("storage", (e) => {
    if (e.key === "display_lang") {
      updateI18nTexts();
    }
  });

  // カスタムイベントでも更新（同じタブ内での変更用）
  window.addEventListener("languagechange", () => {
    updateI18nTexts();
  });
}

// 言語変更時にカスタムイベントを発火
export function dispatchLanguageChange() {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new Event("languagechange"));
}
