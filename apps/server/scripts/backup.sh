#!/bin/sh#!/usr/bin/env bash

set -euset -euo pipefail



DB_HOST=${DB_HOST:-localhost}# Required envs:

DB_PORT=${DB_PORT:-3306}# MYSQL_HOST, MYSQL_PORT, MYSQL_USER, MYSQL_PASSWORD, MYSQL_DATABASE

DB_USER=${DB_USER:-root}# FTP_HOST, FTP_USER, FTP_PASSWORD, FTP_DIR (e.g., /backups)

DB_PASSWORD=${DB_PASSWORD:-}# BACKUP_RETENTION_DAYS (default 7)

DB_NAME=${DB_NAME:-taskly}# Optional: MYSQLDUMP_EXTRA_OPTS



OUT_DIR=${BACKUP_DIR:-/var/backups}RETENTION=${BACKUP_RETENTION_DAYS:-7}

mkdir -p "$OUT_DIR"STAMP=$(date +"%Y%m%d-%H%M%S")

BACKUP_NAME="backup-${STAMP}.sql.gz"

STAMP=$(date +%Y%m%d-%H%M%S)TMP_DIR=${TMP_DIR:-/tmp}

OUT_FILE="$OUT_DIR/backup-$STAMP.sql.gz"BACKUP_PATH="${TMP_DIR}/${BACKUP_NAME}"



# Requires mariadb-client in image (installed in Dockerfile)if [[ -z "${MYSQL_HOST:-}" || -z "${MYSQL_USER:-}" || -z "${MYSQL_PASSWORD:-}" || -z "${MYSQL_DATABASE:-}" ]]; then

mysqldump -h"$DB_HOST" -P"$DB_PORT" -u"$DB_USER" -p"$DB_PASSWORD" "$DB_NAME" | gzip > "$OUT_FILE"  echo "[backup] Missing MySQL envs" >&2

chmod 600 "$OUT_FILE"  exit 1

fi

echo "Backup created: $OUT_FILE"FTP_DIR=${FTP_DIR:-/backups}
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
