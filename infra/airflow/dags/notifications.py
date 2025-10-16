from datetime import datetime, time
from airflow import DAG
from airflow.operators.bash import BashOperator

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
) as dag:

    queue_manager = BashOperator(
        task_id="queue_manager",
        bash_command=build_cmd("notifications/que-manager"),
        env={
            "DB_HOST": "${DB_HOST}",
            "DB_PORT": "${DB_PORT}",
            "DB_USER": "${DB_USER}",
            "DB_PASSWORD": "${DB_PASSWORD}",
            "DB_NAME": "${DB_NAME}",
            "TIMEZONE": "Asia/Tokyo",
        },
    )

    email_delivery = BashOperator(
        task_id="email_delivery",
        bash_command=build_cmd("notifications/delivery-channels/email"),
        env={
            "DB_HOST": "${DB_HOST}",
            "DB_PORT": "${DB_PORT}",
            "DB_USER": "${DB_USER}",
            "DB_PASSWORD": "${DB_PASSWORD}",
            "DB_NAME": "${DB_NAME}",
            "SMTP_HOST": "${SMTP_HOST}",
            "SMTP_PORT": "${SMTP_PORT}",
            "SMTP_FROM": "${SMTP_FROM}",
            "SMTP_PASSWORD": "${SMTP_PASSWORD}",
        },
    )

    queue_manager >> email_delivery
