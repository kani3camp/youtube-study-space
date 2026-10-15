"""Dummy-only behavioral checks for the fixed owner bootstrap boundary."""
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

PATH = Path(__file__).resolve().parents[1] / 'scripts' / 'bootstrap_runtime_readers.py'
spec = importlib.util.spec_from_file_location('bootstrap', PATH)
m = importlib.util.module_from_spec(spec)
spec.loader.exec_module(m)
TIME = datetime(2030, 1, 1, 0, 10, tzinfo=timezone.utc)


def fixture():
    number = '111111111111'
    pool = 'projects/' + number + '/locations/global/workloadIdentityPools/aws-runtime'
    workflow = ('env:\n' + ''.join('  ' + key + ': "false"\n' for key in m.CLOSED_GATES)
        + ''.join('  ' + key + ': ""\n' for key in m.EMPTY_TIMES)
        + '  DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED: "true"\njobs:\n') .encode()
    manifest = {'schema_version': 1, 'kind': 'private-owner-runtime-reader-bootstrap-intent',
        'source_usage': 'reviewed-target-identifiers-only', 'reviewed_capture_sha256': 'a' * 64,
        'project_id': m.PROJECT, 'project_number': number, 'runtime_pool_resource': pool,
        'runtime_provider_resource': pool + '/providers/aws-provider',
        'runtime_iam_policy_resource': 'projects/' + m.PROJECT + '/serviceAccounts/' + m.PROJECT + '@appspot.gserviceaccount.com',
        'ci_plan_service_account': m.fixed_members()[0].split(':', 1)[1],
        'ci_apply_service_account': m.fixed_members()[1].split(':', 1)[1],
        'reviewed_revision': 'b' * 40, 'script_sha256': m.source_hash(), 'workflow_sha256': m.sha(workflow),
        'issued_utc': '2030-01-01T00:00:00Z', 'expires_utc': '2030-01-01T01:00:00Z',
        'owner_session_scope': 'current-normal-gcloud-user', 'owner_exception_approved': True,
        'known_iam_writers_quiescent': True, 'allow_empty_pool_policy_initialization': True,
        'role_creations': copy.deepcopy(m.ROLES), 'binding_additions': m.additions()}
    return manifest, workflow


class FakeCloud:
    def __init__(self, manifest, *, empty=True):
        self.manifest = manifest
        self.calls = []
        self.writes = []
        self.roles = {}
        self.pool_policy = {} if empty else {'version': 1, 'etag': 'dummy-pool-before',
            'bindings': [{'role': 'roles/dummy.reader', 'members': ['dummy-old-pool-member']}]}
        self.sa_policy = {'version': 3, 'etag': 'dummy-sa-before', 'bindings': [{
            'role': 'roles/iam.workloadIdentityUser', 'members': ['dummy-old-runtime-member'],
            'condition': {'title': 'dummy-title', 'expression': 'dummy-CEL', 'location': 'dummy-location'}}],
            'auditConfigs': [{'service': 'allServices', 'auditLogConfigs': [
                {'logType': 'ADMIN_READ', 'exemptedMembers': ['dummy-exempt-member']}]}]}
        self.project = {'projectId': m.PROJECT, 'projectNumber': manifest['project_number'], 'lifecycleState': 'ACTIVE'}
        self.pool = {'name': manifest['runtime_pool_resource'], 'state': 'ACTIVE', 'mode': 'FEDERATION_ONLY'}
        self.provider = {'name': manifest['runtime_provider_resource'], 'state': 'ACTIVE',
            'aws': {'accountId': '222222222222'}, 'attributeMapping': {'google.subject': 'dummy-subject',
                'attribute.account': 'dummy-account', 'attribute.aws_role': 'dummy-role'},
            'attributeCondition': 'dummy-provider-CEL'}
        self.attestation = {}
        self.hook = None

    def __call__(self, host, path, *, method='GET', body=None, token=None, raw=False):
        call = (host, path, method, copy.deepcopy(body))
        self.calls.append(call)
        if self.hook:
            result = self.hook(call)
            if result is not None:
                return result
        if path == '/v1/projects/' + m.PROJECT:
            return 200, copy.deepcopy(self.project)
        if ':testIamPermissions' in path:
            return 200, {'permissions': copy.deepcopy(body['permissions'])}
        if path.endswith('/roles'):
            self.writes.append(call)
            alias = next(alias for alias in m.ROLES if body['roleId'] == m.ROLES[alias]['roleId'])
            result = dict(copy.deepcopy(body['role']), name=m.role_name(alias), etag='dummy-new-role-etag')
            self.roles[alias] = result
            return 200, copy.deepcopy(result)
        for alias in m.ROLES:
            if path == '/v1/' + m.role_name(alias):
                return (200, copy.deepcopy(self.roles[alias])) if alias in self.roles else (404, None)
        if path.endswith(':getIamPolicy') or ':getIamPolicy?' in path:
            return 200, copy.deepcopy(self.pool_policy if '/workloadIdentityPools/' in path else self.sa_policy)
        if path.endswith(':setIamPolicy'):
            self.writes.append(call)
            result = dict(copy.deepcopy(body['policy']), etag='dummy-after-' + str(len(self.writes)))
            if '/workloadIdentityPools/' in path:
                self.pool_policy = result
            else:
                self.sa_policy = result
            return 200, copy.deepcopy(result)
        if path.endswith(':listAttestationRules'):
            return 200, copy.deepcopy(self.attestation)
        if '/providers/' in path:
            return 200, copy.deepcopy(self.provider)
        if '/workloadIdentityPools/' in path:
            return 200, copy.deepcopy(self.pool)
        raise AssertionError('unexpected dummy API interface')


