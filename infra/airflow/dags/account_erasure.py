"""Daily closed-account erasure, before the application database backup."""

from datetime import datetime, timedelta
import os

from airflow.sdk import DAG
from airflow.providers.standard.operators.bash import BashOperator
from common import discord_failure_callback, discord_success_callback

with DAG(
    dag_id="account_erasure",
    description="Erase closed accounts whose 30-day retention has expired",
    schedule="30 17 * * *",  # 02:30 JST
    start_date=datetime(2024, 1, 1),
    catchup=False,
    default_args={"retries": 2, "retry_delay": timedelta(minutes=10)},
    tags=["taskly", "privacy"],
    on_failure_callback=discord_failure_callback,
    on_success_callback=discord_success_callback,
) as dag:
    erase_accounts = BashOperator(
        task_id="erase_accounts",
        bash_command="python /opt/airflow/dags/account_erasure_worker.py",
        env={
            "PATH": os.environ.get("PATH", "/usr/local/bin:/usr/bin:/bin"),
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", ""),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
        },
    )
