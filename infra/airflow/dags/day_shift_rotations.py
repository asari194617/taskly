from datetime import datetime
from airflow import DAG
from airflow.operators.bash import BashOperator

with DAG(
    dag_id="day_shift_rotations",
    description="Generate day-shift rotations",
    schedule_interval="0 18 * * *",  # 03:00 JST
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["taskly"],
) as dag:

    generate = BashOperator(
        task_id="generate_rotations",
        bash_command="cd /workspace/batches/day-shift-rotations && go run .",
        env={
            "DB_HOST": "${DB_HOST}",
            "DB_PORT": "${DB_PORT}",
            "DB_USER": "${DB_USER}",
            "DB_PASSWORD": "${DB_PASSWORD}",
            "DB_NAME": "${DB_NAME}",
        },
    )
