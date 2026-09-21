# MySQL バックアップの運用・復元

## 設定

保存先は XServer Static の FTPS 領域です。通信は明示的 TLS（証明書検証あり）で暗号化し、保存するファイル自体も age X25519 公開鍵で暗号化します。FTP アカウントには専用の非公開ディレクトリを使い、HTTP から参照できないことを確認してください。

1. 信頼できる別端末で `age-keygen -o backup-age-key.txt` を実行し、秘密鍵を安全な場所に保管します。秘密鍵を Git や本番サーバーに置かないでください。復元担当者がアクセスできる別の保管先にもバックアップします。
2. 公開鍵 `age1...` を `.env` の `BACKUP_AGE_RECIPIENT` に設定します。さらに `FTP_HOST`, `FTP_PORT`, `FTP_USER`, `FTP_PASSWORD`, `FTP_DIR` を設定します。設定不足ならジョブは失敗し、Airflow の Discord 通知が発火します。
3. `FTP_DIR` が公開領域の外にあることを確認します。XServer Static でそれが保証できない場合、この保存先は使用せず非公開の FTPS 領域へ変更します。

バックアップは毎日 03:00 JST に実行されます。転送は最大3回、Airflow は10分間隔で2回再試行します。アップロード後に FTPS で再取得し、ローカルとリモートのサイズ・SHA-256 を比較してから正式名に変更します。リモートは14日間、最低3世代を保持します。ローカルの暗号化ファイルは `infra/ops/disk-maintenance.sh` が同じ期間・最低世代数で整理します。古い平文 `.sql.gz` ファイルはこの処理の対象外なので、移行時に別途安全に削除してください。

Airflow イメージ内の `mysqldump` は MySQL 8.4 版であることを `mysqldump --version` で確認します。MariaDB 版で生成した 2026-09-21 09:46:17 UTC より前の `.sql.gz.age` は、gzip 検査に成功しても `tasks.effective_deadline` の生成列で SQL 復元が失敗することを訓練で確認しました。これらを復元可能な世代として数えず、再取得したバックアップを隔離 DB に最後まで投入してから利用可能と判定してください。

## 復元確認（毎月・鍵変更後）

本番 DB に復元しないでください。検証用 MySQL に対し、次を実行します。

```sh
# FTPS から任意の正式名のバックアップを取得する。資格情報は対話入力または
# 権限600の curl 設定ファイルに保存し、コマンドラインには載せない。
curl --fail --ssl-reqd --ftp-ssl-control --user "$FTP_USER" \
  "ftp://$FTP_HOST$FTP_DIR/backup-YYYYMMDD-HHMMSS.sql.gz.age" \
  -o backup.sql.gz.age
age -d -i /safe/path/backup-age-key.txt backup.sql.gz.age | gzip -t
age -d -i /safe/path/backup-age-key.txt backup.sql.gz.age | gzip -dc \
  | mysql --defaults-extra-file=/safe/path/restore-client.cnf -h 127.0.0.1 -P 3306 restore_test
```

`restore-client.cnf` は権限600で `[client]` と検証用 DB の `user`/`password` を設定します。復元後は重要テーブルの件数と代表データを確認し、実行日・バックアップ名・SHA-256・結果・確認者を運用記録に残します。失敗時は Airflow のログと Discord 通知を確認し、最後に検証済みの世代を保持したまま原因を調査します。

## 復元確認記録

| 実行日 | バックアップ名 | SHA-256 | 検証先 | 結果 | 確認者 |
| --- | --- | --- | --- | --- | --- |
| 2026-09-21 | 統合テストで生成した一時 age ファイル | 一時ファイルのため記録なし | Docker MySQL 8.4 `restore_target` | `backup_source.sample` の行を復元し `restored` を確認 | 自動テスト `TestIntegrationRestore` |

2026-09-21 に本番設定の FTPS 接続先へ一時ファイルを送受信し、Go クライアントでサイズ・SHA-256 照合が成功した。一時ファイルはテスト後に削除した。
