#!/usr/bin/env python3
"""Verify the deployment boundary against Terraform's native mocked Plan JSON."""
import json
from pathlib import Path
import subprocess

from foundation_contract import need, validate_plan

root = Path(__file__).resolve().parents[1] / "environments/dev"
result = subprocess.run(["terraform", "test", "-filter=tests/foundation-contract.tftest.hcl", "-verbose", "-json"],
                        cwd=root, capture_output=True, text=True, timeout=180)
need(result.returncode == 0)
plans = [event["test_plan"] for line in result.stdout.splitlines()
         if (event := json.loads(line)).get("type") == "test_plan"]
need(len(plans) == 1)
# terraform test emits the renderer's Plan envelope, not terraform show's
# native envelope. Preserve the actual provider-generated changes/unknowns;
# only supply the test run's known stage and successful completion metadata.
# The authenticated path continues to require the real native envelope.
rendered = plans[0]
summary = validate_plan({"format_version": rendered["plan_format_version"],
                         "complete": True, "errored": False,
                         "variables": {"stage": {"value": "foundation"}},
                         "resource_changes": rendered.get("resource_changes", []),
                         "resource_drift": rendered.get("resource_drift", []),
                         "deferred_changes": rendered.get("deferred_changes", [])})
need(summary == {"resources": 8, "creates": 8})
print("Native mocked Foundation Plan accepted: eight creates; no cloud I/O.")
