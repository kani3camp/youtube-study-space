"""Contracts for the simplified authenticated operations workflow."""
import unittest
from test_ci_workflows import (
    ROOT,
    load,
    AUDIT_SECRETS,
    FOUNDATION_SECRETS,
    GCP_INFRA_SECRETS,
)


class OperationsWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.workflow = load(ROOT / '.github/workflows/ci-operations.yml')

    def test_workflow_is_call_only_and_uses_one_operation_selector(self):
        self.assertEqual(set(self.workflow['on']), {'workflow_call'})
        self.assertEqual(self.workflow['permissions'], {'contents': 'read'})
        call = self.workflow['on']['workflow_call']
        self.assertEqual(call['inputs']['operation'], {'required': True, 'type': 'string'})
        self.assertEqual(call['inputs']['environment'], {'required': True, 'type': 'string'})
        for result in ('repository_result', 'frontend_result', 'system_result',
                       'gcp_terraform_validate_result'):
            self.assertEqual(call['inputs'][result], {'required': True, 'type': 'string'})
        self.assertEqual(set(call['inputs']), {
            'operation', 'environment', 'repository_result', 'frontend_result',
            'system_result', 'gcp_terraform_validate_result',
        })

        validator = self.workflow['jobs']['validate-selection']
        self.assertGreater(validator['timeout-minutes'], 0)
        command = validator['steps'][0]['run']
        for fragment in (
            'gcp-plan|gcp-apply',
            'gcp-user-activity-schema-audit',
            'mypage-image-build',
            'mypage-hosting-release',
            'mypage-backend-release',
            'REPOSITORY_RESULT',
            'GCP_TERRAFORM_VALIDATE_RESULT',
            'FRONTEND_RESULT',
            'SYSTEM_RESULT',
        ):
            self.assertIn(fragment, command)
        for retired in ('selected=$((selected + 1))', 'mypage_foundation', 'terraform_authenticated'):
            self.assertNotIn(retired, command)

    def test_gcp_infrastructure_replaces_direct_terraform_and_foundation_calls(self):
        jobs = self.workflow['jobs']
        self.assertEqual(set(jobs), {
            'validate-selection',
            'gcp-infrastructure',
            'gcp-user-activity-schema-audit',
            'mypage-image-build',
            'mypage-hosting-authenticated',
            'mypage-backend-authenticated',
        })
        self.assertNotIn('gcp-terraform-authenticated', jobs)
        self.assertNotIn('mypage-foundation-authenticated', jobs)

        job = jobs['gcp-infrastructure']
        self.assertEqual(job['needs'], 'validate-selection')
        self.assertEqual(
            job['if'],
            "${{ inputs.operation == 'gcp-plan' || inputs.operation == 'gcp-apply' }}",
        )
        self.assertEqual(job['permissions'], {'contents': 'read', 'id-token': 'write'})
        self.assertEqual(job['uses'], './.github/workflows/gcp-infrastructure-authenticated.yml')
        self.assertEqual(job['with'], {
            'environment': '${{ inputs.environment }}',
            'action': "${{ inputs.operation == 'gcp-apply' && 'apply' || 'plan' }}",
        })
        self.assertEqual(set(job['secrets']), GCP_INFRA_SECRETS)

    def test_non_infrastructure_operations_remain_bounded(self):
        jobs = self.workflow['jobs']
        audit = jobs['gcp-user-activity-schema-audit']
        self.assertEqual(audit['if'], "${{ inputs.operation == 'gcp-user-activity-schema-audit' }}")
        self.assertEqual(set(audit['secrets']), set(AUDIT_SECRETS))
        self.assertEqual(audit['with'], {'target': 'dev'})

        image = jobs['mypage-image-build']
        self.assertEqual(image['if'], "${{ inputs.operation == 'mypage-image-build' }}")
        self.assertEqual(image['uses'], './.github/workflows/mypage-foundation-authenticated.yml')
        self.assertEqual(set(image['secrets']), set(FOUNDATION_SECRETS))
        self.assertEqual(image['with'], {
            'target': '${{ inputs.environment }}',
            'mode': 'image-build',
            'expected_sha': '${{ github.sha }}',
        })

        hosting = jobs['mypage-hosting-authenticated']
        self.assertEqual(hosting['if'], "${{ inputs.operation == 'mypage-hosting-release' }}")
        self.assertNotIn('secrets', hosting)
        self.assertEqual(hosting['with'], {
            'expected_sha': '${{ github.sha }}',
            'other_operation': False,
        })

        backend = jobs['mypage-backend-authenticated']
        self.assertEqual(backend['if'], "${{ inputs.operation == 'mypage-backend-release' }}")
        self.assertEqual(set(backend['secrets']), set(FOUNDATION_SECRETS))
        self.assertEqual(backend['with'], {
            'expected_sha': '${{ github.sha }}',
            'other_operation': False,
        })

    def test_no_operation_can_ignore_failure(self):
        for job in self.workflow['jobs'].values():
            self.assertNotIn('continue-on-error', job)
            for step in job.get('steps', []):
                self.assertNotIn('continue-on-error', step)
