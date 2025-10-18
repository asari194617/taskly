import os
import logging
import json
import urllib.request
import urllib.error
from typing import Any

# Initialize Sentry SDK if DSN is provided (works alongside Airflow Sentry provider)
try:
    import sentry_sdk
    dsn = os.environ.get("SENTRY_DSN")
    if dsn:
        sentry_sdk.init(
            dsn=dsn,
            environment=os.environ.get("SENTRY_ENVIRONMENT", os.environ.get("AIRFLOW__CORE__ENVIRONMENT", "development")),
            traces_sample_rate=float(os.environ.get("SENTRY_TRACES_SAMPLE_RATE", "0.0")),
        )
except Exception:
    # Avoid breaking DAG import on Sentry issues
    pass

# Discord helper via provider
def discord_notify(task_id: str, message: str, webhook_conn_id: str = "discord_webhook"):
    try:
        from airflow.providers.discord.operators.discord_webhook import DiscordWebhookOperator
        return DiscordWebhookOperator(
            task_id=task_id,
            message=message,
            http_conn_id=webhook_conn_id,
        )
    except Exception as e:
        # Fallback to no-op BashOperator if provider not present
        from airflow.operators.empty import EmptyOperator
        return EmptyOperator(task_id=task_id)


def _send_discord_message(message: str) -> None:
    url = os.environ.get("DISCORD_WEBHOOK_URL")
    if url:
        url = url.strip().strip('"').strip("'")
    token = os.environ.get("DISCORD_WEBHOOK_TOKEN")
    try:
        from airflow.providers.discord.hooks.discord_webhook import DiscordWebhookHook
        # If URL present, derive webhook_token as '<id>/<token>' and try provider first
        if url:
            logging.getLogger(__name__).info("Sending Discord message via DISCORD_WEBHOOK_URL (provider)")
            token_path = None
            if "/api/webhooks/" in url:
                token_path = url.split("/api/webhooks/")[-1].strip("/")
            if token_path:
                try:
                    DiscordWebhookHook(webhook_token=token_path).send(message)
                    return
                except Exception:
                    logging.getLogger(__name__).info("Provider send via token_path failed, falling back to direct POST")
            # Fallback to direct POST with a User-Agent header
            data = json.dumps({"content": message}).encode("utf-8")
            headers = {"Content-Type": "application/json", "User-Agent": "taskly-airflow/1.0"}
            req = urllib.request.Request(url, data=data, headers=headers, method="POST")
            try:
                with urllib.request.urlopen(req, timeout=10) as resp:
                    if resp.status >= 300:
                        raise RuntimeError(f"Discord webhook returned status {resp.status}")
            except urllib.error.HTTPError as e:
                try:
                    body = e.read().decode("utf-8", errors="ignore")
                except Exception:
                    body = ""
                logging.getLogger(__name__).warning("Discord POST failed: %s %s", e, body)
                raise
            return

        if token:
            logging.getLogger(__name__).info("Sending Discord message via token env")
            DiscordWebhookHook(webhook_token=token).send(message)
            return
    # Fallback to Airflow connection if provided; default ids are common conventions
        conn_id = os.environ.get("DISCORD_CONN_ID", "discord_webhook")
        logging.getLogger(__name__).info("Sending Discord message via Airflow connection %s", conn_id)
        DiscordWebhookHook(http_conn_id=conn_id).send(message)
    except Exception:
        # Avoid raising inside callbacks
        logging.getLogger(__name__).warning("Discord notification failed (suppressed)", exc_info=True)


def discord_failure_callback(context: dict[str, Any]):
    """Airflow on_failure_callback to send a Discord message when task/DAG fails.
    Requires apache-airflow-providers-discord and DISCORD_WEBHOOK_TOKEN env var.
    """
    try:
        ti = context.get("ti")
        dag_id = context.get("dag" ).dag_id if context.get("dag") else "unknown"
        task_id = ti.task_id if ti else "unknown"
        run_id = context.get("run_id", "")
        exception = context.get("exception")
        message = f":rotating_light: Airflow failure in {dag_id}.{task_id} (run_id={run_id})\nError: {exception}"
        _send_discord_message(message)
    except Exception:
        logging.getLogger(__name__).warning("discord_failure_callback error (suppressed)", exc_info=True)


def discord_success_callback(context: dict[str, Any]):
    """Airflow on_success_callback to send a Discord message when DAG succeeds.
    Sends a summary regardless of task-level success notifications.
    """
    try:
        dag = context.get("dag")
        dag_id = dag.dag_id if dag else "unknown"
        run_id = context.get("run_id", "")
        message = f"✅ Airflow DAG succeeded: {dag_id} (run_id={run_id})"
        _send_discord_message(message)
    except Exception:
        logging.getLogger(__name__).warning("discord_success_callback error (suppressed)", exc_info=True)
