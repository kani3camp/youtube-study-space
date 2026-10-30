"""ユースケース単位のDeploy workflowが、認証境界と配備対象を取り違えないことを検証する。"""
import unittest

from test_ci_workflows import ROOT, load

WORKFLOWS = ROOT / '.github/workflows'
# 最上位workflowのjobで直接認証している既知の箇所。移行計画#7aでreusable化して空にする。
DIRECT_AUTH_EXCEPTIONS = {('mypage-maintenance.yml', 'transition')}


def triggers(workflow):
    return set(workflow.get('on') or {})


class DeployWorkflowTest(unittest.TestCase):
    def test_credentials_are_obtained_only_inside_reusable_workflows(self):
        for path in sorted(WORKFLOWS.glob('*.yml')):
            workflow = load(path)
            if triggers(workflow) == {'workflow_call'}:
                continue
            self.assertNotEqual(workflow.get('permissions', {}).get('id-token'), 'write', path.name)
            for name, job in workflow['jobs'].items():
                if job.get('permissions', {}).get('id-token') != 'write' or 'uses' in job:
                    continue
                with self.subTest(workflow=path.name, job=name):
                    self.assertIn((path.name, name), DIRECT_AUTH_EXCEPTIONS)

    def test_operator_deploy_is_manual_dev_only_and_builds_only_the_operator_image(self):
        workflow = load(WORKFLOWS / 'deploy-mypage-maintenance.yml')
        self.assertEqual(triggers(workflow), {'workflow_dispatch'})
        self.assertEqual(workflow['permissions'], {'contents': 'read'})
        inputs = workflow['on']['workflow_dispatch']['inputs']
        self.assertEqual(set(inputs), {'mode', 'image_digest', 'image_source_sha'})
        self.assertEqual(inputs['mode']['options'], ['build-and-deploy', 'deploy-existing'])
        self.assertNotIn('environment', inputs)
        jobs = workflow['jobs']
        build, deploy = jobs['build'], jobs['deploy']
        self.assertEqual(build['uses'], './.github/workflows/mypage-foundation-authenticated.yml')
        self.assertEqual(build['with'], {'target': 'development', 'mode': 'image-build',
                                         'expected_sha': '${{ github.sha }}',
                                         'image_component': 'maintenance-operator'})
        self.assertEqual(set(build['secrets']), {'MYPAGE_GCP_WIF_PROVIDER', 'MYPAGE_GCP_SERVICE_ACCOUNT'})
        self.assertEqual(deploy['uses'], './.github/workflows/mypage-operator-authenticated.yml')
        self.assertEqual(deploy['needs'], ['select', 'validate', 'build'])
        self.assertIn("needs.select.result == 'success'", deploy['if'])
        self.assertIn("needs.validate.result == 'success'", deploy['if'])
        # The deployed domain is validated in the same run before any credentials.
        self.assertEqual(jobs['validate']['uses'], './.github/workflows/ci-system.yml')
        self.assertEqual(jobs['validate']['with'], {'system': True, 'firestore_integration': True})
        self.assertEqual(build['needs'], ['select', 'validate'])
        self.assertNotIn('secrets', deploy)
        self.assertEqual(deploy['with']['expected_sha'], '${{ github.sha }}')

    def test_operator_release_authenticates_after_the_contract_and_joins_the_cloud_run_lock(self):
        workflow = load(WORKFLOWS / 'mypage-operator-authenticated.yml')
        self.assertEqual(triggers(workflow), {'workflow_call'})
        self.assertEqual(workflow['concurrency'], {'group': 'mypage-dev-runtime-change',
                                                   'cancel-in-progress': False})
        job = workflow['jobs']['deploy']
        self.assertEqual(job['environment'], 'mypage-dev-operator')
        steps = job['steps']
        names = [step.get('uses', step.get('run', '')) for step in steps]
        checkout = next(i for i, step in enumerate(steps) if step.get('uses', '').startswith('actions/checkout@'))
        context = names.index('python3 infra/mypage-platform/scripts/operator_release.py context')
        auth = next(i for i, step in enumerate(steps) if step.get('uses', '').startswith('google-github-actions/auth@'))
        self.assertLess(checkout, context)
        self.assertLess(context, auth)
        self.assertIs(steps[checkout]['with']['persist-credentials'], False)
        self.assertEqual(steps[auth]['with']['service_account'],
                         'mypage-operator-deploy@test-youtube-study-space.iam.gserviceaccount.com')

    def test_release_steps_receive_every_runtime_input_their_script_reads(self):
        # The release scripts verify the branch HEAD through the GitHub API.
        for name, script in (('mypage-backend-authenticated.yml', 'backend_release.py deploy'),
                             ('mypage-operator-authenticated.yml', 'operator_release.py deploy'),
                             ('mypage-hosting-authenticated.yml', 'hosting_release.py deploy')):
            jobs = load(WORKFLOWS / name)['jobs']
            steps = [step for job in jobs.values() for step in job.get('steps', [])
                     if step.get('run', '').endswith(script)]
            with self.subTest(workflow=name):
                self.assertEqual(len(steps), 1)
                self.assertEqual(steps[0]['env'].get('GITHUB_TOKEN'), '${{ github.token }}')

    def test_api_release_and_operator_release_use_distinct_identities(self):
        api = load(WORKFLOWS / 'mypage-backend-authenticated.yml')['jobs']['deploy']
        operator = load(WORKFLOWS / 'mypage-operator-authenticated.yml')['jobs']['deploy']
        def account(job):
            return next(step['with']['service_account'] for step in job['steps']
                        if step.get('uses', '').startswith('google-github-actions/auth@'))
        self.assertNotEqual(account(api), account(operator))
        self.assertNotEqual(api['environment'], operator['environment'])


if __name__ == '__main__':
    unittest.main()
