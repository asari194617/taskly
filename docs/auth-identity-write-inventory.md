# 認証データ書き込み経路の棚卸し

Status: cutover blocker inventory (2026-09-23)

この文書は Taskly API が現在アプリ DB に書き込む認証関連データを列挙する。`taskly-auth` の本番 DB へのスナップショットコピーだけでは、その後の書き込みが同期されない。新しいログインを有効にする前に、各経路の正本と同期方法を実装・検証する。

| データ | 現行の書き込み経路 | 切り替え前に必要な対応 |
| --- | --- | --- |
| `users.hashed_password`, `email_code_allowed`, `is_registered` | GraphQL `setPassword`, `setAuthSettings`; REST `/settings`; 旧ログイン成功時 | credential とログイン許可設定は auth DB で更新。旧ログインを併用する期間は両経路の整合性を保証する。 |
| `users.email` | GraphQL `adminUserUpdate`, `adminUserCreate`, `adminUserBulkCreate` | メールの一意性とユーザー ID の採番を auth 側で確定し、同じ ID の app 側 shadow record を作成・更新する。片方だけ成功した場合の補償と再試行を用意する。 |
| `users.is_deleted`, `deleted_at` / 退会後の消去 | REST `/users/me`; GraphQL `deleteMe`, `adminUserDelete`; Airflow `account_erasure` | [退会・消去同期](auth-account-lifecycle-sync.md)に従い、退会後に auth 側のログインとトークンを失効する。期限後は auth DB を先に消去し、再実行可能なまま app DB を消去する。`deleted_at` を両 DB に保持し、保留・復元時の再消去も整合させる。 |
| `policy_consents` | REST `/consent`; GraphQL `agreePolicies`; app API の認可ゲート | 同意イベントは app DB に保持し、OAuth から作成した app session でも現行版への同意を確認する。退会後の保存期間と `users` への外部キーを、app 側の消去処理および auth 側のユーザー削除と整合させる。 |
| `users.display_name`, `notification_lang`, `is_agreed_policies` | REST `/users/me`, `/settings`; GraphQL `updateMe`, `agreePolicies`, `adminUserUpdate` | app 側プロフィールを正本とする場合も、auth UI が必要とする値の読み取りまたは同期を用意する。 |
| `login_codes` | REST 旧 `/auth` の発行、使用、試行回数・ロック更新 | 新ログインの発行と検証は auth DB に一本化。旧ログインを残す間は旧 DB のコードを新サービスが誤って受け付けないようにする。 |
| `totp_secrets`, `recovery_codes` | REST `/2fa`; GraphQL TOTP 有効化/無効化、復旧コード再生成; `utils.ConsumeRecoveryCode` | 設定・再生成・使用を auth DB に一本化し、旧認証経路との併存期間中の二重使用を防ぐ。 |
| `passkeys` | REST `/webauthn` 登録・認証回数更新; GraphQL 登録・名前変更・削除 | 新 UI を旧 RP ID `taskly.iy0306.com` のサブドメインに置き、同じ RP ID で既存クレデンシャルを検証する。両 DB の署名カウンタ・名前・削除を整合させる。 |
| `sessions` | `handler/session.go` 発行、`utils.RevokeSession` / `RevokeOtherSessions` 削除、GraphQL logout | 旧セッションの期限まで旧 DB を検証する。DB 行を毎回正本として照合し、Redis に失効済みのキャッシュが残っても再認証を拒否する。OAuth トークンとの一括失効と切り戻し時の扱いを検証する。 |
| `blocks` | REST `/admin/users`、GraphQL `adminUserBlock` / `adminUserUnblock` | app API と auth ログインの両方で同一のブロック状態を判定し、更新を同期する。 |

`role_users`、`permissions`、`api_keys` と通知・タスク等の業務データは app DB に残る。auth DB の `users.id` を変えず、app DB の shadow record を維持する。auth サービスから app DB へ直接広い権限を与える設計にはしない。

2026-09-23 に main へ追加された `deleted_at` は、既存の移行ツールが `SELECT *` で全列をコピーするため認証 DB にも必要となる。すでに作成済みの空の認証 DB には、列の有無と順序を確認してからスキーマを適用する。`policy_consents` はアプリ側の同意履歴であり認証 DB へ単純コピーしない。Airflow の消去処理が app DB だけを削除する現状では auth DB の認証行が残るため、消去連携と復元試験を切り替え条件に追加する。

同日の本番読み取り専用確認では、`taskly.users` 47行と `taskly_auth.users` 0行のどちらにも `deleted_at` はまだない。Taskly 側の本番 DB migration を先に適用・確認してから、空の認証 DB に列を追加する。この順序が揃うまで認証データコピーを再実行しない。

新認証サービスのパスキー管理画面が利用可能になった切り替え時には、Web ビルドの `PUBLIC_AUTH_PASSKEY_URL=https://auth.taskly.iy0306.com/passkeys` を設定し、設定画面から同画面へ移す。このフラグが空の間は従来の管理 UI を維持する。フラグを有効化しても旧 API の GraphQL/REST 書き込み口は残るため、全クライアントとバックグラウンド経路を含めた書き込み所有権の移管が終わるまで最終コピー・公開切り替えをしない。

