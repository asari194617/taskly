from datetime import datetime, date
import os
import smtplib
from email.header import Header
from email.mime.text import MIMEText

import requests
import pymysql

from airflow import DAG
from airflow.operators.python import PythonOperator


def mysql_connect():
    return pymysql.connect(
        host=os.getenv("MYSQL_HOST", "mysql"),
        port=int(os.getenv("MYSQL_PORT", "3306")),
        user=os.getenv("MYSQL_USER", "taskly"),
        password=os.getenv("MYSQL_PASSWORD", "taskly"),
        database=os.getenv("MYSQL_DATABASE", "taskly"),
        charset="utf8mb4",
        autocommit=True,
        cursorclass=pymysql.cursors.DictCursor,
    )



def read_config(conn, key: str) -> str:
    with conn.cursor() as cur:
        cur.execute("SELECT value FROM configs WHERE `key`=%s LIMIT 1", (key,))
        row = cur.fetchone()
        return (row or {}).get("value", "")


def send_day_shift_emails(users, subject: str, body: str) -> None:
    """Send day shift notification emails via SMTP.

    - Uses the same env vars as other components:
      SMTP_HOST, SMTP_PORT, SMTP_FROM, SMTP_USER, SMTP_PASSWORD,
      SMTP_TLS ("starttls" to enable), SMTP_TIMEOUT.
    - Sends a single email with all recipients in the To header.
    """

    # Collect recipient addresses
    to_addrs = []
    for u in users:
        email = (u.get("email") or "").strip()
        if email:
            to_addrs.append(email)

    if not to_addrs:
        return

    smtp_host = os.getenv("SMTP_HOST", "localhost")
    smtp_port = int(os.getenv("SMTP_PORT", "25"))
    from_addr = os.getenv("SMTP_FROM", "noreply@taskly.local")
    smtp_user = os.getenv("SMTP_USER", from_addr)
    smtp_pass = os.getenv("SMTP_PASSWORD", "")
    smtp_tls = os.getenv("SMTP_TLS", "").strip().lower()
    timeout = int(os.getenv("SMTP_TIMEOUT", "60"))

    msg = MIMEText(body, _subtype="plain", _charset="utf-8")
    msg["Subject"] = str(Header(subject, "utf-8"))
    msg["From"] = from_addr
    msg["To"] = ", ".join(to_addrs)

    try:
        with smtplib.SMTP(host=smtp_host, port=smtp_port, timeout=timeout) as smtp:
            try:
                smtp.ehlo()
            except Exception:
                pass

            if smtp_tls == "starttls":
                try:
                    smtp.starttls()
                    smtp.ehlo()
                except Exception:
                    # If STARTTLS negotiation fails, abort sending to avoid plain-text fallthrough.
                    print("[DayShift] SMTP STARTTLS failed; aborting send.")
                    return

            if smtp_pass.strip():
                try:
                    smtp.login(smtp_user, smtp_pass)
                except Exception as e:
                    print(f"[DayShift] SMTP login failed: {e}")
                    return

            smtp.sendmail(from_addr, to_addrs, msg.as_string())
    except Exception as e:
        # Log but do not raise to avoid failing the DAG run entirely.
        print(f"[DayShift] SMTP send failed: {e}")


