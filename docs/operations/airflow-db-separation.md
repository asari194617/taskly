# Airflow メタDBの分離

2026-09-22 の本番移行と検証結果は [`airflow-db-separation-2026-09-22.md`](airflow-db-separation-2026-09-22.md) に記録しています。

## 接続とバックアップ

`MYSQL_DATABASE` / `MYSQL_USER` / `MYSQL_PASSWORD` は Taskly のアプリDB用です。API、MCP、DAG の Python 処理とバッチ、アプリ migration、毎日の暗号化バックアップはこの接続を使います。`AIRFLOW_META_DATABASE` / `AIRFLOW_META_USER` / `AIRFLOW_META_PASSWORD` は Airflow のメタデータ専用です。別のDB名とユーザー名を使い、パスワードは英数字・`_`・`-` で生成してください（例: `openssl rand -hex 32`）。URL に埋め込むため、この文字集合に限定します。両方の資格情報は本番 `PROD_ENV` とローカル `.env` に設定します。

アプリのバックアップはアプリDBだけを含みます。同じ `backup_mysql` DAG の次タスクがメタDBを暗号化し、ローカル `airflow-backups` volume と FTPS の `airflow/` サブディレクトリに保存します。保持期間と最低世代数はDBごとに適用します。移行前の混在バックアップは旧構成を復元する場合の切り戻し用に保管します。メタDBの復元先には `AIRFLOW_META_DATABASE` を指定し、アプリDBの復元先には `MYSQL_DATABASE` を指定します。

## 初回移行

本番デプロイは `infra/ops/prepare-airflow-db.sh` を Airflow サービス停止後、アプリ migration と Airflow 起動前に実行します。MySQL が既存 volume で起動していても root 接続で新DB・ユーザーを作ります。新 Airflow イメージで一時的なメタスキーマを作成してテーブル一覧を取得し、旧アプリDBにある Airflow テーブルを1つの `RENAME TABLE` 文で移動します。MySQL が旧スキーマへの内部参照を保持する場合があるため、移動したメタテーブルの外部キーを新スキーマ内への参照として作り直します。その後 `airflow db migrate` を再実行して新バージョンに必要なテーブルを補い、`dag_run` と `task_instance` の件数を照合します。Airflow のメタユーザーにはメタDBだけ、アプリユーザーにはアプリDBだけを許可します。双方のアカウントで反対側のDBへの書き込み拒否を確認します。

デプロイ前に以下を確認します。

1. アプリDBと Airflow メタDBの復元可能なバックアップがあること。初回分離前の混在DBには少なくとも検証済みのバックアップが必要です。
2. `PROD_ENV` に新しい3変数を追加すること。既存の `MYSQL_*` を変更しないでください。
3. 実行中の Airflow DAG を確認し、タスク実行中でない時間にマージすること。停止から再起動まで定期ジョブは実行されません。

移行後は `airflow dags list`、主要 DAG の状態、直近の `dag_run` / `task_instance` 件数、API・MCP の healthcheck、バックアップ DAG の次回実行を確認します。アプリ migration と復元確認はアプリDBに対して行います。ローカルの新規DBでは旧 Airflow テーブルが存在しないため、新DBを直接初期化します。CI の Compose 設定にも `AIRFLOW_META_*` が必要です。

## 失敗時の切り戻し

`prepare-airflow-db.sh` の `RENAME TABLE` 前に失敗した場合、旧DBのメタテーブルは残っています。旧リリースの Compose と旧 `.env` で Airflow を再起動します。`RENAME TABLE` 後は、まず新 Airflow サービスを停止し、メタDBのバックアップを取得してください。新DB内の Airflow テーブルと旧DB内の同名テーブルに重複がないことを確認し、root 接続で新DBから旧DBへ逆向きの単一 `RENAME TABLE` を実行します。その後、旧リリースの Compose・旧 `.env` に戻して Airflow を再起動します。新バージョンの `airflow db migrate` が旧バージョンと互換でない場合は、旧混在DBの検証済みバックアップから復元します。移行後に新規実行された DAG 履歴は、切り戻し前に別途保存してください。

Airflow 起動後に障害が出た場合、アプリDBを一緒に巻き戻さず、メタDBの状態と実行履歴を先に確認してください。DB volume を削除しないでください。
