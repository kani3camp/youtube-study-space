#!/usr/bin/env python3
"""Execute exact-runtime route boundaries and verify independent protected jobs."""
from datetime import datetime, timedelta, timezone
from pathlib import Path
import unittest

import test_terraform_authenticated_workflow as workflow_contract


class RuntimeReadWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        workflow_contract.TerraformAuthenticatedWorkflowTest.setUpClass()
        cls.harness = workflow_contract.TerraformAuthenticatedWorkflowTest()
        cls.text = cls.harness.text
        cls.caller = cls.harness.caller
        import re
        cls.jobs = {identity: re.split(r"\n  [a-z_]+:\n", cls.text.split("  runtime_metadata_read_" + identity + ":\n", 1)[1], maxsplit=1)[0]
                    for identity in ("plan", "apply")}

    def route(self, **overrides):
        now = datetime.now(timezone.utc).replace(microsecond=0)
        return self.harness.run_preflight(**{
            "MODE": "runtime-metadata-read", "TARGET": "dev",
            "DEV_RUNTIME_METADATA_READ_ENABLED": "true", "GITHUB_RUN_ATTEMPT": "1",
            "HISTORY_POST_NOOP": "false", "CURRENT_STATE_ATTESTATION": "false", "OWNERSHIP_WAVE": "none",
            "OWNERSHIP_ROOT_BASIS": "original-history-import", "HISTORY_IMPORT_APPROVAL": "",
            "PLAN_COST_EVIDENCE": "", "PLAN_SAFETY_EVIDENCE": "",
            "DEV_RUNTIME_METADATA_READ_ISSUED_UTC": (now - timedelta(seconds=1)).strftime("%Y-%m-%dT%H:%M:%SZ"),
            "DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": (now + timedelta(hours=7)).strftime("%Y-%m-%dT%H:%M:%SZ"),
            **overrides})

    def test_enabled_separate_route_emits_no_backend_or_ownership_inputs(self):
        result = self.route()
        self.assertEqual(result.returncode, 0)
        self.assertEqual(set(result.outputs.splitlines()), {"project_id=test-youtube-study-space", "ownership_wave=none"})
        # Runtime observations remain independent of Terraform/current-state activation.
        self.assertEqual(self.route(DEV_AUTHENTICATED_TERRAFORM_ENABLED="false", DEV_CURRENT_STATE_READ_ENABLED="false").returncode, 0)

    def test_closed_gate_mixed_modes_and_untrusted_context_cannot_reach_credentials(self):
        for key, value in {
            "DEV_RUNTIME_METADATA_READ_ENABLED": "false", "TARGET": "prod", "GITHUB_RUN_ATTEMPT": "2",
            "HISTORY_POST_NOOP": "true", "CURRENT_STATE_ATTESTATION": "true", "OWNERSHIP_WAVE": "pool",
            "OWNERSHIP_ROOT_BASIS": "current-state-bootstrap", "HISTORY_IMPORT_APPROVAL": "{}",
            "PLAN_COST_EVIDENCE": "{}", "PLAN_SAFETY_EVIDENCE": "{}", "GITHUB_EVENT_NAME": "pull_request",
            "GITHUB_REPOSITORY": "dummy/repo", "GITHUB_REPOSITORY_ID": "0", "GITHUB_REPOSITORY_OWNER_ID": "0",
            "GITHUB_WORKFLOW_REF": "untrusted", "GITHUB_REF": "refs/heads/dev", "GITHUB_SHA": "a" * 39,
        }.items():
            with self.subTest(key=key):
                result = self.route(**{key: value})
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.outputs, "")

    def test_empty_invalid_expired_future_or_overlong_source_windows_stop(self):
        now = datetime.now(timezone.utc)
        for override in ({"DEV_RUNTIME_METADATA_READ_ISSUED_UTC": ""},
                         {"DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": ""},
                         {"DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": "DUMMY_PRIVATE_INVALID"},
                         {"DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": "2000-01-01T00:00:00Z"},
                         {"DEV_RUNTIME_METADATA_READ_ISSUED_UTC": "2999-01-01T00:00:00Z"},
                         {"DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": (now + timedelta(hours=9)).strftime("%Y-%m-%dT%H:%M:%SZ")}):
            with self.subTest(override=override):
                result = self.route(**override)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.outputs, "")

    def test_source_activation_is_separate_boolean_and_has_at_most_eight_hours(self):
        import re
        values = dict(re.findall(r'(?m)^  (DEV_RUNTIME_METADATA_READ_[A-Z_]+): "([^"\n]*)"$', self.text))
        self.assertIn(values["DEV_RUNTIME_METADATA_READ_ENABLED"], {"true", "false"})
        if values["DEV_RUNTIME_METADATA_READ_ENABLED"] == "true":
            issued = datetime.strptime(values["DEV_RUNTIME_METADATA_READ_ISSUED_UTC"], "%Y-%m-%dT%H:%M:%SZ")
            expires = datetime.strptime(values["DEV_RUNTIME_METADATA_READ_EXPIRES_UTC"], "%Y-%m-%dT%H:%M:%SZ")
            self.assertLess(issued, expires)
            self.assertLessEqual(expires - issued, timedelta(hours=8))

    def test_distinct_explicit_environment_and_token_for_each_exact_identity(self):
        for identity, job in self.jobs.items():
            with self.subTest(identity=identity):
                self.assertIn("needs: preflight", job)
                self.assertIn("inputs.mode == 'runtime-metadata-read' && inputs.target == 'dev'", job)
                self.assertIn("name: terraform-dev-" + identity, job)
                self.assertIn("RUNTIME_READ_IDENTITY: " + identity, job)
                self.assertIn("GCP_SMOKE_SERVICE_ACCOUNT: ${{ secrets.GCP_TERRAFORM_SERVICE_ACCOUNT }}", job)
                self.assertIn("RUNTIME_READ_WIF_PROVIDER: ${{ secrets.GCP_TERRAFORM_WIF_PROVIDER }}", job)
                self.assertIn("ref: ${{ github.sha }}", job)
                self.assertIn("persist-credentials: false", job)
                self.assertIn("steps.runtime_gcp_auth.outputs.access_token", job)
                self.assertIn("create_credentials_file: false", job)
                self.assertIn("export_environment_variables: false", job)
                self.assertIn("if: always()", job)
                self.assertIn("--phase cleanup", job)
                sequence = ["Mask existing", "Checkout exact", "Validate bounded", "Verify exact Environment", "Configure existing GCP", "Observe six", "Verify private"]
                positions = [job.index(part) for part in sequence]
                self.assertEqual(positions, sorted(positions))

    def test_metadata_jobs_cannot_use_terraform_state_artifacts_or_normal_apply(self):
        for job in self.jobs.values():
            for forbidden in ("setup-terraform", "terraform init", "terraform plan", "terraform apply", "terraform -chdir",
                              "upload-artifact", "actions/cache", "backend.hcl", "STATE_BUCKET", "AWS_TERRAFORM",
                              "configure-aws-credentials", "GCP_RUNTIME_OWNERSHIP_PACKET_JSON", "force-unlock", "continue-on-error"):
                self.assertNotIn(forbidden, job)
        plan = self.text.split("  plan:\n", 1)[1].split("  current_state_read:\n", 1)[0]
        self.assertIn("inputs.mode != 'runtime-metadata-read'", plan)
        apply = self.text.split("  apply:\n", 1)[1].split("    needs:", 1)[0]
        self.assertNotIn("runtime-metadata-read", apply)
        self.assertEqual(self.text.count("workflow_call:"), 1)
        self.assertNotIn("workflow_dispatch:", self.text)
        self.assertIn("          - runtime-metadata-read\n", self.caller)
        self.assertIn("test_terraform_runtime_metadata_read_workflow.py", self.caller)
        self.assertNotIn("secrets: inherit", self.caller)


if __name__ == "__main__":
    unittest.main()
