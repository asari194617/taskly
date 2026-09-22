# 認証データ書き込み経路の棚卸し

Status: cutover blocker inventory (2026-09-21)

この文書は Taskly API が現在アプリ DB に書き込む認証関連データを列挙する。`taskly-auth` の本番 DB へのスナップショットコピーだけでは、その後の書き込みが同期されない。新しいログインを有効にする前に、各経路の正本と同期方法を実装・検証する。

| データ | 現行の書き込み経路 | 切り替え前に必要な対応 |
| --- | --- | --- |
| `users.hashed_password`, `email_code_allowed`, `is_registered` | GraphQL `setPassword`, `setAuthSettings`; REST `/settings`; 旧ログイン成功時 | credential とログイン許可設定は auth DB で更新。旧ログインを併用する期間は両経路の整合性を保証する。 |
| `users.email` | GraphQL `adminUserUpdate`, `adminUserCreate`, `adminUserBulkCreate` | メールの一意性とユーザー ID の採番を auth 側で確定し、同じ ID の app 側 shadow record を作成・更新する。片方だけ成功した場合の補償と再試行を用意する。 |
| `users.is_deleted` / hard delete | REST `/users/me`; GraphQL `deleteMe`, `adminUserDelete` | auth 側のログイン禁止・トークン失効と app 側の表示/参照を連動させる。管理者の hard delete は app 側の参照を壊し得るため、データ保持と整合性を設計するまで切り替えない。 |
| `users.display_name`, `notification_lang`, `is_agreed_policies` | REST `/users/me`, `/settings`; GraphQL `updateMe`, `agreePolicies`, `adminUserUpdate` | app 側プロフィールを正本とする場合も、auth UI が必要とする値の読み取りまたは同期を用意する。 |
| `login_codes` | REST 旧 `/auth` の発行、使用、試行回数・ロック更新 | 新ログインの発行と検証は auth DB に一本化。旧ログインを残す間は旧 DB のコードを新サービスが誤って受け付けないようにする。 |
| `totp_secrets`, `recovery_codes` | REST `/2fa`; GraphQL TOTP 有効化/無効化、復旧コード再生成; `utils.ConsumeRecoveryCode` | 設定・再生成・使用を auth DB に一本化し、旧認証経路との併存期間中の二重使用を防ぐ。 |
| `passkeys` | REST `/webauthn` 登録・認証回数更新; GraphQL 登録・名前変更・削除 | 新 UI を旧 RP ID `taskly.iy0306.com` のサブドメインに置き、同じ RP ID で既存クレデンシャルを検証する。両 DB の署名カウンタ・名前・削除を整合させる。 |
| `sessions` | `handler/session.go` 発行、`utils.RevokeSession` / `RevokeOtherSessions` 削除、GraphQL logout | 旧セッションの期限まで旧 DB を検証する。DB 行を毎回正本として照合し、Redis に失効済みのキャッシュが残っても再認証を拒否する。OAuth トークンとの一括失効と切り戻し時の扱いを検証する。 |
| `blocks` | REST `/admin/users`、GraphQL `adminUserBlock` / `adminUserUnblock` | app API と auth ログインの両方で同一のブロック状態を判定し、更新を同期する。 |

`role_users`、`permissions`、`api_keys` と通知・タスク等の業務データは app DB に残る。auth DB の `users.id` を変えず、app DB の shadow record を維持する。auth サービスから app DB へ直接広い権限を与える設計にはしない。

## 切り替え判定

1. 上表の書き込みを auth API または明示した同期経路へ移す。各更新の失敗・再試行・競合を試験する。
2. Taskly API の全インスタンスで `AUTH_WRITES_PAUSED=1` の保守ゲートを有効にし、別プロセスと外部 DB クライアントの書き込みも停止してから最終コピー・件数/行単位の照合を行う。以後の旧 API 経路が auth DB と食い違わないことを確認する。
3. 新規ユーザー作成、メール変更、パスワード変更、TOTP・復旧コード、パスキー、セッション失効、ブロック、削除を両サービスから確認する。
4. 切り戻し後に auth DB で発生した更新を旧 DB に戻せることを確認する。これができるまで旧ログインを止めない。
5. 現行の Airflow バックアップはアプリ DB のみを対象にする。認証 DB の独立した暗号化定期バックアップと空DBの隔離環境への SQL 復元は確認済み。実ユーザー移行後に同じ復元・照合を再試験する。

2026-09-21 の読み取り専用スナップショット移行リハーサルは別 DB で成功したが、実際の auth DB はまだ空であり、上記の書き込み移管は未実装である。したがって本番 OAuth ログインを有効化しない。

2026-09-22 に旧 API のセッション検証を DB 正本に変更し、DB 行が消えた後に Redis キャッシュだけが残る場合の拒否を回帰テストで確認した。GraphQL の `setPassword` はパスワード更新と他端末の永続セッション削除を同一トランザクションに変更し、削除失敗時のロールバックと成功時の失効をテストした。認証情報の書き込み先移管は引き続き未完了である。

同日の本番 DB 読み取り専用集計では、有効ユーザー46件、パスキー保有者1件、そのうちパスキー以外の現行認証手段を持たないユーザー0件だった。新 UI の origin から旧 RP ID で実際に認証できることを確認するまで旧ログインを維持し、移行直前にも再集計する。
