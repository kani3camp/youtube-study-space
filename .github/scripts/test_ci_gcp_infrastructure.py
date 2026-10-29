"""Contracts for the unified GCP infrastructure manual operation."""
import unittest

from test_ci_workflows import (
    ROOT,
    load,
    AUTH_SECRETS,
    FOUNDATION_SECRETS,
)


class GcpInfrastructureWorkflowTest(unittest.TestCase):
    def setUp(self):
        self.gcp = load(ROOT / '.github/workflows/gcp-infrastructure-authenticated.yml')
        self.mypage = load(ROOT / '.github/workflows/mypage-infrastructure-authenticated.yml')

    def test_gcp_orchestrator_exposes_only_environment_and_action(self):
        call = self.gcp['on']['workflow_call']
        self.assertEqual(call['inputs'], {
            'environment': {'required': True, 'type': 'string'},
            'action': {'required': True, 'type': 'string'},
        })
        self.assertEqual(self.gcp['permissions'], {'contents': 'read'})
        self.assertFalse(self.gcp['concurrency']['cancel-in-progress'])
        self.assertEqual(
            self.gcp['concurrency']['group'],
            'gcp-infrastructure-${{ inputs.environment }}',
        )

    def test_plan_covers_core_and_mypage_before_any_apply(self):
        jobs = self.gcp['jobs']
        self.assertEqual(set(jobs), {
            'validate', 'core-plan', 'mypage-plan', 'core-apply', 'mypage-apply', 'gate',
        })
        core_plan = jobs['core-plan']
        mypage_plan = jobs['mypage-plan']
        self.assertEqual(core_plan['needs'], 'validate')
        self.assertEqual(mypage_plan['needs'], 'validate')
        self.assertEqual(core_plan['uses'], './.github/workflows/gcp-terraform-authenticated.yml')
        self.assertEqual(mypage_plan['uses'], './.github/workflows/mypage-infrastructure-authenticated.yml')
        self.assertEqual(core_plan['with']['mode'], 'normal-plan')
        self.assertEqual(core_plan['with']['expected_sha'], '${{ github.sha }}')
        self.assertEqual(mypage_plan['with'], {
            'target': '${{ inputs.environment }}',
            'action': 'plan',
            'expected_sha': '${{ github.sha }}',
        })

        core_apply = jobs['core-apply']
        mypage_apply = jobs['mypage-apply']
        self.assertEqual(core_apply['needs'], ['core-plan', 'mypage-plan'])
        self.assertEqual(mypage_apply['needs'], ['core-plan', 'mypage-plan', 'core-apply'])
        self.assertEqual(core_apply['if'], "${{ inputs.action == 'apply' }}")
        self.assertEqual(mypage_apply['if'], "${{ inputs.action == 'apply' }}")
        self.assertEqual(core_apply['with']['mode'], 'normal-apply')
        self.assertEqual(mypage_apply['with']['action'], 'apply')

    def test_core_and_mypage_keep_independent_secret_boundaries(self):
        jobs = self.gcp['jobs']
        for name in ('core-plan', 'core-apply'):
            self.assertEqual(set(jobs[name]['secrets']), set(AUTH_SECRETS))
        for name in ('mypage-plan', 'mypage-apply'):
            self.assertEqual(set(jobs[name]['secrets']), set(FOUNDATION_SECRETS))
        for job in jobs.values():
            self.assertNotIn('continue-on-error', job)
            for step in job.get('steps', []):
                self.assertNotIn('continue-on-error', step)

    def test_gate_requires_both_plans_and_both_applies_when_requested(self):
        gate = self.gcp['jobs']['gate']
        self.assertEqual(gate['if'], '${{ always() }}')
        self.assertEqual(
            gate['needs'],
            ['validate', 'core-plan', 'mypage-plan', 'core-apply', 'mypage-apply'],
        )
        command = gate['steps'][0]['run']
        for variable in (
            'CORE_PLAN_RESULT', 'MYPAGE_PLAN_RESULT', 'CORE_APPLY_RESULT', 'MYPAGE_APPLY_RESULT',
        ):
            self.assertIn(variable, command)
        self.assertIn('[[ "${CORE_PLAN_RESULT}" == "success" ]]', command)
        self.assertIn('[[ "${MYPAGE_PLAN_RESULT}" == "success" ]]', command)
        self.assertIn('if [[ "${ACTION}" == "apply" ]]', command)

    def test_mypage_normal_path_maps_to_full_desired_state_only(self):
        call = self.mypage['on']['workflow_call']
        self.assertEqual(call['inputs'], {
            'target': {'required': True, 'type': 'string'},
            'action': {'required': True, 'type': 'string'},
            'expected_sha': {'required': True, 'type': 'string'},
        })
        self.assertEqual(set(self.mypage['jobs']), {'resolve', 'infrastructure', 'gate'})
        resolve = self.mypage['jobs']['resolve']
        self.assertEqual(resolve['outputs']['stage'], '${{ steps.desired.outputs.stage }}')
        self.assertIn('desired-stage.txt', resolve['steps'][-1]['run'])
        job = self.mypage['jobs']['infrastructure']
        self.assertEqual(job['uses'], './.github/workflows/mypage-foundation-authenticated.yml')
        self.assertEqual(job['needs'], 'resolve')
        self.assertEqual(job['if'], "${{ needs.resolve.outputs.stage != 'disabled' }}")
        self.assertEqual(
            job['with']['mode'],
            "${{ inputs.action == 'apply' && 'desired-apply' || 'desired-plan' }}",
        )
        # Bootstrap stages remain internal to the legacy bootstrap workflow; the
        # normal boundary resolves a reviewed per-environment desired stage.
        public = str(call['inputs'])
        for retired in (
            'foundation-metadata', 'foundation-plan', 'registry-plan',
            'maintenance-upgrade', 'runtime-private',
        ):
            self.assertNotIn(retired, public)

        gate = self.mypage['jobs']['gate']
        self.assertEqual(gate['if'], '${{ always() }}')
        command = gate['steps'][0]['run']
        self.assertIn('[[ "${INFRASTRUCTURE_RESULT}" == "skipped" ]]', command)
        self.assertIn('[[ "${INFRASTRUCTURE_RESULT}" == "success" ]]', command)
        self.assertEqual(
            (ROOT / 'infra/mypage-platform/environments/dev/desired-stage.txt').read_text().strip(),
            'runtime_public',
        )
        self.assertEqual(
            (ROOT / 'infra/mypage-platform/environments/prod/desired-stage.txt').read_text().strip(),
            'disabled',
        )


if __name__ == '__main__':
    unittest.main()
