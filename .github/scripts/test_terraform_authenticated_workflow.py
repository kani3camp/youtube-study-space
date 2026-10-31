#!/usr/bin/env python3
"""Run credentialless admission and assert the current execution boundary."""
import os
from pathlib import Path
import re
import subprocess
import tempfile
import textwrap
import unittest

import sys
sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'infra/gcp/tests'))
from test_terraform_normal_ops import env as environment

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / '.github/workflows/gcp-terraform-authenticated.yml'
CALLER = ROOT / '.github/workflows/terraform.yml'


class TerraformAuthenticatedWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.text = WORKFLOW.read_text()
        cls.caller = CALLER.read_text()

    def run_preflight(self, **changes):
        block = self.text.split('      - name: Enforce trusted execution surface\n', 1)[1]
        script = textwrap.dedent(block.split('        run: |\n', 1)[1].split('\n  normal_plan:', 1)[0])
        values = environment() | {'TARGET': 'dev', 'MODE': 'normal-plan'}
        values.pop('NORMAL_SOURCE_SHA', None)  # preflightには確定後のjob出力を持ち込まない。
        values.update(changes)
        values['NORMAL_TARGET'] = values['TARGET']
        values['NORMAL_MODE'] = values['MODE']
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'outputs'
            result = subprocess.run(['bash', '-c', script], cwd=ROOT,
                env={'PATH': os.environ.get('PATH', os.defpath), 'PYTHONDONTWRITEBYTECODE': '1', **values, 'GITHUB_OUTPUT': str(output)},
                capture_output=True, text=True)
            result.outputs = output.read_text() if output.exists() else ''
            return result

    def test_only_caller_exposes_plan_and_apply(self):
        # Migration and live E2E were retired (#1521); the Terraform caller exposes only plan/apply.
        self.assertIn('mode: normal-plan', self.caller)
        self.assertIn('mode: normal-apply', self.caller)
        for retired in ('migration-plan', 'migration-apply', 'normal-e2e', 'terraform_mode:'):
            self.assertNotIn(retired, self.caller)
            self.assertNotIn(retired + ')', self.text)


    def test_dev_preflight_checks_owner_while_environment_selects_trusted_branches(self):
        ref = 'refs/heads/feature/no-environment-registration'
        context = {'GITHUB_REF': ref, 'NORMAL_EXPECTED_SHA': 'a' * 40,
                   'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + ref}
        for mode in ('normal-plan', 'normal-apply'):
            self.assertEqual(self.run_preflight(MODE=mode, **context).returncode, 0)
            for key, value in (('GITHUB_ACTOR', 'other'), ('GITHUB_ACTOR_ID', '0'),
                               ('GITHUB_TRIGGERING_ACTOR', 'other'), ('GITHUB_RUN_ATTEMPT', '2'),
                               ('GITHUB_EVENT_NAME', 'pull_request')):
                with self.subTest(mode=mode, key=key):
                    result = self.run_preflight(MODE=mode, **(context | {key: value}))
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(result.outputs, '')

    def test_production_normal_requires_main_and_disallows_cloud_e2e(self):
        ref = 'refs/heads/main'
        context = {'TARGET': 'prod', 'GITHUB_REF': ref,
                   'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + ref,
                   'NORMAL_EXPECTED_SHA': 'a' * 40}
        for mode in ('normal-plan', 'normal-apply'):
            with self.subTest(mode=mode):
                result = self.run_preflight(MODE=mode, **context)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotEqual(self.run_preflight(MODE='normal-e2e', **context).returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE='normal-plan', TARGET='prod',
            GITHUB_REF='refs/heads/dev',
            GITHUB_WORKFLOW_REF='kani3camp/youtube-study-space/.github/workflows/terraform.yml@refs/heads/dev',
            NORMAL_EXPECTED_SHA='a' * 40).returncode, 0)

    def test_control_branch_uses_only_the_terraform_caller_and_paths_stay_fixed(self):
        # The use-case redesign (#1500) made terraform.yml the only caller for DEV and PROD;
        # the CI workflow no longer dispatches Terraform.
        for branch in ('dev', 'integration/mypage-canon-20261006', 'feature/trusted-controller'):
            ref = 'refs/heads/' + branch
            values = {'MODE': 'normal-plan', 'NORMAL_EXPECTED_SHA': 'a' * 40, 'GITHUB_REF': ref,
                      'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + ref}
            result = self.run_preflight(**values)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            for workflow in ('attacker.yml@' + ref, 'ci.yml@' + ref, 'terraform.yml@refs/heads/other'):
                bad = values | {'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/' + workflow}
                self.assertNotEqual(self.run_preflight(**bad).returncode, 0)
        main = 'kani3camp/youtube-study-space/.github/workflows/{}@refs/heads/main'
        self.assertEqual(self.run_preflight(TARGET='prod', MODE='normal-plan', NORMAL_EXPECTED_SHA='a' * 40,
                         GITHUB_REF='refs/heads/main', GITHUB_WORKFLOW_REF=main.format('terraform.yml')).returncode, 0)
        self.assertNotEqual(self.run_preflight(TARGET='prod', MODE='normal-plan', NORMAL_EXPECTED_SHA='a' * 40,
                            GITHUB_REF='refs/heads/main', GITHUB_WORKFLOW_REF=main.format('ci.yml')).returncode, 0)

    def test_normal_oidc_targets_correct_gcp_project(self):
        for role in ('normal_plan', 'normal_apply'):
            block = self.text.split('  ' + role + ':', 1)[1]
            self.assertIn("project_id: $", block, role)
            self.assertIn("'test-youtube-study-space'", block, role)
            self.assertIn("'youtube-study-space'", block, role)
            self.assertIn('GCP_TERRAFORM_NORMAL_WIF_PROVIDER', block, role)
            self.assertIn('GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT', block, role)
            self.assertNotIn('project_id: test-youtube-study-space', block, role)

    def test_retired_modes_cannot_reach_credentials_even_with_old_gates_enabled(self):
        modes = ('plan', 'apply', 'history_post_noop', 'security-probe', 'email-adoption',
                 'quota-plan', 'quota-create', 'quota-refresh', 'current-state-read',
                 'runtime-metadata-read', 'dev-history-observation-sync', 'migration-unknown', 'migration-plan', 'migration-apply', 'normal-e2e', 'destroy')
        for target in ('dev', 'prod'):
            ref = 'refs/heads/' + ('dev' if target == 'dev' else 'main')
            for mode in modes:
                with self.subTest(target=target, mode=mode):
                    result = self.run_preflight(TARGET=target, MODE=mode, GITHUB_REF=ref,
                        GITHUB_WORKFLOW_REF='kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + ref,
                        DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED='true', DEV_HISTORY_POST_NOOP_ENABLED='true',
                        DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED='true', PROD_AUTHENTICATED_TERRAFORM_ENABLED='true')
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(result.outputs, '')
        inputs = self.text.split('    # Explicit names', 1)[0]
        self.assertEqual(re.findall(r'^      ([a-z_]+):', inputs, re.M), ['target', 'mode', 'expected_sha', 'source_branch'])
        caller_inputs = self.caller.split('\npermissions:', 1)[0]
        self.assertNotRegex(caller_inputs, r'terraform_(history|current_state|ownership|plan_cost)')

    def test_untrusted_context_is_rejected_before_credentials(self):
        for key, value in {
            'GITHUB_EVENT_NAME': 'pull_request', 'GITHUB_REPOSITORY': 'attacker/repo',
            'GITHUB_REPOSITORY_ID': '0', 'GITHUB_REPOSITORY_OWNER_ID': '0',
            'GITHUB_ACTOR_ID': '0', 'GITHUB_ACTOR': 'attacker', 'GITHUB_TRIGGERING_ACTOR': 'attacker',
            'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_SHA': 'invalid', 'TARGET': 'unknown',
            'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/other.yml@refs/heads/dev',
        }.items():
            with self.subTest(key=key):
                result = self.run_preflight(**{key: value})
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.outputs, '')
        for target, refs in (('dev', ('refs/heads/main', 'refs/tags/v1', 'refs/pull/1/merge')),
                             ('prod', ('refs/heads/dev', 'refs/heads/integration/mypage-canon-20261006'))):
            for ref in refs:
                result = self.run_preflight(TARGET=target, GITHUB_REF=ref,
                    GITHUB_WORKFLOW_REF='kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + ref)
                self.assertNotEqual(result.returncode, 0)
        result = self.run_preflight(GITHUB_REF='refs/heads/integration/mypage-canon-20261006')
        self.assertNotEqual(result.returncode, 0)

    def test_credentials_and_apply_require_independent_environments_and_preflight(self):
        jobs = dict(re.findall(r'^  ([a-z_]+):\n(.*?)(?=^  [a-z_]+:|\Z)', self.text.split('jobs:\n', 1)[1], re.M | re.S))
        self.assertEqual(set(jobs), {'preflight', 'normal_plan', 'normal_apply'})
        self.assertNotIn('id-token: write', jobs['preflight'])
        self.assertNotIn('secrets.', jobs['preflight'])
        self.assertIn('needs: [preflight, normal_plan]', jobs['normal_apply'])
        for role in ('plan', 'apply'):
            block = jobs['normal_' + role]
            self.assertIn('preflight', block.split('    steps:', 1)[0])
            self.assertIn('environment: terraform-${{ inputs.target }}-' + role, block)
            self.assertIn('id-token: write', block)
            ordered = ['terraform_normal_ops.py stage', 'terraform_normal_ops.py oidc',
                       'uses: aws-actions/configure-aws-credentials@', 'uses: google-github-actions/auth@',
                       'terraform_normal_ops.py ' + ('execute' if role == 'plan' else 'prepare'),
                       'terraform_normal_ops.py cleanup']
            self.assertEqual([block.index(x) for x in ordered], sorted(block.index(x) for x in ordered))
            self.assertIn('ref: ${{ github.sha }}', block)
            self.assertIn('persist-credentials: false', block)
            self.assertIn('if: ${{ always() }}', block)
            self.assertNotIn('continue-on-error', block)
        self.assertIn('steps.prepared.outputs.digest == needs.normal_plan.outputs.digest', jobs['normal_apply'])

    def test_dev_source_checkout_never_selects_workflow_or_executor(self):
        self.assertIn('source_sha: ${{ steps.guard.outputs.source_sha }}', self.text)
        self.assertIn('source_branch: ${{ steps.guard.outputs.source_branch }}', self.text)
        for role in ('plan', 'apply'):
            block = self.text.split('\n  normal_' + role + ':', 1)[1].split('\n  normal_', 1)[0]
            self.assertIn("NORMAL_SOURCE_SHA: ${{ inputs.target == 'dev' && needs.preflight.outputs.source_sha || '' }}", block)
            self.assertIn('repository: kani3camp/youtube-study-space', block)
            self.assertIn('ref: ${{ needs.preflight.outputs.source_sha }}', block)
            self.assertNotIn('ref: ${{ inputs.expected_sha }}', block)
            self.assertIn('path: .terraform-source', block)
            self.assertIn('sparse-checkout: infra/gcp', block)
            self.assertLess(block.index('ref: ${{ github.sha }}'), block.index('path: .terraform-source'))
            self.assertNotRegex(block, r'run:.*\.terraform-source')
            self.assertNotIn('working-directory: .terraform-source', block)
        # terraform.yml does not expose source selection yet: it always targets its own commit.
        self.assertNotIn('source_branch:', self.caller)
        self.assertIn('expected_sha: ${{ github.sha }}', self.caller)

    def test_dev_sha_can_be_omitted_and_production_sha_remains_required(self):
        for mode in ('normal-plan', 'normal-apply'):
            result = self.run_preflight(MODE=mode, NORMAL_EXPECTED_SHA='')
            self.assertEqual(result.returncode, 0)
            self.assertIn('source_sha=' + 'a' * 40, result.outputs)
            self.assertIn('source_branch=dev', result.outputs)
            result = self.run_preflight(TARGET='prod', MODE=mode, NORMAL_EXPECTED_SHA='',
                GITHUB_REF='refs/heads/main',
                GITHUB_WORKFLOW_REF='kani3camp/youtube-study-space/.github/workflows/terraform.yml@refs/heads/main')
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(result.outputs, '')

    def test_no_proof_history_artifact_or_unpinned_action_authorizes_execution(self):
        for forbidden in ('actions: read', 'actions/upload-artifact', 'actions/download-artifact',
                          'actions/cache', 'force-unlock', '-auto-approve', 'pull_request_target'):
            self.assertNotIn(forbidden, self.text)
        for action in re.findall(r'^\s*uses: (\S+)', self.text, re.M):
            self.assertRegex(action, r'@[0-9a-f]{40}$')
        self.assertNotIn('workflow_dispatch:', self.text)
        self.assertIn('cancel-in-progress: false', self.text)
        self.assertIn("          - plan\n          - apply\n", self.caller)
        self.assertNotIn("terraform_authenticated:", self.caller)


if __name__ == '__main__':
    unittest.main()
