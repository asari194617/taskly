from datetime import datetime, timedelta
from airflow.sdk import DAG
from common import discord_failure_callback, discord_success_callback
from airflow.providers.standard.operators.bash import BashOperator
import os

with DAG(
    dag_id="backup_mysql",
    description="Encrypted Taskly MySQL backup via FTPS",
    schedule="0 18 * * *",  # 03:00 JST (UTC 18:00)
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["taskly"],
    on_failure_callback=discord_failure_callback,
    on_success_callback=discord_success_callback,
    default_args={"retries": 2, "retry_delay": timedelta(minutes=10)},
) as dag:

    run_backup = BashOperator(
        task_id="run_backup",
        bash_command="/opt/airflow/bin/backup-mysql",
        env={
            # DB connection for mysqldump (explicit constants to avoid localhost fallback)
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", ""),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            # Persist backups in the dedicated airflow-backups volume.
            "BACKUP_DIR": "/opt/airflow/backups",
            "BACKUP_AGE_RECIPIENT": os.environ.get("BACKUP_AGE_RECIPIENT", ""),
            "BACKUP_RETENTION_DAYS": os.environ.get("BACKUP_RETENTION_DAYS", "14"),
            # Explicit FTPS credentials (forward from container env)
            "FTP_HOST": os.environ.get("FTP_HOST", ""),
            "FTP_PORT": os.environ.get("FTP_PORT", "21"),
            "FTP_USER": os.environ.get("FTP_USER", ""),
            "FTP_PASSWORD": os.environ.get("FTP_PASSWORD", ""),
            "FTP_DIR": os.environ.get("FTP_DIR", "/backups"),
        },
    )
