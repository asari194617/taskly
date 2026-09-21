import * as React from "react";
import { t, type Lang } from "@/lib/i18n";

type Ctx = {
  lang: Lang;
  setLang: (l: Lang) => void;
  dict: ReturnType<typeof t>;
};

const LangContext = React.createContext<Ctx | null>(null);

function readInitialLang(): Lang {
  try {
    const stored = localStorage.getItem("display_lang");
    if (stored === "ja" || stored === "en") return stored;
    const preferred = navigator.languages?.[0] || navigator.language;
    return preferred?.toLowerCase().startsWith("ja") ? "ja" : "en";
  } catch {
    return "ja";
  }
}

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLangState] = React.useState<Lang>("ja");

  const dict = React.useMemo(() => t(lang), [lang]);

  React.useEffect(() => {
    const initial = readInitialLang();
    setLangState(initial);
    try {
      localStorage.setItem("display_lang", initial);
    } catch {}
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
  // Provider-less fallback: keep lang in local state synced with localStorage & events
  const [lang, setLangState] = React.useState<Lang>("ja");
  const dict = React.useMemo(() => t(lang), [lang]);
  React.useEffect(() => {
    const update = () => setLangState(readInitialLang());
    update();
    const handleStorage = (e: StorageEvent) => {
      if (e.key === "display_lang") update();
    };
    window.addEventListener("languagechange", update);
    window.addEventListener("storage", handleStorage);
    return () => {
      window.removeEventListener("languagechange", update);
      window.removeEventListener("storage", handleStorage);
    };
  }, []);
  const setLang = React.useCallback((l: Lang) => {
    setLangState(l);
    try {
      localStorage.setItem("display_lang", l);
      window.dispatchEvent(new Event("languagechange"));
    } catch {}
  }, []);
  return { lang, setLang, dict };
}
