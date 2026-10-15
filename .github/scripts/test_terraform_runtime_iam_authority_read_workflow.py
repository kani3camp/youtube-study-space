#!/usr/bin/env python3
"""Separate authority step cannot bypass trusted metadata admission or cleanup."""
from datetime import datetime, timedelta
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]


class AuthorityWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.workflow = (ROOT / ".github/workflows/gcp-terraform-authenticated.yml").read_text()
        cls.caller = (ROOT / ".github/workflows/ci.yml").read_text()
        cls.jobs = {identity: re.split(r"\n  [a-z_]+:\n", cls.workflow.split("  runtime_metadata_read_" + identity + ":\n", 1)[1], maxsplit=1)[0]
                    for identity in ("plan", "apply")}

    def test_independent_step_requires_new_gate_auth_success_and_no_cancellation(self):
        condition = "if: ${{ always() && !cancelled() && env.DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED == 'true' && steps.runtime_gcp_auth.outcome == 'success' }}"
        for identity, job in self.jobs.items():
            self.assertIn(condition, job)
            self.assertIn("AUTHORITY_AUTH_OUTCOME: ${{ steps.runtime_gcp_auth.outcome }}", job)
            self.assertIn("AUTHORITY_JOB_STATUS: ${{ job.status }}", job)
            self.assertIn("name: terraform-dev-" + identity, job)
            names = ["Verify exact Environment", "Configure existing GCP", "Observe six exact runtime", "Observe IAM authority", "Verify private runtime read cleanup"]
            positions = [job.index(name) for name in names]
            self.assertEqual(positions, sorted(positions))
            cleanup = job.split("      - name: Verify private runtime read cleanup\n", 1)[1]
            self.assertIn("if: always()", cleanup)
            self.assertNotIn("continue-on-error", job)

    def test_authority_gate_stays_separate_from_existing_execution_gates(self):
        value = re.search(r'^  DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED: "(true|false)"$', self.workflow, re.M)[1]
        if value == "true":
            self.assertIn('DEV_RUNTIME_METADATA_READ_ENABLED: "true"', self.workflow)
            issued = re.search(r'^  DEV_RUNTIME_METADATA_READ_ISSUED_UTC: "([^"]+)"$', self.workflow, re.M)[1]
            expires = re.search(r'^  DEV_RUNTIME_METADATA_READ_EXPIRES_UTC: "([^"]+)"$', self.workflow, re.M)[1]
            start, end = (datetime.strptime(item, "%Y-%m-%dT%H:%M:%SZ") for item in (issued, expires))
            self.assertLess(start, end)
            self.assertLessEqual(end - start, timedelta(hours=8))
            for gate in ("DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY", "DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED"):
                self.assertIn(gate + ': "false"', self.workflow)
        # A separate ownership-plan window does not activate authority observation.
        for gate in ("DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED", "PROD_AUTHENTICATED_TERRAFORM_ENABLED"):
            self.assertIn(gate + ': "false"', self.workflow)

    def test_old_metadata_and_negative_steps_are_not_replaced_or_made_successful(self):
        for job in self.jobs.values():
            self.assertEqual(job.count("terraform_runtime_metadata_read.py --phase read"), 1)
            self.assertEqual(job.count("terraform_runtime_iam_authority_read.py"), 1)
            self.assertEqual(job.count("terraform_runtime_metadata_read.py --phase cleanup"), 1)
        for name in ("plan", "apply", "current_state_read", "security-probe"):
            part = self.workflow.split("  " + name + ":\n", 1)[1]
            part = re.split(r"\n  [a-z_-]+:\n", part, maxsplit=1)[0]
            self.assertNotIn("terraform_runtime_iam_authority_read.py", part)
        self.assertEqual(self.workflow.count("terraform_runtime_iam_authority_read.py"), 2)
        self.assertIn("test_terraform_runtime_iam_authority_read.py", self.caller)
        self.assertIn("test_terraform_runtime_iam_authority_read_workflow.py", self.caller)
        self.assertNotIn("secrets: inherit", self.caller)


if __name__ == "__main__":
    unittest.main()
