from datetime import datetime, time
from airflow import DAG
from common import discord_failure_callback, discord_success_callback
from airflow.operators.bash import BashOperator
import os

# Runs queue manager and email delivery at 07:00 and 17:00 JST daily
# Assumes docker container context where `go run` works, or built binaries available.
# Adjust cmd paths to your environment.

def build_cmd(app):
    return f"cd /workspace/batches/{app} && go run ."

with DAG(
    dag_id="notifications",
    description="Taskly notifications queue and delivery",
    schedule_interval="0 22,8 * * *",  # 07:00,17:00 JST (UTC-9/UTC? adjust as needed)
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["taskly"],
    on_failure_callback=discord_failure_callback,
    on_success_callback=discord_success_callback,
) as dag:

    queue_manager = BashOperator(
        task_id="queue_manager",
        bash_command=build_cmd("notifications/que-manager"),
        env={
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "DB_USER": os.environ.get("DB_USER", "taskly"),
            "DB_PASSWORD": os.environ.get("DB_PASSWORD", "taskly_password"),
            "DB_NAME": os.environ.get("DB_NAME", "taskly"),
            "TIMEZONE": "Asia/Tokyo",
        },
    )

    email_delivery = BashOperator(
        task_id="email_delivery",
        bash_command=build_cmd("notifications/delivery-channels/email"),
        env={
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "DB_USER": os.environ.get("DB_USER", "taskly"),
            "DB_PASSWORD": os.environ.get("DB_PASSWORD", "taskly_password"),
            "DB_NAME": os.environ.get("DB_NAME", "taskly"),
            "SMTP_HOST": os.environ.get("SMTP_HOST", "postfix"),
            "SMTP_PORT": os.environ.get("SMTP_PORT", "25"),
            "SMTP_FROM": os.environ.get("SMTP_FROM", "noreply@taskly.local"),
            "SMTP_PASSWORD": os.environ.get("SMTP_PASSWORD", ""),
        },
    )

    queue_manager >> email_delivery
