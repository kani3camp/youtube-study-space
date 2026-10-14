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
        if not key.startswith(("AWS_", "GOOGLE_", "GCLOUD_", "GCP_", "TF_"))
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
        modes = ("plan", "apply", "security-probe", "email-adoption", "quota-plan", "quota-create", "quota-refresh")
        for mode in modes:
            with self.subTest(mode=mode):
                result = self.assert_stops_before_job_inputs(mode)
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
        for mode in ("security-probe", "email-adoption", "quota-plan", "quota-create", "quota-refresh"):
            with self.subTest(mode=mode):
                self.assert_stops_before_job_inputs(mode, **gates)
        for key, value in (("HISTORY_POST_NOOP", "true"), ("CURRENT_STATE_ATTESTATION", "true"),
                           ("OWNERSHIP_WAVE", "pool"), ("OWNERSHIP_ROOT_BASIS", "current-state-bootstrap"),
                           ("HISTORY_IMPORT_APPROVAL", "development-receipt"),
                           ("HISTORY_PLAN_SAFETY_EVIDENCE", "development-safety"),
                           ("PLAN_COST_EVIDENCE", "legacy"), ("GITHUB_RUN_ATTEMPT", "2")):
            with self.subTest(key=key):
                self.assert_stops_before_job_inputs(**gates, **{key: value})

    def test_production_gates_do_not_change_development_default_plan_routing(self):
        result, outputs = self.preflight(TARGET="dev")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(outputs["terraform_root"], "infra/gcp/environments/dev")
        self.assertEqual(outputs["state_key"], "youtube-study-space/dev/terraform.tfstate")
        self.assertEqual(outputs["manage_quota"], "true")
        self.assertEqual(outputs["manage_user_activity_history"], "true")

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
            for name in ("monitoring-notification-channels", "youtube-quota-alerts"):
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


if __name__ == "__main__":
    unittest.main()
