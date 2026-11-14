#!/usr/bin/env python3
"""Validate the one reviewed interrupted release; never accept arbitrary run inputs."""

import argparse
import hashlib
import http.client
import importlib.util
import io
import json
import os
import re
import stat
import zipfile
from pathlib import Path
from urllib.parse import urlsplit

REPOSITORY = "asari194617/taskly"
REPOSITORY_ID = 1074628894
RUN = 43592044959
JOB = 132440223224
TARGET = "85222d44a5bf082d36d9dd781889eda73f0ff013"
ARTIFACT = 14113475260
ZIP_SHA256 = "80fecd28390042e391ecac96c4949a632f0b1296fc01e5c7cd13f7f89324d8d8"
NAMES = {"taskly-schema-baseline.json", "taskly-schema-proof.json"}
SUCCESS_STEPS = {
    "Verify SSH connectivity",
    "Verify unchanged schema against the committed production baseline",
    "Check and mark production deployment",
    "Rsync repository to server",
    "Install production environment",
}


def fail():
    raise SystemExit(
        "Reconciliation blocked: interrupted-release provenance is invalid"
    )


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            fail()
        result[key] = value
    return result


def decode(data):
    try:
        return json.loads(data, object_pairs_hook=unique_object)
    except (ValueError, UnicodeError):
        fail()


def validate_metadata(run, job, artifact):
    if (
        run.get("id") != RUN
        or run.get("run_attempt") != 1
        or run.get("status") != "completed"
        or run.get("conclusion") != "failure"
        or run.get("event") != "push"
        or run.get("head_branch") != "main"
        or run.get("head_sha") != TARGET
        or run.get("path") != ".github/workflows/deploy-prod.yml"
        or run.get("repository", {}).get("full_name") != REPOSITORY
        or run.get("repository", {}).get("id") != REPOSITORY_ID
        or run.get("head_repository", {}).get("id") != REPOSITORY_ID
        or job.get("id") != JOB
        or job.get("run_id") != RUN
        or job.get("run_attempt") != 1
        or job.get("name") != "deploy"
        or job.get("status") != "completed"
        or job.get("conclusion") != "failure"
        or job.get("runner_name") != "taskly-prod"
    ):
        fail()
    steps = job.get("steps", [])
    if not isinstance(steps, list) or len(steps) > 50:
        fail()
    for name, expected in [
        *((name, "success") for name in SUCCESS_STEPS),
        ("Remote deploy", "failure"),
        ("Mark production deployment complete", "skipped"),
    ]:
        matches = [step for step in steps if step.get("name") == name]
        if len(matches) != 1 or matches[0].get("conclusion") != expected:
            fail()
    source = artifact.get("workflow_run", {})
    if (
        artifact.get("id") != ARTIFACT
        or artifact.get("name") != "taskly-code-only-schema-" + TARGET
        or artifact.get("expired") is not False
        or type(artifact.get("size_in_bytes")) is not int
        or not 0 < artifact["size_in_bytes"] <= 16384
        or artifact.get("digest") != "sha256:" + ZIP_SHA256
        or source.get("id") != RUN
        or source.get("head_sha") != TARGET
        or source.get("head_branch") != "main"
        or source.get("repository_id") != REPOSITORY_ID
        or source.get("head_repository_id") != REPOSITORY_ID
    ):
        fail()


def unpack(data):
    if not 0 < len(data) <= 16384 or hashlib.sha256(data).hexdigest() != ZIP_SHA256:
        fail()
    try:
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            entries = archive.infolist()
            if len(entries) != 2 or {entry.filename for entry in entries} != NAMES:
                fail()
            result = {}
            for entry in entries:
                if (
                    entry.flag_bits & 1
                    or stat.S_ISLNK(entry.external_attr >> 16)
                    or entry.compress_type
                    not in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED)
                    or not 0 < entry.file_size <= 4096
                    or entry.compress_size <= 0
                    or entry.file_size > entry.compress_size * 20
                ):
                    fail()
                with archive.open(entry) as source:
                    payload = source.read(4097)
                if len(payload) != entry.file_size:
                    fail()
                result[entry.filename] = payload
    except (ValueError, OSError, zipfile.BadZipFile, RuntimeError):
        fail()
    baseline = decode(result["taskly-schema-baseline.json"])
    if baseline != decode(result["taskly-schema-proof.json"]):
        fail()
    return result["taskly-schema-baseline.json"]


