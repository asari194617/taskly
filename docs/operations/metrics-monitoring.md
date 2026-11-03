# Metrics and monitoring

Production adds `infra/docker-compose.prod.yml` to the base Compose file. It
runs Prometheus, Alertmanager, Blackbox Exporter, and Grafana. Metric definitions,
scrape targets, alert thresholds, and dashboards live in `infra/monitoring/`.
The base development stack does not start these services.

## Collection and access

Prometheus scrapes every 30 seconds:

| Job | Target | Signal |
| --- | --- | --- |
| `taskly-api` | `server:9464/metrics` | HTTP traffic, integration jobs, application state |
| `taskly-airflow` | `airflow-scheduler:9470/metrics` | DAG runs, component health, batch delivery |
| `taskly-services` | Blackbox HTTP probes | API `/healthz`, MCP `/healthz`, Airflow `/api/v2/monitor/health` |
| `prometheus`, `alertmanager` | Their internal metrics endpoints | Monitoring service availability |

The production scheduler command runs `airflow db migrate`, starts
`airflow_exporter.py` in the background, then starts the scheduler. The exporter
reads Airflow metadata and application metrics; a collection failure returns
HTTP 503. Investigate the exporter if the scheduler is healthy but its metrics
are unavailable.

API/MCP Blackbox probes currently check **liveness**. Compose and deployment
verification check **readiness**, including dependencies. During an incident,
check both as described in [readiness](readiness.md); a successful liveness
probe alone does not establish that MySQL or Redis works.

The dashboards and query interfaces bind to host loopback. For an operator with
SSH access to the production host:

```bash
ssh -N -L 3067:127.0.0.1:3067 -L 3068:127.0.0.1:3068 \
  -L 3069:127.0.0.1:3069 <production-host>
```

Open Grafana at `http://localhost:3067`, Prometheus at `http://localhost:3068`,
and Alertmanager at `http://localhost:3069`. Grafana is provisioned with the
Prometheus datasource and Taskly operations dashboard. It enables anonymous
Viewer access and disables the login form; keep the loopback binding and use
SSH access control. Prometheus retains 15 days of data. All three use named
volumes. See the [port inventory](network-ports.md).

## Alerts and response

`alerts.yml` is the source for the current expressions and durations. Rules
cover scrape/probe failures, API 5xx ratio and latency, failed or stalled
integration jobs, notification failures, and failed, stale, or inactive DAGs.
Read the expression before deciding whether an alert describes the latest run,
a backlog, or an unavailable collector. Backup recovery and freshness checks
also follow the [backup runbook](mysql-backup.md).

Alertmanager groups alerts by alert name, instance, and DAG, waits 30 seconds
before the first notification, and repeats unresolved alerts every four hours.
`DISCORD_WEBHOOK_URL` is required for its startup wrapper, which writes the URL
to a private file for the Discord receiver. Resolved notifications are enabled.
Changing the webhook requires recreating the container with the new environment.

For an alert:

1. Check `docker compose ps` from the configured production `infra/` directory
   with both Compose files selected. Inspect the affected service logs.
2. In Prometheus, inspect `/targets`, `/alerts`, and the alert expression.
   Confirm the API/MCP `/readyz` endpoints and the Airflow health endpoint.
3. For integration failures, inspect the admin job state and downstream service
   before retrying; [delivery is at least once](../integration-jobs.md).
4. For failed notification or Airflow jobs, inspect the task logs and last
   successful run. Use [recovery procedures](recovery-drill.md) for intervention.
5. Confirm both the underlying service and metric collection recover.

The host crontab installed by `infra/ops/install-user-cron.sh` runs
`monitoring-watchdog.sh` every minute independently of Prometheus. It checks
Prometheus and Alertmanager readiness and the expected scrape sample counts.
It sends Discord notifications only when its saved state changes, using
`MONITORING_ALERT_WEBHOOK_URL` or the root `.env`'s `DISCORD_WEBHOOK_URL`.
It does not validate individual alert routing or actual notification delivery.
Check host cron/syslog if the entire monitoring stack is unavailable.

## Configuration checks

From the repository root, with `.env` configured:

```bash
docker compose -f infra/docker-compose.yml -f infra/docker-compose.prod.yml config --quiet
infra/ops/test-port-exposure.sh
docker run --rm --entrypoint promtool -w /etc/prometheus \
  -v "$PWD/infra/monitoring:/etc/prometheus:ro" prom/prometheus:v3.15.1 \
  check rules /etc/prometheus/alerts.yml
docker run --rm --entrypoint promtool -w /etc/prometheus \
  -v "$PWD/infra/monitoring:/etc/prometheus:ro" prom/prometheus:v3.15.1 \
  test rules /etc/prometheus/alerts.test.yml
```

Use the Prometheus image pinned in the production Compose file if its version
changes. On the production host, check `http://127.0.0.1:3068/-/ready` and
`http://127.0.0.1:3069/-/ready`, then inspect targets. Sending a test alert creates
a real Discord notification and belongs in a coordinated operations check.
