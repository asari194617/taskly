# 障害復旧と定期訓練

この手順の担当はリリース担当者です。代行者は、訓練記録に名前を残した別の運用担当者とします。作業中は開始・終了時刻を JST と UTC の両方で記録します。秘密情報、dump、個人データを Issue や Actions のログに貼らないでください。

## 復旧目標と測定

| 対象 | 暫定目標 | 起点と終点 |
| --- | --- | --- |
| API のサービス復旧 (RTO) | 4 時間以内 | 利用不能を検知した時刻から、ログイン・課題の作成/更新/閲覧と `/healthz` の成功まで |
| DB 復元時のデータ損失 (RPO) | 24 時間以内 | 復元前に確認した最後の正常な DB 更新時刻から、復元した backup の取得時刻まで |
| 通知配信の復旧 | 次回の 07:00 / 17:00 JST 実行前 | 未処理の通知と DAG の正常実行を確認するまで |

バックアップ DAG は毎日 03:00 JST に動くため、障害発生時刻によっては RPO 24 時間を超えます。バックアップの実際の取得時刻と最新の正常更新時刻で毎回計算し、未達なら改善 Issue を作ります。バックアップ方式・保持期間の改善は別 Issue です。この表は運用上の暫定目標であり、実測値を代入した結果も記録します。

## 最初の切り分け

1. 障害の検知時刻、最後に正常だった時刻、影響範囲、直前のリリース SHA と変更した migration を記録する。復旧担当と代行者を決め、利用者への障害告知と書き込み停止の判断を行う。
2. 本番ホストの作業ディレクトリ `infra` で `export COMPOSE_FILE=docker-compose.yml:docker-compose.prod.yml` を設定し、`docker compose ps`、`docker compose logs --tail=100 --no-color server db airflow-scheduler`、`./ops/verify-deploy-health.sh` を確認する。ログの共有時は秘密値を除去する。
3. DB に異常がある場合、復元前に現状の dump と障害時点の `migrations` 表を保全する。破損や容量不足で dump が取れない場合は理由を記録する。DB volume やバックアップを削除しない。
4. デプロイ失敗なら下記の「デプロイ失敗」、通知だけなら「通知バッチ停止」、データ破損・喪失なら「DB 復元」へ進む。

## デプロイ失敗と migration の切り戻し

GitHub Actions の本番更新は `deploy-production` concurrency group で直列化し、実行中の run は後続 push でキャンセルしない。更新前に `infra/ops/deploy-state.sh begin` が稼働中の API・email-compose・MCP の共通リリース SHA、同一リリースで稼働する Airflow 3サービスの SHA と health、`migrations` の名前一覧の digest を確認し、`<PROD_WORKDIR>/.deploy-state` に `in_progress` を記録する。更新後のヘルス確認とサービス確認がすべて成功すると `complete` が `committed` を記録する。Airflowを変更しない通常リリースでは、AirflowのSHAはアプリ3サービスと異なってよい。`.deploy-state` は rsync の削除対象から除外される。

運用スクリプトだけの変更では画像ビルドをスキップする。`release-images` が既存画像を新 SHA に付け替えて成功した後は、ビルドのスキップに関係なく `deploy` を実行する。run がキャンセルされた場合は開始しない。

本番の旧 `migrations` 履歴には `.sql` 拡張子のない名前がある。保護チェックは拡張子の有無を許容し、名前一覧全体の digest で変更を検知する。既存履歴を改名・削除しない。

`in_progress` が残った場合、次の run は本番変更前に停止する。**記録を削除して再実行しない。** `docker compose ps`、実際のコンテナイメージ、Airflow の状態、`migrations` 表と対象 SQL、失敗したジョブのログを確認する。Airflow 停止後なら DB とアプリの状態を調べて Airflow を復旧する。migration 後なら旧 API との互換性を確認し、下記の手順で切り戻しまたは修正リリースを選ぶ。復旧して全サービスのヘルスと稼働 SHA、migration 履歴が確定したら、担当者が `infra/ops/deploy-state.sh` の記録をその状態へ手動で調整し、別の担当者が確認してから次のデプロイを実行する。操作内容と判断根拠は障害記録に残す。MySQL DDL の部分適用は migration 履歴だけでは検出できないため、失敗時の SQL と実スキーマの照合が必要。

