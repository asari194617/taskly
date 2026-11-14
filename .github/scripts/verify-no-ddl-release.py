#!/usr/bin/env python3
"""Require an attested unchanged schema before the code-only production path."""

import argparse
import hashlib
import json
import os
import re
import subprocess
from pathlib import Path


SOURCES = ("apps/server/schema.sql", "atlas.hcl")
SHA = re.compile(r"[0-9a-f]{40}\Z")
DIGEST = re.compile(r"[0-9a-f]{64}\Z")
GIT_OVERRIDES = {
    "GIT_DIR",
    "GIT_COMMON_DIR",
    "GIT_WORK_TREE",
    "GIT_OBJECT_DIRECTORY",
    "GIT_ALTERNATE_OBJECT_DIRECTORIES",
    "GIT_INDEX_FILE",
    "GIT_NAMESPACE",
    "GIT_REPLACE_REF_BASE",
    "GIT_SHALLOW_FILE",
    "GIT_GRAFT_FILE",
    "GIT_CONFIG",
    "GIT_CONFIG_GLOBAL",
    "GIT_CONFIG_SYSTEM",
    "GIT_CONFIG_NOSYSTEM",
    "GIT_CONFIG_PARAMETERS",
    "GIT_CONFIG_COUNT",
}


def fail(message):
    raise SystemExit("Code-only deployment blocked: " + message)


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            fail("duplicate baseline fields")
        result[key] = value
    return result


def git(repo, *args, no_match=False):
    # Persistent runners can retain refs/replace. A named immutable commit must
    # always refer to its actual object, including when checking its ancestors.
    if any(
        name in GIT_OVERRIDES
        or name.startswith(("GIT_CONFIG_KEY_", "GIT_CONFIG_VALUE_"))
        for name in os.environ
    ):
        fail("Git repository/history environment overrides are not permitted")
    environment = dict(
        os.environ,
        GIT_NO_REPLACE_OBJECTS="1",
        GIT_GRAFT_FILE=os.devnull,
        GIT_NO_LAZY_FETCH="1",
    )
    result = subprocess.run(
        ["git", "-c", "core.commitGraph=false", "-C", str(repo), *args],
        capture_output=True,
        env=environment,
    )
    if result.returncode != 0 and not (
        no_match and result.returncode == 1 and not result.stdout
    ):
        fail("immutable Git source or history is unavailable")
    return result.stdout


def canonical_git_repository(repo):
    if (
        Path(git(repo, "rev-parse", "--show-toplevel").decode().strip()).resolve()
        != repo
    ):
        fail("the requested directory is not the checkout root")
    for argument, expected in [
        ("--is-bare-repository", "false"),
        ("--is-shallow-repository", "false"),
        ("--show-object-format", "sha1"),
    ]:
        if git(repo, "rev-parse", argument).decode().strip() != expected:
            fail("a complete canonical SHA-1 checkout is required")
    for argument in ["--git-common-dir", "--absolute-git-dir"]:
        directory = Path(
            git(repo, "rev-parse", "--path-format=absolute", argument).decode().strip()
        )
        for name in ["info/grafts", "objects/info/alternates"]:
            metadata = directory / name
            if metadata.exists() or metadata.is_symlink():
                fail("Git grafts or alternate object stores require a clean checkout")
    if git(
        repo,
        "config",
        "--get-regexp",
        r"^(extensions\.partialclone|remote\..*\.promisor|core\.alternaterefscommand)$",
        no_match=True,
    ):
        fail("Git partial-clone or alternate-reference configuration is not permitted")


def source_blob(repo, revision, path):
    object_name = revision + ":" + path
    size = git(repo, "cat-file", "-s", object_name).decode().strip()
    if not size.isdigit() or not 0 < int(size) <= 8 * 1024 * 1024:
        fail("schema source is empty or oversized")
    return git(repo, "cat-file", "blob", object_name)


def verify(repo, target, baseline_file):
    if not SHA.fullmatch(target):
        fail("target must be an immutable commit SHA")
    repo = Path(repo).resolve()
    canonical_git_repository(repo)
    if git(repo, "rev-parse", "HEAD").decode().strip() != target:
        fail("checkout HEAD does not match the target release")
    path = Path(baseline_file)
    if path.is_symlink() or not path.is_file() or not 0 < path.stat().st_size <= 4096:
        fail("a bounded regular baseline artifact is required")
    try:
        baseline = json.loads(path.read_bytes(), object_pairs_hook=unique_object)
    except (ValueError, UnicodeError):
        fail("baseline artifact is invalid")
    if not isinstance(baseline, dict) or set(baseline) != {
        "version",
        "target_sha",
        "baseline_sha",
        "database_digest",
        "sources",
    }:
        fail("baseline artifact fields are invalid")
    old = baseline["baseline_sha"]
    database = baseline["database_digest"]
    sources = baseline["sources"]
    if (
        type(baseline["version"]) is not int
        or baseline["version"] != 1
        or baseline["target_sha"] != target
        or not isinstance(old, str)
        or not SHA.fullmatch(old)
        or not isinstance(database, str)
        or not DIGEST.fullmatch(database)
        or not isinstance(sources, dict)
        or set(sources) != set(SOURCES)
        or any(
            not isinstance(value, str) or not DIGEST.fullmatch(value)
            for value in sources.values()
        )
    ):
        fail("baseline is not bound to this target and known schema sources")
    git(repo, "merge-base", "--is-ancestor", old, target)
    for name in SOURCES:
        previous = source_blob(repo, old, name)
        desired = source_blob(repo, target, name)
        current = repo
        for component in Path(name).parts:
            current = current / component
            if current.is_symlink():
                fail("working schema source has a symlinked path component")
        if not current.is_file() or current.read_bytes() != desired:
            fail("working schema source does not match the immutable target")
        if previous != desired:
            fail("schema changed; use a separately reviewed schema deployment")
        if hashlib.sha256(previous).hexdigest() != sources[name]:
            fail(
                "host schema source differs from its successfully deployed Git revision"
            )
    return baseline


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=".")
    parser.add_argument("--target-sha", required=True)
    parser.add_argument("--baseline-file", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--github-env")
    args = parser.parse_args()
    proof = verify(args.repo, args.target_sha, args.baseline_file)
    output = Path(args.output)
    output.write_text(json.dumps(proof, sort_keys=True) + "\n")
    output.chmod(0o600)
    if args.github_env:
        with Path(args.github_env).open("a") as environment:
            environment.write(
                "SCHEMA_BASELINE_SHA=" + proof["baseline_sha"] + "\n"
                "SCHEMA_DATABASE_DIGEST=" + proof["database_digest"] + "\n"
                "SCHEMA_TARGET_SHA=" + args.target_sha + "\n"
            )
    print(
        "Verified unchanged authoritative SQL and Atlas configuration for this code-only release."
    )


if __name__ == "__main__":
    main()
