#!/usr/bin/env python3
"""Bounded dummy preview proofs and mocked GitHub GETs; no actual credentials."""
from __future__ import annotations

import copy
from datetime import timedelta
from decimal import Decimal
import hashlib
from io import BytesIO, StringIO
import json
import stat
import struct
import unittest
from unittest.mock import patch
from contextlib import redirect_stdout, redirect_stderr
from urllib.error import HTTPError
import zipfile

import terraform_history_observation_sync_profile as profile
import terraform_history_observation_sync_provenance as gate
import terraform_ownership_receipt as ownership
import terraform_current_state_attestation as legacy
from test_terraform_history_observation_sync_profile import SHA, NOW, dummy_profile
from test_terraform_ownership_receipt import dummy_binding


def dummy_proof():
    request = dummy_profile()
    return dict(schema_version=1, kind=gate.TYPE, repository_id=ownership.REPOSITORY_ID,
        source_sha=SHA, run_id=101, run_attempt=1, job_role='plan',
        environment='terraform-dev-plan', target='dev', root_contract='gcp-dev-default',
        request_id=request['request_id'], issued_utc=request['issued_utc'],
        expires_utc=request['expires_utc'], observed_utc='2026-10-16T00:30:30Z',
        counts=copy.deepcopy(gate.COUNTS), drift_alias='USER_ACTIVITY_HISTORY',
        field_codes=['ETAG', 'LAST_MODIFIED_TIME'],
        commitments={key: hashlib.sha256(key.encode()).hexdigest() for key in gate.COMMITMENT_PURPOSES},
        raw_candidate_state_verified=True, execution_permitted=False, migration_ready=False,
        checks={key: True for key in gate.CHECKS},
        accounting=dict(provider_requests='unknown', actual_cost='unknown'))


def archive(body, *, filename=gate.PUBLIC_FILE, mode=stat.S_IFREG | 0o600,
            extra=b'', comment=b'', second=False, stream=None):
    stream = stream or BytesIO()
    with zipfile.ZipFile(stream, 'w') as output:
        entry = zipfile.ZipInfo(filename)
        entry.external_attr = mode << 16
        entry.compress_type = zipfile.ZIP_DEFLATED
        entry.extra = extra
        output.writestr(entry, body)
        output.comment = comment
        if second:
            output.writestr('other.json', b'{}')
    return stream.getvalue()


