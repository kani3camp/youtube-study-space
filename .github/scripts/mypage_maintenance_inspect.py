#!/usr/bin/env python3
"""Fail-closed context and sanitized receipt parser for maintenance inspection."""
from __future__ import annotations

import os
from pathlib import Path
import re
import sys
from urllib.parse import urlsplit

from mypage_maintenance import (
    OWNER_ID,
    REPOSITORY,
    REPOSITORY_ID,
    REASONS,
    STATES,
    TARGETS,
    canonical_rfc3339,
    need,
)


ERROR = "maintenance inspect context rejected"
WORKFLOW = ".github/workflows/mypage-maintenance-inspect.yml"


def context(get=os.environ.get) -> dict[str, str]:
    environment = get("INPUT_ENVIRONMENT", "")
    need(environment in TARGETS)
    project, project_number, region, expected_ref, _public_origin = TARGETS[environment]
    expected_sha = get("INPUT_EXPECTED_SHA", "")
    confirmation = get("INPUT_PRODUCTION_CONFIRMATION", "")
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
    need(confirmation == ("PRODUCTION INSPECT" if environment == "production" else ""))
    service_url = get("INPUT_SERVICE_URL", "")
    parsed = urlsplit(service_url)
    deterministic = f"mypage-maintenance-inspect-{project_number}.{region}.run.app"
    legacy = re.fullmatch(r"mypage-maintenance-inspect-[a-z0-9-]{1,63}\.a\.run\.app", parsed.hostname or "")
    need(parsed.scheme == "https" and parsed.username is None and parsed.password is None and parsed.port is None)
    need((parsed.hostname == deterministic or legacy is not None) and parsed.path in ("", "/") and not parsed.query and not parsed.fragment)
    return {
        "environment": environment,
        "project": project,
        "provider": f"projects/{project_number}/locations/global/workloadIdentityPools/mypage-maintenance/providers/github",
        "service_account": f"mypage-maint-inspect-invoker@{project}.iam.gserviceaccount.com",
        "service_url": service_url.rstrip("/"),
    }


def parse_receipt(candidate: object, environment: str) -> dict[str, object]:
    need(environment in TARGETS and isinstance(candidate, dict))
    required = {"environment", "state", "revision", "updatedAt", "messageTemplate", "reasonCategory"}
    proof = {"authoritativeReadbackVerified", "readOnly"}
    need(set(candidate) in (required | proof, required | proof | {"estimatedEndAt"}))
    need(candidate.get("environment") == environment)
    state = candidate.get("state")
    need(state in STATES)
    revision = candidate.get("revision")
    need(type(revision) is int and 1 <= revision <= 9_223_372_036_854_775_807)
    updated_at = canonical_rfc3339(candidate.get("updatedAt"))
    need(updated_at is not None)
    need(candidate.get("messageTemplate") == "MYPAGE_MAINTENANCE_V1")
    reason = candidate.get("reasonCategory")
    need(reason in REASONS)
    need(candidate.get("authoritativeReadbackVerified") is True and candidate.get("readOnly") is True)
    estimate = None
    if "estimatedEndAt" in candidate:
        estimate = canonical_rfc3339(candidate["estimatedEndAt"])
        need(estimate is not None and state == "ACTIVE")
    need(state != "NORMAL" or estimate is None)
    receipt: dict[str, object] = {
        "environment": environment,
        "state": state,
        "revision": revision,
        "updatedAt": updated_at,
        "messageTemplate": "MYPAGE_MAINTENANCE_V1",
        "reasonCategory": reason,
        "authoritativeReadbackVerified": True,
        "readOnly": True,
    }
    if estimate is not None:
        receipt["estimatedEndAt"] = estimate
    return receipt


def main() -> int:
    try:
        values = context()
        output = Path(os.environ["GITHUB_OUTPUT"])
        with output.open("a", encoding="utf-8") as stream:
            for key, value in values.items():
                stream.write(f"{key}={value}\n")
        print("maintenance inspect context accepted")
        return 0
    except (KeyError, OSError, ValueError):
        print(f"::error::{ERROR}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
