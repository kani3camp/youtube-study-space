#!/usr/bin/env python3
"""Run credentialless admission and assert the current execution boundary."""
import os
from pathlib import Path
import re
import subprocess
import tempfile
import textwrap
import unittest

from test_terraform_migration import environment

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / '.github/workflows/gcp-terraform-authenticated.yml'
CALLER = ROOT / '.github/workflows/ci.yml'


class TerraformAuthenticatedWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.text = WORKFLOW.read_text()
        cls.caller = CALLER.read_text()

    def run_preflight(self, **changes):
        block = self.text.split('      - name: Enforce trusted execution surface\n', 1)[1]
        script = textwrap.dedent(block.split('        run: |\n', 1)[1].split('\n  migration_plan:', 1)[0])
        values = environment() | {'TARGET': 'dev', 'MODE': 'migration-plan', 'MIGRATION_PRODUCTION_READY': 'true'}
        values.update(changes)
        values['MIGRATION_TARGET'] = values['TARGET']
        values['MIGRATION_MODE'] = values['MODE']
        values['NORMAL_TARGET'] = values['TARGET']
        values['NORMAL_MODE'] = values['MODE']
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'outputs'
            result = subprocess.run(['bash', '-c', script], cwd=ROOT,
                env={'PATH': os.environ.get('PATH', os.defpath), 'PYTHONDONTWRITEBYTECODE': '1', **values, 'GITHUB_OUTPUT': str(output)},
                capture_output=True, text=True)
            result.outputs = output.read_text() if output.exists() else ''
            return result

    def test_display_names_describe_existing_resource_boundary_without_mode_changes(self):
        self.assertIn('name: "Terraform Existing Resources Plan (${{ inputs.target }})"', self.text)
        self.assertIn('name: "Terraform Existing Resources Apply (${{ inputs.target }})"', self.text)
        self.assertIn('name: Validate bounded existing-resource Terraform plan', self.text)
        self.assertIn('name: Validate bounded existing-resource Terraform apply', self.text)
        self.assertIn('description: Existing-resource adoption/no-op only (not general create/update)', self.text)
        self.assertIn('description: Authenticated Terraform (DEV native operations or existing-resource migration)', self.caller)
        self.assertNotIn('name: "Import-only migration plan', self.text)
        self.assertNotIn('name: "Import-only migration apply', self.text)
        # Keep the public dispatch values and fail-closed controller selectors stable.
        self.assertIn('default: migration-plan', self.caller)
        self.assertIn('migration-plan|migration-apply) ;;', self.text)
        self.assertIn('  migration_plan:', self.text)
        self.assertIn('  migration_apply:', self.text)

    def test_current_modes_admit_same_branch_dev_and_main_only_prod(self):
        for mode in ('migration-plan', 'migration-apply'):
            for target, refs in (('dev', ('dev', 'integration/mypage-canon-20261006')), ('prod', ('main',))):
                for branch in refs:
                    ref = 'refs/heads/' + branch
                    with self.subTest(mode=mode, target=target, ref=ref):
                        result = self.run_preflight(TARGET=target, MODE=mode, GITHUB_REF=ref,
                            GITHUB_WORKFLOW_REF='kani3camp/youtube-study-space/.github/workflows/ci.yml@' + ref)
                        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_retired_modes_cannot_reach_credentials_even_with_old_gates_enabled(self):
        modes = ('plan', 'apply', 'history_post_noop', 'security-probe', 'email-adoption',
                 'quota-plan', 'quota-create', 'quota-refresh', 'current-state-read',
                 'runtime-metadata-read', 'dev-history-observation-sync', 'migration-unknown', 'destroy')
        for target in ('dev', 'prod'):
            ref = 'refs/heads/' + ('dev' if target == 'dev' else 'main')
            for mode in modes:
                with self.subTest(target=target, mode=mode):
                    result = self.run_preflight(TARGET=target, MODE=mode, GITHUB_REF=ref,
                        GITHUB_WORKFLOW_REF='kani3camp/youtube-study-space/.github/workflows/ci.yml@' + ref,
                        DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED='true', DEV_HISTORY_POST_NOOP_ENABLED='true',
                        DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED='true', PROD_AUTHENTICATED_TERRAFORM_ENABLED='true')
                    self.assertNotEqual(result.returncode, 0)
                    self.assertEqual(result.outputs, '')
        inputs = self.text.split('    # Explicit names', 1)[0]
        self.assertEqual(re.findall(r'^      ([a-z_]+):', inputs, re.M), ['target', 'mode', 'expected_sha'])
        caller_inputs = self.caller.split('\npermissions:', 1)[0]
        self.assertNotRegex(caller_inputs, r'terraform_(history|current_state|ownership|plan_cost)')

    def test_untrusted_context_is_rejected_before_credentials(self):
        for key, value in {
            'GITHUB_EVENT_NAME': 'pull_request', 'GITHUB_REPOSITORY': 'attacker/repo',
            'GITHUB_REPOSITORY_ID': '0', 'GITHUB_REPOSITORY_OWNER_ID': '0',
            'GITHUB_ACTOR_ID': '0', 'GITHUB_ACTOR': 'attacker', 'GITHUB_TRIGGERING_ACTOR': 'attacker',
            'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_SHA': 'invalid', 'TARGET': 'unknown',
            'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/other.yml@refs/heads/dev',
            'MIGRATION_PRODUCTION_READY': 'false',
        }.items():
            with self.subTest(key=key):
                extra = {'TARGET': 'prod', 'GITHUB_REF': 'refs/heads/main',
                    'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/main'} if key == 'MIGRATION_PRODUCTION_READY' else {}
                result = self.run_preflight(**(extra | {key: value}))
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.outputs, '')
        for target, refs in (('dev', ('refs/heads/main', 'refs/heads/feature/gcp-terraform-iac', 'refs/tags/v1', 'refs/pull/1/merge')),
                             ('prod', ('refs/heads/dev', 'refs/heads/integration/mypage-canon-20261006'))):
            for ref in refs:
                result = self.run_preflight(TARGET=target, GITHUB_REF=ref,
                    GITHUB_WORKFLOW_REF='kani3camp/youtube-study-space/.github/workflows/ci.yml@' + ref)
                self.assertNotEqual(result.returncode, 0)
        result = self.run_preflight(GITHUB_REF='refs/heads/integration/mypage-canon-20261006')
        self.assertNotEqual(result.returncode, 0)

    def test_credentials_and_apply_require_independent_environments_and_preflight(self):
        jobs = dict(re.findall(r'^  ([a-z_]+):\n(.*?)(?=^  [a-z_]+:|\Z)', self.text.split('jobs:\n', 1)[1], re.M | re.S))
        self.assertEqual(set(jobs), {'preflight', 'migration_plan', 'migration_apply', 'normal_plan', 'normal_apply'})
        self.assertNotIn('id-token: write', jobs['preflight'])
        self.assertNotIn('secrets.', jobs['preflight'])
        self.assertIn('needs: [preflight, migration_plan]', jobs['migration_apply'])
        for role in ('plan', 'apply'):
            block = jobs['migration_' + role]
            self.assertIn('preflight', block.split('    steps:', 1)[0])
            self.assertIn('environment: terraform-${{ inputs.target }}-' + role, block)
            self.assertIn('id-token: write', block)
            ordered = ['terraform_migration.py stage', 'terraform_migration.py oidc',
                       'uses: aws-actions/configure-aws-credentials@', 'uses: google-github-actions/auth@',
                       'terraform_migration.py execute', 'terraform_migration.py cleanup']
            self.assertEqual([block.index(x) for x in ordered], sorted(block.index(x) for x in ordered))
            self.assertIn('ref: ${{ github.sha }}', block)
            self.assertIn('persist-credentials: false', block)
            self.assertIn('if: ${{ always() }}', block)
            self.assertIn('token_format: access_token', block)
            self.assertNotIn('continue-on-error', block)
        for expected in ('MIGRATION_EXPECTED_COMMITMENT: ${{ needs.migration_plan.outputs.commitment }}',
                         'MIGRATION_EXPECTED_COMPONENTS: ${{ needs.migration_plan.outputs.components }}'):
            self.assertIn(expected, jobs['migration_apply'])

    def test_no_proof_history_artifact_or_unpinned_action_authorizes_execution(self):
        for forbidden in ('actions: read', 'actions/upload-artifact', 'actions/download-artifact',
                          'actions/cache', 'force-unlock', '-auto-approve', 'pull_request_target'):
            self.assertNotIn(forbidden, self.text)
        for action in re.findall(r'^\s*uses: (\S+)', self.text, re.M):
            self.assertRegex(action, r'@[0-9a-f]{40}$')
        self.assertNotIn('workflow_dispatch:', self.text)
        self.assertIn('cancel-in-progress: false', self.text)
        self.assertIn("inputs.terraform_authenticated == true", self.caller)


if __name__ == '__main__':
    unittest.main()
