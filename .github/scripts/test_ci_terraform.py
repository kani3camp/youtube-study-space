"""手動Terraform workflow（UC2手動Plan / UC3 Apply）の選択・順序・結果判定を検証する。"""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

from test_ci_workflows import ROOT, load

CORE_SECRETS = {'GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT', 'GCP_TERRAFORM_NORMAL_WIF_PROVIDER',
                'AWS_TERRAFORM_BACKEND_ROLE_ARN', 'AWS_TERRAFORM_BACKEND_ROLE_ID',
                'AWS_TERRAFORM_STATE_BUCKET', 'AWS_TERRAFORM_STATE_ACCOUNT_ID',
                'GCP_TERRAFORM_MANAGED_INPUTS_JSON'}
MYPAGE_SECRETS = {'MYPAGE_STATE_BUCKET', 'MYPAGE_STATE_ACCOUNT', 'MYPAGE_BACKEND_ROLE_ARN',
                  'MYPAGE_BACKEND_ROLE_ID', 'MYPAGE_GCP_WIF_PROVIDER',
                  'MYPAGE_GCP_SERVICE_ACCOUNT', 'MYPAGE_PLAN_HMAC_KEY',
                  'MYPAGE_RUNTIME_SETTINGS_JSON'}


class TerraformWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.workflow = load(ROOT / '.github/workflows/terraform.yml')
        self.jobs = self.workflow['jobs']

    def run_step(self, job, **env):
        steps = self.jobs[job]['steps']
        command = next((step for step in steps if step.get('id') == job), steps[-1])['run']
        return subprocess.run(['bash', '-c', command], cwd=ROOT, capture_output=True, text=True,
                              env={'PATH': os.environ.get('PATH', os.defpath), **env,
                                   'GITHUB_OUTPUT': os.devnull}).returncode

    def run_pr_select(self, changed, *, git_status=0, **env):
        """Run the real selection shell with a fake git that lists `changed`."""
        with tempfile.TemporaryDirectory() as directory:
            git = Path(directory) / 'git'
            git.write_text('#!/bin/sh\n'
                           '[ "$2" = "--no-renames" ] || exit 99\n'
                           'printf "%s\\n" ' + ' '.join(f"'{path}'" for path in changed) + '\n'
                           f'exit {git_status}\n')
            git.chmod(0o755)
            output = Path(directory) / 'output'
            values = {'GITHUB_REPOSITORY': 'kani3camp/youtube-study-space', 'HEAD_REPOSITORY': 'kani3camp/youtube-study-space',
                      'GITHUB_ACTOR_ID': '54093651', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp', 'GITHUB_RUN_ATTEMPT': '1',
                      'BASE_SHA': 'a' * 40, 'HEAD_SHA': 'b' * 40} | env
            result = subprocess.run(['bash', '-c', self.jobs['pr-select']['steps'][-1]['run']], cwd=ROOT,
                                    capture_output=True, text=True,
                                    env={'PATH': directory + os.pathsep + os.environ.get('PATH', os.defpath),
                                         **values, 'GITHUB_OUTPUT': str(output)})
            outputs = dict(line.split('=', 1) for line in output.read_text().splitlines()) if output.exists() else {}
            return result.returncode, outputs

    def test_pr_plan_selects_changed_roots_and_fails_visibly_for_forks_and_other_actors(self):
        for changed, core, mypage in (
            (['infra/gcp/environments/dev/main.tf'], 'true', 'false'),
            (['infra/mypage-platform/modules/runtime/main.tf'], 'false', 'true'),
            (['.github/workflows/terraform.yml'], 'true', 'true'),
            (['.github/workflows/gcp-terraform-authenticated.yml'], 'true', 'false'),
            (['.github/workflows/mypage-foundation-authenticated.yml'], 'false', 'true'),
            (['.github/scripts/terraform_plan_summary.py', 'docs/README.md'], 'true', 'false'),
            (['docs/README.md'], 'false', 'false'),
        ):
            with self.subTest(changed=changed):
                self.assertEqual(self.run_pr_select(changed), (0, {'core': core, 'mypage': mypage}))
        # A moved file is listed under both paths, so a root it leaves is still planned.
        self.assertEqual(self.run_pr_select(['infra/gcp/environments/dev/example.tf', 'docs/example.tf']),
                         (0, {'core': 'true', 'mypage': 'false'}))
        # A failed diff fails the job instead of selecting nothing.
        code, outputs = self.run_pr_select(['infra/gcp/environments/dev/main.tf'], git_status=128)
        self.assertNotEqual(code, 0)
        self.assertEqual(outputs, {})
        for env in ({'HEAD_REPOSITORY': 'fork/youtube-study-space'}, {'GITHUB_ACTOR_ID': '1'},
                    {'GITHUB_TRIGGERING_ACTOR': 'other'}, {'GITHUB_RUN_ATTEMPT': '2'}):
            with self.subTest(env=env):
                code, outputs = self.run_pr_select(['infra/gcp/environments/dev/main.tf'], **env)
                self.assertNotEqual(code, 0)
                self.assertEqual(outputs, {})

    def test_pr_plan_reuses_the_dev_manual_plan_and_never_reaches_apply(self):
        self.assertEqual(self.workflow['concurrency']['cancel-in-progress'], "${{ github.event_name == 'pull_request' }}")
        self.assertIn("format('terraform-pr-plan-{0}', github.event.pull_request.number)", self.workflow['concurrency']['group'])
        self.assertFalse((ROOT / '.github/workflows/terraform-pr-plan.yml').exists())
        for name, uses, target, mode, secrets in (
            ('pr-plan-core', 'gcp-terraform-authenticated.yml', 'dev', 'pr-plan', CORE_SECRETS),
            ('pr-plan-mypage', 'mypage-foundation-authenticated.yml', 'development', 'desired-plan', MYPAGE_SECRETS),
        ):
            job = self.jobs[name]
            with self.subTest(job=name):
                self.assertEqual(job['uses'], './.github/workflows/' + uses)
                self.assertEqual(job['needs'], 'pr-select')
                self.assertNotIn('strategy', job)
                self.assertEqual(job['with'], {
                    'target': target, 'mode': mode, 'expected_sha': '${{ github.sha }}',
                    'pr_number': "${{ format('{0}', github.event.pull_request.number) }}",
                    'head_repository': '${{ github.event.pull_request.head.repo.full_name }}'})
                self.assertEqual(set(job['secrets']), secrets)
        # Every manual job is unreachable from a PR event.
        self.assertEqual(self.jobs['select']['if'], "${{ github.event_name == 'workflow_dispatch' }}")
        for name in ('validate', 'core-plan', 'mypage-plan', 'core-apply', 'mypage-apply'):
            self.assertIn('select', self.jobs[name]['needs'])
        # In the reusables a PR reaches only the Plan job: Apply jobs require a dispatch-only mode.
        core = load(ROOT / '.github/workflows/gcp-terraform-authenticated.yml')['jobs']
        self.assertIn("inputs.mode == 'pr-plan'", core['normal_plan']['if'])
        self.assertEqual(core['normal_plan']['environment'], 'terraform-${{ inputs.target }}-plan')
        self.assertEqual(core['normal_apply']['if'], "${{ inputs.mode == 'normal-apply' }}")
        mypage = load(ROOT / '.github/workflows/mypage-foundation-authenticated.yml')['jobs']
        self.assertNotIn("'desired-plan'", mypage['foundation_apply']['if'])
        self.assertNotIn("'desired-plan'", mypage['image_build']['if'])

    def test_manual_only_with_environment_operation_and_component(self):
        self.assertEqual(set(self.workflow['on']), {'workflow_dispatch', 'pull_request'})
        inputs = self.workflow['on']['workflow_dispatch']['inputs']
        self.assertEqual({key: value['options'] for key, value in inputs.items() if value['type'] == 'choice'}, {
            'environment': ['development', 'production'],
            'operation': ['plan', 'apply'],
            'component': ['all', 'core', 'mypage'],
        })
        # Optional DEV Core source selectors; empty means the dispatched commit.
        for key in ('source_branch', 'source_sha'):
            self.assertEqual(inputs[key], inputs[key] | {'type': 'string', 'required': False, 'default': ''})
        self.assertEqual(set(inputs), {'environment', 'operation', 'component', 'source_branch', 'source_sha'})
        self.assertEqual(self.workflow['permissions'], {'contents': 'read'})
        self.assertIn("(inputs.environment == 'development' && 'mypage-dev-cloud-run' || 'mypage-prod-cloud-run')",
                      self.workflow['concurrency']['group'])

    def test_selection_rejects_unknown_values_and_disabled_mypage(self):
        valid = {'ENVIRONMENT': 'development', 'OPERATION': 'plan', 'COMPONENT': 'all'}
        self.assertEqual(self.run_step('select', **valid), 0)
        for change in ({'ENVIRONMENT': 'staging'}, {'OPERATION': 'destroy'}, {'COMPONENT': 'both'}):
            with self.subTest(change=change):
                self.assertNotEqual(self.run_step('select', **(valid | change)), 0)
        # Production MyPage is disabled by its reviewed desired stage.
        self.assertEqual(self.run_step('select', **(valid | {'ENVIRONMENT': 'production'})), 0)
        self.assertNotEqual(self.run_step('select', **(valid | {'ENVIRONMENT': 'production', 'COMPONENT': 'mypage'})), 0)
        # Only DEV Core may select another source branch/SHA.
        for source in ({'SOURCE_BRANCH': 'feature/x'}, {'SOURCE_SHA': 'b' * 40}):
            self.assertEqual(self.run_step('select', **(valid | {'COMPONENT': 'core'} | source)), 0)
            for change in ({'COMPONENT': 'all'}, {'COMPONENT': 'mypage'}, {'ENVIRONMENT': 'production', 'COMPONENT': 'core'}):
                with self.subTest(source=source, change=change):
                    self.assertNotEqual(self.run_step('select', **(valid | source | change)), 0)

    def select_outputs(self, **env):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'output'
            steps = self.jobs['select']['steps']
            command = next(step for step in steps if step.get('id') == 'select')['run']
            result = subprocess.run(['bash', '-c', command], cwd=ROOT, capture_output=True, text=True,
                                    env={'PATH': os.environ.get('PATH', os.defpath), **env,
                                         'GITHUB_OUTPUT': str(output)})
            outputs = dict(line.split('=', 1) for line in output.read_text().splitlines()) if output.exists() else {}
            return result.returncode, outputs, result.stdout

    def test_mypage_runs_only_from_the_ref_its_cloud_trust_pins(self):
        pinned = 'refs/heads/integration/mypage-canon-20261006'
        stage = (ROOT / 'infra/mypage-platform/environments/dev/desired-stage.txt').read_text().strip()
        self.assertNotEqual(stage, 'disabled')
        base = {'ENVIRONMENT': 'development', 'OPERATION': 'plan'}
        code, outputs, _ = self.select_outputs(**base, COMPONENT='all', GITHUB_REF=pinned)
        self.assertEqual((code, outputs.get('mypage_stage')), (0, stage))
        # From another ref (for example dev before the trust moves) Core still runs and MyPage is skipped.
        for ref in ('refs/heads/dev', 'refs/heads/feature/x', ''):
            code, outputs, log = self.select_outputs(**base, COMPONENT='all', GITHUB_REF=ref)
            self.assertEqual((code, outputs.get('mypage_stage')), (0, 'disabled'), ref)
            self.assertIn('MyPage is skipped', log)
            code, _, log = self.select_outputs(**base, COMPONENT='mypage', GITHUB_REF=ref)
            self.assertNotEqual(code, 0)
            self.assertIn('MyPage can only be operated from ' + pinned, log)
            code, outputs, _ = self.select_outputs(**base, COMPONENT='core', GITHUB_REF=ref)
            self.assertEqual(code, 0)

    def test_each_root_keeps_its_reusable_mode_and_secret_boundary(self):
        for name, uses, mode, secrets in (
            ('core-plan', 'gcp-terraform-authenticated.yml', 'normal-plan', CORE_SECRETS),
            ('core-apply', 'gcp-terraform-authenticated.yml', 'normal-apply', CORE_SECRETS),
            ('mypage-plan', 'mypage-foundation-authenticated.yml', 'desired-plan', MYPAGE_SECRETS),
            ('mypage-apply', 'mypage-foundation-authenticated.yml', 'desired-apply', MYPAGE_SECRETS),
        ):
            job = self.jobs[name]
            with self.subTest(job=name):
                self.assertEqual(job['uses'], './.github/workflows/' + uses)
                self.assertEqual(job['with']['mode'], mode)
                if name.startswith('core-'):
                    # Core plans/applies exactly the commit that validate checked.
                    self.assertEqual(job['with']['expected_sha'], '${{ needs.select.outputs.validation_sha }}')
                    self.assertEqual(job['with']['source_branch'], '')
                else:
                    # MyPage has no source selection: the dispatched commit is the validated one.
                    self.assertEqual(job['with']['expected_sha'], '${{ github.sha }}')
                self.assertEqual(set(job['secrets']), secrets)
                # A caller may pass only secrets the reusable workflow declares (workflow_call contract).
                declared = load(ROOT / '.github/workflows' / uses)['on']['workflow_call']['secrets']
                self.assertLessEqual(set(job['secrets']), set(declared))
                self.assertEqual(job['permissions'], {'contents': 'read', 'id-token': 'write'})
                self.assertNotIn('continue-on-error', job)

    def test_credentialless_validation_and_every_selected_plan_precede_any_apply(self):
        self.assertEqual(self.jobs['validate']['uses'], './.github/workflows/ci-gcp-terraform.yml')
        self.assertEqual(self.jobs['validate']['with'], {'source_sha': '${{ needs.select.outputs.validation_sha }}'})
        for name in ('core-plan', 'mypage-plan'):
            self.assertEqual(self.jobs[name]['needs'], ['select', 'validate'])
            self.assertNotIn('always()', self.jobs[name]['if'])
        core, mypage = self.jobs['core-apply'], self.jobs['mypage-apply']
        self.assertEqual(core['needs'], ['select', 'core-plan', 'mypage-plan'])
        self.assertEqual(mypage['needs'], ['select', 'core-plan', 'mypage-plan', 'core-apply'])
        for job in (core, mypage):
            # Cancelling the run after the Plans must never start an Apply.
            self.assertTrue(job['if'].startswith('${{ !cancelled() && '))
            self.assertIn("inputs.operation == 'apply'", job['if'])
            self.assertIn("needs.core-plan.result == 'success'", job['if'])
            self.assertIn("needs.mypage-plan.result == 'success'", job['if'])
        # MyPage applies after Core unless only MyPage was selected.
        self.assertIn("needs.core-apply.result == 'success' || (inputs.component == 'mypage' && needs.core-apply.result == 'skipped')", mypage['if'])

    def test_gate_requires_exactly_the_selected_results(self):
        gate = self.jobs['gate']
        self.assertEqual(gate['if'], "${{ always() && github.event_name == 'workflow_dispatch' }}")
        self.assertEqual(set(gate['needs']), {name for name in self.jobs if name != 'gate' and not name.startswith('pr-')})
        ok = {'SELECT_RESULT': 'success', 'VALIDATE_RESULT': 'success', 'MYPAGE_STAGE': 'runtime_public'}
        def results(core_plan, mypage_plan, core_apply, mypage_apply):
            return {'CORE_PLAN_RESULT': core_plan, 'MYPAGE_PLAN_RESULT': mypage_plan,
                    'CORE_APPLY_RESULT': core_apply, 'MYPAGE_APPLY_RESULT': mypage_apply}
        passing = (
            ('plan', 'all', results('success', 'success', 'skipped', 'skipped'), ok),
            ('apply', 'all', results('success', 'success', 'success', 'success'), ok),
            ('apply', 'core', results('success', 'skipped', 'success', 'skipped'), ok),
            ('apply', 'mypage', results('skipped', 'success', 'skipped', 'success'), ok),
            ('apply', 'all', results('success', 'skipped', 'success', 'skipped'), ok | {'MYPAGE_STAGE': 'disabled'}),
        )
        for operation, component, values, base in passing:
            with self.subTest(operation=operation, component=component, values=values):
                self.assertEqual(self.run_step('gate', OPERATION=operation, COMPONENT=component, **base, **values), 0)
        failing = (
            ('apply', 'all', results('success', 'success', 'success', 'skipped'), ok),
            ('apply', 'all', results('success', 'failure', 'skipped', 'skipped'), ok),
            ('plan', 'all', results('success', 'success', 'success', 'skipped'), ok),
            ('plan', 'core', results('success', 'success', 'skipped', 'skipped'), ok),
            ('plan', 'all', results('success', 'success', 'skipped', 'skipped'), ok | {'VALIDATE_RESULT': 'failure'}),
            ('destroy', 'all', results('success', 'success', 'skipped', 'skipped'), ok),
        )
        for operation, component, values, base in failing:
            with self.subTest(operation=operation, component=component, values=values):
                self.assertNotEqual(self.run_step('gate', OPERATION=operation, COMPONENT=component, **base, **values), 0)

    def test_retired_orchestrators_and_router_operations_are_gone(self):
        for retired in ('gcp-infrastructure-authenticated.yml', 'mypage-infrastructure-authenticated.yml'):
            self.assertFalse((ROOT / '.github/workflows' / retired).exists())
        ci = (ROOT / '.github/workflows/ci.yml').read_text()
        for operation in ('gcp-plan', 'gcp-apply'):
            self.assertNotIn(operation, ci)


if __name__ == '__main__':
    unittest.main()
