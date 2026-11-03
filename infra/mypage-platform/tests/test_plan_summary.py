"""検証済みの通常Planだけが値を含まないJob Summaryを公開する。"""
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

from test_foundation import c, ops, plan_fixture, refreshed_metadata_fixture, ROOT
from test_runtime import fixture as runtime_fixture


class PlanSummaryTest(unittest.TestCase):
    def test_current_contract_legacy_deletes_are_counted_without_publishing_identities(self):
        plan = runtime_fixture('runtime_public')
        runtime = ops.runtime_contract
        for address, values in runtime.legacy_image_release().items():
            before = dict(values)
            if address == runtime.MAINTENANCE_IMAGE_BINDING:
                before['name'] = 'mypage-maintenance-operator'
            plan['resource_changes'].append({
                'address': address, 'mode': 'managed',
                'provider_name': 'registry.terraform.io/hashicorp/google',
                'change': {'actions': ['delete'], 'before': before, 'after': None},
            })
        runtime.validate_plan(plan, 'runtime_public')
        with tempfile.TemporaryDirectory() as directory:
            summary = Path(directory) / 'summary'
            ops.publish_plan_summary({'GITHUB_STEP_SUMMARY': str(summary)}, plan, 'runtime_public')
            text = summary.read_text()
        self.assertIn('| delete | 2 |', text)
        self.assertIn('| `google_cloud_run_v2_job_iam_member` | delete | 1 |', text)
        self.assertIn('| `google_service_account_iam_member` | delete | 1 |', text)
        self.assertNotIn('mypage-backend-deploy', text)
        self.assertNotIn(c.PROJECT, text)

    def test_unknown_addresses_or_actions_never_create_a_partial_summary(self):
        for field, value in (('address', 'PRIVATE|<script>bad</script>'),
                             ('actions', ['PRIVATE|<script>bad</script>'])):
            plan = plan_fixture()
            item = plan['resource_changes'][-1]
            if field == 'address':
                item[field] = value
            else:
                item['change'][field] = value
            with self.subTest(field=field), tempfile.TemporaryDirectory() as directory:
                summary = Path(directory) / 'summary'
                with self.assertRaises(c.ContractError):
                    ops.publish_plan_summary({'GITHUB_STEP_SUMMARY': str(summary)}, plan, 'foundation')
                self.assertFalse(summary.exists())

    def test_counts_and_grouping_never_publish_plan_fields_names_or_values(self):
        for stage, plan in (('foundation', plan_fixture()),
                            ('foundation', refreshed_metadata_fixture()),
                            ('runtime_public', runtime_fixture('runtime_public'))):
            marker = 'PRIVATE|<script>untrusted</script>\n# markdown'
            plan['outputs'] = {'secret': {'value': marker}}
            plan['terraform_version'] = marker
            for item in plan['resource_changes']:
                item['type'] = marker  # Even the native type field is not trusted for publication.
                item['change']['after']['private_metadata'] = marker
            for item in plan.get('resource_drift', []):
                item['change']['before']['private_metadata'] = marker
                item['change']['after']['private_metadata'] = marker
            validator = ops.runtime_contract.validate_plan if stage == 'runtime_public' else c.validate_plan
            validator(plan, stage)
            with self.subTest(stage=stage), tempfile.TemporaryDirectory() as directory:
                summary = Path(directory) / 'summary'
                ops.publish_plan_summary({'GITHUB_STEP_SUMMARY': str(summary)}, plan, stage)
                text = summary.read_text()
                creates = sum(item['change']['actions'] == ['create'] for item in plan['resource_changes'])
                self.assertIn(f'| create | {creates} |', text)
                self.assertIn(f'| drift | {len(plan.get("resource_drift", []))} |', text)
                self.assertIn('| Resource type | Actions | Count |', text)
                self.assertIn('| `google_secret_manager_secret` |', text)
                if stage == 'runtime_public':
                    self.assertIn('| import | 1 |', text)
                    self.assertIn('| `google_cloud_run_v2_service` |', text)
                for private in (marker, c.PROJECT, c.EMAIL, *c.SECRET_IDS.values()):
                    self.assertNotIn(private, text)
                for item in plan['resource_changes']:
                    self.assertNotIn(item['address'], text)

    def test_prepare_publishes_after_all_checks_and_only_for_the_plan_role(self):
        for role, rejected, pr in (('plan', None, False), ('apply', None, False), ('plan', None, True),
                                   ('plan', 'contract', False), ('plan', 'ownership', False), ('plan', 'state', False),
                                   ('plan', 'contract', True), ('plan', 'ownership', True), ('plan', 'state', True)):
            with self.subTest(role=role, rejected=rejected, pr=pr), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                summary = root / 'summary'
                env = {'MYPAGE_ROLE': role, 'MYPAGE_MODE': 'desired-plan' if role == 'plan' else 'desired-apply',
                       'MYPAGE_STATE_ACCOUNT': '123456789012', 'MYPAGE_BACKEND_ROLE_ID': 'FIXTURE_ROLE',
                       'GITHUB_EVENT_NAME': 'pull_request' if pr else 'workflow_dispatch',
                       'GITHUB_SHA': 'a' * 40, 'MYPAGE_PLAN_HMAC_KEY': 'k' * 32,
                       'GITHUB_OUTPUT': str(root / 'output'), 'GITHUB_STEP_SUMMARY': str(summary)}
                plan = plan_fixture()
                if rejected == 'contract':
                    plan['resource_changes'][0]['change']['actions'] = ['delete']
                (root / 'foundation.tfplan').write_bytes(b'private saved plan')
                def run(args, **kwargs):
                    if args[:3] == ['aws', 'sts', 'get-caller-identity']:
                        return json.dumps({'Account': '123456789012', 'UserId': 'FIXTURE_ROLE:session',
                            'Arn': 'arn:aws:sts::123456789012:assumed-role/youtube-study-space-' + c.GITHUB_ENVIRONMENT + '-' + role + '/session'})
                    if args[:2] == ['terraform', 'version']:
                        return json.dumps({'terraform_version': (ROOT / 'infra/mypage-platform/.terraform-version').read_text().strip()})
                    return json.dumps(plan) if args[:2] == ['terraform', 'show'] else ''
                ownership_error = c.ContractError('ownership') if rejected == 'ownership' else None
                with patch.object(c, 'context'), patch.object(c, 'stage_for_mode', return_value='foundation'), \
                     patch.object(ops, 'directory', return_value=root), patch.object(ops, 'run', side_effect=run), \
                     patch.object(ops, 'state_version', side_effect=['v1', 'v2' if rejected == 'state' else 'v1']), \
                     patch.object(ops, 'reject_unowned_resources', side_effect=ownership_error), \
                     redirect_stdout(io.StringIO()):
                    if rejected:
                        with self.assertRaises(c.ContractError):
                            ops.prepare(env)
                    else:
                        ops.prepare(env)
                self.assertEqual(summary.exists(), role == 'plan' and rejected is None)
                if summary.exists():
                    self.assertNotIn('commitment', summary.read_text())
                    if pr:
                        self.assertIn('Nothing from this run can be applied', summary.read_text())
                        self.assertNotIn('| Resource type |', summary.read_text())
                        self.assertFalse((root / 'output').exists())
                        self.assertFalse((root / 'prepared.json').exists())
                    else:
                        self.assertIn('| Resource type |', summary.read_text())
                        self.assertIn('commitment=', (root / 'output').read_text())


if __name__ == '__main__':
    unittest.main()
