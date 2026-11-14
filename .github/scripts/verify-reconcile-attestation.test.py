#!/usr/bin/env python3
import copy
import hashlib
import http.client
import http.server
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest.mock import patch
import zipfile

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location(
    "reconcile", HERE / "verify-reconcile-attestation.py"
)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)


def metadata():
    run = dict(
        id=m.RUN,
        run_attempt=1,
        status="completed",
        conclusion="failure",
        event="push",
        head_branch="main",
        head_sha=m.TARGET,
        path=".github/workflows/deploy-prod.yml",
        repository=dict(id=m.REPOSITORY_ID, full_name=m.REPOSITORY),
        head_repository=dict(id=m.REPOSITORY_ID),
    )
    job = dict(
        id=m.JOB,
        run_id=m.RUN,
        run_attempt=1,
        name="deploy",
        status="completed",
        conclusion="failure",
        runner_name="taskly-prod",
        steps=[dict(name=name, conclusion="success") for name in m.SUCCESS_STEPS]
        + [
            dict(name="Remote deploy", conclusion="failure"),
            dict(name="Mark production deployment complete", conclusion="skipped"),
        ],
    )
    artifact = dict(
        id=m.ARTIFACT,
        name="taskly-code-only-schema-" + m.TARGET,
        expired=False,
        size_in_bytes=853,
        digest="sha256:" + m.ZIP_SHA256,
        workflow_run=dict(
            id=m.RUN,
            head_sha=m.TARGET,
            head_branch="main",
            repository_id=m.REPOSITORY_ID,
            head_repository_id=m.REPOSITORY_ID,
        ),
    )
    return run, job, artifact


def archive(entries=None, compression=zipfile.ZIP_DEFLATED):
    out = io.BytesIO()
    with zipfile.ZipFile(out, "w", compression=compression) as z:
        for name, payload in entries or [
            ("taskly-schema-baseline.json", b'{"fixture":1}'),
            ("taskly-schema-proof.json", b'{"fixture":1}'),
        ]:
            z.writestr(name, payload)
    return out.getvalue()


