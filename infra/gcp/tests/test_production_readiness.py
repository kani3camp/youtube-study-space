#!/usr/bin/env python3
"""Credentialless production root/preflight contracts; no live readiness claim."""
from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
GCP = ROOT / "infra/gcp"
WORKFLOW = ROOT / ".github/workflows/gcp-terraform-authenticated.yml"
sys.dont_write_bytecode = True
sys.path.insert(0, str(ROOT / ".github/scripts"))
from terraform_plan_summary import build_summary


def credentialless_environment() -> dict[str, str]:
    # Do not let local credentials, Terraform inputs or CLI flags affect the mock.
    return {
        key: value for key, value in os.environ.items()
        if not key.startswith(("AWS_", "GOOGLE_", "GCLOUD_", "GCP_", "CLOUDSDK_", "TF_"))
    } | {"TF_IN_AUTOMATION": "true", "TF_INPUT": "false", "TF_WORKSPACE": "default"}


class ProductionReadinessTest(unittest.TestCase):
    def preflight(self, mode="plan", **overrides):
        source = WORKFLOW.read_text()
        preflight = source.split("      - name: Enforce trusted execution surface\n", 1)[1]
        script = textwrap.dedent(preflight.split("        run: |\n", 1)[1].split("\n  plan:\n", 1)[0])
        gates = dict(re.findall(r'(?m)^  ([A-Z_0-9]+): "(true|false)"$', source))
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "output"
            env = credentialless_environment() | gates | {
                "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_RUN_ATTEMPT": "1",
                "GITHUB_ACTOR": "kani3camp", "GITHUB_TRIGGERING_ACTOR": "kani3camp", "GITHUB_ACTOR_ID": "54093651",
                "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
                "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
                "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
                "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac", "GITHUB_SHA": "a" * 40,
                "GITHUB_OUTPUT": str(output), "TARGET": "prod", "MODE": mode,
                "HISTORY_POST_NOOP": "false", "CURRENT_STATE_ATTESTATION": "false",
                "OWNERSHIP_WAVE": "none", "OWNERSHIP_ROOT_BASIS": "original-history-import",
                "HISTORY_IMPORT_APPROVAL": "", "HISTORY_PLAN_SAFETY_EVIDENCE": "",
                "PLAN_COST_EVIDENCE": "",
            } | overrides
            result = subprocess.run(["bash", "-c", script], env=env, capture_output=True, text=True)
            outputs = dict(line.split("=", 1) for line in output.read_text().splitlines()) if output.exists() else {}
            return result, outputs

    def assert_stops_before_job_inputs(self, mode="plan", **overrides):
        result, outputs = self.preflight(mode, **overrides)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(outputs, {}, "Rejected production inputs must not produce executable job inputs.")
        return result

    def test_backend_example_uses_only_prod_paths_and_private_placeholders(self):
        source = (GCP / "environments/prod/backend.hcl.example").read_text()
        assignments = dict(re.findall(r"(?m)^([a-z_]+)\s*=\s*(.+)$", source))
        self.assertEqual(assignments, {
            "bucket": '"REPLACE_WITH_PERSONAL_TERRAFORM_STATE_BUCKET"',
            "key": '"youtube-study-space/prod/terraform.tfstate"',
            "region": '"REPLACE_WITH_STATE_BUCKET_AWS_REGION"',
            "encrypt": "true", "use_lockfile": "true",
            "workspace_key_prefix": '"youtube-study-space/prod/workspaces"',
            "allowed_account_ids": '["REPLACE_WITH_STATE_AWS_ACCOUNT_ID"]',
        })

    def test_every_production_mode_stops_in_preflight_before_authentication(self):
        source = WORKFLOW.read_text()
        self.assertIn('PROD_AUTHENTICATED_TERRAFORM_ENABLED: "false"', source)
        modes = ("plan", "apply", "security-probe", "email-adoption", "quota-plan", "quota-create", "quota-refresh", "current-state-read")
        for mode in modes:
            with self.subTest(mode=mode):
                result = self.assert_stops_before_job_inputs(mode)
                if mode != "current-state-read":
                    self.assertIn("Production authenticated Terraform is intentionally disabled", result.stdout)

    def test_single_production_plan_gate_cannot_bypass_missing_contract_or_development_exit(self):
        for ready in ({}, {"PROD_EXECUTION_CONTRACT_READY": "true"}, {"PROD_DEVELOPMENT_EXIT_READY": "true"}):
            with self.subTest(ready=ready):
                result = self.assert_stops_before_job_inputs(PROD_AUTHENTICATED_TERRAFORM_ENABLED="true", **ready)
                self.assertIn("Production execution contract and measured development exit are not ready", result.stdout)

    def test_plan_activation_keeps_apply_independently_closed_and_outputs_exact_prod_inputs(self):
        gates = {"PROD_AUTHENTICATED_TERRAFORM_ENABLED": "true", "PROD_EXECUTION_CONTRACT_READY": "true",
                 "PROD_DEVELOPMENT_EXIT_READY": "true"}
        result, outputs = self.preflight(**gates)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(outputs, {
            "terraform_root": "infra/gcp/environments/prod", "project_id": "youtube-study-space",
            "state_key": "youtube-study-space/prod/terraform.tfstate", "ownership_wave": "none",
            **{name: "false" for name in ("manage_primary_email", "manage_quota", "manage_export_topic",
                                          "manage_export_scheduler", "manage_export_function", "manage_user_activity_history")},
        })
        result = self.assert_stops_before_job_inputs("apply", **gates)
        self.assertIn("Production apply is independently disabled", result.stdout)
        result, apply_outputs = self.preflight("apply", **gates, PROD_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true")
        self.assertEqual(result.returncode, 0)
        self.assertEqual(apply_outputs, outputs)

    def test_production_rejects_development_modes_inputs_and_reruns_even_when_all_gates_are_open(self):
        gates = {name: "true" for name in ("PROD_AUTHENTICATED_TERRAFORM_ENABLED", "PROD_EXECUTION_CONTRACT_READY",
                                          "PROD_DEVELOPMENT_EXIT_READY", "PROD_AUTHENTICATED_TERRAFORM_APPLY_ENABLED")}
        for mode in ("security-probe", "email-adoption", "quota-plan", "quota-create", "quota-refresh", "current-state-read"):
            with self.subTest(mode=mode):
                self.assert_stops_before_job_inputs(mode, **gates)
        for key, value in (("HISTORY_POST_NOOP", "true"), ("CURRENT_STATE_ATTESTATION", "true"),
                           ("OWNERSHIP_WAVE", "pool"), ("OWNERSHIP_ROOT_BASIS", "current-state-bootstrap"),
                           ("HISTORY_IMPORT_APPROVAL", "development-receipt"),
                           ("HISTORY_PLAN_SAFETY_EVIDENCE", "development-safety"),
                           ("PLAN_COST_EVIDENCE", "legacy"), ("GITHUB_RUN_ATTEMPT", "2")):
            with self.subTest(key=key):
                self.assert_stops_before_job_inputs(**gates, **{key: value})

    def test_production_gates_do_not_change_development_plan_routing(self):
        closed_runtime = {name: "false" for name in (
            "DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED", "DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY",
            "DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED", "DEV_RUNTIME_OWNERSHIP_SPLIT_INPUT_ENABLED")}
        production_open = {name: "true" for name in (
            "PROD_AUTHENTICATED_TERRAFORM_ENABLED", "PROD_EXECUTION_CONTRACT_READY",
            "PROD_DEVELOPMENT_EXIT_READY", "PROD_AUTHENTICATED_TERRAFORM_APPLY_ENABLED")}
        for wave, development in (("none", closed_runtime), ("pool", {**{name: "true" for name in closed_runtime},
                "OWNERSHIP_WAVE": "pool", "OWNERSHIP_ROOT_BASIS": "current-state-bootstrap"})):
            for production in ({}, production_open):
                with self.subTest(wave=wave, production=production):
                    result, outputs = self.preflight(TARGET="dev", **development, **production)
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertEqual(outputs, {
                        "terraform_root": "infra/gcp/environments/dev", "project_id": "test-youtube-study-space",
                        "state_key": "youtube-study-space/dev/terraform.tfstate", "ownership_wave": wave,
                        **{name: "true" for name in ("manage_primary_email", "manage_quota", "manage_export_topic",
                                                      "manage_export_scheduler", "manage_export_function", "manage_user_activity_history")},
                    })
        # Runtime activation requires a selection before any job inputs.
        self.assert_stops_before_job_inputs(TARGET="dev", DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED="true")

    def test_readiness_ci_has_no_cloud_authorization_or_state_operations(self):
        source = (ROOT / ".github/workflows/gcp-production-readiness.yml").read_text()
        self.assertIn("pull_request:", source)
        self.assertIn("contents: read", source)
        for forbidden in ("id-token:", "secrets", "environment:", "workflow_dispatch:",
                          "pull_request_target", "upload-artifact", "configure-aws-credentials", "google-github-actions/auth",
                          "terraform apply", "terraform import", "-backend-config"):
            self.assertNotIn(forbidden, source)
        for action in re.findall(r"(?m)^\s+uses: (\S+)", source):
            self.assertRegex(action, r"@[0-9a-f]{40}$")
        self.assertIn("python3 infra/gcp/tests/test_production_readiness.py", source)


class ProductionMockPlanTest(unittest.TestCase):
    EXISTING_ADDRESSES = {
        "backup_schedule": "google_firestore_backup_schedule.daily[0]",
        "backup_bucket": "module.backup_bucket[0].google_storage_bucket.retained",
        "dataset": "module.retained_dataset[0].google_bigquery_dataset.retained",
        "order_history": "module.order_history[0].google_bigquery_table.retained",
        "user_activity_history": "module.user_activity_history[0].google_bigquery_table.retained",
        "export_topic": "module.export_topic[0].google_pubsub_topic.export",
        "export_scheduler": "module.export_scheduler[0].google_cloud_scheduler_job.export",
        "export_function": "module.export_function[0].google_cloudfunctions_function.export",
        **{f"quota_{key}": f'module.existing_youtube_quota_alerts.google_monitoring_alert_policy.existing["{key}"]'
           for key in ("minute_80", "day_80", "day_60")},
    }

    @classmethod
    def setUpClass(cls):
        # Copy only source and lock files into a fresh backendless tree; never a
        # live backend config, credentials, local state, tfvars or provider cache.
        with tempfile.TemporaryDirectory(prefix="prod-readiness-") as directory:
            target = Path(directory) / "environments/prod"
            target.mkdir(parents=True)
            for source in (GCP / "environments/prod").glob("*.tf"):
                shutil.copyfile(source, target / source.name)
            shutil.copyfile(GCP / "environments/prod/.terraform.lock.hcl", target / ".terraform.lock.hcl")
            shutil.copytree(GCP / "environments/prod/tests", target / "tests")
            for name in ("monitoring-notification-channels", "youtube-quota-alerts", "retained-backup-bucket",
                         "retained-bigquery-dataset", "retained-order-history", "retained-user-activity-history",
                         "firestore-export-topic", "firestore-export-scheduler", "firestore-export-function"):
                shutil.copytree(GCP / "modules" / name, Path(directory) / "modules" / name,
                                ignore=shutil.ignore_patterns(".terraform", "*.tfstate*", "*.tfplan", "*.tfvars*"))
            env = credentialless_environment()
            for args in (("init", "-backend=false", "-lockfile=readonly", "-input=false"),
                         ("test", "-json", "-verbose")):
                result = subprocess.run(["terraform", f"-chdir={target}", *args], env=env,
                                        capture_output=True, text=True, check=False)
                if result.returncode:
                    raise RuntimeError("Production backendless mock failed; raw output suppressed.")
            cls.plans = {
                event["@testrun"]: event["test_plan"]
                for line in result.stdout.splitlines()
                if (event := json.loads(line)).get("type") == "test_plan"
            }

    def test_default_root_has_zero_resource_actions_imports_and_drift(self):
        summary = build_summary(self.plans["default_production_owns_nothing"],
                                environment="prod", git_sha="0" * 40, policy="import-only")
        self.assertTrue(summary["policy_passed"])
        self.assertEqual(summary["resources"], [])
        self.assertTrue(all(count == 0 for count in summary["counts"].values()))

    def existing_change(self, alias):
        changes = self.plans[f"one_existing_{alias}"]["resource_changes"]
        self.assertEqual(len(changes), 1)
        return changes[0]["change"]

    @staticmethod
    def has_sensitive_leaf(mask):
        if isinstance(mask, dict):
            return any(ProductionMockPlanTest.has_sensitive_leaf(value) for value in mask.values())
        if isinstance(mask, list):
            return any(ProductionMockPlanTest.has_sensitive_leaf(value) for value in mask)
        return mask is True

    def test_private_metadata_alone_cannot_activate_any_resource(self):
        summary = build_summary(self.plans["private_metadata_alone_does_not_activate_ownership"],
                                environment="prod", git_sha="0" * 40, policy="import-only")
        self.assertTrue(summary["policy_passed"])
        self.assertEqual(summary["resources"], [])
        self.assertTrue(all(count == 0 for count in summary["counts"].values()))

    def test_each_existing_alias_selects_only_one_exact_import_address(self):
        for alias, address in self.EXISTING_ADDRESSES.items():
            with self.subTest(alias=alias):
                summary = build_summary(self.plans[f"one_existing_{alias}"],
                                        environment="prod", git_sha="0" * 40, policy="import-only")
                self.assertTrue(summary["policy_passed"])
                self.assertEqual(summary["counts"], {
                    "import": 1, "no-op": 1, **{key: 0 for key in
                    ("create", "update", "delete", "replace", "read", "other", "drift")},
                })
                self.assertEqual([resource["address"] for resource in summary["resources"]], [address])
                change = self.existing_change(alias)
                self.assertEqual(change["importing"]["id"], change["after"]["id"])
                self.assertFalse(self.has_sensitive_leaf(change["after_sensitive"]))

    def test_cumulative_selection_keeps_all_eleven_addresses_without_creation(self):
        summary = build_summary(self.plans["cumulative_existing_resources"], environment="prod",
                                git_sha="0" * 40, policy="import-only")
        self.assertTrue(summary["policy_passed"])
        self.assertEqual(summary["counts"]["import"], 11)
        self.assertEqual(summary["counts"]["no-op"], 11)
        self.assertEqual({resource["address"] for resource in summary["resources"]},
                         set(self.EXISTING_ADDRESSES.values()))

    def test_existing_retention_location_and_table_schema_are_preserved(self):
        bucket = self.existing_change("backup_bucket")["after"]
        self.assertEqual((bucket["location"], bucket["storage_class"]), ("ASIA-NORTHEAST2", "COLDLINE"))
        self.assertEqual(bucket["retention_policy"], [{"retention_period": "8035200", "is_locked": False}])
        self.assertEqual(bucket["soft_delete_policy"][0]["retention_duration_seconds"], 604800)
        self.assertFalse(bucket["force_destroy"])
        self.assertTrue(bucket["uniform_bucket_level_access"])
        self.assertEqual(bucket["public_access_prevention"], "enforced")
        dataset = self.existing_change("dataset")["after"]
        self.assertEqual((dataset["dataset_id"], dataset["location"]), ("fixture_export", "asia-northeast2"))
        self.assertEqual(str(dataset["max_time_travel_hours"]), "168")
        self.assertFalse(dataset["delete_contents_on_destroy"])
        for alias, fields in (
            ("order_history", ["ordered_at", "seat_id", "user_id", "menu_code", "is_member_seat", "__key__", "__error__", "__has_error__"]),
            ("user_activity_history", ["seat_id", "taken_at", "activity_type", "user_id", "__key__", "__error__", "__has_error__", "is_member_seat"]),
        ):
            change = self.existing_change(alias)
            schema = json.loads(change["after"]["schema"])
            self.assertEqual([field["name"] for field in schema], fields)
            self.assertTrue(change["after"]["deletion_protection"])
            self.assertFalse(change["after_sensitive"].get("schema", False))
            if alias == "user_activity_history":
                self.assertEqual(schema[1]["description"], "Synthetic existing description")
            else:
                self.assertFalse(any("description" in field for field in schema))

    def test_export_chain_uses_existing_names_and_production_runtime_settings(self):
        function = self.existing_change("export_function")["after"]
        self.assertEqual((function["name"], function["runtime"], function["entry_point"]),
                         ("fixtureExistingFunction", "nodejs22", "scheduledFirestoreExport"))
        self.assertEqual((function["available_memory_mb"], function["timeout"],
                          function["min_instances"], function["max_instances"]), (512, 120, 0, 1))
        self.assertEqual(function["environment_variables"], {
            "YSS_EXPORT_ENVIRONMENT": "production", "YSS_EXPORT_PROJECT_ID": "youtube-study-space",
        })
        self.assertEqual(function["service_account_email"], "fixture-execution@youtube-study-space.iam.gserviceaccount.com")
        self.assertFalse(function["event_trigger"][0]["failure_policy"][0]["retry"])
        self.assertFalse(function.get("source_archive_bucket"))
        self.assertFalse(function.get("source_repository"))
        scheduler = self.existing_change("export_scheduler")["after"]
        self.assertEqual((scheduler["name"], scheduler["region"], scheduler["schedule"], scheduler["time_zone"]),
                         ("fixture-existing-scheduler", "asia-northeast2", "0 0 * * *", "Asia/Tokyo"))
        self.assertFalse(scheduler["paused"])
        self.assertEqual(scheduler["pubsub_target"][0]["topic_name"], function["event_trigger"][0]["resource"])
        self.assertEqual(scheduler["pubsub_target"][0]["data"], "c3RhcnQgZXhwb3J0")
        self.assertEqual(scheduler["retry_config"][0]["retry_count"], 3)
        topic = self.existing_change("export_topic")["after"]
        self.assertEqual(topic["name"], "fixture-existing-topic")
        self.assertEqual(topic["labels"], {})

    def test_existing_quota_keeps_distinct_dangling_references_and_documentation(self):
        for key, label, suffix in (("minute_80", "minute", "minute"), ("day_80", "high day", "day-high"),
                                   ("day_60", "warning day", "day-warning")):
            policy = self.existing_change(f"quota_{key}")["after"]
            self.assertEqual(policy["display_name"], f"Existing {label} fixture")
            self.assertEqual(policy["documentation"][0], {
                "content": f"Existing {label} documentation fixture", "mime_type": "text/markdown", "subject": None, "links": [],
            })
            self.assertEqual(policy["notification_channels"],
                             [f"projects/youtube-study-space/notificationChannels/fixture-missing-{suffix}"])
            self.assertTrue(policy["enabled"])
            self.assertEqual(policy["combiner"], "OR")
            condition = policy["conditions"][0]["condition_monitoring_query_language"][0]
            self.assertEqual(condition["duration"], "60s")
            self.assertEqual(condition["trigger"][0]["count"], 1)
            self.assertIn("resource.project_id=='youtube-study-space'", condition["query"])
            self.assertIn("0.6 '1'" if key == "day_60" else "0.8 '1'", condition["query"])
            self.assertEqual(policy["alert_strategy"][0]["auto_close"], "604800s")
            # Both shared-module resources must retain identical workload
            # behavior; only existing name/docs/references and import differ.
            legacy = next(resource["change"]["after"] for resource in
                          self.plans["quota_opt_in_is_a_create_not_an_import"]["resource_changes"]
                          if resource["address"].endswith(f'["{key}"]'))
            self.assertEqual(policy["conditions"][0]["condition_monitoring_query_language"],
                             legacy["conditions"][0]["condition_monitoring_query_language"])
            for attribute in ("alert_strategy", "combiner", "enabled", "severity", "user_labels"):
                self.assertEqual(policy[attribute], legacy[attribute])

    def test_email_opt_in_cannot_pass_the_existing_import_only_guard(self):
        summary = build_summary(self.plans["email_opt_in_is_a_create_not_an_import"],
                                environment="prod", git_sha="0" * 40, policy="import-only")
        self.assertFalse(summary["policy_passed"])
        self.assertEqual(summary["counts"]["create"], 1)
        self.assertEqual(summary["counts"]["import"], 0)
        self.assertEqual(summary["resources"][0]["address"],
                         "module.notification_channels.google_monitoring_notification_channel.primary_email[0]")

    def test_quota_opt_in_cannot_pass_the_existing_import_only_guard(self):
        summary = build_summary(self.plans["quota_opt_in_is_a_create_not_an_import"],
                                environment="prod", git_sha="0" * 40, policy="import-only")
        self.assertFalse(summary["policy_passed"])
        self.assertEqual(summary["counts"]["create"], 3)
        self.assertEqual(summary["counts"]["import"], 0)
        self.assertEqual({resource["address"] for resource in summary["resources"]}, {
            f'module.youtube_quota_alerts.google_monitoring_alert_policy.quota["{key}"]'
            for key in ("minute_80", "day_80", "day_60")
        })
        # The established creation path (also used by development) keeps its
        # original sensitive channel attribute and exact resource addresses.
        self.assertTrue(all(resource["change"]["after_sensitive"]["notification_channels"]
                            for resource in self.plans["quota_opt_in_is_a_create_not_an_import"]["resource_changes"]))


if __name__ == "__main__":
    unittest.main()
