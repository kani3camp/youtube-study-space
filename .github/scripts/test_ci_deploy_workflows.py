"""配備用GitHub Actionsワークフローの書き方が、認証情報の扱いと配備対象を取り違えていないかをテストする。

目的: クラウド認証情報の取得がワークフローの外側に出る、キャンセル後に認証が始まる、DEVと本番の配備が互いのブランチ・Environment・本人情報・ロックを取り違える、といった書き間違いを防ぐ。
使われる場面: CI Repository ワークフローの contracts 検査(.github/ci-validation/validate.sh が `unittest discover -p 'test_ci_*.py'` で実行)。ローカルでも同じ方法で実行できる。
失敗したとき: CI Repository が失敗し、PRをマージできない。
なくした場合: 配備ワークフロー(mypage-*-authenticated.yml など)の認証・配備対象の書き間違いが自動では見つからなくなる。認証まわりは dev にある test_ci_workflows.py や test_ci_security.py も見ており、一部重なる。
"""
import os
import subprocess
import sys
import unittest

from test_ci_workflows import ROOT, load

WORKFLOWS = ROOT / '.github/workflows'
SCRIPTS = ROOT / 'infra/mypage-platform/scripts'
# 各配備単位の (最上位 workflow, reusable, 配備スクリプト, 環境別 GitHub Environment の接尾辞)。
RELEASES = (
    ('deploy-mypage-api.yml', 'mypage-backend-authenticated.yml', 'backend_release.py', 'backend'),
    ('deploy-mypage-maintenance.yml', 'mypage-operator-authenticated.yml', 'operator_release.py', 'operator'),
    ('deploy-mypage-frontend.yml', 'mypage-hosting-authenticated.yml', 'hosting_release.py', 'hosting'),
)
TARGET_REFS = 'development:refs/heads/integration/mypage-canon-20261006|production:refs/heads/main'


def environment_expression(dev, prod):
    return "${{ inputs.environment == 'production' && '" + prod + "' || '" + dev + "' }}"


def auth_step(job):
    return next(step for step in job['steps'] if step.get('uses', '').startswith('google-github-actions/auth@'))


def release_target(script, target):
    """The SA/project that the release script publishes for the auth step of the target."""
    code = ('import sys; sys.path.insert(0, sys.argv[1]); import foundation_contract as c; '
            'print(c.release_account(sys.argv[2]), c.PROJECT, c.REF)')
    env = {key: value for key, value in os.environ.items() if not key.startswith('MYPAGE_')}
    result = subprocess.run([sys.executable, '-c', code, str(SCRIPTS), script],
                            env=env | {'MYPAGE_TARGET': target}, capture_output=True, text=True, check=True)
    return result.stdout.split()
