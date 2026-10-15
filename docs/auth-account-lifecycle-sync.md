# 認証サービスとの退会・消去同期

## 対象

認証 DB 分離後も、REST `/users/me`、GraphQL `deleteMe`、管理者の soft delete / hard delete、日次 `account_erasure` から生じる利用者状態を認証サービスへ反映する。利用者 ID は既存値を維持し、アプリ DB のプロフィール・同意・業務データとの参照を変えない。

## 退会フロー

1. Taskly API はアプリ DB の `users.is_deleted=1` と `deleted_at`、session / MCP / iCal / API key の失効を一つのトランザクションで完了する。
2. 同じ UTC の `deleted_at` を `PUT {AUTH_LIFECYCLE_SYNC_URL}/internal/users/{id}/lifecycle` へ送る。
3. 認証サービスは退会状態を保存し、認証 session、challenge、OAuth code / token と移行済み旧 session を失効する。
4. 同期失敗時もアプリ DB の退会を戻さず、API は「認証サービスとの同期保留」を返す。OAuth callback はアプリ DB の退会利用者を拒否する。
5. 運用担当者は同じ利用者 ID と `deleted_at` を再送する。認証側は同じ要求を冪等に処理し、既存の退会日時を後ろへ延ばさない。

## 管理者による復元

管理画面は `is_deleted` を表示し、削除済みユーザーに「復元」を提供する。`adminUserRestore` はユーザー管理と同じ admin / manager 権限を確認し、監査ログに `admin.user.restore:{id}` を記録する。

アプリの利用者行をロックして消去処理と直列化し、残存するセッション・APIキー・iCalキー・MCP資格情報を失効する。認証 API に `{"deleted":false}` を送信し、成功後にアプリの `is_deleted=0, deleted_at=NULL` を保存する。認証サービスは同じ利用者 ID を維持してセッションとトークンを再失効する。ブロック設定は維持し、利用者は再ログインする。物理削除済みのユーザーは復元できない。

同期失敗時はアプリのトランザクションを戻し、削除済み表示を維持する。認証側の成功後にアプリ側の更新が失敗しても、アプリへのログインは拒否され、管理者は復元を再試行できる。導入は復元に対応した taskly-auth を先に反映する。

## 期限後の消去フロー

日次 Airflow `account_erasure` は従来どおりアプリ DB の `deleted_at`、30日保持、`account_erasure_holds` を判定する。対象ごとのアプリ DB トランザクション内で、最初に `DELETE {AUTH_LIFECYCLE_SYNC_URL}/internal/users/{id}` を呼ぶ。

- 認証消去成功後に、アプリ側の関連行と `users` を削除して commit する。
- 認証消去失敗時はアプリ DB を rollback し、DAG を失敗させる。
- 認証消去後にアプリ DB が失敗した場合、退会行が残る。次回の認証 DELETE は「すでに存在しない」を成功として扱い、アプリ消去を再実行する。
- `AUTH_ACCOUNT_LIFECYCLE_EXTERNAL=0` の開発環境では外部呼び出しを行わず、既存の単一 DB 消去を維持する。

管理者の即時 hard delete は、フラグ有効時にまずアプリ側を soft delete して認証停止を同期し、次に認証 DB、最後にアプリ DB を物理削除する。途中で失敗してもアカウントを有効状態へ戻さない。すでにアプリ側だけが退会済みなら、認証側へ退会状態を再送してから消去を再試行する。通常運用では30日保持と保留判定のある soft delete を使用する。

## 設定

Taskly API と Airflow の両方へ次を設定する。

```dotenv
AUTH_ACCOUNT_LIFECYCLE_EXTERNAL=0
AUTH_LIFECYCLE_SYNC_URL=http://auth:3070
AUTH_INTERNAL_TOKEN=
```

- production でフラグを `1` にする場合、URL は HTTPS または private network 上の HTTP、token は32文字以上とする。
- redirect は追従せず、応答本文をログへ出さない。
- `AUTH_INTERNAL_TOKEN` は OAuth client secret と分け、Taskly API、Airflow、認証サービスだけに渡す。
- 最終コピー前は必ず `0` のままにする。認証 DB に対象利用者が存在しない段階では有効化しない。

## エラーと通知

- Taskly API は同期失敗を利用者向けの汎用エラーに変換し、詳細をエラー監視へ送る。メール、token、応答本文を含めない。
- Airflow は例外で task を失敗させ、既存の Discord 失敗通知を使う。通知には利用者 ID や個人データを含めない。
- 認証サービスの 5xx は同サービスの汎用 Discord アラートへ送る。

## 検証

1. REST、GraphQL、管理者 soft delete が同じ `deleted_at` を送る。
2. 通信失敗時にもアプリ session が失効し、利用者がログインできない。
3. 認証 API の再送で退会日時が延びない。
4. Airflow が期限前と保留中を消去しない。
5. 認証消去失敗時はアプリ行を維持し、成功後だけアプリ行を削除する。
6. 認証消去済み・アプリ消去失敗の状態から再実行して完走する。
7. バックアップ復元後の退会・消去再適用を両 DB で確認する。
