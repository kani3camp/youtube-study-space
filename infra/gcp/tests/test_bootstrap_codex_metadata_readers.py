"""Dummy-only behavioral checks; no canonical selectors, credentials or network."""
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

PATH = Path(__file__).resolve().parents[1] / 'scripts' / 'bootstrap_codex_metadata_readers.py'
spec = importlib.util.spec_from_file_location('codex_metadata_bootstrap', PATH)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
TIME = datetime(2030, 1, 1, 0, 10, tzinfo=timezone.utc)


def fixture(b):
    number = '111111111111'
    pool = 'projects/' + number + '/locations/global/workloadIdentityPools/aws-runtime'
    email = 'dummy-metadata-reader@' + m.PROJECT + '.iam.gserviceaccount.com'
    workflow = ('env:\n' + ''.join('  ' + key + ': "false"\n' for key in b.CLOSED_GATES)
        + ''.join('  ' + key + ': ""\n' for key in b.EMPTY_TIMES)
        + '  DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED: "true"\njobs:\n').encode()
    return {'schema_version': 1, 'kind': 'private-owner-codex-metadata-reader-extension-intent',
        'source_usage': 'reviewed-target-identifiers-only', 'reviewed_capture_sha256': 'a' * 64,
        'reviewed_metadata_identity_sha256': 'c' * 64,
        'project_id': m.PROJECT, 'project_number': number, 'runtime_pool_resource': pool,
        'runtime_provider_resource': pool + '/providers/aws-provider',
        'runtime_iam_policy_resource': 'projects/' + m.PROJECT + '/serviceAccounts/' + m.PROJECT + '@appspot.gserviceaccount.com',
        'metadata_service_account_email': email,
        'metadata_service_account_resource': 'projects/' + m.PROJECT + '/serviceAccounts/' + email,
        'history_table_resource': m.TABLE,
        'ci_plan_service_account': b.fixed_members()[0].split(':', 1)[1],
        'ci_apply_service_account': b.fixed_members()[1].split(':', 1)[1],
        'existing_role_definitions': copy.deepcopy(b.ROLES), 'role_creation': copy.deepcopy(m.HISTORY_ROLE),
        'binding_additions': m.additions(b, email), 'reviewed_revision': 'b' * 40,
        'script_sha256': m.source_hash(), 'base_script_sha256': m.BASE_SHA256,
        'workflow_sha256': b.sha(workflow), 'issued_utc': '2030-01-01T00:00:00Z',
        'expires_utc': '2030-01-01T01:00:00Z', 'owner_session_scope': 'current-normal-gcloud-user',
        'owner_route_acknowledged': True, 'known_iam_writers_quiescent': True}, workflow


