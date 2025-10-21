# 本番環境でのスキーマ反映手順（created_at/updated_at の DB デフォルト標準化）

このドキュメントは、CI/CD だけでは反映が難しい場合に、本番の MySQL に安全にスキーマ変更（timestamp の DEFAULT/ON UPDATE 標準化）を適用するための手順をまとめたものです。Taskly サーバは起動時にアプリ内マイグレーションを実行するため、基本は「新バージョンのサーバを起動する」ことで反映されます。ゼロダウンタイムでの切替や、事前に DB へだけ適用したいケースのために代替手順も用意しています。

## 変更の概要

- 目的: DB レベルで `created_at` と `updated_at` を自動設定/更新する
  - `created_at`: `DEFAULT CURRENT_TIMESTAMP`（NOT NULL）
  - `updated_at`: `DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`（該当テーブル）
- 主な対象テーブル: `sessions`, `api_keys`, `permissions`, `blocks`, `tasks`, `passkeys`, `day_shift_assignments`, `business_calendar`, `web_push_subscriptions`（created_at/updated_at 両方）、`role_users`（created_at）、`subjects`（created_at）、`migrations`（applied_at）
- 不正値対策: 変更前に NULL や `0000-00-00 00:00:00` を安全値へ置換するサニタイズ処理を実行（セッション単位で sql_mode を一時的に緩和）
- 影響範囲: 既存アプリ動作は維持。SQLite（テスト環境）は GORM の `autoUpdateTime` で互換を確保。

## 運用方針

原則は「新サーバを起動 → 起動時にマイグレーション実行 → 正常起動を確認」のローリング更新で OK です。単一インスタンス運用で短時間の停止が許容される場合はシンプルに切替します。事前に DB へだけ反映したい場合は「手動 SQL」手順を使えます。

## 事前チェックリスト（必須）

- [ ] メンテナンス時間・方法の合意（単一/複数インスタンスによって方針を決定）
- [ ] 本番 DB のバックアップ取得（直近フル + binlog/ポイントインタイム復旧が望ましい）
- [ ] ステージングでの検証（DDL/アプリ起動ログ/主要機能）
- [ ] 接続情報と権限の確認（MySQL 8.x, `ALTER`/`UPDATE`/`SET SESSION` 権限）
- [ ] リリースアーティファクト（新イメージ or バイナリ）を準備

## 推奨手順 A: ローリングデプロイで自動適用

複数インスタンスで LB 配下の場合に推奨。新バージョンのサーバ起動時にアプリ内マイグレーション（idempotent）が実行されます。

1. 事前にステージングで新バージョンを検証
2. 本番 LB のターゲットを 1 台ずつ drain（停止）し、新イメージへ更新して起動
3. 該当インスタンスの起動ログで以下を確認（エラーがないこと）
   - `Running migration: 0012_timestamp_sanitize` / `0011_timestamp_defaults` / `0013_role_users_timestamp_fix` / `0014_subjects_timestamp_fix` / `0015_migrations_applied_at_fix`
   - `Migration <name> applied successfully`
4. ヘルスチェック OK を待ち、LB へ復帰
5. 全台に対して順次繰り返す
6. 反映後、検証用コマンドで DDL を確認（後述）

### Docker Compose の例（単一 or 少数台）

```bash
# 新イメージへ更新して起動
docker compose -f infra/docker-compose.yml pull server
docker compose -f infra/docker-compose.yml up -d server

# 起動ログでマイグレーション適用を確認
docker compose -f infra/docker-compose.yml logs server --tail=200
```

### Kubernetes の例

```bash
# デプロイ更新（RollingUpdate 戦略を推奨）
kubectl set image deploy/taskly-server server=<your-registry>/taskly-server:<tag>
kubectl rollout status deploy/taskly-server

# 起動ログでマイグレーション適用を確認
kubectl logs deploy/taskly-server --tail=200
```

## 代替手順 B: 事前に DB へだけ適用（手動 SQL）

