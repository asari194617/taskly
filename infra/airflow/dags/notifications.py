from datetime import datetime, time
from airflow.sdk import DAG
from airflow.providers.standard.operators.bash import BashOperator
import os


def discord_failure_callback(context):
    from common import discord_failure_callback as _discord_failure_callback

    _discord_failure_callback(context)


def discord_success_callback(context):
    from common import discord_success_callback as _discord_success_callback

    _discord_success_callback(context)

# Runs queue manager and delivery channels (email / webpush / discord)
# at 07:00 and 17:00 JST daily. The batch programs are built into the
# Airflow image and executed directly from /opt/airflow/bin.

def build_cmd(app):
    # Echo key envs for diagnostics, then run. For email channel, do SMTP reachability pre-check via openssl s_client.
    binary_name = app.replace("/", "-")
    base = (
        "set -euo pipefail; "
        "echo '--- NOTIFICATIONS ENV ---'; env | grep -E '^(SMTP_|EMAIL_COMPOSE_|APP_NAME|LOCALE|WEB_URL|VAPID_)' || true; "
    )
    if app == "notifications/delivery-channels/email":
        # Determine STARTTLS usage and port from env; default to starttls:587 for Resend/SendGrid.
        # Fail early with a clear log line if SMTP is unreachable.
        precheck = (
            "SMTP_HOST=\"${SMTP_HOST:-smtp.sendgrid.net}\"; "
            "SMTP_PORT=\"${SMTP_PORT:-587}\"; "
            "SMTP_TLS=\"${SMTP_TLS:-starttls}\"; "
            "echo \"SMTP precheck: host=$SMTP_HOST port=$SMTP_PORT tls=$SMTP_TLS\"; "
            "apk add --no-cache openssl >/dev/null 2>&1 || true; "
            "if [ \"$SMTP_TLS\" = \"starttls\" ]; then "
            "  if ! echo QUIT | openssl s_client -connect \"$SMTP_HOST:$SMTP_PORT\" -starttls smtp -crlf -quiet >/dev/null 2>&1; then "
            "    echo '\n[ERROR] SMTP到達不可: STARTTLSハンドシェイク失敗' >&2; exit 2; "
            "  fi; "
            "else "
            "  if ! echo QUIT | openssl s_client -connect \"$SMTP_HOST:$SMTP_PORT\" -crlf -quiet >/dev/null 2>&1; then "
            "    echo '\n[ERROR] SMTP到達不可: TLSなし接続失敗' >&2; exit 2; "
            "  fi; "
            "fi; "
        )
        return base + precheck + f"/opt/airflow/bin/{binary_name}"
    # Default path (discord, web-push, queue-manager, etc.)
    return base + f"/opt/airflow/bin/{binary_name}"

