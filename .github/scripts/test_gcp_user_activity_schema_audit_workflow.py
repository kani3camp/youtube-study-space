#!/usr/bin/env python3

from __future__ import annotations

import os
import subprocess
import textwrap
import unittest
from pathlib import Path

GITHUB_DIR = Path(__file__).parents[1]
WORKFLOW = GITHUB_DIR / "workflows" / "gcp-user-activity-schema-audit.yml"
CALLER = GITHUB_DIR / "workflows" / "ci.yml"


class UserActivitySchemaAuditWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.text = WORKFLOW.read_text(encoding="utf-8")
        cls.caller = CALLER.read_text(encoding="utf-8")

    def test_reusable_workflow_is_not_directly_triggerable(self) -> None:
        self.assertIn("workflow_call:", self.text)
        self.assertNotIn("workflow_dispatch:", self.text)
        self.assertNotIn("pull_request_target", self.text)
        self.assertNotIn("secrets: inherit", self.text)

    def test_audit_is_default_off_and_development_only(self) -> None:
        self.assertIn('DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED: "false"', self.text)
        self.assertIn('[[ "${TARGET}" == "dev" ]]', self.text)
        self.assertIn("Production schema audit is not enabled", self.text)
        self.assertIn("pending dedicated IAM and user approval", self.text)

    def test_trusted_surface_is_fixed_before_auth(self) -> None:
        guard = self.text.split("      - name: Enforce trusted read-only audit surface", 1)[1]
        auth = guard.index("      - name: Configure dedicated GCP audit credential")
        for token in (
            'GITHUB_EVENT_NAME}" == "workflow_dispatch"',
            'GITHUB_REPOSITORY}" == "kani3camp/youtube-study-space"',
            'GITHUB_REPOSITORY_ID}" == "340900071"',
            'GITHUB_REPOSITORY_OWNER_ID}" == "54093651"',
            "refs/heads/feature/gcp-terraform-iac",
        ):
            self.assertLess(guard.index(token), auth)

    def test_dedicated_environment_and_identity_are_separate_from_terraform(self) -> None:
        self.assertIn("name: gcp-dev-user-activity-schema-audit", self.text)
        self.assertIn("GCP_USER_ACTIVITY_SCHEMA_AUDIT_SERVICE_ACCOUNT", self.text)
        self.assertNotIn("GCP_TERRAFORM_SERVICE_ACCOUNT", self.text)
        self.assertIn("GCP_TERRAFORM_WIF_PROVIDER", self.text)
        self.assertNotIn("AWS_TERRAFORM_", self.text)
        self.assertNotIn("terraform init", self.text)
        self.assertNotIn("terraform plan", self.text)
        self.assertNotIn("terraform apply", self.text)

    def test_business_data_output_stays_private(self) -> None:
        self.assertIn(' >"${RUNNER_TEMP}/user-activity-schema-audit.json"'.strip(), self.text)
        self.assertIn("Exact aggregate counts are intentionally not published.", self.text)
        self.assertNotIn("upload-artifact", self.text)
        self.assertNotIn('cat "${RUNNER_TEMP}/user-activity-schema-audit.json"', self.text)
        for key in ("legacy_non_null", "legacy_only", "both_equal", "both_different"):
            self.assertIn(f"{key} is zero", self.text)

    def test_cli_scope_is_exact(self) -> None:
        self.assertIn("go run ./cmd/user-activity-schema-audit", self.text)
        self.assertIn("test-youtube-study-space", self.text)
        self.assertIn("asia-southeast2", self.text)
        self.assertIn('target.get("dataset") != "firestore_export"', self.text)
        self.assertIn('target.get("table") != "user-activity-history"', self.text)

    def test_caller_exposes_only_explicit_manual_toggle(self) -> None:
        self.assertIn("gcp_user_activity_schema_audit:", self.caller)
        self.assertIn("inputs.gcp_user_activity_schema_audit == true", self.caller)
        self.assertIn("uses: ./.github/workflows/gcp-user-activity-schema-audit.yml", self.caller)
        call = self.caller.split("  gcp-user-activity-schema-audit:\n", 1)[1].split("\n  ci-gate:\n", 1)[0]
        self.assertNotIn("secrets:", call)
        self.assertIn("target: dev", call)

    def run_guard(self, **overrides: str) -> subprocess.CompletedProcess[str]:
        script = (
            self.text.split("      - name: Enforce trusted read-only audit surface", 1)[1]
            .split("        run: |\n", 1)[1]
            .split("\n      - name: Checkout trusted commit", 1)[0]
        )
        env = {
            "PATH": os.environ["PATH"],
            "GITHUB_EVENT_NAME": "workflow_dispatch",
            "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
            "GITHUB_REPOSITORY_ID": "340900071",
            "GITHUB_REPOSITORY_OWNER_ID": "54093651",
            "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
            "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac",
            "TARGET": "dev",
            "DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED": "false",
        }
        env.update(overrides)
        return subprocess.run(
            ["bash", "-c", textwrap.dedent(script)],
            env=env,
            capture_output=True,
            text=True,
        )

    def test_guard_rejects_until_enabled_and_rejects_production(self) -> None:
        self.assertNotEqual(self.run_guard().returncode, 0)
        self.assertEqual(
            self.run_guard(DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED="true").returncode,
            0,
        )
        self.assertNotEqual(
            self.run_guard(
                DEV_USER_ACTIVITY_SCHEMA_AUDIT_ENABLED="true",
                TARGET="prod",
            ).returncode,
            0,
        )


if __name__ == "__main__":
    unittest.main()
