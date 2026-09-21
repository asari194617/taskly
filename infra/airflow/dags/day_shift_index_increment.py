from datetime import datetime, timedelta, timezone
import requests
import icalendar  # type: ignore
import os
from typing import Optional, List, Dict

from airflow.sdk import DAG
from airflow.providers.standard.operators.python import PythonOperator
import pymysql  # type: ignore
from holiday_calendar import is_calendar_holiday

# Database connection (read/write directly, no API calls)
DB_HOST = os.environ.get("DB_HOST", "mysql")
DB_PORT = int(os.environ.get("DB_PORT", "3306"))
DB_USER = os.environ.get("DB_USER", "taskly")
DB_PASSWORD = os.environ.get("DB_PASSWORD", "taskly")
DB_NAME = os.environ.get("DB_NAME", "taskly")
SCHOOL_CAL_URL = os.environ.get("SCHOOL_CAL_URL", "")
JAPAN_HOLIDAY_URL = os.environ.get("JAPAN_HOLIDAYS_ICS_URL", "https://one.webcal.jp/JapanHolidays/")
SCHOOL_CAL_TIMEOUT = 10
DISCORD_WEBHOOK_URL = os.environ.get("DISCORD_WEBHOOK_URL", "")
SENTRY_DSN = os.environ.get("SENTRY_DSN", "")


def is_weekend(d: datetime) -> bool:
    return d.weekday() >= 5


def fetch_ics(url: str) -> icalendar.Component:
    resp = requests.get(url, timeout=SCHOOL_CAL_TIMEOUT)
    resp.raise_for_status()
    return icalendar.Calendar.from_ical(resp.content.decode('utf-8'))


def is_holiday(d: datetime) -> bool:
    if is_weekend(d):
        return True
    try:
        return is_calendar_holiday(fetch_ics(JAPAN_HOLIDAY_URL), d.date())
    except Exception as e:
        if DISCORD_WEBHOOK_URL:
            try:
                requests.post(DISCORD_WEBHOOK_URL, json={"content": f"Holiday ICS fetch failed: {e}"}, timeout=5)
            except Exception:
                pass
    return False


def is_day_shift_off(d: datetime) -> bool:
    try:
        if not SCHOOL_CAL_URL:
            # No school calendar configured; cannot detect DAY_SHIFT_OFF
            return False
        cal = fetch_ics(SCHOOL_CAL_URL)
        target_date = d.date()
        for comp in cal.walk("vevent"):
            # Description に DAY_SHIFT_OFF を含む予定のみを対象とする
            desc = comp.get("description") or ""
            if isinstance(desc, bytes):
                desc = desc.decode("utf-8", errors="ignore")
            if "DAY_SHIFT_OFF" not in desc:
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

                # RFC 5545 に従い、DTEND は非包含とみなし、開始日と異なる場合は 1 日戻す
                if end_date_candidate > start_date:
                    end_date = end_date_candidate - timedelta(days=1)
                else:
                    end_date = start_date
            elif duration_prop is not None:
                duration_value = getattr(duration_prop, "dt", duration_prop)
                if isinstance(duration_value, timedelta) and duration_value.days > 0:
                    # duration は経過時間なので、開始日 + duration - 1 日を最終日とする
                    end_date = start_date + duration_value - timedelta(days=1)

            if start_date <= target_date <= end_date:
                return True
        return False
    except Exception as e:
        # On failure, do NOT block increment (spec says proceed)
        if DISCORD_WEBHOOK_URL:
            try:
                requests.post(DISCORD_WEBHOOK_URL, json={"content": f"School calendar fetch failed: {e}"}, timeout=5)
            except Exception:
                pass
        return False


def read_index() -> int:
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT value FROM configs WHERE `key`='day_shift_current_index' LIMIT 1")
            row = cur.fetchone()
            if not row or row[0] is None:
                return 0
            try:
                return int(row[0])
            except Exception:
                return 0
    finally:
        conn.close()


def write_index(v: int) -> None:
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            # Try update; if no row, insert
            cur.execute("UPDATE configs SET value=%s WHERE `key`='day_shift_current_index'", (str(v),))
            if cur.rowcount == 0:
                cur.execute("INSERT INTO configs(`key`, `value`) VALUES('day_shift_current_index', %s) ON DUPLICATE KEY UPDATE value=VALUES(value)", (str(v),))
        conn.commit()
    finally:
        conn.close()


def read_increment_skip_flag() -> bool:
    """Return True when configs.day_shift_index_increment_skip is truthy.

    Any non-empty, non-"0", non-"false" (case-insensitive) value is treated
    as enabled. This keeps semantics simple while allowing future extensions.
    """
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT value FROM configs WHERE `key`='day_shift_index_increment_skip' LIMIT 1")
            row = cur.fetchone()
            if not row or row[0] is None:
                return False
            v = str(row[0]).strip().lower()
            if v == "" or v == "0" or v == "false" or v == "no":
                return False
            return True
    finally:
        conn.close()


