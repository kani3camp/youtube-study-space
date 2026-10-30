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
            'OPERATION': 'gcp-plan', 'ENVIRONMENT': 'development', 'REPOSITORY_RESULT': 'success',
            'CD_EVENT': 'workflow_dispatch', 'CD_REF': 'refs/heads/integration/mypage-canon-20261006',
            'CD_ACTOR_ID': '54093651',
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

    def test_push_requires_owner_integration_branch_and_selected_quality_checks(self):
        normal = {'OPERATION': 'mypage-auto-cd', 'ENVIRONMENT': 'development',
                  'CD_EVENT': 'push', 'CD_REF': 'refs/heads/integration/mypage-canon-20261006',
                  'CD_ACTOR_ID': '54093651'}
        self.assertEqual(self.run_selector(**normal), 0)
        # Unaffected CI domains skip on system-only commits, and this must not
        # prevent delivery of a fully verified system image.
        unaffected = {key: 'skipped' for key in RESULTS if key != 'SYSTEM_RESULT'}
        self.assertEqual(self.run_selector(**(normal | unaffected)), 0)
        for extra in ({'CD_EVENT': 'workflow_dispatch'}, {'CD_EVENT': 'pull_request'},
                      {'CD_REF': 'refs/heads/main'}, {'CD_ACTOR_ID': '123'},
                      {'ENVIRONMENT': 'production'}, {'SYSTEM_RESULT': 'skipped'},
                      {'FRONTEND_RESULT': 'failure'}, {'GCP_TERRAFORM_VALIDATE_RESULT': 'cancelled'},
                      {'REPOSITORY_RESULT': 'skipped'}):
            with self.subTest(extra=extra):
                self.assertNotEqual(self.run_selector(**(normal | unaffected | extra)), 0)

    def test_direct_build_digest_is_passed_to_isolated_backend_deploy(self):
        automatic = self.workflow['jobs']['mypage-auto-deploy']
        self.assertEqual(automatic['needs'], ['validate-selection', 'mypage-image-build'])
        self.assertEqual(automatic['uses'], './.github/workflows/mypage-backend-authenticated.yml')
        self.assertIn("outputs.build == 'true'", automatic['if'])
        self.assertIn('mypage-image-build.outputs.image_digest',
                      automatic['with']['image_digest'])
        self.assertNotIn('secrets', automatic)
        self.assertNotIn('mypage-release-pr', self.workflow['jobs'])
        build = self.workflow['jobs']['mypage-image-build']
        self.assertIn("outputs.build == 'true'", build['if'])
        self.assertNotIn('contents: write', str(self.workflow))

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
