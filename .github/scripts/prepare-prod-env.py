#!/usr/bin/env python3
"""Merge dedicated settings into the existing, complete deployment environment."""

import json
import os
import re
import secrets
from pathlib import Path
from urllib.parse import urlsplit


STORAGE_KEYS = ("R2_ENDPOINT", "R2_BUCKET", "R2_ACCESS_KEY_ID", "R2_SECRET_ACCESS_KEY")
GUIDANCE_KEYS = ("NNCT_GUIDANCE_EVENT_ID", "NNCT_GUIDANCE_TASK_BINDINGS")
GUIDANCE_PREFIX = "DEPLOY_"
MCP_PARENT_KEY = "NNCT_GUIDANCE_MCP_PARENT_RESOURCE"
TASK_KEY = re.compile(r"^(?:assigned-visit|visit-survey)-[1-9][0-9]?$")
EVENT_ID = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$")


def dedicated_values(keys, prefix):
    values = {}
    for key in keys:
        raw = os.environ.get(prefix + key, "")
        if "\n" in raw or "\r" in raw:
            raise SystemExit(prefix + key + " must be a single-line secret")
        values[key] = raw.strip()
    if not any(values.values()):
        return {}
    for key, value in values.items():
        if not value:
            raise SystemExit(prefix + key + " must be a non-empty single-line secret")
    return values


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate object key")
        result[key] = value
    return result


def guidance_values():
    values = dedicated_values(GUIDANCE_KEYS, GUIDANCE_PREFIX)
    if not values:
        return {}
    if not EVENT_ID.fullmatch(values["NNCT_GUIDANCE_EVENT_ID"]):
        raise SystemExit("DEPLOY_NNCT_GUIDANCE_EVENT_ID must be a valid event identifier")
    raw = values["NNCT_GUIDANCE_TASK_BINDINGS"]
    invalid = "DEPLOY_NNCT_GUIDANCE_TASK_BINDINGS must contain valid task bindings"
    if len(raw) > 16384:
        raise SystemExit(invalid)
    try:
        bindings = json.loads(raw, object_pairs_hook=unique_object)
    except (ValueError, TypeError):
        raise SystemExit(invalid) from None
    if not isinstance(bindings, dict) or not 4 <= len(bindings) <= 64:
        raise SystemExit(invalid)
    if "overall-survey" not in bindings or "sheet-submission" not in bindings:
        raise SystemExit(invalid)
    ids = set()
    for key, task_id in bindings.items():
        if key not in ("overall-survey", "sheet-submission") and not TASK_KEY.fullmatch(key):
            raise SystemExit(invalid)
        if type(task_id) is not int or not 0 < task_id <= 2**64 - 1 or task_id in ids:
            raise SystemExit(invalid)
        ids.add(task_id)
    if not any(key.startswith("assigned-visit-") for key in bindings) or not any(
        key.startswith("visit-survey-") for key in bindings
    ):
        raise SystemExit(invalid)
    # Only whitelisted keys and integer values survive serialization. Literal
    # dotenv quoting keeps the JSON intact when Compose reads both env files.
    values["NNCT_GUIDANCE_TASK_BINDINGS"] = "'" + json.dumps(
        bindings, sort_keys=True, separators=(",", ":"), ensure_ascii=True
    ) + "'"
    return values


def mcp_parent_values():
    name = GUIDANCE_PREFIX + MCP_PARENT_KEY
    raw = os.environ.get(name, "")
    if raw == "":
        return {}
    invalid = name + " must be a canonical HTTPS resource ending in /mcp"
    if len(raw) > 2048 or not raw.isascii() or any(ord(char) <= 32 or ord(char) == 127 for char in raw):
        raise SystemExit(invalid)
    try:
        resource = urlsplit(raw)
        host = resource.hostname or ""
        port = resource.port
    except ValueError:
        raise SystemExit(invalid) from None
    labels = host.split(".")
    valid_host = len(host) <= 253 and all(
        1 <= len(label) <= 63
        and label[0].isalnum()
        and label[-1].isalnum()
        and all(char in "abcdefghijklmnopqrstuvwxyz0123456789-" for char in label)
        for label in labels
    )
    authority = host if port is None else f"{host}:{port}"
    if (
        resource.scheme != "https"
        or not valid_host
        or resource.username is not None
        or resource.password is not None
        or resource.netloc != authority
        or (port is not None and port == 0)
        or resource.path != "/mcp"
        or resource.query
        or resource.fragment
        or resource.geturl() != raw
    ):
        raise SystemExit(invalid)
    return {MCP_PARENT_KEY: raw}


def main():
    values = dedicated_values(STORAGE_KEYS, "TASK_IMAGES_")
    values.update(guidance_values())
    values.update(mcp_parent_values())
    if not values:
        return  # Existing PROD_ENV can already contain these settings.
    original = os.environ.get("PROD_ENV", "")
    if not original.strip():
        raise SystemExit("PROD_ENV must contain the production .env before deployment.")
    assignment = re.compile(
        r"^\s*(?:export\s+)?(?:" + "|".join(re.escape(key) for key in values) + r")\s*="
    )
    merged = "".join(
        line for line in original.splitlines(keepends=True) if not assignment.match(line)
    )
    if merged and not merged.endswith(("\n", "\r")):
        merged += "\n"
    merged += "".join(
        key + "=" + value + "\n" for key, value in values.items()
    )
    delimiter = "TASKLY_PROD_ENV_" + secrets.token_hex(16)
    with Path(os.environ["GITHUB_ENV"]).open("a") as output:
        output.write("PROD_ENV<<" + delimiter + "\n" + merged + delimiter + "\n")


if __name__ == "__main__":
    main()