class GitHubFixture:
    def __init__(self):
        self.value = dummy_proof()
        self.raw = archive(gate.canonical(self.value) + b'\n')
        self.ref = dict(run_id=101, attempt=1, job_id=201, source_sha=SHA,
            proof_sha256=gate.digest(self.value), transport=gate.TRANSPORT, artifact_id=301,
            artifact_sha256=hashlib.sha256(self.raw).hexdigest())
        self.profile = dummy_profile('synchronize'); self.profile['preview_reference'] = copy.deepcopy(self.ref)
        repository = dict(id=ownership.REPOSITORY_ID, full_name=ownership.REPOSITORY,
                          owner=dict(id=ownership.OWNER_ID))
        self.run = dict(id=101, run_attempt=1, status='completed', conclusion='success',
            event='workflow_dispatch', head_sha=SHA, head_branch=ownership.BRANCH,
            path=ownership.CALLER, workflow_id=gate.WORKFLOW_ID,
            repository=repository, head_repository=copy.deepcopy(repository),
            actor=dict(id=ownership.OWNER_ID), triggering_actor=dict(id=ownership.OWNER_ID),
            referenced_workflows=[dict(path=f'{ownership.REPOSITORY}/{ownership.REUSABLE}@{SHA}',
                                      sha=SHA, ref='refs/heads/' + ownership.BRANCH)])
        self.jobs = [dict(id=201, run_id=101, head_sha=SHA, head_branch=ownership.BRANCH,
            name=gate.JOB_NAME, status='completed', conclusion='success',
            started_at='2026-10-16T00:30:00Z', completed_at='2026-10-16T00:31:00Z',
            steps=[dict(name=name, number=i + 1, status='completed', conclusion='success')
                   for i, name in enumerate(gate.REQUIRED_STEPS)])]
        self.artifact = dict(id=301, name=gate.artifact_name(101, SHA), expired=False,
            size_in_bytes=len(self.raw), digest='sha256:' + self.ref['artifact_sha256'],
            created_at='2026-10-16T00:30:45Z', updated_at='2026-10-16T00:30:45Z',
            expires_at='2026-10-17T00:30:45Z', workflow_run=dict(id=101,
                repository_id=ownership.REPOSITORY_ID, head_repository_id=ownership.REPOSITORY_ID,
                head_sha=SHA, head_branch=ownership.BRANCH))
        self.reviews = [dict(state='approved', user=dict(id=ownership.OWNER_ID),
                            environments=[dict(id=gate.ENVIRONMENT_ID, name='terraform-dev-plan')])]
        self.base = f'/repos/{ownership.REPOSITORY}/actions/runs/101'
        self.artifact_path = f'/repos/{ownership.REPOSITORY}/actions/artifacts/301'
        self.calls = []

    def __call__(self, path, *, archive=False):
        self.calls.append((path, archive))
        if archive:
            if path != self.artifact_path + '/zip':raise AssertionError('unexpected archive target')
            return self.raw
        if path in (self.base, self.base + '/attempts/1'):return copy.deepcopy(self.run)
        if path == self.artifact_path:return copy.deepcopy(self.artifact)
        for suffix, collection, field in (('/attempts/1/jobs', self.jobs, 'jobs'),
                                          ('/artifacts', [self.artifact], 'artifacts')):
            prefix = self.base + suffix + '?per_page=100&page='
            if path.startswith(prefix):
                page = int(path[len(prefix):]); start = (page - 1) * 100
                return dict(total_count=len(collection), **{field: copy.deepcopy(collection[start:start + 100])})
        prefix = self.base + '/approvals?per_page=100&page='
        if path.startswith(prefix):
            page = int(path[len(prefix):]); start = (page - 1) * 100
            return copy.deepcopy(self.reviews[start:start + 100])
        raise AssertionError('unexpected GET target')