class FakeCloud:
    def __init__(self, b, manifest):
        self.b, self.manifest = b, manifest
        self.calls, self.writes = [], []
        self.hook = None
        self.project = {'projectId': m.PROJECT, 'projectNumber': manifest['project_number'], 'lifecycleState': 'ACTIVE'}
        self.account = {'name': manifest['metadata_service_account_resource'], 'projectId': m.PROJECT,
            'email': manifest['metadata_service_account_email'], 'uniqueId': '333333333333', 'disabled': False}
        self.roles = {alias: dict(copy.deepcopy(b.ROLES[alias]['role']), name=b.role_name(alias),
                                 etag='dummy-role-' + alias) for alias in ('pool', 'sa')}
        self.policies = {}
        for alias in ('pool', 'sa', 'history'):
            bindings = [{'role': 'roles/dummy.other', 'members': ['dummy-old-member'],
                'condition': {'title': 'dummy-title', 'expression': 'dummy-CEL', 'location': 'dummy-location'}}]
            if alias != 'history':
                bindings.insert(0, {'role': b.role_name(alias), 'members': b.fixed_members()})
            self.policies[alias] = {'version': 3, 'etag': 'dummy-policy-' + alias, 'bindings': bindings,
                'auditConfigs': [{'service': 'allServices', 'auditLogConfigs': [
                    {'logType': 'ADMIN_READ', 'exemptedMembers': ['dummy-exempt-member']}]}]}

    def __call__(self, host, path, *, method='GET', body=None, token=None, raw=False):
        call = (host, path, method, copy.deepcopy(body))
        self.calls.append(call)
        is_write = method == 'POST' and (path.endswith('/roles') or path.endswith(':setIamPolicy'))
        if is_write:
            self.writes.append(call)
        if self.hook:
            result = self.hook(call)
            if result is not None:
                return result
        if path == '/v1/projects/' + m.PROJECT:
            return 200, copy.deepcopy(self.project)
        if path == '/v1/' + self.manifest['metadata_service_account_resource']:
            return 200, copy.deepcopy(self.account)
        if path.endswith(':testIamPermissions'):
            return 200, {'permissions': copy.deepcopy(body['permissions'])}
        if path.endswith('/roles'):
            result = dict(copy.deepcopy(body['role']), name=m.history_role_name(), etag='dummy-new-history-role')
            self.roles['history'] = result
            return 200, copy.deepcopy(result)
        for alias in ('pool', 'sa', 'history'):
            name = m.history_role_name() if alias == 'history' else self.b.role_name(alias)
            if path == '/v1/' + name:
                return (200, copy.deepcopy(self.roles[alias])) if alias in self.roles else (404, None)
        target = 'history' if host == 'bigquery.googleapis.com' else 'pool' if '/workloadIdentityPools/' in path else 'sa'
        if ':getIamPolicy' in path:
            return 200, copy.deepcopy(self.policies[target])
        if path.endswith(':setIamPolicy'):
            result = dict(copy.deepcopy(body['policy']), etag='dummy-after-' + str(len(self.writes)))
            self.policies[target] = result
            return 200, copy.deepcopy(result)
        raise AssertionError('unexpected dummy API interface')