def notify_day_shift(**context):
    conn = mysql_connect()
    try:
        today = date.today()
        # Always notify regardless of day-of-week

        # Global skip flag via configs.skip_day_shift_notifications
        skip_flag = read_config(conn, "skip_day_shift_notifications").strip()
        if skip_flag == "1":
            # One-shot flag: remove after reading so it only applies once
            try:
                with conn.cursor() as cur:
                    cur.execute(
                        "DELETE FROM configs WHERE `key`=%s",
                        ("skip_day_shift_notifications",),
                    )
            except Exception as e:
                print(f"[DayShift] Failed to delete skip_day_shift_notifications config: {e}")

            hook = os.getenv("DISCORD_WEBHOOK_URL", "")
            if hook:
                try:
                    requests.post(hook, json={
                        "embeds": [{
                            "title": "Day Shift Notification Skipped",
                            "description": "skip_day_shift_notifications が有効なため、本日の通知をスキップしました。",
                            "color": 0xE67E22,
                            "fields": [
                                {"name": "Date", "value": today.strftime("%Y-%m-%d"), "inline": True},
                                {"name": "Config Key", "value": "skip_day_shift_notifications", "inline": True},
                                {"name": "Config Value", "value": skip_flag or "(empty)", "inline": True},
                                {"name": "Reason", "value": "Skip flag enabled", "inline": False},
                            ],
                            "timestamp": datetime.utcnow().isoformat() + "Z",
                        }]
                    }, timeout=5)
                except Exception:
                    pass
            return

        # Read current cycle and index (0-based) from configs
        idx_str = read_config(conn, "day_shift_current_index").strip()
        cur_idx = int(idx_str) if idx_str.isdigit() else 0
        cid_str = read_config(conn, "day_shift_current_cycle").strip()
        cur_cid = int(cid_str) if cid_str.isdigit() else 0

        # If cycle not configured, detect by date (latest finish_date)
        if cur_cid == 0:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT id FROM day_shift_cycles WHERE begin_date <= %s AND finish_date >= %s ORDER BY finish_date DESC, id DESC LIMIT 1",
                    (today.strftime("%Y-%m-%d"), today.strftime("%Y-%m-%d")),
                )
                row = cur.fetchone()
                if row:
                    cur_cid = int(row["id"])  # use detected cycle but do not persist here
        if cur_cid == 0:
            # No active cycle; send diagnostic embed and return
            hook = os.getenv("DISCORD_WEBHOOK_URL", "")
            if hook:
                requests.post(hook, json={
                    "embeds": [{
                        "title": "Day Shift Notification Skipped",
                        "description": "有効な日直サイクルが存在しないため、本日の通知をスキップしました。",
                        "color": 0xE67E22,
                        "fields": [
                            {"name": "Date", "value": today.strftime("%Y-%m-%d"), "inline": True},
                            {"name": "Reason", "value": "No active cycle", "inline": False},
                        ],
                        "timestamp": datetime.utcnow().isoformat() + "Z",
                    }]
                }, timeout=5)
            return
        
        

        # Load groups for the cycle ordered by position asc, id asc
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id, position FROM day_shift_cycle_groups WHERE cycle_id = %s ORDER BY position ASC, id ASC",
                (cur_cid,),
            )
            groups = cur.fetchall() or []
        if not groups:
            hook = os.getenv("DISCORD_WEBHOOK_URL", "")
            if hook:
                requests.post(hook, json={
                    "embeds": [{
                        "title": "Day Shift Notification Skipped",
                        "description": "グループが設定されていないため、本日の通知をスキップしました。",
                        "color": 0xE67E22,
                        "fields": [
                            {"name": "Date", "value": today.strftime("%Y-%m-%d"), "inline": True},
                            {"name": "Cycle ID", "value": str(cur_cid), "inline": True},
                            {"name": "Reason", "value": "No groups/pairs", "inline": False},
                        ],
                        "timestamp": datetime.utcnow().isoformat() + "Z",
                    }]
                }, timeout=5)
            return
        else:
            # Select group via modulo
            g = groups[cur_idx % len(groups)]
            gid = int(g["id"])
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT user_id FROM day_shift_cycle_group_members WHERE group_id = %s ORDER BY position ASC, id ASC",
                    (gid,),
                )
                members = cur.fetchall() or []
            target_users = [int(m["user_id"]) for m in members]

        if not target_users:
            hook = os.getenv("DISCORD_WEBHOOK_URL", "")
            if hook:
                requests.post(hook, json={
                    "embeds": [{
                        "title": "Day Shift Notification Skipped",
                        "description": "本日の担当メンバーが空のため、通知をスキップしました。",
                        "color": 0xE67E22,
                        "fields": [
                            {"name": "Date", "value": today.strftime("%Y-%m-%d"), "inline": True},
                            {"name": "Cycle ID", "value": str(cur_cid), "inline": True},
                            {"name": "Reason", "value": "Empty group members", "inline": False},
                        ],
                        "timestamp": datetime.utcnow().isoformat() + "Z",
                    }]
                }, timeout=5)
            return

        # Resolve user emails and check notification settings (default ON)
        ids_csv = ",".join(str(uid) for uid in target_users)
        with conn.cursor() as cur:
            cur.execute(
                f"SELECT id, email, display_name FROM users WHERE id IN ({ids_csv}) AND is_deleted = 0"
            )
            all_users = cur.fetchall() or []
        # Filter by NotificationTypePref and EMAIL device overrides
        enabled_users = []
        for u in all_users:
            uid = int(u.get("id"))
            # Type off check: if a record exists with DAYSHIFT and is_enabled=0, skip
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT is_enabled FROM notification_type_prefs WHERE user_id=%s AND notification_type='DAYSHIFT' ORDER BY id DESC LIMIT 1",
                    (uid,),
                )
                pref = cur.fetchone()
            if pref is not None and int(pref.get("is_enabled", 1)) == 0:
                continue
            # EMAIL device check:
            #   - default: enabled even when no record exists in notification_devices
            #   - if there is an EMAIL record with is_enabled = 0, treat as disabled
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT is_enabled FROM notification_devices WHERE user_id=%s AND platform='EMAIL' ORDER BY id DESC LIMIT 1",
                    (uid,),
                )
                dev = cur.fetchone()
            if dev is not None and int(dev.get("is_enabled", 1)) == 0:
                continue

            enabled_users.append(u)
        users = enabled_users
        if not users:
            hook = os.getenv("DISCORD_WEBHOOK_URL", "")
            if hook:
                requests.post(hook, json={
                    "embeds": [{
                        "title": "Day Shift Notification Skipped",
                        "description": "通知対象のユーザーが見つからないため、通知をスキップしました。",
                        "color": 0xE67E22,
                        "fields": [
                            {"name": "Date", "value": today.strftime("%Y-%m-%d"), "inline": True},
                            {"name": "Cycle ID", "value": str(cur_cid), "inline": True},
                            {"name": "Reason", "value": "No eligible users", "inline": False},
                        ],
                        "timestamp": datetime.utcnow().isoformat() + "Z",
                    }]
                }, timeout=5)
            return

        # Compose and send email via SMTP settings in env (simple direct send)
        # For production, consider queueing notifications in notifications_queue.
        subject = "[Taskly] 本日の日直のお知らせ"
        names = ", ".join(u.get("display_name") or u.get("email") for u in users)
        body = f"本日({today.strftime('%Y-%m-%d')})の日直: {names}"
        print(body)

        # Actually send email to all target users via SMTP.
        send_day_shift_emails(users, subject, body)
        # Discord embeds: report notified users
        hook = os.getenv("DISCORD_WEBHOOK_URL", "")
        if hook:
            fields = [
                {"name": "Date", "value": today.strftime("%Y-%m-%d"), "inline": True},
                {"name": "Cycle ID", "value": str(cur_cid), "inline": True},
                {"name": "Users", "value": names or "(none)", "inline": False},
                {"name": "Count", "value": str(len(users)), "inline": True},
            ]
            try:
                requests.post(hook, json={
                    "embeds": [{
                        "title": "Day Shift Notified",
                        "description": subject,
                        "color": 0x3498DB,
                        "fields": fields,
                        "timestamp": datetime.utcnow().isoformat() + "Z",
                    }]
                }, timeout=5)
            except Exception:
                pass
            # TODO: integrate with notifications_queue or direct SMTP/WebPush send
    finally:
        try:
            conn.close()
        except Exception:
            pass

with DAG(
    dag_id="day_shift_notify_0830",
    start_date=datetime(2025, 1, 1),
    schedule_interval="30 23 * * *",
    catchup=False,
    default_args={"owner": "taskly"},
) as dag:
    PythonOperator(task_id="notify_day_shift", python_callable=notify_day_shift)