def clear_increment_skip_flag() -> None:
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE configs SET value='' WHERE `key`='day_shift_index_increment_skip'"
            )
        conn.commit()
    finally:
        conn.close()


def set_skip_notifications_flag() -> None:
    """Set configs.skip_day_shift_notifications to "1" (upsert).

    This flag is consumed as a one-shot skip by the 08:30
    notification DAG (day_shift_notify_0830).
    """
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            cur.execute(
                "INSERT INTO configs(`key`, `value`) VALUES('skip_day_shift_notifications', '1') "
                "ON DUPLICATE KEY UPDATE value='1'",
            )
        conn.commit()
    finally:
        conn.close()


def get_current_cycle_id_for_date(d: datetime) -> int:
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id FROM day_shift_cycles WHERE begin_date <= %s AND finish_date >= %s ORDER BY finish_date DESC, id DESC LIMIT 1",
                (d.date(), d.date()),
            )
            row = cur.fetchone()
            if not row:
                return 0
            return int(row[0])
    finally:
        conn.close()


def resolve_next_day_shift_members(index: int, cycle_id: int) -> str:
    """Resolve display names for the group that will be on duty for the given index.

    This mirrors the server-side logic: index is 0-based and the group is
    selected by (index % number_of_groups) within the active cycle. Members are
    resolved from day_shift_cycle_group_members and users, skipping deleted users.
    """
    if cycle_id == 0:
        return ""
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            # Fetch ordered groups for this cycle
            cur.execute(
                "SELECT id FROM day_shift_cycle_groups WHERE cycle_id=%s ORDER BY position ASC, id ASC",
                (cycle_id,),
            )
            groups = [row[0] for row in cur.fetchall() or []]
            if not groups:
                return ""
            base = max(index, 0)
            gidx = base % len(groups)
            target_gid = groups[gidx]

            # Fetch member user IDs for the target group
            cur.execute(
                "SELECT user_id FROM day_shift_cycle_group_members WHERE group_id=%s ORDER BY id ASC",
                (target_gid,),
            )
            uids = [row[0] for row in cur.fetchall() or []]
            if not uids:
                return ""

            # Resolve user display names (or email as fallback), skipping deleted users
            format_strings = ",".join(["%s"] * len(uids))
            cur.execute(
                f"SELECT id, COALESCE(NULLIF(TRIM(display_name), ''), email) AS name, is_deleted "
                f"FROM users WHERE id IN ({format_strings})",
                tuple(uids),
            )
            names = []
            for uid, name, is_deleted in cur.fetchall() or []:
                if is_deleted:
                    continue
                names.append(name or f"ID:{uid}")
            return ", ".join(names)
    finally:
        conn.close()


def discord_embed(title: str, description: Optional[str] = None, fields: Optional[List[Dict]] = None, color: int = 0x3498DB):
    url = DISCORD_WEBHOOK_URL
    if not url:
        return
    payload = {"embeds": [{
        "title": title,
        "color": color,
        "timestamp": datetime.utcnow().isoformat() + "Z",
    }]}
    if description:
        payload["embeds"][0]["description"] = description
    if fields:
        payload["embeds"][0]["fields"] = fields
    try:
        requests.post(url, json=payload, timeout=5)
    except Exception:
        pass

def read_current_cycle_config() -> int:
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT value FROM configs WHERE `key`='day_shift_current_cycle' LIMIT 1")
            row = cur.fetchone()
            if not row or row[0] is None or str(row[0]).strip() == "":
                return 0
            try:
                return int(row[0])
            except Exception:
                return 0
    finally:
        conn.close()

def write_current_cycle_config(cid: int) -> None:
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            cur.execute("UPDATE configs SET value=%s WHERE `key`='day_shift_current_cycle'", (str(cid),))
            if cur.rowcount == 0:
                cur.execute("INSERT INTO configs(`key`, `value`) VALUES('day_shift_current_cycle', %s) ON DUPLICATE KEY UPDATE value=VALUES(value)", (str(cid),))
        conn.commit()
    finally:
        conn.close()


def clear_current_cycle_config() -> None:
    conn = pymysql.connect(host=DB_HOST, port=DB_PORT, user=DB_USER, password=DB_PASSWORD, database=DB_NAME, charset="utf8mb4")
    try:
        with conn.cursor() as cur:
            cur.execute("UPDATE configs SET value='' WHERE `key`='day_shift_current_cycle'")
        conn.commit()
    finally:
        conn.close()

JST = timezone(timedelta(hours=9))


