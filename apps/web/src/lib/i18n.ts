export type Lang = "ja" | "en";

export const t = (lang: Lang) => ({
  common: {
    home: lang === "ja" ? "ホーム" : "Home",
    settings: lang === "ja" ? "設定" : "Settings",
    admin: lang === "ja" ? "管理" : "Admin",
    guide: lang === "ja" ? "ガイド" : "Guide",
    api: "API",
    support: lang === "ja" ? "サポート" : "Support",
    contact: lang === "ja" ? "お問い合わせ" : "Contact",
    releaseNotes: lang === "ja" ? "リリースノート" : "Release Notes",
    privacy: lang === "ja" ? "プライバシー" : "Privacy",
    terms: lang === "ja" ? "利用規約" : "Terms",
    language: lang === "ja" ? "表示言語" : "Language",
    japanese: lang === "ja" ? "日本語" : "Japanese",
    english: lang === "ja" ? "英語" : "English",
  },
});
