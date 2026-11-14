#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd -P)
temp=$(mktemp -d)
temp=$(cd "$temp" && pwd -P)
trap 'rm -rf "$temp"' EXIT
mkdir -p "$temp/bin" "$temp/host/infra/ops" "$temp/host/apps/server"
for name in deploy-state verify-deploy-health disk-maintenance monitoring-watchdog; do cp "$root/infra/ops/$name.sh" "$temp/host/infra/ops/"; done
cp "$root/apps/server/schema.sql" "$temp/host/apps/server/schema.sql"
cp "$root/atlas.hcl" "$temp/host/atlas.hcl"
touch "$temp/host/infra/.env"
printf 'CREATE TABLE fixture (id bigint PRIMARY KEY);\n' > "$temp/schema"
digest=$(sha256sum "$temp/schema" | cut -d' ' -f1)
release=85222d44a5bf082d36d9dd781889eda73f0ff013
baseline=1111111111111111111111111111111111111111
cat > "$temp/bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$MOCK_CALLS"
case "$1 $2" in
 'compose ps') printf '%s\n' "${*: -1}" ;;
 'compose exec')
   if [[ $* == *mysqldump* && $4 == db ]]; then cat "$MOCK_SCHEMA"
   elif [[ $* == *'dags details'* ]]; then printf 'is_paused %s\n' "${MOCK_PAUSED:-False}"
   elif [[ $* == *9470/metrics* ]]; then printf 'taskly_airflow_dag_last_run_timestamp_seconds 1\ntaskly_airflow_dag_active 1\n'
   fi ;;
 'inspect --format')
   case "$3" in
    '{{.Config.Image}}') printf 'ghcr.io/asari194617/taskly/api:%s\n' "$MOCK_RELEASE" ;;
    '{{.RestartCount}}') printf '0\n' ;;
    '{{.State.Status}}') printf 'running\n' ;;
    *)
      if [[ $3 == *'{{.State.Status}}'* ]]; then
        case "$4" in prometheus|alertmanager|blackbox|grafana) echo 'running missing' ;; *) echo "running ${MOCK_HEALTH:-healthy}" ;; esac
      else echo "${MOCK_HEALTH:-healthy}"; fi ;;
   esac ;;
 *) exit 91 ;;
esac
MOCK
cat > "$temp/bin/curl" <<'MOCK'
#!/usr/bin/env bash
if [[ $* == *write-out* ]]; then printf 200
elif [[ $* == *3064/metrics* ]]; then printf 'go_goroutines\ntaskly_http_requests_total\ntaskly_integration_jobs\ntaskly_notification_deliveries\n'
else printf '{"metadatabase":{"status":"healthy"},"scheduler":{"status":"healthy"},"dag_processor":{"status":"healthy"}}'; fi
MOCK
cat > "$temp/bin/crontab" <<'MOCK'
#!/usr/bin/env bash
[[ $# == 1 && $1 == -l ]] || exit 92
cat "$MOCK_CRON"
MOCK
printf '#!/usr/bin/env bash\nexit 0\n' > "$temp/bin/sleep"
chmod +x "$temp/bin/"*
export PATH="$temp/bin:$PATH" MOCK_CALLS="$temp/calls" MOCK_SCHEMA="$temp/schema" MOCK_RELEASE="$release" MOCK_CRON="$temp/cron"
cat > "$temp/cron" <<EOF
# BEGIN taskly disk maintenance
*/5 * * * * cd '$temp/host/infra' && bash -o pipefail -c './ops/disk-maintenance.sh status 2>&1 | logger -t taskly-disk'
* * * * * cd '$temp/host/infra' && ./ops/monitoring-watchdog.sh 2>&1 | logger -t taskly-monitoring
25 3 * * * cd '$temp/host/infra' && bash -o pipefail -c './ops/disk-maintenance.sh cleanup 2>&1 | logger -t taskly-disk'
# END taskly disk maintenance
EOF
reset_state() { printf 'in_progress\n%s\n%s\n%s\nschema-v1\n' "$baseline" "$release" "$digest" > "$temp/host/.deploy-state"; chmod 600 "$temp/host/.deploy-state"; }
reset_state
bash "$root/infra/ops/reconcile-code-only.sh" "$temp/host" "$release" "$baseline" "$digest"
[[ $(head -1 "$temp/host/.deploy-state") == committed ]]
for issue in committed bad-mode missing-state wrong-target paused unhealthy cron schema-drift changed-helper symlink-parent; do
 reset_state
 target=$release
 case "$issue" in
  committed) sed '1s/in_progress/committed/' "$temp/host/.deploy-state" > "$temp/next"; cp "$temp/next" "$temp/host/.deploy-state" ;;
  bad-mode) chmod 644 "$temp/host/.deploy-state" ;;
  missing-state) rm "$temp/host/.deploy-state" ;;
  wrong-target) target=$baseline ;;
  paused) export MOCK_PAUSED=True ;;
  unhealthy) export MOCK_HEALTH=unhealthy ;;
  cron) cp "$temp/cron" "$temp/cron.saved"; printf '\n' > "$temp/cron" ;;
  schema-drift) printf 'CREATE TABLE drift (id int);\n' >> "$temp/schema" ;;
  changed-helper) printf '\n# changed\n' >> "$temp/host/infra/ops/deploy-state.sh" ;;
  symlink-parent) mv "$temp/host/infra/ops" "$temp/ops"; ln -s "$temp/ops" "$temp/host/infra/ops" ;;
 esac
 if bash "$root/infra/ops/reconcile-code-only.sh" "$temp/host" "$target" "$baseline" "$digest" > "$temp/output" 2>&1; then echo "Unsafe recovery accepted: $issue" >&2; exit 1; fi
 if [[ -e $temp/host/.deploy-state && $issue != committed ]]; then [[ $(head -1 "$temp/host/.deploy-state") == in_progress ]]; fi
 unset MOCK_PAUSED MOCK_HEALTH
 case "$issue" in
  cron) mv "$temp/cron.saved" "$temp/cron" ;;
  schema-drift) sed '$d' "$temp/schema" > "$temp/next"; mv "$temp/next" "$temp/schema" ;;
  changed-helper) cp "$root/infra/ops/deploy-state.sh" "$temp/host/infra/ops/" ;;
  symlink-parent) rm "$temp/host/infra/ops"; mv "$temp/ops" "$temp/host/infra/ops" ;;
 esac
done
if grep -Eq 'compose (up|down|pull|stop)|dags unpause|docker login|apply-schema|atlas' "$temp/calls"; then echo 'Recovery invoked a mutating command' >&2; exit 1; fi
echo 'Read-only reconciliation preconditions and guarded completion: PASS'