パスワード管理画面を切り替える Web ビルド設定は `PUBLIC_AUTH_PASSWORD_URL=https://auth.taskly.iy0306.com/security`。空なら旧設定ダイアログを維持する。新画面では直近10分の認証と既存パスワードの照合後に更新し、auth セッションと OAuth トークンを失効させる。旧 GraphQL `setPassword` と REST 設定経路は別途停止する必要があり、このリンクだけで切り替え可能にはならない。

`AUTH_PASSWORD_EXTERNAL=1` は旧 GraphQL `setPassword` を拒否する。既定値は `0` で、旧ログインを使う間は有効化しない。REST 設定はパスワード自体を更新しないが、メールコード許可を旧 DB へ書くため別の移管が必要。

`AUTH_BLOCKS_EXTERNAL=1` は管理者のブロック・解除を app DB へ確定した後、認証サービスの internal API へ同じ状態を同期する。ブロック同期の失敗時は app 側が先に拒否し、解除同期の失敗時は auth 側が拒否を継続するため、通信障害でアクセスを広げない。同期 API は冪等とし、ブロック時に auth session と OAuth token 系列を失効する。`AUTH_BLOCK_SYNC_URL` と共有 token が揃い、最終コピー後の再送試験が成功するまではフラグを有効にしない。

メールコード許可設定の本人操作は、認証サービスの `/security` から auth DB の `users.email_code_allowed` を更新する。Web の `PUBLIC_AUTH_EMAIL_CODE_URL` は旧トグルを新画面のリンクへ変え、API の `AUTH_EMAIL_SETTINGS_EXTERNAL=1` は旧 GraphQL `setAuthSettings` と REST `POST /settings/auth` を拒否する。管理者更新と旧ログインのコード使用は別の経路であり、このフラグだけでは所有権を移せない。

復旧コードの再発行は認証サービスの `/security` から auth DB で行う。Web の `PUBLIC_AUTH_RECOVERY_URL` は旧再発行ボタンを新画面へのリンクへ変え、API の `AUTH_RECOVERY_EXTERNAL=1` は旧 GraphQL `recoveryGenerate`、REST `/2fa/recovery/generate`、旧 `/auth/login` の復旧コード利用を拒否し、旧方法一覧と MFA 選択肢から復旧コードを外す。フラグは最終コピー後にだけ有効にし、旧 DB と新 DB の所有権を同時に切り替える。

TOTP の設定・有効化・コード確認付き無効化は認証サービスの `/security` から auth DB で行う。既存の `totp_secrets.secret` は同じ `TOTP_ENC_KEY` と暗号形式のままコピーする。Web の `PUBLIC_AUTH_TOTP_URL=https://auth.taskly.iy0306.com/security` は旧設定 UI を新画面へのリンクへ変え、API の `AUTH_TOTP_EXTERNAL=1` は旧 GraphQL `totpSetup` / `totpActivate` / `totpDisable`、REST `/2fa/totp/*`、旧 `/auth/login` の TOTP 利用を拒否し、旧方法一覧と MFA 選択肢から TOTP を外す。フラグは最終コピー後にだけ有効にする。

切り替え時の API 側ゲート `AUTH_PASSKEYS_EXTERNAL=1` は旧 `/webauthn/*` と GraphQL のパスキー登録・名前変更・削除を拒否する。既定値は `0` とし、旧ログインを使う間は有効化しない。このゲートはパスキー以外の認証情報更新や直接 DB 書き込みを止めないため、全認証データの切り替え承認には使わない。

## 切り替え判定

1. 上表の書き込みを auth API または明示した同期経路へ移す。各更新の失敗・再試行・競合を試験する。
2. Taskly API の全インスタンスで `AUTH_WRITES_PAUSED=1` の保守ゲートを有効にし、別プロセスと外部 DB クライアントの書き込みも停止してから最終コピー・件数/行単位の照合を行う。以後の旧 API 経路が auth DB と食い違わないことを確認する。
3. 新規ユーザー作成、メール変更、パスワード変更、TOTP・復旧コード、パスキー、セッション失効、ブロック、削除を両サービスから確認する。
4. 切り戻し後に auth DB で発生した更新を旧 DB に戻せることを確認する。これができるまで旧ログインを止めない。
5. 現行の Airflow バックアップはアプリ DB のみを対象にする。認証 DB の独立した暗号化定期バックアップと空DBの隔離環境への SQL 復元は確認済み。実ユーザー移行後に同じ復元・照合を再試験する。

2026-09-21 の読み取り専用スナップショット移行リハーサルは別 DB で成功したが、実際の auth DB はまだ空であり、上記の書き込み移管は未実装である。したがって本番 OAuth ログインを有効化しない。

2026-09-22 に旧 API のセッション検証を DB 正本に変更し、DB 行が消えた後に Redis キャッシュだけが残る場合の拒否を回帰テストで確認した。GraphQL の `setPassword` はパスワード更新と他端末の永続セッション削除を同一トランザクションに変更し、削除失敗時のロールバックと成功時の失効をテストした。認証情報の書き込み先移管は引き続き未完了である。

同日の本番 DB 読み取り専用集計では、有効ユーザー46件、パスキー保有者1件、そのうちパスキー以外の現行認証手段を持たないユーザー0件だった。新 UI の origin から旧 RP ID で実際に認証できることを確認するまで旧ログインを維持し、移行直前にも再集計する。
