# Issue #401: 個人情報の取扱い棚卸し（2026-09-23）

公開文面の改訂に使う技術面の根拠。リポジトリで確認できた機能と、本番設定・契約で確認すべき事項を分ける。個々の機能が本番で実際に送信しているかどうかは、この表だけでは確定しない。

本番で有効な外部連携と委託先は、秘密値・ホスト名・コンテナ名を含まない非公開の運用台帳で確認する。この公開資料はコード上で利用可能な経路と、利用者への説明に必要なデータ種別だけを示し、特定環境で各連携が有効かどうかは示さない。

| 経路 | 取得・保存・送信する情報 | 主な目的・相手先 | 根拠 |
| --- | --- | --- | --- |
| 登録・認証 | メールアドレス、表示名、パスワードのハッシュ、ログインコードのハッシュ、セッション、パスキー公開鍵、TOTP シークレット、復旧コードのハッシュ、言語・同意状態 | アカウント管理、認証、不正利用防止。ログインコードは SMTP 事業者を経由 | `apps/server/migrations/0001_init.sql`, `0007_auth_users_extensions.sql`, `apps/server/internal/handler/auth.go` |
| 課題・日直 | 課題のタイトル・説明・添付画像・期限・作成者・完了者、日直の担当者 | 課題管理、共有、通知。添付画像は非公開の Cloudflare R2 に保存し、短寿命の署名付きURLで送受信 | `apps/server/migrations/0001_init.sql`, `0002_core.sql`, `0004_day_shift.sql`, `0032_task_images.sql`, `docs/features/task-images.md` |
| 通知 | メールアドレス、通知設定、メール本文、Web Push 購読 endpoint と鍵、通知履歴 | SMTP 事業者、ブラウザーの Push サービスへの配信 | `apps/server/migrations/0007_manual_sync.sql`, `0009_notification_devices.sql`, `apps/server/internal/handler/push_vapid.go` |
| Teams・Discord | Teams 由来のメールアドレス、課題内容、メッセージ ID、callback URL。Discord には課題タイトル・期限・操作した表示名等を送る | Teams/Power Automate 連携、Discord 通知 | `apps/server/internal/handler/integration_teams.go`, `apps/server/internal/utils/support_discord.go`, `docs/integration-jobs.md` |
| メール受信 | Cloudflare Email Worker からの生メール、送受信元、件名、添付情報。Sentry に送受信元と件名、Discord に受信・配送停止の情報を送る経路あり | 配信エラー・DMARC 等の運用 | `cloudflare-email-workers/README.md`, `apps/server/internal/handler/integration_email.go` |
| モデレーション・日直判定 | 課題のタイトル・説明、日直関連メールの件名・本文。ユーザーの通報は別途保存 | 設定時に課題・メールの内容を Google Gemini へ送信し判定。判定結果・課題のスナップショットを保存 | `batches/moderation/main.go`, `apps/server/internal/utils/support_ai.go`, `apps/server/migrations/0016_moderation_flags.sql` |
| 利用・障害ログ | ユーザー ID、操作、API 利用時の IP、エラー・トレース | 運用、セキュリティ、障害解析。設定時に Sentry・Datadog へ送信 | `apps/server/migrations/0002_core.sql`, `apps/server/infra.go`, `apps/server/run.go`, `infra/docker-compose.prod.yml` |
| バックアップ | アプリ DB と Airflow メタ DB のデータ | age で暗号化して FTPS 保存先へ転送。リモートは原則14日・最低3世代 | `docs/operations/mysql-backup.md`, `batches/backup-mysql/main.go` |
| API・予定表連携 | API/MCP トークンのハッシュ、iCal 購読トークン | 利用者が設定した外部クライアントからのアクセス | `apps/server/migrations/0001_init.sql`, `0002_core.sql`, `0007_manual_sync.sql` |

## 改訂前の公開文面との相違

- 改訂前の第2条にあった生年月日、住所、電話番号、銀行口座、カード番号、運転免許証番号、広告・決済提携先からの収集は、登録スキーマ・画面で確認できなかった。自由入力の課題本文等に利用者が記入する可能性まで否定するものではない。
- 第3条のキャンペーン案内、有料サービスの請求は、該当する実装・運用を確認できない。
- 第6条の開示手数料1,000円は、徴収・決済・支払案内の運用を確認できない。
- 第7条の「当サービスが定める手続き」は具体的な申請方法を示していない。
- 開示等の窓口、運営者の正式名称・住所、法人の場合の代表者氏名、安全管理措置の説明が不足している。

## 公開後の運用確認事項

1. 個人運営者の正式な氏名・住所を非公開の運用記録で確定し、請求者へ遅滞なく回答できる状態を維持する。公開ページには運営者の希望により直接掲載しない。
2. 開示等・苦情の受付メールアドレスの管理者と対応手順。現行文面の連絡先が実際に使えることを確認する。
3. 本番の SMTP、Cloudflare、Teams/Power Automate、Discord、Google Gemini、Sentry、Datadog、Web Push、FTPS の実際の送信経路と各事業者・処理地域・契約条件。秘密値を記録せず、有効/無効と委託先のみ記録する。
4. バックアップとログの実際の保持期間、退会後の削除範囲、法令等による保存例外。
5. 外国にある第三者への提供・外国での委託に該当する経路、その説明内容と必要な措置。
6. 代理人申請、本人確認、希望する回答方法への対応、無料での開示等を公開文面どおりに運用する。
7. 法務レビューは個人運営者本人が承認済み。[改訂文面](privacy-policy.md)を2026-09-23に公開 Notion ページへ反映した。Chrome で[公開ページ](https://ritzy-lift-e38.notion.site/28ba3874f8c88073a4b2c0a56b82bc03)の改訂日・無料の開示手続・正しいメールリンクを確認した。本番の `/consent/` 画面の本文リンクとフッターリンクが同じ公開ページを指すことも確認した。

## 法令参照

[個人情報保護委員会・通則ガイドライン 3-8-1、3-8-7、3-8-8](https://www.ppc.go.jp/personalinfo/legal/guidelines_tsusoku/)を2026-09-23に確認。これは実装の棚卸しであり、法的評価や本番設定の証明ではない。