`main` の ruleset は `api-build`、`api-empty-database`、`web-build-and-test`、`mcp-build`、`email-compose-build`、`compose-build` を必須チェックとする。これらのワークフローは全 PR で実行する。必要承認数は 0 で、自己レビューでマージできる運用とする。例外権限は設定しない。必須チェックの変更時は、ruleset とこの一覧を同時に更新する。

CI と本番デプロイの画像ビルドは、ワークフローごとの runner 選択に従う。セルフホストランナーを選択できない場合は GitHub ホストランナーへのフォールバックがある。必須チェックが完了しない場合はマージせず、選択された runner とジョブログを確認する。
Web の Playwright はセルフホストランナーではブラウザのみを導入し、GitHub ホストランナーではOS依存パッケージも導入する。セルフホストランナーに `sudo` は使えない。

リリース前に担当者は変更された `apps/server/migrations/NNNN_*.sql` を列挙し、旧 API イメージが新スキーマで動くか、旧スキーマへ戻す必要があるかを PR に記録する。追加だけの変更でも、旧 API の読み書き経路を検証する。列削除や型変更がある場合は原則として先に互換期間を設け、旧イメージへの即時切り戻しを「不可」と判定する。

1. migration が未実行、または旧 API と新スキーマの互換性を検証済みなら、直前に成功した commit SHA を `TASKLY_IMAGE_TAG` に指定し、`docker compose up -d --wait --wait-timeout 600 --no-build server email-compose mcp airflow-webserver airflow-scheduler airflow-dag-processor` を実行する。DB と Redis の volume は保持する。
2. `./ops/verify-deploy-health.sh` と主要操作を確認する。失敗時のコンテナ状態とログを保存する。
3. migration が途中で失敗した場合、`migrations` 表だけで成功を判定しない。MySQL DDL はファイル全体で巻き戻らない。`SHOW CREATE TABLE` と対象データを migration SQL に照らして確認し、修正 migration を別リリースで適用するか、DB 復元へ進む。適用履歴を手作業で削除して再実行しない。
4. 旧 API が新スキーマと非互換なら、イメージだけを戻さない。書き込みを止め、復元時点以降の更新を失うことを確認したうえで、互換性を回復する migration または DB 復元を選ぶ。復元には下記の訓練で確認済みの手順を使う。

## 通知バッチ停止

1. `docker compose ps airflow-scheduler airflow-dag-processor airflow-webserver email-compose db` と各ログを確認する。Airflow の `notifications` DAG（07:00 / 17:00 JST）の最終実行、失敗した task、`day_shift_notify_0830` の状態を確認する。
2. DB、email-compose、SMTP 等の依存先を復旧し、Airflow 上で該当 DAG の失敗 task を再実行する。手動実行前に同じ時間帯の成功済み配信がないか、`notifications_queue` の未処理件数と配信ログを確認する。メールや Web Push の重複送信を避けるため、成功済み task を一括再実行しない。
3. DAG の次回実行時刻と task の成功、未処理件数の減少を確認する。外部配信先の受信もテスト用アカウントで確認する。欠落・重複があれば影響件数を記録し、改善 Issue を作る。

## DB 復元

本番 DB へ直接投入する前に、同じバックアップを**隔離した検証環境**で復元し、下記の操作と migration の互換性を確認する。本番から取得した dump は機密データとして扱い、検証環境でのアクセスと削除を記録する。現行バックアップは `airflow-backups` volume と FTPS 上の `backup-YYYYMMDD-HHMMSS.sql.gz.age`（ファイル名は UTC）で、age 秘密鍵は本番ホスト以外の安全な場所に保管する。取得・復号と整合性確認は [`mysql-backup.md`](mysql-backup.md) に従う。旧 `airflow-data` volume と FTP にある `.sql.gz` は暗号化導入前の形式であり、別途保護して扱う。ファイル名の時刻と Airflow の `backup_mysql` 成功時刻を照合し、最新の正常バックアップが不明なら復元を止める。**復号と gzip 検査だけでは復元可能と判定しない。** 2026-09-21 の訓練では MariaDB 版 `mysqldump` の生成列処理により、整合性検査に通った backup の SQL 投入が失敗した。Airflow イメージの `mysqldump --version` が MySQL 8.4 であること、実際に SQL 投入とアプリ操作まで成功することを確認する。

