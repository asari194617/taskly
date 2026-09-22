import sys
import unittest
from pathlib import Path

from airflow.dag_processing.dagbag import DagBag

DAGS = Path(__file__).resolve().parents[1] / "dags"
sys.path.insert(0, str(DAGS))


class DagImportTest(unittest.TestCase):
    def test_python_syntax(self):
        for path in DAGS.glob("*.py"):
            with self.subTest(path=path.name):
                compile(path.read_bytes(), str(path), "exec")

    def test_all_dags_load(self):
        bag = DagBag(dag_folder=str(DAGS), safe_mode=False)
        self.assertEqual(bag.import_errors, {})
        self.assertEqual(
            set(bag.dag_ids),
            {
                "account_erasure",
                "backup_mysql",
                "day_shift_index_increment",
                "day_shift_notify_0830",
                "moderation_daily",
                "notifications",
            },
        )


if __name__ == "__main__":
    unittest.main()
