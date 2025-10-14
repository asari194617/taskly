import * as React from "react";
import { t, type Lang } from "@/lib/i18n";

type Ctx = {
  lang: Lang;
  setLang: (l: Lang) => void;
  dict: ReturnType<typeof t>;
};

const LangContext = React.createContext<Ctx | null>(null);

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const [lang, setLangState] = React.useState<Lang>("ja");
  const dict = React.useMemo(() => t(lang), [lang]);

  React.useEffect(() => {
    const ls =
      (typeof window !== "undefined" && localStorage.getItem("display_lang")) ||
      null;
    if (ls === "ja" || ls === "en") {
      setLangState(ls);
      return;
    }
    try {
      const nav = navigator?.language || (navigator as any)?.userLanguage;
      const langs = navigator?.languages || [];
      const pick = (s: string) =>
        (s || "").toLowerCase().startsWith("ja") ? "ja" : "en";
      const detected = (langs[0] && pick(langs[0])) || pick(nav as string);
      setLangState(detected as Lang);
      localStorage.setItem("display_lang", detected as string);
    } catch {}
  }, []);

  const setLang = React.useCallback((l: Lang) => {
    setLangState(l);
    try {
      localStorage.setItem("display_lang", l);
    } catch {}
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