class VerifyTests(unittest.TestCase):
    def test_exact_failed_release_metadata(self):
        m.validate_metadata(*metadata())

    def test_untrusted_or_active_run_and_wrong_phase(self):
        for field, bad in [
            ("id", 3),
            ("run_attempt", 2),
            ("status", "in_progress"),
            ("conclusion", "success"),
            ("event", "pull_request"),
            ("head_branch", "feature"),
            ("head_sha", "a" * 40),
            ("path", ".github/workflows/other.yml"),
            ("repository", {}),
            ("head_repository", {}),
        ]:
            values = list(metadata())
            values[0][field] = bad
            with self.subTest(field=field), self.assertRaises(SystemExit):
                m.validate_metadata(*values)
        for field, bad in [
            ("id", 3),
            ("run_id", 4),
            ("run_attempt", 2),
            ("status", "in_progress"),
            ("conclusion", "cancelled"),
            ("runner_name", "other"),
        ]:
            values = list(metadata())
            values[1][field] = bad
            with self.subTest(field=field), self.assertRaises(SystemExit):
                m.validate_metadata(*values)
        for mode in ("missing", "duplicate", "failed", "complete"):
            values = list(metadata())
            if mode == "missing":
                values[1]["steps"].pop(0)
            elif mode == "duplicate":
                values[1]["steps"].append(copy.deepcopy(values[1]["steps"][0]))
            elif mode == "failed":
                values[1]["steps"][0]["conclusion"] = "failure"
            else:
                values[1]["steps"][-1]["conclusion"] = "success"
            with self.subTest(mode=mode), self.assertRaises(SystemExit):
                m.validate_metadata(*values)

    def test_artifact_identity_expiry_size_digest_origin(self):
        for field, bad in [
            ("id", 2),
            ("name", "wrong"),
            ("expired", True),
            ("size_in_bytes", 0),
            ("size_in_bytes", True),
            ("size_in_bytes", 16385),
            ("digest", "sha256:" + "b" * 64),
            ("workflow_run", {}),
        ]:
            values = list(metadata())
            values[2][field] = bad
            with self.subTest(field=field, bad=bad), self.assertRaises(SystemExit):
                m.validate_metadata(*values)

    def test_archive_valid_and_no_path_extraction(self):
        data = archive()
        with patch.object(m, "ZIP_SHA256", hashlib.sha256(data).hexdigest()):
            self.assertEqual(m.unpack(data), b'{"fixture":1}')
        with self.assertRaises(SystemExit):
            m.unpack(data)
        for names in [
            (".. /baseline.json", "taskly-schema-proof.json"),
            ("../baseline.json", "taskly-schema-proof.json"),
            ("taskly-schema-baseline.json", "taskly-schema-baseline.json"),
        ]:
            data = archive([(name, b"{}") for name in names])
            with (
                patch.object(m, "ZIP_SHA256", hashlib.sha256(data).hexdigest()),
                self.assertRaises(SystemExit),
            ):
                m.unpack(data)

    def test_archive_bounds_ratio_duplicate_json_and_mismatched_proof(self):
        for entries in [
            [("taskly-schema-baseline.json", b""), ("taskly-schema-proof.json", b"{}")],
            [
                ("taskly-schema-baseline.json", b"a" * 4097),
                ("taskly-schema-proof.json", b"{}"),
            ],
            [
                ("taskly-schema-baseline.json", b" " * 3000),
                ("taskly-schema-proof.json", b"{}"),
            ],
            [
                ("taskly-schema-baseline.json", b'{"a":1,"a":2}'),
                ("taskly-schema-proof.json", b"{}"),
            ],
            [
                ("taskly-schema-baseline.json", b"{}"),
                ("taskly-schema-proof.json", b'{"a":1}'),
            ],
            [
                ("taskly-schema-baseline.json", b"\xff"),
                ("taskly-schema-proof.json", b"{}"),
            ],
        ]:
            data = archive(entries)
            with (
                patch.object(m, "ZIP_SHA256", hashlib.sha256(data).hexdigest()),
                self.assertRaises(SystemExit),
            ):
                m.unpack(data)
        for data in (b"", b"badzip", b"a" * 16385):
            with (
                patch.object(m, "ZIP_SHA256", hashlib.sha256(data).hexdigest()),
                self.assertRaises(SystemExit),
            ):
                m.unpack(data)

    def test_archive_symlink_rejected(self):
        out = io.BytesIO()
        with zipfile.ZipFile(out, "w") as z:
            info = zipfile.ZipInfo("taskly-schema-baseline.json")
            info.external_attr = 0o120777 << 16
            z.writestr(info, b"{}")
            z.writestr("taskly-schema-proof.json", b"{}")
        data = out.getvalue()
        with (
            patch.object(m, "ZIP_SHA256", hashlib.sha256(data).hexdigest()),
            self.assertRaises(SystemExit),
        ):
            m.unpack(data)

    def test_main_real_immutable_git_guard_and_private_output(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            repo = root / "repo"
            repo.mkdir()

            def git(*args):
                return (
                    subprocess.check_output(
                        ["git", "-C", str(repo), *args], stderr=subprocess.DEVNULL
                    )
                    .decode()
                    .strip()
                )

            git("init", "-q")
            git("config", "commit.gpgsign", "false")
            git("config", "user.name", "Fixture")
            git("config", "user.email", "fixture@example.test")
            (repo / "apps/server").mkdir(parents=True)
            (repo / "apps/server/schema.sql").write_text(
                "CREATE TABLE fixture (id BIGINT);\n"
            )
            (repo / "atlas.hcl").write_text('env "fixture" {}\n')
            git("add", ".")
            git("commit", "-qm", "fixture")
            sha = git("rev-parse", "HEAD")
            baseline = dict(
                version=1,
                target_sha=sha,
                baseline_sha=sha,
                database_digest="a" * 64,
                sources={
                    name: hashlib.sha256((repo / name).read_bytes()).hexdigest()
                    for name in ("apps/server/schema.sql", "atlas.hcl")
                },
            )
            payload = json.dumps(baseline).encode()
            data = archive([(name, payload) for name in m.NAMES])
            zipsha = hashlib.sha256(data).hexdigest()
            with patch.object(m, "TARGET", sha), patch.object(m, "ZIP_SHA256", zipsha):
                run, job, artifact = metadata()
                responses = [json.dumps(v).encode() for v in (run, job, artifact)] + [
                    data
                ]
                env = root / "github-env"
                output = root / "proof"
                args = [
                    "guard",
                    "--repo",
                    str(repo),
                    "--output-dir",
                    str(output),
                    "--github-env",
                    str(env),
                ]
                with (
                    patch.object(m, "api", side_effect=responses),
                    patch.object(sys, "argv", args),
                ):
                    m.main()
                self.assertEqual(
                    json.loads((output / "verified.json").read_text()), baseline
                )
                self.assertEqual(output.stat().st_mode & 0o777, 0o700)
                self.assertEqual(
                    (output / "baseline.json").stat().st_mode & 0o777, 0o600
                )
                self.assertIn("SCHEMA_TARGET_SHA=" + sha, env.read_text())
                (repo / "apps/server/schema.sql").write_text("changed")
                responses = [json.dumps(v).encode() for v in (run, job, artifact)] + [
                    data
                ]
                args[args.index(str(output))] = str(root / "rejected")
                with (
                    patch.object(m, "api", side_effect=responses),
                    patch.object(sys, "argv", args),
                    self.assertRaises(SystemExit),
                ):
                    m.main()


class HTTPTests(unittest.TestCase):
    def setUp(self):
        self.servers = []
        self.seen = []
        self.original_connection = http.client.HTTPConnection
        self.environment = patch.dict(
            os.environ, {"GH_TOKEN": "synthetic-bearer-secret"}
        )
        self.environment.start()

    def tearDown(self):
        self.environment.stop()
        for server, thread in self.servers:
            server.shutdown()
            server.server_close()
            thread.join()

    def server(self, callback):
        seen = self.seen

        class Handler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                seen.append((self.path, self.headers.get("Authorization")))
                try:
                    callback(self)
                except (BrokenPipeError, ConnectionResetError):
                    pass

            def log_message(self, *args):
                pass

        server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.servers.append((server, thread))
        return server.server_address[1]

    @staticmethod
    def reply(handler, status=200, body=b"{}", headers=()):
        handler.send_response(status)
        for name, value in headers:
            handler.send_header(name, value)
        handler.end_headers()
        handler.wfile.write(body)

    def connect(self, ports):
        original = self.original_connection
        seen = []

        def factory(host, timeout):
            seen.append((host, timeout))
            if host not in ports:
                raise AssertionError("Unexpected network destination")
            return original("127.0.0.1", ports[host], timeout=timeout)

        return patch.object(m.http.client, "HTTPSConnection", side_effect=factory), seen

    def test_api_auth_and_artifact_redirect_strips_token(self):
        def api_reply(handler):
            if handler.path.endswith("/zip"):
                self.reply(
                    handler,
                    302,
                    b"",
                    [
                        (
                            "Location",
                            "https://" + m.ARTIFACT_HOST + "/fixture.zip?sig=fixture",
                        )
                    ],
                )
            else:
                self.reply(handler, body=b'{"fixture":true}')

        api_port = self.server(api_reply)
        download_port = self.server(
            lambda handler: self.reply(handler, body=b"fixture-zip")
        )
        transport, calls = self.connect(
            {m.API_HOST: api_port, m.ARTIFACT_HOST: download_port}
        )
        with transport:
            self.assertEqual(m.api("actions/runs/" + str(m.RUN)), b'{"fixture":true}')
            self.assertEqual(
                m.api("actions/artifacts/" + str(m.ARTIFACT) + "/zip"), b"fixture-zip"
            )
        self.assertEqual(
            [host for host, _ in calls], [m.API_HOST, m.API_HOST, m.ARTIFACT_HOST]
        )
        self.assertTrue(all(timeout == 15 for _, timeout in calls))
        self.assertEqual(
            [auth for _, auth in self.seen],
            ["Bearer synthetic-bearer-secret", "Bearer synthetic-bearer-secret", None],
        )
        self.assertEqual(self.seen[-1][0], "/fixture.zip?sig=fixture")

    def test_missing_invalid_token_and_unknown_path_never_connect(self):
        with patch.object(m.http.client, "HTTPSConnection") as connection:
            for token in ("", "line\r\ninjection", "space value", "nonascii-\u00e9"):
                with (
                    patch.dict(os.environ, {"GH_TOKEN": token}),
                    self.assertRaises(SystemExit),
                ):
                    m.api("actions/runs/" + str(m.RUN))
            with self.assertRaises(SystemExit):
                m.api("actions/runs/other")
            connection.assert_not_called()

    def test_untrusted_redirect_targets_never_connect(self):
        target = [""]
        port = self.server(
            lambda handler: self.reply(handler, 302, b"", [("Location", target[0])])
        )
        transport, calls = self.connect({m.API_HOST: port})
        bad = [
            "http://" + m.ARTIFACT_HOST + "/fixture",
            "https://evil.example/fixture",
            "https://" + m.ARTIFACT_HOST + ".evil.example/fixture",
            "https://user@" + m.ARTIFACT_HOST + "/fixture",
            "https://" + m.ARTIFACT_HOST + ":443/fixture",
            "https://" + m.ARTIFACT_HOST + "/fixture#fragment",
            "//" + m.ARTIFACT_HOST + "/fixture",
            "https://" + m.ARTIFACT_HOST + "/has space",
            "https://" + m.ARTIFACT_HOST,
        ]
        with transport:
            for location in bad:
                target[0] = location
                with self.subTest(location=location), self.assertRaises(SystemExit):
                    m.api("actions/artifacts/" + str(m.ARTIFACT) + "/zip")
        self.assertTrue(all(host == m.API_HOST for host, _ in calls))
        for location in (
            None,
            "",
            "x" * 16385,
            "https://[bad",
            "https://" + m.ARTIFACT_HOST + "/\n",
        ):
            with self.assertRaises(SystemExit):
                m.download_target(location)

    def test_non_success_metadata_redirect_and_artifact_second_redirect_fail_closed(
        self,
    ):
        status = [401]
        port = self.server(
            lambda handler: self.reply(
                handler,
                status[0],
                b"private-error-body",
                [("Location", "https://evil.example/")],
            )
        )
        transport, _ = self.connect({m.API_HOST: port})
        with transport:
            for code in (301, 302, 401, 403, 404, 500):
                status[0] = code
                with self.assertRaises(SystemExit) as failed:
                    m.api("actions/runs/" + str(m.RUN))
                self.assertNotIn("private-error", str(failed.exception))
                self.assertNotIn("synthetic-bearer", str(failed.exception))
            for code in (200, 301, 403):
                status[0] = code
                with self.assertRaises(SystemExit):
                    m.api("actions/artifacts/" + str(m.ARTIFACT) + "/zip")
        first = self.server(
            lambda handler: self.reply(
                handler,
                302,
                b"",
                [("Location", "https://" + m.ARTIFACT_HOST + "/fixture")],
            )
        )
        second = self.server(
            lambda handler: self.reply(
                handler, 302, b"", [("Location", "https://evil.example/fixture")]
            )
        )
        transport, calls = self.connect({m.API_HOST: first, m.ARTIFACT_HOST: second})
        with transport, self.assertRaises(SystemExit):
            m.api("actions/artifacts/" + str(m.ARTIFACT) + "/zip")
        self.assertEqual(len(calls), 2)
        self.assertIsNone(self.seen[-1][1])

    def test_body_bounds_length_duplicates_and_partial_payload(self):
        configuration = [b"{}", ()]
        port = self.server(
            lambda handler: self.reply(
                handler, body=configuration[0], headers=configuration[1]
            )
        )
        transport, _ = self.connect({m.API_HOST: port})
        with transport:
            for body, headers in [
                (b"x" * (1024 * 1024 + 1), ()),
                (b"x", (("Content-Length", "1048577"),)),
                (b"x", (("Content-Length", "invalid"),)),
                (b"x", (("Content-Length", "-1"),)),
                (b"x", (("Content-Length", "1"), ("Content-Length", "1"))),
                (b"x", (("Content-Length", "2"),)),
            ]:
                configuration[:] = [body, headers]
                with self.assertRaises(SystemExit):
                    m.api("actions/runs/" + str(m.RUN))
        # The download cap is smaller than JSON and is checked independently.
        port = self.server(lambda handler: self.reply(handler, body=b"x" * 16385))
        transport, _ = self.connect({m.ARTIFACT_HOST: port})
        with transport, self.assertRaises(SystemExit):
            m.request(m.ARTIFACT_HOST, "/fixture", {}, 16384)

    def test_redirect_location_missing_or_duplicate_rejected(self):
        headers = [[]]
        port = self.server(lambda handler: self.reply(handler, 302, b"", headers[0]))
        transport, _ = self.connect({m.API_HOST: port})
        with transport:
            for locations in (
                [],
                [
                    ("Location", "https://" + m.ARTIFACT_HOST + "/one"),
                    ("Location", "https://" + m.ARTIFACT_HOST + "/two"),
                ],
            ):
                headers[0] = locations
                with self.assertRaises(SystemExit):
                    m.api("actions/artifacts/" + str(m.ARTIFACT) + "/zip")

    def test_network_constructor_protocol_and_timeout_errors_are_sanitized(self):
        for failure in (
            OSError("synthetic-bearer-secret"),
            http.client.HTTPException("private-response"),
        ):
            with (
                patch.object(m.http.client, "HTTPSConnection", side_effect=failure),
                self.assertRaises(SystemExit) as result,
            ):
                m.api("actions/runs/" + str(m.RUN))
            self.assertNotIn("secret", str(result.exception))
            self.assertNotIn("private-response", str(result.exception))
        port = self.server(lambda handler: (time.sleep(0.1), self.reply(handler)))
        transport, _ = self.connect({m.API_HOST: port})
        with (
            transport,
            patch.object(m, "HTTP_TIMEOUT", 0.02),
            self.assertRaises(SystemExit),
        ):
            m.api("actions/runs/" + str(m.RUN))


if __name__ == "__main__":
    unittest.main()
