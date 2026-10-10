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
        preflight = source.split("      - name: Enforce trusted execution surface\n", 1)[1]
        script = textwrap.dedent(preflight.split("        run: |\n", 1)[1].split("\n  plan:\n", 1)[0])
        modes = ("plan", "apply", "email-adoption", "quota-plan", "quota-create", "quota-refresh")
        for mode in modes:
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                output = Path(directory) / "output"
                env = credentialless_environment() | {
                    "GITHUB_EVENT_NAME": "workflow_dispatch",
                    "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
                    "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
                    "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
                    "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac", "GITHUB_SHA": "a" * 40,
                    "GITHUB_OUTPUT": str(output), "TARGET": "prod", "MODE": mode,
                    "HISTORY_POST_NOOP": "false",
                    "PROD_AUTHENTICATED_TERRAFORM_ENABLED": "false",
                }
                result = subprocess.run(["bash", "-c", script], env=env, capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("Production authenticated Terraform is intentionally disabled", result.stdout)
                self.assertFalse(output.exists(), "Production must not produce executable job inputs.")

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
