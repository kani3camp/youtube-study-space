#!/usr/bin/env python3
"""Credentialless export-topic ownership boundary and import-only contracts."""
import json
import shutil
import subprocess
import tempfile
from pathlib import Path

from terraform_plan_summary import build_summary

ROOT = Path(__file__).resolve().parents[2]


def main():
    module = ROOT / "infra/gcp/modules/firestore-export-topic"
    with tempfile.TemporaryDirectory(prefix="export-topic-contract-") as directory:
        target = Path(directory) / "module"
        shutil.copytree(module, target, ignore=shutil.ignore_patterns(".terraform", ".terraform.lock.hcl"))
        shutil.copyfile(ROOT / "infra/gcp/environments/dev/.terraform.lock.hcl", target / ".terraform.lock.hcl")
        for args in [("init", "-backend=false", "-input=false", "-lockfile=readonly"), ("test", "-json", "-verbose")]:
            result = subprocess.run(["terraform", f"-chdir={target}", *args], capture_output=True, text=True)
            if result.returncode:
                raise SystemExit("Export topic contracts failed; raw plan suppressed.")
        plans = [event["test_plan"] for line in result.stdout.splitlines() if (event := json.loads(line)).get("type") == "test_plan"]
        assert len(plans) == 2
        for plan in plans:
            summary = build_summary(plan, environment="dev", git_sha="0" * 40, policy="import-only")
            assert summary["counts"]["create"] == 1 and not summary["policy_passed"]
            assert len(summary["resources"]) == 1
            assert summary["resources"][0]["address"] == "google_pubsub_topic.export"
    # Test the real disabled dev import boundary with a minimal credentialless root.
    with tempfile.TemporaryDirectory(prefix="export-topic-root-") as directory:
        base = Path(directory)
        target = base / "environments/dev"
        target.mkdir(parents=True)
        shutil.copytree(module, base / "modules/firestore-export-topic", ignore=shutil.ignore_patterns(".terraform", ".terraform.lock.hcl"))
        shutil.copyfile(ROOT / "infra/gcp/environments/dev/export-topic.tf", target / "export-topic.tf")
        shutil.copyfile(ROOT / "infra/gcp/environments/dev/.terraform.lock.hcl", target / ".terraform.lock.hcl")
        (target / "main.tf").write_text('terraform {\n required_providers {\n google = { source = "hashicorp/google", version = "= 8.5.0" }\n }\n}\nprovider "google" {}\nvariable "project_id" {\n type = string\n default = "test-youtube-study-space"\n}\n')
        (target / "tests").mkdir()
        (target / "tests/boundary.tftest.hcl").write_text('''mock_provider "google" {}
run "disabled_has_no_export_ownership" {
  command = plan
  assert {
    condition = length(module.export_topic) == 0
    error_message = "Default configuration must not adopt the export chain."
  }
}
run "enabled_adopts_only_existing_development_topic" {
  command = plan
  override_resource {
    target = module.export_topic[0].google_pubsub_topic.export
    values = {
      id = "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"
      project = "test-youtube-study-space"
      name = "initiateFirestoreCollectionsExport"
      labels = {}
    }
  }
  variables {
    manage_export_topic = true
  }
  assert {
    condition = length(module.export_topic) == 1
    error_message = "The first wave must select exactly one topic."
  }
}
run "reject_cross_project_adoption" {
  command = plan
  variables {
    project_id = "youtube-study-space"
    manage_export_topic = true
  }
  expect_failures = [var.manage_export_topic]
}
''')
        for args in [("init", "-backend=false", "-input=false", "-lockfile=readonly"), ("test", "-json", "-verbose")]:
            result = subprocess.run(["terraform", f"-chdir={target}", *args], capture_output=True, text=True)
            if result.returncode:
                raise SystemExit("Export topic root boundary failed; raw plan suppressed.")
        plans = [event["test_plan"] for line in result.stdout.splitlines() if (event := json.loads(line)).get("type") == "test_plan"]
        assert len(plans) == 3
        assert not plans[0].get("resource_changes")
        resources = plans[1]["resource_changes"]
        assert len(resources) == 1 and resources[0]["address"] == "module.export_topic[0].google_pubsub_topic.export"
    assert "prevent_destroy = true" in (module / "main.tf").read_text()
    print("Export topic dev/prod metadata contracts PASS; generated/IAM ownership0; global guard rejects create.")


if __name__ == "__main__":
    main()
