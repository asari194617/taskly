from datetime import datetime
from airflow import DAG
from common import discord_failure_callback, discord_success_callback
from airflow.operators.bash import BashOperator
import os

with DAG(
    dag_id="backup_mysql",
    description="Taskly MySQL backup via FTP",
    schedule_interval="0 18 * * *",  # 03:00 JST (UTC 18:00)
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["taskly"],
    on_failure_callback=discord_failure_callback,
    on_success_callback=discord_success_callback,
) as dag:

    run_backup = BashOperator(
        task_id="run_backup",
        bash_command="cd /workspace/batches/backup-mysql && go run .",
        env={
            # Ensure Go can resolve module cache during task runs
            "GOPATH": "/home/airflow/go",
            "GOMODCACHE": "/home/airflow/go/pkg/mod",
            "GOCACHE": "/home/airflow/.cache/go-build",
            "PATH": "/usr/local/go/bin:/home/airflow/go/bin:/usr/lib/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            # DB connection for mysqldump (explicit constants to avoid localhost fallback)
            "DB_HOST": "db",
            "DB_PORT": "3306",
            "DB_USER": "taskly",
            "DB_PASSWORD": "taskly_password",
            "DB_NAME": "taskly",
            # Persist backups under airflow-data volume
            "BACKUP_DIR": "/opt/airflow/backups",
            # FTP credentials (forward from container env)
            "FTP_HOST": os.environ.get("FTP_HOST", ""),
            "FTP_PORT": os.environ.get("FTP_PORT", "21"),
            "FTP_USER": os.environ.get("FTP_USER", ""),
            "FTP_PASSWORD": os.environ.get("FTP_PASSWORD", ""),
            "FTP_DIR": os.environ.get("FTP_DIR", "/backups"),
        },
    )
