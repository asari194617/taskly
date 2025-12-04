from datetime import datetime, date
import os
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


def is_business_day(conn: pymysql.connections.Connection, d: date) -> bool:
    with conn.cursor() as cur:
        cur.execute(
            "SELECT is_business_day FROM business_calendar WHERE date = %s LIMIT 1",
            (d.strftime("%Y-%m-%d"),),
        )
        row = cur.fetchone()
        if row is None:
            # Fallback: weekdays Mon-Fri are business days
            return d.weekday() < 5
        return bool(row.get("is_business_day", 0))


def read_config(conn, key: str) -> str:
    with conn.cursor() as cur:
        cur.execute("SELECT value FROM configs WHERE `key`=%s LIMIT 1", (key,))
        row = cur.fetchone()
        return (row or {}).get("value", "")


def notify_day_shift(**context):
    conn = mysql_connect()
    try:
        today = date.today()
        if not is_business_day(conn, today):
            print("Skip: not a business day")
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
            print("Skip: no active cycle")
            return

        # Load groups for the cycle ordered by position asc, id asc
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id, position FROM day_shift_cycle_groups WHERE cycle_id = %s ORDER BY position ASC, id ASC",
                (cur_cid,),
            )
            groups = cur.fetchall() or []
        if not groups:
            # Fallback to legacy pairs
            with conn.cursor() as cur:
                cur.execute(
                    "SELECT id, position, user1_id, user2_id FROM day_shift_cycle_pairs WHERE cycle_id = %s ORDER BY position ASC, id ASC",
                    (cur_cid,),
                )
                pairs = cur.fetchall() or []
            if not pairs:
                print("Skip: no groups/pairs configured")
                return
            # Build users list from pairs
            target_users = [p.get("user1_id"), p.get("user2_id")] if pairs else []
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
            print("Skip: empty members")
            return

        # Resolve user emails and send notifications (email only for now)
        with conn.cursor() as cur:
            cur.execute(
                "SELECT id, email, display_name FROM users WHERE id IN (%s) AND is_deleted = 0",
                (",".join(str(uid) for uid in target_users),),
            )
            users = cur.fetchall() or []
        if not users:
            print("Skip: users not found")
            return

        # Compose and send email via SMTP settings in env (simple direct send)
        # For production, consider queueing notifications in notifications_queue.
        subject = "[Taskly] 本日の日直のお知らせ"
        names = ", ".join(u.get("display_name") or u.get("email") for u in users)
        body = f"本日({today.strftime('%Y-%m-%d')})の日直: {names}"
        print(body)
        # TODO: integrate with notifications_queue or direct SMTP send
    finally:
        try:
            conn.close()
        except Exception:
            pass

with DAG(
    dag_id="day_shift_notify_0830",
    start_date=datetime(2025, 1, 1),
    schedule_interval="30 8 * * *",
    catchup=False,
    default_args={"owner": "taskly"},
) as dag:
    PythonOperator(task_id="notify_day_shift", python_callable=notify_day_shift)