API_HOST = "api.github.com"
ARTIFACT_HOST = "productionresultssa10.blob.core.windows.net"
HTTP_TIMEOUT = 15
API_PATHS = {
    "actions/runs/" + str(RUN),
    "actions/jobs/" + str(JOB),
    "actions/artifacts/" + str(ARTIFACT),
    "actions/artifacts/" + str(ARTIFACT) + "/zip",
}


def download_target(location):
    # This exact host was observed for the pinned artifact. Do not accept a
    # wildcard Azure/GitHub domain or let a URL choose an authenticated host.
    if not isinstance(location, str) or not 0 < len(location) <= 16384:
        fail()
    if any(not 33 <= ord(character) <= 126 for character in location):
        fail()
    try:
        url = urlsplit(location)
        if (
            url.scheme != "https"
            or url.netloc != ARTIFACT_HOST
            or url.hostname != ARTIFACT_HOST
            or url.username is not None
            or url.password is not None
            or url.port is not None
            or url.fragment
            or not url.path.startswith("/")
        ):
            fail()
    except ValueError:
        fail()
    return url.path + ("?" + url.query if url.query else "")


def request(host, target, headers, limit, artifact_redirect=False):
    # http.client never follows redirects. New connections for the artifact
    # receive a newly constructed header map containing no workflow token.
    connection = None
    try:
        connection = http.client.HTTPSConnection(host, timeout=HTTP_TIMEOUT)
        connection.request("GET", target, headers=headers)
        response = connection.getresponse()
        if artifact_redirect:
            if response.status != 302:
                fail()
            locations = response.headers.get_all("Location", [])
            if len(locations) != 1:
                fail()
            return download_target(locations[0])
        if response.status != 200:
            fail()
        lengths = response.headers.get_all("Content-Length", [])
        if len(lengths) > 1 or (
            lengths
            and (not re.fullmatch(r"[0-9]+", lengths[0]) or int(lengths[0]) > limit)
        ):
            fail()
        body = response.read(limit + 1)
        if len(body) > limit or (lengths and len(body) != int(lengths[0])):
            fail()
        return body
    except (OSError, http.client.HTTPException, ValueError):
        # Do not print exceptions: they may contain a token, signed URL or
        # private API response. Error bodies are never read or logged.
        fail()
    finally:
        if connection is not None:
            connection.close()


def api(path):
    if path not in API_PATHS:
        fail()
    token = os.environ.get("GH_TOKEN", "")
    if not token or any(not 33 <= ord(character) <= 126 for character in token):
        fail()
    headers = {
        "Authorization": "Bearer " + token,
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "taskly-code-only-reconciliation",
        "Accept-Encoding": "identity",
    }
    target = "/repos/" + REPOSITORY + "/" + path
    if path.endswith("/zip"):
        redirected_target = request(
            API_HOST, target, headers, 16384, artifact_redirect=True
        )
        return request(
            ARTIFACT_HOST,
            redirected_target,
            {"User-Agent": headers["User-Agent"], "Accept-Encoding": "identity"},
            16384,
        )
    return request(API_HOST, target, headers, 1024 * 1024)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--github-env", required=True)
    args = parser.parse_args()
    os.umask(0o077)
    run = decode(api("actions/runs/" + str(RUN)))
    job = decode(api("actions/jobs/" + str(JOB)))
    artifact = decode(api("actions/artifacts/" + str(ARTIFACT)))
    validate_metadata(run, job, artifact)
    baseline = unpack(api("actions/artifacts/" + str(ARTIFACT) + "/zip"))
    directory = Path(args.output_dir)
    directory.mkdir(mode=0o700, exist_ok=False)
    baseline_file = directory / "baseline.json"
    baseline_file.write_bytes(baseline)
    path = Path(__file__).with_name("verify-no-ddl-release.py")
    spec = importlib.util.spec_from_file_location("no_ddl", path)
    guard = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(guard)
    proof = guard.verify(args.repo, TARGET, baseline_file)
    (directory / "verified.json").write_text(json.dumps(proof, sort_keys=True) + "\n")
    with Path(args.github_env).open("a") as environment:
        environment.write(
            "SCHEMA_BASELINE_SHA=" + proof["baseline_sha"] + "\n"
            "SCHEMA_DATABASE_DIGEST=" + proof["database_digest"] + "\n"
            "SCHEMA_TARGET_SHA=" + TARGET + "\n"
        )
    print("Verified the exact failed main deployment and unchanged-schema attestation.")


if __name__ == "__main__":
    main()
