"""Dummy-only completion checks; no canonical metadata, credentials or APIs."""
import contextlib
import copy
from datetime import datetime, timedelta, timezone
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import stat
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import urllib.error

SCRIPTS = Path(__file__).resolve().parents[1] / 'scripts'
spec = importlib.util.spec_from_file_location('completion', SCRIPTS / 'complete_codex_history_reader.py')
m = importlib.util.module_from_spec(spec); spec.loader.exec_module(m)
spec = importlib.util.spec_from_file_location('dummy_bootstrap_tests', Path(__file__).with_name('test_bootstrap_codex_metadata_readers.py'))
fixtures = importlib.util.module_from_spec(spec); spec.loader.exec_module(fixtures)
TIME = datetime(2030, 1, 1, 0, 10, tzinfo=timezone.utc)
PRIVATE = 'dummy-private-error-message-or-identity'


def fixture(base, reader):
    prior, workflow = fixtures.fixture(base)
    prior['script_sha256'] = m.PRIOR_READER_SHA256
    cloud = fixtures.FakeCloud(base, prior)
    cloud.roles['history'] = dict(copy.deepcopy(reader.HISTORY_ROLE['role']), name=reader.history_role_name(), etag='dummy-history-role-etag')
    for alias in ('pool', 'sa'):
        cloud.policies[alias] = reader.planned_policy(base, cloud.policies[alias], alias, prior['metadata_service_account_email'])
        cloud.policies[alias]['etag'] = 'dummy-after-' + alias
    manifest = {'schema_version': 1, 'kind': 'private-owner-history-reader-completion-intent',
        'source_usage': 'reviewed-partial-setup-and-target-identifiers', 'prior_result_sha256': '7' * 64,
        'prior_intent_sha256': base.sha(base.encoded(prior)), 'prior_intent': prior,
        'expected_snapshot': {'project': copy.deepcopy(cloud.project),
            'metadata_service_account': copy.deepcopy(cloud.account), 'roles': copy.deepcopy(cloud.roles),
            'policies': copy.deepcopy(cloud.policies)}, 'reviewed_revision': 'd' * 40,
        'script_sha256': m.source_hash(), 'reader_script_sha256': m.READER_SHA256,
        'base_script_sha256': m.BASE_SHA256, 'workflow_sha256': base.sha(workflow),
        'issued_utc': '2030-01-01T00:00:00Z', 'expires_utc': '2030-01-01T08:00:00Z',
        'owner_session_scope': 'current-normal-gcloud-user', 'owner_route_acknowledged': True,
        'known_iam_writers_quiescent': True}
    return manifest, cloud