with DAG(
    dag_id="notifications",
    description="Taskly notifications queue and delivery",
    schedule="0 22,8 * * *",  # 07:00,17:00 JST (UTC-9/UTC? adjust as needed)
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
            "DB_HOST": os.environ.get("DB_HOST", os.environ.get("MYSQL_HOST", "db")),
            "DB_PORT": os.environ.get("DB_PORT", os.environ.get("MYSQL_PORT", "3306")),
            "DB_USER": os.environ.get("DB_USER", os.environ.get("MYSQL_USER", "taskly")),
            "DB_PASSWORD": os.environ.get("DB_PASSWORD", os.environ.get("MYSQL_PASSWORD", "taskly_password")),
            "DB_NAME": os.environ.get("DB_NAME", os.environ.get("MYSQL_DATABASE", "taskly")),
            "MYSQL_HOST": os.environ.get("MYSQL_HOST", os.environ.get("DB_HOST", "db")),
            "MYSQL_PORT": os.environ.get("MYSQL_PORT", os.environ.get("DB_PORT", "3306")),
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
            "DB_HOST": os.environ.get("DB_HOST", os.environ.get("MYSQL_HOST", "db")),
            "DB_PORT": os.environ.get("DB_PORT", os.environ.get("MYSQL_PORT", "3306")),
            "DB_USER": os.environ.get("DB_USER", os.environ.get("MYSQL_USER", "taskly")),
            "DB_PASSWORD": os.environ.get("DB_PASSWORD", os.environ.get("MYSQL_PASSWORD", "taskly_password")),
            "DB_NAME": os.environ.get("DB_NAME", os.environ.get("MYSQL_DATABASE", "taskly")),
            "MYSQL_HOST": os.environ.get("MYSQL_HOST", os.environ.get("DB_HOST", "db")),
            "MYSQL_PORT": os.environ.get("MYSQL_PORT", os.environ.get("DB_PORT", "3306")),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", "taskly_password"),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            "TIMEZONE": os.environ.get("TIMEZONE", "Asia/Tokyo"),
            "SMTP_HOST": os.environ.get("SMTP_HOST", ""),
            "SMTP_PORT": os.environ.get("SMTP_PORT", "25"),
            "SMTP_FROM": os.environ.get("SMTP_FROM", "noreply@taskly.local"),
            "SMTP_USER": os.environ.get("SMTP_USER", os.environ.get("SMTP_FROM", "noreply@taskly.local")),
            "SMTP_PASSWORD": os.environ.get("SMTP_PASSWORD", ""),
            "SMTP_TLS": os.environ.get("SMTP_TLS", "starttls"),
            "SMTP_TIMEOUT": os.environ.get("SMTP_TIMEOUT", "60"),
            # Some SMTP providers require auth only when password exists; pass through a flag if needed.
            "SMTP_USE_AUTH": os.environ.get("SMTP_USE_AUTH", "auto"),  # auto | always | never
            "EMAIL_COMPOSE_HTTP_ADDR": os.environ.get("EMAIL_COMPOSE_HTTP_ADDR", "http://email-compose:50052"),
            "APP_NAME": os.environ.get("APP_NAME", "Taskly"),
            "LOCALE": os.environ.get("LOCALE", "ja"),
        },
    )

    webpush_delivery = BashOperator(
        task_id="webpush_delivery",
        bash_command=build_cmd("notifications/delivery-channels/web-push"),
        env={
            # DB settings
            "DB_HOST": os.environ.get("DB_HOST", os.environ.get("MYSQL_HOST", "db")),
            "DB_PORT": os.environ.get("DB_PORT", os.environ.get("MYSQL_PORT", "3306")),
            "DB_USER": os.environ.get("DB_USER", os.environ.get("MYSQL_USER", "taskly")),
            "DB_PASSWORD": os.environ.get("DB_PASSWORD", os.environ.get("MYSQL_PASSWORD", "taskly_password")),
            "DB_NAME": os.environ.get("DB_NAME", os.environ.get("MYSQL_DATABASE", "taskly")),
            "MYSQL_HOST": os.environ.get("MYSQL_HOST", os.environ.get("DB_HOST", "db")),
            "MYSQL_PORT": os.environ.get("MYSQL_PORT", os.environ.get("DB_PORT", "3306")),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", "taskly_password"),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            "TIMEZONE": os.environ.get("TIMEZONE", "Asia/Tokyo"),
            # WebPush specific
            "WEB_URL": os.environ.get("WEB_URL", "https://app.taskly.local"),
            "VAPID_PUBLIC_KEY": os.environ.get("VAPID_PUBLIC_KEY", ""),
            "VAPID_PRIVATE_KEY": os.environ.get("VAPID_PRIVATE_KEY", ""),
            "VAPID_SUBJECT": os.environ.get("VAPID_SUBJECT", "mailto:noreply@taskly.local"),
        },
    )

    discord_delivery = BashOperator(
        task_id="discord_delivery",
        bash_command=build_cmd("notifications/delivery-channels/discord"),
        env={
            # Provide both DB_* and MYSQL_* for compatibility with different batch apps
            "DB_HOST": os.environ.get("DB_HOST", os.environ.get("MYSQL_HOST", "db")),
            "DB_PORT": os.environ.get("DB_PORT", os.environ.get("MYSQL_PORT", "3306")),
            "DB_USER": os.environ.get("DB_USER", os.environ.get("MYSQL_USER", "taskly")),
            "DB_PASSWORD": os.environ.get("DB_PASSWORD", os.environ.get("MYSQL_PASSWORD", "taskly_password")),
            "DB_NAME": os.environ.get("DB_NAME", os.environ.get("MYSQL_DATABASE", "taskly")),
            "MYSQL_HOST": os.environ.get("MYSQL_HOST", os.environ.get("DB_HOST", "db")),
            "MYSQL_PORT": os.environ.get("MYSQL_PORT", os.environ.get("DB_PORT", "3306")),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", "taskly_password"),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            "TIMEZONE": "Asia/Tokyo",
            "DISCORD_WEBHOOK_URL": os.environ.get("DISCORD_WEBHOOK_URL", ""),
        },
    )

    queue_manager.set_downstream([email_delivery, webpush_delivery, discord_delivery])
