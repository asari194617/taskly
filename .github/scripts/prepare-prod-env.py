#!/usr/bin/env python3
"""Add private task-image secrets to the existing deployment environment."""

import os
import re
import secrets
from pathlib import Path


STORAGE_KEYS = ("R2_ENDPOINT", "R2_BUCKET", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY")
STORAGE_ASSIGNMENT = re.compile(
    r"^\s*(?:export\s+)?(?:" + "|".join(STORAGE_KEYS) + r")\s*="
)


def main():
    values = {key: os.environ.get("TASK_IMAGES_" + key, "").strip() for key in STORAGE_KEYS}
    if not any(values.values()):
        return  # Existing PROD_ENV can already include all storage settings.
    for key, value in values.items():
        if not value or "\n" in value or "\r" in value:
            raise SystemExit("TASK_IMAGES_" + key + " must be a non-empty single-line secret")
    original = os.environ.get("PROD_ENV", "")
    if not original.strip():
        raise SystemExit("PROD_ENV must contain the production .env before deployment.")
    lines = [line for line in original.splitlines() if not STORAGE_ASSIGNMENT.match(line)]
    merged = "\n".join(lines).rstrip() + "\n" + "".join(
        key + "=" + value + "\n" for key, value in values.items()
    )
    delimiter = "TASKLY_PROD_ENV_" + secrets.token_hex(16)
    with Path(os.environ["GITHUB_ENV"]).open("a") as output:
        output.write("PROD_ENV<<" + delimiter + "\n" + merged + delimiter + "\n")


if __name__ == "__main__":
    main()
