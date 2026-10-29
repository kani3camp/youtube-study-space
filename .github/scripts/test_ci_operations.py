"""Execute the real operation selector before any cloud credential is acquired."""
import os
import subprocess
import unittest

from test_ci_workflows import ROOT, load

RESULTS = ('FRONTEND_RESULT', 'SYSTEM_RESULT', 'TOOLS_RESULT',
           'INFRA_RESULT', 'GCP_TERRAFORM_VALIDATE_RESULT')


class OperationSelectionTest(unittest.TestCase):
    def setUp(self):
        self.workflow = load(ROOT / '.github/workflows/ci-operations.yml')
        self.command = self.workflow['jobs']['validate-selection']['steps'][0]['run']
        self.env = dict(os.environ) | dict.fromkeys(RESULTS, 'success') | {
            'OPERATION': 'gcp-plan', 'ENVIRONMENT': 'development', 'REPOSITORY_RESULT': 'success',
        }

    def run_selector(self, **changes):
        return subprocess.run(['bash', '-c', self.command], env=self.env | changes,
                              capture_output=True).returncode

    def test_supported_operations_require_full_validation_before_credentials(self):
        for operation in ('gcp-plan', 'gcp-apply', 'mypage-image-build',
                          'mypage-hosting-release', 'mypage-backend-release'):
            with self.subTest(operation=operation):
                self.assertEqual(self.run_selector(OPERATION=operation), 0)
                self.assertNotEqual(self.run_selector(OPERATION=operation, SYSTEM_RESULT='skipped'), 0)
        for changes in ({'OPERATION': 'validate'}, {'ENVIRONMENT': 'staging'},
                        {'REPOSITORY_RESULT': 'failure'}, {'TOOLS_RESULT': 'failure'}):
            with self.subTest(changes=changes):
                self.assertNotEqual(self.run_selector(**changes), 0)

    def test_production_and_read_only_audit_keep_their_distinct_boundaries(self):
        self.assertEqual(self.run_selector(ENVIRONMENT='production', OPERATION='gcp-apply'), 0)
        self.assertNotEqual(self.run_selector(ENVIRONMENT='production', OPERATION='mypage-hosting-release'), 0)
        self.assertNotEqual(self.run_selector(ENVIRONMENT='production', OPERATION='mypage-backend-release'), 0)
        audit = {'OPERATION': 'gcp-user-activity-schema-audit', 'SYSTEM_RESULT': 'skipped'}
        self.assertEqual(self.run_selector(**audit), 0)
        self.assertNotEqual(self.run_selector(**audit, ENVIRONMENT='production'), 0)
        self.assertNotEqual(self.run_selector(**audit, GCP_TERRAFORM_VALIDATE_RESULT='skipped'), 0)

    def test_credentialed_calls_cannot_bypass_the_selector(self):
        self.assertEqual(set(self.workflow['on']), {'workflow_call'})
        for name, job in self.workflow['jobs'].items():
            if job.get('permissions', {}).get('id-token') == 'write':
                with self.subTest(job=name):
                    self.assertEqual(job['needs'], 'validate-selection')
                    self.assertNotIn('continue-on-error', job)

    def test_apply_requires_the_independently_reproduced_plan_commitment(self):
        workflow = load(ROOT / '.github/workflows/mypage-foundation-authenticated.yml')
        apply = workflow['jobs']['foundation_apply']
        step = next(s for s in apply['steps'] if s.get('id') == 'applied')
        self.assertIn("steps.prepared.outputs.commitment == needs.foundation_plan.outputs.commitment", step['if'])
        self.assertIn("steps.prepared.outputs.commitment != ''", step['if'])
