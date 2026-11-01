#!/usr/bin/env python3
"""Credentialless metadata contracts for the retained backup bucket."""
import json
import shutil
import subprocess
import tempfile
from pathlib import Path

from terraform_plan_summary import build_summary

ROOT = Path(__file__).resolve().parents[2]


def main():
    module = ROOT / "infra/gcp/modules/retained-backup-bucket"
    with tempfile.TemporaryDirectory(prefix="backup-bucket-contract-") as directory:
        target = Path(directory) / "module"
        shutil.copytree(module, target, ignore=shutil.ignore_patterns(".terraform", ".terraform.lock.hcl"))
        shutil.copyfile(ROOT / "infra/gcp/environments/dev/.terraform.lock.hcl", target / ".terraform.lock.hcl")
        for args in [("init", "-backend=false", "-input=false", "-lockfile=readonly"), ("test", "-json", "-verbose")]:
            result = subprocess.run(["terraform", f"-chdir={target}", *args], capture_output=True, text=True)
            if result.returncode:
                raise SystemExit("Backup bucket contracts failed; raw plan suppressed.")
        plans = [event["test_plan"] for line in result.stdout.splitlines() if (event := json.loads(line)).get("type") == "test_plan"]
        assert len(plans) == 2
        for plan in plans:
            summary = build_summary(plan, environment="dev", git_sha="0" * 40, policy="import-only")
            assert summary["counts"]["create"] == 1 and not summary["policy_passed"]
            assert len(summary["resources"]) == 1 and summary["resources"][0]["address"] == "google_storage_bucket.retained"
        # Mechanical guard: destroying retained data must never become an option.
        assert "prevent_destroy = true" in (module / "main.tf").read_text()
    print("Backup bucket metadata dev/prod contracts PASS; only bucket ownership; unchanged import-only guard rejects create.")


if __name__ == "__main__":
    main()
