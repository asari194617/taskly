from datetime import datetime, date, timedelta, timezone
import os
import smtplib
import time
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
from airflow.providers.standard.operators.bash import BashOperator
from airflow.providers.standard.operators.python import PythonOperator
from airflow.utils.trigger_rule import TriggerRule


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

    def _get_message_class(message_name: str):
        descriptor = pool.FindMessageTypeByName(message_name)
        get_message_class = getattr(message_factory, "GetMessageClass", None)
        if callable(get_message_class):
            return get_message_class(descriptor)

        get_prototype = getattr(factory, "GetPrototype", None)
        if callable(get_prototype):
            return get_prototype(descriptor)

        raise RuntimeError("protobuf message factory does not provide a compatible message class API")

    ComposeDayShiftRequest = _get_message_class("emailcompose.ComposeDayShiftRequest")
    ComposeResponse = _get_message_class("emailcompose.ComposeResponse")

    _email_compose_messages = (ComposeDayShiftRequest, ComposeResponse)
    return _email_compose_messages


def compose_day_shift_email_via_grpc(members, events, locale: str, app_name: str):
    if grpc is None:
        raise RuntimeError("grpc module is not available")
    ComposeDayShiftRequest, ComposeResponse = _get_email_compose_messages()
    addr = os.getenv("EMAIL_COMPOSE_GRPC_ADDR", "email-compose:50051").strip()
    timeout = int(os.getenv("EMAIL_COMPOSE_GRPC_TIMEOUT", "10"))
    max_attempts = int(os.getenv("EMAIL_COMPOSE_GRPC_MAX_ATTEMPTS", "3"))
    retry_delay = float(os.getenv("EMAIL_COMPOSE_GRPC_RETRY_DELAY", "1"))

    req = ComposeDayShiftRequest(
        locale=locale,
        app_name=app_name,
        members=members,
        events=events,
    )

    def _is_retryable_error(exc: Exception) -> bool:
        code = getattr(exc, "code", None)
        if callable(code):
            try:
                if code() == grpc.StatusCode.UNAVAILABLE:
                    return True
            except Exception:
                pass

        message = str(exc).lower()
        retryable_markers = (
            "dns resolution failed",
            "address lookup failed",
            "name not found",
            "hostname lookup error",
            "unavailable",
        )
        return any(marker in message for marker in retryable_markers)

    last_error: Exception | None = None
    for attempt in range(1, max_attempts + 1):
        try:
            with grpc.insecure_channel(addr) as channel:
                method = channel.unary_unary(
                    "/emailcompose.EmailComposer/ComposeDayShift",
                    request_serializer=ComposeDayShiftRequest.SerializeToString,
                    response_deserializer=ComposeResponse.FromString,
                )
                res = method(req, timeout=timeout)
            return res.subject, res.html, res.text
        except Exception as exc:
            last_error = exc
            if attempt >= max_attempts or not _is_retryable_error(exc):
                raise
            time.sleep(retry_delay * (2 ** (attempt - 1)))

    if last_error is not None:
        raise last_error
    raise RuntimeError("email-compose gRPC call failed")