def increment_if_needed(**context):
    # Use JST so that "today" is evaluated in Japan local time,
    # independent of the Airflow scheduler's server timezone.
    now = datetime.now(JST)
    today_str = now.strftime("%Y-%m-%d")
    before = read_index()

    # Batch runs at 03:00; check current day status
    if is_holiday(now):
        set_skip_notifications_flag()
        discord_embed(
            title="Day Shift Index Skipped",
            description="本日は休日のためインクリメントを行いませんでした。",
            fields=[
                {"name": "Date", "value": today_str, "inline": True},
                {"name": "Before", "value": str(before), "inline": True},
                {"name": "After", "value": str(before), "inline": True},
                {"name": "Reason", "value": "Holiday/Weekend", "inline": False},
            ],
            color=0xE67E22,
        )
        return
    if is_day_shift_off(now):
        set_skip_notifications_flag()
        discord_embed(
            title="Day Shift Index Skipped",
            description="学校カレンダーにより本日は日直業務を行わないためスキップしました。",
            fields=[
                {"name": "Date", "value": today_str, "inline": True},
                {"name": "Before", "value": str(before), "inline": True},
                {"name": "After", "value": str(before), "inline": True},
                {"name": "Reason", "value": "DAY_SHIFT_OFF (School Calendar)", "inline": False},
            ],
            color=0xE67E22,
        )
        return

    # Honor one-shot skip flag for business days. The flag is ignored on
    # holidays/DAY_SHIFT_OFF above so that it naturally carries over to the
    # next business day.
    if read_increment_skip_flag():
        clear_increment_skip_flag()
        set_skip_notifications_flag()
        discord_embed(
            title="Day Shift Index Skipped",
            description="スキップフラグによりインクリメントを行いませんでした。",
            fields=[
                {"name": "Date", "value": today_str, "inline": True},
                {"name": "Before", "value": str(before), "inline": True},
                {"name": "After", "value": str(before), "inline": True},
                {"name": "Reason", "value": "day_shift_index_increment_skip flag", "inline": False},
            ],
            color=0xE67E22,
        )
        return
    
    # Reset index if cycle changed
    db_cid = get_current_cycle_id_for_date(now)
    cfg_cid = read_current_cycle_config()
    if db_cid == 0:
        # No active cycle; clear stale config and skip increment.
        if cfg_cid != 0:
            clear_current_cycle_config()
        set_skip_notifications_flag()
        discord_embed(
            title="Day Shift Index Skipped",
            description="有効な日直サイクルが存在しないため、インクリメントを行いませんでした。",
            fields=[
                {"name": "Date", "value": today_str, "inline": True},
                {"name": "Before", "value": str(before), "inline": True},
                {"name": "After", "value": str(before), "inline": True},
                {"name": "Reason", "value": "No active cycle", "inline": False},
            ],
            color=0xE67E22,
        )
        return
    if db_cid != 0 and db_cid != cfg_cid:
        write_current_cycle_config(db_cid)
        write_index(0)
        discord_embed(
            title="Day Shift Cycle Changed",
            description="有効なサイクルの変更を検知し、インデックスをリセットしました。",
            fields=[
                {"name": "Date", "value": today_str, "inline": True},
                {"name": "Old Cycle ID", "value": str(cfg_cid), "inline": True},
                {"name": "New Cycle ID", "value": str(db_cid), "inline": True},
                {"name": "Index Reset", "value": f"{before} → 0", "inline": False},
            ],
            color=0x9B59B6,
        )
        return

    after = before + 1
    write_index(after)
    # Resolve next day's members based on the incremented index and current cycle
    effective_cid = db_cid or cfg_cid or 0
    next_members = resolve_next_day_shift_members(after, effective_cid)
    discord_embed(
        title="Day Shift Index Incremented",
        description="日直インデックスをインクリメントしました。",
        fields=[
            {"name": "Date", "value": today_str, "inline": True},
            {"name": "Cycle ID", "value": str(effective_cid), "inline": True},
            {"name": "Before", "value": str(before), "inline": True},
            {"name": "After", "value": str(after), "inline": True},
            {
                "name": "Next Members",
                "value": next_members or "(メンバー情報が取得できませんでした)",
                "inline": False,
            },
        ],
        color=0x2ECC71,
    )


def send_830_notifications(**context):
    # Notifications are handled in the dedicated notifications DAG.
    # No action here.
    return

with DAG(
    dag_id="day_shift_index_increment",
    start_date=datetime(2025, 1, 1),
    schedule="0 18 * * *",
    catchup=False,
    default_args={"owner": "taskly"},
    tags=["day_shift"],
) as dag:
    inc = PythonOperator(task_id="increment_index", python_callable=increment_if_needed)
