#!/usr/bin/env python3
"""Validate the host-selected unfinished release before immutable Git checks."""

import argparse
import json
import re
from pathlib import Path

SHA = re.compile(r"[0-9a-f]{40}\Z")
DIGEST = re.compile(r"[0-9a-f]{64}\Z")
SOURCES = {"apps/server/schema.sql", "atlas.hcl"}


def fail():
    raise SystemExit("Reconciliation blocked: unfinished deployment baseline is invalid")


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            fail()
        result[key] = value
    return result


def validate(path):
    source = Path(path)
    if source.is_symlink() or not source.is_file() or not 0 < source.stat().st_size <= 4096:
        fail()
    try:
        proof = json.loads(source.read_bytes(), object_pairs_hook=unique_object)
    except (ValueError, UnicodeError, OSError):
        fail()
    if not isinstance(proof, dict) or set(proof) != {
        "version",
        "target_sha",
        "baseline_sha",
        "database_digest",
        "sources",
    }:
        fail()
    sources = proof["sources"]
    if (
        type(proof["version"]) is not int
        or proof["version"] != 1
        or not isinstance(proof["target_sha"], str)
        or not SHA.fullmatch(proof["target_sha"])
        or not isinstance(proof["baseline_sha"], str)
        or not SHA.fullmatch(proof["baseline_sha"])
        or not isinstance(proof["database_digest"], str)
        or not DIGEST.fullmatch(proof["database_digest"])
        or not isinstance(sources, dict)
        or set(sources) != SOURCES
        or any(not isinstance(value, str) or not DIGEST.fullmatch(value) for value in sources.values())
    ):
        fail()
    return proof


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--baseline-file", required=True)
    parser.add_argument("--github-env", required=True)
    args = parser.parse_args()
    proof = validate(args.baseline_file)
    with Path(args.github_env).open("a") as environment:
        environment.write(
            "RECOVERY_TARGET_SHA=" + proof["target_sha"] + "\n"
            "RECOVERY_BASELINE_SHA=" + proof["baseline_sha"] + "\n"
            "RECOVERY_DATABASE_DIGEST=" + proof["database_digest"] + "\n"
        )
    print("Validated the host-selected unfinished schema-v2 deployment.")


if __name__ == "__main__":
    main()
