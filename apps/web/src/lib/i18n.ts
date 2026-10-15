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
    creatorIconAlt: lang === "ja" ? "asari194617のアイコン" : "asari194617's icon",
    english: lang === "ja" ? "英語" : "English",
    products: lang === "ja" ? "製品" : "Products",
    documentation: lang === "ja" ? "ドキュメント" : "Documentation",
    platform: lang === "ja" ? "プラットフォーム" : "Platform",
    appleComingSoon:
      lang === "ja" ? "iPhone,iPad,Mac(近日公開)" : "iPhone, iPad, Mac (coming soon)",
    androidComingSoon:
      lang === "ja" ? "Android(近日公開)" : "Android (coming soon)",
    windowsComingSoon:
      lang === "ja" ? "Windows(近日公開)" : "Windows (coming soon)",
    other: lang === "ja" ? "その他" : "Other",
    calendar: lang === "ja" ? "カレンダー" : "Calendar",
    mcpServer: lang === "ja" ? "MCPサーバー" : "MCP Server",
    login: lang === "ja" ? "ログイン" : "Login",
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
    menu: lang === "ja" ? "メニュー" : "Menu",
    email: lang === "ja" ? "メールアドレス" : "Email",
    displayName: lang === "ja" ? "表示名" : "Display name",
    notificationLanguage:
      lang === "ja" ? "通知の言語" : "Notification language",
    copy: lang === "ja" ? "コピー" : "Copy",
    copied: lang === "ja" ? "コピーしました" : "Copied",
    print: lang === "ja" ? "印刷" : "Print",
    saving: lang === "ja" ? "保存中..." : "Saving...",
    enabled: lang === "ja" ? "有効" : "Enabled",
    disabled: lang === "ja" ? "無効" : "Disabled",
    enable: lang === "ja" ? "有効化" : "Enable",
    disable: lang === "ja" ? "無効化" : "Disable",
    openapiFile: lang === "ja" ? "OpenAPIファイル" : "OpenAPI file",
    warning: lang === "ja" ? "注意" : "Warning",
  },
  home: {
    title: lang === "ja" ? "課題一覧" : "Task List",
    description:
      lang === "ja" ? "期限が近い順に表示します。" : "Sorted by deadline.",
  },
  consent: {
    heading: lang === "ja" ? "ご利用にあたって" : "Before you continue",
    description:
      lang === "ja"
        ? "以下のポリシーに同意の上、進んでください。"
        : "Please agree to the following policies to continue.",
    agree: lang === "ja" ? "に同意します" : "I agree to the",
    continue: lang === "ja" ? "同意して進む" : "Agree and continue",
    version: lang === "ja" ? "文書版" : "Document version",
    error: lang === "ja"
      ? "同意を記録できませんでした。再読み込みしてお試しください。"
      : "Could not record consent. Reload and try again.",
    notice: (date: string) => lang === "ja"
      ? `利用規約・プライバシー・セキュリティポリシーが ${date} に改定されます。`
      : `The terms, privacy and security policies change on ${date}.`,
    noticeAction: lang === "ja" ? "変更内容を確認して同意する" : "Review and agree",
  },
  landing: {
    hero: {
      title: "Taskly",
      tagline:
        lang === "ja"
          ? "シンプルで高速なクラス管理サービス。課題の管理、日直の通知、APIキーやiCalなどをサポートします。"
          : "Simple and fast class management. Manage tasks, morning digest notifications, API keys, iCal, and more.",
      loginCta: lang === "ja" ? "ログイン" : "Log in",
      settingsCta: lang === "ja" ? "設定" : "Settings",
    },
    features: {
      title: lang === "ja" ? "主な機能" : "Key features",
      fast: {
        title: lang === "ja" ? "高速な課題管理" : "Fast task management",
        desc:
          lang === "ja"
            ? "締切順の一覧、クイック作成、自己採点のフラグなど。"
            : "Deadline-first list, quick create, self-check flags, and more.",
      },
      push: {
        title: lang === "ja" ? "朝のプッシュ通知" : "Morning push digest",
        desc:
          lang === "ja"
            ? "毎朝のダイジェストやテスト通知をサポート。"
            : "Daily digest and test notifications.",
      },
      api: {
        title: "APIキー",
        desc:
          lang === "ja"
            ? "CI/CDや外部連携に使えるAPIキーを発行。"
            : "Issue API keys for CI/CD and integrations.",
      },
      ical: {
        title: "iCal",
        desc:
          lang === "ja"
            ? "購読URLで予定表アプリと同期。"
            : "Subscribe via URL and sync with calendar apps.",
      },
      security: {
        title: lang === "ja" ? "セキュリティ" : "Security",
        desc:
          lang === "ja"
            ? "パスキー/TOTP/メールコード、リカバリーコードに対応。"
            : "Passkey/TOTP/email codes and recovery codes.",
      },
    },
  },
  tasks: {
    title: lang === "ja" ? "タイトル" : "Title",
    completion: lang === "ja" ? "完了" : "Completed",
    subject: lang === "ja" ? "サブジェクト" : "Subject",
    subjectId: lang === "ja" ? "教科ID" : "Subject ID",
    deadline: lang === "ja" ? "期限" : "Deadline",
    noDeadline: lang === "ja" ? "期限なし" : "No deadline",
    description: lang === "ja" ? "説明" : "Description",
    image: lang === "ja" ? "画像" : "Image",
    images: lang === "ja" ? "画像" : "Images",
    imageDrop:
      lang === "ja"
        ? "画像をドラッグ&ドロップ、またはクリックして選択"
        : "Drag and drop images, or click to choose",
    removeImage: lang === "ja" ? "画像を削除" : "Remove image",
    imageReady: lang === "ja" ? "アップロード待ち" : "Ready to upload",
    imageUploading: lang === "ja" ? "アップロード中" : "Uploading",
    imageProcessing: lang === "ja" ? "処理中" : "Processing",
    imageUploaded: lang === "ja" ? "アップロード完了" : "Uploaded",
    imageUploadFailed: lang === "ja"
      ? "アップロードに失敗しました。保存し直して再試行してください。"
      : "Upload failed. Save again to retry.",
    imageInvalid:
      lang === "ja"
        ? "対応形式の10 MiB以下の画像を選択してください。"
        : "Choose a supported image no larger than 10 MiB.",
    imageTooMany:
      lang === "ja" ? "画像は最大5枚までです。" : "You can attach up to 5 images.",
    imageHelp:
      lang === "ja"
        ? "JPEG・PNG・WebP・GIF（1枚10 MiB、最大5枚）"
        : "JPEG, PNG, WebP, or GIF (10 MiB each, up to 5)",
    type: lang === "ja" ? "タイプ" : "Type",
    requireHandIn: lang === "ja" ? "提出必須" : "Submission required",
    disableCompletion: lang === "ja" ? "完了マークを無効にする" : "Disable completion marking",
    selfCheckRequired: lang === "ja" ? "自己採点が必要" : "Self-check required",
    selfCheckBadge: lang === "ja" ? "自己採点" : "Self-check",
    optional: lang === "ja" ? "（任意）" : "(Optional)",
    searchPlaceholder:
      lang === "ja"
        ? "課題を検索 (タイトル/説明)"
        : "Search tasks (title/description)",
    searchLabel: lang === "ja" ? "課題を検索" : "Search tasks",
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
    completionFilter: lang === "ja" ? "完了状態で絞り込み" : "Filter by completion status",
    incomplete: lang === "ja" ? "未完了" : "Incomplete",
    completed: lang === "ja" ? "完了済み" : "Completed",
    all: lang === "ja" ? "すべて" : "All",
    showingFilter: lang === "ja" ? "表示中" : "Showing",
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
    // Report form
    reportReasonLabel: lang === "ja" ? "通報理由" : "Report reason",
    reportReasonPlaceholder:
      lang === "ja" ? "スパム・不適切な内容など" : "Spam/Inappropriate/etc.",
    reportCommentLabel: lang === "ja" ? "コメント" : "Comment",
    reportCommentPlaceholder:
      lang === "ja" ? "任意の詳細（任意）" : "Optional details",
    reportSubmit: lang === "ja" ? "通報する" : "Report",
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
    openMenu:
      lang === "ja" ? "設定メニューを開く" : "Open settings menu",
    toggleMenu:
      lang === "ja" ? "設定メニューを切り替える" : "Toggle settings menu",
    description:
      lang === "ja"
        ? "プロフィール、セキュリティ、通知、APIキー、iCalの設定を行います。"
        : "Manage profile, security, notifications, API keys, and iCal settings.",
    profile: lang === "ja" ? "アカウント情報" : "Account profile",
    profileDesc:
      lang === "ja"
        ? "メールアドレスの確認・表示名の設定"
        : "Verify your email address and set your display name",
    basic: lang === "ja" ? "基本設定" : "Basic Settings",
    basicDesc:
      lang === "ja"
        ? "配信メールの言語・退会"
        : "Email language, account closure",
    basicLegacyDesc:
      lang === "ja"
        ? "表示名・配信メールの言語・退会"
        : "Display name, email language, account closure",
    security: lang === "ja" ? "セキュリティ設定" : "Security Settings",
    securityDesc:
      lang === "ja"
        ? "パスキー・TOTP・メールログイン・リカバリーコード"
        : "Passkey, TOTP, email login, recovery code",
    notifications: lang === "ja" ? "通知設定" : "Notification Settings",
    notificationsDesc:
      lang === "ja" ? "メール・Web Push など" : "Email, web push, etc.",
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
  settingsBasic: {
    heading: lang === "ja" ? "基本設定" : "Basic Settings",
    email: lang === "ja" ? "メールアドレス" : "Email",
    displayName: lang === "ja" ? "表示名" : "Display name",
    notificationLang: lang === "ja" ? "配信メールの言語" : "Email language",
    deleteAccount: lang === "ja" ? "退会する" : "Close account",
    startClosure: lang === "ja" ? "退会手続きへ" : "Continue to account closure",
    closingAccount: lang === "ja" ? "退会処理中..." : "Closing account...",
    closureSummary:
      lang === "ja"
        ? "退会すると、ログインと連携はすぐに停止します。課題などのデータは30日後の日次処理で消去されます。"
        : "Closing your account immediately stops sign-in and integrations. Tasks and other data are erased in a daily job after 30 days.",
    closureAccess:
      lang === "ja"
        ? "ログインと連携がすぐに使えなくなります。"
        : "You will immediately lose access to sign-in and integrations.",
    closureData:
      lang === "ja"
        ? "共有課題を含むデータは30日間保持され、その後の日次処理で消去されます。"
        : "Data, including shared tasks, is retained for 30 days, then erased in a daily job.",
    erasureLink: lang === "ja" ? "退会後のデータ取扱い" : "account data policy",
    deleteConfirmTitle:
      lang === "ja" ? "退会しますか？" : "Close your account?",
    emailLanguageSaveFailed:
      lang === "ja" ? "メールの言語設定を保存できませんでした" : "Could not save the email language setting",
    saved: lang === "ja" ? "保存しました" : "Saved",
    saveFailed: lang === "ja" ? "保存に失敗しました" : "Failed to save",
    deleteFailed: lang === "ja" ? "退会できませんでした" : "Failed to close account",
  },
  settingsDetail: {
    help: lang === "ja" ? "ヘルプ" : "Help",
    expiryOptional: lang === "ja" ? "有効期限(任意)" : "Expiration (optional)",
    selectDateTime: lang === "ja" ? "日時を選択" : "Select date and time",
    expiry: lang === "ja" ? "期限" : "Expiration",
    expired: lang === "ja" ? "期限切れ" : "Expired",
    updateExpiry: lang === "ja" ? "期限を更新" : "Update expiration",
    expiryDialog: lang === "ja" ? "有効期限を設定/更新" : "Set or update expiration",
    clearExpiryHint:
      lang === "ja" ? "空欄で保存すると期限を解除します。" : "Save an empty value to remove the expiration date.",
    invalidExpiry: lang === "ja" ? "有効な日時を選択してください。" : "Select a valid date and time.",
    loadFailed: lang === "ja" ? "読み込みに失敗しました" : "Failed to load",
    updateFailed: lang === "ja" ? "更新に失敗しました" : "Failed to update",
    copyFailed: lang === "ja" ? "コピーに失敗しました" : "Failed to copy",
    deleteFailed: lang === "ja" ? "削除に失敗しました" : "Failed to delete",
    deleteConfirm: lang === "ja" ? "削除の確認" : "Confirm deletion",
  },
  dateTimePicker: {
    select: lang === "ja" ? "選択してください" : "Select date and time",
    time: lang === "ja" ? "時刻" : "Time",
    clear: lang === "ja" ? "クリア" : "Clear",
    done: lang === "ja" ? "決定" : "Done",
  },
  settingsApiKeys: {
    heading: lang === "ja" ? "APIキー" : "API Keys",
    descOptional: lang === "ja" ? "説明（任意）" : "Description (optional)",
    exampleDescription: lang === "ja" ? "例: CI/CD" : "e.g. CI/CD",
    issueFailed: lang === "ja" ? "発行に失敗しました" : "Failed to issue API key",
    issueNew: lang === "ja" ? "新しいAPIキーを発行" : "Issue new API key",
    dialogTitle: lang === "ja" ? "APIキーを発行" : "Issue API key",
    tokenWarn:
      lang === "ja"
        ? "このトークンは一度しか表示されません。必ず保存してください。"
        : "This token will be shown only once. Please store it securely.",
    shareWarning:
      lang === "ja"
        ? "APIキーは他人と共有しないでください。パスワードと同様に安全に保管してください。"
        : "Never share your API keys. Treat them like passwords and store them securely.",
    table: {
      id: "ID",
      description: lang === "ja" ? "説明" : "Description",
      status: lang === "ja" ? "状態" : "Status",
      createdAt: lang === "ja" ? "作成日" : "Created at",
      actions: lang === "ja" ? "操作" : "Actions",
    },
  },
  settingsIcal: {
    heading: "iCal",
    descOptional: lang === "ja" ? "説明（任意）" : "Description (optional)",
    exampleDescription: lang === "ja" ? "例: 個人用" : "e.g. Personal",
    createFailed: lang === "ja" ? "作成に失敗しました" : "Failed to create iCal URL",
    deleteUrlConfirm:
      lang === "ja" ? "このiCal URLを削除します。よろしいですか？" : "Delete this iCal URL?",
    deleteUrl: lang === "ja" ? "削除する" : "Delete URL",
    issueUrl: lang === "ja" ? "URLを発行" : "Issue URL",
    shareWarning:
      lang === "ja"
        ? "iCal購読URLを第三者に共有することは利用規約で禁止されています。URLは機密情報として扱い、公開しないでください。"
        : "Sharing your iCal subscription URL with others is prohibited by the terms of service. Treat it as a secret and do not share it.",
    table: {
      id: "ID",
      description: lang === "ja" ? "説明" : "Description",
      url: "URL",
      createdAt: lang === "ja" ? "作成日" : "Created at",
      actions: lang === "ja" ? "操作" : "Actions",
    },
  },
  settingsMcp: {
    heading: lang === "ja" ? "MCP 設定" : "MCP Settings",
    disabled:
      lang === "ja"
        ? "この環境ではMCPが無効化されています。"
        : "MCP is disabled in this environment.",
    description:
      lang === "ja"
        ? "アプリ連携用のMCPトークンを発行・無効化できます。トークンは一度しか表示されません。"
        : "You can issue/revoke MCP tokens for integrations. Token is shown only once.",
    setup: {
      title: lang === "ja" ? "接続手順" : "Connect to Taskly",
      description:
        lang === "ja"
          ? "Taskly は接続用プログラム（taskly-mcp）を経由して利用します。接続先URLをクライアントへ直接追加する方式ではありません。"
          : "Taskly connects through the taskly-mcp helper. Do not add the destination URL directly as a remote MCP server.",
      installTitle: lang === "ja" ? "接続用プログラムをインストール" : "Install the connection helper",
      installDescription:
        lang === "ja"
          ? "macOS・Linux はガイドのコマンドでインストールできます。Windows の手順もガイドにあります。"
          : "Use the guide command on macOS or Linux. The guide also includes Windows instructions.",
      tokenTitle: lang === "ja" ? "この画面でトークンを発行" : "Issue a token on this page",
      tokenDescription:
        lang === "ja"
          ? "表示されたトークンは一度だけコピーできます。接続設定が終わるまで安全に保管してください。"
          : "You can copy the token only when it is shown. Keep it secure until setup is complete.",
      configureTitle: lang === "ja" ? "クライアントに設定を追加" : "Add the client configuration",
      configureDescription:
        lang === "ja"
          ? "下の設定値を使い、クライアントを再起動して Taskly のツールが表示されることを確認します。"
          : "Use the values below, restart your client, and confirm that the Taskly tools appear.",
      detailsTitle: lang === "ja" ? "接続に使う設定値" : "Connection settings",
      command: lang === "ja" ? "実行コマンド" : "Command",
      baseUrl: lang === "ja" ? "接続先URL" : "Destination URL",
      tokenVariable: lang === "ja" ? "トークンの設定名" : "Token variable",
      configTitle: lang === "ja" ? "設定例（JSON形式）" : "Example configuration (JSON)",
      configDescription:
        lang === "ja"
          ? "<issued token> を、この画面で発行したトークンに置き換えてください。設定ファイルの場所はクライアントごとに異なります。"
          : "Replace <issued token> with the token issued on this page. The configuration file location varies by client.",
      copyConfig: lang === "ja" ? "設定例をコピー" : "Copy example",
      openGuide: lang === "ja" ? "詳しい接続ガイドを開く" : "Open the detailed setup guide",
      nextStep:
        lang === "ja"
          ? "次に、接続ガイドの設定例へこのトークンを入力してください。"
          : "Next, put this token into the configuration shown in the setup guide.",
    },
    issueToken: lang === "ja" ? "トークンを発行" : "Issue token",
    revokeToken: lang === "ja" ? "トークンを無効化" : "Revoke token",
    revokedMsg: lang === "ja" ? "MCPトークンを無効化しました" : "MCP token revoked",
    dialogTitle: lang === "ja" ? "MCPトークン" : "MCP Token",
    tokenWarn:
      lang === "ja"
        ? "このトークンは一度しか表示されません。必ず安全な場所に保存してください。"
        : "This token will be shown only once. Store it securely.",
    shareWarning:
      lang === "ja"
        ? "MCPトークンは第三者に共有しないでください。アプリへの完全なアクセス権を与える可能性があります。"
        : "Do not share MCP tokens. They may grant full access to your app.",
    logs: {
      title: lang === "ja" ? "操作履歴" : "Activity Logs",
      action: lang === "ja" ? "アクション" : "Action",
      time: lang === "ja" ? "日時" : "Time",
      reload: lang === "ja" ? "再読み込み" : "Reload",
    },
  },
  settingsNotifications: {
    heading: lang === "ja" ? "通知設定" : "Notification Settings",
    methods: lang === "ja" ? "通知方法" : "Notification methods",
    email: lang === "ja" ? "メール" : "Email",
    webPushLabel: lang === "ja" ? "Webプッシュ通知" : "Web push notifications",
    types: lang === "ja" ? "通知の種類" : "Notification types",
    taskDue: lang === "ja" ? "期限が近い課題" : "Tasks due soon",
    morning: lang === "ja" ? "朝の通知" : "Morning notifications",
    evening: lang === "ja" ? "夕方の通知" : "Evening notifications",
    dayShift: lang === "ja" ? "日直のお知らせ" : "Day shift notifications",
    deviceSettings: lang === "ja" ? "通知端末の設定" : "Notification device settings",
    defaultDevice: "Web Push",
    devicesDescription:
      lang === "ja"
        ? "Web Push通知設定をしている各端末で、通知を受け取るかどうかを設定できます。"
        : "Choose which devices with web push enabled receive notifications.",
    noDevices:
      lang === "ja"
        ? "この端末で一度Webプッシュ通知を有効化すると表示されます。"
        : "Devices will appear here after you enable web push on this device.",
    requirements:
      lang === "ja"
        ? "通知を受け取るには、通知方法と通知の種類の両方が有効である必要があります。"
        : "To receive notifications, enable both a notification method and a notification type.",
    requirementsDescription:
      lang === "ja"
        ? "いずれかが無効の場合、配信されません。また、Webプッシュ通知を有効にするには、ブラウザの通知許可設定で「許可」にする必要があります。"
        : "Notifications are not delivered if either is disabled. To enable web push, allow notifications in your browser's site settings.",
    other: lang === "ja" ? "その他の設定" : "Other settings",
    testLabel: lang === "ja" ? "テスト通知" : "Test notification",
    errorVapidFetch:
      lang === "ja" ? "VAPIDキーの取得に失敗しました" : "Failed to fetch the VAPID key",
    errorVapidEmpty:
      lang === "ja" ? "VAPIDキーが空です" : "The VAPID key is empty",
    emailDesc:
      lang === "ja"
        ? "タスクや日直の更新をメールで受け取ります。"
        : "Receive updates by email.",
    webPush: lang === "ja" ? "Web Push通知" : "Web Push",
    daily:
      lang === "ja"
        ? "毎日のプッシュ通知（朝のダイジェスト）"
        : "Daily push digest",
    serverDefault: lang === "ja" ? "サーバー既定" : "Server default",
    sendTest: lang === "ja" ? "テスト通知を送信" : "Send test notification",
    checkSubs: lang === "ja" ? "購読を確認" : "Check subscriptions",
    enableAndAllow: lang === "ja" ? "許可して有効化" : "Allow & enable",
    blocked:
      lang === "ja"
        ? "通知がブラウザでブロックされています。ブラウザのサイト設定から通知を許可してください。"
        : "Notifications are blocked by the browser. Please allow notifications in site settings.",
    // Messages / Errors
    errorSwUnsupported:
      lang === "ja"
        ? "Service Worker未対応です"
        : "Service Worker not supported",
    errorPushUnsupported:
      lang === "ja" ? "Push通知未対応です" : "Push is not supported",
    errorNotAllowed:
      lang === "ja"
        ? "通知が許可されていません"
        : "Notifications are not allowed",
    errorServerRegister:
      lang === "ja"
        ? "サーバー登録に失敗しました"
        : "Failed to register on server",
    errorTestSend:
      lang === "ja"
        ? "テスト送信に失敗しました"
        : "Failed to send test notification",
    errorFetchSubs:
      lang === "ja"
        ? "購読の取得に失敗しました"
        : "Failed to fetch subscriptions",
    errorGeneric: lang === "ja" ? "エラーが発生しました" : "An error occurred",
    enabledMsg:
      lang === "ja"
        ? "プッシュ通知を有効化しました"
        : "Web push has been enabled",
    disabledMsg:
      lang === "ja"
        ? "プッシュ通知を無効化しました"
        : "Web push has been disabled",
    testSent:
      lang === "ja"
        ? "テスト通知を送信しました（{n}件）"
        : "Test notification sent ({n})",
    testSentZeroHint:
      lang === "ja"
        ? "テスト通知を送信しました（0件）。購読が保存されていない可能性があります。Web Pushを一度OFF→ONして再登録してください。"
        : "Test sent (0). Subscription may not be saved. Toggle Web Push OFF→ON to re-register.",
    subsCount:
      lang === "ja"
        ? "登録済みの購読: {n} 件"
        : "Registered subscriptions: {n}",
  },
  settingsSecurity: {
    heading: lang === "ja" ? "セキュリティ設定" : "Security Settings",
    password: {
      title: lang === "ja" ? "パスワード" : "Password",
      desc:
        lang === "ja"
          ? "パスワードの設定・変更を行います。"
          : "Set or change your password.",
      open: lang === "ja" ? "設定" : "Configure",
      dialogTitle: lang === "ja" ? "パスワードの設定" : "Set password",
      dialogDesc:
        lang === "ja"
          ? "現在のパスワード（設定済みの場合）と新しいパスワードを入力してください。"
          : "Enter current (if set) and new password.",
      current:
        lang === "ja"
          ? "現在のパスワード（任意）"
          : "Current password (optional)",
      next: lang === "ja" ? "新しいパスワード" : "New password",
      update: lang === "ja" ? "更新" : "Update",
      close: lang === "ja" ? "閉じる" : "Close",
    },
    emailLogin: {
      title: lang === "ja" ? "メールコードでのログイン" : "Email code login",
      manage: lang === "ja" ? "設定" : "Configure",
      note:
        lang === "ja"
          ? "ONをオフにするには他の認証手段（パスワード/TOTP/パスキー）が必要です。"
          : "To turn this off, you need another auth method (password/TOTP/passkey).",
      externalNote:
        lang === "ja"
          ? "認証サービスで設定します。オフにするにはパスワードかパスキーが必要です。"
          : "Configure this in the auth service. Turning it off requires a password or passkey.",
    },
    totp: {
      title: "TOTP",
      manage: lang === "ja" ? "設定" : "Configure",
      enabled: lang === "ja" ? "有効になっています" : "Enabled",
      disable: lang === "ja" ? "無効化" : "Disable",
      disabled: lang === "ja" ? "TOTPを無効化しました" : "TOTP disabled",
      disableConfirmTitle:
        lang === "ja" ? "TOTPを無効化しますか？" : "Disable TOTP?",
      disableConfirmDesc:
        lang === "ja"
          ? "認証アプリのコードをログインに使用できなくなります。"
          : "Authenticator app codes will no longer be available for sign-in.",
      begin: lang === "ja" ? "セットアップを開始" : "Begin setup",
      setupTitle: lang === "ja" ? "二段階認証をセットアップ" : "Set up 2FA",
      setupDesc:
        lang === "ja"
          ? "認証アプリでQRコードを読み取り、次へ進みます。"
          : "Scan QR code with your authenticator app, then continue.",
      secret: lang === "ja" ? "シークレット" : "Secret",
      next: lang === "ja" ? "次へ" : "Next",
      back: lang === "ja" ? "戻る" : "Back",
      code: lang === "ja" ? "認証コード" : "Verification code",
      activate: lang === "ja" ? "有効化" : "Activate",
      codeInvalid: lang === "ja" ? "コードが正しくありません" : "Invalid code",
    },
    passkey: {
      title: lang === "ja" ? "パスキー（Passkey）" : "Passkey",
      desc:
        lang === "ja"
          ? "この端末のパスキーを登録します。"
          : "Register a passkey on this device.",
      register: lang === "ja" ? "パスキーを登録" : "Register passkey",
      registered:
        lang === "ja" ? "パスキーを登録しました" : "Passkey registered",
      manage: lang === "ja" ? "パスキーを管理" : "Manage passkeys",
      addThisDevice:
        lang === "ja"
          ? "この端末のパスキーを追加"
          : "Add passkey for this device",
      name: lang === "ja" ? "名称" : "Name",
      rename: lang === "ja" ? "名称を変更" : "Rename",
      delete: lang === "ja" ? "削除" : "Delete",
      none:
        lang === "ja"
          ? "登録済みのパスキーはありません"
          : "No passkeys registered",
    },
    recovery: {
      title: lang === "ja" ? "リカバリーコード" : "Recovery codes",
      desc:
        lang === "ja"
          ? "新しいコードを発行すると、以前の未使用コードは無効になります。"
          : "Issuing new codes invalidates previous unused codes.",
      reissueConfirmTitle:
        lang === "ja" ? "リカバリーコードを再発行しますか？" : "Reissue recovery codes?",
      reissueConfirmDesc:
        lang === "ja"
          ? "現在の未使用コードはすべて無効になります。"
          : "All current unused recovery codes will become invalid.",
      generate:
        lang === "ja" ? "リカバリーコードを発行" : "Generate recovery codes",
      reissue:
        lang === "ja" ? "リカバリーコードを再発行" : "Re-issue recovery codes",
      dialogTitle: lang === "ja" ? "リカバリーコード" : "Recovery codes",
      dialogDesc:
        lang === "ja"
          ? "二度と表示できません。必ず記録・保管してください。"
          : "They will not be shown again. Store them securely.",
    },
  },
});