運用上、アプリをまだ切替えたくないが DB は先に揃えたい場合。MySQL クライアントでセッション単位の `sql_mode` 緩和と DDL/DML を実行します。以下は代表例（MySQL 8.x）。

注意: 本番実行前に必ずステージングで検証してください。`ALTER TABLE ... MODIFY COLUMN` はオンライン DDL 設定・ロック時間を考慮します。

```sql
-- サニタイズ（セッションだけ NO_ZERO_DATE/NO_ZERO_IN_DATE を無効化）
SET @OLD_SQL_MODE = @@sql_mode;
SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_DATE',''));
SET SESSION sql_mode=(SELECT REPLACE(@@sql_mode,'NO_ZERO_IN_DATE',''));

-- 例: 代表的なテーブル（存在する環境のみ実行）
UPDATE role_users SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE subjects   SET created_at = '1970-01-01 00:00:00' WHERE created_at IS NULL OR created_at = '0000-00-00 00:00:00';
UPDATE migrations SET applied_at = NOW()                   WHERE applied_at IS NULL OR applied_at = '0000-00-00 00:00:00';

-- タイムスタンプのデフォルト/ON UPDATE を付与（存在するテーブルのみ）
ALTER TABLE sessions             MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																 MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE api_keys             MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																 MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE permissions          MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																 MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE blocks               MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																 MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE tasks                MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																 MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE passkeys             MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																 MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE day_shift_assignments MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																	MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE business_calendar    MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																 MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE web_push_subscriptions MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
																	 MODIFY COLUMN updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;
ALTER TABLE role_users           MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE subjects             MODIFY COLUMN created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE migrations           MODIFY COLUMN applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP;

-- 元の sql_mode に戻す
SET SESSION sql_mode=@OLD_SQL_MODE;
```

実行ホスト例（zsh / macOS）:

```bash
export DB_HOST="<prod-db-host>"
export DB_PORT=3306
export DB_USER="<user>"
export DB_PASS="<pass>"
export DB_NAME="taskly"

mysql -h "$DB_HOST" -P "$DB_PORT" -u"$DB_USER" -p"$DB_PASS" "$DB_NAME" < ./sql/apply_timestamp_defaults.sql
```

任意で、上記 SQL を `./sql/apply_timestamp_defaults.sql` に保存して流す方法も推奨です。

## 反映後の検証（必須）

以下のいずれかで DDL を目視確認します。

```sql
SHOW CREATE TABLE tasks\G;
SHOW CREATE TABLE sessions\G;
SHOW CREATE TABLE permissions\G;
SHOW CREATE TABLE api_keys\G;
SHOW CREATE TABLE role_users\G;
SHOW CREATE TABLE subjects\G;
SHOW CREATE TABLE web_push_subscriptions\G;
SHOW CREATE TABLE migrations\G;
```

期待値（例）:

- `created_at` が `datetime NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `updated_at` が `datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP`（該当テーブル）

アプリログでもマイグレーション成功が出ていることを確認します。

## ロールバック/異常時の対応

1. アプリ異常: 直前のイメージに切り戻し（Compose なら前バージョンのタグへ、K8s なら `kubectl rollout undo`）
2. DB 異常: 取得済みバックアップからリストア（またはポイントインタイム復旧）。今回の変更は主に列定義変更と一部の timestamp 値のサニタイズ（無効値 → 安全値）です。
3. 追加調査: 影響テーブルの DDL/エラーログを採取して共有

## よくある質問

- Q: アプリを再起動しないと反映されませんか？

  - A: 基本は「新バージョン起動時に自動実行」ですが、手動 SQL 手順で DB 側だけ先行反映も可能です。

- Q: セッションで `sql_mode` を変えて大丈夫？

  - A: 変更はセッション限定でグローバルへ影響しません。不正な 0 日付や NULL を正規化するために一時的に緩和しています。

- Q: SQLite への影響は？
  - A: テストは SQLite を使用しますが、`ON UPDATE` はタグではなく GORM の `autoUpdateTime` を使っており互換を確保しています。