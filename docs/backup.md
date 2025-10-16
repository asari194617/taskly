MySQL backups (server container)

- Schedule: daily at 03:00 JST via crond inside the server container
- Tooling: mysqldump + gzip; upload via curl to FTP

Environment variables required (set in infra/.env):

- MYSQL_HOST, MYSQL_PORT (default 3306)
- MYSQL_USER, MYSQL_PASSWORD
- MYSQL_DATABASE
- FTP_HOST, FTP_USER, FTP_PASSWORD
- FTP_DIR: remote path like /backups
- BACKUP_RETENTION_DAYS: number of most recent files to keep (default 7)

Notes:

- FTP chmod is skipped for portability. Ensure your FTP server enforces secure defaults.
- Filenames include timestamp; retention deletes oldest files beyond the count.
- Timezone uses Asia/Tokyo; adjust in Dockerfile crontab if needed.
