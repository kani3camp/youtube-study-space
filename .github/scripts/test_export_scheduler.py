#!/usr/bin/env python3
"""Credentialless contracts for preserved delivery and disabled adoption."""
import json
import shutil
import subprocess
import tempfile
from pathlib import Path

from terraform_plan_summary import build_summary

ROOT = Path(__file__).resolve().parents[2]
SCHEDULER = "module.export_scheduler[0].google_cloud_scheduler_job.export"


def plans(target):
    for args in [("init", "-backend=false", "-input=false", "-lockfile=readonly"), ("test", "-json", "-verbose")]:
        result = subprocess.run(["terraform", f"-chdir={target}", *args], capture_output=True, text=True)
        if result.returncode:
            raise SystemExit("Scheduler contracts failed; raw plan suppressed.")
    return [event["test_plan"] for line in result.stdout.splitlines()
            if (event := json.loads(line)).get("type") == "test_plan"]


def copy_module(name, target):
    shutil.copytree(ROOT / "infra/gcp/modules" / name, target,
                    ignore=shutil.ignore_patterns(".terraform", ".terraform.lock.hcl"))


def main():
    with tempfile.TemporaryDirectory(prefix="export-scheduler-contract-") as directory:
        base = Path(directory)
        target = base / "environments/dev"
        target.mkdir(parents=True)
        for name in ["firestore-export-topic", "firestore-export-scheduler"]:
            copy_module(name, base / "modules" / name)
        lock = ROOT / "infra/gcp/environments/dev/.terraform.lock.hcl"
        module = base / "modules/firestore-export-scheduler"
        shutil.copyfile(lock, module / ".terraform.lock.hcl")
        module_plans = plans(module)
        assert len(module_plans) == 1
        summary = build_summary(module_plans[0], environment="dev", git_sha="0" * 40, policy="import-only")
        assert summary["counts"]["create"] == 1 and not summary["policy_passed"]
        for name in ["export-topic.tf", "export-scheduler.tf"]:
            shutil.copyfile(ROOT / "infra/gcp/environments/dev" / name, target / name)
        shutil.copyfile(lock, target / ".terraform.lock.hcl")
        (target / "main.tf").write_text('terraform {\n required_providers {\n google = { source = "hashicorp/google", version = "= 8.5.0" }\n }\n}\nprovider "google" {}\nvariable "project_id" {\n type = string\n default = "test-youtube-study-space"\n}\n')
        (target / "tests").mkdir()
        (target / "tests/boundary.tftest.hcl").write_text('''mock_provider "google" {}
override_resource {
  target = module.export_topic[0].google_pubsub_topic.export
  values = { id = "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport" }
}
override_resource {
  target = module.export_scheduler[0].google_cloud_scheduler_job.export
  values = { id = "projects/test-youtube-study-space/locations/asia-southeast2/jobs/scheduledFirestoreCollectionsExport" }
}
run "default_has_no_export_ownership" {
  command = plan
  assert {
    condition = length(module.export_scheduler) == 0 && length(module.export_topic) == 0
    error_message = "Default root must not adopt the export chain."
  }
}
run "adopted_topic_does_not_activate_scheduler" {
  command = plan
  variables { manage_export_topic = true }
  assert {
    condition = length(module.export_scheduler) == 0 && length(module.export_topic) == 1
    error_message = "Existing topic activation must leave Scheduler disabled."
  }
}
run "scheduler_uses_adopted_topic_dependency" {
  command = plan
  variables {
    manage_export_topic = true
    manage_export_scheduler = true
  }
  override_resource {
    target = module.export_topic[0].google_pubsub_topic.export
    values = { id = "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport" }
  }
  assert {
    condition = length(module.export_scheduler) == 1 && module.export_topic[0].topic_id == "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"
    error_message = "Scheduler must reference the adopted topic output."
  }
}
run "reject_scheduler_without_topic" {
  command = plan
  variables { manage_export_scheduler = true }
  expect_failures = [var.manage_export_scheduler]
}
run "reject_production_scheduler" {
  command = plan
  variables {
    project_id = "youtube-study-space"
    manage_export_scheduler = true
  }
  expect_failures = [var.manage_export_scheduler]
}
''')
        root_plans = plans(target)
        assert len(root_plans) == 5 and not root_plans[0].get("resource_changes")
        assert {r["address"] for r in root_plans[1]["resource_changes"]} == {"module.export_topic[0].google_pubsub_topic.export"}
        assert {r["address"] for r in root_plans[2]["resource_changes"]} == {"module.export_topic[0].google_pubsub_topic.export", SCHEDULER}
        scheduler = next(r for r in root_plans[2]["resource_changes"] if r["address"] == SCHEDULER)
        assert scheduler["change"]["after"]["pubsub_target"][0]["topic_name"] == "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"
        imported = build_summary(root_plans[2], environment="dev", git_sha="0" * 40, policy="import-only")
        assert imported["policy_passed"] and imported["counts"]["import"] == 2 and imported["counts"]["create"] == 0
    print("Scheduler preserved-delivery/default-off/dependency/dev-only contracts PASS; global guard rejects create.")


if __name__ == "__main__":
    main()
