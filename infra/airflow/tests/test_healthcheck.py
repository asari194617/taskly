import importlib.util
import os
from pathlib import Path
import sys
import unittest
from unittest import mock


spec = importlib.util.spec_from_file_location(
    "taskly_airflow_healthcheck", Path(__file__).resolve().parents[1] / "healthcheck.py"
)
healthcheck = importlib.util.module_from_spec(spec)
spec.loader.exec_module(healthcheck)


class HealthcheckTest(unittest.TestCase):
    def setUp(self):
        self.original_connect = healthcheck.open_connection
        patcher = mock.patch.object(healthcheck, "open_connection")
        self.connect = patcher.start()
        self.addCleanup(patcher.stop)
        self.connection = self.connect.return_value
        self.cursor = self.connection.cursor.return_value
        patcher = mock.patch.dict(os.environ, {}, clear=True)
        patcher.start()
        self.addCleanup(patcher.stop)

    def test_only_fresh_running_local_job_is_healthy(self):
        for job_type in healthcheck.THRESHOLDS:
            for row, expected in [
                (("running", 5), True),
                (("running", 29.9), True),
                (("running", 30), False),
                (("running", 90), False),
                (("success", 5), False),
                (("failed", 5), False),
                (("running", None), False),
                (None, False),
            ]:
                with self.subTest(job_type=job_type, row=row):
                    self.cursor.fetchone.return_value = row
                    with mock.patch.object(healthcheck.socket, "gethostname", return_value="local-host"):
                        self.assertEqual(healthcheck.is_healthy(job_type), expected)
                    self.assertEqual(self.cursor.execute.call_args.args[1], (job_type, "local-host"))
                    self.cursor.close.assert_called()
                    self.connection.close.assert_called()

    def test_configured_threshold(self):
        for job_type, variable in healthcheck.THRESHOLDS.items():
            with self.subTest(job_type=job_type), mock.patch.dict(os.environ, {variable: "60"}):
                self.cursor.fetchone.return_value = ("running", 45)
                self.assertTrue(healthcheck.is_healthy(job_type))

    def test_query_failure_closes_resources(self):
        self.cursor.execute.side_effect = RuntimeError("query failed")
        with self.assertRaises(RuntimeError):
            healthcheck.is_healthy("SchedulerJob")
        self.cursor.close.assert_called_once()
        self.connection.close.assert_called_once()

    def test_connection_uses_utc_for_timestamp_columns(self):
        connector = mock.MagicMock()
        with mock.patch.dict(sys.modules, {"mysql": mock.MagicMock(connector=connector), "mysql.connector": connector}), mock.patch.dict(
            os.environ,
            {"AIRFLOW_META_USER": "test", "AIRFLOW_META_PASSWORD": "test", "AIRFLOW_META_DATABASE": "metadata"},
        ):
            self.original_connect()
        connector.connect.assert_called_once_with(
            host="db", port=3306, user="test", password="test", database="metadata",
            connection_timeout=3, time_zone="+00:00",
        )

    def test_cli_fails_closed_without_logging_connection_details(self):
        self.connect.side_effect = RuntimeError("secret password")
        with mock.patch.object(sys, "argv", ["healthcheck.py", "SchedulerJob"]), mock.patch.object(sys, "stderr") as stderr:
            self.assertEqual(healthcheck.main(), 1)
        self.assertNotIn("secret password", str(stderr.write.call_args_list))


if __name__ == "__main__":
    unittest.main()
