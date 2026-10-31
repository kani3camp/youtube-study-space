"""登録ポリシーの採用範囲、既存Auth保護、配備ソースの再現性を検証する。"""
from copy import deepcopy
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch
from contextlib import redirect_stdout
import io
import zipfile

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
import monitor_auth_registration as policy
import package_monitor_auth as source
import terraform_normal_ops as ops
import bootstrap_monitor_auth as bootstrap
import inspect_dev_monitor_auth as inspector
from urllib.parse import parse_qs, urlsplit

PROJECT = 'test-youtube-study-space'
SETTINGS = {'runtime_service_account_email': f'monitor-auth-runtime@{PROJECT}.iam.gserviceaccount.com',
            'build_service_account_email': f'monitor-auth-build@{PROJECT}.iam.gserviceaccount.com'}


class RegistrationPolicy(unittest.TestCase):
    def preflight(self, config, settings=SETTINGS, function=None):
        def read(host, path):
            return config if host.startswith('identitytoolkit') else function
        policy.preflight(settings, PROJECT, 'asia-southeast2', read)

    def test_identity_platform_and_existing_signin_must_be_verified(self):
        self.preflight({'subtype': 'IDENTITY_PLATFORM'})
        with self.assertRaises(ValueError):
            self.preflight({'subtype': 'FIREBASE_AUTH'})
        config = {'subtype': 'IDENTITY_PLATFORM', 'blockingFunctions': {'triggers': {
            'beforeSignIn': {'functionUri': 'https://existing.example.run.app'}}}}
        with self.assertRaises(ValueError):
            self.preflight(config)
        self.preflight(config, SETTINGS | {'preserved_before_sign_in_uri': 'https://existing.example.run.app'})
        with self.assertRaises(ValueError):
            self.preflight({'subtype': 'IDENTITY_PLATFORM'}, SETTINGS | {'preserved_before_sign_in_uri': 'https://stale.example.run.app'})

    def test_never_overwrite_another_before_create(self):
        config = {'subtype': 'IDENTITY_PLATFORM', 'blockingFunctions': {'triggers': {
            'beforeCreate': {'functionUri': 'https://existing.example.run.app'}}}}
        with self.assertRaises(ValueError):
            self.preflight(config, function={'serviceConfig': {'uri': 'https://ours.example.run.app'}})
        self.preflight(config, function={'serviceConfig': {'uri': 'https://existing.example.run.app'}})

    def test_preflight_requests_only_fields_needed_for_policy(self):
        calls = []
        def read(host, path):
            calls.append((host, path))
            if host == 'identitytoolkit.googleapis.com':
                return {'subtype': 'IDENTITY_PLATFORM', 'blockingFunctions': {'triggers': {
                    'beforeCreate': {'functionUri': 'https://ours.example.run.app'}}}}
            return {'serviceConfig': {'uri': 'https://ours.example.run.app'}}
        policy.preflight(SETTINGS, PROJECT, 'asia-southeast2', read)
        self.assertEqual([parse_qs(urlsplit(path).query)['fields'] for _, path in calls],
                         [['subtype,blockingFunctions.triggers'], ['serviceConfig.uri']])

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

    def test_disabled_policy_does_not_contact_auth(self):
        policy.preflight(None, PROJECT, 'asia-southeast2', lambda *args: self.fail('Authを参照してはいけません'))

    def test_bootstrap_grants_source_access_only_to_the_code_bucket(self):
        for target in ('dev', 'prod'):
            spec = bootstrap.desired(target)
            self.assertTrue(all(spec['runtime'] != email for email, _, _ in spec['grants']))
            self.assertFalse(any(role.startswith('roles/artifactregistry.') for _, role, _ in spec['grants']))
            for email, role, condition in spec['grants']:
                if role.startswith('roles/storage.'):
                    if 'gcf-v2-sources-' in condition:
                        self.assertEqual(email, spec['builder'])
                        self.assertEqual(role, 'roles/storage.objectViewer')
                        self.assertIn("resource.type == 'storage.googleapis.com/Object'", condition)
                        self.assertIn('-' + spec['region'] + '/objects/monitor-password-signup-guard/', condition)
                    else:
                        self.assertIn(spec['project'] + '-monitor-auth-source', condition)
                    self.assertNotIn('*', condition)
            permissions = {permission for values in spec['roles'].values() for permission in values}
            self.assertEqual(permissions, {'firebaseauth.configs.get', 'firebaseauth.configs.update'})
            self.assertNotIn('firebaseauth.configs.getHashConfig', permissions)

    def test_bootstrap_preview_never_changes_cloud_permissions(self):
        with patch.object(sys, 'argv', ['bootstrap_monitor_auth.py', '--target', 'dev']), \
                patch.object(bootstrap, 'provision') as apply, redirect_stdout(io.StringIO()):
            bootstrap.main()
        apply.assert_not_called()

    def test_bootstrap_refuses_a_privileged_runtime_before_grants(self):
        spec = bootstrap.desired('dev')
        def reply(args, **kwargs):
            if args[:2] == ['projects', 'describe']:
                return {'projectId': spec['project'], 'projectNumber': '123456789'}
            if args[:2] == ['projects', 'get-iam-policy']:
                return {'bindings': [{'role': 'roles/editor', 'members': ['serviceAccount:' + spec['runtime']]}]}
            return {}
        with patch.object(bootstrap, 'command', side_effect=reply) as command:
            with self.assertRaises(ValueError):
                bootstrap.provision(spec)
        self.assertFalse(any('add-iam-policy-binding' in call.args[0] for call in command.call_args_list))

    def test_bootstrap_resolves_staged_source_to_the_verified_project_number(self):
        spec = bootstrap.desired('dev')
        def reply(args, **kwargs):
            if args[:2] == ['projects', 'describe']:
                return {'projectId': spec['project'], 'projectNumber': '123456789'}
            if args[:3] == ['iam', 'roles', 'describe']:
                return None
            return {}
        with patch.object(bootstrap, 'command', side_effect=reply) as command:
            bootstrap.provision(spec)
        grants = [call.args[0] for call in command.call_args_list if 'add-iam-policy-binding' in call.args[0]]
        staged = [args for args in grants if any('gcf-v2-sources-' in arg for arg in args)]
        self.assertEqual(len(staged), 1)
        self.assertIn('--member=serviceAccount:' + spec['builder'], staged[0])
        condition = next(arg for arg in staged[0] if arg.startswith('--condition='))
        self.assertIn('gcf-v2-sources-123456789-asia-southeast2/objects/monitor-password-signup-guard/', condition)
        self.assertNotIn('PROJECT_NUMBER', condition)

    def test_bootstrap_refuses_unverified_project_metadata_before_mutation(self):
        with patch.object(bootstrap, 'command', return_value={'projectId': 'another-project', 'projectNumber': '123'}) as command:
            with self.assertRaises(ValueError):
                bootstrap.provision(bootstrap.desired('dev'))
        self.assertEqual(command.call_count, 1)

    def test_archive_is_deterministic_and_contains_only_reviewed_source(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            directory = root / 'modules/monitor-auth-registration/function'
            directory.mkdir(parents=True)
            for name in source.FILES:
                (directory / name).write_text(name)
            (directory / 'untracked-secret.json').write_text('must not be packaged')
            first = source.package(root)
            self.assertEqual(first, source.package(root))
            with zipfile.ZipFile(directory / 'source.zip') as archive:
                self.assertEqual(archive.namelist(), list(source.FILES))
            (directory / 'guard.cjs').write_text('new reviewed implementation')
            self.assertNotEqual(first, source.package(root))


if __name__ == '__main__':
    unittest.main()
