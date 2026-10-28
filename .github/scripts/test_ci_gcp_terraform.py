"""Contracts for the credentialless GCP Terraform validation module."""
import unittest

from test_ci_workflows import ROOT, load


class GcpTerraformWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.workflow = load(ROOT / '.github/workflows/ci-gcp-terraform.yml')
        self.caller = load(ROOT / '.github/workflows/ci.yml')

    def test_module_is_call_only_read_only_and_bounded(self):
        self.assertEqual(self.workflow['name'], 'CI GCP Terraform')
        self.assertEqual(self.workflow['on'], {'workflow_call': None})
        self.assertEqual(self.workflow['permissions'], {'contents': 'read'})
        self.assertEqual(set(self.workflow['jobs']), {'validate'})

        job = self.workflow['jobs']['validate']
        self.assertEqual(job['name'], 'GCP Terraform Validate')
        self.assertEqual(job['runs-on'], 'ubuntu-latest')
        self.assertEqual(job['timeout-minutes'], 25)
        for forbidden in ('if', 'needs', 'secrets', 'environment', 'continue-on-error', 'permissions'):
            self.assertNotIn(forbidden, job)

        for step in job['steps']:
            self.assertNotIn('continue-on-error', step)
            self.assertNotIn('environment', step)

    def test_existing_validation_surface_was_moved_without_cloud_credentials(self):
        job = self.workflow['jobs']['validate']
        commands = '\n'.join(step.get('run', '') for step in job['steps'])
        for required in (
            'test_terraform_authenticated_workflow.py',
            'test_gcp_user_activity_schema_audit_workflow.py',
            'infra/gcp/tests/test_runtime_ownership.py',
            'infra/gcp/tests/test_production_readiness.py',
            'infra/gcp/tests/test_production_backend_packet.py',
            'infra/gcp/tests/test_production_execution_contract.py',
            'terraform fmt -check -recursive infra/gcp',
            'test_terraform_normal_ops_native.py',
            'providers lock',
            'cmp infra/gcp/environments/dev/.terraform.lock.hcl infra/gcp/environments/prod/.terraform.lock.hcl',
            'bash infra/mypage-platform/scripts/check-local.sh',
            'bash infra/mypage-oauth-ttl/scripts/check-local.sh',
        ):
            self.assertIn(required, commands)

        terraform = next(
            step for step in job['steps']
            if step.get('uses', '').startswith('hashicorp/setup-terraform@')
        )
        self.assertEqual(terraform['with'], {
            'terraform_version': '1.16.4',
            'terraform_wrapper': False,
        })
        self.assertNotIn('id-token', str(self.workflow))
        self.assertNotIn('secrets.', str(self.workflow))

    def test_top_level_ci_only_routes_to_the_module(self):
        call = self.caller['jobs']['gcp-terraform-validate']
        self.assertEqual(call, {
            'name': 'GCP Terraform',
            'needs': 'changes',
            'if': "${{ needs.changes.outputs.gcp_terraform == 'true' || needs.changes.outputs.all == 'true' }}",
            'uses': './.github/workflows/ci-gcp-terraform.yml',
        })


if __name__ == '__main__':
    unittest.main()
