#!/usr/bin/env bash
set -euo pipefail

# Required envs:
# MYSQL_HOST, MYSQL_PORT, MYSQL_USER, MYSQL_PASSWORD, MYSQL_DATABASE
# FTP_HOST, FTP_USER, FTP_PASSWORD, FTP_DIR (e.g., /backups)
# BACKUP_RETENTION_DAYS (default 7)
# Optional: MYSQLDUMP_EXTRA_OPTS

RETENTION=${BACKUP_RETENTION_DAYS:-7}
STAMP=$(date +"%Y%m%d-%H%M%S")
BACKUP_NAME="backup-${STAMP}.sql.gz"
TMP_DIR=${TMP_DIR:-/tmp}
BACKUP_PATH="${TMP_DIR}/${BACKUP_NAME}"

if [[ -z "${MYSQL_HOST:-}" || -z "${MYSQL_USER:-}" || -z "${MYSQL_PASSWORD:-}" || -z "${MYSQL_DATABASE:-}" ]]; then
  echo "[backup] Missing MySQL envs" >&2
  exit 1
fi
FTP_DIR=${FTP_DIR:-/backups}
if [[ -z "${FTP_HOST:-}" || -z "${FTP_USER:-}" || -z "${FTP_PASSWORD:-}" ]]; then
  echo "[backup] Missing FTP envs" >&2
  exit 1
fi

# Dump
echo "[backup] Dumping MySQL ${MYSQL_DATABASE}..."
mysqldump -h"${MYSQL_HOST}" -P"${MYSQL_PORT:-3306}" -u"${MYSQL_USER}" -p"${MYSQL_PASSWORD}" \
  --single-transaction --quick --routines --events ${MYSQLDUMP_EXTRA_OPTS:-} "${MYSQL_DATABASE}" | gzip -c > "${BACKUP_PATH}"

echo "[backup] Uploading to FTP via curl..."
FTP_URL="ftp://${FTP_HOST}${FTP_DIR}/${BACKUP_NAME}"
curl -sS --ftp-create-dirs --user "${FTP_USER}:${FTP_PASSWORD}" -T "${BACKUP_PATH}" "${FTP_URL}"
# Set permission 600 if server supports SITE CHMOD
curl -sS --user "${FTP_USER}:${FTP_PASSWORD}" -Q "SITE CHMOD 600 ${FTP_DIR}/${BACKUP_NAME}" "ftp://${FTP_HOST}"
# Retention: list only names and delete older ones beyond RETENTION; assumes lexicographic sort by timestamp
LIST=$(curl -sS --list-only --user "${FTP_USER}:${FTP_PASSWORD}" "ftp://${FTP_HOST}${FTP_DIR}/" | sort)
COUNT=$(echo "$LIST" | wc -l | tr -d ' ')
KEEP=${RETENTION}
if [[ ${COUNT} -gt ${KEEP} ]]; then
  DEL_NUM=$((COUNT-KEEP))
  DEL_LIST=$(echo "$LIST" | head -n ${DEL_NUM})
  while IFS= read -r f; do
    [[ -n "$f" ]] && curl -sS --user "${FTP_USER}:${FTP_PASSWORD}" -Q "DELE ${FTP_DIR}/${f}" "ftp://${FTP_HOST}"
  done <<< "$DEL_LIST"
fi

# Local cleanup
rm -f "${BACKUP_PATH}"
echo "[backup] Done"
