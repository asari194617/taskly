import ast
import pathlib
import unittest


SOURCE = pathlib.Path(__file__).parents[1] / "dags" / "day_shift_notify_0830.py"
tree = ast.parse(SOURCE.read_text())
selector = next(node for node in tree.body if isinstance(node, ast.FunctionDef)
                and node.name == "select_day_shift_recipients")
namespace = {}
exec(compile(ast.Module(body=[selector], type_ignores=[]), str(SOURCE), "exec"), namespace)


class DayShiftRecipientTest(unittest.TestCase):
    def test_channel_settings_and_blocked_users(self):
        users = [{"id": uid, "email": f"{uid}@example.test"} for uid in range(1, 6)]
        enabled, blocked, email, push = namespace["select_day_shift_recipients"](
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
