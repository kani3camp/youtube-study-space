#!/usr/bin/env python3
"""Verify the deployment boundary against Terraform's native mocked Plan JSON."""
import json
from pathlib import Path
import subprocess

from foundation_contract import need, validate_plan
import runtime_contract

root = Path(__file__).resolve().parents[1] / "environments/dev"
result = subprocess.run(["terraform", "test", "-filter=tests/foundation-contract.tftest.hcl", "-verbose", "-json"],
                        cwd=root, capture_output=True, text=True, timeout=180)
need(result.returncode == 0)
plans = [event["test_plan"] for line in result.stdout.splitlines()
         if (event := json.loads(line)).get("type") == "test_plan"]
need(len(plans) == 5)
# terraform test emits the renderer's Plan envelope, not terraform show's
# native envelope. Preserve the actual provider-generated changes/unknowns;
# only supply the test run's known stage and successful completion metadata.
# The authenticated path continues to require the real native envelope.
for rendered, stage, creates in zip(plans, ("foundation_metadata", "foundation", "registry", "runtime_private", "runtime_public"), (4, 8, 9, 11, 12)):
    variables = {"stage": {"value": stage}}
    if stage.startswith("runtime_"):
        variables["runtime_settings"] = {"value": json.loads((root / "tests/runtime-settings.fixture.json").read_text())}
    envelope = {"format_version": rendered["plan_format_version"],
                         "complete": True, "errored": False,
                         "variables": variables,
                         "resource_changes": rendered.get("resource_changes", []),
                         "resource_drift": rendered.get("resource_drift", []),
                         "deferred_changes": rendered.get("deferred_changes", [])}
    validator = runtime_contract.validate_plan if stage.startswith("runtime_") else validate_plan
    summary = validator(envelope, stage)
    need(summary == {"resources": creates, "creates": creates})
print("Native mocked Plans accepted: four / eight / nine / private eleven / public twelve; no cloud I/O.")
