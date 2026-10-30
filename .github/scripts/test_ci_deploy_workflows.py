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

    def test_credentialed_calls_never_start_after_the_run_was_cancelled(self):
        # always() stays true after a cancel; a job that calls a reusable workflow
        # (and so may obtain credentials) must use !cancelled() to tolerate skips.
        for path in sorted(WORKFLOWS.glob('*.yml')):
            for name, job in load(path)['jobs'].items():
                if 'uses' in job:
                    with self.subTest(workflow=path.name, job=name):
                        self.assertNotIn('always()', str(job.get('if', '')))

    def test_image_deploys_are_manual_dev_only_and_build_only_their_own_image(self):
        for name, component, reusable in (
            ('deploy-mypage-api.yml', 'api', 'mypage-backend-authenticated.yml'),
            ('deploy-mypage-maintenance.yml', 'maintenance-operator', 'mypage-operator-authenticated.yml'),
        ):
            with self.subTest(workflow=name):
                workflow = load(WORKFLOWS / name)
                self.assertEqual(triggers(workflow), {'workflow_dispatch'})
                self.assertEqual(workflow['permissions'], {'contents': 'read'})
                inputs = workflow['on']['workflow_dispatch']['inputs']
                self.assertEqual(set(inputs), {'mode', 'image_digest', 'image_source_sha'})
                self.assertEqual(inputs['mode']['options'], ['build-and-deploy', 'deploy-existing'])
                jobs = workflow['jobs']
                build, deploy = jobs['build'], jobs['deploy']
                self.assertEqual(build['uses'], './.github/workflows/mypage-foundation-authenticated.yml')
                self.assertEqual(build['with'], {'target': 'development', 'mode': 'image-build',
                                                 'expected_sha': '${{ github.sha }}',
                                                 'image_component': component})
                self.assertEqual(set(build['secrets']), {'MYPAGE_GCP_WIF_PROVIDER', 'MYPAGE_GCP_SERVICE_ACCOUNT'})
                self.assertEqual(deploy['uses'], './.github/workflows/' + reusable)
                self.assertEqual(set(deploy['needs']), set(jobs) - {'deploy'})
                for need in deploy['needs']:
                    if need != 'build':
                        self.assertIn('needs.' + need + ".result == 'success'", deploy['if'])
                self.assertNotIn('secrets', deploy)
                self.assertEqual(deploy['with']['expected_sha'], '${{ github.sha }}')

    def test_deploys_validate_their_domain_in_the_same_run_before_credentials(self):
        for name in ('deploy-mypage-api.yml', 'deploy-mypage-maintenance.yml'):
            with self.subTest(workflow=name):
                jobs = load(WORKFLOWS / name)['jobs']
                self.assertEqual(jobs['validate']['uses'], './.github/workflows/ci-system.yml')
                self.assertEqual(jobs['validate']['with'], {'system': True, 'firestore_integration': True})
                self.assertEqual(jobs['build']['needs'], ['select', 'validate'])
                self.assertIn('validate', jobs['deploy']['needs'])
        frontend = load(WORKFLOWS / 'mypage-hosting-authenticated.yml')['jobs']
        build = '\n'.join(step.get('run', '') for step in frontend['build']['steps'])
        for command in ('pnpm audit --audit-level=high', 'pnpm check', 'pnpm typecheck', 'pnpm test'):
            self.assertIn(command, build)
        self.assertEqual(frontend['apply']['needs'], 'build')

    def test_frontend_deploy_is_manual_dev_only_and_validates_mypage_first(self):
        workflow = load(WORKFLOWS / 'deploy-mypage-frontend.yml')
        self.assertEqual(triggers(workflow), {'workflow_dispatch'})
        self.assertIsNone(workflow['on']['workflow_dispatch'])
        self.assertEqual(workflow['permissions'], {'contents': 'read'})
        jobs = workflow['jobs']
        self.assertEqual(set(jobs), {'validate', 'hosting'})
        self.assertEqual(jobs['validate']['uses'], './.github/workflows/ci-frontend.yml')
        self.assertEqual(jobs['validate']['with'], {'youtube_monitor': False, 'docs_site': False, 'mypage': True})
        self.assertEqual(jobs['hosting']['needs'], 'validate')
        self.assertNotIn('if', jobs['hosting'])
        self.assertEqual(jobs['hosting']['uses'], './.github/workflows/mypage-hosting-authenticated.yml')
        self.assertEqual(jobs['hosting']['with'], {'expected_sha': '${{ github.sha }}'})
        self.assertNotIn('secrets', jobs['hosting'])

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
