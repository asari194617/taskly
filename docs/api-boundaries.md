# API の使い分け

| 用途 | インターフェース | 認証 |
| --- | --- | --- |
| Web と将来のアプリ画面で扱う課題、ユーザー設定、管理画面の通常操作 | GraphQL | セッション Cookie |
| 外部アプリからの公開課題 API | REST `/tasks`, `/users/me` | `X-API-Key` |
| 現行 Web ログイン | API の REST `/auth/oauth/start`・`/auth/oauth/callback` と taskly-auth | OAuth state / PKCE、API のセッション Cookie |
| 認証方法・プロフィール・セキュリティ設定 | taskly-auth の画面と API | 認証サービスのセッション |
| 旧ログイン、2FA、パスキーの開始・完了 | REST（所有権フラグで制限） | 手順に応じた Cookie / challenge |
| Web Push の購読・解除、VAPID 公開鍵、通知端末管理 | REST `/push` | セッション Cookie（一部公開） |
| iCal フィード、Webhook、Teams とメールの受信 | REST | 専用トークンまたは署名 |
| ヘルスチェック | REST | なし |
| MCP ツール | MCP サーバーの REST | MCP トークン |

認証サービスは別リポジトリで管理し、このリポジトリの Compose には含めません。
API は認証後のセッションを管理し、ユーザー投影を同期します。
[ローカル設定](local-development.md)、[所有権ゲート](auth-identity-write-inventory.md)、
[プロフィール同期](account-profile-ownership.md)を参照してください。

## 現行 Web の例外

- 通知設定 (`/settings/notification-methods`, `/settings/notification-types`) とモデレーション画面 (`/admin/moderation`) は既存の REST を使用する。機能を変更する際に GraphQL への移行を判断する。
- Web Push のブラウザ購読はブラウザ API の結果をそのまま送る手順であり、REST を使う。
- REST の認証失敗は HTTP 401。GraphQL もセッション欠落を HTTP 401 として返す。Web はエラー文言を認証判定に使わない。

## 課題操作の契約

- Go サーバーの REST と GraphQL は同じ一覧処理を使う。通常ユーザーに非表示課題は返さず、admin / manager は取得できる。更新・削除にも同じ閲覧条件と権限判定を適用する。
- 一覧の標準件数は20、最大200。`cursor_deadline` は `YYYY-MM-DD`、`cursor_id` は正の整数文字列。日付カーソルを指定する場合は ID も必要。
- 通常一覧（`q` なし、`overdue=false`）は期限が今日以降の課題を返す。空白だけの `q` も通常一覧と同じ。検索語がある場合はタイトルまたは説明に一致する課題を期限日に関係なく返し、`effective_deadline ASC, id ASC` で並べる。`next_cursor` の期限日と ID を次ページへ渡す。明示的な `overdue=true` は期限切れだけを期限日・ID の降順で返す。どの場合も閲覧権限と非表示条件を先に適用する。
- MCP は既存の所有者限定インターフェースで、一覧は ID 降順の offset/limit ページング（最大500件）を使う。非表示課題は返さず、更新・削除も所有者の課題に限定する。この差異は MCP トークンの権限を狭く保ち、既存 MCP クライアントのページング契約を維持するための意図的なもの。
- MCP の課題操作は MCP トークンを Go サーバーの `/tasks` に転送する。Go サーバー側が所有者スコープ、権限、入力検証、課題通知を担当する。MCP の本人情報読取は DB 経路を使い、更新は Go API の `PATCH /users/me` に転送する。`AUTH_PROFILE_EXTERNAL=1` の環境では表示名の更新を拒否し、通知言語は引き続き変更できる。