class Tests(unittest.TestCase):
    def setUp(self):
        self.b = m.load_base()
        self.manifest, self.workflow = fixture(self.b)
        clock = patch.object(m, 'now', return_value=TIME)
        self.clock = clock.start()
        self.addCleanup(clock.stop)
        self.root = Path(tempfile.mkdtemp())
        self.root.chmod(0o700)
        self.addCleanup(shutil.rmtree, self.root)

    def evidence(self):
        evidence = m.Evidence(self.b)
        self.addCleanup(shutil.rmtree, evidence.root)
        self.addCleanup(evidence.close)
        return evidence

    def session(self, execute=True):
        self.cloud = FakeCloud(self.b, self.manifest)
        self.gates = []
        return m.Bootstrap(self.b, self.manifest, self.evidence(), execute=execute, request=self.cloud,
            github=lambda manifest: self.gates.append('checked'), token='dummy-secret-token')

    def file_args(self):
        path = self.root / 'intent.json'
        path.write_bytes(self.b.encoded(self.manifest))
        path.chmod(0o600)
        return ['--manifest-file', str(path), '--manifest-sha256', self.b.sha(path.read_bytes())]

    def capture_main(self, args):
        public = io.StringIO()
        with patch.object(m, 'load_base', return_value=self.b), contextlib.redirect_stdout(public), contextlib.redirect_stderr(public):
            result = m.main(args)
        output = public.getvalue()
        for value in ('dummy-secret-token', 'dummy-CEL', 'dummy-old-member', self.manifest['project_number'],
                      self.manifest['metadata_service_account_email'], self.manifest['runtime_pool_resource']):
            self.assertNotIn(value, output)
        directories = [line.split('=', 1)[1] for line in output.splitlines() if line.startswith('RESULT_DIRECTORY=')]
        for directory in directories:
            self.addCleanup(shutil.rmtree, directory)
        return result, output, Path(directories[0]) if directories else None

    def test_offline_has_no_auth_http_github_subprocess_and_private_timestamped_output(self):
        args = self.file_args()
        with patch.object(self.b, 'owner_token') as token, patch.object(self.b, 'http') as http, \
             patch.object(self.b, 'check_github') as github, patch.object(m, 'bq_http') as bq, \
             patch.object(self.b.subprocess, 'run') as subprocess:
            result, output, directory = self.capture_main(args)
        self.assertEqual(result, 0)
        for dependency in (token, http, github, bq, subprocess): dependency.assert_not_called()
        self.assertIn('write-attempts: 0', output)
        self.assertEqual(stat.S_IMODE(directory.stat().st_mode), 0o700)
        self.assertTrue(all(stat.S_IMODE(path.stat().st_mode) == 0o600 for path in directory.iterdir()))
        events = [json.loads(line) for line in (directory / 'journal.jsonl').read_text().splitlines()]
        self.assertIn('started_utc', events[0]); self.assertIn('completed_utc', events[-1])
        self.assertTrue(all(event['timestamp_source'] == 'local-clock-self-report' for event in events))
        for key in ('setup_complete', 'dedicated_identity_read_verified', 'fresh_full7_verified',
                    'owner_identity_independently_verified', 'all_writers_quiescence_proven', 'migration_ready'):
            self.assertIs(events[-1][key], False)

    def test_pinned_dependency_rejects_changed_or_symlinked_bytes_before_execution(self):
        fake_source = self.root / 'source.py'
        base = self.root / 'bootstrap_runtime_readers.py'
        marker = self.root / 'must-not-exist'
        base.write_text('open(' + repr(str(marker)) + ', "w").write("unsafe")\n')
        with patch.object(m, '__file__', str(fake_source)):
            with self.assertRaises(ValueError): m.load_base()
        self.assertFalse(marker.exists())
        base.unlink(); base.symlink_to(PATH.with_name('bootstrap_runtime_readers.py'))
        with patch.object(m, '__file__', str(fake_source)):
            with self.assertRaises(OSError): m.load_base()

    def test_fixed_manifest_targets_roles_permissions_members_and_approval_types(self):
        self.assertEqual(m.validate_manifest(self.b, self.manifest), self.manifest)
        bad = [{'unexpected': 1}, {'schema_version': True}, {'project_id': 'dummy-prod'},
            {'project_number': '1/../../other'}, {'history_table_resource': m.TABLE + '-other'},
            {'runtime_pool_resource': self.manifest['runtime_pool_resource'] + '-other'},
            {'runtime_provider_resource': self.manifest['runtime_provider_resource'] + '-other'},
            {'runtime_iam_policy_resource': self.manifest['runtime_iam_policy_resource'] + '-other'},
            {'metadata_service_account_email': 'dummy@other-project.iam.gserviceaccount.com'},
            {'metadata_service_account_resource': self.manifest['runtime_iam_policy_resource']},
            {'ci_plan_service_account': 'dummy-other'}, {'ci_apply_service_account': 'dummy-other'},
            {'owner_route_acknowledged': 1}, {'known_iam_writers_quiescent': 1},
            {'base_script_sha256': '0' * 64}, {'source_usage': 'fresh-old-snapshot'},
            {'owner_session_scope': 'service-account'}, {'expires_utc': '2030-01-01T08:00:01Z'}]
        for change in bad:
            with self.assertRaises(ValueError): m.validate_manifest(self.b, dict(self.manifest, **change))
        for email in (self.b.fixed_members()[0].split(':', 1)[1], self.b.fixed_members()[1].split(':', 1)[1],
                      m.PROJECT + '@appspot.gserviceaccount.com'):
            value = dict(self.manifest, metadata_service_account_email=email,
                metadata_service_account_resource='projects/' + m.PROJECT + '/serviceAccounts/' + email,
                binding_additions=m.additions(self.b, email))
            with self.assertRaises(ValueError): m.validate_manifest(self.b, value)
        for field in ('role_creation', 'existing_role_definitions', 'binding_additions'):
            value = copy.deepcopy(self.manifest)
            if field == 'role_creation': value[field]['role']['includedPermissions'].append('bigquery.tables.getData')
            elif field == 'existing_role_definitions': value[field]['pool']['role']['includedPermissions'].append('iam.roles.update')
            else: value[field][0]['members_to_add'].append('dummy-extra-member')
            with self.assertRaises(ValueError): m.validate_manifest(self.b, value)

    def test_private_parser_and_missing_or_wrong_pins_stop_without_auth(self):
        args = self.file_args()
        failures = [[], args + ['--dummy-private-argument'], args + ['--accept-empty-pool-no-cas'],
            args + ['--execute-owner-route'], args + ['--execute-owner-route', '--source-sha256', '0' * 64],
            args + ['--execute-owner-route', '--source-sha256', 'not-hex'],
            args + ['--live-plan', '--execute-owner-route']]
        with patch.object(self.b, 'owner_token') as token, patch.object(self.b, 'check_github') as github:
            for argv in failures:
                result, output, _ = self.capture_main(argv)
                self.assertEqual(result, 1)
                self.assertNotIn('dummy-private-argument', output)
                self.assertNotIn(args[1], output)
        token.assert_not_called(); github.assert_not_called()

    def test_false_ack_or_quiescence_and_bad_source_stop_before_auth(self):
        for field in ('owner_route_acknowledged', 'known_iam_writers_quiescent', 'script_sha256'):
            before = self.manifest[field]
            self.manifest[field] = '0' * 64 if field == 'script_sha256' else False
            args = self.file_args()
            with patch.object(self.b, 'owner_token') as token, patch.object(self.b, 'check_github') as github:
                result, _, _ = self.capture_main(args + ['--execute-owner-route', '--source-sha256', m.source_hash()])
            self.assertEqual(result, 1); token.assert_not_called(); github.assert_not_called()
            self.manifest[field] = before

    def test_expiry_in_github_check_stops_before_token(self):
        args = self.file_args()
        def delayed(manifest): self.clock.return_value = TIME + timedelta(hours=2)
        with patch.object(self.b, 'check_github', side_effect=delayed), patch.object(self.b, 'owner_token') as token:
            self.assertEqual(self.capture_main(args + ['--live-plan', '--source-sha256', m.source_hash()])[0], 1)
        token.assert_not_called()

    def test_all_preflight_reads_and_exact_table_methods_finish_before_any_write(self):
        session = self.session()
        session.prepare()
        self.assertEqual(self.cloud.writes, [])
        self.assertEqual(self.cloud.calls[0][1], '/v1/projects/' + m.PROJECT)
        self.assertEqual(self.cloud.calls[1][1], '/v1/' + self.manifest['metadata_service_account_resource'])
        bq = [call for call in self.cloud.calls if call[0] == 'bigquery.googleapis.com']
        self.assertEqual({call[1].split(':')[-1] for call in bq}, {'getIamPolicy', 'testIamPermissions'})
        get = next(call for call in bq if call[1].endswith(':getIamPolicy'))
        self.assertEqual(get[2:], ('POST', {'options': {'requestedPolicyVersion': 3}}))
        sa = next(call for call in self.cloud.calls if ':getIamPolicy?' in call[1])
        self.assertEqual(sa[2:], ('POST', None))

    def test_project_or_metadata_sa_mismatch_stops_before_policy_reads(self):
        changes = [('project', 'projectNumber', '222222222222'), ('project', 'lifecycleState', 'DELETE_REQUESTED'),
            ('account', 'disabled', True), ('account', 'email', 'dummy-wrong'),
            ('account', 'projectId', 'dummy-prod'), ('account', 'name', 'dummy-wrong')]
        for container, field, value in changes:
            session = self.session(); getattr(self.cloud, container)[field] = value
            with self.assertRaises(ValueError): session.prepare()
            self.assertEqual(self.cloud.writes, [])
            self.assertFalse(any(':getIamPolicy' in call[1] for call in self.cloud.calls))

    def test_every_policy_requires_iam_etag_even_raw_empty_or_table_metadata_etag(self):
        for alias in ('pool', 'sa', 'history'):
            for empty in (False, True):
                session = self.session()
                if empty: self.cloud.policies[alias] = {}
                else: self.cloud.policies[alias].pop('etag')
                with self.assertRaises(ValueError): session.prepare()
                self.assertEqual(self.cloud.writes, [])
        session = self.session()
        self.cloud.policies['history'] = {'kind': 'bigquery#table', 'etag': 'dummy-table-metadata-etag'}
        with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(self.cloud.writes, [])

    def test_existing_roles_must_match_every_field_and_new_role_must_be_absent(self):
        for alias in ('pool', 'sa'):
            session = self.session(); self.cloud.roles[alias]['includedPermissions'].append('iam.roles.update')
            with self.assertRaises(ValueError): session.prepare()
            self.assertEqual(self.cloud.writes, [])
        session = self.session()
        self.cloud.roles['history'] = dict(copy.deepcopy(m.HISTORY_ROLE['role']), name=m.history_role_name(), etag='dummy-collision')
        with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(self.cloud.writes, [])

    def test_existing_binding_requires_exact_ci_two_members_unconditioned_once(self):
        for alias in ('pool', 'sa'):
            for change in ('missing', 'extra', 'already', 'conditional', 'duplicate'):
                session = self.session(); binding = self.cloud.policies[alias]['bindings'][0]
                if change == 'missing': binding['members'].pop()
                if change == 'extra': binding['members'].append('dummy-unreviewed-member')
                if change == 'already': binding['members'].append('serviceAccount:' + self.manifest['metadata_service_account_email'])
                if change == 'conditional': binding['condition'] = {'title': 'dummy', 'expression': 'true'}
                if change == 'duplicate': self.cloud.policies[alias]['bindings'].append(copy.deepcopy(binding))
                with self.assertRaises(ValueError): session.prepare()
                self.assertEqual(self.cloud.writes, [])
        session = self.session()
        self.cloud.policies['history']['bindings'].append({'role': m.history_role_name(), 'members': ['dummy-collision']})
        with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(self.cloud.writes, [])

    def test_missing_any_permission_or_http_error_fails_all_preflight_before_write(self):
        for suffix in (':testIamPermissions', ':getIamPolicy'):
            session = self.session()
            self.cloud.hook = lambda call: (403, None) if suffix in call[1] else None
            with self.assertRaises(ValueError): session.prepare()
            self.assertEqual(self.cloud.writes, [])
        session = self.session()
        self.cloud.hook = lambda call: (200, {'permissions': []}) if call[1].endswith(':testIamPermissions') else None
        with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(self.cloud.writes, [])

    def test_four_exact_writes_keep_other_members_conditions_and_audit_configs(self):
        session = self.session(); session.prepare()
        before = copy.deepcopy(self.cloud.policies)
        session.run()
        self.assertEqual(session.attempts, 4)
        self.assertEqual(session.completed, ['role-history-create', 'pool-set', 'sa-set', 'history-set'])
        self.assertEqual(len(self.cloud.writes), 4)
        self.assertEqual(self.cloud.writes[0][3], m.HISTORY_ROLE)
        for alias in ('pool', 'sa', 'history'):
            expected = m.planned_policy(self.b, before[alias], alias, self.manifest['metadata_service_account_email'])
            self.assertEqual(self.b.comparable_policy(self.cloud.policies[alias]), self.b.comparable_policy(expected))
            self.assertEqual(self.cloud.policies[alias]['auditConfigs'], before[alias]['auditConfigs'])
            self.assertEqual(self.cloud.policies[alias]['bindings'][-1 if alias != 'history' else 0],
                             before[alias]['bindings'][-1 if alias != 'history' else 0])
        self.assertEqual(self.cloud.writes[-1][:3], ('bigquery.googleapis.com', '/bigquery/v2/' + m.TABLE + ':setIamPolicy', 'POST'))
        self.assertEqual(self.cloud.writes[-1][3]['updateMask'], 'bindings,etag')
        self.assertTrue(all('/datasets/' not in call[1] or call[1].startswith('/bigquery/v2/' + m.TABLE + ':') for call in self.cloud.calls))
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(len(self.cloud.writes), 4)

    def test_live_plan_cannot_mutate_and_repeated_run_cannot_add_fifth_attempt(self):
        session = self.session(execute=False); session.prepare()
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 0); self.assertEqual(self.cloud.writes, [])

    def test_concurrent_full_policy_or_etag_change_stops_before_next_set(self):
        for alias in ('pool', 'sa', 'history'):
            for field in ('etag', 'member'):
                session = self.session(); session.prepare()
                if field == 'etag': self.cloud.policies[alias]['etag'] += '-drift'
                else: self.cloud.policies[alias]['bindings'][0]['members'].append('dummy-drift-member')
                with self.assertRaises(ValueError): session.run()
                expected = {'pool': 1, 'sa': 2, 'history': 3}[alias]
                self.assertEqual(len(self.cloud.writes), expected)
                self.assertEqual(session.attempts, expected)

    def test_role_drift_is_rechecked_before_policy_set(self):
        session = self.session(); session.prepare()
        self.cloud.roles['pool']['includedPermissions'].append('iam.roles.update')
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 1)
        self.assertEqual(len(self.cloud.writes), 1)

    def test_same_email_recreated_sa_during_invocation_stops_before_first_write(self):
        session = self.session(); session.prepare()
        self.cloud.account['uniqueId'] = '444444444444'
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 0); self.assertEqual(self.cloud.writes, [])

    def test_each_unknown_write_stops_without_retry_and_preserves_partial_journal(self):
        for fail_at in range(1, 5):
            session = self.session(); session.prepare()
            def unknown(call):
                if len(self.cloud.writes) == fail_at and (call[1].endswith(':setIamPolicy') or call[1].endswith('/roles')):
                    raise TimeoutError('dummy-secret-error')
            self.cloud.hook = unknown
            with self.assertRaises(ValueError): session.run()
            self.assertEqual(session.attempts, fail_at)
            self.assertEqual(len(self.cloud.writes), fail_at)
            self.assertEqual(len(session.completed), fail_at - 1)
            journal = (session.evidence.root / 'journal.jsonl').read_text()
            self.assertIn('write-unconfirmed', journal)
            self.assertNotIn('dummy-secret-error', journal)
            with self.assertRaises(ValueError): session.run()
            self.assertEqual(len(self.cloud.writes), fail_at)

    def test_conflict_and_malformed_success_are_not_retried_or_saved(self):
        for response in ((409, None), (200, {'error': {'message': 'dummy-secret-error'}})):
            session = self.session(); session.prepare()
            self.cloud.hook = lambda call: response if call[1].endswith('/roles') else None
            with self.assertRaises(ValueError): session.run()
            self.assertEqual(session.attempts, 1); self.assertEqual(len(self.cloud.writes), 1)
            self.assertFalse((session.evidence.root / 'write-1.json').exists())
            self.assertFalse(any('dummy-secret-error' in path.read_text() for path in session.evidence.root.iterdir()))

    def test_unexpected_after_policy_delta_stops_and_is_not_saved_as_verified(self):
        session = self.session(); session.prepare()
        def extra(call):
            if call[1].endswith(':setIamPolicy'):
                result = dict(copy.deepcopy(call[3]['policy']), etag='dummy-new')
                result['bindings'][0]['members'].append('dummy-unexpected-after')
                return 200, result
        self.cloud.hook = extra
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.completed, ['role-history-create'])
        self.assertFalse((session.evidence.root / 'write-2.json').exists())

    def test_post_set_get_drift_cannot_be_reported_as_verified(self):
        session = self.session(); session.prepare()
        def drift(call):
            if len(self.cloud.writes) == 2 and ':getIamPolicy' in call[1]:
                value = copy.deepcopy(self.cloud.policies['pool'])
                value['bindings'][0]['members'].append('dummy-after-get-drift')
                return 200, value
        self.cloud.hook = drift
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.completed, ['role-history-create'])
        self.assertEqual(session.attempts, 2)
        self.assertFalse((session.evidence.root / 'after-pool-policy.json').exists())

    def test_main_execute_uses_reviewed_packet_reports_setup_only_and_clears_token(self):
        args = self.file_args()
        cloud = FakeCloud(self.b, self.manifest)
        original = m.Bootstrap
        sessions = []
        def factory(base, manifest, evidence, **kwargs):
            session = original(base, manifest, evidence, request=cloud, github=lambda _: None, **kwargs)
            sessions.append(session)
            return session
        with patch.object(self.b, 'owner_token', return_value='dummy-secret-token'), \
             patch.object(self.b, 'check_github'), patch.object(m, 'Bootstrap', side_effect=factory):
            result, output, directory = self.capture_main(args + ['--execute-owner-route', '--source-sha256', m.source_hash()])
        self.assertEqual(result, 0)
        self.assertEqual(len(cloud.writes), 4)
        self.assertIsNone(sessions[0].token)
        finish = json.loads((directory / 'journal.jsonl').read_text().splitlines()[-1])
        self.assertTrue(finish['setup_complete'])
        self.assertFalse(finish['dedicated_identity_read_verified'])
        self.assertFalse(finish['fresh_full7_verified']); self.assertFalse(finish['migration_ready'])
        self.assertEqual(len(finish['verified_write_aliases']), 4)
        self.assertIn('completed_utc', finish)
        after = json.loads((directory / 'after-final.json').read_text())
        self.assertEqual(set(after['policies']), {'pool', 'sa', 'history'})

    def final_drift_main(self, target):
        args = self.file_args()
        cloud = FakeCloud(self.b, self.manifest)
        original = m.Bootstrap
        changed = False
        def drift(call):
            nonlocal changed
            if len(cloud.writes) == 4 and call[1] == '/v1/' + self.b.role_name('pool') and not changed:
                changed = True
                if target == 'policy':
                    cloud.policies['pool']['bindings'][0]['members'].append('dummy-final-drift')
                else:
                    cloud.account['uniqueId'] = '555555555555'
        cloud.hook = drift
        def factory(base, manifest, evidence, **kwargs):
            return original(base, manifest, evidence, request=cloud, github=lambda _: None, **kwargs)
        with patch.object(self.b, 'owner_token', return_value='dummy-secret-token'), \
             patch.object(self.b, 'check_github'), patch.object(m, 'Bootstrap', side_effect=factory):
            result, _, directory = self.capture_main(args + ['--execute-owner-route', '--source-sha256', m.source_hash()])
        self.assertEqual(result, 1)
        self.assertEqual(len(cloud.writes), 4)
        finish = json.loads((directory / 'journal.jsonl').read_text().splitlines()[-1])
        self.assertEqual(finish['status'], 'STOP')
        self.assertFalse(finish['setup_complete'])
        self.assertEqual(finish['write_attempts'], 4)
        self.assertEqual(len(finish['verified_write_aliases']), 4)
        self.assertFalse((directory / 'after-final.json').exists())

    def test_final_policy_drift_after_all_four_writes_stops_setup(self):
        self.final_drift_main('policy')

    def test_final_same_email_sa_recreation_after_all_four_writes_stops_setup(self):
        self.final_drift_main('account')

    def test_expiry_after_durable_attempt_reservation_stops_before_http(self):
        session = self.session(); session.prepare()
        event = session.evidence.event
        def delayed(value):
            event(value)
            if value['event'] == 'write-attempt': self.clock.return_value = TIME + timedelta(hours=2)
        session.evidence.event = delayed
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 1); self.assertEqual(session.completed, [])
        self.assertEqual(self.cloud.writes, [])
        self.assertIn('write-unconfirmed', (session.evidence.root / 'journal.jsonl').read_text())

    def test_one_hour_window_cannot_restart_after_reads_or_saves(self):
        session = self.session()
        self.clock.return_value = TIME + timedelta(hours=1)
        with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(self.cloud.calls, [])
        self.clock.return_value = TIME
        session = self.session(); session.prepare()
        save = session.evidence.save
        def delayed(name, value):
            result = save(name, value)
            self.clock.return_value = TIME + timedelta(hours=2)
            return result
        session.evidence.save = delayed
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.completed, [])
        self.assertEqual(len(self.cloud.writes), 1)

    def test_bq_transport_fixed_host_path_error_body_and_redirect_boundary(self):
        class Error(urllib.error.HTTPError):
            def read(self, *args): raise AssertionError('must not read error body')
        error = Error('https://bigquery.googleapis.com/dummy', 403, 'dummy-secret-error', {}, None)
        opener = SimpleNamespace(open=lambda *args, **kwargs: (_ for _ in ()).throw(error))
        path = '/bigquery/v2/' + m.TABLE + ':getIamPolicy'
        with patch.object(m.urllib.request, 'build_opener', return_value=opener):
            self.assertEqual(m.bq_http(self.b, path, method='POST', body={'options': {'requestedPolicyVersion': 3}}, token='dummy-token'), (403, None))
        for invalid in (path + '?other=1', path.replace('firestore_export', 'other'), '/bigquery/v2/projects/other:testIamPermissions'):
            with self.assertRaises(ValueError): m.bq_http(self.b, invalid, method='POST', body={}, token='dummy-token')
        with self.assertRaises(ValueError): m.bq_http(self.b, path, method='GET', body={}, token='dummy-token')
        with self.assertRaises(ValueError): self.b.NoRedirect().redirect_request(None, None, 302, '', {}, 'https://dummy.invalid')

    def test_known_closed_gates_and_active_runs_fail_before_owner_auth(self):
        def github(host, path, **kwargs):
            if host == 'raw.githubusercontent.com': return 200, self.workflow
            if '/git/ref/' in path: return 200, {'ref': 'refs/heads/' + self.b.BRANCH, 'object': {'type': 'commit', 'sha': self.manifest['reviewed_revision']}}
            self.assertIn('/actions/workflows/ci.yml/runs?', path)
            return 200, {'total_count': 0, 'workflow_runs': []}
        self.b.check_github(self.manifest, request=github)
        def active(host, path, **kwargs):
            if '/runs?' in path: return 200, {'total_count': 1, 'workflow_runs': [{'id': 1}]}
            return github(host, path, **kwargs)
        with self.assertRaises(ValueError): self.b.check_github(self.manifest, request=active)
        changed = self.workflow.replace(b'DEV_RUNTIME_METADATA_READ_ENABLED: "false"', b'DEV_RUNTIME_METADATA_READ_ENABLED: "true"')
        value = dict(self.manifest, workflow_sha256=self.b.sha(changed))
        def open_gate(host, path, **kwargs):
            return (200, changed) if host == 'raw.githubusercontent.com' else github(host, path, **kwargs)
        with self.assertRaises(ValueError): self.b.check_github(value, request=open_gate)


if __name__ == '__main__':
    unittest.main()
