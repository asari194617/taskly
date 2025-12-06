import os
import logging
import json
import urllib.request
import urllib.error
from datetime import datetime, timezone
from typing import Any

__all__ = [
    "discord_failure_callback",
    "discord_success_callback",
    "discord_notify",
]

# Load environment variables from mounted .env if available
try:
    from dotenv import load_dotenv
    env_path = "/opt/airflow/.env"
    if os.path.exists(env_path):
        load_dotenv(env_path, override=False)
    else:
        load_dotenv(override=False)
except Exception:
    # Do not fail DAG import if python-dotenv is missing
    pass

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
        data = json.dumps({"content": message}).encode("utf-8")
        headers = {"Content-Type": "application/json", "User-Agent": "taskly-airflow/1.0"}
        targets = []
        if url:
            targets.append(url)
        if token:
            targets.append(f"https://discord.com/api/webhooks/{token.strip()}")
        for target in targets:
            try:
                req = urllib.request.Request(target, data=data, headers=headers, method="POST")
                with urllib.request.urlopen(req, timeout=10) as resp:
                    if resp.status >= 300:
                        raise RuntimeError(f"Discord webhook returned status {resp.status}")
                return
            except urllib.error.HTTPError as e:
                try:
                    body = e.read().decode("utf-8", errors="ignore")
                except Exception:
                    body = ""
                logging.getLogger(__name__).warning("Discord POST failed to %s: %s %s", target, e, body)
                # try next target
                continue
        if not targets:
            logging.getLogger(__name__).warning("No Discord webhook configured (DISCORD_WEBHOOK_URL or DISCORD_WEBHOOK_TOKEN)")
    except Exception:
        # Avoid raising inside callbacks
        logging.getLogger(__name__).warning("Discord notification failed (suppressed)", exc_info=True)


def _build_embed_payload(title: str, fields: list[dict[str, str]], *, ts_iso: str | None = None, color: int | None = None) -> dict:
    if not ts_iso:
        ts_iso = datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
    if color is None:
        # Defaults; override via env if desired
        if "fail" in title.lower():
            color = int(os.environ.get("DISCORD_EMBED_COLOR_FAILURE", "15158332"))  # red-ish
        else:
            # default green (0x2ecc71)
            color = int(os.environ.get("DISCORD_EMBED_COLOR_SUCCESS", "3066993"))
    author_url = os.environ.get("WEB_URL", "")
    author_icon = os.environ.get("DISCORD_AUTHOR_ICON_URL", "https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/png/apache-airflow.png")
    embed = {
        "title": title,
        "timestamp": ts_iso,
        "color": color,
        "author": {
            "name": "Apache Airflow",
            "url": author_url,
            "icon_url": author_icon,
        },
        "fields": fields,
    }
    return {"content": "", "tts": False, "embeds": [embed], "components": []}


def _send_discord_embed(title: str, fields: list[dict[str, str]], ts: str | None = None) -> None:
    url = os.environ.get("DISCORD_WEBHOOK_URL")
    if url:
        url = url.strip().strip('"').strip("'")
    token = os.environ.get("DISCORD_WEBHOOK_TOKEN")
    payload = _build_embed_payload(title, fields, ts_iso=ts)
    data = json.dumps(payload).encode("utf-8")
    headers = {"Content-Type": "application/json", "User-Agent": "taskly-airflow/1.0"}
    targets: list[str] = []
    if url:
        targets.append(url)
    if token:
        targets.append(f"https://discord.com/api/webhooks/{token.strip()}")
    if not targets:
        logging.getLogger(__name__).warning("No Discord webhook configured for embeds")
        return
    last_error = None
    for target in targets:
        try:
            req = urllib.request.Request(target, data=data, headers=headers, method="POST")
            with urllib.request.urlopen(req, timeout=10) as resp:
                if resp.status >= 300:
                    raise RuntimeError(f"Discord webhook returned status {resp.status}")
            return
        except urllib.error.HTTPError as e:
            try:
                body = e.read().decode("utf-8", errors="ignore")
            except Exception:
                body = ""
            logging.getLogger(__name__).warning("Discord embed POST failed to %s: %s %s", target, e, body)
            last_error = e
            continue
        except Exception as e:  # network/other
            logging.getLogger(__name__).warning("Discord embed POST error to %s (suppressed): %s", target, e, exc_info=True)
            last_error = e
            continue
    # Fallback to plain text if embeds couldn't be sent
    try:
        summary = title + "\n" + "\n".join(f"- {f.get('name')}: {f.get('value')}" for f in fields)
        _send_discord_message(summary)
    except Exception:
        logging.getLogger(__name__).warning("Discord plain text fallback also failed (suppressed)", exc_info=True)


def discord_failure_callback(context: dict):
    """Airflow on_failure_callback to send a Discord message when task/DAG fails.
    Works with DISCORD_WEBHOOK_URL or DISCORD_WEBHOOK_TOKEN.
    """
    try:
        ti = context.get("ti")
        dag = context.get("dag")
        dag_id = dag.dag_id if dag else "unknown"
        task_id = ti.task_id if ti else "unknown"
        run_id = context.get("run_id", "")
        exception = context.get("exception")
        ts = context.get("ts")
        fields = [
            {"name": "DAG", "value": dag_id},
            {"name": "task_id", "value": task_id},
            {"name": "run_id", "value": str(run_id)},
        ]
        if exception:
            fields.append({"name": "error", "value": str(exception)})
        _send_discord_embed("Airflow DAG failed", fields, ts=ts)
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
        ts = context.get("ts")
        fields = [
            {"name": "DAG", "value": dag_id},
            {"name": "run_id", "value": str(run_id)},
        ]
        _send_discord_embed("Airflow DAG succeeded", fields, ts=ts)
    except Exception:
        logging.getLogger(__name__).warning("discord_success_callback error (suppressed)", exc_info=True)
