#!/usr/bin/env python3
"""Execute routing denial cases and verify the protected metadata-only surface."""
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).parent))
import test_terraform_authenticated_workflow as workflow_contract


class CurrentStateReadWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        workflow_contract.TerraformAuthenticatedWorkflowTest.setUpClass()
        cls.harness = workflow_contract.TerraformAuthenticatedWorkflowTest()
        cls.text = cls.harness.text
        cls.reader = cls.text.split("  current_state_read:\n", 1)[1].split("  apply:\n", 1)[0]

    def route(self, **overrides):
        return self.harness.run_preflight(**{
            "MODE": "current-state-read", "TARGET": "dev",
            "DEV_CURRENT_STATE_READ_ENABLED": "true",
            "DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED": "true",
            "HISTORY_POST_NOOP": "false", "CURRENT_STATE_ATTESTATION": "false",
            "GITHUB_RUN_ATTEMPT": "1", "OWNERSHIP_WAVE": "none",
            "OWNERSHIP_ROOT_BASIS": "original-history-import",
            "HISTORY_IMPORT_APPROVAL": "", "PLAN_COST_EVIDENCE": "",
            "PLAN_SAFETY_EVIDENCE": "", **overrides})

    def test_enabled_route_only_selects_development_metadata(self):
        result = self.route()
        self.assertEqual(result.returncode, 0)
        self.assertEqual(set(result.outputs.splitlines()), {
            "state_key=youtube-study-space/dev/terraform.tfstate",
            "project_id=test-youtube-study-space", "ownership_wave=none"})

    def test_closed_gate_and_mixed_or_untrusted_inputs_stop_before_credentials(self):
        for key, value in {
            "DEV_CURRENT_STATE_READ_ENABLED": "false", "TARGET": "prod",
            "DEV_AUTHENTICATED_TERRAFORM_ENABLED": "false",
            "DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED": "false",
            "HISTORY_POST_NOOP": "true", "CURRENT_STATE_ATTESTATION": "true",
            "OWNERSHIP_WAVE": "pool", "OWNERSHIP_ROOT_BASIS": "current-state-bootstrap",
            "GITHUB_RUN_ATTEMPT": "2", "GITHUB_EVENT_NAME": "pull_request",
            "GITHUB_REF": "refs/heads/dev", "GITHUB_REPOSITORY_ID": "0",
            "GITHUB_REPOSITORY_OWNER_ID": "0", "GITHUB_WORKFLOW_REF": "untrusted",
            "HISTORY_IMPORT_APPROVAL": "approval", "PLAN_COST_EVIDENCE": "cost",
            "PLAN_SAFETY_EVIDENCE": "safety"}.items():
            with self.subTest(key=key):
                result = self.route(**{key: value})
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.outputs, "")

    def test_existing_environment_trust_and_same_sha_actions_are_preserved(self):
        self.assertIn("needs: preflight", self.reader)
        self.assertIn("name: terraform-dev-plan", self.reader)
        self.assertIn("inputs.mode == 'current-state-read' && inputs.target == 'dev'", self.reader)
        self.assertIn("ref: ${{ github.sha }}", self.reader)
        self.assertIn("persist-credentials: false", self.reader)
        self.assertIn("secrets.AWS_TERRAFORM_BACKEND_ROLE_ID", self.reader)
        self.assertIn("--phase identity", self.reader)
        ordered = ["Mask private identity", "Checkout trusted", "Validate bounded read",
                   "Configure existing AWS", "Verify same-SHA", "Configure existing GCP",
                   "Read exact State", "Cleanup private"]
        self.assertEqual([self.reader.index(s) for s in ordered], sorted(self.reader.index(s) for s in ordered))

    def test_read_job_cannot_initialize_terraform_mutate_locks_or_publish_private_artifacts(self):
        for forbidden in ("setup-terraform", "terraform -chdir", "terraform init", "terraform plan",
                          "terraform apply", "backend.hcl", "upload-artifact", "actions/cache",
                          "GCP_RUNTIME_OWNERSHIP_PACKET_JSON", "force-unlock", "continue-on-error"):
            self.assertNotIn(forbidden, self.reader)
        plan = self.text.split("  plan:\n", 1)[1].split("  current_state_read:\n", 1)[0]
        self.assertIn("inputs.mode != 'current-state-read'", plan)
        apply_condition = self.text.split("  apply:\n", 1)[1].split("    needs:", 1)[0]
        self.assertNotIn("current-state-read", apply_condition)
        cleanup = self.reader.split("      - name: Cleanup private", 1)[1]
        self.assertIn("if: always()", cleanup)
        self.assertIn("--phase cleanup", cleanup)
        self.assertIn('rm -f "${GITHUB_WORKSPACE}"/gha-creds-*.json', cleanup)
        self.assertIn('exit "${cleanup_status}"', cleanup)


if __name__ == "__main__":
    unittest.main()
