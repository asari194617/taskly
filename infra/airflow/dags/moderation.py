from datetime import datetime, timedelta
import os
from airflow import DAG
from airflow.operators.bash import BashOperator
from common import discord_failure_callback, discord_success_callback
import os

# Run daily at 02:15 local time (Airflow default UTC unless tz configured)
default_args = {
    "owner": "taskly",
    "depends_on_past": False,
    "retries": 0,
}

with DAG(
    dag_id="moderation_daily",
    default_args=default_args,
    description="Daily moderation of tasks using Gemini API",
    schedule_interval="15 2 * * *",
    start_date=datetime(2024, 1, 1),
    catchup=False,
    on_failure_callback=discord_failure_callback,
    on_success_callback=discord_success_callback,
    tags=["moderation"],
) as dag:

    run_moderation = BashOperator(
        task_id="run_moderation",
        bash_command="go run /workspace/batches/moderation/main.go",
        env={
            "GOPATH": os.environ.get("GOPATH", "/home/airflow/go"),
            "GOMODCACHE": os.environ.get("GOMODCACHE", "/home/airflow/go/pkg/mod"),
            "GOCACHE": os.environ.get("GOCACHE", "/home/airflow/.cache/go-build"),
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", "taskly_password"),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            "GEMINI_API_KEY": os.environ.get("GEMINI_API_KEY", ""),
            "SMTP_HOST": os.environ.get("SMTP_HOST", "postfix"),
            "SMTP_PORT": os.environ.get("SMTP_PORT", "25"),
            "SMTP_FROM": os.environ.get("SMTP_FROM", "noreply@taskly.local"),
            "SMTP_USER": os.environ.get("SMTP_USER", os.environ.get("SMTP_FROM", "noreply@taskly.local")),
            "SMTP_PASSWORD": os.environ.get("SMTP_PASSWORD", ""),
            "DISCORD_WEBHOOK_URL": os.environ.get("DISCORD_WEBHOOK_URL", ""),
            # Email composer service and admin review URL
            "EMAIL_COMPOSE_HTTP_ADDR": os.environ.get("EMAIL_COMPOSE_HTTP_ADDR", "http://email-compose:50052"),
            "ADMIN_REVIEW_URL": (os.environ.get("WEB_URL", "http://localhost:3063").rstrip("/") + "/admin/moderation"),
        },
    )

    # Single-task DAG
