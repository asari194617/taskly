from __future__ import annotations

import textwrap
from datetime import datetime, timedelta

from airflow.providers.standard.operators.bash import BashOperator
from airflow.sdk import DAG


with DAG(
    dag_id="tutorial_example",
    default_args={
        "depends_on_past": False,
        "retries": 1,
        "retry_delay": timedelta(minutes=5),
    },
    description="A simple tutorial DAG",
    schedule=timedelta(days=1),
    start_date=datetime(2021, 1, 1),
    catchup=False,
    tags=["example"],
) as dag:
    t1 = BashOperator(
        task_id="print_date",
        bash_command="date",
    )

    t2 = BashOperator(
        task_id="sleep",
        depends_on_past=False,
        bash_command="sleep 5",
        retries=3,
    )

    dag.doc_md = textwrap.dedent(
        """\
        #### Tutorial Example DAG

        This DAG is a small smoke test based on the official Airflow tutorial.
        It uses two BashOperator tasks and keeps the runtime light.
        """
    )

    templated_command = textwrap.dedent(
        """
        {% for i in range(5) %}
            echo "{{ ds }}"
            echo "{{ macros.ds_add(ds, 7) }}"
        {% endfor %}
        """
    )

    t3 = BashOperator(
        task_id="templated",
        depends_on_past=False,
        bash_command=templated_command,
    )

    t1 >> [t2, t3]