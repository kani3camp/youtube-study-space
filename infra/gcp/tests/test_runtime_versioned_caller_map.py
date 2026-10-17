#!/usr/bin/env python3
"""Dummy-only historical/current boundary, explicit grant map, and private IO."""
from __future__ import annotations

import copy
from datetime import timedelta
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
sys.path.insert(0, str(Path(__file__).parent))
import prepare_runtime_versioned_caller_map as join
import test_collect_aws_runtime_versioned_config as aws_fixture
import test_collect_runtime_metadata as gcp_fixture
import test_runtime_ownership as inventory_fixture

POINT = aws_fixture.POINT


def substitute(value):
    if type(value) is dict:
        return {key: substitute(item) for key, item in value.items()}
    if type(value) is list:
        return [substitute(item) for item in value]
    if type(value) is str:
        return value.replace('example-study-space-dev', join.aws.PROJECT)
    return value


class CallerJoinContracts(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.historical = substitute(aws_fixture.capture())
        self.aws_request = aws_fixture.request()
        old_raw = json.dumps(self.historical, indent=4).encode() + b'\n'
        self.aws_request['owner_capture_sha256'] = join.sha(old_raw)
        responses = iter(substitute(aws_fixture.responses()))
        current = join.aws.collect(self.aws_request, self.historical, current=True,
                                   read=lambda _: next(responses), clock=lambda: POINT)
        canonical_source = dict(schema_version='dummy-v1', canonical_page='dummy-source',
            source_account='333333333333', source_arn='arn:aws:iam::333333333333:user/DummySource',
            source_user_id='AIDAAAAAAAAAAAAAAAAAA', environments={'dev': dict(account='222222222222',
                region=join.aws.REGION, stack_name=join.aws.STACK,
                readonly_role_arn=self.aws_request['readonly_role_arn'])})
        self.gcp_manifest = dict(schema_version=1, kind='private-codex-dev-runtime-metadata-read',
            execute_approved=True, source_revision='b' * 40, auth_selector_sha256='c' * 64,
            script_sha256=join.sha(join.SOURCE_FILES['gcp_collector'].read_bytes()),
            collector_sha256=join.sha(join.SOURCE_FILES['gcp_validator'].read_bytes()),
            requirements_sha256=join.sha(join.codex.REQUIREMENTS.read_bytes()),
            runtime_selector_sha256='f' * 64, dependency_selector_sha256='1' * 64,
            source_identity=dict(account_id=canonical_source['source_account'], arn=canonical_source['source_arn'],
                                 user_id=canonical_source['source_user_id']),
            issued_utc='2026-01-02T00:25:00Z', expires_utc='2026-01-02T00:40:00Z')
        data = substitute(inventory_fixture.inventory())
        data.pop('runtime_grants')
        data.pop('api_classification')
        data['project']['lifecycleState'] = 'ACTIVE'
        data['history_table'] = substitute(gcp_fixture.fixture()[1]['history'])
        roles = [item['role'].rsplit('/', 1)[1] for item in current['observations'][3]['value']['selected']]
        roles.append(self.historical['task_role_arn'].rsplit('/', 1)[1])
        data['provider']['attributeCondition'] = "attribute.account == '222222222222' && attribute.aws_role in " + repr(roles)
        prefix = 'principalSet://iam.googleapis.com/' + data['pool']['name'] + '/attribute.aws_role/'
        data['iam_policy']['bindings'] = [dict(role=join.ownership.ROLE, members=[prefix + role]) for role in roles]
        data['iam_policy']['bindings'][0]['condition'] = dict(title='dummy-condition', expression='true')
        data['iam_policy']['bindings'].append(dict(role=join.ownership.ROLE, members=['DUMMY_UNRESOLVED_MEMBER']))
        data['iam_policy']['bindings'].append(dict(role='roles/viewer', members=['DUMMY_UNRELATED_MEMBER']))
        capture = dict(schema_version=1, kind='private-codex-runtime-metadata-capture', source_revision='b' * 40,
            manifest_sha256=join.sha(join.canonical(self.gcp_manifest)), capture_started_utc=POINT.isoformat(),
            capture_completed_utc=POINT.isoformat(), complete=True, metadata=data, explicit_auth_requests=3,
            explicit_metadata_requests=7, source_identity_verified=True, canonical_auth_selector_sha256='c' * 64,
            lambda6_source_match=True, ecs_source_match=False, ci_identity_read_verified=False,
            caller_scope_verified=False, root_admission_verified=False, migration_ready=False)
        journal = b'\n'.join(join.canonical(row) for row in join.expected_journal(current['call_plan'])) + b'\n'
        self.raw = dict(aws_request=join.canonical(self.aws_request), aws_current=join.canonical(current),
            owner_task_capture=old_raw, aws_auth_receipt=journal, aws_auth_wrapper=b'# dummy reviewed authentication source\n',
            aws_diagnostic_harness=b'# dummy reviewed diagnostic source\n', aws_canonical=join.canonical(canonical_source),
            gcp_capture=join.canonical(capture), gcp_manifest=join.canonical(self.gcp_manifest))
        self.request = dict(schema_version=1, kind='private-runtime-versioned-caller-map-request', environment='dev',
            git_sha='a' * 40, issued_utc='2026-01-02T00:00:00Z', expires_utc='2026-01-02T01:00:00Z',
            sources={key: join.sha(path.read_bytes()) for key, path in join.SOURCE_FILES.items()},
            inputs={key: dict(path=str(self.root / (key + '.json')), sha256=join.sha(raw)) for key, raw in self.raw.items()},
            aws_source_revision='d' * 40, gcp_source_revision='b' * 40, gcp_auth_selector_sha256='c' * 64)
        for context in (patch('socket.socket', side_effect=AssertionError('network forbidden')),
                        patch('subprocess.run', side_effect=AssertionError('child forbidden')),
                        patch.object(join.aws, 'aws_read', side_effect=AssertionError('AWS forbidden')),
                        patch.object(join.gcp, 'google_token', side_effect=AssertionError('token forbidden')),
                        patch.object(join.gcp, 'transport', side_effect=AssertionError('GCP forbidden')),
                        patch.object(join.codex, 'load_sdk', side_effect=AssertionError('SDK forbidden')),
                        patch.object(join, 'now', return_value=POINT)):
            context.start()
            self.addCleanup(context.stop)

    def update(self, key, mutate):
        value = join.decode(self.raw[key])
        mutate(value)
        self.raw[key] = join.canonical(value)
        self.request['inputs'][key]['sha256'] = join.sha(self.raw[key])

    def run_join(self, *, current_time=POINT):
        return join.prepare_candidate(self.request, self.raw, current_time=current_time)

    def test_seven_configuration_callers_preserve_full_inventory_historical_bytes_and_false_claims(self):
        candidate = self.run_join()
        self.assertEqual(candidate['caller_count'], 7)
        self.assertEqual(candidate['required_grant_candidate_count'], 7)
        self.assertTrue(candidate['configuration_map_complete'])
        self.assertFalse(any(candidate['interpretation'].values()))
        self.assertFalse(candidate['evidence']['signed_execution_attestation'])
        self.assertFalse(candidate['evidence']['gcp_capture_ecs_source_match'])
        self.assertEqual(candidate['historical_task_projection']['body'], self.historical)
        self.assertEqual(candidate['historical_task_projection']['sha256'], join.sha(self.raw['owner_task_capture']))
        self.assertEqual(candidate['inventory_sha256'], join.sha(join.canonical(candidate['inventory'])))
        source = join.decode(self.raw['gcp_capture'])['metadata']
        for key, value in source.items():
            self.assertEqual(candidate['inventory'][key], value)
        self.assertEqual(candidate['remaining_wif_members_requiring_purpose_review'],
                         [dict(member='DUMMY_UNRESOLVED_MEMBER', condition=None)])
        self.assertEqual(len(candidate['inventory']['enabled_services']['services']), len(source['enabled_services']['services']))
        self.assertTrue(all(v['classification'] == 'Investigate' for v in candidate['inventory']['api_classification'].values()))
        self.assertEqual(self.raw['owner_task_capture'], json.dumps(self.historical, indent=4).encode() + b'\n')
        self.assertNotEqual(candidate['historical_task_projection']['body']['captured_at'], candidate['prepared_utc'])
        self.assertEqual(join.decode(self.raw['gcp_capture'])['ecs_source_match'], False)

    def test_multiple_callers_sharing_role_deduplicate_instead_of_assuming_seven_grants(self):
        def change_current(value):
            items = value['observations'][3]['value']['selected']
            items[1]['role'] = items[0]['role']
        self.update('aws_current', change_current)
        def change_gcp(value):
            data = value['metadata']
            data['provider']['attributeCondition'] = data['provider']['attributeCondition'].replace("'DummyLambda-1', ", '')
            data['iam_policy']['bindings'].pop(1)
        self.update('gcp_capture', change_gcp)
        candidate = self.run_join()
        self.assertEqual(candidate['caller_count'], 7)
        self.assertEqual(candidate['required_grant_candidate_count'], 6)
        self.assertEqual(candidate['callers'][0]['grant_alias'], candidate['callers'][1]['grant_alias'])

    def test_source_and_each_raw_hash_pins_are_required(self):
        for kind, key in [('source', key) for key in self.request['sources']] + [('input', key) for key in self.raw]:
            with self.subTest(kind=kind, key=key):
                request = copy.deepcopy(self.request)
                if kind == 'source': request['sources'][key] = '0' * 64
                else: request['inputs'][key]['sha256'] = '0' * 64
                with self.assertRaises(ValueError):
                    join.prepare_candidate(request, self.raw, current_time=POINT)

    def test_strict_json_header_and_time_order_are_required(self):
        for raw in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":1e999}', b'\xff'):
            with self.assertRaises(ValueError): join.decode(raw)
        for key, value in (('schema_version', True), ('kind', 'legacy'), ('extra', 'DUMMY_PRIVATE'),
                           ('expires_utc', '2026-01-02T02:00:00Z')):
            request = copy.deepcopy(self.request)
            request[key] = value
            with self.assertRaises(ValueError): join.prepare_candidate(request, self.raw, current_time=POINT)
        self.update('aws_current', lambda v: v['observations'][1].update(started_at_utc='2026-01-02T00:29:59Z'))
        with self.assertRaises(ValueError): self.run_join()

    def test_current_record_shape_call_order_flags_and_observations_are_rechecked(self):
        mutations = [lambda v: v.update(mode='offline'), lambda v: v.update(schema_version=True),
            lambda v: v.update(metadata_reads=3), lambda v: v.update(extra='DUMMY_PRIVATE'),
            lambda v: v['observations'].reverse(), lambda v: v['observations'].pop(),
            lambda v: v['interpretation'].update(full_seven_caller_fresh=True),
            lambda v: v['interpretation'].update(reader_identity_verified=1),
            lambda v: v['observations'][0]['value'].update(account='999999999999'),
            lambda v: v['observations'][1]['value'][0]['outputs'][0].update(OutputValue=v['historical_capture']['body']['task_definition_arn'][:-1]+'8'),
            lambda v: v['observations'][2]['value'].append('DUMMY_PRIVATE'),
            lambda v: v['observations'][3]['value']['selected'].pop(),
            lambda v: v['observations'][3]['value']['selected'].append(copy.deepcopy(v['observations'][3]['value']['selected'][0])),
            lambda v: v['observations'][3]['value']['selected'][0].update(version='1'),
            lambda v: v['observations'][3]['value']['selected'][0].update(role='arn:aws:iam::999999999999:role/Wrong'),
            lambda v: v['observations'][3]['value']['selected'][0]['auth'].update(GOOGLE_CLOUD_PROJECT='wrong')]
        original = copy.deepcopy(self.raw), copy.deepcopy(self.request)
        for index, mutate in enumerate(mutations):
            with self.subTest(case=index):
                self.raw, self.request = copy.deepcopy(original)
                self.update('aws_current', mutate)
                with self.assertRaises(ValueError): self.run_join()

    def test_replacing_historical_time_body_or_immutable_anchor_is_not_a_fresh_capture(self):
        for mutate in (lambda v: v['historical_capture']['body'].update(captured_at=POINT.isoformat()),
                       lambda v: v['historical_capture'].update(sha256='0' * 64),
                       lambda v: v['historical_capture']['body'].update(task_role_arn=self.historical['execution_role_arn'])):
            original = self.raw['aws_current']
            self.update('aws_current', mutate)
            with self.assertRaises(ValueError): self.run_join()
            self.raw['aws_current'] = original
            self.request['inputs']['aws_current']['sha256'] = join.sha(original)

    def test_expired_aws_or_gcp_cannot_be_reapproved_by_refreshing_map_dates(self):
        self.request.update(issued_utc='2026-01-02T01:00:00Z', expires_utc='2026-01-02T02:00:00Z')
        with self.assertRaises(ValueError): self.run_join(current_time=POINT + timedelta(hours=1))
        self.request.update(issued_utc='2026-01-02T00:00:00Z', expires_utc='2026-01-02T01:00:00Z')
        self.update('gcp_manifest', lambda v: v.update(expires_utc=POINT.isoformat()))
        self.update('gcp_capture', lambda v: v.update(manifest_sha256=join.sha(self.raw['gcp_manifest'])))
        with self.assertRaises(ValueError): self.run_join()

    def test_gcp_execute_window_is_not_extended_and_capture_freshness_is_separate(self):
        later = POINT + timedelta(minutes=15)
        result = self.run_join(current_time=later)
        self.assertEqual(result['evidence']['gcp_execution_window'],
                         {key:self.gcp_manifest[key] for key in ('issued_utc','expires_utc')})
        self.assertLess(join.aws.stamp(self.gcp_manifest['expires_utc']), later)
        self.assertEqual(join.aws.stamp(result['evidence_expires_utc']),
                         join.aws.stamp(self.request['expires_utc']))
        canonical_source = join.decode(self.raw['aws_canonical'])
        with self.assertRaises(ValueError):
            join.validate_gcp(self.raw, self.request, canonical_source, POINT+join.MAX_CAPTURE_AGE)

    def test_auth_journal_source_canonical_and_exact_readonly_role_are_pinned(self):
        original = copy.deepcopy(self.raw), copy.deepcopy(self.request)
        for key, mutate in [('aws_canonical', lambda v: v['environments']['dev'].update(readonly_role_arn='wrong')),
                            ('aws_canonical', lambda v: v.update(source_account='999999999999')),
                            ('gcp_manifest', lambda v: v['source_identity'].update(user_id='wrong'))]:
            self.raw, self.request = copy.deepcopy(original)
            self.update(key, mutate)
            if key == 'gcp_manifest':
                self.update('gcp_capture', lambda v: v.update(manifest_sha256=join.sha(self.raw['gcp_manifest'])))
            with self.assertRaises(ValueError): self.run_join()
        self.raw, self.request = copy.deepcopy(original)
        rows = [join.decode(line) for line in self.raw['aws_auth_receipt'].splitlines()]
        rows[2]['return_code'] = 254
        self.raw['aws_auth_receipt'] = b'\n'.join(join.canonical(row) for row in rows)
        self.request['inputs']['aws_auth_receipt']['sha256'] = join.sha(self.raw['aws_auth_receipt'])
        with self.assertRaises(ValueError): self.run_join()

    def test_gcp_manifest_is_strict_and_linked_to_source_and_current_capture(self):
        original = copy.deepcopy(self.raw), copy.deepcopy(self.request)
        for key, value in (('schema_version', True), ('kind', 'offline'), ('execute_approved', False),
                           ('script_sha256', '0' * 64), ('collector_sha256', '0' * 64),
                           ('extra', 'DUMMY_PRIVATE')):
            with self.subTest(key=key):
                self.raw, self.request = copy.deepcopy(original)
                self.update('gcp_manifest', lambda v: v.update(**{key:value}))
                self.update('gcp_capture', lambda v: v.update(manifest_sha256=join.sha(self.raw['gcp_manifest'])))
                with self.assertRaises(ValueError): self.run_join()

    def test_gcp_full_policy_mapping_auth_completeness_and_claims_are_not_relaxed(self):
        mutations = [lambda v: v.update(complete=False), lambda v: v.update(explicit_metadata_requests=True),
            lambda v: v.update(ci_identity_read_verified=True), lambda v: v.update(root_admission_verified=True),
            lambda v: v.update(canonical_auth_selector_sha256='0' * 64),
            lambda v: v['metadata']['provider']['aws'].update(accountId='999999999999'),
            lambda v: v['metadata']['runtime_config'].update(audience='wrong'),
            lambda v: v['metadata']['provider']['attributeMapping'].update(**{'attribute.aws_role':'assertion.arn'}),
            lambda v: v['metadata']['provider']['attributeMapping'].update(**{'google.subject':'unsupported'}),
            lambda v: v['metadata']['provider']['attributeMapping'].update(extra='unsupported'),
            lambda v: v['metadata']['provider'].update(attributeCondition=v['metadata']['provider']['attributeCondition']+' || true'),
            lambda v: v['metadata']['provider'].update(attributeCondition=v['metadata']['provider']['attributeCondition'].replace('DummyLambda-0','Unexpected')),
            lambda v: v['metadata']['pool_attestation_rules'].update(nextPageToken='remaining'),
            lambda v: v['metadata']['enabled_services'].update(nextPageToken='remaining'),
            lambda v: v['metadata']['iam_policy']['bindings'].append(copy.deepcopy(v['metadata']['iam_policy']['bindings'][0])),
            lambda v: v['metadata']['iam_policy']['bindings'][0].update(members=['no-match'])]
        original = self.raw['gcp_capture']
        for index, mutate in enumerate(mutations):
            with self.subTest(case=index):
                self.raw['gcp_capture'] = original
                self.update('gcp_capture', mutate)
                with self.assertRaises(ValueError): self.run_join()

    def write_inputs(self):
        for key, raw in self.raw.items():
            path = Path(self.request['inputs'][key]['path'])
            path.write_bytes(raw)
            path.chmod(0o600)
        request = self.root / 'request.json'
        request.write_bytes(join.canonical(self.request))
        request.chmod(0o600)
        return ['--request-file', str(request), '--request-sha256', join.sha(request.read_bytes())]

    def test_default_off_private_exclusive_output_and_fixed_diagnostics(self):
        args = self.write_inputs()
        output = self.root / 'candidate.json'
        with patch('sys.stdout', io.StringIO()), patch('sys.stderr', io.StringIO()):
            self.assertEqual(join.main(args), 0)
            self.assertFalse(output.exists())
            self.assertEqual(join.main(args+['--prepare','--output',str(output)]), 0)
            raw = output.read_bytes()
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            self.assertEqual(join.main(args+['--prepare','--output',str(output)]), 3)
            self.assertEqual(output.read_bytes(), raw)
        with patch('sys.stdout', io.StringIO()) as out, patch('sys.stderr', io.StringIO()) as err:
            self.assertEqual(join.main(['--unknown-DUMMY_PRIVATE','DUMMY_PRIVATE']), 3)
        self.assertNotIn('DUMMY_PRIVATE', out.getvalue()+err.getvalue())

    def test_candidate_size_is_bounded_in_default_offline_mode_too(self):
        self.update('gcp_capture', lambda v: v['metadata']['pool'].update(description='DUMMY_PADDING'*1000))
        candidate = self.run_join()
        limit = max(len(raw) for raw in self.raw.values())
        self.assertGreater(len(join.canonical(candidate)), limit)
        with patch.object(join, 'MAX_BYTES', limit), self.assertRaises(ValueError): self.run_join()

    def test_output_failure_or_window_expiry_removes_only_new_candidate(self):
        args = self.write_inputs()
        output = self.root / 'candidate.json'
        with patch.object(join, 'now', side_effect=[POINT, POINT, POINT+timedelta(hours=1)]), \
             patch('sys.stdout', io.StringIO()), patch('sys.stderr', io.StringIO()):
            self.assertEqual(join.main(args+['--prepare','--output',str(output)]), 3)
        self.assertFalse(output.exists())
        self.assertTrue((self.root/'owner_task_capture.json').exists())

    def test_owned_private_files_no_links_and_checkout_exclusion(self):
        args = self.write_inputs()
        path = self.root/'aws_current.json'
        with patch('sys.stdout', io.StringIO()), patch('sys.stderr', io.StringIO()):
            path.chmod(0o644)
            self.assertEqual(join.main(args), 3)
            path.chmod(0o600)
            link = self.root/'link'
            os.link(path, link)
            self.assertEqual(join.main(args), 3)
            link.unlink()
            path.rename(link)
            path.symlink_to(link)
            self.assertEqual(join.main(args), 3)
            path.unlink()
            link.rename(path)
            self.root.chmod(0o755)
            self.assertEqual(join.main(args), 3)
            self.root.chmod(0o700)
            marker = self.root/'.git'
            marker.write_text('gitdir: dummy\n')
            self.assertEqual(join.main(args), 3)
            marker.unlink()
        self.assertEqual(path.read_bytes(), self.raw['aws_current'])

    def test_new_candidate_never_becomes_a_legacy_capture_or_packet(self):
        import collect_runtime_read_scope as old_scope
        import runtime_ownership_packet as old_packet
        candidate = self.run_join()
        with self.assertRaises(ValueError): old_scope.validate_capture(candidate, now=POINT)
        with self.assertRaises(ValueError): old_packet.validate_packet(candidate, wave='pool', git_sha='a'*40, now=POINT)


if __name__ == '__main__':
    unittest.main()
