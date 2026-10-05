#!/usr/bin/env python3
"""Run credentialless mock plans and prove quota creation stays import-guarded."""
from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
from pathlib import Path

from terraform_plan_summary import build_summary
from terraform_quota_create_gate import TYPES, expected, stable

ROOT = Path(__file__).resolve().parents[2]
MODULE = ROOT / "infra/gcp/modules/youtube-quota-alerts"


def main() -> None:
    with tempfile.TemporaryDirectory(prefix="youtube-quota-contract-") as directory:
        target = Path(directory) / "module"
        shutil.copytree(MODULE, target, ignore=shutil.ignore_patterns(".terraform", ".terraform.lock.hcl"))
        shutil.copyfile(ROOT / "infra/gcp/environments/dev/.terraform.lock.hcl", target / ".terraform.lock.hcl")
        init = subprocess.run(
            ["terraform", f"-chdir={target}", "init", "-backend=false", "-lockfile=readonly", "-input=false"],
            capture_output=True, text=True, check=False,
        )
        if init.returncode:
            raise SystemExit("Quota mock provider init failed; raw output suppressed.")
        result = subprocess.run(
            ["terraform", f"-chdir={target}", "test", "-json", "-verbose"],
            capture_output=True, text=True, check=False,
        )
        if result.returncode:
            raise SystemExit("Quota mock contract failed; raw plan output suppressed.")
        plans = {
            event["@testrun"]: event["test_plan"]
            for line in result.stdout.splitlines()
            if (event := json.loads(line)).get("type") == "test_plan"
        }
        inert = build_summary(plans["migration_is_inert"], environment="dev", git_sha="0" * 40, policy="import-only")
        assert inert["policy_passed"] and all(count == 0 for count in inert["counts"].values())
        for run in ("three_types_preserve_quota_semantics", "production_values_are_parameters"):
            summary = build_summary(plans[run], environment="dev", git_sha="0" * 40, policy="import-only")
            assert summary["counts"]["create"] == 3 and not summary["policy_passed"]
            assert len(summary["violations"]) == 3
        # Independently generated provider mock values must match the fixed gate
        # after substituting only the intentionally dummy project/channel.
        for resource in plans["three_types_preserve_quota_semantics"]["resource_changes"]:
            key = next(key for key in TYPES if resource["address"].endswith(f'["{key}"]'))
            value = json.loads(json.dumps(resource["change"]["after"]).replace("quota-test-dev", "test-youtube-study-space"))
            channel = "projects/test-youtube-study-space/notificationChannels/verified-fixture"
            assert stable(value) == expected(key, channel)
    print("Quota mock contracts PASS; migration resource actions=0; opt-in create=3 rejected by unchanged import-only guard.")


if __name__ == "__main__":
    main()
