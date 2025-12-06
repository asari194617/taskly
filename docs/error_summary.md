# DagRunNotFound: DagRun for notifications with run_id or execution_date of 'manual_1765013373' not found

**Issue ID:** 7091795735
**Project:** go-fiber
**Date:** 12/6/2025, 6:29:35 PM
## Issue Summary
Missing DagRun for Manual Task Execution
**What's wrong:** **DagRunNotFound** for DAG **notifications** using **run_id** 'manual_1765013373'.
**In the trace:** The error occurred during a **CLI task run** command, attempting to get a **Task Instance (TI)**.
**Possible cause:** The **manual run ID** was likely used without ensuring the corresponding **DagRun** was created first, possibly due to a race condition or incorrect manual trigger.

## Tags

- **environment:** development
- **handled:** no
- **level:** error
- **mechanism:** excepthook
- **runtime:** CPython 3.11.9
- **runtime.name:** CPython
- **server_name:** ec5b5e82b898

## Exception

### Exception 1
**Type:** DagRunNotFound
**Value:** DagRun for notifications with run_id or execution_date of 'manual_1765013373' not found

#### Stacktrace

```
 _get_dag_run in airflow/cli/commands/task_command.py [Line 128] (Not in app)
        if execution_date:
            dag_run = dag.get_dagrun(execution_date=execution_date, session=session)
        if dag_run:
            return dag_run, False
        elif not create_if_necessary:
            raise DagRunNotFound(  <-- SUSPECT LINE
                f"DagRun for {dag.dag_id} with run_id or execution_date "
                f"of {exec_date_or_run_id!r} not found"
            )

    if execution_date is not None:
---
Variable values:
{
  "create_if_necessary": "False",
  "dag": "<DAG: notifications>",
  "dag_run": "None",
  "exec_date_or_run_id": "'manual_1765013373'",
  "execution_date": "None",
  "session": "[Filtered]"
}

=======
 _get_ti in airflow/cli/commands/task_command.py [Line 179] (Not in app)
    if needs_expansion(task):
        if map_index < 0:
            raise RuntimeError("No map_index passed to mapped task")
    elif map_index >= 0:
        raise RuntimeError("map_index passed to non-mapped task")
    dag_run, dr_created = _get_dag_run(  <-- SUSPECT LINE
        dag=dag,
        exec_date_or_run_id=exec_date_or_run_id,
        create_if_necessary=create_if_necessary,
        session=session,
    )
---
Variable values:
{
  "create_if_necessary": "False",
  "dag": "<DAG: notifications>",
  "exec_date_or_run_id": "'manual_1765013373'",
  "map_index": "-1",
  "pool": "None",
  "session": "[Filtered]",
  "task": "<Task(BashOperator): email_delivery>"
}

=======
 wrapper in airflow/utils/session.py [Line 79] (Not in app)
    def wrapper(*args, **kwargs) -> RT:
        if "session" in kwargs or session_args_idx < len(args):
            return func(*args, **kwargs)
        else:
            with create_session() as session:
                return func(*args, session=session, **kwargs)  <-- SUSPECT LINE

    return wrapper


# A fake session to use in functions decorated by provide_session. This allows
---
Variable values:
{
  "args": [
    "<Task(BashOperator): email_delivery>",
    "-1"
  ],
  "func": "<function _get_ti at 0xffffa8d5e840>",
  "kwargs": {
    "exec_date_or_run_id": "'manual_1765013373'",
    "pool": "None"
  },
  "session": "[Filtered]",
  "session_args_idx": "5"
}

=======
 task_run in airflow/cli/commands/task_command.py [Line 421] (Not in app)
    elif not dag:
        _dag = get_dag(args.subdir, args.dag_id, args.read_from_db)
    else:
        _dag = dag
    task = _dag.get_task(task_id=args.task_id)
    ti, _ = _get_ti(task, args.map_index, exec_date_or_run_id=args.execution_date_or_run_id, pool=args.pool)  <-- SUSPECT LINE
    ti.init_run_context(raw=args.raw)

    hostname = get_hostname()

    log.info("Running %s on host %s", ti, hostname)
---
Variable values:
{
  "_dag": "<DAG: notifications>",
  "args": "Namespace(subcommand='run', dag_id='notifications', task_id='email_delivery', execution_date_or_run_id='manual_1765013373', cfg_path=None, depends_on_past='check', force=False, ignore_all_dependencies=False, ignore_dependencies=False, ignore_depends_on_past=False, interactive=False, job_id=None, local=True, map_index=-1, mark_success=False, shut_down_logging=True, pickle=None, pool=None, raw=False, read_from_db=False, ship_dag=False, subdir='DAGS_FOLDER/notifications.py', verbose=False, func=<function lazy_load_command.<locals>.command at 0xffffabb2f7e0>)",
  "dag": "None",
  "task": "<Task(BashOperator): email_delivery>"
}

=======
 wrapper in airflow/utils/cli.py [Line 114] (Not in app)
                    from airflow.utils.db import check_and_run_migrations, synchronize_log_template

                    if conf.getboolean("database", "check_migrations"):
                        check_and_run_migrations()
                    synchronize_log_template()
                return f(*args, **kwargs)  <-- SUSPECT LINE
            except Exception as e:
                metrics["error"] = e
                raise
            finally:
                metrics["end_datetime"] = timezone.utcnow()
---
Variable values:
{
  "args": [
    "Namespace(subcommand='run', dag_id='notifications', task_id='email_delivery', execution_date_or_run_id='manual_1765013373', cfg_path=None, depends_on_past='check', force=False, ignore_all_dependencies=False, ignore_dependencies=False, ignore_depends_on_past=False, interactive=False, job_id=None, local=True, map_index=-1, mark_success=False, shut_down_logging=True, pickle=None, pool=None, raw=False, read_from_db=False, ship_dag=False, subdir='DAGS_FOLDER/notifications.py', verbose=False, func=<function lazy_load_command.<locals>.command at 0xffffabb2f7e0>)"
  ],
  "check_db": "False",
  "f": "<function task_run at 0xffffa8d5f060>",
  "kwargs": {},
  "metrics": {
    "dag_id": "'notifications'",
    "end_datetime": "datetime.datetime(2025, 12, 6, 9, 29, 35, 731592, tzinfo=Timezone('UTC'))",
    "error": "DagRunNotFound(\"DagRun for notifications with run_id or execution_date of 'manual_1765013373' not found\")",
    "execution_date": "None",
    "full_command": "\"['/home/airflow/.local/bin/airflow', 'tasks', 'run', 'notifications', 'email_delivery', 'manual_1765013373', '--local', '--subdir', 'DAGS_FOLDER/notifications.py']\"",
    "host_name": "'ec5b5e82b898'",
    "start_datetime": "datetime.datetime(2025, 12, 6, 9, 29, 34, 816151, tzinfo=Timezone('UTC'))",
    "sub_command": "'task_run'",
    "task_id": "'email_delivery'",
    "user": "'airflow'"
  },
  "root_logger": "<RootLogger root (INFO)>",
  "verbose": "False"
}

=======
 command in airflow/cli/cli_config.py [Line 49] (Not in app)
    """Create a lazy loader for command."""
    _, _, name = import_path.rpartition(".")

    def command(*args, **kwargs):
        func = import_string(import_path)
        return func(*args, **kwargs)  <-- SUSPECT LINE

    command.__name__ = name

    return command

---
Variable values:
{
  "args": [
    "Namespace(subcommand='run', dag_id='notifications', task_id='email_delivery', execution_date_or_run_id='manual_1765013373', cfg_path=None, depends_on_past='check', force=False, ignore_all_dependencies=False, ignore_dependencies=False, ignore_depends_on_past=False, interactive=False, job_id=None, local=True, map_index=-1, mark_success=False, shut_down_logging=True, pickle=None, pool=None, raw=False, read_from_db=False, ship_dag=False, subdir='DAGS_FOLDER/notifications.py', verbose=False, func=<function lazy_load_command.<locals>.command at 0xffffabb2f7e0>)"
  ],
  "func": "<function task_run at 0xffffa8d5f100>",
  "import_path": "'airflow.cli.commands.task_command.task_run'",
  "kwargs": {}
}

=======
 main in airflow/__main__.py [Line 58] (Not in app)
        from airflow.configuration import write_default_airflow_configuration_if_needed

        conf = write_default_airflow_configuration_if_needed()
        if args.subcommand in ["webserver", "internal-api", "worker"]:
            write_webserver_configuration_if_needed(conf)
    args.func(args)  <-- SUSPECT LINE


if __name__ == "__main__":
    main()
---
Variable values:
{
  "args": "Namespace(subcommand='run', dag_id='notifications', task_id='email_delivery', execution_date_or_run_id='manual_1765013373', cfg_path=None, depends_on_past='check', force=False, ignore_all_dependencies=False, ignore_dependencies=False, ignore_depends_on_past=False, interactive=False, job_id=None, local=True, map_index=-1, mark_success=False, shut_down_logging=True, pickle=None, pool=None, raw=False, read_from_db=False, ship_dag=False, subdir='DAGS_FOLDER/notifications.py', verbose=False, func=<function lazy_load_command.<locals>.command at 0xffffabb2f7e0>)",
  "conf": {
    "DEFAULT": {},
    "api": {
      "access_control_allow_headers": "''",
      "access_control_allow_methods": "''",
      "access_control_allow_origins": "''",
      "auth_backends": "[Filtered]",
      "enable_experimental_api": "'False'",
      "enable_xcom_deserialize_support": "'False'",
      "fallback_page_limit": "'100'",
      "google_key_path": "''",
      "google_oauth2_audience": "[Filtered]",
      "maximum_page_limit": "'100'"
    },
    "cli": {
      "api_client": "'airflow.api.client.local_client'",
      "endpoint_url": "'http://localhost:8080'"
    },
    "core": {
      "auth_manager": "[Filtered]",
      "dags_are_paused_at_creation": "'True'",
      "dags_folder": "'/opt/airflow/dags'",
      "default_timezone": "'utc'",
      "executor": "'SequentialExecutor'",
      "hostname_callable": "'airflow.utils.net.getfqdn'",
      "max_active_runs_per_dag": "'16'",
      "max_active_tasks_per_dag": "'16'",
      "might_contain_dag_callable": "'airflow.utils.file.might_contain_dag_via_default_heuristic'",
      "parallelism": "'32'"
    },
    "database": {
      "alembic_ini_file_path": "'alembic.ini'",
      "load_default_connections": "'True'",
      "sql_alchemy_conn": "'sqlite:////opt/airflow/airflow.db'",
      "sql_alchemy_max_overflow": "'10'",
      "sql_alchemy_pool_enabled": "'True'",
      "sql_alchemy_pool_pre_ping": "'True'",
      "sql_alchemy_pool_recycle": "'1800'",
      "sql_alchemy_pool_size": "'5'",
      "sql_alchemy_schema": "''",
      "sql_engine_encoding": "'utf-8'"
    },
    "debug": {
      "fail_fast": "'False'"
    },
    "lineage": {
      "backend": "''"
    },
    "logging": {
      "base_log_folder": "'/opt/airflow/logs'",
      "celery_logging_level": "''",
      "delete_local_logs": "'False'",
      "encrypt_s3_logs": "'False'",
      "google_key_path": "''",
      "logging_level": "'INFO'",
      "remote_base_log_folder": "''",
      "remote_log_conn_id": "''",
      "remote_logging": "'False'",
      "remote_task_handler_kwargs": "''"
    },
    "metrics": {
      "metrics_allow_list": "''",
      "metrics_block_list": "''",
      "metrics_use_pattern_match": "'False'",
      "stat_name_handler": "''",
      "statsd_datadog_enabled": "'False'",
      "statsd_datadog_tags": "''",
      "statsd_host": "'localhost'",
      "statsd_on": "'False'",
      "statsd_port": "'8125'",
      "statsd_prefix": "'airflow'"
    },
    "secrets": null
  },
  "parser": "DefaultHelpParser(prog='airflow', usage=None, description=None, formatter_class=<class 'airflow.cli.cli_parser.AirflowHelpFormatter'>, conflict_handler='error', add_help=True)",
  "write_default_airflow_configuration_if_needed": "<function write_default_airflow_configuration_if_needed at 0xffffad0b6520>"
}

=======
 <module> in airflow [Line 8] (Not in app)
import re
import sys
from airflow.__main__ import main
if __name__ == '__main__':
    sys.argv[0] = re.sub(r'(-script\.pyw|\.exe)?$', '', sys.argv[0])
    sys.exit(main())  <-- SUSPECT LINE
---
Variable values:
{
  "__annotations__": {},
  "__builtins__": "<module 'builtins' (built-in)>",
  "__cached__": "None",
  "__doc__": "None",
  "__file__": "'/home/airflow/.local/bin/airflow'",
  "__loader__": "<_frozen_importlib_external.SourceFileLoader object at 0xffffb06c9990>",
  "__name__": "'__main__'",
  "__package__": "None",
  "__spec__": "None",
  "re": "<module 're' from '/usr/local/lib/python3.11/re/__init__.py'>"
}

=======
```
