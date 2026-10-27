"""Contracts for the call-only authenticated operations workflow."""
import unittest
from test_ci_workflows import (
    ROOT,
    load,
    AUTH_SECRETS,
    AUDIT_SECRETS,
    FOUNDATION_SECRETS,
    OPERATION_TOGGLES,
)


class OperationsWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.workflow = load(ROOT / '.github/workflows/ci-operations.yml')

    def test_workflow_is_call_only_and_requires_explicit_selection(self):
        self.assertEqual(set(self.workflow['on']), {'workflow_call'})
        self.assertEqual(self.workflow['permissions'], {'contents': 'read'})
        call = self.workflow['on']['workflow_call']
        for toggle in OPERATION_TOGGLES:
            self.assertEqual(call['inputs'][toggle], {'required': True, 'type': 'boolean'})
        for result in ('repository_result', 'frontend_result', 'system_result',
                       'gcp_terraform_validate_result'):
            self.assertEqual(call['inputs'][result], {'required': True, 'type': 'string'})

        validator = self.workflow['jobs']['validate-selection']
        self.assertEqual(validator['permissions'] if 'permissions' in validator else None, None)
        self.assertGreater(validator['timeout-minutes'], 0)
        command = validator['steps'][0]['run']
        for fragment in (
            'selected=$((selected + 1))',
            'if [[ "$selected" -ne 1 ]]',
            'REPOSITORY_RESULT',
            'GCP_TERRAFORM_VALIDATE_RESULT',
            'FRONTEND_RESULT',
            'SYSTEM_RESULT',
        ):
            self.assertIn(fragment, command)

    def test_each_operation_is_a_single_nested_reusable_call(self):
        expected = {
            'gcp-terraform-authenticated': (
                'terraform_authenticated',
                './.github/workflows/gcp-terraform-authenticated.yml',
                set(AUTH_SECRETS),
            ),
            'gcp-user-activity-schema-audit': (
                'gcp_user_activity_schema_audit',
                './.github/workflows/gcp-user-activity-schema-audit.yml',
                set(AUDIT_SECRETS),
            ),
            'mypage-foundation-authenticated': (
                'mypage_foundation',
                './.github/workflows/mypage-foundation-authenticated.yml',
                set(FOUNDATION_SECRETS),
            ),
            'mypage-hosting-authenticated': (
                'mypage_hosting',
                './.github/workflows/mypage-hosting-authenticated.yml',
                set(),
            ),
            'mypage-backend-authenticated': (
                'mypage_backend',
                './.github/workflows/mypage-backend-authenticated.yml',
                set(FOUNDATION_SECRETS),
            ),
        }
        self.assertEqual(set(self.workflow['jobs']), {'validate-selection', *expected})
        for name, (toggle, reusable, secrets) in expected.items():
            job = self.workflow['jobs'][name]
            with self.subTest(job=name):
                self.assertEqual(job['needs'], 'validate-selection')
                self.assertEqual(job['if'], '${{ inputs.' + toggle + ' }}')
                self.assertEqual(job['permissions'], {'contents': 'read', 'id-token': 'write'})
                self.assertEqual(job['uses'], reusable)
                if secrets:
                    self.assertEqual(set(job['secrets']), secrets)
                    self.assertEqual(
                        job['secrets'],
                        {key: '${{ secrets.' + key + ' }}' for key in secrets},
                    )
                else:
                    self.assertNotIn('secrets', job)

    def test_no_operation_can_ignore_failure(self):
        for job in self.workflow['jobs'].values():
            self.assertNotIn('continue-on-error', job)
            for step in job.get('steps', []):
                self.assertNotIn('continue-on-error', step)
