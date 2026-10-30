"""クラウド認証情報の取得前に、実際の操作選択処理を実行する。"""
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
            'OPERATION': 'gcp-user-activity-schema-audit', 'ENVIRONMENT': 'development', 'REPOSITORY_RESULT': 'success',
        }

    def run_selector(self, **changes):
        return subprocess.run(['bash', '-c', self.command], env=self.env | changes,
                              capture_output=True).returncode

    def test_only_the_schema_audit_remains_and_keeps_its_boundaries(self):
        self.assertEqual(self.run_selector(), 0)
        self.assertEqual(self.run_selector(SYSTEM_RESULT='skipped'), 0)
        for changes in ({'OPERATION': 'validate'}, {'ENVIRONMENT': 'staging'},
                        {'ENVIRONMENT': 'production'}, {'GCP_TERRAFORM_VALIDATE_RESULT': 'skipped'},
                        {'REPOSITORY_RESULT': 'failure'}, {'TOOLS_RESULT': 'failure'},
                        # Terraform and application deploys moved to their own workflows.
                        {'OPERATION': 'gcp-plan'}, {'OPERATION': 'gcp-apply'},
                        {'OPERATION': 'mypage-image-build'}, {'OPERATION': 'mypage-hosting-release'},
                        {'OPERATION': 'mypage-backend-release'}, {'OPERATION': 'mypage-auto-cd'}):
            with self.subTest(changes=changes):
                self.assertNotEqual(self.run_selector(**changes), 0)

    def test_credentialed_calls_cannot_bypass_the_selector(self):
        self.assertEqual(set(self.workflow['on']), {'workflow_call'})
        for name, job in self.workflow['jobs'].items():
            if job.get('permissions', {}).get('id-token') == 'write':
                with self.subTest(job=name):
                    self.assertIn('validate-selection', [job['needs']] if isinstance(job['needs'], str) else job['needs'])
                    self.assertNotIn('continue-on-error', job)

    def test_apply_requires_the_independently_reproduced_plan_commitment(self):
        workflow = load(ROOT / '.github/workflows/mypage-foundation-authenticated.yml')
        apply = workflow['jobs']['foundation_apply']
        step = next(s for s in apply['steps'] if s.get('id') == 'applied')
        self.assertIn("steps.prepared.outputs.commitment == needs.foundation_plan.outputs.commitment", step['if'])
        self.assertIn("steps.prepared.outputs.commitment != ''", step['if'])
