"""Backend release must include serving verification and bounded State readback."""
import unittest
from test_ci_workflows import ROOT, load, FOUNDATION_SECRETS


class BackendWorkflowTest(unittest.TestCase):
    def test_release_is_selected_only_through_operations_boundary(self):
        operations = load(ROOT / '.github/workflows/ci-operations.yml')
        call = operations['jobs']['mypage-backend-authenticated']
        self.assertEqual(call['needs'], 'validate-selection')
        self.assertEqual(call['if'], '${{ inputs.mypage_backend }}')
        self.assertEqual(call['permissions'], {'contents': 'read', 'id-token': 'write'})
        self.assertEqual(call['uses'], './.github/workflows/mypage-backend-authenticated.yml')
        self.assertEqual(call['secrets'], {
            key: '${{ secrets.' + key + ' }}' for key in FOUNDATION_SECRETS
        })
        self.assertEqual(call['with'], {
            'expected_sha': '${{ inputs.mypage_backend_expected_sha }}',
            'other_operation': False,
        })

    def test_isolated_deployer_and_sequential_state_proof(self):
        workflow = load(ROOT / '.github/workflows/mypage-backend-authenticated.yml')
        self.assertEqual(set(workflow['on']), {'workflow_call'})
        hosting = load(ROOT / '.github/workflows/mypage-hosting-authenticated.yml')
        self.assertEqual(workflow['concurrency'], hosting['concurrency'])
        self.assertEqual(set(workflow['jobs']), {'deploy', 'state', 'receipt'})
        deploy = workflow['jobs']['deploy']
        self.assertEqual(deploy['environment'], 'mypage-dev-backend')
        auth = next(s for s in deploy['steps'] if s.get('id') == 'auth')
        self.assertFalse(auth['with']['create_credentials_file'])
        self.assertFalse(auth['with']['export_environment_variables'])
        self.assertEqual(
            auth['with']['service_account'],
            'mypage-backend-deploy@test-youtube-study-space.iam.gserviceaccount.com',
        )
        self.assertEqual(workflow['jobs']['state']['needs'], 'deploy')
        self.assertEqual(workflow['jobs']['state']['with']['target'], 'development')
        self.assertEqual(workflow['jobs']['state']['with']['mode'], 'runtime-public-apply')
        self.assertEqual(workflow['jobs']['receipt']['needs'], 'state')
        self.assertEqual(workflow['jobs']['receipt']['permissions'], {'contents': 'read'})
        self.assertEqual(deploy['steps'][-1]['if'], '${{ always() }}')
        self.assertNotIn('secrets.', str(deploy))
        for job in workflow['jobs'].values():
            for step in job.get('steps', []):
                self.assertNotIn('continue-on-error', step)
                if 'uses' in step:
                    self.assertRegex(step['uses'], r'@[a-f0-9]{40}$')
