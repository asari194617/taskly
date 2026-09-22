import sys
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "dags"))
from day_shift_recipients import select_day_shift_recipients


class DayShiftRecipientTest(unittest.TestCase):
    def test_channel_settings_and_blocked_users(self):
        users = [{"id": uid, "email": f"{uid}@example.test"} for uid in range(1, 6)]
        enabled, blocked, email, push = select_day_shift_recipients(
            users,
            {5: 1},
            {4: 0},
            {(1, "WEBPUSH"): 1, (2, "EMAIL"): 0,
             (2, "WEBPUSH"): 1, (3, "WEBPUSH"): 1},
            {1, 2},
        )
        self.assertEqual([u["id"] for u in enabled], [1, 2, 3])
        self.assertEqual([u["id"] for u in blocked], [5])
        self.assertEqual([u["id"] for u in email], [1, 3])
        self.assertEqual([u["id"] for u in push], [1, 2])


if __name__ == "__main__":
    unittest.main()