class ProvenanceTest(unittest.TestCase):
    def authenticate(self, fixture=None, **kwargs):
        fixture = fixture or GitHubFixture()
        return gate.authenticate(fixture.ref, fixture.profile, request=fixture, now=NOW, **kwargs)

    def test_full_positive_new_type_is_not_existing_issuer_or_execution_permission(self):
        fixture = GitHubFixture()
        value, timing = self.authenticate(fixture)
        self.assertEqual(value, fixture.value)
        self.assertEqual(timing['completed_at'], fixture.jobs[0]['completed_at'])
        self.assertIs(value['execution_permitted'], False)
        for validator in (ownership.validate_receipt, legacy.validate_attestation):
            with self.assertRaises(ValueError):validator(value)
        for validator in (ownership.reference, legacy.reference):
            with self.assertRaises(ValueError):validator(fixture.ref)
        self.assertTrue(all('/logs' not in path for path, _ in fixture.calls))
        self.assertEqual(sum(archive for _, archive in fixture.calls), 1)

    def test_full_jobs_pagination_with_exact_preview_job_on_second_page(self):
        fixture = GitHubFixture()
        filler = [dict(id=1000 + i, run_id=101, head_sha=SHA, name='dummy check ' + str(i),
                       status='completed', conclusion='success', steps=[]) for i in range(100)]
        fixture.jobs = filler + fixture.jobs
        self.authenticate(fixture)
        self.assertEqual(sum('/jobs?per_page=100&page=2' in path for path, _ in fixture.calls), 2)
        def partial(path, **kwargs):
            result = fixture(path, **kwargs)
            if '/jobs?per_page=100&page=1' in path:result['jobs'] = result['jobs'][:99]
            return result
        with self.assertRaises(ValueError):gate.authenticate(fixture.ref, fixture.profile, request=partial, now=NOW)

    def test_strict_proof_schema_counts_flags_budget_and_no_retiming(self):
        mutations = [('kind', legacy.TYPE), ('schema_version', True), ('repository_id', True),
            ('run_id', True), ('run_attempt', True), ('run_attempt', 2), ('job_role', 'apply'),
            ('target', 'prod'), ('environment', 'terraform-dev-apply'), ('drift_alias', 'OTHER'),
            ('field_codes', ['ETAG']), ('raw_candidate_state_verified', False),
            ('execution_permitted', True), ('migration_ready', True), ('nonce', 'DUMMY_PRIVATE'),
            ('issued_utc', '2026-10-16T00:30:31Z'), ('expires_utc', '2026-10-18T00:00:00Z'),
            ('observed_utc', '2026-10-16T01:00:01Z'),
            ('accounting', {'provider_requests': 0, 'actual_cost': 'unknown'})]
        for key, new in mutations:
            with self.subTest(key=key), self.assertRaises(ValueError):
                value = dummy_proof(); value[key] = new; gate.validate_proof(value, now=NOW)
        for group, key, new in [('counts', 'drift', True), ('counts', 'no-op', 13),
                                ('checks', 'native_candidate_verified', 1),
                                ('commitments', 'intent', 'not-hmac')]:
            with self.subTest(group=group, key=key), self.assertRaises(ValueError):
                value = dummy_proof(); value[group][key] = new; gate.validate_proof(value, now=NOW)
        value = dummy_proof(); value['request_id'] = '87654321-1234-4234-8234-123456789abc'
        with self.assertRaises(ValueError):gate.validate_proof(value, profile=dummy_profile(), now=NOW)
        for raw in (b'{"x":1,"x":2}', b'{"x":NaN}', b'{"x":1.0}', b'[' * 40 + b']' * 40):
            with self.assertRaises(ValueError):gate.validate_proof(raw, now=NOW)

    def test_zip_rejects_hidden_payload_noncanonical_crc_links_and_extra(self):
        body = gate.canonical(dummy_proof()) + b'\n'
        def check(raw):
            return gate.archive_proof(raw, hashlib.sha256(raw).hexdigest(), gate.digest(dummy_proof()), now=NOW)
        self.assertEqual(check(archive(body)), dummy_proof())
        invalid = [archive(body, filename='../' + gate.PUBLIC_FILE), archive(body, second=True),
            archive(body, mode=stat.S_IFLNK | 0o600), archive(body, extra=b'\x01\x00\x00\x00'),
            archive(body, comment=b'DUMMY_EXTRA'), archive(body + b' '),
            archive(json.dumps(dummy_proof()).encode()), archive(body + b' ' * 8192),
            archive(body) + b'APPENDED', b'PREPENDED' + archive(body)]
        corrupted = bytearray(archive(body)); corrupted[30 + len(gate.PUBLIC_FILE)] ^= 0xff
        invalid.append(bytes(corrupted))
        encrypted = bytearray(archive(body)); encrypted[6] |= 1
        invalid.append(bytes(encrypted))
        for raw in invalid:
            with self.subTest(size=len(raw)), self.assertRaises((ValueError, zipfile.BadZipFile)):
                check(raw)
        with self.assertRaises(ValueError):gate.archive_proof(archive(body), '0' * 64, '1' * 64, now=NOW)

    def test_streaming_zip_data_descriptor_still_has_one_exact_bounded_entry(self):
        class Nonseekable(BytesIO):
            def seek(self, *args):raise OSError('dummy nonseekable')
        body = gate.canonical(dummy_proof()) + b'\n'
        raw = archive(body, stream=Nonseekable())
        self.assertTrue(struct.unpack_from('<H', raw, 6)[0] & 8)
        self.assertEqual(gate.archive_proof(raw, hashlib.sha256(raw).hexdigest(), gate.digest(dummy_proof()), now=NOW), dummy_proof())

    def test_wrong_run_job_source_owner_environment_or_cleanup_cannot_authenticate(self):
        mutations = [lambda f:f.run.update(event='push'), lambda f:f.run.update(run_attempt=2),
            lambda f:f.run.update(path='.github/workflows/other.yml'), lambda f:f.run.update(workflow_id=1),
            lambda f:f.run['actor'].update(id=1), lambda f:f.run['repository']['owner'].update(id=1),
            lambda f:f.run['referenced_workflows'][0].update(sha='d' * 40),
            lambda f:f.jobs[0].update(id=202), lambda f:f.jobs[0].update(name='renamed'),
            lambda f:f.jobs[0].update(head_sha='d' * 40),
            lambda f:f.jobs[0]['steps'][-1].update(conclusion='failure'),
            lambda f:f.jobs[0]['steps'][6].update(conclusion='skipped'),
            lambda f:f.jobs[0]['steps'][7].update(number=1),
            lambda f:f.reviews[0]['user'].update(id=1),
            lambda f:f.reviews[0].update(state='rejected'),
            lambda f:f.reviews[0]['environments'][0].update(id=True),
            lambda f:f.reviews[0]['environments'][0].update(name='terraform-dev-apply'),
            lambda f:f.artifact.update(expired=True),
            lambda f:f.artifact.update(expires_at='2026-10-18T00:30:45Z'),
            lambda f:f.artifact.update(digest='sha256:' + 'd' * 64)]
        for mutation in mutations:
            with self.subTest(mutation=mutations.index(mutation)), self.assertRaises(ValueError):
                fixture = GitHubFixture(); mutation(fixture); self.authenticate(fixture)
        fixture = GitHubFixture(); fixture.jobs.append(copy.deepcopy(fixture.jobs[0]))
        with self.assertRaises(ValueError):self.authenticate(fixture)
        fixture = GitHubFixture(); fixture.jobs.append(dict(id=999, run_id=101, head_sha=SHA,
            name=gate.APPLY_JOB_NAME, status='completed', conclusion='success', steps=[]))
        with self.assertRaises(ValueError):self.authenticate(fixture)

    def test_unknown_partial_duplicate_pagination_or_changed_final_fails(self):
        for collection in ('jobs', 'artifacts', 'approvals'):
            with self.subTest(collection=collection), self.assertRaises(ValueError):
                fixture = GitHubFixture()
                def unknown(path, **kwargs):
                    result = fixture(path, **kwargs)
                    if '/' + collection + '?' in path:
                        if isinstance(result, dict):result['total_count'] = True
                        else:return None
                    return result
                gate.authenticate(fixture.ref, fixture.profile, request=unknown, now=NOW)
        fixture = GitHubFixture(); fixture.reviews *= 2
        with self.assertRaises(ValueError):self.authenticate(fixture)
        fixture = GitHubFixture(); calls = 0
        def changed(path, **kwargs):
            nonlocal calls
            result = fixture(path, **kwargs)
            if path == fixture.base:
                calls += 1
                if calls == 2:result['conclusion'] = 'failure'
            return result
        with self.assertRaises(ValueError):gate.authenticate(fixture.ref, fixture.profile, request=changed, now=NOW)
        fixture = GitHubFixture()
        with self.assertRaises(ValueError):gate.authenticate(fixture.ref, fixture.profile,
            request=lambda *args, **kwargs: None, now=NOW)
        # A long GET sequence must not carry a proof past its original expiry.
        fixture = GitHubFixture()
        class Clock:
            count = 0
            @classmethod
            def now(cls, _):
                cls.count += 1
                return NOW if cls.count < 5 else NOW + timedelta(days=1)
        with patch.object(gate, 'datetime', Clock), patch.object(profile, 'datetime', Clock):
            # timestamp needs the real parser while now() is controlled.
            import datetime as real_datetime
            Clock.fromisoformat = real_datetime.datetime.fromisoformat
            with self.assertRaises(ValueError):gate.authenticate(fixture.ref, fixture.profile, request=fixture)

    def test_artifacts_and_approvals_visit_all_pages_then_reject_extra_proof_or_review(self):
        fixture = GitHubFixture()
        def many_artifacts(path, **kwargs):
            if '/artifacts?per_page=100&page=' in path:
                fixture.calls.append((path, False))
                page = int(path.rsplit('=', 1)[1])
                start = (page - 1) * 100
                return dict(total_count=101, artifacts=[dict(fixture.artifact, id=400 + i)
                    for i in range(start, min(start + 100, 101))])
            return fixture(path, **kwargs)
        with self.assertRaises(ValueError):
            gate.authenticate(fixture.ref, fixture.profile, request=many_artifacts, now=NOW)
        self.assertTrue(any('/artifacts?per_page=100&page=2' in path for path, _ in fixture.calls))
        self.assertFalse(any(is_archive for _, is_archive in fixture.calls))
        fixture = GitHubFixture()
        fixture.reviews = [dict(copy.deepcopy(fixture.reviews[0]), dummy_review_number=i) for i in range(101)]
        with self.assertRaises(ValueError):self.authenticate(fixture)
        self.assertTrue(any('/approvals?per_page=100&page=2' in path for path, _ in fixture.calls))
        fixture = GitHubFixture()
        def too_many(path, **kwargs):
            response = fixture(path, **kwargs)
            if '/jobs?' in path:response['total_count'] = 1001
            return response
        with self.assertRaises(ValueError):
            gate.authenticate(fixture.ref, fixture.profile, request=too_many, now=NOW)
        self.assertFalse(any('page=2' in path for path, _ in fixture.calls))

    def test_purpose_hmac_binds_nonce_private_values_and_all_domains(self):
        binding = dummy_binding()
        private = {'etag': 'DUMMY_PRIVATE_ETAG', 'raw_state_sha256': 'a' * 64}
        values = {purpose: gate.commitment(binding, purpose, private) for purpose in gate.COMMITMENT_PURPOSES}
        self.assertEqual(len(set(values.values())), len(values))
        changed = copy.deepcopy(binding); changed['nonce'] = 'cd' * 32
        self.assertNotEqual(values['intent'], gate.commitment(changed, 'intent', private))
        self.assertNotEqual(values['intent'], gate.commitment(binding, 'intent', dict(private, etag='OTHER')))
        self.assertNotEqual(values['intent'], ownership.digest(private))
        with self.assertRaises(ValueError):gate.commitment(binding, 'arbitrary', private)

    def test_private_intent_fractional_nested_quota_inputs_are_preserved_only_in_hmac(self):
        binding = dummy_binding()
        value = dict(variables=dict(youtube_quota_policy_settings=dict(value={
            'minute_80': dict(threshold=0.8, duration='60s'),
            'day_80': dict(threshold=0.8, duration='60s'),
            'day_60': dict(threshold=0.6, duration='60s')})),
            nested=[{'fraction': 0.125, 'enabled': True, 'count': 1}, None])
        result = gate.commitment(binding, 'intent', value)
        self.assertRegex(result, r'^[0-9a-f]{64}$')
        self.assertEqual(result, gate.commitment(binding, 'intent', json.loads(json.dumps(value))))
        changed = copy.deepcopy(value)
        changed['variables']['youtube_quota_policy_settings']['value']['day_60']['threshold'] = 0.7
        self.assertNotEqual(result, gate.commitment(binding, 'intent', changed))
        self.assertNotEqual(gate.commitment(binding, 'intent', {'v': True}),
                            gate.commitment(binding, 'intent', {'v': 1}))
        self.assertNotEqual(gate.commitment(binding, 'intent', {'v': 1.0}),
                            gate.commitment(binding, 'intent', {'v': 1}))
        for integer in (2 ** 63 - 1, -(2 ** 63 - 1)):
            self.assertRegex(gate.commitment(binding, 'intent', {'v': integer}), r'^[0-9a-f]{64}$')
        # The separate public profile/proof contracts never acquire float support.
        with self.assertRaises(ValueError):profile.bounded(value)
        request = dummy_profile(); request['max_state_bytes'] = 4194304.0
        with self.assertRaises(ValueError):profile.validate_profile(request, SHA, now=NOW)

    def test_private_intent_nonfinite_nonnumeric_and_large_integer_canaries_rejected_silently(self):
        binding = dummy_binding()
        class IntSubclass(int):pass
        class FloatSubclass(float):pass
        canary = 'DUMMY_PRIVATE_CANARY_DO_NOT_EMIT'
        invalid = [float('nan'), float('inf'), float('-inf'), complex(0.8, 0.6),
            Decimal('0.8'), object(), b'private-bytes', (0.8,), {0.8},
            2 ** 63, -(2 ** 63), IntSubclass(1), FloatSubclass(0.8)]
        invalid.extend(({1: 'wrong-key'}, {True: 'wrong-key'}))
        for bad in invalid:
            stdout, stderr = StringIO(), StringIO()
            with self.subTest(kind=type(bad)), redirect_stdout(stdout), redirect_stderr(stderr):
                with self.assertRaises(ValueError) as raised:
                    gate.commitment(binding, 'intent', {canary: [bad]})
            self.assertNotIn(canary, str(raised.exception))
            self.assertEqual(stdout.getvalue() + stderr.getvalue(), '')

    def test_private_intent_keeps_depth_nodes_string_and_encoded_byte_budgets(self):
        binding = dummy_binding()
        nested = 0.8
        for _ in range(gate.MAX_PRIVATE_INTENT_DEPTH + 1):nested = [nested]
        with self.assertRaises(ValueError):gate.commitment(binding, 'intent', nested)
        cycle = []; cycle.append(cycle)
        with self.assertRaises(ValueError):gate.commitment(binding, 'intent', cycle)
        with self.assertRaises(ValueError):
            gate.commitment(binding, 'intent', [0.8] * gate.MAX_PRIVATE_INTENT_NODES)
        # Smaller test cap exercises the same structural/string/UTF-8 JSON budget
        # without allocating multi-megabyte private buffers in the test runner.
        with patch.object(gate, 'MAX_PRIVATE_INTENT_BYTES', 128):
            self.assertRegex(gate.commitment(binding, 'intent', {'v': 0.8}), r'^[0-9a-f]{64}$')
            for value in ({'v': 'x' * 129}, {'x' * 129: 0.8}, {'v': '\u2603' * 30}):
                with self.assertRaises(ValueError):gate.commitment(binding, 'intent', value)

    def test_transport_allows_only_fixed_get_routes_and_strips_redirect_authorization(self):
        class Response(BytesIO):
            status = 200
        class Opener:
            def __init__(self):self.calls = []
            def open(self, request, timeout):
                self.calls.append((request, timeout))
                if len(self.calls) == 1:
                    raise HTTPError(request.full_url, 302, 'dummy',
                        {'Location': 'https://dummy.actions.githubusercontent.com/signed?dummy=1'}, None)
                return Response(b'DUMMY_ZIP')
        opener = Opener()
        with patch.object(ownership, 'build_opener', return_value=opener):
            reader = gate.GitHubRead('DUMMY_TOKEN')
            base = f'/repos/{ownership.REPOSITORY}/actions/'
            self.assertEqual(reader(base + 'artifacts/301/zip', archive=True), b'DUMMY_ZIP')
            self.assertEqual(opener.calls[0][0].get_method(), 'GET')
            self.assertEqual(opener.calls[0][0].get_header('Authorization'), 'Bearer DUMMY_TOKEN')
            self.assertIsNone(opener.calls[1][0].get_header('Authorization'))
            self.assertEqual(len(opener.calls), 2)
            for path in ('/repos/other/repo/actions/runs/1', base + 'runs/1/logs',
                         base + 'runs/1/attempts/2', base + 'runs/1/artifacts?per_page=100&page=11',
                         base + 'runs/../secrets', base + 'artifacts/1/zip'):
                with self.subTest(path=path), self.assertRaises(ValueError):reader(path)
        self.assertEqual(len(opener.calls), 2)
        for target in ('http://dummy.actions.githubusercontent.com/file', 'https://evil.example/file',
                       'https://user:pass@dummy.blob.core.windows.net/file'):
            class BadOpener:
                calls = 0
                def open(self, request, timeout):
                    self.calls += 1
                    raise HTTPError(request.full_url, 302, 'dummy', {'Location': target}, None)
            bad = BadOpener()
            with patch.object(ownership, 'build_opener', return_value=bad), self.assertRaises(ValueError):
                gate.GitHubRead('DUMMY_TOKEN')(f'/repos/{ownership.REPOSITORY}/actions/artifacts/301/zip', archive=True)
            self.assertEqual(bad.calls, 1)


if __name__ == '__main__':
    unittest.main()
