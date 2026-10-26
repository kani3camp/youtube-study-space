#!/usr/bin/env python3
"""Verify the deployment boundary against Terraform's native mocked Plan JSON."""
import json
from pathlib import Path
import subprocess

from foundation_contract import ContractError, need, validate_plan
import runtime_contract

root = Path(__file__).resolve().parents[1] / "environments/dev"
result = subprocess.run(["terraform", "test", "-filter=tests/foundation-contract.tftest.hcl", "-verbose", "-json"],
                        cwd=root, capture_output=True, text=True, timeout=180)
need(result.returncode == 0)
plans = [event["test_plan"] for line in result.stdout.splitlines()
         if (event := json.loads(line)).get("type") == "test_plan"]
need(len(plans) == 6)
# terraform test emits the renderer's Plan envelope, not terraform show's
# native envelope. Preserve the actual provider-generated changes/unknowns;
# only supply the test run's known stage and successful completion metadata.
# The authenticated path continues to require the real native envelope.
for rendered, stage, resources in zip(plans, ("foundation_metadata", "foundation", "registry", "maintenance_operator", "runtime_private", "runtime_public"), (4, 8, 9, 27, 29, 30)):
    variables = {"stage": {"value": stage}}
    if stage in set(runtime_contract.MODES.values()):
        variables["runtime_settings"] = {"value": json.loads((root / "tests/runtime-settings.fixture.json").read_text())}
    envelope = {"format_version": rendered["plan_format_version"],
                         "complete": True, "errored": False,
                         "variables": variables,
                         "resource_changes": rendered.get("resource_changes", []),
                         "resource_drift": rendered.get("resource_drift", []),
                         "deferred_changes": rendered.get("deferred_changes", [])}
    validator = runtime_contract.validate_plan if stage in set(runtime_contract.MODES.values()) else validate_plan
    if stage in {"runtime_private", "runtime_public"}:
        # Every run in this credentialless native test starts without State, so
        # provider-generated runtime Plans try to create the maintenance
        # boundary again. Prove that skipping the operator/bootstrap stage is
        # rejected; exact existing-resource no-op Plans are covered separately.
        try:
            validator(envelope, stage)
        except ContractError as error:
            need(str(error) == "runtime-maintenance-bootstrap-complete")
        else:
            need(False)
    else:
        summary = validator(envelope, stage)
        need(summary == {"resources": resources, "creates": resources})
print("Native mocked maintenance operator Plan accepted; direct runtime creation without completed bootstrap rejected; no cloud I/O.")
