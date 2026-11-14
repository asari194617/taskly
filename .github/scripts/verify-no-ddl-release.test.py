import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).with_name("verify-no-ddl-release.py")
SOURCES = ("apps/server/schema.sql", "atlas.hcl")


class VerifyNoDDLReleaseTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.git("init", "-q")
        self.git("config", "user.name", "Synthetic Test")
        self.git("config", "user.email", "synthetic@example.test")
        self.git("config", "commit.gpgsign", "false")
        (self.repo / "apps/server").mkdir(parents=True)
        self.sql = b"CREATE TABLE synthetic_records (id bigint PRIMARY KEY);\n"
        self.atlas = b'// synthetic fixture\nenv "taskly" {}\n'
        (self.repo / SOURCES[0]).write_bytes(self.sql)
        (self.repo / SOURCES[1]).write_bytes(self.atlas)
        self.old = self.commit("synthetic schema baseline")
        (self.repo / "code.txt").write_text("synthetic new application code\n")
        self.target = self.commit("synthetic code-only release")
        self.baseline_file = self.root / "baseline.json"
        self.output = self.root / "proof.json"
        self.github_env = self.root / "github-env"
        self.baseline = {
            "version": 1,
            "target_sha": self.target,
            "baseline_sha": self.old,
            "database_digest": "a" * 64,
            "sources": {
                SOURCES[0]: hashlib.sha256(self.sql).hexdigest(),
                SOURCES[1]: hashlib.sha256(self.atlas).hexdigest(),
            },
        }
        self.docker_calls = self.root / "docker-calls"
        binaries = self.root / "bin"
        binaries.mkdir()
        docker = binaries / "docker"
        docker.write_text(
            '#!/bin/sh\nprintf "unexpected Docker invocation\\n" >> "$MOCK_DOCKER_CALLS"\nexit 99\n'
        )
        docker.chmod(0o700)
        self.env = dict(
            os.environ,
            PATH=str(binaries) + os.pathsep + os.environ["PATH"],
            MOCK_DOCKER_CALLS=str(self.docker_calls),
        )

    def git(self, *args):
        result = subprocess.run(
            ["git", "-C", str(self.repo), *args], capture_output=True, text=True
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        return result.stdout.strip()

    def commit(self, message):
        self.git("add", ".")
        self.git("commit", "-qm", message)
        return self.git("rev-parse", "HEAD")

    def run_verify(
        self, baseline=None, target=None, write_baseline=True, committed_digest=None
    ):
        if write_baseline:
            self.baseline_file.write_text(
                json.dumps(self.baseline if baseline is None else baseline)
            )
        return subprocess.run(
            [
                sys.executable,
                str(SCRIPT),
                "--repo",
                str(self.repo),
                "--target-sha",
                self.target if target is None else target,
                "--baseline-file",
                str(self.baseline_file),
                "--output",
                str(self.output),
                "--github-env",
                str(self.github_env),
                *(
                    []
                    if committed_digest is None
                    else ["--committed-digest", committed_digest]
                ),
            ],
            capture_output=True,
            text=True,
            env=self.env,
        )

    def blocked(self, result):
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("Code-only deployment blocked:", result.stderr)
        self.assertFalse(self.output.exists())
        self.assertFalse(self.github_env.exists())
        self.assertFalse(self.docker_calls.exists())

    def test_unchanged_source_produces_target_bound_proof_without_docker(self):
        result = self.run_verify()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(self.output.read_text()), self.baseline)
        self.assertEqual(self.output.stat().st_mode & 0o777, 0o600)
        self.assertEqual(
            self.github_env.read_text(),
            f"SCHEMA_BASELINE_SHA={self.old}\nSCHEMA_DATABASE_DIGEST={'a' * 64}\nSCHEMA_TARGET_SHA={self.target}\n",
        )
        self.assertNotIn(self.sql.decode(), result.stdout + result.stderr)
        self.assertFalse(self.docker_calls.exists())

    def test_committed_completion_requires_target_as_current_and_expected_digest(self):
        for baseline_sha, digest, expected in [
            (self.target, "a" * 64, "a" * 64),
            (self.old, "a" * 64, "a" * 64),
            (self.target, "a" * 64, "b" * 64),
            (self.target, "a" * 64, "invalid"),
        ]:
            with self.subTest(baseline_sha=baseline_sha, expected=expected):
                self.output.unlink(missing_ok=True)
                self.github_env.unlink(missing_ok=True)
                value = copy.deepcopy(self.baseline)
                value["baseline_sha"] = baseline_sha
                value["database_digest"] = digest
                result = self.run_verify(value, committed_digest=expected)
                if baseline_sha == self.target and digest == expected:
                    self.assertEqual(result.returncode, 0, result.stderr)
                else:
                    self.blocked(result)

    def test_unknown_first_deployment_has_no_inferred_baseline(self):
        self.blocked(self.run_verify(write_baseline=False))

    def test_changed_sql_or_atlas_requires_separate_schema_deployment(self):
        for path in SOURCES:
            with self.subTest(path=path):
                self.git("reset", "--hard", self.target)
                with (self.repo / path).open("ab") as source:
                    source.write(b"// SYNTHETIC_PRIVATE_CHANGE_DO_NOT_LOG\n")
                target = self.commit("synthetic schema change")
                baseline = dict(self.baseline, target_sha=target)
                result = self.run_verify(baseline=baseline, target=target)
                self.blocked(result)
                self.assertIn("separately reviewed schema deployment", result.stderr)
                self.assertNotIn(
                    "SYNTHETIC_PRIVATE_CHANGE_DO_NOT_LOG", result.stdout + result.stderr
                )

    def test_mutated_working_copy_cannot_reuse_a_clean_commit_proof(self):
        (self.repo / SOURCES[0]).write_text("uncommitted synthetic source\n")
        result = self.run_verify()
        self.blocked(result)
        self.assertIn("immutable target", result.stderr)

    def test_symlink_source_cannot_reuse_the_same_bytes(self):
        original = self.repo / SOURCES[1]
        other = self.root / "other-atlas"
        other.write_bytes(original.read_bytes())
        original.unlink()
        original.symlink_to(other)
        self.blocked(self.run_verify())

    def test_symlinked_source_directory_cannot_reuse_the_same_bytes(self):
        original = self.repo / "apps"
        outside = self.root / "outside-apps"
        original.rename(outside)
        original.symlink_to(outside, target_is_directory=True)
        result = self.run_verify()
        self.blocked(result)
        self.assertIn("symlinked path component", result.stderr)

    def test_host_sources_must_match_the_successfully_deployed_git_revision(self):
        baseline = copy.deepcopy(self.baseline)
        baseline["sources"][SOURCES[0]] = "b" * 64
        result = self.run_verify(baseline=baseline)
        self.blocked(result)
        self.assertIn("host schema source differs", result.stderr)

    def test_a_matching_schema_from_an_unrelated_revision_is_not_a_baseline(self):
        tree = self.git("rev-parse", "HEAD^{tree}")
        unrelated = self.git("commit-tree", tree, "-m", "synthetic unrelated root")
        self.blocked(
            self.run_verify(baseline=dict(self.baseline, baseline_sha=unrelated))
        )

    def test_target_must_be_the_exact_immutable_checkout(self):
        for target in ["main", "x" * 40, self.old]:
            with self.subTest(target=target):
                self.blocked(self.run_verify(target=target))

    def test_replacing_the_baseline_cannot_hide_a_changed_schema(self):
        changed = self.sql + b"CREATE TABLE synthetic_change (id bigint);\n"
        (self.repo / SOURCES[0]).write_bytes(changed)
        target = self.commit("synthetic changed target")
        replacement = self.git(
            "commit-tree",
            self.git("rev-parse", "HEAD^{tree}"),
            "-m",
            "synthetic forged baseline",
        )
        self.git("replace", self.old, replacement)
        baseline = copy.deepcopy(self.baseline)
        baseline["target_sha"] = target
        baseline["sources"][SOURCES[0]] = hashlib.sha256(changed).hexdigest()
        self.env["GIT_NO_REPLACE_OBJECTS"] = "0"
        result = self.run_verify(baseline=baseline, target=target)
        self.blocked(result)
        self.assertIn("separately reviewed schema deployment", result.stderr)

    def test_replacing_the_target_cannot_make_dirty_source_look_immutable(self):
        (self.repo / SOURCES[0]).write_bytes(
            self.sql + b"CREATE TABLE synthetic_change (id bigint);\n"
        )
        target = self.commit("synthetic changed target")
        self.git("replace", target, self.target)
        self.git("checkout", self.old, "--", SOURCES[0])
        result = self.run_verify(
            baseline=dict(self.baseline, target_sha=target), target=target
        )
        self.blocked(result)
        self.assertIn("immutable target", result.stderr)

    def test_legacy_grafts_cannot_make_an_unrelated_baseline_an_ancestor(self):
        unrelated = self.git(
            "commit-tree",
            self.git("rev-parse", "HEAD^{tree}"),
            "-m",
            "synthetic unrelated schema baseline",
        )
        graft = self.repo / ".git/info/grafts"
        graft.write_text(f"{self.target} {unrelated}\n")
        result = self.run_verify(baseline=dict(self.baseline, baseline_sha=unrelated))
        self.blocked(result)
        self.assertIn("grafts", result.stderr)

    def test_repository_metadata_and_environment_cannot_redirect_the_proof(self):
        for variable, value in [
            ("GIT_DIR", str(self.repo / ".git")),
            ("GIT_GRAFT_FILE", str(self.root / "grafts")),
            ("GIT_CONFIG_COUNT", "1"),
            ("GIT_CONFIG_KEY_0", "core.worktree"),
            ("GIT_CONFIG_VALUE_0", "SYNTHETIC_PRIVATE_CONFIG_DO_NOT_LOG"),
        ]:
            with self.subTest(variable=variable):
                self.env[variable] = value
                result = self.run_verify()
                self.blocked(result)
                self.assertNotIn(
                    "SYNTHETIC_PRIVATE_CONFIG_DO_NOT_LOG", result.stdout + result.stderr
                )
                del self.env[variable]
        self.git("config", "remote.origin.promisor", "true")
        self.blocked(self.run_verify())
        self.git("config", "--unset", "remote.origin.promisor")
        alternates = self.repo / ".git/objects/info/alternates"
        alternates.write_text(str(self.root / "unknown-object-store") + "\n")
        self.blocked(self.run_verify())

    def test_invalid_baseline_artifacts_do_not_write_proofs(self):
        cases = [
            None,
            [],
            {},
            dict(self.baseline, version=True),
            dict(self.baseline, version=2),
            dict(self.baseline, target_sha=self.old),
            dict(self.baseline, baseline_sha="f" * 40),
            dict(self.baseline, database_digest="not-a-digest"),
            dict(self.baseline, extra="untrusted"),
            dict(self.baseline, sources={SOURCES[0]: "a" * 64}),
            dict(
                self.baseline,
                sources={**self.baseline["sources"], "extra.sql": "a" * 64},
            ),
        ]
        for baseline in cases:
            with self.subTest(baseline_type=type(baseline).__name__):
                self.baseline_file.write_text(json.dumps(baseline))
                self.blocked(self.run_verify(write_baseline=False))
        for raw in [b"not-json", b"x" * 4097, b'{"version":1,"version":1}']:
            self.baseline_file.write_bytes(raw)
            self.blocked(self.run_verify(write_baseline=False))
        self.baseline_file.unlink()
        elsewhere = self.root / "elsewhere-baseline.json"
        elsewhere.write_text(json.dumps(self.baseline))
        self.baseline_file.symlink_to(elsewhere)
        self.blocked(self.run_verify(write_baseline=False))


if __name__ == "__main__":
    unittest.main()
