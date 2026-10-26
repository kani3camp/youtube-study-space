#!/usr/bin/env python3
"""Fail-closed admission for the manual MyPage maintenance workflow."""

from __future__ import annotations

from datetime import datetime, timedelta, timezone
import hashlib
import os
from pathlib import Path
import re
import sys


ERROR = "maintenance workflow context rejected"
REPOSITORY = "kani3camp/youtube-study-space"
REPOSITORY_ID = "340900071"
OWNER_ID = "54093651"
WORKFLOW = ".github/workflows/mypage-maintenance.yml"
TARGETS = {
    "development": ("test-youtube-study-space", "asia-southeast2", "refs/heads/integration/mypage-canon-20261006"),
    "production": ("youtube-study-space", "asia-northeast2", "refs/heads/main"),
}
STATES = {"NORMAL", "ACTIVE"}
REASONS = {"INITIALIZATION", "PLANNED_MAINTENANCE", "DATA_MIGRATION", "SECURITY_MAINTENANCE", "RECOVERY"}


def need(condition: bool) -> None:
    if not condition:
        raise ValueError(ERROR)


def context(get=os.environ.get, now=lambda: datetime.now(timezone.utc)) -> dict[str, str]:
    environment = get("INPUT_ENVIRONMENT", "")
    need(environment in TARGETS)
    project, region, expected_ref = TARGETS[environment]
    state = get("INPUT_STATE", "")
    reason = get("INPUT_REASON_CATEGORY", "")
    expected_revision = get("INPUT_EXPECTED_REVISION", "")
    expected_sha = get("INPUT_EXPECTED_SHA", "")
    estimate = get("INPUT_ESTIMATED_END_AT", "")
    confirmation = get("INPUT_PRODUCTION_CONFIRMATION", "")
    need(state in STATES and reason in REASONS)
    need(re.fullmatch(r"0|[1-9][0-9]{0,17}", expected_revision) is not None)
    need(re.fullmatch(r"[0-9a-f]{40}", expected_sha) is not None and expected_sha == get("GITHUB_SHA", ""))
    need(get("GITHUB_REPOSITORY", "") == REPOSITORY)
    need(get("GITHUB_REPOSITORY_ID", "") == REPOSITORY_ID)
    need(get("GITHUB_REPOSITORY_OWNER_ID", "") == OWNER_ID)
    need(get("GITHUB_ACTOR_ID", "") == OWNER_ID and get("MYPAGE_TRIGGERING_ACTOR_ID", "") == OWNER_ID)
    need(get("GITHUB_EVENT_NAME", "") == "workflow_dispatch")
    need(get("GITHUB_RUN_ATTEMPT", "") == "1")
    need(re.fullmatch(r"[1-9][0-9]{0,19}", get("GITHUB_RUN_ID", "")) is not None)
    need(get("GITHUB_REF", "") == expected_ref)
    workflow_ref = f"{REPOSITORY}/{WORKFLOW}@{expected_ref}"
    need(get("GITHUB_WORKFLOW_REF", "") == workflow_ref)
    if expected_revision == "0":
        need(state == "NORMAL" and reason == "INITIALIZATION")
    else:
        need(reason != "INITIALIZATION")
    if state == "NORMAL":
        need(estimate == "")
    elif estimate:
        need(re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:Z|[+-]\d{2}:\d{2})", estimate) is not None)
        parsed = datetime.fromisoformat(estimate.replace("Z", "+00:00"))
        current = now()
        need(parsed.tzinfo is not None and current < parsed.astimezone(timezone.utc) <= current + timedelta(days=14))
    if environment == "production":
        need(confirmation == f"PRODUCTION {state}")
    else:
        need(confirmation == "")
    run_id = get("GITHUB_RUN_ID", "")
    action_ref = hashlib.sha256(f"{REPOSITORY_ID}:{run_id}:1:{expected_sha}:{environment}:{state}".encode()).hexdigest()
    return {
        "project": project,
        "region": region,
        "environment": environment,
        "state": state,
        "reason": reason,
        "expected_revision": expected_revision,
        "estimated_end_at": estimate,
        "expected_sha": expected_sha,
        "action_ref": action_ref,
        "workflow_ref": workflow_ref,
    }


def main() -> int:
    try:
        values = context()
        output = Path(os.environ["GITHUB_OUTPUT"])
        with output.open("a", encoding="utf-8") as stream:
            for key, value in values.items():
                stream.write(f"{key}={value}\n")
        print("maintenance workflow context accepted")
        return 0
    except (KeyError, OSError, ValueError):
        print(f"::error::{ERROR}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
