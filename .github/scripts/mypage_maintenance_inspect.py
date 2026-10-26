#!/usr/bin/env python3
"""Fail-closed context and Firestore REST parser for read-only maintenance inspection."""
from __future__ import annotations

import os
from pathlib import Path
import re
import sys

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
    project, project_number, _region, expected_ref, _public_origin = TARGETS[environment]
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
    return {
        "environment": environment,
        "project": project,
        "provider": f"projects/{project_number}/locations/global/workloadIdentityPools/mypage-maintenance/providers/github",
        "service_account": f"mypage-maintenance-bootstrap@{project}.iam.gserviceaccount.com",
        "document_url": f"https://firestore.googleapis.com/v1/projects/{project}/databases/(default)/documents/mypage-runtime-control/current",
    }


def _scalar(fields: dict[str, object], name: str, kind: str) -> str:
    value = fields.get(name)
    need(isinstance(value, dict) and set(value) == {kind})
    scalar = value[kind]
    need(isinstance(scalar, str))
    return scalar


def parse_document(candidate: object, environment: str, project: str) -> dict[str, object]:
    need(environment in TARGETS and TARGETS[environment][0] == project)
    need(isinstance(candidate, dict) and set(candidate) == {"name", "fields", "createTime", "updateTime"})
    need(candidate.get("name") == f"projects/{project}/databases/(default)/documents/mypage-runtime-control/current")
    need(canonical_rfc3339(candidate.get("createTime")) is not None)
    need(canonical_rfc3339(candidate.get("updateTime")) is not None)
    fields = candidate.get("fields")
    need(isinstance(fields, dict))
    required = {"environment", "state", "revision", "updatedAt", "messageTemplate", "reasonCategory"}
    need(required <= set(fields) <= required | {"estimatedEndAt"})
    need(_scalar(fields, "environment", "stringValue") == environment)
    state = _scalar(fields, "state", "stringValue")
    need(state in STATES)
    revision_text = _scalar(fields, "revision", "integerValue")
    need(re.fullmatch(r"[1-9][0-9]{0,17}", revision_text) is not None)
    revision = int(revision_text)
    need(revision <= 9_223_372_036_854_775_807)
    updated_at = canonical_rfc3339(_scalar(fields, "updatedAt", "timestampValue"))
    need(updated_at is not None)
    need(_scalar(fields, "messageTemplate", "stringValue") == "MYPAGE_MAINTENANCE_V1")
    reason = _scalar(fields, "reasonCategory", "stringValue")
    need(reason in REASONS)
    estimate = None
    if "estimatedEndAt" in fields:
        estimate = canonical_rfc3339(_scalar(fields, "estimatedEndAt", "timestampValue"))
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
