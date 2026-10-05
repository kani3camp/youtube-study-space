#!/usr/bin/env python3
"""Credentialless Gen1 preservation, default-off and adoption boundaries."""
import json
import shutil
import subprocess
import tempfile
from pathlib import Path

from test_export_scheduler import ROOT, copy_module
from terraform_plan_summary import build_summary

FUNCTION = "module.export_function[0].google_cloudfunctions_function.export"
TOPIC = "module.export_topic[0].google_pubsub_topic.export"
SCHEDULER = "module.export_scheduler[0].google_cloud_scheduler_job.export"


def plans(target):
    for args in [("init", "-backend=false", "-input=false", "-lockfile=readonly"), ("test", "-json", "-verbose")]:
        result = subprocess.run(["terraform", f"-chdir={target}", *args], capture_output=True, text=True)
        if result.returncode:
            diagnostics = [event["diagnostic"] for line in result.stdout.splitlines()
                           if line.startswith("{") and (event := json.loads(line)).get("type") == "diagnostic"]
            raise SystemExit("Function mock contracts failed; raw plan suppressed. " +
                             json.dumps([{k: d[k] for k in ["summary", "detail"]} for d in diagnostics]))
    return [event["test_plan"] for line in result.stdout.splitlines()
            if (event := json.loads(line)).get("type") == "test_plan"]


def main():
    with tempfile.TemporaryDirectory(prefix="export-function-contract-") as directory:
        base = Path(directory)
        target = base / "environments/dev"
        target.mkdir(parents=True)
        for name in ["firestore-export-topic", "firestore-export-scheduler", "firestore-export-function"]:
            copy_module(name, base / "modules" / name)
        lock = ROOT / "infra/gcp/environments/dev/.terraform.lock.hcl"
        module = base / "modules/firestore-export-function"
        shutil.copyfile(lock, module / ".terraform.lock.hcl")
        module_plans = plans(module)
        assert len(module_plans) == 1
        created = build_summary(module_plans[0], environment="dev", git_sha="0" * 40, policy="import-only")
        assert created["counts"]["create"] == 1 and not created["policy_passed"]
        for name in ["export-topic.tf", "export-scheduler.tf", "export-function.tf"]:
            shutil.copyfile(ROOT / "infra/gcp/environments/dev" / name, target / name)
        shutil.copyfile(lock, target / ".terraform.lock.hcl")
        (target / "main.tf").write_text('''terraform {
  required_providers {
    google = { source = "hashicorp/google", version = "= 8.5.0" }
  }
}
provider "google" {}
variable "project_id" {
  type = string
  default = "test-youtube-study-space"
}
''')
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
override_resource {
  target = module.export_function[0].google_cloudfunctions_function.export
  values = { id = "projects/test-youtube-study-space/locations/asia-southeast2/functions/firestoreCollectionsExport" }
}
run "default_has_no_function_ownership_or_identity_requirement" {
  command = plan
  assert {
    condition = length(module.export_function) == 0
    error_message = "Default must leave Function ownership disabled."
  }
}
run "completed_scheduler_does_not_activate_function" {
  command = plan
  variables {
    manage_export_topic = true
    manage_export_scheduler = true
  }
  assert {
    condition = length(module.export_function) == 0
    error_message = "Existing CI topic/Scheduler flags must leave Function disabled."
  }
}
run "explicit_function_import_uses_adopted_topic" {
  command = plan
  variables {
    manage_export_topic = true
    manage_export_scheduler = true
    manage_export_function = true
    export_function_execution_service_account_email = "existing-execution@example.invalid"
  }
  assert {
    condition = length(module.export_function) == 1
    error_message = "Explicit development candidate may contain the existing Function."
  }
}
run "reject_missing_execution_identity" {
  command = plan
  variables {
    manage_export_topic = true
    manage_export_scheduler = true
    manage_export_function = true
  }
  expect_failures = [var.manage_export_function]
}
run "reject_function_before_scheduler_ownership" {
  command = plan
  variables {
    manage_export_topic = true
    manage_export_function = true
    export_function_execution_service_account_email = "existing-execution@example.invalid"
  }
  expect_failures = [var.manage_export_function]
}
run "reject_function_before_topic_ownership" {
  command = plan
  variables {
    manage_export_function = true
    export_function_execution_service_account_email = "existing-execution@example.invalid"
  }
  expect_failures = [var.manage_export_function]
}
run "reject_production_function_ownership" {
  command = plan
  variables {
    project_id = "youtube-study-space"
    manage_export_function = true
    export_function_execution_service_account_email = "existing-execution@example.invalid"
  }
  expect_failures = [var.manage_export_function]
}
''')
        root_plans = plans(target)
        assert len(root_plans) == 7 and not root_plans[0].get("resource_changes")
        assert {r["address"] for r in root_plans[1]["resource_changes"]} == {TOPIC, SCHEDULER}
        candidate = root_plans[2]
        assert {r["address"] for r in candidate["resource_changes"]} == {TOPIC, SCHEDULER, FUNCTION}
        function = next(r["change"]["after"] for r in candidate["resource_changes"] if r["address"] == FUNCTION)
        assert function["event_trigger"][0]["resource"] == "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"
        assert function["service_account_email"] == "existing-execution@example.invalid"
        assert not any(function.get(k) for k in ["labels", "terraform_labels", "source_archive_bucket", "source_archive_object", "source_repository"])
        imported = build_summary(candidate, environment="dev", git_sha="0" * 40, policy="import-only")
        assert imported["policy_passed"] and imported["counts"]["import"] == 3 and imported["counts"]["create"] == 0
    print("Function Gen1 fidelity/default-off/dependency/identity/dev-only contracts PASS; global guard rejects create.")


if __name__ == "__main__":
    main()