def send_day_shift_emails(users, subject: str, text_body: str, html_body: str) -> None:
    """Send day shift notification emails via SMTP.

    - Uses the same env vars as other components:
      SMTP_HOST, SMTP_PORT, SMTP_FROM, SMTP_USER, SMTP_PASSWORD,
      SMTP_TLS ("starttls" to enable), SMTP_TIMEOUT.
    - Call with one recipient so failures and retries are tracked per user.
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

    with smtplib.SMTP(host=smtp_host, port=smtp_port, timeout=timeout) as smtp:
        try:
            smtp.ehlo()
        except Exception:
            pass

        if smtp_tls == "starttls":
            try:
                smtp.starttls()
                smtp.ehlo()
            except Exception as e:
                raise RuntimeError("SMTP STARTTLS failed") from e

        if smtp_pass.strip():
            try:
                smtp.login(smtp_user, smtp_pass)
            except Exception as e:
                raise RuntimeError("SMTP login failed") from e

        smtp.sendmail(from_addr, to_addrs, msg.as_string())


def claim_delivery(conn, key: str, channel: str, user_id: int) -> bool:
    with conn.cursor() as cur:
        cur.execute("""INSERT INTO notification_deliveries
            (delivery_key, channel, user_id, status, attempts)
            VALUES (%s, %s, %s, 'sending', 1)
            ON DUPLICATE KEY UPDATE
            attempts=IF(status='error', attempts+1, attempts),
            error_message=IF(status='error', NULL, error_message),
            status=IF(status='error', 'sending', status)""", (key, channel, user_id))
        return cur.rowcount > 0


def finish_delivery(conn, key: str, error: Exception | None) -> None:
    with conn.cursor() as cur:
        cur.execute("""UPDATE notification_deliveries
            SET status=%s, error_message=%s,
                sent_at=IF(%s='sent', NOW(), sent_at)
            WHERE delivery_key=%s AND status='sending'""",
            ("error" if error else "sent", str(error)[:2000] if error else None,
             "error" if error else "sent", key))


def enqueue_day_shift_push(conn, today: date, user_id: int) -> None:
    key = f"dayshift:{today.isoformat()}:webpush:{user_id}"
    with conn.cursor() as cur:
        cur.execute("""INSERT INTO notifications_queue
            (kind, when_label, subject, title, body, user_id, channel, event_key)
            VALUES ('dayshift', '今日', '日直', '日直の担当日です',
                    '本日は日直の担当日です。よろしくお願いします。', %s, 'webpush', %s)
            ON DUPLICATE KEY UPDATE id=id""", (user_id, key))


def select_day_shift_recipients(all_users, blocked_map, pref_map, dev_map, subscribed_ids):
    enabled, blocked, email, push = [], [], [], []
    for user in all_users:
        if user.get("id") is None:
            continue
        uid = int(user["id"])
        if blocked_map.get(uid, 0) > 0:
            blocked.append(user)
            continue
        if pref_map.get(uid, 1) == 0:
            continue
        enabled.append(user)
        if dev_map.get((uid, "EMAIL"), 1) == 1 and (user.get("email") or "").strip():
            email.append(user)
        if dev_map.get((uid, "WEBPUSH"), 0) == 1 and uid in subscribed_ids:
            push.append(user)
    return enabled, blocked, email, push


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
                f"SELECT id, email, display_name, notification_lang FROM users WHERE id IN ({ids_csv}) AND is_deleted = 0"
            )
            all_users = cur.fetchall() or []
        # Filter by NotificationTypePref and EMAIL device overrides (batch queries to avoid N+1)
        enabled_users = []
        blocked_users = []
        users = []
        push_users = []

        # all_users was selected earlier using ids_csv
        # Build ids with explicit None-check to satisfy static type checkers
        ids: list[int] = []
        for u in all_users:
            uid_val = u.get("id")
            if uid_val is None:
                continue
            try:
                ids.append(int(uid_val))
            except Exception:
                # Skip any invalid id values
                continue
        if ids:
            # Reuse ids_csv style used earlier
            ids_csv = ",".join(str(i) for i in ids)

            # Fetch active blocks for all users in one query
            with conn.cursor() as cur:
                cur.execute(
                    f"SELECT user_id, COUNT(1) AS c FROM blocks WHERE user_id IN ({ids_csv}) AND (expires_at IS NULL OR expires_at > NOW()) GROUP BY user_id"
                )
                blk_rows = cur.fetchall() or []
            blocked_map = {int(r["user_id"]): int(r.get("c", 0) or 0) for r in blk_rows}

            # Fetch latest notification_type_prefs for DAYSHIFT for these users (ordered by id desc)
            with conn.cursor() as cur:
                cur.execute(
                    f"SELECT user_id, is_enabled FROM notification_type_prefs WHERE notification_type='DAYSHIFT' AND user_id IN ({ids_csv}) ORDER BY user_id ASC, id DESC"
                )
                pref_rows = cur.fetchall() or []
            pref_map = {}
            for r in pref_rows:
                uid = int(r["user_id"])
                if uid not in pref_map:
                    pref_map[uid] = int(r.get("is_enabled", 1))

            # Fetch latest device records for both delivery channels.
            with conn.cursor() as cur:
                cur.execute(
                    f"SELECT user_id, platform, MAX(is_enabled) AS is_enabled FROM notification_devices WHERE platform IN ('EMAIL','WEBPUSH') AND user_id IN ({ids_csv}) GROUP BY user_id, platform"
                )
                dev_rows = cur.fetchall() or []
            dev_map = {}
            for r in dev_rows:
                uid = int(r["user_id"])
                platform = r["platform"]
                dev_map[(uid, platform)] = int(r.get("is_enabled", 1))

            with conn.cursor() as cur:
                cur.execute(
                    f"SELECT DISTINCT wps.user_id FROM web_push_subscriptions wps JOIN notification_devices d ON d.user_id=wps.user_id AND d.platform='WEBPUSH' AND d.is_enabled=1 AND JSON_UNQUOTE(d.payload)=wps.endpoint WHERE wps.user_id IN ({ids_csv})"
                )
                subscribed_ids = {int(r["user_id"]) for r in (cur.fetchall() or [])}

            enabled_users, blocked_users, users, push_users = select_day_shift_recipients(
                all_users, blocked_map, pref_map, dev_map, subscribed_ids)
        for user in push_users:
            enqueue_day_shift_push(conn, today, int(user["id"]))
        if not users and not push_users:
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
        members = [
            (u.get("display_name") or u.get("email") or "").strip()
            for u in enabled_users
            if (u.get("display_name") or u.get("email") or "").strip()
        ]
        app_name = "Taskly"
        subject = "日直のお知らせ"
        events = list_events_for_date(today)
        failures = []
        # Compose once per language and send only to users with that preference.
        for locale in ("ja", "en"):
            recipients = [
                u for u in users
                if ("en" if u.get("notification_lang") == "en" else "ja") == locale
            ]
            if not recipients:
                continue
            try:
                subject, html_body, text_body = compose_day_shift_email_via_grpc(
                    members=members,
                    events=events,
                    locale=locale,
                    app_name=app_name,
                )
            except Exception as e:
                print(f"[DayShift] email-compose gRPC failed ({locale}): {e}")
                failures.append(e)
                continue
            for recipient in recipients:
                uid = int(recipient["id"])
                key = f"dayshift:{today.isoformat()}:email:{uid}"
                if not claim_delivery(conn, key, "email", uid):
                    with conn.cursor() as cur:
                        cur.execute("SELECT status FROM notification_deliveries WHERE delivery_key=%s", (key,))
                        row = cur.fetchone()
                    if not row or row["status"] != "sent":
                        failures.append(RuntimeError(f"unresolved delivery: {key}"))
                    continue
                try:
                    send_day_shift_emails([recipient], subject, text_body, html_body)
                except Exception as e:
                    finish_delivery(conn, key, e)
                    failures.append(e)
                else:
                    finish_delivery(conn, key, None)
        # Discord embeds: report notified users (including blocked ones in Users)
        blocked_names = [
            (u.get("display_name") or u.get("email") or "").strip()
            for u in blocked_users
            if (u.get("display_name") or u.get("email") or "").strip()
        ]
        all_member_names = members + blocked_names
        users_field_value = ", ".join(all_member_names) or "(none)"
        hook = os.getenv("DISCORD_WEBHOOK_URL", "")
        if hook:
            fields = [
                {"name": "Date", "value": today.strftime("%Y-%m-%d"), "inline": True},
                {"name": "Cycle ID", "value": str(cur_cid), "inline": True},
                {"name": "Users", "value": users_field_value, "inline": False},
                {"name": "Email", "value": str(len(users)), "inline": True},
                {"name": "Web Push", "value": str(len(push_users)), "inline": True},
            ]
            if blocked_names:
                fields.append({
                    "name": "Blocked",
                    "value": ", ".join(blocked_names),
                    "inline": False,
                })
            try:
                requests.post(hook, json={
                    "embeds": [{
                        "title": "Day Shift Delivery Attempted",
                        "description": subject,
                        "color": 0x3498DB,
                        "fields": fields,
                        "timestamp": datetime.utcnow().isoformat() + "Z",
                    }]
                }, timeout=5)
            except Exception:
                pass
        if failures:
            raise RuntimeError(f"day shift email delivery failed for {len(failures)} recipient(s)")
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
    default_args={"owner": "taskly", "retries": 2, "retry_delay": timedelta(minutes=5)},
    tags=["day_shift"],
) as dag:
    notify = PythonOperator(task_id="notify_day_shift", python_callable=notify_day_shift)
    deliver_push = BashOperator(
        task_id="deliver_day_shift_push",
        bash_command="/opt/airflow/bin/notifications-delivery-channels-web-push",
        env={
            "DB_HOST": os.environ.get("DB_HOST", os.environ.get("MYSQL_HOST", "db")),
            "DB_PORT": os.environ.get("DB_PORT", os.environ.get("MYSQL_PORT", "3306")),
            "MYSQL_USER": os.environ.get("MYSQL_USER", "taskly"),
            "MYSQL_PASSWORD": os.environ.get("MYSQL_PASSWORD", "taskly_password"),
            "MYSQL_DATABASE": os.environ.get("MYSQL_DATABASE", "taskly"),
            "WEB_URL": os.environ.get("WEB_URL", "https://app.taskly.local"),
            "VAPID_PUBLIC_KEY": os.environ.get("VAPID_PUBLIC_KEY", ""),
            "VAPID_PRIVATE_KEY": os.environ.get("VAPID_PRIVATE_KEY", ""),
            "VAPID_SUBJECT": os.environ.get("VAPID_SUBJECT", "mailto:noreply@taskly.local"),
        },
        trigger_rule=TriggerRule.ALL_DONE,
    )
    notify >> deliver_push
