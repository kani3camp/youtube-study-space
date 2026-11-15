"""メール認証の既存Auth採用境界と秘密値を出さない構成確認を検証する。"""
from copy import deepcopy
from pathlib import Path
import sys
import unittest
from unittest.mock import patch
from contextlib import redirect_stdout
import io
from urllib.parse import parse_qs, urlsplit

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
import monitor_auth as policy
import terraform_normal_ops as ops
import inspect_dev_monitor_auth as inspector

PROJECT = 'test-youtube-study-space'


class MonitorEmailAuth(unittest.TestCase):
    def test_inspector_drops_unrequested_secrets_and_trigger_urls(self):
        config = {'subtype': 'IDENTITY_PLATFORM', 'signIn': {
            'email': {'enabled': True, 'passwordRequired': True, 'hashConfig': 'TEST_SECRET'},
            'hashConfig': {'signerKey': 'TEST_SECRET'}},
            'blockingFunctions': {'triggers': {'beforeCreate': {'functionUri': 'TEST_SECRET'}},
                                  'forwardInboundCredentials': 'TEST_SECRET'},
            'unexpected': 'TEST_SECRET'}
        self.assertEqual(inspector.summarize(config), {
            'project': PROJECT, 'subtype': 'IDENTITY_PLATFORM', 'email_enabled': True,
            'password_required': True, 'blocking_triggers': ['beforeCreate']})
        for mutate in (
            lambda c: c.update(subtype='TEST_SECRET'),
            lambda c: c['signIn']['email'].update(enabled='TEST_SECRET'),
            lambda c: c['blockingFunctions']['triggers'].update(TEST_SECRET={}),
        ):
            candidate = deepcopy(config); mutate(candidate)
            with self.assertRaises(ValueError) as failure:
                inspector.summarize(candidate)
            self.assertNotIn('TEST_SECRET', str(failure.exception))

    def test_inspector_uses_dev_read_only_request_with_leaf_field_mask(self):
        from types import SimpleNamespace
        response = unittest.mock.MagicMock()
        response.__enter__.return_value.read.return_value = b'{"subtype":"IDENTITY_PLATFORM"}'
        with patch.object(inspector.subprocess, 'run', return_value=SimpleNamespace(stdout='TEST_TOKEN\n')) as run, \
                patch.object(inspector.urllib.request, 'urlopen', return_value=response) as open_url:
            inspector.inspect()
        self.assertEqual(run.call_args.args[0], ['gcloud', 'auth', 'print-access-token'])
        self.assertTrue(run.call_args.kwargs['capture_output'])
        request = open_url.call_args.args[0]
        self.assertEqual(request.get_method(), 'GET')
        url = urlsplit(request.full_url)
        self.assertEqual(url.path, f'/admin/v2/projects/{PROJECT}/config')
        self.assertEqual(parse_qs(url.query)['fields'], [
            'subtype,signIn.email.enabled,signIn.email.passwordRequired,blockingFunctions.triggers'])

    def test_inspector_does_not_print_raw_failures_or_contact_prod(self):
        output = io.StringIO()
        with patch.object(sys, 'argv', ['inspect_dev_monitor_auth.py', '--target', 'dev']), \
                patch.object(inspector, 'inspect', side_effect=RuntimeError('TEST_SECRET')), \
                unittest.mock.patch('sys.stderr', output), redirect_stdout(output):
            self.assertEqual(inspector.main(), 1)
        self.assertNotIn('TEST_SECRET', output.getvalue())
        with patch.object(sys, 'argv', ['inspect_dev_monitor_auth.py', '--target', 'prod']), \
                patch.object(inspector, 'inspect') as inspect, unittest.mock.patch('sys.stderr', io.StringIO()):
            with self.assertRaises(SystemExit):
                inspector.main()
        inspect.assert_not_called()

    def test_import_requires_explicit_enable_exact_project_address_and_non_destructive_action(self):
        item = {'address': policy.ADDRESS, 'type': 'google_identity_platform_config', 'mode': 'managed',
                'provider_name': 'registry.terraform.io/hashicorp/google',
                'change': {'actions': ['update'], 'importing': {'id': f'projects/{PROJECT}/config'}, 'before': {}, 'after': {'project': PROJECT}}}
        plan = {'format_version': '1.2', 'terraform_version': ops.common.VERSION,
                'errored': False, 'complete': True, 'resource_changes': [item]}
        self.assertEqual(ops.validate(plan, {}, monitor_auth_enabled=True)['update'], 1)
        for mutate in (
            lambda p: p['resource_changes'][0].update(address='google_identity_platform_config.other'),
            lambda p: p['resource_changes'][0]['change'].update(importing={'id': 'projects/other/config'}),
            lambda p: p['resource_changes'][0]['change'].update(actions=['delete', 'create']),
        ):
            candidate = deepcopy(plan); mutate(candidate)
            with self.assertRaises(ValueError):
                ops.validate(candidate, {}, monitor_auth_enabled=True)
        with self.assertRaises(ValueError):
            ops.validate(plan, {})

    def test_preflight_only_reads_existing_auth_subtype(self):
        for subtype in ('FIREBASE_AUTH', 'IDENTITY_PLATFORM'):
            read = unittest.mock.Mock(return_value={'subtype': subtype})
            policy.preflight(True, PROJECT, read)
            read.assert_called_once_with('identitytoolkit.googleapis.com',
                f'admin/v2/projects/{PROJECT}/config?fields=subtype')
        with self.assertRaises(ValueError):
            policy.preflight(True, PROJECT, lambda *args: {'subtype': 'unexpected'})

    def test_disabled_auth_does_not_contact_the_api(self):
        read = unittest.mock.Mock()
        policy.preflight(False, PROJECT, read)
        read.assert_not_called()


if __name__ == '__main__':
    unittest.main()
