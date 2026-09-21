# API の使い分け

| 用途 | インターフェース | 認証 |
| --- | --- | --- |
| Web と将来のアプリ画面で扱う課題、ユーザー設定、管理画面の通常操作 | GraphQL | セッション Cookie |
| 外部アプリからの公開課題 API | REST `/tasks`, `/users/me` | `X-API-Key` |
| ログイン、2FA、パスキーの開始・完了、ログアウト | REST | 手順に応じた Cookie / challenge |
| Web Push の購読・解除、VAPID 公開鍵、通知端末管理 | REST `/push` | セッション Cookie（一部公開） |
| iCal フィード、Webhook、Teams とメールの受信 | REST | 専用トークンまたは署名 |
| ヘルスチェック | REST | なし |
| MCP ツール | MCP サーバーの REST | MCP トークン |

## 現行 Web の例外

- 通知設定 (`/settings/notification-methods`, `/settings/notification-types`) とモデレーション画面 (`/admin/moderation`) は既存の REST を使用する。機能を変更する際に GraphQL への移行を判断する。
- Web Push のブラウザ購読はブラウザ API の結果をそのまま送る手順であり、REST を使う。
- REST の認証失敗は HTTP 401。GraphQL もセッション欠落を HTTP 401 として返す。Web はエラー文言を認証判定に使わない。

## 課題操作の契約

- Go サーバーの REST と GraphQL は同じ一覧処理を使う。通常ユーザーに非表示課題は返さず、admin / manager は取得できる。更新・削除にも同じ閲覧条件と権限判定を適用する。
- 一覧の標準件数は20、最大200。`cursor_deadline` は `YYYY-MM-DD`、`cursor_id` は正の整数文字列。日付カーソルを指定する場合は ID も必要。
- MCP は既存の所有者限定インターフェースで、一覧は ID 降順の offset/limit ページング（最大500件）を使う。非表示課題は返さず、更新・削除も所有者の課題に限定する。この差異は MCP トークンの権限を狭く保ち、既存 MCP クライアントのページング契約を維持するための意図的なもの。
- MCP の課題操作は MCP トークンを Go サーバーの `/tasks` に転送する。Go サーバー側が所有者スコープ、権限、入力検証、課題通知を担当する。MCP サーバーの本人情報操作は既存の DB 経路を使う。
