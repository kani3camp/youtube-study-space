#!/usr/bin/env python3
"""Credentialless synthetic metadata -> inputs -> native Terraform schema contracts."""
from __future__ import annotations

import copy
import json
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
GCP = ROOT / "infra/gcp"
sys.dont_write_bytecode = True
sys.path.insert(0, str(GCP / "scripts"))
from prepare_user_activity_history_adoption import normalize_field, prepare
from test_gcp_user_activity_schema_audit_workflow import (
    DUMMY_DESCRIPTIONS, UserActivityAdoptionPreparationTest, UserActivityHistoryPlanTest,
)
from terraform_plan_summary import build_summary


def plans(target: Path, env: dict) -> dict:
    for args in (("init", "-backend=false", "-input=false", "-lockfile=readonly"), ("test", "-json", "-verbose")):
        result = subprocess.run(["terraform", f"-chdir={target}", *args], env=env, capture_output=True, text=True)
        if result.returncode:
            raise RuntimeError("History native mock contract failed; raw output suppressed.")
    return {event["@testrun"]: event["test_plan"] for line in result.stdout.splitlines()
            if (event := json.loads(line)).get("type") == "test_plan"}


def main() -> None:
    metadata = UserActivityAdoptionPreparationTest().metadata(descriptions=True)
    metadata["schema"]["fields"].reverse()
    candidate = prepare(metadata)
    with tempfile.TemporaryDirectory(prefix="history-schema-contract-") as directory:
        base = Path(directory)
        cache = base / "cache"
        cache.mkdir()
        env = {key: value for key, value in os.environ.items()
               if not key.startswith(("AWS_", "GOOGLE_", "GCLOUD_", "GCP_", "TF_"))}
        env.update(TF_IN_AUTOMATION="true", TF_INPUT="false", TF_WORKSPACE="default", TF_PLUGIN_CACHE_DIR=str(cache))
        module = base / "modules/retained-user-activity-history"
        module.mkdir(parents=True)
        for name in ("main.tf", "schema.json"):
            shutil.copyfile(GCP / "modules/retained-user-activity-history" / name, module / name)
        lock = GCP / "environments/dev/.terraform.lock.hcl"
        shutil.copyfile(lock, module / lock.name)
        tests = module / "tests"
        tests.mkdir()
        # Only synthetic descriptions enter mock plans. The expected schema is
        # the independently constructed metadata, not Terraform's merge logic.
        (tests / "contract.tftest.hcl").write_text('''mock_provider "google" {}
variables {
  project_id = "test-youtube-study-space"
  dataset_id = "firestore_export"
  field_order = ''' + json.dumps(candidate["user_activity_history_field_order"]) + '''
  field_descriptions = ''' + json.dumps(DUMMY_DESCRIPTIONS, ensure_ascii=False) + '''
}
run "preserves_descriptions_and_order" { command = plan }
run "absent_descriptions_preserve_original_schema" {
  command = plan
  variables { field_descriptions = {} }
}
''' + "\n".join('''run "reject_description_%s" {
  command = plan
  variables { field_descriptions = %s }
  expect_failures = [var.field_descriptions]
}''' % (index, json.dumps(descriptions)) for index, descriptions in enumerate((
            {"timestamp": "Synthetic unknown column"}, {"__key__.unknown": "Synthetic unknown child"},
            {"seat_id.child": "Synthetic invalid nesting"}, {"is_member_seat": ""}, {"is_member_seat": None},
        ))) + '''
run "reject_legacy_order" {
  command = plan
  variables { field_order = ["seat_id", "timestamp", "user_id", "activity_type", "is_member_seat", "__key__", "__error__", "__has_error__"] }
  expect_failures = [var.field_order]
}
run "reject_duplicate_order" {
  command = plan
  variables { field_order = ["seat_id", "seat_id", "user_id", "activity_type", "is_member_seat", "__key__", "__error__", "__has_error__"] }
  expect_failures = [var.field_order]
}
''')
        native = plans(module, env)
        for run, described in (("preserves_descriptions_and_order", True), ("absent_descriptions_preserve_original_schema", False)):
            value = native[run]["resource_changes"][0]["change"]["after"]
            expected = metadata if described else UserActivityAdoptionPreparationTest().metadata()
            expected_fields = expected["schema"]["fields"] if described else list(reversed(expected["schema"]["fields"]))
            assert json.loads(value["schema"]) == expected_fields
            assert value["deletion_protection"] is True
            assert not native[run]["resource_changes"][0]["change"]["after_sensitive"].get("schema")
            assert not build_summary(native[run], environment="dev", git_sha="0" * 40, policy="import-only")["policy_passed"]

        # Exercise the actual dev root's input/module/import wiring. A local
        # dataset output stub keeps this test focused on the history boundary.
        target = base / "environments/dev"
        target.mkdir(parents=True)
        shutil.copyfile(GCP / "environments/dev/user-activity-history.tf", target / "user-activity-history.tf")
        shutil.copyfile(lock, target / lock.name)
        dataset = base / "dataset-stub"
        dataset.mkdir()
        (dataset / "main.tf").write_text('output "dataset_id" { value = "firestore_export" }\n')
        (target / "main.tf").write_text('''terraform {
  required_providers { google = { source = "hashicorp/google", version = "= 8.5.0" } }
}
provider "google" {}
variable "project_id" { default = "test-youtube-study-space" }
module "firestore_export_dataset" { source = "../../dataset-stub" }
''')
        inputs = {key: value for key, value in candidate.items() if key != "manage_user_activity_history"}
        inputs["manage_user_activity_history"] = True  # Test input only; source gate stays false.
        (target / "tests").mkdir()
        (target / "tests/boundary.tftest.hcl").write_text('''mock_provider "google" {}
run "default_ownership_off" { command = plan }
run "private_inputs_reach_module" {
  command = plan
  override_resource {
    target = module.user_activity_history[0].google_bigquery_table.retained
    values = {
      id = "projects/test-youtube-study-space/datasets/firestore_export/tables/user-activity-history"
      project = "test-youtube-study-space"
      dataset_id = "firestore_export"
      table_id = "user-activity-history"
      location = "asia-southeast2"
      deletion_protection = true
      description = ""
      friendly_name = ""
      labels = {}
      effective_labels = {}
      terraform_labels = {}
      resource_tags = {}
      max_staleness = ""
      require_partition_filter = false
      schema = ''' + json.dumps(native["preserves_descriptions_and_order"]["resource_changes"][0]["change"]["after"]["schema"], ensure_ascii=False) + '''
    }
  }
  variables {
''' + "\n".join(f"    {key} = {json.dumps(value, ensure_ascii=False)}" for key, value in inputs.items()) + '''
  }
}
''')
        native = plans(target, env)
        assert not native["default_ownership_off"].get("resource_changes")
        resources = native["private_inputs_reach_module"]["resource_changes"]
        assert len(resources) == 1 and resources[0]["address"] == "module.user_activity_history[0].google_bigquery_table.retained"
        value = resources[0]["change"]["after"]
        assert [normalize_field(field) for field in json.loads(value["schema"])] == [
            normalize_field(field) for field in metadata["schema"]["fields"]]
        change = resources[0]["change"]
        assert change["actions"] == ["no-op"] and change["before"] == change["after"]
        assert change["importing"] == {"id": "projects/test-youtube-study-space/datasets/firestore_export/tables/user-activity-history"}
        assert not change["after_sensitive"].get("schema")
        summary = build_summary(native["private_inputs_reach_module"], environment="dev", git_sha="0" * 40, policy="import-only")
        assert summary["policy_passed"] and summary["counts"]["import"] == summary["counts"]["no-op"] == 1
        for key in ("create", "update", "delete", "replace", "read", "other", "drift"):
            assert summary["counts"][key] == 0
        # Join the real rendered mock schema to the synthetic existing-eleven
        # fixture, then run the unchanged full-root import/post strict contract.
        fixture = UserActivityHistoryPlanTest()
        fixture.setUp()
        for phase in ("before", "post"):
            full_plan, _ = fixture.fixture()
            full_plan["resource_changes"][-1] = copy.deepcopy(resources[0])
            if phase == "post":
                del full_plan["resource_changes"][-1]["change"]["importing"]
            fixture.review(full_plan, metadata, phase)
    print("History synthetic native module/root contracts PASS; descriptions preserved, default ownership off; no live plan.")


if __name__ == "__main__":
    main()
