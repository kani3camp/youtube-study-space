"""CI orchestration must preserve the Hosting credential and artifact boundary."""
import unittest
from test_ci_workflows import ROOT, load


class HostingWorkflowTest(unittest.TestCase):
    def test_only_apply_job_has_oidc_and_build_must_finish_first(self):
        workflow = load(ROOT / '.github/workflows/mypage-hosting-authenticated.yml')
        self.assertEqual(set(workflow['on']), {'workflow_call'})
        self.assertEqual(workflow['permissions'], {})
        self.assertFalse(workflow['concurrency']['cancel-in-progress'])
        jobs = workflow['jobs']
        self.assertEqual(set(jobs), {'build', 'apply', 'browser'})
        self.assertEqual(jobs['build']['permissions'], {'contents': 'read'})
        self.assertEqual(jobs['browser']['permissions'], {'contents': 'read'})
        self.assertEqual(jobs['apply']['permissions'], {'contents': 'read', 'id-token': 'write'})
        self.assertEqual(jobs['apply']['needs'], 'build')
        self.assertEqual(jobs['browser']['needs'], 'apply')
        for name in ('build', 'apply'):
            self.assertEqual(jobs[name]['environment'], 'mypage-dev-hosting')
        for job in jobs.values():
            self.assertGreater(job['timeout-minutes'], 0)
            for step in job['steps']:
                self.assertNotIn('continue-on-error', step)
                if 'uses' in step:
                    self.assertRegex(step['uses'], r'@[a-f0-9]{40}$')
        auth = next(s for s in jobs['apply']['steps'] if s.get('id') == 'auth')
        self.assertFalse(auth['with']['create_credentials_file'])
        self.assertFalse(auth['with']['export_environment_variables'])
        self.assertEqual(auth['with']['project_id'], 'test-youtube-study-space')
        publish = next(s for s in jobs['apply']['steps'] if s.get('run', '').endswith(' deploy'))
        self.assertEqual(publish['env']['MYPAGE_HOSTING_PACKAGE_DIGEST'], '${{ needs.build.outputs.digest }}')

    def test_existing_operations_cannot_run_alongside_hosting(self):
        caller = load(ROOT / '.github/workflows/ci.yml')
        for name in ('gcp-terraform-authenticated', 'mypage-foundation-authenticated', 'gcp-user-activity-schema-audit'):
            self.assertIn('inputs.mypage_hosting != true', caller['jobs'][name]['if'])
        hosting = caller['jobs']['mypage-hosting-authenticated']
        self.assertEqual(hosting['needs'], ['repository', 'frontend'])
        self.assertNotIn('secrets', hosting)
        self.assertEqual(hosting['with']['other_operation'], '${{ inputs.terraform_authenticated == true || inputs.mypage_foundation == true || inputs.gcp_user_activity_schema_audit == true }}')
