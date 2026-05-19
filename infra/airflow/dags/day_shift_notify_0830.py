from datetime import datetime, date, timedelta, timezone
import os
import smtplib
from email.header import Header
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText
from typing import Any

import requests
import pymysql  # type: ignore
try:
    import icalendar  # type: ignore
except Exception:  # pragma: no cover - allow DAG to load without optional deps
    icalendar = None
try:
    import grpc
    from google.protobuf import descriptor_pb2, descriptor_pool, message_factory
except Exception:  # pragma: no cover - allow DAG to load without optional deps
    grpc = None
    descriptor_pb2 = None
    descriptor_pool = None
    message_factory = None

from airflow.sdk import DAG
from airflow.providers.standard.operators.python import PythonOperator


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


_email_compose_messages = None


SCHOOL_CAL_URL = os.environ.get("SCHOOL_CAL_URL", "").strip()
SCHOOL_CAL_TIMEOUT = 10


def fetch_ics(url: str) -> Any:
    if icalendar is None:
        raise RuntimeError("icalendar module is not available")
    resp = requests.get(url, timeout=SCHOOL_CAL_TIMEOUT)
    resp.raise_for_status()
    return icalendar.Calendar.from_ical(resp.content.decode('utf-8'))


def list_events_for_date(target_date: date) -> list[str]:
    if not SCHOOL_CAL_URL:
        return []
    try:
        cal = fetch_ics(SCHOOL_CAL_URL)
        seen = set()
        events = []
        for comp in cal.walk("vevent"):
            summary = comp.get("summary") or ""
            if isinstance(summary, bytes):
                summary = summary.decode("utf-8", errors="ignore")
            summary = str(summary).strip()
            if not summary:
                continue

            dtstart_prop = comp.get("dtstart")
            if not dtstart_prop:
                continue
            dtstart = dtstart_prop.dt
            if isinstance(dtstart, datetime):
                start_date = dtstart.date()
            else:
                start_date = dtstart

            end_date = start_date
            dtend_prop = comp.get("dtend")
            duration_prop = comp.get("duration")

            if dtend_prop is not None:
                dtend = dtend_prop.dt
                if isinstance(dtend, datetime):
                    end_date_candidate = dtend.date()
                else:
                    end_date_candidate = dtend
                if end_date_candidate > start_date:
                    end_date = end_date_candidate - timedelta(days=1)
            elif duration_prop is not None:
                duration_value = getattr(duration_prop, "dt", duration_prop)
                if isinstance(duration_value, timedelta) and duration_value.days > 0:
                    end_date = start_date + duration_value - timedelta(days=1)

            if start_date <= target_date <= end_date:
                if summary not in seen:
                    events.append(summary)
                    seen.add(summary)

        return events
    except Exception as e:
        print(f"[DayShift] School calendar fetch failed: {e}")
        return []


def _get_email_compose_messages():
    global _email_compose_messages
    if _email_compose_messages is not None:
        return _email_compose_messages

    if descriptor_pb2 is None or descriptor_pool is None or message_factory is None:
        raise RuntimeError("protobuf modules are not available")

    file_proto = descriptor_pb2.FileDescriptorProto()
    file_proto.name = "emailcompose_dynamic.proto"
    file_proto.package = "emailcompose"
    file_proto.syntax = "proto3"

    # ComposeResponse
    msg = file_proto.message_type.add()
    msg.name = "ComposeResponse"
    field = msg.field.add()
    field.name = "subject"
    field.number = 1
    field.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    field.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    field = msg.field.add()
    field.name = "html"
    field.number = 2
    field.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    field.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    field = msg.field.add()
    field.name = "text"
    field.number = 3
    field.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    field.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING

    # ComposeDayShiftRequest
    msg = file_proto.message_type.add()
    msg.name = "ComposeDayShiftRequest"
    field = msg.field.add()
    field.name = "locale"
    field.number = 1
    field.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    field.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    field = msg.field.add()
    field.name = "app_name"
    field.number = 2
    field.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    field.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    field = msg.field.add()
    field.name = "members"
    field.number = 3
    field.label = descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED
    field.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    field = msg.field.add()
    field.name = "events"
    field.number = 4
    field.label = descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED
    field.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING

    pool = descriptor_pool.DescriptorPool()
    pool.Add(file_proto)
    factory = message_factory.MessageFactory(pool)
    ComposeDayShiftRequest = factory.GetPrototype(  # type: ignore
        pool.FindMessageTypeByName("emailcompose.ComposeDayShiftRequest")
    )
    ComposeResponse = factory.GetPrototype(  # type: ignore
        pool.FindMessageTypeByName("emailcompose.ComposeResponse")
    )

    _email_compose_messages = (ComposeDayShiftRequest, ComposeResponse)
    return _email_compose_messages


