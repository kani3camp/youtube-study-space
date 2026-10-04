#!/usr/bin/env python3

from __future__ import annotations

import re
import unittest
from pathlib import Path

GITHUB_DIR = Path(__file__).parents[1]
WORKFLOW = GITHUB_DIR / "workflows" / "gcp-terraform-authenticated.yml"
CALLER = GITHUB_DIR / "workflows" / "ci.yml"


class TerraformAuthenticatedWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.text = WORKFLOW.read_text(encoding="utf-8")
        cls.caller = CALLER.read_text(encoding="utf-8")

    def test_reusable_workflow_is_not_directly_triggerable(self) -> None:
        self.assertIn("workflow_call:", self.text)
        self.assertNotIn("workflow_dispatch:", self.text)
        self.assertNotIn("pull_request_target", self.text)
        self.assertNotRegex(self.text, r"(?m)^\s*pull_request\s*:")

    def test_default_branch_ci_is_the_manual_dispatch_entrypoint(self) -> None:
        self.assertIn("workflow_dispatch:", self.caller)
        self.assertIn("terraform_authenticated:", self.caller)
        self.assertIn("terraform_target:", self.caller)
        self.assertIn("terraform_mode:", self.caller)
        self.assertIn("uses: ./.github/workflows/gcp-terraform-authenticated.yml", self.caller)
        self.assertIn("github.event_name == 'workflow_dispatch'", self.caller)
        self.assertIn("inputs.terraform_authenticated == true", self.caller)
        self.assertNotIn("secrets: inherit", self.caller)

    def test_source_control_skeleton_is_fail_closed_until_trust_mutation(self) -> None:
        self.assertIn('DEV_AUTHENTICATED_TERRAFORM_ENABLED: "false"', self.text)
        self.assertIn('PROD_AUTHENTICATED_TERRAFORM_ENABLED: "false"', self.text)
        self.assertIn("refs/heads/feature/gcp-terraform-iac", self.text)
        self.assertIn('GITHUB_REPOSITORY_ID}" == "340900071"', self.text)
        self.assertIn('GITHUB_REPOSITORY_OWNER_ID}" == "54093651"', self.text)
        self.assertIn(
            'GITHUB_WORKFLOW_REF}" == "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac"',
            self.text,
        )

    def test_oidc_permission_is_limited_to_authenticated_call_and_jobs(self) -> None:
        self.assertEqual(self.text.count("id-token: write"), 2)
        self.assertEqual(self.caller.count("id-token: write"), 1)
        preflight = self.text.split("  plan:\n", 1)[0]
        self.assertNotIn("id-token: write", preflight)

    def test_authenticated_jobs_use_environment_boundaries(self) -> None:
        self.assertIn("name: terraform-${{ inputs.target }}-plan", self.text)
        self.assertIn("name: terraform-${{ inputs.target }}-apply", self.text)

    def test_no_saved_plan_artifact_or_cache(self) -> None:
        for value in ("actions/upload-artifact", "actions/download-artifact", "actions/cache", "-auto-approve"):
            self.assertNotIn(value, self.text)
        self.assertNotRegex(self.text, r"(?m)^\s*uses:\s*.*upload-artifact")

    def test_raw_terraform_output_is_redirected(self) -> None:
        self.assertIn('>"${RUNNER_TEMP}/terraform-init.log" 2>&1', self.text)
        self.assertIn('>"${RUNNER_TEMP}/terraform-plan.log" 2>&1', self.text)
        self.assertIn('>"${RUNNER_TEMP}/terraform-apply.log" 2>&1', self.text)
        self.assertNotIn('cat "${RUNNER_TEMP}/terraform-plan.log"', self.text)
        self.assertNotIn('cat "${RUNNER_TEMP}/terraform-apply.log"', self.text)
        self.assertIn("terraform_plan_summary.py", self.text)

    def test_external_actions_are_full_sha_pinned(self) -> None:
        action_lines = re.findall(r"(?m)^\s*uses:\s*([^\s#]+)", self.text)
        self.assertGreaterEqual(len(action_lines), 8)
        for action in action_lines:
            if action.startswith("./"):
                continue
            self.assertRegex(action, r"^[^@]+@[0-9a-f]{40}$", action)

    def test_checkout_does_not_persist_github_token(self) -> None:
        self.assertEqual(self.text.count("persist-credentials: false"), 2)

    def test_import_only_policy_is_enforced_before_apply_and_post_apply(self) -> None:
        self.assertGreaterEqual(self.text.count("--policy import-only"), 3)
        self.assertIn("Re-plan does not match the approved sanitized projection", self.text)
        self.assertIn("Post-apply Terraform plan is not no-op", self.text)

    def test_private_identifiers_are_environment_secret_inputs_not_literals(self) -> None:
        required_secret_names = (
            "AWS_TERRAFORM_BACKEND_ROLE_ARN",
            "AWS_TERRAFORM_STATE_BUCKET",
            "AWS_TERRAFORM_STATE_ACCOUNT_ID",
            "GCP_TERRAFORM_WIF_PROVIDER",
            "GCP_TERRAFORM_SERVICE_ACCOUNT",
        )
        for name in required_secret_names:
            self.assertIn(f"secrets.{name}", self.text)


if __name__ == "__main__":
    unittest.main()
