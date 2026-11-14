#!/usr/bin/env python3
import copy
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
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

    def test_api_bounded_success_and_failure(self):
        result = subprocess.CompletedProcess([], 0, b"{}", b"")
        with patch.object(m.subprocess, "run", return_value=result) as run:
            self.assertEqual(m.api("fixture"), b"{}")
            self.assertEqual(run.call_args.kwargs["timeout"], 30)
            self.assertEqual(
                run.call_args.args[0], ["gh", "api", "repos/asari194617/taskly/fixture"]
            )
        for result in [
            subprocess.CompletedProcess([], 1, b"", b"private"),
            subprocess.CompletedProcess([], 0, b"a" * (1024 * 1024 + 1), b""),
        ]:
            with (
                patch.object(m.subprocess, "run", return_value=result),
                self.assertRaises(SystemExit),
            ):
                m.api("fixture")

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


if __name__ == "__main__":
    unittest.main()
