"""Unit tests for auth-service account erasure synchronization."""

import os
import sys
import unittest
from pathlib import Path
from unittest import mock
from urllib import error

try:
    import pymysql  # noqa: F401
except ModuleNotFoundError:
    sys.modules["pymysql"] = mock.Mock()

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "dags"))
import account_erasure_worker


class _Response:
    def __init__(self, status):
        self.status = status

    def close(self):
        pass


class AccountErasureAuthSyncTest(unittest.TestCase):
    def test_disabled_does_not_call_auth(self):
        with mock.patch.dict(os.environ, {"AUTH_ACCOUNT_LIFECYCLE_EXTERNAL": "0"}, clear=False):
            with mock.patch.object(account_erasure_worker._auth_opener, "open") as open_request:
                account_erasure_worker.erase_auth_user(42)
                open_request.assert_not_called()

    def test_delete_uses_internal_token(self):
        environment = {
            "AUTH_ACCOUNT_LIFECYCLE_EXTERNAL": "1",
            "AUTH_LIFECYCLE_SYNC_URL": "https://auth.example.test",
            "AUTH_INTERNAL_TOKEN": "internal-test-token",
            "NODE_ENV": "development",
        }
        with mock.patch.dict(os.environ, environment, clear=False):
            with mock.patch.object(account_erasure_worker._auth_opener, "open", return_value=_Response(204)) as open_request:
                account_erasure_worker.erase_auth_user(42)
        req = open_request.call_args.args[0]
        self.assertEqual(req.full_url, "https://auth.example.test/internal/users/42")
        self.assertEqual(req.method, "DELETE")
        self.assertEqual(req.get_header("Authorization"), "Bearer internal-test-token")
        self.assertEqual(open_request.call_args.kwargs["timeout"], 5)

    def test_production_rejects_unsafe_http(self):
        environment = {
            "AUTH_ACCOUNT_LIFECYCLE_EXTERNAL": "1",
            "AUTH_LIFECYCLE_SYNC_URL": "http://auth.example.test",
            "AUTH_INTERNAL_TOKEN": "x" * 32,
            "NODE_ENV": "production",
        }
        with mock.patch.dict(os.environ, environment, clear=False):
            with self.assertRaisesRegex(RuntimeError, "not configured safely"):
                account_erasure_worker.erase_auth_user(42)

    def test_error_does_not_include_response_body(self):
        environment = {
            "AUTH_ACCOUNT_LIFECYCLE_EXTERNAL": "1",
            "AUTH_LIFECYCLE_SYNC_URL": "https://auth.example.test",
            "AUTH_INTERNAL_TOKEN": "internal-test-token",
            "NODE_ENV": "development",
        }
        failure = error.HTTPError(
            "https://auth.example.test/internal/users/42", 503, "secret detail", {}, None
        )
        with mock.patch.dict(os.environ, environment, clear=False):
            with mock.patch.object(account_erasure_worker._auth_opener, "open", side_effect=failure):
                with self.assertRaisesRegex(RuntimeError, "status 503") as raised:
                    account_erasure_worker.erase_auth_user(42)
        self.assertNotIn("secret detail", str(raised.exception))


if __name__ == "__main__":
    unittest.main()
