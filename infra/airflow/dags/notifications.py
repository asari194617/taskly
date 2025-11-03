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
            "GOPATH": os.environ.get("GOPATH", "/home/airflow/go"),
            "GOMODCACHE": os.environ.get("GOMODCACHE", "/home/airflow/go/pkg/mod"),
            "GOCACHE": os.environ.get("GOCACHE", "/home/airflow/.cache/go-build"),
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", "taskly_password"),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            "TIMEZONE": "Asia/Tokyo",
        },
    )

    email_delivery = BashOperator(
        task_id="email_delivery",
        bash_command=build_cmd("notifications/delivery-channels/email"),
        env={
            "GOPATH": os.environ.get("GOPATH", "/home/airflow/go"),
            "GOMODCACHE": os.environ.get("GOMODCACHE", "/home/airflow/go/pkg/mod"),
            "GOCACHE": os.environ.get("GOCACHE", "/home/airflow/.cache/go-build"),
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", "taskly_password"),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            "TIMEZONE": os.environ.get("TIMEZONE", "Asia/Tokyo"),
            "SMTP_HOST": os.environ.get("SMTP_HOST", "postfix"),
            "SMTP_PORT": os.environ.get("SMTP_PORT", "25"),
            "SMTP_FROM": os.environ.get("SMTP_FROM", "noreply@taskly.local"),
            "SMTP_USER": os.environ.get("SMTP_USER", os.environ.get("SMTP_FROM", "noreply@taskly.local")),
            "SMTP_PASSWORD": os.environ.get("SMTP_PASSWORD", ""),
            "SMTP_TLS": os.environ.get("SMTP_TLS", "starttls"),
            "EMAIL_COMPOSE_HTTP_ADDR": os.environ.get("EMAIL_COMPOSE_HTTP_ADDR", "http://email-compose:50052"),
            "APP_NAME": os.environ.get("APP_NAME", "Taskly"),
            "LOCALE": os.environ.get("LOCALE", "ja"),
        },
    )

    discord_delivery = BashOperator(
        task_id="discord_delivery",
        bash_command=build_cmd("notifications/delivery-channels/discord"),
        env={
            "GOPATH": os.environ.get("GOPATH", "/home/airflow/go"),
            "GOMODCACHE": os.environ.get("GOMODCACHE", "/home/airflow/go/pkg/mod"),
            "GOCACHE": os.environ.get("GOCACHE", "/home/airflow/.cache/go-build"),
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", "taskly_password"),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            "TIMEZONE": "Asia/Tokyo",
            "DISCORD_WEBHOOK_URL": os.environ.get("DISCORD_WEBHOOK_URL", ""),
        },
    )

    queue_manager.set_downstream([email_delivery, discord_delivery])