# 最上位workflowのjobで直接認証してよい例外はない（#6でmaintenanceもreusable化した）。
DIRECT_AUTH_EXCEPTIONS = set()


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

    def test_image_deploys_are_manual_and_build_only_their_own_image_for_the_selected_target(self):
        for name, component, reusable, modes in (
            ('deploy-mypage-api.yml', 'api', 'mypage-backend-authenticated.yml',
             ['build-and-deploy', 'deploy-existing', 'build-only']),
            ('deploy-mypage-maintenance.yml', 'maintenance-operator', 'mypage-operator-authenticated.yml',
             ['build-and-deploy', 'deploy-existing']),
        ):
            with self.subTest(workflow=name):
                workflow = load(WORKFLOWS / name)
                self.assertEqual(triggers(workflow), {'workflow_dispatch'})
                self.assertEqual(workflow['permissions'], {'contents': 'read'})
                inputs = workflow['on']['workflow_dispatch']['inputs']
                self.assertEqual(set(inputs), {'environment', 'mode', 'image_digest', 'image_source_sha'})
                self.assertEqual(inputs['environment']['options'], ['development', 'production'])
                self.assertEqual(inputs['environment']['default'], 'development')
                self.assertEqual(inputs['mode']['options'], modes)
                jobs = workflow['jobs']
                build, deploy = jobs['build'], jobs['deploy']
                self.assertEqual(build['uses'], './.github/workflows/mypage-foundation-authenticated.yml')
                self.assertEqual(build['with'], {'target': '${{ inputs.environment }}', 'mode': 'image-build',
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
                self.assertEqual(deploy['with']['environment'], '${{ inputs.environment }}')

    def test_build_only_publishes_an_image_and_never_deploys(self):
        jobs = load(WORKFLOWS / 'deploy-mypage-api.yml')['jobs']
        self.assertIn("inputs.mode == 'build-only'", jobs['build']['if'])
        self.assertIn("inputs.mode != 'build-only'", jobs['deploy']['if'])

    def test_every_deploy_pins_each_target_to_its_own_branch_before_any_reusable_call(self):
        for name, _, _, _ in RELEASES:
            with self.subTest(workflow=name):
                jobs = load(WORKFLOWS / name)['jobs']
                select = '\n'.join(step.get('run', '') for step in jobs['select']['steps'])
                self.assertIn(TARGET_REFS, select)
                self.assertNotIn('uses', jobs['select'])
                for job_name, job in jobs.items():
                    if job_name != 'select':
                        needs = job['needs'] if isinstance(job['needs'], list) else [job['needs']]
                        self.assertIn('select', needs)

    def test_each_target_uses_its_own_environment_lock_and_source_defined_identity(self):
        for _, reusable, script, component in RELEASES:
            with self.subTest(workflow=reusable):
                workflow = load(WORKFLOWS / reusable)
                self.assertEqual(workflow['on']['workflow_call']['inputs']['environment'],
                                 {'required': True, 'type': 'string'})
                lock = ('mypage-dev-hosting', 'mypage-prod-hosting') if component == 'hosting' else \
                    ('mypage-dev-cloud-run', 'mypage-prod-cloud-run')
                self.assertEqual(workflow['concurrency'], {'group': environment_expression(*lock),
                                                           'cancel-in-progress': False})
                jobs = workflow['jobs']
                credentialed = [job for job in jobs.values() if job.get('permissions', {}).get('id-token') == 'write']
                self.assertEqual(len(credentialed), 1)
                job = credentialed[0]
                self.assertEqual(job['environment'], environment_expression(
                    'mypage-dev-' + component, 'mypage-prod-' + component))
                steps = job['steps']
                context = next(i for i, step in enumerate(steps)
                               if step.get('run') == 'python3 infra/mypage-platform/scripts/' + script + ' context')
                auth = steps.index(auth_step(job))
                self.assertLess(context, auth)
                self.assertEqual(steps[context]['id'], 'target')
                self.assertEqual(auth_step(job)['with']['service_account'], '${{ steps.target.outputs.service_account }}')
                self.assertEqual(auth_step(job)['with']['project_id'], '${{ steps.target.outputs.project }}')
                target_env = (workflow.get('env') or {}) | (job.get('env') or {})
                self.assertEqual(target_env['MYPAGE_TARGET'], '${{ inputs.environment }}')
        accounts = {}
        for target, project, ref in (('development', 'test-youtube-study-space', 'refs/heads/integration/mypage-canon-20261006'),
                                     ('production', 'youtube-study-space', 'refs/heads/main')):
            for _, _, _, component in RELEASES:
                account, actual_project, actual_ref = release_target(component, target)
                self.assertEqual((actual_project, actual_ref), (project, ref))
                self.assertEqual(account, 'mypage-' + component + '-deploy@' + project + '.iam.gserviceaccount.com')
                accounts[target, component] = account
        self.assertEqual(len(set(accounts.values())), len(accounts))

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

    def test_frontend_deploy_is_manual_and_validates_mypage_first(self):
        workflow = load(WORKFLOWS / 'deploy-mypage-frontend.yml')
        self.assertEqual(triggers(workflow), {'workflow_dispatch'})
        inputs = workflow['on']['workflow_dispatch']['inputs']
        self.assertEqual(set(inputs), {'environment'})
        self.assertEqual(inputs['environment']['options'], ['development', 'production'])
        self.assertEqual(inputs['environment']['default'], 'development')
        self.assertEqual(workflow['permissions'], {'contents': 'read'})
        jobs = workflow['jobs']
        self.assertEqual(set(jobs), {'select', 'validate', 'hosting'})
        self.assertEqual(jobs['validate']['uses'], './.github/workflows/ci-frontend.yml')
        self.assertEqual(jobs['validate']['with'], {'youtube_monitor': False, 'docs_site': False, 'mypage': True})
        self.assertEqual(jobs['hosting']['needs'], ['select', 'validate'])
        self.assertNotIn('if', jobs['hosting'])
        self.assertEqual(jobs['hosting']['uses'], './.github/workflows/mypage-hosting-authenticated.yml')
        self.assertEqual(jobs['hosting']['with'], {'environment': '${{ inputs.environment }}',
                                                   'expected_sha': '${{ github.sha }}'})
        self.assertNotIn('secrets', jobs['hosting'])

    def test_operator_release_checks_out_without_credentials_before_the_contract(self):
        workflow = load(WORKFLOWS / 'mypage-operator-authenticated.yml')
        self.assertEqual(triggers(workflow), {'workflow_call'})
        steps = workflow['jobs']['deploy']['steps']
        names = [step.get('uses', step.get('run', '')) for step in steps]
        checkout = next(i for i, step in enumerate(steps) if step.get('uses', '').startswith('actions/checkout@'))
        context = names.index('python3 infra/mypage-platform/scripts/operator_release.py context')
        self.assertLess(checkout, context)
        self.assertIs(steps[checkout]['with']['persist-credentials'], False)

    def test_every_cloud_run_writer_shares_one_lock_per_environment_and_hosting_stays_separate(self):
        for name in ('mypage-backend-authenticated.yml', 'mypage-operator-authenticated.yml'):
            with self.subTest(workflow=name):
                self.assertEqual(load(WORKFLOWS / name)['concurrency'],
                                 {'group': environment_expression('mypage-dev-cloud-run', 'mypage-prod-cloud-run'),
                                  'cancel-in-progress': False})
        terraform = load(WORKFLOWS / 'terraform.yml')['concurrency']['group']
        self.assertIn("(inputs.environment == 'development' && 'mypage-dev-cloud-run' || 'mypage-prod-cloud-run')", terraform)
        hosting = load(WORKFLOWS / 'mypage-hosting-authenticated.yml')['concurrency']['group']
        self.assertNotIn('cloud-run', hosting)
        for path in WORKFLOWS.glob('*.yml'):
            self.assertNotIn('mypage-dev-runtime-change', path.read_text(), path.name)

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

    def test_receipts_are_verified_by_the_release_script_without_credentials(self):
        for name, script, record in (('mypage-backend-authenticated.yml', 'backend_release.py', 'mypage-backend-release-record'),
                                     ('mypage-operator-authenticated.yml', 'operator_release.py', 'mypage-operator-release-record')):
            with self.subTest(workflow=name):
                receipt = load(WORKFLOWS / name)['jobs']['receipt']
                self.assertEqual(receipt['permissions'], {'contents': 'read'})
                runs = [step for step in receipt['steps'] if 'run' in step]
                self.assertEqual([step['run'] for step in runs],
                                 ['python3 infra/mypage-platform/scripts/' + script + ' receipt'])
                self.assertEqual(runs[0]['env']['MYPAGE_TARGET'], '${{ inputs.environment }}')
                download = next(step for step in receipt['steps'] if step.get('uses', '').startswith('actions/download-artifact@'))
                self.assertEqual(download['with']['name'], record)


if __name__ == '__main__':
    unittest.main()
