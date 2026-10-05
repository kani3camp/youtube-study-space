#!/usr/bin/env python3
"""Run credentialless mock plans and prove quota creation stays import-guarded."""
from __future__ import annotations

import json
import shutil
import subprocess
import tempfile
from pathlib import Path

from terraform_plan_summary import build_summary

ROOT = Path(__file__).resolve().parents[2]
MODULE = ROOT / "infra/gcp/modules/monitoring-notification-channels"


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
        inert = build_summary(plans["migration_defaults_are_inert"], environment="dev", git_sha="0" * 40, policy="import-only")
        assert inert["policy_passed"] and all(count == 0 for count in inert["counts"].values())
        for run in ("development_owns_only_private_enabled_email", "production_uses_same_logical_structure"):
            plan = plans[run]
            summary = build_summary(plan, environment="dev", git_sha="0" * 40, policy="import-only")
            assert summary["counts"]["create"] == 1 and not summary["policy_passed"]
            assert len(summary["resources"]) == 1
            assert "example.invalid" not in json.dumps(summary)
            assert plan["output_changes"]["primary_email_name"]["after_sensitive"]
            changes = plan["resource_changes"][0]["change"]
            assert changes["after_sensitive"]["labels"]["email_address"]
        assert "prevent_destroy = true" in (MODULE / "main.tf").read_text()
    print("Email dev/prod/inert/invalid-input contracts PASS; sensitive labels/output; value-free summary; unchanged import-only guard rejects create1.")



if __name__ == "__main__":
    main()
