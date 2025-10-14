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
    privacyPolicy: lang === "ja" ? "プライバシーポリシー" : "Privacy Policy",
    terms: lang === "ja" ? "利用規約" : "Terms",
    securityPolicy: lang === "ja" ? "セキュリティポリシー" : "Security Policy",
    continuityStatement:
      lang === "ja"
        ? "サービスの継続性確保に関する声明"
        : "Service Continuity Statement",
    language: lang === "ja" ? "表示言語" : "Language",
    japanese: lang === "ja" ? "日本語" : "Japanese",
    english: lang === "ja" ? "英語" : "English",
    products: lang === "ja" ? "製品" : "Products",
    documentation: lang === "ja" ? "ドキュメント" : "Documentation",
    platform: lang === "ja" ? "プラットフォーム" : "Platform",
    other: lang === "ja" ? "その他" : "Other",
    calendar: lang === "ja" ? "カレンダー" : "Calendar",
    mcpServer: lang === "ja" ? "MCPサーバー" : "MCP Server",
    logout: lang === "ja" ? "ログアウト" : "Logout",
    user: lang === "ja" ? "ユーザー" : "User",
  },
  home: {
    title: lang === "ja" ? "課題一覧" : "Task List",
    description:
      lang === "ja" ? "期限が近い順に表示します。" : "Sorted by deadline.",
  },
  toast: {
    userInfoFetchError:
      lang === "ja"
        ? "ユーザー情報の取得に失敗しました"
        : "Failed to fetch user information",
    loggedOut: lang === "ja" ? "ログアウトしました" : "Logged out",
  },
});
