"""Small, private Prometheus exporter for fixed Taskly Airflow DAGs.

Runs beside the scheduler, which already has access to Airflow's separate
metadata database. No metadata database credentials are added to the API.
"""

import logging
import os
import sys
import json
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.request import urlopen

import mysql.connector


DAGS = (
    "account_erasure",
    "backup_mysql",
    "day_shift_index_increment",
    "day_shift_notify_0830",
    "moderation_daily",
    "notifications",
)
OUTCOMES = ("success", "failed")
NOTIFICATION_TASKS = {
    "email_delivery": "email",
    "webpush_delivery": "webpush",
    "discord_delivery": "discord",
}
AIRFLOW_COMPONENTS = ("metadatabase", "scheduler", "dag_processor")


def collect() -> bytes:
    connection = mysql.connector.connect(
        host="db",
        port=3306,
        user=os.environ["AIRFLOW_META_USER"],
        password=os.environ["AIRFLOW_META_PASSWORD"],
        database=os.environ["AIRFLOW_META_DATABASE"],
        connection_timeout=3,
    )
    try:
        cursor = connection.cursor()
        try:
            placeholders = ",".join(["%s"] * len(DAGS))
            cursor.execute(
                "SELECT dag_id, state, UNIX_TIMESTAMP(MAX(end_date)) "
                f"FROM dag_run WHERE dag_id IN ({placeholders}) "
                "AND state IN ('success', 'failed') GROUP BY dag_id, state",
                DAGS,
            )
            latest = {(dag, outcome): float(ts) for dag, outcome, ts in cursor if ts is not None}
            cursor.execute(
                "SELECT dag_id, is_paused FROM dag "
                f"WHERE dag_id IN ({placeholders})",
                DAGS,
            )
            active = {dag: not bool(is_paused) for dag, is_paused in cursor}
            cursor.execute(
                "SELECT task_id, state, UNIX_TIMESTAMP(MAX(end_date)) "
                "FROM task_instance WHERE dag_id = 'notifications' "
                "AND task_id IN ('email_delivery', 'webpush_delivery', 'discord_delivery') "
                "AND state IN ('success', 'failed') GROUP BY task_id, state"
            )
            delivery_latest = {
                (NOTIFICATION_TASKS[task], outcome): float(ts)
                for task, outcome, ts in cursor
                if ts is not None and task in NOTIFICATION_TASKS
            }
        finally:
            cursor.close()
    finally:
        connection.close()

    lines = [
        "# HELP taskly_airflow_dag_last_run_timestamp_seconds Last completed DAG run by fixed DAG and outcome.",
        "# TYPE taskly_airflow_dag_last_run_timestamp_seconds gauge",
    ]
    for dag in DAGS:
        for outcome in OUTCOMES:
            lines.append(
                f'taskly_airflow_dag_last_run_timestamp_seconds{{dag="{dag}",outcome="{outcome}"}} '
                f"{latest.get((dag, outcome), 0):.0f}"
            )
    lines.extend([
        "# HELP taskly_airflow_dag_active Whether a fixed DAG is registered and unpaused.",
        "# TYPE taskly_airflow_dag_active gauge",
    ])
    for dag in DAGS:
        lines.append(f'taskly_airflow_dag_active{{dag="{dag}"}} {int(active.get(dag, False))}')
    lines.extend([
        "# HELP taskly_notification_batch_last_run_timestamp_seconds Last notification batch task run by channel and outcome.",
        "# TYPE taskly_notification_batch_last_run_timestamp_seconds gauge",
    ])
    for channel in NOTIFICATION_TASKS.values():
        for outcome in OUTCOMES:
            lines.append(
                f'taskly_notification_batch_last_run_timestamp_seconds{{channel="{channel}",outcome="{outcome}"}} '
                f"{delivery_latest.get((channel, outcome), 0):.0f}"
            )
    try:
        with urlopen("http://airflow-webserver:3065/api/v2/monitor/health", timeout=3) as response:
            health = json.load(response)
    except Exception:
        health = {}
    lines.extend([
        "# HELP taskly_airflow_component_healthy Airflow component health from its monitor endpoint.",
        "# TYPE taskly_airflow_component_healthy gauge",
    ])
    for component in AIRFLOW_COMPONENTS:
        healthy = int(health.get(component, {}).get("status") == "healthy")
        lines.append(f'taskly_airflow_component_healthy{{component="{component}"}} {healthy}')
    return ("\n".join(lines) + "\n").encode()


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != "/metrics":
            self.send_error(404)
            return
        try:
            body = collect()
        except Exception as exc:
            logging.error("Airflow metrics collection failed: %s", type(exc).__name__)
            self.send_error(503)
            return
        self.send_response(200)
        self.send_header("Content-Type", "text/plain; version=0.0.4")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


if __name__ == "__main__":
    if "--once" in sys.argv:
        sys.stdout.buffer.write(collect())
    else:
        ThreadingHTTPServer(("0.0.0.0", 9470), Handler).serve_forever()
