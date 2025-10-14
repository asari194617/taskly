import * as React from "react";
import { t, type Lang } from "@/lib/i18n";

type Ctx = {
  lang: Lang;
  setLang: (l: Lang) => void;
  dict: ReturnType<typeof t>;
};

const LangContext = React.createContext<Ctx | null>(null);

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLangState] = React.useState<Lang>(() => {
    // 初期値をlocalStorageから読み込む
    if (typeof window === "undefined") return "ja";

    const stored = localStorage.getItem("display_lang");
    if (stored === "ja" || stored === "en") {
      return stored;
    }

    // localStorageになければブラウザの言語設定から判定
    try {
      const nav = navigator?.language || (navigator as any)?.userLanguage;
      const langs = navigator?.languages || [];
      const pick = (s: string) =>
        (s || "").toLowerCase().startsWith("ja") ? "ja" : "en";
      const detected = (langs[0] && pick(langs[0])) || pick(nav as string);
      return detected as Lang;
    } catch {
      return "ja";
    }
  });

  const dict = React.useMemo(() => t(lang), [lang]);

  React.useEffect(() => {
    // 初回マウント時にlocalStorageに保存（まだ保存されていない場合）
    const stored = localStorage.getItem("display_lang");
    if (!stored || (stored !== "ja" && stored !== "en")) {
      localStorage.setItem("display_lang", lang);
    }
  }, []);

  const setLang = React.useCallback((l: Lang) => {
    console.log("[LanguageProvider] setLang called with:", l);
    setLangState(l);
    try {
      localStorage.setItem("display_lang", l);
      console.log("[LanguageProvider] Saved to localStorage:", l);
      // カスタムイベントを発火してAstroページも更新
      window.dispatchEvent(new Event("languagechange"));
    } catch (e) {
      console.error("[LanguageProvider] Failed to save to localStorage:", e);
    }
  }, []);

  return (
    <LangContext.Provider value={{ lang, setLang, dict }}>
      {children}
    </LangContext.Provider>
  );
}

export function useLang() {
  const ctx = React.useContext(LangContext);
  if (ctx) return ctx;
  // Fallback for SSR or when provider not mounted yet
  return {
    lang: "ja",
    setLang: () => {},
    dict: t("ja"),
  };
}
