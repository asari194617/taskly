"""Erase closed accounts after their retention period.

Run only after the account_erasure_holds table and deleted_at column exist.
Each account is processed in its own transaction; failures leave it due for retry.
"""

import os
from datetime import datetime, timedelta, timezone
from urllib import error, parse, request

import pymysql

RETENTION_DAYS = 30


class _NoRedirect(request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


_auth_opener = request.build_opener(_NoRedirect())


def erase_auth_user(user_id):
    if os.environ.get("AUTH_ACCOUNT_LIFECYCLE_EXTERNAL") != "1":
        return
    base_url = os.environ.get("AUTH_LIFECYCLE_SYNC_URL", "").strip().rstrip("/")
    token = os.environ.get("AUTH_INTERNAL_TOKEN", "")
    parsed = parse.urlparse(base_url)
    if (
        not base_url
        or not token
        or parsed.scheme not in ("http", "https")
        or not parsed.hostname
        or parsed.username is not None
        or parsed.password is not None
        or parsed.path not in ("", "/")
        or parsed.query
        or parsed.fragment
    ):
        raise RuntimeError("auth lifecycle synchronization is not configured safely")
    if os.environ.get("NODE_ENV", "").lower() == "production":
        private_http = parsed.scheme == "http" and parsed.hostname in ("auth", "localhost", "127.0.0.1")
        if (parsed.scheme != "https" and not private_http) or len(token) < 32:
            raise RuntimeError("auth lifecycle synchronization is not configured safely")
    req = request.Request(
        f"{base_url}/internal/users/{int(user_id)}",
        headers={"Authorization": f"Bearer {token}"},
        method="DELETE",
    )
    try:
        response = _auth_opener.open(req, timeout=5)
        status = response.status
        response.close()
    except error.HTTPError as exc:
        if exc.fp is not None:
            exc.close()
        raise RuntimeError(f"auth lifecycle synchronization returned status {exc.code}") from None
    except error.URLError:
        raise RuntimeError("auth lifecycle synchronization request failed") from None
    if status != 204:
        raise RuntimeError(f"auth lifecycle synchronization returned status {status}")


def erase_task_images(user_id):
    names = ("R2_ENDPOINT", "R2_BUCKET", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY")
    config = {name: os.environ.get(name, "").strip() for name in names}
    if not any(config.values()):
        return
    if not all(config.values()):
        raise RuntimeError("R2 task image settings are incomplete")

    import boto3

    client = boto3.client(
        "s3",
        endpoint_url=config["R2_ENDPOINT"],
        aws_access_key_id=config["R2_ACCESS_KEY_ID"],
        aws_secret_access_key=config["R2_SECRET_ACCESS_KEY"],
        region_name="auto",
    )
    prefix = f"task-images/{user_id}/"
    paginator = client.get_paginator("list_objects_v2")
    for page in paginator.paginate(Bucket=config["R2_BUCKET"], Prefix=prefix):
        objects = [{"Key": item["Key"]} for item in page.get("Contents", [])]
        if objects:
            client.delete_objects(
                Bucket=config["R2_BUCKET"],
                Delete={"Objects": objects, "Quiet": True},
            )


def erase_due_accounts(connection, now=None):
    now = now or datetime.now(timezone.utc).replace(tzinfo=None)
    cutoff = now - timedelta(days=RETENTION_DAYS)
    erased = 0
    # Re-query after each commit to avoid holding a large transaction open.
    while erased < 100:
        try:
            with connection.cursor() as cursor:
                cursor.execute(
                    """SELECT u.id, u.email FROM users u
                       LEFT JOIN account_erasure_holds h ON h.user_id = u.id
                       WHERE u.is_deleted = 1 AND u.deleted_at IS NOT NULL
                         AND u.deleted_at <= %s AND h.user_id IS NULL
                       ORDER BY u.deleted_at, u.id LIMIT 1 FOR UPDATE""",
                    (cutoff,),
                )
                row = cursor.fetchone()
                if row is None:
                    connection.rollback()
                    break
                user_id, email = row
                # Erase credentials first. If the app transaction later fails,
                # the still-present soft-deleted row makes the next run retryable.
                erase_auth_user(user_id)
                # Remove external objects before the rows identifying their owner.
                # A failure aborts this account so the next run retries cleanup.
                erase_task_images(user_id)
                # Clear queued and historical jobs containing this user's task
                # data before deleting the task rows used for the lookup.
                cursor.execute(
                    """DELETE j FROM integration_jobs j
                       LEFT JOIN tasks t ON t.id = CAST(JSON_UNQUOTE(JSON_EXTRACT(j.payload, '$.TaskID')) AS UNSIGNED)
                       WHERE CAST(JSON_UNQUOTE(JSON_EXTRACT(j.payload, '$.Task.AddedUserID')) AS UNSIGNED) = %s
                          OR CAST(JSON_UNQUOTE(JSON_EXTRACT(j.payload, '$.AddedUserID')) AS UNSIGNED) = %s
                          OR t.added_user_id = %s""",
                    (user_id, user_id, user_id),
                )
                cursor.execute(
                    """DELETE q FROM notifications_queue q JOIN tasks t
                       ON t.id = CAST(SUBSTRING_INDEX(SUBSTRING_INDEX(q.event_key, ':', 4), ':', -1) AS UNSIGNED)
                       WHERE q.event_key LIKE 'task:%%' AND t.added_user_id = %s""",
                    (user_id,),
                )
                # Remove account-created shared content and its dependent rows.
                for table in ("tasks_teams_logs", "flagged_contents", "user_reports", "tasks_users_completions"):
                    cursor.execute(
                        f"DELETE child FROM {table} child JOIN tasks t ON t.id = child.task_id WHERE t.added_user_id = %s",
                        (user_id,),
                    )
                cursor.execute("DELETE FROM tasks WHERE added_user_id = %s", (user_id,))
                cursor.execute("DELETE FROM user_reports WHERE reporter_user_id = %s", (user_id,))
                for table in (
                    "api_action_logs", "api_keys", "blocks", "day_shift_assignment_users",
                    "day_shift_cycle_group_members", "ical_keys", "logs", "mcps",
                    "notification_deliveries", "notification_devices", "notification_type_prefs",
                    "notifications_queue", "passkeys", "permissions", "recovery_codes",
                    "role_users", "sessions", "tasks_users_completions", "totp_secrets",
                    "web_push_subscriptions",
                ):
                    cursor.execute(f"DELETE FROM {table} WHERE user_id = %s", (user_id,))
                cursor.execute("DELETE FROM login_codes WHERE email = %s", (email,))
                cursor.execute("DELETE FROM email_suppressions WHERE email = %s", (email,))
                cursor.execute("DELETE FROM users WHERE id = %s", (user_id,))
            connection.commit()
            erased += 1
            print(f"erased account id={user_id}")
        except Exception:
            connection.rollback()
            raise
    # Actor names in historical jobs cannot always be attributed to a user ID.
    # Enforce their age limit even on days without an account due for erasure.
    # Jobs still pending after 30 days are stale and are discarded as well.
    try:
        with connection.cursor() as cursor:
            cursor.execute(
                """DELETE FROM integration_jobs WHERE kind IN
                   ('task_discord', 'task_email', 'teams_inbound', 'teams_callback')
                   AND created_at <= %s""",
                (cutoff,),
            )
            cursor.execute(
                "DELETE FROM notifications_queue WHERE created_at <= %s",
                (cutoff,),
            )
        connection.commit()
    except Exception:
        connection.rollback()
        raise
    return erased


if __name__ == "__main__":
    connection = pymysql.connect(
        host=os.environ["DB_HOST"],
        port=int(os.environ.get("DB_PORT", "3306")),
        user=os.environ["MYSQL_USER"],
        password=os.environ["MYSQL_PASSWORD"],
        database=os.environ["MYSQL_DATABASE"],
        autocommit=False,
        connect_timeout=10,
    )
    try:
        print(f"erased {erase_due_accounts(connection)} accounts")
    finally:
        connection.close()