class Tests(unittest.TestCase):
    def setUp(self):
        self.manifest, self.workflow = fixture()
        self.clock = patch.object(m, 'now', return_value=TIME)
        self.clock.start()
        self.addCleanup(self.clock.stop)
        self.root = Path(tempfile.mkdtemp())
        self.root.chmod(0o700)
        self.addCleanup(shutil.rmtree, self.root)

    def evidence(self):
        evidence = m.Evidence()
        self.addCleanup(shutil.rmtree, evidence.root)
        self.addCleanup(evidence.close)
        return evidence

    def session(self, *, empty=True, execute=True, no_cas=True):
        self.cloud = FakeCloud(self.manifest, empty=empty)
        self.gates = []
        return m.Bootstrap(self.manifest, self.evidence(), execute=execute, no_cas=no_cas,
            request=self.cloud, github=lambda manifest: self.gates.append('checked'), token='dummy-secret-token')

    def manifest_file(self, value=None):
        path = self.root / 'intent.json'
        path.write_bytes(m.encoded(value or self.manifest))
        path.chmod(0o600)
        return path, m.sha(path.read_bytes())

    def args(self, path, checksum):
        return ['--manifest-file', str(path), '--manifest-sha256', checksum]

    def capture_main(self, args):
        public = io.StringIO()
        with contextlib.redirect_stdout(public), contextlib.redirect_stderr(public):
            result = m.main(args)
        output = public.getvalue()
        for value in ('dummy-secret-token', 'dummy-CEL', 'dummy-old-runtime-member',
                      self.manifest['project_number'], self.manifest['runtime_pool_resource']):
            self.assertNotIn(value, output)
        directories = [line.split('=', 1)[1] for line in output.splitlines() if line.startswith('RESULT_DIRECTORY=')]
        for directory in directories:
            self.addCleanup(shutil.rmtree, directory)
        return result, output, Path(directories[0]) if directories else None

    def test_default_offline_never_authenticates_or_calls_network_even_with_approval_bits(self):
        path, checksum = self.manifest_file()
        with patch.object(m, 'owner_token') as token, patch.object(m, 'http') as http, \
             patch.object(m, 'check_github') as github:
            result, output, directory = self.capture_main(self.args(path, checksum))
        self.assertEqual(result, 0)
        token.assert_not_called(); http.assert_not_called(); github.assert_not_called()
        self.assertIn('write-attempts: 0', output)
        self.assertEqual(stat.S_IMODE(directory.stat().st_mode), 0o700)
        self.assertTrue(all(stat.S_IMODE(p.stat().st_mode) == 0o600 for p in directory.iterdir()))
        last = json.loads((directory / 'journal.jsonl').read_text().splitlines()[-1])
        self.assertFalse(last['setup_complete']); self.assertFalse(last['migration_ready'])

    def test_false_owner_approval_bits_are_valid_offline_but_execute_stops_before_auth(self):
        self.manifest.update(owner_exception_approved=False, known_iam_writers_quiescent=False)
        path, checksum = self.manifest_file()
        with patch.object(m, 'owner_token') as token, patch.object(m, 'check_github') as github:
            self.assertEqual(self.capture_main(self.args(path, checksum))[0], 0)
            self.assertEqual(self.capture_main(self.args(path, checksum) + ['--execute-owner-exception',
                '--source-sha256', m.source_hash(), '--accept-empty-pool-no-cas'])[0], 1)
        token.assert_not_called(); github.assert_not_called()

    def test_private_parser_missing_unknown_and_bad_hash_are_silent_before_auth(self):
        path, checksum = self.manifest_file()
        failures = [[], ['--manifest-file', str(path)], self.args(path, 'g' * 64),
            self.args(path, checksum) + ['--private-dummy-sensitive-argument'],
            self.args(path, checksum) + ['--live-plan', '--execute-owner-exception'],
            self.args(path, checksum) + ['--execute-owner-exception'],
            self.args(path, checksum) + ['--execute-owner-exception', '--source-sha256', 'a' * 64],
            self.args(path, checksum) + ['--accept-empty-pool-no-cas']]
        with patch.object(m, 'owner_token') as token, patch.object(m, 'check_github') as github:
            for args in failures:
                result, output, _ = self.capture_main(args)
                self.assertEqual(result, 1)
                self.assertNotIn(str(path), output)
                self.assertNotIn('private-dummy-sensitive-argument', output)
        token.assert_not_called(); github.assert_not_called()

    def test_hash_mismatch_symlink_hardlink_checkout_and_unsafe_modes_reject(self):
        path, checksum = self.manifest_file()
        with self.assertRaises(ValueError): m.private_input(path, '0' * 64)
        link = self.root / 'link'; link.symlink_to(path)
        with self.assertRaises(ValueError): m.private_input(link, checksum)
        hard = self.root / 'hard'; os.link(path, hard)
        with self.assertRaises(ValueError): m.private_input(path, checksum)
        hard.unlink()
        for mode in (0o666, 0o620, 0o660, 0o700):
            path.chmod(mode)
            with self.assertRaises(ValueError): m.private_input(path, checksum)
        path.chmod(0o644)
        self.assertEqual(m.private_input(path, checksum), self.manifest)
        self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
        (self.root / '.git').write_text('gitdir: dummy-checkout')
        with self.assertRaises(ValueError): m.private_input(path, checksum)
        (self.root / '.git').unlink(); self.root.chmod(0o777)
        with self.assertRaises(ValueError): m.private_input(path, checksum)

    def test_duplicate_json_unknown_shape_and_4mib_limit(self):
        for raw in (b'{"x":1,"x":2}', b'{"x":NaN}', b'[]', b' ' * (m.MAX_BYTES + 1)):
            with self.assertRaises((ValueError, TypeError)): m.decode(raw)
        for change in ({'unexpected': True}, {'schema_version': True}, {'project_id': 'dummy-prod'},
                       {'project_number': '1/../../dummy'}, {'source_usage': 'old-fresh-evidence'},
                       {'owner_session_scope': 'service-account'}, {'owner_exception_approved': 1}):
            with self.assertRaises(ValueError): m.validate_manifest(dict(self.manifest, **change))

    def test_exact_targets_two_ci_principals_and_all_role_fields_are_fixed(self):
        for key in ('runtime_pool_resource', 'runtime_provider_resource', 'runtime_iam_policy_resource',
                    'ci_plan_service_account', 'ci_apply_service_account'):
            value = copy.deepcopy(self.manifest); value[key] += '-dummy-other'
            with self.assertRaises(ValueError): m.validate_manifest(value)
        for key in ('roleId', 'role'):
            value = copy.deepcopy(self.manifest)
            if key == 'roleId': value['role_creations']['pool'][key] = 'dummyCollisionFallback'
            else: value['role_creations']['pool'][key]['includedPermissions'].append('iam.roles.update')
            with self.assertRaises(ValueError): m.validate_manifest(value)
        value = copy.deepcopy(self.manifest)
        value['binding_additions'][0]['members'].append('serviceAccount:dummy-new@example.invalid')
        with self.assertRaises(ValueError): m.validate_manifest(value)

    def test_window_not_extended_and_default_offline_does_not_claim_freshness(self):
        path, checksum = self.manifest_file()
        with patch.object(m, 'now', return_value=TIME + timedelta(days=1)), patch.object(m, 'owner_token') as token:
            self.assertEqual(self.capture_main(self.args(path, checksum))[0], 0)
            self.assertEqual(self.capture_main(self.args(path, checksum) + ['--live-plan',
                '--source-sha256', m.source_hash()])[0], 1)
        token.assert_not_called()
        value = copy.deepcopy(self.manifest); value['expires_utc'] = '2030-01-02T00:00:00Z'
        with self.assertRaises(ValueError): m.validate_manifest(value)

    def test_all_preflight_reads_complete_before_any_write_and_full_policy_preserved(self):
        session = self.session()
        before = copy.deepcopy(self.cloud.sa_policy)
        session.prepare()
        self.assertEqual(self.cloud.writes, [])
        self.assertEqual(self.cloud.calls[0][1], '/v1/projects/' + m.PROJECT)
        self.assertEqual(session.before['sa_policy'], before)
        session.run()
        self.assertEqual(session.attempts, 4)
        self.assertEqual(session.completed, ['role-pool-create', 'role-sa-create', 'pool-set', 'sa-set'])
        self.assertEqual(self.cloud.sa_policy['bindings'][0], before['bindings'][0])
        self.assertEqual(self.cloud.sa_policy['auditConfigs'], before['auditConfigs'])
        for alias, observed in (('pool', self.cloud.pool_policy), ('sa', self.cloud.sa_policy)):
            self.assertEqual(next(b for b in observed['bindings'] if b['role'] == m.role_name(alias))['members'], m.fixed_members())
        for _, path, _, body in self.cloud.writes:
            self.assertFalse('/projects/' + m.PROJECT + ':setIamPolicy' in path)
            if path.endswith(':setIamPolicy'):
                self.assertEqual(body['updateMask'], 'bindings,etag')
        journal = [json.loads(line) for line in (session.evidence.root / 'journal.jsonl').read_text().splitlines()]
        self.assertEqual(sum(row['event'] == 'write-attempt' for row in journal), 4)

    def test_nonempty_pool_etag_is_used_and_unrelated_binding_is_preserved(self):
        session = self.session(empty=False, no_cas=False)
        before = copy.deepcopy(self.cloud.pool_policy)
        session.prepare(); session.run()
        call = next(call for call in self.cloud.writes if '/workloadIdentityPools/' in call[1])
        self.assertEqual(call[3]['policy']['etag'], before['etag'])
        self.assertEqual(self.cloud.pool_policy['bindings'][0], before['bindings'][0])

    def test_empty_pool_requires_both_explicit_no_cas_ack_and_manifest_ack_before_write(self):
        for option, manifest_ack in ((False, True), (True, False)):
            self.manifest['allow_empty_pool_policy_initialization'] = manifest_ack
            session = self.session(no_cas=option)
            session.prepare()
            with self.assertRaises(ValueError): session.run()
            self.assertEqual(session.attempts, 0)
            self.assertEqual(self.cloud.writes, [])

    def test_empty_pool_two_immediate_equal_gets_do_not_claim_cas(self):
        session = self.session(); session.prepare(); session.run()
        index = next(i for i, call in enumerate(self.cloud.calls) if '/workloadIdentityPools/' in call[1] and call[1].endswith(':setIamPolicy'))
        previous = self.cloud.calls[index - 2:index]
        self.assertTrue(all(call[1].endswith(':getIamPolicy') for call in previous))
        dry = json.loads((session.evidence.root / 'dry-plan.json').read_text())
        self.assertFalse(dry['pool_cas_available'])
        self.assertFalse(dry['all_writers_quiescence_proven'])

    def test_project_mismatch_stops_before_pool_permissions_or_any_other_cloud_read(self):
        session = self.session(); self.cloud.project['projectNumber'] = '333333333333'
        with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(len(self.cloud.calls), 1)
        self.assertEqual(self.cloud.writes, [])

    def test_missing_authority_and_role_collision_never_create_or_patch(self):
        for fail in ('authority', 'existing-role', 'deleted-role', 'role-403'):
            session = self.session()
            def hook(call):
                if fail == 'authority' and call[1].endswith(':testIamPermissions'):
                    return 200, {'permissions': []}
                if call[1] == '/v1/' + m.role_name('pool'):
                    if fail == 'role-403': return 403, None
                    if fail in {'existing-role', 'deleted-role'}:
                        return 200, dict(copy.deepcopy(m.ROLES['pool']['role']), name=m.role_name('pool'), deleted=fail == 'deleted-role')
            self.cloud.hook = hook
            with self.assertRaises(ValueError): session.prepare()
            self.assertFalse(session.prepared)
            self.assertEqual(session.attempts, 0)

    def test_unknown_or_nonempty_attestation_and_bad_provider_prevent_all_writes(self):
        for kind, value in [('attestation', {'attestationRules': [{}]}),
                            ('attestation', {'nextPageToken': 'dummy-continuation'}),
                            ('attestation', {'future': []}), ('attestation', {'attestationRules': False}),
                            ('provider', {'name': self.manifest['runtime_provider_resource'], 'state': 'ACTIVE', 'oidc': {}})]:
            session = self.session(); setattr(self.cloud, kind, value)
            with self.assertRaises(ValueError): session.prepare()
            self.assertEqual(self.cloud.writes, [])

    def test_policy_conditions_duplicate_bindings_unknown_fields_and_missing_etag_stop(self):
        session = self.session()
        cases = [dict(self.cloud.sa_policy, etag=''),
                 {key: value for key, value in self.cloud.sa_policy.items() if key != 'etag'},
                 dict(self.cloud.sa_policy, version=1), dict(self.cloud.sa_policy, future=True),
                 dict(self.cloud.sa_policy, bindings=self.cloud.sa_policy['bindings'] * 2)]
        for value in cases:
            with self.assertRaises(ValueError): m.policy(value)
        with self.assertRaises(ValueError): m.policy({'bindings': []}, empty_pool=True)
        before = copy.deepcopy(self.cloud.sa_policy)
        expected = m.new_policy(before, 'sa')
        self.assertEqual(before, self.cloud.sa_policy)
        self.assertEqual(expected['bindings'][0]['condition'], before['bindings'][0]['condition'])

    def test_policy_drift_between_preflight_and_set_leaves_only_roles_with_no_policy_restore(self):
        session = self.session(); session.prepare()
        self.cloud.pool_policy = {'etag': 'dummy-new-etag', 'bindings': [{'role': 'roles/dummy.reader', 'members': ['dummy-concurrent-member']}]}
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 2)
        self.assertEqual(len(self.cloud.writes), 2)
        self.assertEqual(self.cloud.pool_policy['bindings'][0]['members'], ['dummy-concurrent-member'])

    def test_last_empty_get_drift_prevents_only_set_and_does_not_retry(self):
        session = self.session(); session.prepare()
        count = [0]
        def hook(call):
            if call[1].endswith(':getIamPolicy') and '/workloadIdentityPools/' in call[1]:
                count[0] += 1
                if count[0] == 2: return 200, {'etag': 'dummy-concurrent-etag', 'bindings': []}
        self.cloud.hook = hook
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 2)
        self.assertEqual(len(self.cloud.writes), 2)

    def test_timeout_409_403_or_malformed_write_is_durable_unknown_and_never_retried(self):
        for outcome in ('timeout', 409, 403, 'malformed'):
            session = self.session(); session.prepare()
            def hook(call):
                if call[1].endswith('/roles'):
                    if outcome == 'timeout': raise TimeoutError('dummy-secret-token dummy-CEL')
                    if outcome == 'malformed': return 200, {'future': 'dummy-sensitive-body'}
                    return outcome, None
            self.cloud.hook = hook
            with self.assertRaises(ValueError): session.run()
            self.assertEqual(session.attempts, 1)
            with self.assertRaises(ValueError): session.run()
            self.assertEqual(session.attempts, 1)
            rows = (session.evidence.root / 'journal.jsonl').read_text().splitlines()
            self.assertEqual(json.loads(rows[0])['event'], 'write-attempt')
            self.assertEqual(json.loads(rows[-1])['event'], 'write-unconfirmed')

    def test_read_only_live_plan_and_out_of_order_write_cannot_mutate(self):
        session = self.session(execute=False); session.prepare()
        with self.assertRaises(ValueError): session.run()
        with self.assertRaises(ValueError): session.write('role-pool-create', m.ROLES['pool'])
        self.assertEqual(session.attempts, 0)
        session = self.session(); session.prepare()
        with self.assertRaises(ValueError): session.write('pool-set', {'policy': m.new_policy({}, 'pool'), 'updateMask': 'bindings,etag'})
        self.assertEqual(session.attempts, 0)

    def test_changed_github_gate_before_mutation_stops_with_zero_attempts(self):
        session = self.session(); session.prepare()
        def stop(_manifest): raise ValueError('dummy-private-source')
        session.github = stop
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 0)

    def test_expiry_before_read_after_response_before_write_and_save_is_fail_closed(self):
        session = self.session()
        with patch.object(m, 'now', return_value=TIME + timedelta(hours=1)):
            with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(self.cloud.calls, [])
        session = self.session()
        def expire(call):
            m.now.return_value = TIME + timedelta(hours=1)
        self.cloud.hook = expire
        with self.assertRaises(ValueError): session.prepare()
        self.assertEqual(len(self.cloud.calls), 1)
        m.now.return_value = TIME
        session = self.session(); session.prepare()
        with patch.object(m, 'now', return_value=TIME + timedelta(hours=1)):
            with self.assertRaises(ValueError): session.run()
            with self.assertRaises(ValueError): session.save('must-not-exist.json', {})
        self.assertEqual(session.attempts, 0)
        self.assertFalse((session.evidence.root / 'must-not-exist.json').exists())

    def github(self, *, workflow=None, head=None, active=None, denied=False):
        calls = []
        def request(host, path, **kwargs):
            calls.append((host, path, kwargs))
            if denied: return 403, None
            if host == 'raw.githubusercontent.com': return 200, workflow or self.workflow
            if '/git/ref/' in path:
                return 200, {'ref': 'refs/heads/' + m.BRANCH, 'object': {'type': 'commit', 'sha': head or self.manifest['reviewed_revision']}}
            return 200, active or {'total_count': 0, 'workflow_runs': []}
        return request, calls

    def test_github_requires_exact_head_workflow_hash_explicit_closed_gates_and_zero_known_active_runs(self):
        request, calls = self.github()
        m.check_github(self.manifest, request)
        self.assertEqual(len(calls), 2 + len(m.ACTIVE_RUN_STATES))
        self.assertTrue(all('/actions/workflows/ci.yml/runs?' in path for _, path, _ in calls[2:]))
        for kwargs in ({'head': 'c' * 40}, {'active': {'total_count': 1, 'workflow_runs': [{}]}},
                       {'active': {'total_count': True, 'workflow_runs': []}}, {'denied': True},
                       {'workflow': b'dummy-unreviewed-source'}):
            with self.assertRaises(ValueError): m.check_github(self.manifest, self.github(**kwargs)[0])
        for raw in (self.workflow.replace(b'DEV_RUNTIME_METADATA_READ_ENABLED: "false"', b'DEV_RUNTIME_METADATA_READ_ENABLED: "true"'),
                    self.workflow + b'  DEV_RUNTIME_METADATA_READ_ENABLED: "false"\n'):
            value = dict(self.manifest, workflow_sha256=m.sha(raw))
            with self.assertRaises(ValueError): m.check_github(value, self.github(workflow=raw)[0])

    def cli(self, *, account='dummy-owner@example.invalid', override=False, token='dummy-secret-token'):
        calls = []
        def run(args, **kwargs):
            calls.append((args, kwargs))
            if args[1:3] == ['auth', 'list']:
                out = json.dumps([{'account': account, 'status': 'ACTIVE'}])
            elif args[1:3] == ['config', 'get-value']:
                out = 'dummy-credential-file' if override else '(unset)'
            else:
                out = token
            return SimpleNamespace(returncode=0, stdout=out, stderr='dummy-private-diagnostic')
        return run, calls

    def test_owner_current_normal_account_explicit_token_selection_and_no_config_writes(self):
        run, calls = self.cli()
        with patch.dict(os.environ, {}, clear=True), patch.object(m.subprocess, 'run', side_effect=run):
            self.assertEqual(m.owner_token(), 'dummy-secret-token')
        self.assertIn('--account=dummy-owner@example.invalid', calls[-1][0])
        self.assertTrue(all(kwargs['capture_output'] for _, kwargs in calls))
        self.assertFalse(any('login' in args or 'activate-service-account' in args or 'set' in args for args, _ in calls))
        self.assertEqual(calls[-1][1]['env']['CLOUDSDK_CORE_LOG_HTTP'], 'false')

    def test_sa_or_impersonation_adc_external_override_and_bad_token_reject(self):
        for kwargs in ({'account': 'dummy@dummy.iam.gserviceaccount.com'}, {'override': True},
                       {'token': 'dummy token'}, {'account': 'principal://dummy-external'}):
            run, _ = self.cli(**kwargs)
            with patch.dict(os.environ, {}, clear=True), patch.object(m.subprocess, 'run', side_effect=run):
                with self.assertRaises(ValueError): m.owner_token()
        for variable in ('GOOGLE_APPLICATION_CREDENTIALS', 'CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE',
                         'CLOUDSDK_AUTH_ACCESS_TOKEN', 'GOOGLE_GHA_CREDS_PATH'):
            with patch.dict(os.environ, {variable: 'dummy-private-credential'}, clear=True), patch.object(m.subprocess, 'run') as run:
                with self.assertRaises(ValueError): m.owner_token()
                run.assert_not_called()

    def test_transport_redirect_host_error_body_and_oversize_boundaries(self):
        with self.assertRaises(ValueError): m.NoRedirect().redirect_request(None, None, 302, '', {}, 'https://dummy.invalid')
        with self.assertRaises(ValueError): m.http('dummy.invalid', '/v1/dummy', token='dummy-secret-token')
        with self.assertRaises(ValueError): m.http('api.github.com', '/dummy', token='dummy-secret-token')
        body = io.BytesIO(b'dummy-private-error-body')
        error = urllib.error.HTTPError('https://iam.googleapis.com/dummy-private-path', 403,
            'dummy-secret-token', {}, body)
        public = io.StringIO()
        with patch.object(m.urllib.request, 'build_opener') as opener, \
             contextlib.redirect_stdout(public), contextlib.redirect_stderr(public):
            opener.return_value.open.side_effect = error
            self.assertEqual(m.http('iam.googleapis.com', '/v1/dummy', token='dummy-secret-token'), (403, None))
        self.assertEqual(public.getvalue(), '')
        self.assertTrue(body.closed)
        response = SimpleNamespace(status=200, headers={'Content-Length': str(m.MAX_BYTES + 1)})
        context = unittest.mock.MagicMock(); context.__enter__.return_value = response
        with patch.object(m.urllib.request, 'build_opener') as opener:
            opener.return_value.open.return_value = context
            with self.assertRaises(ValueError): m.http('iam.googleapis.com', '/v1/dummy', token='dummy-secret-token')

    def test_main_live_plan_never_writes_and_private_dependency_errors_are_suppressed(self):
        path, checksum = self.manifest_file()
        args = self.args(path, checksum) + ['--live-plan', '--source-sha256', m.source_hash()]
        cloud = FakeCloud(self.manifest)
        original = m.Bootstrap
        def constructor(*args, **kwargs):
            kwargs.update(request=cloud, github=lambda manifest: None)
            return original(*args, **kwargs)
        with patch.object(m, 'check_github'), patch.object(m, 'owner_token', return_value='dummy-secret-token'), \
             patch.object(m, 'Bootstrap', side_effect=constructor):
            self.assertEqual(self.capture_main(args)[0], 0)
        self.assertEqual(cloud.writes, [])
        with patch.object(m, 'check_github', side_effect=ValueError('dummy-private-error-body')):
            result, output, _ = self.capture_main(args)
        self.assertEqual(result, 1); self.assertNotIn('dummy-private-error-body', output)

    def test_main_execute_complete_is_distinct_from_ci_or_migration_readiness(self):
        path, checksum = self.manifest_file()
        cloud = FakeCloud(self.manifest)
        original = m.Bootstrap
        def constructor(*args, **kwargs):
            kwargs.update(request=cloud, github=lambda manifest: None)
            return original(*args, **kwargs)
        with patch.object(m, 'check_github'), patch.object(m, 'owner_token', return_value='dummy-secret-token'), \
             patch.object(m, 'Bootstrap', side_effect=constructor):
            result, output, directory = self.capture_main(self.args(path, checksum) +
                ['--execute-owner-exception', '--accept-empty-pool-no-cas', '--source-sha256', m.source_hash()])
        self.assertEqual(result, 0); self.assertIn('write-attempts: 4', output)
        last = json.loads((directory / 'journal.jsonl').read_text().splitlines()[-1])
        self.assertTrue(last['setup_complete'])
        for key in ('ci_identity_read_verified', 'provider_inheritance_verified', 'owner_identity_independently_verified',
                    'all_writers_quiescence_proven', 'migration_ready'):
            self.assertFalse(last[key])

    def test_lost_third_write_response_retains_partial_journal_without_rollback(self):
        session = self.session(); session.prepare()
        def hook(call):
            if call[1].endswith(':setIamPolicy'):
                raise TimeoutError('dummy-private-body')
        self.cloud.hook = hook
        with self.assertRaises(ValueError): session.run()
        self.assertEqual(session.attempts, 3)
        self.assertEqual(session.completed, ['role-pool-create', 'role-sa-create'])
        self.assertEqual(len(self.cloud.writes), 2)
        self.assertEqual(self.cloud.pool_policy, {})
        rows = [json.loads(line) for line in (session.evidence.root / 'journal.jsonl').read_text().splitlines()]
        self.assertEqual(rows[-1], {'event': 'write-unconfirmed', 'alias': 'pool-set'})
        self.assertFalse(any('delete' in call[1] or 'patch' in call[1] for call in self.cloud.calls))

    def test_record_failure_prevents_the_corresponding_mutation(self):
        session = self.session(); session.prepare()
        with patch.object(session.evidence, 'event', side_effect=OSError('dummy-private-path')):
            with self.assertRaises(OSError): session.run()
        self.assertEqual(self.cloud.writes, [])
        self.assertEqual(session.attempts, 1)

    def test_routes_never_allow_list_key_project_policy_or_trust_mutation(self):
        session = self.session()
        for alias in ('roles-list', 'project-policy', 'sa-keys', 'provider-update', 'pool-delete', 'state'):
            with self.assertRaises(ValueError): session.route(alias)
        host, path, method, body = session.route('sa-policy')
        self.assertEqual((host, method, body), ('iam.googleapis.com', 'POST', None))
        self.assertTrue(path.endswith(':getIamPolicy?options.requestedPolicyVersion=3'))
        self.assertEqual(session.route('pool-policy')[3], {'options': {'requestedPolicyVersion': 3}})

    def test_token_generation_does_not_restart_invocation_freshness_window(self):
        path, checksum = self.manifest_file()
        # Keep the authorization manifest valid for eight hours, but expire
        # the independent invocation window while the captured CLI runs.
        self.manifest['expires_utc'] = '2030-01-01T08:00:00Z'
        path, checksum = self.manifest_file()
        cloud = FakeCloud(self.manifest)
        original = m.Bootstrap
        def constructor(*args, **kwargs):
            kwargs.update(request=cloud, github=lambda manifest: None)
            return original(*args, **kwargs)
        def token():
            m.now.return_value = TIME + timedelta(hours=1)
            return 'dummy-secret-token'
        with patch.object(m, 'check_github'), patch.object(m, 'owner_token', side_effect=token), \
             patch.object(m, 'Bootstrap', side_effect=constructor):
            self.assertEqual(self.capture_main(self.args(path, checksum) + ['--live-plan', '--source-sha256', m.source_hash()])[0], 1)
        self.assertEqual(cloud.calls, [])


if __name__ == '__main__':
    unittest.main()
