"""MySQL 8.4 integration test for the account erasure transaction."""

import os
import sys
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path


@unittest.skipUnless(os.environ.get("TEST_ACCOUNT_ERASURE_DB") == "1", "requires disposable MySQL")
class AccountErasureIntegrationTest(unittest.TestCase):
    def test_due_account_erased_and_hold_respected(self):
        import pymysql

        sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "dags"))
        from account_erasure_worker import erase_due_accounts

        now = datetime.now(timezone.utc).replace(tzinfo=None)
        db = pymysql.connect(
            host=os.environ["DB_HOST"], port=int(os.environ["DB_PORT"]),
            user=os.environ["MYSQL_USER"], password=os.environ["MYSQL_PASSWORD"],
            database=os.environ["MYSQL_DATABASE"], autocommit=False,
        )
        try:
            with db.cursor() as cursor:
                ids = []
                for name in ("due", "held"):
                    cursor.execute(
                        "INSERT INTO users(email, is_deleted, deleted_at) VALUES (%s, 1, %s)",
                        (f"erasure-{name}@example.test", now - timedelta(days=31)),
                    )
                    ids.append(cursor.lastrowid)
                due, held = ids
                cursor.execute("INSERT INTO account_erasure_holds(user_id, reason, review_at) VALUES (%s, 'test hold', %s)", (held, now + timedelta(days=1)))
                cursor.execute("INSERT INTO subjects(name_ja) VALUES ('erasure test')")
                subject = cursor.lastrowid
                cursor.execute("INSERT INTO tasks(added_user_id, title, subject_id, deadline) VALUES (%s, 'erase this task', %s, '2026-10-01')", (due, subject))
                task = cursor.lastrowid
                cursor.execute("INSERT INTO tasks_users_completions(user_id, task_id) VALUES (%s, %s)", (held, task))
                cursor.execute("INSERT INTO notifications_queue(kind, when_label, subject, title, user_id, event_key) VALUES ('task', 'today', 'subject', 'erase', %s, %s)", (held, f"task:2026-09-22:morning:{task}:webpush:{held}"))
                cursor.execute("INSERT INTO integration_jobs(kind, payload, status) VALUES ('task_discord', %s, 'done')", (f'{{"Task":{{"AddedUserID":{due}}}}}',))
            db.commit()

            self.assertEqual(erase_due_accounts(db, now), 1)
            with db.cursor() as cursor:
                for table, column, value in (("users", "id", due), ("tasks", "id", task), ("tasks_users_completions", "task_id", task), ("notifications_queue", "user_id", held), ("integration_jobs", "kind", "task_discord")):
                    cursor.execute(f"SELECT COUNT(*) FROM {table} WHERE {column} = %s", (value,))
                    self.assertEqual(cursor.fetchone()[0], 0, table)
                cursor.execute("SELECT COUNT(*) FROM users WHERE id = %s", (held,))
                self.assertEqual(cursor.fetchone()[0], 1)
                cursor.execute("DELETE FROM account_erasure_holds WHERE user_id = %s", (held,))
            db.commit()
            self.assertEqual(erase_due_accounts(db, now), 1)
            self.assertEqual(erase_due_accounts(db, now), 0)
        finally:
            db.close()
