"""Check the local Airflow job heartbeat without importing Airflow/providers."""

import argparse
import os
import socket
import sys


THRESHOLDS = {
    "SchedulerJob": "AIRFLOW__SCHEDULER__SCHEDULER_HEALTH_CHECK_THRESHOLD",
    "DagProcessorJob": "AIRFLOW__DAG_PROCESSOR__HEALTH_CHECK_THRESHOLD",
}


def open_connection():
    import mysql.connector

    return mysql.connector.connect(
        host="db",
        port=3306,
        user=os.environ["AIRFLOW_META_USER"],
        password=os.environ["AIRFLOW_META_PASSWORD"],
        database=os.environ["AIRFLOW_META_DATABASE"],
        connection_timeout=3,
        # latest_heartbeat is a TIMESTAMP: use UTC for both sides even when
        # the database host/session defaults to Asia/Tokyo.
        time_zone="+00:00",
    )


def is_healthy(job_type):
    threshold = int(os.environ.get(THRESHOLDS[job_type], "30"))
    if threshold <= 0:
        return False
    connection = open_connection()
    try:
        cursor = connection.cursor()
        try:
            cursor.execute(
                "SELECT state, "
                "TIMESTAMPDIFF(MICROSECOND, latest_heartbeat, UTC_TIMESTAMP(6)) / 1000000 "
                "FROM job WHERE job_type = %s AND hostname = %s "
                "ORDER BY latest_heartbeat DESC LIMIT 1",
                (job_type, socket.gethostname()),
            )
            row = cursor.fetchone()
        finally:
            cursor.close()
    finally:
        connection.close()
    return bool(row and row[0] == "running" and row[1] is not None and row[1] < threshold)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("job_type", choices=THRESHOLDS)
    args = parser.parse_args()
    try:
        return 0 if is_healthy(args.job_type) else 1
    except Exception as exc:
        # Connector errors may contain credentials or host details.
        print(f"Airflow heartbeat check failed: {type(exc).__name__}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
