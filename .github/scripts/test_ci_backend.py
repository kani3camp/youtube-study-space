"""Backend release must include serving verification and bounded State readback."""
import unittest
from test_ci_workflows import ROOT, load, FOUNDATION_SECRETS
import test_ci_results as results


class BackendWorkflowTest(unittest.TestCase):
    def test_release_requires_system_tests_and_propagates_failures(self):
        caller = load(ROOT / '.github/workflows/ci.yml')
        call = caller['jobs']['mypage-backend-authenticated']
        self.assertEqual(call['needs'], ['repository', 'system'])
        self.assertEqual(call['secrets'], {key: '${{ secrets.' + key + ' }}' for key in FOUNDATION_SECRETS})
        for name in ('gcp-terraform-authenticated', 'mypage-foundation-authenticated',
                     'gcp-user-activity-schema-audit', 'mypage-hosting-authenticated'):
            self.assertIn('inputs.mypage_backend != true', caller['jobs'][name]['if'])
        needs = results.GateResultsTest().fixture()
        for status in ('skipped', 'cancelled', 'failure'):
            needs['mypage-backend-authenticated']['result'] = status
            with self.assertRaises(ValueError):
                results.gate.validate(needs, 'false', 'false', 'false', 'true')
        needs['mypage-backend-authenticated']['result'] = 'success'
        results.gate.validate(needs, 'false', 'false', 'false', 'true')
        for invalid in ('', 'TRUE', True):
            with self.assertRaises(ValueError):
                results.gate.validate(needs, 'false', 'false', 'false', invalid)

    def test_isolated_deployer_and_sequential_state_proof(self):
        workflow = load(ROOT / '.github/workflows/mypage-backend-authenticated.yml')
        self.assertEqual(set(workflow['on']), {'workflow_call'})
        hosting = load(ROOT / '.github/workflows/mypage-hosting-authenticated.yml')
        self.assertEqual(workflow['concurrency'], hosting['concurrency'])
        self.assertEqual(set(workflow['jobs']), {'deploy', 'state', 'receipt'})
        deploy = workflow['jobs']['deploy']
        self.assertEqual(deploy['environment'], 'mypage-dev-backend')
        auth = next(s for s in deploy['steps'] if s.get('id') == 'auth')
        self.assertTrue(auth['with']['create_credentials_file'])
        self.assertTrue(auth['with']['export_environment_variables'])
        self.assertTrue(auth['with']['cleanup_credentials'])
        self.assertEqual(auth['with']['service_account'],
                         'mypage-backend-deploy@test-youtube-study-space.iam.gserviceaccount.com')
        names = [step.get('name') for step in deploy['steps']]
        self.assertLess(names.index('Require initialized maintenance control before mutating the serving revision'),
                        names.index('Conditionally update only the source-pinned image and verify serving'))
        bootstrap = next(step for step in deploy['steps'] if step.get('name', '').startswith('Require initialized maintenance control'))
        self.assertIn('gcloud run jobs execute mypage-maintenance-bootstrap-check', bootstrap['run'])
        self.assertNotIn('--args', bootstrap['run'])
        self.assertEqual(workflow['jobs']['state']['needs'], 'deploy')
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
