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
    search: lang === "ja" ? "検索" : "Search",
    create: lang === "ja" ? "作成" : "Create",
    save: lang === "ja" ? "保存" : "Save",
    cancel: lang === "ja" ? "キャンセル" : "Cancel",
    delete: lang === "ja" ? "削除" : "Delete",
    edit: lang === "ja" ? "編集" : "Edit",
    close: lang === "ja" ? "閉じる" : "Close",
    reload: lang === "ja" ? "再読み込み" : "Reload",
    loading: lang === "ja" ? "読み込み中" : "Loading",
    noResults: lang === "ja" ? "結果がありません" : "No results",
  },
  home: {
    title: lang === "ja" ? "課題一覧" : "Task List",
    description:
      lang === "ja" ? "期限が近い順に表示します。" : "Sorted by deadline.",
  },
  tasks: {
    title: lang === "ja" ? "タイトル" : "Title",
    subject: lang === "ja" ? "サブジェクト" : "Subject",
    subjectId: lang === "ja" ? "教科ID" : "Subject ID",
    deadline: lang === "ja" ? "期限" : "Deadline",
    noDeadline: lang === "ja" ? "期限なし" : "No deadline",
    description: lang === "ja" ? "説明" : "Description",
    type: lang === "ja" ? "タイプ" : "Type",
    requireHandIn: lang === "ja" ? "提出必須" : "Submission required",
    selfCheckRequired: lang === "ja" ? "自己採点が必要" : "Self-check required",
    selfCheckBadge: lang === "ja" ? "自己採点" : "Self-check",
    optional: lang === "ja" ? "（任意）" : "(Optional)",
    searchPlaceholder:
      lang === "ja"
        ? "課題を検索 (タイトル/説明)"
        : "Search tasks (title/description)",
    newTask: lang === "ja" ? "新規作成" : "New task",
    createNewTask: lang === "ja" ? "新しい課題を作成" : "Create new task",
    edit: lang === "ja" ? "編集" : "Edit",
    editTask: lang === "ja" ? "課題を編集" : "Edit task",
    delete: lang === "ja" ? "削除" : "Delete",
    deleteTask: lang === "ja" ? "削除" : "Delete",
    actions: lang === "ja" ? "アクション" : "Actions",
    createTaskTitle: lang === "ja" ? "新規課題の作成" : "Create new task",
    createTaskDesc:
      lang === "ja"
        ? "必要な情報を入力してください。"
        : "Enter the required information.",
    enterRequiredInfo:
      lang === "ja"
        ? "必要な情報を入力してください。"
        : "Enter the required information.",
    create: lang === "ja" ? "新規作成" : "Create",
    save: lang === "ja" ? "保存" : "Save",
    cancel: lang === "ja" ? "キャンセル" : "Cancel",
    titlePlaceholder: lang === "ja" ? "課題のタイトル" : "Task title",
    selectPlaceholder: lang === "ja" ? "選択してください" : "Please select",
    notFound: lang === "ja" ? "見つかりませんでした" : "Not found",
    itemsCount: lang === "ja" ? "件" : "items",
    today: lang === "ja" ? "今日" : "Today",
    tomorrow: lang === "ja" ? "あす" : "Tomorrow",
    dayAfterTomorrow: lang === "ja" ? "あさって" : "Day after tomorrow",
    // Task types
    typeOnline: lang === "ja" ? "オンライン" : "Online",
    typeFaceToFace: lang === "ja" ? "対面" : "Face-to-face",
    typeQuiz: lang === "ja" ? "小テスト" : "Quiz",
    typePresentation: lang === "ja" ? "発表" : "Presentation",
    typeSelfStudy: lang === "ja" ? "自習/予習" : "Self-study/Preview",
    typeOther: lang === "ja" ? "その他" : "Other",
    // Empty states
    noTasks: lang === "ja" ? "課題はありません" : "No tasks",
    noTasksDesc:
      lang === "ja"
        ? "新しい課題を追加するとここに表示されます。"
        : "New tasks will appear here.",
    noSearchResults:
      lang === "ja" ? "検索結果がありません" : "No search results",
    noSearchResultsDesc:
      lang === "ja"
        ? "条件に一致する課題は見つかりませんでした。"
        : "No tasks match your search criteria.",
    loadingTasks: lang === "ja" ? "課題を取得しています…" : "Loading tasks...",
    loadFailed: lang === "ja" ? "読み込みに失敗しました" : "Failed to load",
    // Keyboard shortcuts
    shortcutsTitle: lang === "ja" ? "ショートカット" : "Shortcuts",
    shortcutsDesc:
      lang === "ja"
        ? "g → n: 新規作成 / ?: このヘルプ"
        : "g → n: New task / ?: Help",
    // Deadline labels
    morning: lang === "ja" ? "朝" : "Morning",
    lunchBreak: lang === "ja" ? "昼休み" : "Lunch break",
    afterSchool: lang === "ja" ? "放課後" : "After school",
    period: lang === "ja" ? "限" : "Period",
    until: lang === "ja" ? "まで" : "until",
    to: lang === "ja" ? "〜" : " to ",
  },
  toast: {
    userInfoFetchError:
      lang === "ja"
        ? "ユーザー情報の取得に失敗しました"
        : "Failed to fetch user information",
    loggedOut: lang === "ja" ? "ログアウトしました" : "Logged out",
    created: lang === "ja" ? "作成しました" : "Created",
    updated: lang === "ja" ? "更新しました" : "Updated",
    deleted: lang === "ja" ? "削除しました" : "Deleted",
    createFailed: lang === "ja" ? "作成に失敗しました" : "Failed to create",
    updateFailed: lang === "ja" ? "更新に失敗しました" : "Failed to update",
    deleteFailed: lang === "ja" ? "削除に失敗しました" : "Failed to delete",
    requiredFields:
      lang === "ja"
        ? "タイトル・サブジェクト・期限（日付）は必須です"
        : "Title, subject, and deadline are required",
  },
  settings: {
    title: lang === "ja" ? "設定" : "Settings",
    description:
      lang === "ja"
        ? "プロフィール、セキュリティ、通知、APIキー、iCalの設定を行います。"
        : "Manage profile, security, notifications, API keys, and iCal settings.",
    basic: lang === "ja" ? "基本設定" : "Basic Settings",
    basicDesc:
      lang === "ja"
        ? "表示名・アカウント削除"
        : "Display name, account deletion",
    security: lang === "ja" ? "セキュリティ設定" : "Security Settings",
    securityDesc:
      lang === "ja"
        ? "パスキー・TOTP・メールログイン・リカバリーコード"
        : "Passkey, TOTP, email login, recovery code",
    notifications: lang === "ja" ? "通知設定" : "Notification Settings",
    notificationsDesc:
      lang === "ja"
        ? "メール・Web Push など（準備中）"
        : "Email, web push, etc. (Coming soon)",
    apiKeys: lang === "ja" ? "APIキーの設定" : "API Key Settings",
    apiKeysDesc:
      lang === "ja" ? "APIキーの発行と管理" : "Issue and manage API keys",
    ical: lang === "ja" ? "iCalカレンダー設定" : "iCal Calendar Settings",
    icalDesc:
      lang === "ja"
        ? "購読用URLの発行と管理"
        : "Issue and manage subscription URLs",
    mcp: lang === "ja" ? "MCP 設定" : "MCP Settings",
    mcpDesc:
      lang === "ja"
        ? "MCPトークンの発行・無効化、操作履歴"
        : "Issue/revoke MCP tokens, operation history",
  },
});