def compose_day_shift_email_via_grpc(members, events, locale: str, app_name: str):
    if grpc is None:
        raise RuntimeError("grpc module is not available")
    ComposeDayShiftRequest, ComposeResponse = _get_email_compose_messages()
    addr = os.getenv("EMAIL_COMPOSE_GRPC_ADDR", "email-compose:50051").strip()
    timeout = int(os.getenv("EMAIL_COMPOSE_GRPC_TIMEOUT", "10"))

    req = ComposeDayShiftRequest(
        locale=locale,
        app_name=app_name,
        members=members,
        events=events,
    )

    with grpc.insecure_channel(addr) as channel:
        method = channel.unary_unary(
            "/emailcompose.EmailComposer/ComposeDayShift",
            request_serializer=ComposeDayShiftRequest.SerializeToString,
            response_deserializer=ComposeResponse.FromString,
        )
        res = method(req, timeout=timeout)

    return res.subject, res.html, res.text


def send_day_shift_emails(users, subject: str, text_body: str, html_body: str) -> None:
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

    if html_body:
        msg = MIMEMultipart("alternative")
        msg.attach(MIMEText(text_body, _subtype="plain", _charset="utf-8"))
        msg.attach(MIMEText(html_body, _subtype="html", _charset="utf-8"))
    else:
        msg = MIMEText(text_body, _subtype="plain", _charset="utf-8")
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
        today = datetime.now(timezone(timedelta(hours=9))).date()
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
        cur_idx: int = int(idx_str) if idx_str.isdigit() else 0
        cid_str = read_config(conn, "day_shift_current_cycle").strip()
        cur_cid: int = int(cid_str) if cid_str.isdigit() else 0

        # Validate configured cycle is active for today; if not, ignore it.
        if cur_cid != 0:
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT id FROM day_shift_cycles WHERE id=%s AND begin_date <= %s AND finish_date >= %s LIMIT 1",
                    (cur_cid, today.strftime("%Y-%m-%d"), today.strftime("%Y-%m-%d")),
                )
                row = cur.fetchone()
                if not row:
                    cur_cid = 0

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
            uid_raw = u.get("id")
            if uid_raw is None:
                continue
            uid = int(uid_raw)
            # Type off check: if a record exists with DAYSHIFT and is_enabled=0, skip
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT is_enabled FROM notification_type_prefs WHERE user_id=%s AND notification_type='DAYSHIFT' ORDER BY id DESC LIMIT 1",
                    (uid,),
                )
                pref = cur.fetchone()
            pref_enabled = pref.get("is_enabled", 1) if pref is not None else 1
            if pref_enabled is not None and int(pref_enabled) == 0:
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
            dev_enabled = dev.get("is_enabled", 1) if dev is not None else 1
            if dev_enabled is not None and int(dev_enabled) == 0:
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

        # Compose HTML/text via email-compose gRPC (React Email)
        # For production, consider queueing notifications in notifications_queue.
        members = [
            (u.get("display_name") or u.get("email") or "").strip()
            for u in users
            if (u.get("display_name") or u.get("email") or "").strip()
        ]
        locale = "ja"
        app_name = "Taskly"
        events = list_events_for_date(today)

        names = ", ".join(members)
        subject = "[Taskly] 本日の日直のお知らせ"
        text_body = f"本日({today.strftime('%Y-%m-%d')})の日直: {names}"
        html_body = ""

        try:
            subject, html_body, text_body = compose_day_shift_email_via_grpc(
                members=members,
                events=events,
                locale=locale,
                app_name=app_name,
            )
        except Exception as e:
            print(f"[DayShift] email-compose gRPC failed: {e}")

        print(text_body)

        # Actually send email to all target users via SMTP.
        send_day_shift_emails(users, subject, text_body, html_body)
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
    schedule="30 23 * * *",
    catchup=False,
    default_args={"owner": "taskly"},
    tags=["day_shift"],
) as dag:
    PythonOperator(task_id="notify_day_shift", python_callable=notify_day_shift)
