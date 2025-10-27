#!/usr/bin/env bash

set -euo pipefail

# Required envs: MYSQL_HOST, MYSQL_PORT, MYSQL_USER, MYSQL_PASSWORD, MYSQL_DATABASE
# Optional: MYSQLDUMP_EXTRA_OPTS, BACKUP_DIR, TMP_DIR, BACKUP_RETENTION_DAYS, FTP_HOST, FTP_USER, FTP_PASSWORD, FTP_DIR

MYSQL_HOST=${MYSQL_HOST:-localhost}
MYSQL_PORT=${MYSQL_PORT:-3306}
MYSQL_USER=${MYSQL_USER:-root}
MYSQL_PASSWORD=${MYSQL_PASSWORD:-}
MYSQL_DATABASE=${MYSQL_DATABASE:-taskly}

OUT_DIR=${BACKUP_DIR:-/var/backups}
TMP_DIR=${TMP_DIR:-/tmp}
RETENTION=${BACKUP_RETENTION_DAYS:-7}
FTP_DIR=${FTP_DIR:-/backups}

mkdir -p "$OUT_DIR" "$TMP_DIR"
STAMP=$(date +"%Y%m%d-%H%M%S")
BACKUP_NAME="backup-${STAMP}.sql.gz"
BACKUP_PATH="${TMP_DIR}/${BACKUP_NAME}"
OUT_FILE="${OUT_DIR}/${BACKUP_NAME}"

if [[ -z "${MYSQL_HOST}" || -z "${MYSQL_USER}" || -z "${MYSQL_DATABASE}" ]]; then
  echo "[backup] Missing MySQL envs" >&2
  exit 1
fi

echo "[backup] Dumping MySQL ${MYSQL_DATABASE}..."
mysqldump -h"${MYSQL_HOST}" -P"${MYSQL_PORT}" -u"${MYSQL_USER}" -p"${MYSQL_PASSWORD}" \
  --single-transaction --quick --routines --events ${MYSQLDUMP_EXTRA_OPTS:-} "${MYSQL_DATABASE}" | gzip -c > "${BACKUP_PATH}"

install -m 600 "${BACKUP_PATH}" "${OUT_FILE}"
echo "[backup] Local backup created: ${OUT_FILE} (mode 600)"

if [[ -n "${FTP_HOST:-}" && -n "${FTP_USER:-}" && -n "${FTP_PASSWORD:-}" ]]; then
  echo "[backup] Uploading to FTP via curl..."
  FTP_URL="ftp://${FTP_HOST}${FTP_DIR}/${BACKUP_NAME}"
  curl -sS --ftp-create-dirs --user "${FTP_USER}:${FTP_PASSWORD}" -T "${BACKUP_PATH}" "${FTP_URL}"
  # Attempt to set permission 600 if server supports SITE CHMOD (best effort)
  curl -sS --user "${FTP_USER}:${FTP_PASSWORD}" -Q "SITE CHMOD 600 ${FTP_DIR}/${BACKUP_NAME}" "ftp://${FTP_HOST}" || true
  # Retention on FTP: keep latest ${RETENTION}
  LIST=$(curl -sS --list-only --user "${FTP_USER}:${FTP_PASSWORD}" "ftp://${FTP_HOST}${FTP_DIR}/" | sort)
  COUNT=$(echo "$LIST" | wc -l | tr -d ' ')
  if [[ ${COUNT} -gt ${RETENTION} ]]; then
    DEL_NUM=$((COUNT-RETENTION))
    echo "$LIST" | head -n ${DEL_NUM} | while IFS= read -r f; do
      [[ -n "$f" ]] && curl -sS --user "${FTP_USER}:${FTP_PASSWORD}" -Q "DELE ${FTP_DIR}/${f}" "ftp://${FTP_HOST}" || true
    done
  fi
fi

# Local cleanup of temp file
rm -f "${BACKUP_PATH}"
echo "[backup] Done"
