"""手動Terraform workflow（UC2手動Plan / UC3 Apply）の選択・順序・結果判定を検証する。"""
import os
import subprocess
import unittest

from test_ci_workflows import ROOT, load

CORE_SECRETS = {'GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT', 'GCP_TERRAFORM_NORMAL_WIF_PROVIDER',
                'AWS_TERRAFORM_BACKEND_ROLE_ARN', 'AWS_TERRAFORM_BACKEND_ROLE_ID',
                'AWS_TERRAFORM_STATE_BUCKET', 'AWS_TERRAFORM_STATE_ACCOUNT_ID',
                'GCP_TERRAFORM_WIF_PROVIDER', 'GCP_TERRAFORM_SERVICE_ACCOUNT',
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
        command = self.jobs[job]['steps'][-1]['run']
        return subprocess.run(['bash', '-c', command], cwd=ROOT, capture_output=True, text=True,
                              env={'PATH': os.environ.get('PATH', os.defpath), **env,
                                   'GITHUB_OUTPUT': os.devnull}).returncode

    def test_manual_only_with_environment_operation_and_component(self):
        self.assertEqual(set(self.workflow['on']), {'workflow_dispatch'})
        inputs = self.workflow['on']['workflow_dispatch']['inputs']
        self.assertEqual({key: value['options'] for key, value in inputs.items()}, {
            'environment': ['development', 'production'],
            'operation': ['plan', 'apply'],
            'component': ['all', 'core', 'mypage'],
        })
        self.assertEqual(self.workflow['permissions'], {'contents': 'read'})
        self.assertIs(self.workflow['concurrency']['cancel-in-progress'], False)
        self.assertEqual(self.workflow['concurrency']['group'],
                         "${{ inputs.environment == 'development' && 'mypage-dev-runtime-change' || 'gcp-infrastructure-production' }}")

    def test_selection_rejects_unknown_values_and_disabled_mypage(self):
        valid = {'ENVIRONMENT': 'development', 'OPERATION': 'plan', 'COMPONENT': 'all'}
        self.assertEqual(self.run_step('select', **valid), 0)
        for change in ({'ENVIRONMENT': 'staging'}, {'OPERATION': 'destroy'}, {'COMPONENT': 'both'}):
            with self.subTest(change=change):
                self.assertNotEqual(self.run_step('select', **(valid | change)), 0)
        # Production MyPage is disabled by its reviewed desired stage.
        self.assertEqual(self.run_step('select', **(valid | {'ENVIRONMENT': 'production'})), 0)
        self.assertNotEqual(self.run_step('select', **(valid | {'ENVIRONMENT': 'production', 'COMPONENT': 'mypage'})), 0)

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
                self.assertEqual(job['with']['expected_sha'], '${{ github.sha }}')
                self.assertEqual(set(job['secrets']), secrets)
                self.assertEqual(job['permissions'], {'contents': 'read', 'id-token': 'write'})
                self.assertNotIn('continue-on-error', job)

    def test_credentialless_validation_and_every_selected_plan_precede_any_apply(self):
        self.assertEqual(self.jobs['validate']['uses'], './.github/workflows/ci-gcp-terraform.yml')
        for name in ('core-plan', 'mypage-plan'):
            self.assertEqual(self.jobs[name]['needs'], ['select', 'validate'])
            self.assertNotIn('always()', self.jobs[name]['if'])
        core, mypage = self.jobs['core-apply'], self.jobs['mypage-apply']
        self.assertEqual(core['needs'], ['select', 'core-plan', 'mypage-plan'])
        self.assertEqual(mypage['needs'], ['select', 'core-plan', 'mypage-plan', 'core-apply'])
        for job in (core, mypage):
            self.assertIn("inputs.operation == 'apply'", job['if'])
            self.assertIn("needs.core-plan.result == 'success'", job['if'])
            self.assertIn("needs.mypage-plan.result == 'success'", job['if'])
        # MyPage applies after Core unless only MyPage was selected.
        self.assertIn("needs.core-apply.result == 'success' || (inputs.component == 'mypage' && needs.core-apply.result == 'skipped')", mypage['if'])

    def test_gate_requires_exactly_the_selected_results(self):
        gate = self.jobs['gate']
        self.assertEqual(gate['if'], '${{ always() }}')
        self.assertEqual(set(gate['needs']), set(self.jobs) - {'gate'})
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
