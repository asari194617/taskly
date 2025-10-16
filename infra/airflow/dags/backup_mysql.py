from datetime import datetime
from airflow import DAG
from airflow.operators.bash import BashOperator

with DAG(
    dag_id="backup_mysql",
    description="Taskly MySQL backup via FTP",
    schedule_interval="0 18 * * *",  # 03:00 JST (UTC 18:00)
    start_date=datetime(2024, 1, 1),
    catchup=False,
    tags=["taskly"],
) as dag:

    run_backup = BashOperator(
        task_id="run_backup",
        bash_command="cd /workspace/batches/backup-mysql && go run .",
        env={
            "DB_HOST": "${DB_HOST}",
            "DB_PORT": "${DB_PORT}",
            "DB_USER": "${DB_USER}",
            "DB_PASSWORD": "${DB_PASSWORD}",
            "DB_NAME": "${DB_NAME}",
            "FTP_HOST": "${FTP_HOST}",
            "FTP_PORT": "${FTP_PORT}",
            "FTP_USER": "${FTP_USER}",
            "FTP_PASSWORD": "${FTP_PASSWORD}",
            "FTP_DIR": "${FTP_DIR}",
        },
    )
