from datetime import datetime, timedelta
import requests
import icalendar
import os
from typing import Optional, List, Dict

from airflow import DAG
from airflow.operators.python import PythonOperator
import pymysql

# Database connection (read/write directly, no API calls)
DB_HOST = os.environ.get("DB_HOST", "mysql")
DB_PORT = int(os.environ.get("DB_PORT", "3306"))
DB_USER = os.environ.get("DB_USER", "taskly")
DB_PASSWORD = os.environ.get("DB_PASSWORD", "taskly")
DB_NAME = os.environ.get("DB_NAME", "taskly")
SCHOOL_CAL_URL = os.environ.get("SCHOOL_CAL_URL", "")
JAPAN_HOLIDAY_URL = "https://one.webcal.jp/JapanHolidays/"  # This page lists sources; assume env provides ICS URL if needed
SCHOOL_CAL_TIMEOUT = 10
DISCORD_WEBHOOK_URL = os.environ.get("DISCORD_WEBHOOK_URL", "")
SENTRY_DSN = os.environ.get("SENTRY_DSN", "")


def is_weekend(d: datetime) -> bool:
    return d.weekday() >= 5


def fetch_ics(url: str) -> icalendar.Calendar:
    resp = requests.get(url, timeout=SCHOOL_CAL_TIMEOUT)
    resp.raise_for_status()
    return icalendar.Calendar.from_ical(resp.content)


def is_holiday(d: datetime) -> bool:
    # Weekend
    if is_weekend(d):
        return True
    # Holidays via JapanHolidays (optional; if not configured, skip)
    holiday_ics_url = os.environ.get("JAPAN_HOLIDAYS_ICS_URL", "")
    if holiday_ics_url:
        try:
            cal = fetch_ics(holiday_ics_url)
            for comp in cal.walk("vevent"):
                dtstart = comp.get("dtstart").dt
                dt = datetime(dtstart.year, dtstart.month, dtstart.day)
                if dt.date() == d.date():
                    return True
        except Exception as e:
            # On network failure, treat as non-holiday per spec
            if DISCORD_WEBHOOK_URL:
                try:
                    requests.post(DISCORD_WEBHOOK_URL, json={"content": f"Holiday ICS fetch failed: {e}"}, timeout=5)
                except Exception:
                    pass
            # Sentry optional
    return False


def is_day_shift_off(d: datetime) -> bool:
    try:
        if not SCHOOL_CAL_URL:
            # No school calendar configured; cannot detect DAY_SHIFT_OFF
            return False
        cal = fetch_ics(SCHOOL_CAL_URL)
        for comp in cal.walk("vevent"):
            dtstart = comp.get("dtstart").dt
            dt = datetime(dtstart.year, dtstart.month, dtstart.day)
            if dt.date() == d.date():
                desc = comp.get("description") or ""
                if isinstance(desc, bytes):
                    desc = desc.decode("utf-8", errors="ignore")
                if "DAY_SHIFT_OFF" in desc:
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

def increment_if_needed(**context):
    now = datetime.now()
    today_str = now.strftime("%Y-%m-%d")
    before = read_index()

    # Batch runs at 03:00; check current day status
    if is_holiday(now):
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

    # Reset index if cycle changed
    db_cid = get_current_cycle_id_for_date(now)
    cfg_cid = read_current_cycle_config()
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

    # Honor one-shot skip flag for business days. The flag is ignored on
    # holidays/DAY_SHIFT_OFF above so that it naturally carries over to the
    # next business day.
    if read_increment_skip_flag():
        clear_increment_skip_flag()
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

    after = before + 1
    write_index(after)
    discord_embed(
        title="Day Shift Index Incremented",
        description="日直インデックスをインクリメントしました。",
        fields=[
            {"name": "Date", "value": today_str, "inline": True},
            {"name": "Cycle ID", "value": str(db_cid or cfg_cid or 0), "inline": True},
            {"name": "Before", "value": str(before), "inline": True},
            {"name": "After", "value": str(after), "inline": True},
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
    schedule_interval="0 3 * * *",
    catchup=False,
    default_args={"owner": "taskly"},
) as dag:
    inc = PythonOperator(task_id="increment_index", python_callable=increment_if_needed)