1. 検証環境の MySQL 8.4 を**新しい空の volume**で起動する。既存の本番/検証 DB に上書きしない。アプリの外部送信（SMTP、Web Push、Discord、Teams）を無効にする。検証用の `.env` は本番の秘密値をそのまま流用しない。
2. 次のコマンドを検証環境の `infra` で実行する。`BACKUP_FILE` は検証環境内の暗号化 backup の絶対パス、`BACKUP_AGE_IDENTITY` は別途保管した秘密鍵のパスに置き換える。固有の Compose project 名を使い、`docker compose config --volumes` で本番の `db-data` と異なる volume が作られることを確認する。検証用 `.env` は別途用意する。

   ```bash
   export COMPOSE_FILE=docker-compose.yml
   export COMPOSE_PROJECT_NAME=taskly-recovery-YYYYMMDD
   docker compose config --services
   docker compose config --volumes
   docker compose up -d --wait db
   set -o pipefail
   age -d -i "$BACKUP_AGE_IDENTITY" "$BACKUP_FILE" | gzip -t
   age -d -i "$BACKUP_AGE_IDENTITY" "$BACKUP_FILE" | gzip -dc | docker compose exec -T db sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot "$MYSQL_DATABASE"'
   docker compose exec -T db sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot "$MYSQL_DATABASE" -e "SELECT name, applied_at FROM migrations ORDER BY name DESC LIMIT 5"'
   ```

   旧 `.sql.gz` を使う場合は age の復号を除き、次のコマンドに置き換える。どちらも展開後の平文をファイルに保存しない。検証環境には `age` と `gzip` を用意する。

   ```bash
   gzip -t "$BACKUP_FILE"
   gzip -dc "$BACKUP_FILE" | docker compose exec -T db sh -c 'MYSQL_PWD="$MYSQL_ROOT_PASSWORD" exec mysql -uroot "$MYSQL_DATABASE"'
   ```

3. dump 時点に対応する API イメージを起動する。`/app/migrate` を無条件に実行せず、dump の `migrations` と対象イメージの SQL を照合する。新しい migration を試す場合は復元 DB をもう一つ複製し、適用前後のスキーマとデータ件数、旧イメージへの切り戻し可否を確認する。
4. `/healthz` とログイン、テスト用アカウントによる課題の一覧・作成・更新・削除を確認する。Airflow が使う主要テーブルと `notifications_queue` も読めることを確認する。検証環境の外部通知は送信しない。
5. 本番復元が必要なら障害告知と書き込み停止を行い、保全 dump、復元点、データ損失見込み、担当者を記録する。新しい DB volume に同じ復元手順を実行する。トラフィックを戻す前に、[退会・消去の再適用手順](account-data.md#復元時の消去再適用)に従い、バックアップ取得後の退会・消去請求を突合する。検証済みイメージを起動して主要操作を確認してからトラフィックを戻す。旧 volume は原因調査と取りこぼした更新の照合が終わるまで保持する。復元後の差分データは個別に照合し、必要なら再入力する。

## 訓練の実施と記録

- **四半期に 1 回**、リリース担当者が直近の成功バックアップを隔離環境へ復元し、アプリ操作まで確認する。代行者が文書だけを使って手順を再現し、担当者は観察と記録を行う。担当者不在なら代行者が実施し、別の運用担当者が結果を確認する。バックアップ単体の月次復元確認と鍵変更時の確認は [`mysql-backup.md`](mysql-backup.md) に従う。
- スキーマ変更を含む**各リリースの前**に、migration と旧 API の互換性判定、切り戻し手順を PR に記録する。破壊的変更や復元手順の変更があればリリース前にも訓練する。
- 記録は `docs/operations/recovery-drill-record-template.md` を複製した非公開の運用記録またはアクセス制限付き Issue に残す。dump、秘密値、個人データは記録しない。RTO/RPO の実測値、操作結果、担当、証跡の参照、未達の改善 Issue を含める。
- RTO は検知から主要操作成功までの経過時間、RPO は最後の正常更新から復元点までの差を計算する。訓練では障害検知・更新時刻を模擬して明記し、バックアップ取得から利用再開までの実作業時間も別に測る。目標未達や再現不能な手順は改善 Issue に担当と期限を付け、次回訓練で再確認する。
