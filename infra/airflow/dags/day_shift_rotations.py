from datetime import datetime
from airflow import DAG
from common import discord_failure_callback, discord_success_callback
from airflow.operators.bash import BashOperator
import os

with DAG(
    dag_id="day_shift_rotations",
    description="Generate day-shift rotations",
    schedule_interval="0 18 * * *",  # 03:00 JST
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["taskly"],
    on_failure_callback=discord_failure_callback,
    on_success_callback=discord_success_callback,
) as dag:

    generate = BashOperator(
        task_id="generate_rotations",
        bash_command="cd /workspace/batches/day-shift-rotations && go run .",
        env={
            "DB_HOST": os.environ.get("DB_HOST", "db"),
            "DB_PORT": os.environ.get("DB_PORT", "3306"),
            "DB_USER": os.environ.get("DB_USER", "taskly"),
            "DB_PASSWORD": os.environ.get("DB_PASSWORD", "taskly_password"),
            "DB_NAME": os.environ.get("DB_NAME", "taskly"),
        },
    )