class Tests(unittest.TestCase):
    def setUp(self):
        self.b, self.r = m.load_dependencies()
        self.manifest, self.cloud = fixture(self.b, self.r)
        clock = patch.object(m, 'now', return_value=TIME); self.clock = clock.start(); self.addCleanup(clock.stop)
        self.root = Path(tempfile.mkdtemp()); self.root.chmod(0o700); self.addCleanup(shutil.rmtree, self.root)

    def evidence(self):
        result = m.Evidence(self.b); self.addCleanup(shutil.rmtree, result.root); self.addCleanup(result.close); return result

    def write(self, body, token):
        status, value = self.cloud('bigquery.googleapis.com', '/bigquery/v2/' + self.r.TABLE + ':setIamPolicy',
                                   method='POST', body=body, token=token)
        return status, value, None

    def session(self, execute=True, write_request=None):
        return m.Completion(self.b, self.r, self.manifest, self.evidence(), execute=execute,
            request=self.cloud, write_request=write_request or self.write, github=lambda _: None, token='dummy-token')

    def args(self):
        path = self.root / 'intent.json'; path.write_bytes(self.b.encoded(self.manifest)); path.chmod(0o600)
        return ['--manifest-file', str(path), '--manifest-sha256', self.b.sha(path.read_bytes())]

    def capture_main(self, args, *, cloud=None):
        public = io.StringIO()
        original = m.Completion
        sessions = []
        def factory(base, reader, manifest, evidence, **kwargs):
            session = original(base, reader, manifest, evidence, request=self.cloud,
                write_request=self.write, github=lambda _: None, **kwargs)
            sessions.append(session)
            return session
        with patch.object(m, 'load_dependencies', return_value=(self.b, self.r)), \
             patch.object(m, 'Completion', side_effect=factory), contextlib.redirect_stdout(public), contextlib.redirect_stderr(public):
            status = m.main(args)
        output = public.getvalue()
        for value in (PRIVATE, 'dummy-token', self.manifest['prior_intent']['metadata_service_account_email'],
                      self.manifest['prior_intent']['project_number'], 'dummy-CEL'):
            self.assertNotIn(value, output)
        directories = [line.split('=', 1)[1] for line in output.splitlines() if line.startswith('RESULT_DIRECTORY=')]
        for path in directories: self.addCleanup(shutil.rmtree, path)
        return status, Path(directories[0]) if directories else None, sessions

    def test_default_offline_has_zero_auth_network_subprocess_and_false_observation(self):
        with patch.object(self.b, 'owner_token') as token, patch.object(self.b, 'http') as http, \
             patch.object(self.b, 'check_github') as github, patch.object(m, 'set_history_policy') as setter, \
             patch.object(self.b.subprocess, 'run') as cli:
            status, directory, sessions = self.capture_main(self.args())
        self.assertEqual(status, 0); self.assertEqual(sessions, [])
        for dependency in (token, http, github, setter, cli): dependency.assert_not_called()
        self.assertEqual(stat.S_IMODE(directory.stat().st_mode), 0o700)
        self.assertTrue(all(stat.S_IMODE(path.stat().st_mode) == 0o600 for path in directory.iterdir()))
        finish = json.loads((directory / 'journal.jsonl').read_text().splitlines()[-1])
        self.assertIn('completed_utc', finish)
        for key in ('current_owner_observed_complete', 'current_write_verified', 'prior_failed_write_reclassified_as_success',
                    'dedicated_identity_read_verified', 'fresh_full7_verified', 'migration_ready'):
            self.assertFalse(finish[key])

    def test_manifest_pins_prior_canonical_intent_scope_uid_roles_and_members(self):
        self.assertEqual(m.validate_manifest(self.b, self.r, self.manifest), self.manifest)
        for key, value in [('extra', True), ('schema_version', True), ('kind', 'wrong'), ('prior_intent_sha256', '0' * 64),
            ('reader_script_sha256', '0' * 64), ('base_script_sha256', '0' * 64), ('owner_route_acknowledged', 1),
            ('expires_utc', '2030-01-01T08:00:01Z'), ('owner_session_scope', 'service-account')]:
            with self.assertRaises(ValueError): m.validate_manifest(self.b, self.r, dict(self.manifest, **{key: value}))
        changes = [lambda v: v['expected_snapshot']['metadata_service_account'].update(uniqueId='not-numeric'),
            lambda v: v['expected_snapshot']['roles']['history']['includedPermissions'].append('bigquery.tables.getData'),
            lambda v: v['expected_snapshot']['policies']['pool']['bindings'][0]['members'].append('dummy-extra'),
            lambda v: v['expected_snapshot']['policies']['history'].update(version=3),
            lambda v: v['expected_snapshot']['policies']['history'].pop('etag'),
            lambda v: v['prior_intent'].update(script_sha256='f' * 64)]
        for change in changes:
            value = copy.deepcopy(self.manifest); change(value)
            with self.assertRaises(ValueError): m.validate_manifest(self.b, self.r, value)

    def test_arguments_wrong_source_ack_and_expiry_stop_before_auth_or_github(self):
        args = self.args()
        with patch.object(self.b, 'owner_token') as token, patch.object(self.b, 'check_github') as github:
            for extra in (['--private-dummy-argument'], ['--execute-owner-route'],
                ['--execute-owner-route', '--source-sha256', '0' * 64], ['--execute-owner-route', '--live-plan']):
                self.assertEqual(self.capture_main(args + extra)[0], 1)
            self.clock.return_value = TIME + timedelta(hours=9)
            self.assertEqual(self.capture_main(args + ['--execute-owner-route', '--source-sha256', m.source_hash()])[0], 1)
        token.assert_not_called(); github.assert_not_called()
        self.clock.return_value = TIME
        for key in ('owner_route_acknowledged', 'known_iam_writers_quiescent'):
            self.manifest[key] = False
            with patch.object(self.b, 'owner_token') as token:
                self.assertEqual(self.capture_main(self.args() + ['--execute-owner-route', '--source-sha256', m.source_hash()])[0], 1)
            token.assert_not_called(); self.manifest[key] = True

    def test_all_targets_read_before_single_policy_only_native_v1_set(self):
        session = self.session(); session.prepare()
        self.assertEqual(self.cloud.writes, [])
        authorities = [call for call in self.cloud.calls if call[1].endswith(':testIamPermissions')]
        self.assertEqual(len(authorities), 1)
        self.assertEqual(authorities[0][3], {'permissions': ['bigquery.tables.getIamPolicy', 'bigquery.tables.setIamPolicy']})
        self.assertEqual(self.cloud.calls[0][1], '/v1/projects/' + self.r.PROJECT)
        session.run()
        self.assertEqual(session.attempts, 1); self.assertTrue(session.write_verified); self.assertTrue(session.observed_complete)
        self.assertEqual(len(self.cloud.writes), 1)
        host, path, method, body = self.cloud.writes[0]
        self.assertEqual((host, path, method), ('bigquery.googleapis.com', '/bigquery/v2/' + self.r.TABLE + ':setIamPolicy', 'POST'))
        self.assertEqual(set(body), {'policy'}); self.assertEqual(body['policy']['version'], 1)
        self.assertEqual(body['policy']['etag'], self.manifest['expected_snapshot']['policies']['history']['etag'])
        self.assertTrue((session.evidence.root / 'after-final.json').exists())
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(len(self.cloud.writes), 1)

    def test_already_desired_is_new_owner_observation_with_zero_set_and_no_retcon(self):
        old = self.manifest['expected_snapshot']['policies']['history']
        self.cloud.policies['history'] = self.r.planned_policy(self.b, old, 'history', self.manifest['prior_intent']['metadata_service_account_email'])
        self.cloud.policies['history']['etag'] = 'dummy-current-desired-etag'
        with patch.object(self.b, 'owner_token', return_value='dummy-token'), patch.object(self.b, 'check_github'):
            status, directory, sessions = self.capture_main(self.args() + ['--execute-owner-route', '--source-sha256', m.source_hash()])
        self.assertEqual(status, 0); self.assertEqual(self.cloud.writes, [])
        finish = json.loads((directory / 'journal.jsonl').read_text().splitlines()[-1])
        self.assertTrue(finish['current_owner_observed_complete']); self.assertFalse(finish['current_write_verified'])
        self.assertFalse(finish['prior_failed_write_reclassified_as_success']); self.assertIsNone(sessions[0].token)

    def test_main_single_set_reports_current_observation_only_and_clears_token(self):
        with patch.object(self.b, 'owner_token', return_value='dummy-token'), patch.object(self.b, 'check_github'):
            status, directory, sessions = self.capture_main(self.args() + ['--execute-owner-route', '--source-sha256', m.source_hash()])
        self.assertEqual(status, 0); self.assertEqual(len(self.cloud.writes), 1)
        finish = json.loads((directory / 'journal.jsonl').read_text().splitlines()[-1])
        self.assertTrue(finish['current_owner_observed_complete']); self.assertTrue(finish['current_write_verified'])
        for key in ('prior_failed_write_reclassified_as_success', 'dedicated_identity_read_verified', 'fresh_full7_verified', 'migration_ready'):
            self.assertFalse(finish[key])
        self.assertIsNone(sessions[0].token); self.assertIsNone(sessions[0].read_session.token)

    def test_github_delay_expires_before_owner_token_request(self):
        def delayed(manifest): self.clock.return_value = TIME + timedelta(hours=9)
        with patch.object(self.b, 'check_github', side_effect=delayed), patch.object(self.b, 'owner_token') as token:
            status, _, _ = self.capture_main(self.args() + ['--execute-owner-route', '--source-sha256', m.source_hash()])
        self.assertEqual(status, 1); token.assert_not_called()

    def test_any_preflight_or_presend_project_sa_role_policy_drift_stops_before_set(self):
        changes = [lambda: self.cloud.project.update(lifecycleState='DELETE_REQUESTED'),
            lambda: self.cloud.account.update(uniqueId='444444444444'), lambda: self.cloud.account.update(disabled=True),
            lambda: self.cloud.roles['pool'].update(etag='dummy-drift'),
            lambda: self.cloud.roles['history']['includedPermissions'].append('bigquery.tables.getData'),
            lambda: self.cloud.policies['sa'].update(etag='dummy-drift'),
            lambda: self.cloud.policies['pool']['bindings'][0]['members'].pop(),
            lambda: self.cloud.policies['history'].update(etag='dummy-drift'),
            lambda: self.cloud.policies['history'].update(version=3),
            lambda: self.cloud.policies['history'].pop('etag')]
        for presend in (False, True):
            for change in changes:
                self.manifest, self.cloud = fixture(self.b, self.r)
                session = self.session()
                if presend: session.prepare()
                change()
                with self.assertRaises(ValueError): session.run() if presend else session.prepare()
                self.assertEqual(self.cloud.writes, []); self.assertEqual(session.attempts, 0)

    def test_missing_table_permission_and_live_plan_cannot_set(self):
        session = self.session()
        self.cloud.hook = lambda call: (200, {'permissions': []}) if call[1].endswith(':testIamPermissions') else None
        with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(self.cloud.writes, [])
        self.cloud.hook = None
        session = self.session(execute=False); session.prepare()
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(self.cloud.writes, [])
        with self.assertRaises(ValueError): session.read('role-history-create')

    def test_unknown_write_conflict_and_400_stop_without_retry_or_raw_diagnostics(self):
        responses = [(409, None, {'http_status': 409, 'error_code': 409, 'status': 'ABORTED', 'reason': 'UNKNOWN'}),
            (400, None, {'http_status': 400, 'error_code': 400, 'status': 'INVALID_ARGUMENT', 'reason': 'UNKNOWN'})]
        for response in responses:
            session = self.session(write_request=lambda body, token: response); session.prepare()
            with self.assertRaises(ValueError): session.run()
            self.assertEqual(session.attempts, 1); self.assertFalse(session.observed_complete)
            with self.assertRaises(ValueError): session.run()
            self.assertFalse((session.evidence.root / 'write-1.json').exists())
            self.assertIn('write-error-classification', (session.evidence.root / 'journal.jsonl').read_text())
        def unknown(body, token): raise TimeoutError(PRIVATE)
        session = self.session(write_request=unknown); session.prepare()
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 1)
        self.assertNotIn(PRIVATE, (session.evidence.root / 'journal.jsonl').read_text())

    def test_expired_after_durable_reservation_sends_zero_requests(self):
        requests = []
        session = self.session(write_request=lambda body, token: requests.append(body)); session.prepare()
        event = session.evidence.event
        def delayed(value):
            event(value)
            if value['event'] == 'write-attempt': self.clock.return_value = TIME + timedelta(hours=2)
        session.evidence.event = delayed
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 1); self.assertEqual(requests, [])

    def test_final_sa_or_pool_drift_keeps_verified_set_but_completion_false(self):
        for target in ('sa', 'pool'):
            self.manifest, self.cloud = fixture(self.b, self.r)
            def write(body, token):
                result = self.write(body, token)
                if target == 'sa': self.cloud.account['uniqueId'] = '555555555555'
                else: self.cloud.policies['pool']['bindings'][0]['members'].append('dummy-unexpected')
                return result
            session = self.session(write_request=write); session.prepare()
            with self.assertRaises(ValueError): session.run()
            self.assertTrue(session.write_verified); self.assertFalse(session.observed_complete)
            self.assertFalse((session.evidence.root / 'after-final.json').exists())
            self.assertEqual(len(self.cloud.writes), 1)

    def test_malformed_success_and_after_get_extra_member_are_not_verified(self):
        for after_get in (False, True):
            self.manifest, self.cloud = fixture(self.b, self.r)
            def write(body, token):
                if not after_get: return 200, {'error': {'message': PRIVATE}}, None
                result = self.write(body, token)
                self.cloud.policies['history']['bindings'][0]['members'].append('dummy-extra')
                return result
            session = self.session(write_request=write); session.prepare()
            with self.assertRaises(ValueError): session.run()
            self.assertFalse(session.write_verified)
            self.assertFalse((session.evidence.root / 'write-1.json').exists())

    def test_safe_error_classifier_discards_messages_unknowns_duplicates_and_oversize(self):
        raw = self.b.encoded({'error': {'code': 400, 'status': 'INVALID_ARGUMENT', 'message': PRIVATE,
            'details': [{'@type': 'type.googleapis.com/google.rpc.ErrorInfo', 'reason': 'INVALID_POLICY_VERSION',
                         'metadata': {'identity': PRIVATE}}]}})
        result = m.classify_error(self.b, 400, raw)
        self.assertEqual(result, {'http_status': 400, 'error_code': 400, 'status': 'INVALID_ARGUMENT', 'reason': 'INVALID_POLICY_VERSION'})
        self.assertNotIn(PRIVATE, self.b.encoded(result).decode())
        for raw in (b'x' * (m.MAX_ERROR_BYTES + 1), b'{"error":{"code":400,"code":400}}', b'{"error":NaN}',
                    self.b.encoded({'error': {'code': 400, 'status': PRIVATE, 'errors': [{'reason': PRIVATE}]}})):
            result = m.classify_error(self.b, 400, raw)
            self.assertEqual(result['reason'], 'UNKNOWN'); self.assertEqual(result['status'], 'UNKNOWN')
            self.assertNotIn(PRIVATE, self.b.encoded(result).decode())

    def test_set_transport_bounds_error_read_and_accepts_only_native_v1_policy_body(self):
        reads = []
        class Error(urllib.error.HTTPError):
            def read(self, size):
                reads.append(size)
                return b'{"error":{"code":400,"status":"INVALID_ARGUMENT","message":"' + PRIVATE.encode() + b'"}}'
        error = Error('https://bigquery.googleapis.com/dummy', 400, PRIVATE, {}, None)
        opener = SimpleNamespace(open=lambda *args, **kwargs: (_ for _ in ()).throw(error))
        body = {'policy': self.manifest['expected_snapshot']['policies']['history']}
        with patch.object(m.urllib.request, 'build_opener', return_value=opener):
            status, value, classification = m.set_history_policy(self.b, self.r, body, 'dummy-token')
        self.assertEqual((status, value), (400, None)); self.assertEqual(reads, [65537])
        self.assertNotIn(PRIVATE, repr(classification))
        for wrong in (dict(body, updateMask='bindings,etag'), {'policy': dict(body['policy'], version=3)}):
            with self.assertRaises(ValueError): m.set_history_policy(self.b, self.r, wrong, 'dummy-token')

    def test_dependency_mismatch_or_symlink_rejected_before_code_execution(self):
        script = self.root / 'complete.py'; dependency = self.root / 'bootstrap_codex_metadata_readers.py'
        dependency.write_text('raise RuntimeError("must not execute")')
        with patch.object(m, '__file__', str(script)), self.assertRaises(ValueError): m.load_dependencies()
        dependency.unlink(); dependency.symlink_to(SCRIPTS / 'bootstrap_codex_metadata_readers.py')
        with patch.object(m, '__file__', str(script)), self.assertRaises(OSError): m.load_dependencies()


if __name__ == '__main__':
    unittest.main()
