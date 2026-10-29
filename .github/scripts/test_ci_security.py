"""Guard only fail-closed scanner policy and the maintenance credential boundary."""
import unittest

from test_ci_workflows import ROOT, load


class SecurityPolicyTest(unittest.TestCase):
    def blocking_step(self, job, predicate):
        steps = [step for step in job['steps'] if predicate(step)]
        self.assertEqual(len(steps), 1, 'The security check must execute exactly once')
        self.assertIs(job.get('continue-on-error', False), False)
        step = steps[0]
        self.assertNotIn('if', step)
        self.assertIs(step.get('continue-on-error', False), False)
        return step

    def test_source_dependency_scans_block_vulnerable_changes(self):
        for workflow, job, command in (
            ('ci-frontend.yml', 'mypage-check', 'pnpm audit --audit-level=high'),
            ('ci-system.yml', 'go-vulnerability-check', 'govulncheck -test ./...'),
        ):
            with self.subTest(scanner=command):
                self.blocking_step(load(ROOT / '.github/workflows' / workflow)['jobs'][job],
                                   lambda step: step.get('run', '').strip() == command)

    def test_built_image_rejects_high_and_critical_findings_including_unfixed(self):
        job = load(ROOT / '.github/workflows/ci-system.yml')['jobs']['mypage-container']
        scan = self.blocking_step(job, lambda step: step.get('uses', '').startswith('aquasecurity/trivy-action@'))
        options = scan['with']
        self.assertEqual(options['image-ref'], 'mypage-api:ci')
        self.assertEqual(options['scanners'], 'vuln')
        self.assertEqual(set(options['vuln-type'].split(',')), {'os', 'library'})
        self.assertEqual(set(options['severity'].split(',')), {'HIGH', 'CRITICAL'})
        self.assertIs(options['ignore-unfixed'], False)
        self.assertEqual(str(options['exit-code']), '1')

    def test_maintenance_uses_wif_after_checkout_without_saved_credentials(self):
        workflow = load(ROOT / '.github/workflows/mypage-maintenance.yml')
        self.assertEqual(set(workflow['on']), {'workflow_dispatch'})
        self.assertNotEqual(workflow.get('permissions', {}).get('id-token'), 'write')
        job = workflow['jobs']['transition']
        self.assertEqual(job['permissions']['id-token'], 'write')
        auth = self.blocking_step(job, lambda step: step.get('uses', '').startswith('google-github-actions/auth@'))
        self.assertEqual(auth['with']['workload_identity_provider'],
                         '${{ secrets.MYPAGE_MAINTENANCE_WIF_PROVIDER }}')
        self.assertEqual(auth['with']['service_account'],
                         'mypage-maintenance-invoker@${{ steps.target.outputs.project }}.iam.gserviceaccount.com')
        for step in job['steps']:
            self.assertNotIn('credentials_json', step.get('with', {}))
        checkout = next(step for step in job['steps']
                        if step.get('uses', '').startswith('actions/checkout@'))
        self.assertLess(job['steps'].index(checkout), job['steps'].index(auth))
        self.assertIs(checkout['with']['persist-credentials'], False)
