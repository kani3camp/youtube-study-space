#!/usr/bin/env python3
"""Public dummy current-state, GitHub ZIP and cumulative-chain regressions only."""
from __future__ import annotations

import copy
from datetime import datetime, timedelta, timezone
import hashlib
from io import BytesIO
import json
import os
import re
from pathlib import Path
import stat
import subprocess
import sys
import tempfile
import textwrap
from io import StringIO
import unittest
from unittest.mock import patch
import zipfile

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'infra/gcp/tests'))
import terraform_current_state_attestation as gate
import terraform_ownership_receipt as receipt
import terraform_history_plan_receipt as private
import terraform_history_post_noop_receipt as noop
import runtime_ownership_provenance as provenance
import runtime_ownership_packet as packet_module
import prepare_runtime_ownership as preparer
import validate_runtime_ownership_plan as graph
import terraform_runtime_execution as runtime
from test_terraform_ownership_receipt import SHA, dummy_binding, dummy_catalog, dummy_context, dummy_receipt, GitHubFixture
import test_terraform_history_post_noop_receipt as noop_fixture
import test_terraform_history_plan_receipt as state_fixture
import test_terraform_authenticated_workflow as workflow_fixture
from test_runtime_ownership_package import packet, PROJECT

NOW = datetime(2026, 10, 10, 1, 4, tzinfo=timezone.utc)


def dummy_attestation():
    binding = dummy_binding()
    return dict(schema_version=1, attestation_type=gate.TYPE, repository_id=receipt.REPOSITORY_ID,
        source_sha=SHA, run_id=101, run_attempt=1, job_role='plan', environment='terraform-dev-plan',
        target='dev', root_contract='gcp-dev-default', operation='protected-plan', phase='current-state',
        historical_delta_attested=False, issued_utc='2026-10-10T00:00:00Z',
        observed_utc='2026-10-10T01:00:30Z', expires_utc='2026-10-10T23:00:00Z',
        resource_count=12, terraform_version='1.16.4', counts=copy.deepcopy(noop.EXPECTED_COUNTS),
        checks={k: True for k in gate.CHECKS}, scope_commitment=receipt.commitment(binding, gate.scope(binding)),
        state_after_commitment='c'*64, state_observation_commitment='d'*64,
        accounting=dict(terraform_outcomes=dict(init='success', plan='success', validation='success', plan_exit_code=0),
            explicit_state_reads=10, explicit_metadata_reads=2, provider_requests='unknown',
            workflow_requests='unknown', actual_cost='unknown'))


def bootstrap_catalog():
    return dict(schema_version=1, issuers={SHA: dict(workflow_id=19,
        job_names=dict(plan='dummy protected plan', apply='dummy protected apply'),
        environment=dict(id=31, name='terraform-dev-plan'), reviewer_ids=[17],
        attestation_step=gate.EMIT_STEP, publisher_step=gate.PUBLISH_STEP,
        required_steps=copy.deepcopy(gate.REQUIRED_STEPS), purpose=gate.TYPE)})


class CurrentStateFixture(GitHubFixture):
    def __init__(self, value=None):
        super().__init__()
        self.value = value or dummy_attestation()
        self.ref.update(receipt_sha256=receipt.digest(self.value), job_id=self.jobs[0]['id'], transport=gate.TRANSPORT)
        self.jobs[0]['steps'] = [dict(name=s, number=i+1, status='completed', conclusion='success')
                                 for i, s in enumerate(gate.REQUIRED_STEPS)]
        self.jobs[1].update(conclusion='skipped', steps=[])
        self.install_current_archive(receipt.canonical(self.value) + b'\n')

    def install_current_archive(self, body, *, name=gate.PUBLIC_FILE, extra=False, mode=0):
        stream = BytesIO()
        with zipfile.ZipFile(stream, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
            entry = zipfile.ZipInfo(name)
            entry.external_attr = mode << 16
            archive.writestr(entry, body)
            if extra:
                archive.writestr('private-state.json', b'DUMMY_PRIVATE_STATE')
        raw = stream.getvalue()
        self.ref['artifact_sha256'] = hashlib.sha256(raw).hexdigest()
        artifact = self.responses[self.artifact_path]
        artifact.update(name=gate.artifact_name(101, SHA), size_in_bytes=len(raw),
                        digest='sha256:'+self.ref['artifact_sha256'], created_at=self.jobs[0]['started_at'],
                        updated_at=self.jobs[0]['completed_at'])
        self.responses[self.archive_path] = raw


def v3_packet(index=0):
    value = packet(index)
    value.pop('history_post_noop_run_id')
    root = CurrentStateFixture()
    value.update(schema_version=3, current_state_attestation=root.ref,
                 receipt_binding=dummy_binding(), adoption_receipts=[],
                 reviewed_utc='2026-10-10T01:03:00Z', expires_utc='2026-10-10T02:00:00Z')
    if index:
        selected = dict(pool=True, provider=False, grants=[], apis=[])
        scope = provenance.scope(value['receipt_binding'], preparer.prepare(value['inventory']), selected, version=3)
        child = dummy_receipt(run_id=201, wave='pool', count=13,
            scope=receipt.commitment(value['receipt_binding'], scope), prior=receipt.digest(root.value),
            prior_scope=root.value['scope_commitment'])
        child['prior_state_commitment'] = root.value['state_after_commitment']
        fake = GitHubFixture(child, minute=5)
        value['adoption_receipts'].append(dict(reference=fake.ref, wave='pool', selected=selected))
        root.responses.update(fake.responses)
    return value, root


class CurrentStateTest(unittest.TestCase):
    def setUp(self):
        for module in (preparer, graph):
            p = patch.object(module, 'PROJECT', PROJECT)
            p.start()
            self.addCleanup(p.stop)

    def test_separate_schema_counts_freshness_and_historical_claims(self):
        gate.validate_attestation(dummy_attestation())
        mutations = [dict(operation='apply'), dict(phase='post'), dict(attestation_type='dev-history12-v1'),
            dict(historical_delta_attested=True), dict(resource_count=True), dict(resource_count=11),
            dict(resource_count=13), dict(terraform_version='1.16.3'), dict(run_attempt=2),
            dict(environment='terraform-dev-apply'), dict(import_before_count=1),
            dict(backend='DUMMY_PRIVATE'), dict(expires_utc='2026-10-11T00:00:01Z'),
            dict(observed_utc='2026-10-09T23:00:00Z')]
        for change in mutations:
            with self.subTest(change=change), self.assertRaises(ValueError):
                gate.validate_attestation(dummy_attestation() | change)
        for field in noop.EXPECTED_COUNTS:
            value = dummy_attestation()
            value['counts'][field] += 1
            with self.subTest(count=field), self.assertRaises(ValueError):
                gate.validate_attestation(value)
        for value in (dummy_attestation(), dummy_receipt()):
            wrong = receipt.validate_receipt if 'attestation_type' in value else gate.validate_attestation
            with self.assertRaises(ValueError):
                wrong(value)
        with self.assertRaises(ValueError):
            receipt.decode('{"schema_version":1,"schema_version":1}')
        with self.assertRaises(ValueError):
            receipt.decode('{"x":NaN}')

    def test_get_only_plan_provenance_and_unique_publication(self):
        fake = CurrentStateFixture()
        value, timing = gate.authenticate(fake.ref, bootstrap_catalog(), request=fake, now=NOW)
        self.assertEqual(value, fake.value)
        self.assertEqual(timing['completed_at'], fake.jobs[0]['completed_at'])
        self.assertEqual(len(fake.calls), 10)
        self.assertEqual(sum(archive for _, archive in fake.calls), 1)
        self.assertFalse(any('/logs' in path for path, _ in fake.calls))
        self.assertNotEqual(gate.scope(dummy_binding()), receipt.history_scope(dummy_binding()))
        with self.assertRaises(ValueError):
            receipt.authenticate(fake.ref, dummy_catalog(), request=fake)
        with self.assertRaises(ValueError):
            gate.authenticate(GitHubFixture().ref, bootstrap_catalog(), request=fake)
        with self.assertRaises(ValueError):
            receipt.authenticate(fake.ref | dict(transport=receipt.TRANSPORT), dummy_catalog(), request=fake)

    def test_reviewed_source_catalog_pins_only_the_successful_current_state_issuer(self):
        source = '94786489c90666889dd4efe5f303d1444bc8c430'
        catalog = gate.load_catalog(receipt.decode(provenance.BOOTSTRAP_CATALOG.read_bytes()))
        self.assertEqual(catalog, dict(schema_version=1, issuers={source: dict(
            workflow_id=325439161,
            job_names=dict(plan='GCP Terraform Authenticated / Authenticated protected plan',
                           apply='GCP Terraform Authenticated / Protected validated apply'),
            environment=dict(id=23410074023, name='terraform-dev-plan'), reviewer_ids=[54093651],
            attestation_step=gate.EMIT_STEP, publisher_step=gate.PUBLISH_STEP,
            required_steps=gate.REQUIRED_STEPS, purpose=gate.TYPE)}))
        self.assertEqual(receipt.decode(provenance.CATALOG.read_bytes()), dict(schema_version=1, issuers={}))

        # Only public dummy run/ZIP data goes through the actual reviewed catalog.
        fake = CurrentStateFixture(dummy_attestation() | dict(source_sha=source))
        fake.ref['source_sha'] = source
        issuer = catalog['issuers'][source]
        for path in (fake.base, fake.base+'/attempts/1'):
            run = fake.responses[path]
            run.update(head_sha=source, workflow_id=issuer['workflow_id'])
            run['referenced_workflows'][0]['sha'] = source
        for job, role in zip(fake.jobs, ('plan', 'apply')):
            job.update(head_sha=source, name=issuer['job_names'][role])
        approval = fake.responses[fake.base+'/approvals'][0]
        approval['user']['id'] = issuer['reviewer_ids'][0]
        approval['environments'] = [copy.deepcopy(issuer['environment'])]
        artifact = fake.responses[fake.artifact_path]
        artifact['name'] = gate.artifact_name(fake.ref['run_id'], source)
        artifact['workflow_run']['head_sha'] = source
        value, _ = gate.authenticate(fake.ref, catalog, request=fake, now=NOW)
        self.assertEqual(value, fake.value)
        self.assertIs(value['historical_delta_attested'], False)
        for unpinned in (SHA, 'b'*40):
            with self.subTest(source=unpinned), self.assertRaises(ValueError):
                gate.authenticate(fake.ref | dict(source_sha=unpinned), catalog,
                                  request=lambda *_: self.fail('unpinned source must stop before transport'), now=NOW)
        with self.assertRaises(ValueError):
            receipt.authenticate(fake.ref, receipt.decode(provenance.CATALOG.read_bytes()),
                                 request=lambda *_: self.fail('bootstrap must never become an apply issuer'))

    def test_issuer_catalog_is_distinct_strict_and_empty_catalog_stops_before_transport(self):
        self.assertEqual(receipt.decode(provenance.CATALOG.read_bytes()), dict(schema_version=1, issuers={}))
        gate.load_catalog(bootstrap_catalog())
        for mutation in [lambda c: c['issuers'][SHA].update(purpose='history12'),
                         lambda c: c['issuers'][SHA].update(reviewer_ids=[]),
                         lambda c: c['issuers'][SHA]['environment'].update(id=True),
                         lambda c: c['issuers'][SHA]['required_steps'].reverse(),
                         lambda c: c['issuers'][SHA].update(waves=['history12'])]:
            value = bootstrap_catalog()
            mutation(value)
            with self.assertRaises(ValueError):
                gate.load_catalog(value)
        fake = CurrentStateFixture()
        with self.assertRaises(ValueError):
            gate.authenticate(fake.ref, dict(schema_version=1, issuers={}), request=fake)
        self.assertEqual(fake.calls, [])

    def test_rerun_wrong_source_job_approval_apply_cleanup_or_step_order_rejected(self):
        for failure in ('rerun', 'branch', 'caller', 'reusable', 'source', 'job-id', 'duplicate-plan',
                        'apply', 'renamed-apply', 'cleanup', 'step-order', 'step-duplicate', 'publisher',
                        'environment-id', 'reviewer', 'approval', 'unreviewed-source'):
            fake = CurrentStateFixture()
            run = fake.responses[fake.base]
            steps = fake.jobs[0]['steps']
            if failure == 'rerun': run['run_attempt'] = 2
            elif failure == 'branch': run['head_branch'] = 'dev'
            elif failure == 'caller': run['path'] = receipt.REUSABLE
            elif failure == 'reusable': run['referenced_workflows'][0]['ref'] = 'refs/heads/dev'
            elif failure == 'source': run['referenced_workflows'][0]['sha'] = 'b'*40
            elif failure == 'job-id': fake.ref['job_id'] += 100
            elif failure == 'duplicate-plan':
                duplicate = copy.deepcopy(fake.jobs[0]); duplicate['id'] += 10
                fake.jobs.append(duplicate)
                fake.responses[fake.base+'/attempts/1/jobs?per_page=100&page=1']['total_count'] += 1
            elif failure in {'apply', 'renamed-apply'}:
                fake.jobs[1].update(conclusion='success', steps=[dict(name='Apply the locally re-created saved plan', conclusion='success')])
                if failure == 'renamed-apply': fake.jobs[1]['name'] = 'dummy evasion'
            elif failure == 'cleanup': steps[-1]['conclusion'] = 'failure'
            elif failure == 'step-order': steps[-2]['number'], steps[-1]['number'] = steps[-1]['number'], steps[-2]['number']
            elif failure == 'step-duplicate': steps.append(dict(steps[-1], number=99))
            elif failure == 'publisher': fake.jobs[1].update(name='dummy evasion', steps=[dict(name=gate.PUBLISH_STEP, conclusion='success')])
            elif failure == 'environment-id': fake.responses[fake.base+'/approvals'][0]['environments'][0]['id'] = 999
            elif failure == 'reviewer': fake.responses[fake.base+'/approvals'][0]['user']['id'] = 999
            elif failure == 'approval': fake.responses[fake.base+'/approvals'][0]['state'] = 'rejected'
            elif failure == 'unreviewed-source': fake.ref['source_sha'] = 'b'*40
            with self.subTest(failure=failure), self.assertRaises(ValueError):
                gate.authenticate(fake.ref, bootstrap_catalog(), request=fake, now=NOW)

    def test_zip_schema_canonical_bytes_secrecy_limits_and_encryption(self):
        body = receipt.canonical(dummy_attestation()) + b'\n'
        for name, extra, mode in [(gate.PUBLIC_FILE, True, 0), ('../'+gate.PUBLIC_FILE, False, 0),
            ('/'+gate.PUBLIC_FILE, False, 0), ('./'+gate.PUBLIC_FILE, False, 0),
            (gate.PUBLIC_FILE, False, stat.S_IFLNK | 0o777), (gate.PUBLIC_FILE, False, stat.S_IFDIR | 0o700)]:
            fake = CurrentStateFixture()
            fake.install_current_archive(body, name=name, extra=extra, mode=mode)
            with self.subTest(name=name, extra=extra, mode=mode), self.assertRaises(ValueError):
                gate.archive_attestation(fake.responses[fake.archive_path], fake.ref['artifact_sha256'], fake.ref['receipt_sha256'])
        for invalid in (json.dumps(dummy_attestation()).encode(), body[:-1], body+b' ',
                        receipt.canonical(dummy_attestation() | dict(nonce='DUMMY_PRIVATE'))+b'\n', b'x'*16385):
            fake = CurrentStateFixture(); fake.install_current_archive(invalid)
            with self.assertRaises((ValueError, json.JSONDecodeError)):
                gate.archive_attestation(fake.responses[fake.archive_path], fake.ref['artifact_sha256'], fake.ref['receipt_sha256'])
        fake = CurrentStateFixture()
        raw = fake.responses[fake.archive_path]
        encrypted = bytearray(raw); central = encrypted.index(b'PK\x01\x02')
        encrypted[6] |= 1; encrypted[central+8] |= 1
        raw_encrypted = bytes(encrypted)
        for data, digest in [(raw_encrypted, hashlib.sha256(raw_encrypted).hexdigest()),
                             (raw, 'e'*64), (b'x'*65537, 'e'*64)]:
            with self.assertRaises(ValueError): gate.archive_attestation(data, digest, fake.ref['receipt_sha256'])
        self.assertEqual(gate.archive_attestation(raw, fake.ref['artifact_sha256'], fake.ref['receipt_sha256']), fake.value)
        for forbidden in ('nonce', 'bucket', 'VersionId', 'ETag', 'raw_state_sha256', '111111111111', 'DUMMY_PRIVATE'):
            self.assertNotIn(forbidden.encode(), body)

    def test_missing_duplicate_expired_digest_and_final_read_races_stop(self):
        for failure in ('missing', 'multiple', 'expired', 'digest', 'size', 'metadata-race', 'listing-race', 'run-race'):
            fake = CurrentStateFixture()
            if failure == 'missing': fake.responses[fake.base+'/artifacts?per_page=100&page=1'] = dict(total_count=0, artifacts=[])
            elif failure == 'multiple': fake.responses[fake.base+'/artifacts?per_page=100&page=1']['total_count'] = 2
            elif failure == 'expired': fake.responses[fake.artifact_path]['expired'] = True
            elif failure == 'digest': fake.responses[fake.artifact_path]['digest'] = 'sha256:'+'e'*64
            elif failure == 'size': fake.responses[fake.artifact_path]['size_in_bytes'] = 65537
            endpoint = fake.artifact_path if failure == 'metadata-race' else (
                fake.base+'/artifacts?per_page=100&page=1' if failure == 'listing-race' else fake.base)
            def request(path, **kwargs):
                result = fake(path, **kwargs)
                if failure.endswith('-race') and path == endpoint and sum(p == endpoint for p, _ in fake.calls) == 2:
                    if failure == 'metadata-race': result['expired'] = True
                    elif failure == 'listing-race': result['total_count'] = 2
                    else: result['run_attempt'] = 2
                return result
            with self.subTest(failure=failure), self.assertRaises(ValueError):
                gate.authenticate(fake.ref, bootstrap_catalog(), request=request, now=NOW)

    def test_v3_root_and_first_child_keep_scope_state_order_and_deadline(self):
        for index in (0, 1):
            value, fake = v3_packet(index)
            result = provenance.verify_chain(value, dummy_catalog(), bootstrap_catalog=bootstrap_catalog(), request=fake, now=NOW)
            self.assertEqual(result['root_state_observation_commitment'], fake.value['state_observation_commitment'])
            self.assertEqual(len(fake.calls), 10 * (index+1))
            self.assertEqual(result['last_receipt_sha256'], packet_module.root_reference(value)['receipt_sha256']
                if index == 0 else value['adoption_receipts'][-1]['reference']['receipt_sha256'])
        for mutation in [lambda p: p.update(schema_version=2),
            lambda p: p['current_state_attestation'].update(transport=receipt.TRANSPORT),
            lambda p: p['receipt_binding'].update(nonce='cd'*32),
            lambda p: p['receipt_binding']['backend'].update(bucket='dummy-other'),
            lambda p: p['receipt_binding']['history_metadata']['schema']['fields'].reverse(),
            lambda p: p['adoption_receipts'][0]['selected'].update(provider=True),
            lambda p: p['adoption_receipts'][0].update(reference=p['current_state_attestation'])]:
            value, fake = v3_packet(1); mutation(value)
            with self.assertRaises((ValueError, KeyError)):
                provenance.verify_chain(value, dummy_catalog(), bootstrap_catalog=bootstrap_catalog(), request=fake, now=NOW)
        value, fake = v3_packet()
        legacy = copy.deepcopy(value)
        legacy.update(schema_version=2, history_receipt=legacy.pop('current_state_attestation'))
        legacy['history_receipt']['transport'] = receipt.TRANSPORT
        with self.assertRaises(ValueError):
            provenance.verify_chain(legacy, dummy_catalog(), request=fake, now=NOW)
        with self.assertRaises(ValueError):
            provenance.verify_chain(value, dummy_catalog(), bootstrap_catalog=bootstrap_catalog(), request=fake,
                                    now=datetime(2026, 10, 10, 23, tzinfo=timezone.utc))
        # An adopted chain may outlive initial admission; its first plan/apply
        # must have started before the root expiry and after root completion.
        value, fake = v3_packet(1)
        value.update(reviewed_utc='2026-10-11T00:00:00Z', expires_utc='2026-10-11T01:00:00Z')
        later = datetime(2026, 10, 11, 0, 30, tzinfo=timezone.utc)
        provenance.verify_chain(value, dummy_catalog(), bootstrap_catalog=bootstrap_catalog(), request=fake, now=later)
        for role in (0, 1):
            value, fake = v3_packet(1)
            jobs = fake.responses['/repos/'+receipt.REPOSITORY+'/actions/runs/201/attempts/1/jobs?per_page=100&page=1']['jobs']
            jobs[role]['started_at'] = '2026-10-10T23:00:00Z'
            with self.assertRaises(ValueError):
                provenance.verify_chain(value, dummy_catalog(), bootstrap_catalog=bootstrap_catalog(), request=fake, now=NOW)

    def test_first_state_join_catches_same_json_different_bytes_and_head(self):
        value, _ = v3_packet()
        raw, head = b'{"dummy":1}', dict(VersionId='DUMMY_VERSION', ETag='DUMMY_ETAG', ContentLength=11)
        verified = dict(root_expires_utc='2026-10-10T23:00:00Z',
            root_state_observation_commitment=gate.observation_commitment(value['receipt_binding'], head, raw))
        runtime.first_state_join(value, verified, head, raw, now=NOW)
        for changed_head, changed_raw in [(head, b'{ "dummy": 1 }'), (head | dict(VersionId='DUMMY_OTHER'), raw),
                                          (head | dict(ETag='DUMMY_OTHER'), raw), (head | dict(ContentLength=99), raw)]:
            with self.assertRaises(ValueError): runtime.first_state_join(value, verified, changed_head, changed_raw, now=NOW)
        with self.assertRaises(ValueError):
            runtime.first_state_join(value, verified, head, raw, now=datetime(2026, 10, 10, 23, tzinfo=timezone.utc))

    def attestation_fixture(self, *, legacy=False):
        fixture = noop_fixture.IndependentHistoryPostNoopTest()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        binding = dummy_binding()
        binding['history_metadata'] = copy.deepcopy(fixture.metadata)
        envelope = dict(schema_version=1, receipt_binding=binding)
        if not legacy:
            envelope['attestation_type'] = gate.TYPE
        fixture.env |= dummy_context() | dict(MODE='plan', GITHUB_JOB='plan',
            CURRENT_STATE_ATTESTATION='true', DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED='true',
            OWNERSHIP_WAVE='none', OWNERSHIP_ROOT_BASIS='original-history-import',
            RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(envelope))
        gate.stage_binding(fixture.env)
        fixture.env.pop('RUNTIME_OWNERSHIP_PACKET_JSON')
        return fixture

    def test_staging_preserves_exact_legacy_and_native_binding_and_freezes_input(self):
        for legacy in (True, False):
            fixture = self.attestation_fixture(legacy=legacy)
            path = fixture.root / gate.BINDING
            staged = private.private_json(path)
            self.assertEqual(set(staged), {'schema_version', 'attestation_type', 'receipt_binding'})
            self.assertEqual(staged['attestation_type'], gate.TYPE)
            self.assertEqual(staged['receipt_binding'], dummy_binding() | dict(history_metadata=fixture.metadata))
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            path.unlink()
            original = copy.deepcopy(staged)
            if legacy:
                original.pop('attestation_type')
            with patch.dict(os.environ, fixture.env | dict(RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(original)), clear=True), \
                    patch('sys.argv', ['attestation', 'stage-binding']), patch('sys.stdout', StringIO()) as out, \
                    patch('sys.stderr', StringIO()) as err:
                self.assertEqual(gate.main(), 0)
                self.assertEqual(out.getvalue(), 'Private current-state binding checked; values suppressed.\n')
                self.assertEqual(err.getvalue(), '')
            # Later env changes cannot replace the single private source.
            self.assertEqual(gate.check_binding(fixture.env | dict(RUNTIME_OWNERSHIP_PACKET_JSON='invalid'), fresh=True),
                             staged['receipt_binding'])
            with self.assertRaises(Exception):
                gate.stage_binding(fixture.env | dict(RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(staged)))
            self.assertEqual(private.private_json(path), staged)
            self.assertEqual(fixture.aws.calls, [])

    def test_staging_rejects_wrong_shapes_types_bindings_and_ambiguous_json_without_output(self):
        fixture = self.attestation_fixture()
        path = fixture.root / gate.BINDING
        native = private.private_json(path)
        path.unlink()
        legacy = {key: value for key, value in native.items() if key != 'attestation_type'}
        invalid = ['', '{}', 'null', '[]', '{"schema_version":1,"schema_version":1}',
                   '{"schema_version":NaN}', 'x' * (1024 * 1024 + 1), json.dumps(dummy_attestation()),
                   json.dumps(dummy_receipt()), json.dumps(packet(0))]
        for original in (legacy, native):
            for mutation in (lambda p: p.update(schema_version=True), lambda p: p.update(schema_version=2),
                             lambda p: p.update(extra='DUMMY_PRIVATE'), lambda p: p.pop('receipt_binding'),
                             lambda p: p.update(attestation_type='dev-history12-v1'),
                             lambda p: p['receipt_binding'].update(nonce='00'*32),
                             lambda p: p['receipt_binding'].update(extra='DUMMY_PRIVATE'),
                             lambda p: p['receipt_binding']['backend'].update(bucket='dummy-other'),
                             lambda p: p['receipt_binding']['backend'].update(key='dummy-other'),
                             lambda p: p['receipt_binding']['history_metadata'].update(schema={'fields': []})):
                changed = copy.deepcopy(original)
                mutation(changed)
                invalid.append(json.dumps(changed))
        for raw in invalid:
            with self.subTest(raw_size=len(raw)), patch.dict(os.environ, fixture.env | dict(RUNTIME_OWNERSHIP_PACKET_JSON=raw), clear=True), \
                    patch('sys.argv', ['attestation', 'stage-binding']), patch('sys.stdout', StringIO()) as out, \
                    patch('sys.stderr', StringIO()) as err:
                self.assertEqual(gate.main(), 3)
                self.assertEqual(out.getvalue(), '')
                self.assertEqual(err.getvalue(), 'STOP: current-state attestation rejected; private diagnostic suppressed.\n')
                self.assertFalse(path.exists())
        self.assertEqual(fixture.aws.calls, [])

    def test_staging_context_and_private_file_boundaries_fail_closed(self):
        fixture = self.attestation_fixture()
        path = fixture.root / gate.BINDING
        native = private.private_json(path)
        path.unlink()
        env = fixture.env | dict(RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(native))
        for change in (dict(MODE='apply'), dict(GITHUB_JOB='apply'), dict(HISTORY_TARGET='prod'),
                       dict(CURRENT_STATE_ATTESTATION='false'), dict(HISTORY_POST_NOOP='false'),
                       dict(DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED='false'), dict(OWNERSHIP_WAVE='pool'),
                       dict(OWNERSHIP_ROOT_BASIS='current-state-bootstrap'), dict(GITHUB_RUN_ATTEMPT='2'),
                       dict(RUNNER_TEMP=str(ROOT)), dict(RUNNER_TEMP='relative')):
            with self.subTest(change=change), self.assertRaises(Exception):
                gate.stage_binding(env | change)
            self.assertFalse(path.exists())
        target = fixture.root / 'dummy-target'
        with patch.object(gate.os, 'geteuid', return_value=-1), self.assertRaises(Exception):
            gate.stage_binding(env)
        self.assertFalse(path.exists())
        target.write_text('DUMMY_UNCHANGED')
        path.symlink_to(target)
        with self.assertRaises(Exception): gate.stage_binding(env)
        with self.assertRaises(Exception): gate.check_binding(env)
        self.assertEqual(target.read_text(), 'DUMMY_UNCHANGED')
        path.unlink()
        link = fixture.root / 'dummy-root-link'
        link.symlink_to(fixture.root, target_is_directory=True)
        with self.assertRaises(Exception): gate.stage_binding(env | dict(RUNNER_TEMP=str(link)))
        with self.assertRaises(Exception): gate.check_binding(env)
        gate.stage_binding(env)
        path.chmod(0o644)
        with self.assertRaises(Exception): gate.check_binding(env)
        path.chmod(0o600)
        path.write_text(json.dumps({key: native[key] for key in ('schema_version', 'receipt_binding')}))
        with self.assertRaises(Exception): gate.check_binding(env)
        self.assertEqual(fixture.aws.calls, [])

    def test_staged_binding_keeps_fresh_metadata_strictness_and_legacy_contract_separate(self):
        fixture = self.attestation_fixture(legacy=True)
        value = private.private_json(fixture.root / gate.BINDING)
        legacy = {key: value[key] for key in ('schema_version', 'receipt_binding')}
        with self.assertRaises(ValueError): gate.validate_binding_envelope(legacy)
        old_env = fixture.env | dict(MODE='apply', DEV_HISTORY_RECEIPT_EMITTER_ENABLED='true',
                                     RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(legacy))
        self.assertEqual(receipt.history_binding(old_env), value['receipt_binding'])
        with self.assertRaises(ValueError):
            receipt.history_binding(old_env | dict(RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(value)))
        for key, changed in [('numRows', '123'), ('numCurrentPhysicalBytes', '90')]:
            metadata = copy.deepcopy(fixture.metadata) | {key: changed}
            (fixture.root / 'user-history-post.json').write_text(json.dumps(metadata))
            gate.check_binding(fixture.env, fresh=True)
        for mutation in (lambda m: m.update(creationTime='99999'), lambda m: m.update(unknown='DUMMY_CONFIG'),
                         lambda m: m['schema']['fields'].reverse()):
            metadata = copy.deepcopy(fixture.metadata)
            mutation(metadata)
            (fixture.root / 'user-history-post.json').write_text(json.dumps(metadata))
            with self.assertRaises(Exception): gate.check_binding(fixture.env, fresh=True)

    def test_real_post_noop_record_emit_and_zip_join_with_private_dummy_state_only(self):
        for legacy in (True, False):
            with self.subTest(legacy=legacy):
                self.check_post_noop_record_emit_and_zip(legacy=legacy)

    def check_post_noop_record_emit_and_zip(self, *, legacy):
        fixture = self.attestation_fixture(legacy=legacy)
        with patch.object(gate, 'datetime') as clock:
            clock.now.return_value = state_fixture.NOW
            noop.before(fixture.env, request=fixture.aws)
            noop.after(fixture.env, request=fixture.aws, metadata_request=fixture.table_get)
        value = private.private_json(fixture.root / gate.RECORD)
        gate.validate_attestation(value)
        with patch('sys.stdout', StringIO()) as output:
            gate.emit(fixture.env)
        body = (fixture.root / gate.PUBLIC_DIRECTORY / gate.PUBLIC_FILE).read_bytes()
        self.assertEqual(body, receipt.canonical(value)+b'\n')
        self.assertEqual((fixture.root / gate.PUBLIC_DIRECTORY / gate.PUBLIC_FILE).stat().st_mode & 0o777, 0o600)
        self.assertEqual(len(fixture.aws.calls), 10)
        binding = private.private_json(fixture.root / gate.BINDING)['receipt_binding']
        head = private.private_json(fixture.root / noop.BEFORE)['head']
        raw = private.private_bytes(fixture.root / noop.STATE_AFTER)
        self.assertEqual(value['state_after_commitment'], receipt.state_commitment(binding, private.decode(raw)))
        self.assertEqual(value['state_observation_commitment'], gate.observation_commitment(binding, head, raw))
        for forbidden in ('DUMMY_', 'dummy-state-bucket', binding['nonce'], 'VersionId', 'ETag', 'state_sha256'):
            self.assertNotIn(forbidden, body.decode()+output.getvalue())
        with self.assertRaises(FileExistsError): gate.emit(fixture.env)

    def test_actual_workflow_cleanup_removes_staged_input_on_success_and_failure(self):
        workflow = workflow_fixture.WORKFLOW.read_text()
        plan = re.split(r'\n  [a-z][a-z0-9_-]*:\n', workflow.split('  plan:\n', 1)[1], maxsplit=1)[0]
        cleanup = plan.split('      - name: Cleanup sensitive temporary files', 1)[1]
        script = textwrap.dedent(cleanup.split('        run: |\n', 1)[1])
        for valid in (True, False):
            fixture = self.attestation_fixture(legacy=True)
            if not valid:
                (fixture.root / gate.BINDING).write_text('DUMMY_INVALID_PARTIAL_INPUT')
            checkout = fixture.root / 'dummy-checkout'
            checkout.mkdir()
            preserved = checkout / 'unrelated'
            preserved.write_text('DUMMY_PRESERVED')
            directory = fixture.root / gate.PUBLIC_DIRECTORY
            directory.mkdir()
            private.write_private(directory / gate.PUBLIC_FILE, {})
            result = subprocess.run(['bash', '-e', '-c', script], capture_output=True, text=True,
                                    env=dict(os.environ, RUNNER_TEMP=str(fixture.root), GITHUB_WORKSPACE=str(checkout)))
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(result.stdout, '')
            self.assertFalse((fixture.root / gate.BINDING).exists())
            self.assertFalse(directory.exists())
            self.assertEqual(preserved.read_text(), 'DUMMY_PRESERVED')

    def test_invalid_state_and_outcomes_never_create_current_state_record(self):
        for failure in ('11', '13', 'duplicate-container', 'duplicate-instance', 'empty-container', 'provider',
                        'project', 'table', 'schema', 'deletion-protection', 'failed-check', 'unknown-check', 'version'):
            fixture = self.attestation_fixture()
            state = private.decode(fixture.aws.body)
            table = state['resources'][-1]
            attributes = table['instances'][0]['attributes']
            if failure == '11': state['resources'].pop()
            elif failure == '13': state['resources'].append(copy.deepcopy(table) | dict(name='extra'))
            elif failure == 'duplicate-container': state['resources'].append(copy.deepcopy(table))
            elif failure == 'duplicate-instance': table['instances'].append(copy.deepcopy(table['instances'][0]))
            elif failure == 'empty-container': state['resources'].append(copy.deepcopy(table) | dict(name='extra', instances=[]))
            elif failure == 'provider': table['provider'] = 'dummy-foreign-provider'
            elif failure == 'project': attributes['project'] = 'dummy-other-project'
            elif failure == 'table': attributes['table_id'] = 'dummy-other-table'
            elif failure == 'schema': attributes['schema'] = '[]'
            elif failure == 'deletion-protection': attributes['deletion_protection'] = False
            elif failure == 'failed-check': state['check_results'][-1]['status'] = 'fail'
            elif failure == 'unknown-check': state['check_results'][-1]['status'] = 'unknown'
            elif failure == 'version': state['terraform_version'] = '1.16.3'
            fixture.aws.body = json.dumps(state).encode()
            with patch.object(gate, 'datetime') as clock, self.subTest(failure=failure), self.assertRaises(Exception):
                clock.now.return_value = state_fixture.NOW
                noop.before(fixture.env, request=fixture.aws)
                noop.after(fixture.env, request=fixture.aws, metadata_request=fixture.table_get)
            self.assertFalse((fixture.root / gate.RECORD).exists())
        for failure in ('bytes', 'head', 'serial', 'lineage', 'outputs', 'metadata', 'lock', 'workspace', 'denied', 'truncated',
                        'plan', 'init', 'validation', 'summary-extra', 'summary-version', 'summary-count-type', 'expiry'):
            fixture = self.attestation_fixture()
            noop.before(fixture.env, request=fixture.aws)
            state = private.decode(fixture.aws.body)
            if failure == 'bytes': fixture.aws.body = json.dumps(state, indent=2).encode()
            elif failure == 'head': fixture.aws.version = 'DUMMY_OTHER_VERSION'
            elif failure == 'serial': state['serial'] += 1
            elif failure == 'lineage': state['lineage'] = '00000000-0000-0000-0000-000000000001'
            elif failure == 'outputs': state['outputs']['project_id']['value'] = 'dummy-other'
            elif failure == 'metadata': fixture.metadata['creationTime'] = '99999'
            elif failure == 'lock': fixture.aws.lock = True
            elif failure == 'workspace': fixture.aws.workspaces = True
            elif failure == 'denied': fixture.aws.denied = 'get-object'
            elif failure == 'truncated': fixture.aws.overrides['list-objects-v2'] = dict(IsTruncated=True)
            elif failure in {'plan', 'init', 'validation'}:
                fixture.env[{'plan':'TERRAFORM_PLAN_OUTCOME', 'init':'BACKEND_INIT_OUTCOME', 'validation':'PLAN_VALIDATION_OUTCOME'}[failure]] = 'failure'
            elif failure.startswith('summary'):
                path = fixture.root / 'sanitized-plan.json'; summary = private.private_json(path)
                if failure == 'summary-extra': summary['untrusted'] = True
                elif failure == 'summary-version': summary['terraform_version'] = '1.16.3'
                else: summary['counts']['import'] = False
                path.write_text(json.dumps(summary))
            if failure in {'serial', 'lineage', 'outputs'}: fixture.aws.body = json.dumps(state).encode()
            metadata_calls = []
            def read(*args, **kwargs):
                metadata_calls.append(True)
                return fixture.table_get(*args, **kwargs)
            clock_time = state_fixture.NOW if failure != 'expiry' else state_fixture.NOW+timedelta(hours=2)
            actual_policy = state_fixture.ACTUAL_EXECUTION_POLICY
            with patch.object(gate, 'datetime') as clock, patch.object(private, 'execution_policy',
                side_effect=lambda raw, sha, now=None: actual_policy(raw, sha, now=now or clock_time)):
                clock.now.return_value = clock_time
                with self.subTest(failure=failure), self.assertRaises(Exception):
                    noop.after(fixture.env, request=fixture.aws, metadata_request=read)
            self.assertEqual(len(metadata_calls), 1)
            self.assertEqual([call[1] for call in fixture.aws.calls][-2:], ['list-objects-v2', 'list-objects-v2'])
            self.assertFalse((fixture.root / gate.RECORD).exists())

    def test_v3_protected_staging_catalog_and_real_runtime_before_consumer(self):
        for job in ('plan', 'apply'):
            with self.subTest(job=job):
                self.check_runtime_consumer(job)

    def check_runtime_consumer(self, job):
        from test_terraform_runtime_execution import WaveFixture
        fixture = WaveFixture(0)
        with tempfile.TemporaryDirectory() as directory:
            env, catalog = fixture.stage(directory, job=job)
            root = Path(directory)
            packet_value = fixture.packet
            packet_value.pop('history_receipt')
            packet_value['schema_version'] = 3
            current = datetime.now(timezone.utc).replace(microsecond=0)
            fmt = lambda t: t.strftime('%Y-%m-%dT%H:%M:%SZ')
            value = dummy_attestation() | dict(issued_utc=fmt(current-timedelta(minutes=10)),
                observed_utc=fmt(current-timedelta(minutes=5)), expires_utc=fmt(current+timedelta(hours=1)),
                state_after_commitment=receipt.state_commitment(packet_value['receipt_binding'], fixture.old))
            head = dict(ContentLength=len(fixture.aws.body), VersionId=fixture.aws.version,
                        ETag='"dummy-etag"', ServerSideEncryption='AES256', Metadata={})
            value['state_observation_commitment'] = gate.observation_commitment(packet_value['receipt_binding'], head, fixture.aws.body)
            fake = CurrentStateFixture(value)
            fake.jobs[0].update(started_at=fmt(current-timedelta(minutes=6)), completed_at=fmt(current-timedelta(minutes=4)))
            fake.install_current_archive(receipt.canonical(value)+b'\n')
            fixture.fakes = [fake]
            packet_value['current_state_attestation'] = fake.ref
            env.update(OWNERSHIP_ROOT_BASIS='current-state-bootstrap', DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY='true',
                       DEV_HISTORY_POST_NOOP12_READY='false', RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(packet_value))
            for name in ('runtime-ownership-packet.json', 'runtime-ownership.tfvars.json', provenance.VERIFIED):
                (root/name).unlink()
            with self.assertRaises(ValueError):
                packet_module.prepare_workflow(env | dict(OWNERSHIP_ROOT_BASIS='original-history-import'))
            packet_module.prepare_workflow(env)
            bootstrap_path = root/'bootstrap.json'; bootstrap_path.write_text(json.dumps(bootstrap_catalog()))
            with patch.object(provenance, 'CATALOG', catalog), patch.object(provenance, 'BOOTSTRAP_CATALOG', bootstrap_path):
                provenance.verify_workflow(env, request=fixture.github)
                provenance.verify_workflow(env, request=fixture.github, recheck=True)
                env['EXPECTED_RUNTIME_INPUT_COMMITMENT'] = receipt.commitment(packet_value['receipt_binding'],
                    runtime.bundle(env, root, packet_value))
                fixture.begin(env)
                fixture.approve(env)
                runtime.require_fresh_inputs(env, packet_value)
                verified = private.private_json(root/provenance.VERIFIED)
                self.assertEqual(verified['last_receipt_sha256'], receipt.digest(value))
                self.assertEqual(verified['adopted_scope_commitment'], value['scope_commitment'])
                self.assertEqual(verified['last_state_after_commitment'], value['state_after_commitment'])
                with self.assertRaises(ValueError): runtime.context(env | dict(DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY='false'))
                if job == 'apply':
                    fixture.seal(env)
                    fixture.finish(env)
                    with patch('sys.stdout', StringIO()):
                        receipt.emit_runtime(env | dict(DEV_OWNERSHIP_RECEIPT_ARTIFACT_ENABLED='true'))
                    child = receipt.validate_receipt(private.private_json(root/runtime.RECORD))
                    self.assertEqual(child['prior_receipt_sha256'], receipt.digest(value))
                    self.assertEqual(child['prior_scope_commitment'], value['scope_commitment'])
                    self.assertEqual(child['prior_state_commitment'], value['state_after_commitment'])
                    expected_scope = provenance.scope(packet_value['receipt_binding'], preparer.prepare(packet_value['inventory']),
                                                     packet_value['selected'], version=3)
                    self.assertEqual(child['scope_commitment'], receipt.commitment(packet_value['receipt_binding'], expected_scope))

    def test_workflow_order_single_file_and_cleanup_bind_the_current_state_contract(self):
        workflow = workflow_fixture.WORKFLOW.read_text()
        plan = workflow.split('  plan:\n', 1)[1].split('  apply:\n', 1)[0]
        positions = [plan.index(name) for name in gate.REQUIRED_STEPS]
        self.assertEqual(positions, sorted(positions))
        self.assertLess(plan.index('Check private current-state binding'), plan.index('Configure AWS backend credential'))
        cleanup = plan.split('      - name: Cleanup sensitive temporary files', 1)[1]
        self.assertIn('if: always()', cleanup)
        for filename in (gate.BINDING, gate.RECORD, noop.BEFORE, noop.STATE_BEFORE, noop.STATE_AFTER,
                         'history-post-noop-table-after.json', gate.PUBLIC_DIRECTORY+'/'+gate.PUBLIC_FILE):
            self.assertIn('"${RUNNER_TEMP}/'+filename+'"', cleanup)
        self.assertIn('DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED: "true"', workflow)
        self.assertIn('DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED: "false"', workflow)
        self.assertIn('DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY: "false"', workflow)
        self.assertIn('DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED: "false"', workflow)
        stage = plan.split('      - name: Check private current-state binding before credentials', 1)[1].split('      - name:', 1)[0]
        self.assertIn('RUNTIME_OWNERSHIP_PACKET_JSON: ${{ secrets.GCP_RUNTIME_OWNERSHIP_PACKET_JSON }}', stage)
        self.assertIn('terraform_current_state_attestation.py stage-binding', stage)
        for name in ('Check fresh current-state metadata binding', 'Verify independent post-import no-op and unchanged state', gate.EMIT_STEP):
            step = plan.split('      - name: '+name, 1)[1].split('      - name:', 1)[0]
            self.assertNotIn('RUNTIME_OWNERSHIP_PACKET_JSON', step)

    def test_public_dummy_fixture_has_no_authentication_and_is_never_an_issuer(self):
        import terraform_current_state_artifact_fixture as fixture
        text = (ROOT/'.github/workflows/gcp-current-state-artifact-fixture.yml').read_text()
        for forbidden in ('workflow_dispatch:', 'pull_request_target', 'schedule:', 'environment:', 'secrets.',
                          'id-token:', 'actions: write', 'aws-actions/', 'google-github-actions/'):
            self.assertNotIn(forbidden, text)
        self.assertIn('persist-credentials: false', text)
        self.assertIn('retention-days: 1', text)
        self.assertIn('if: always()', text)
        with tempfile.TemporaryDirectory() as directory:
            env = dict(RUNNER_TEMP=directory, GITHUB_EVENT_NAME='pull_request', GITHUB_REPOSITORY=receipt.REPOSITORY,
                       GITHUB_JOB='current-state-artifact-fixture', GITHUB_RUN_ATTEMPT='1')
            fixture.prepare(env)
            body = (Path(directory)/'current-state-fixture-public'/gate.PUBLIC_FILE).read_bytes()
            self.assertEqual(body, receipt.canonical(dummy_attestation())+b'\n')
            with self.assertRaises(FileExistsError): fixture.prepare(env)
        with tempfile.TemporaryDirectory() as directory, self.assertRaises(ValueError):
            fixture.prepare(env | dict(RUNNER_TEMP=directory, GITHUB_EVENT_NAME='workflow_dispatch'))

    def test_selector_combination_and_closed_gates_stop_in_unauthenticated_preflight(self):
        fixture = workflow_fixture.TerraformAuthenticatedWorkflowTest(); fixture.setUpClass()
        selected = fixture.source_gates() | dict(CURRENT_STATE_ATTESTATION='true', HISTORY_POST_NOOP='true')
        self.assertEqual(fixture.run_preflight(**selected).returncode, 0)
        selected['DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED'] = 'false'
        self.assertNotEqual(fixture.run_preflight(**selected).returncode, 0)
        selected['DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED'] = 'true'
        self.assertEqual(fixture.run_preflight(**selected).returncode, 0)
        for change in [dict(TARGET='prod'), dict(MODE='apply'), dict(HISTORY_POST_NOOP='false'),
                       dict(OWNERSHIP_WAVE='pool'), dict(HISTORY_IMPORT_APPROVAL='DUMMY_APPROVAL'),
                       dict(OWNERSHIP_ROOT_BASIS='current-state-bootstrap'), dict(GITHUB_RUN_ATTEMPT='2')]:
            with self.subTest(change=change):
                self.assertNotEqual(fixture.run_preflight(**(selected | change)).returncode, 0)
        selected = fixture.source_gates() | dict(OWNERSHIP_WAVE='pool', OWNERSHIP_ROOT_BASIS='current-state-bootstrap',
            DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED='true', DEV_HISTORY_POST_NOOP12_READY='true')
        self.assertNotEqual(fixture.run_preflight(**selected).returncode, 0)
        selected.update(DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY='true', DEV_HISTORY_POST_NOOP12_READY='false')
        self.assertEqual(fixture.run_preflight(**selected).returncode, 0)
        self.assertNotEqual(fixture.run_preflight(**(selected | dict(OWNERSHIP_ROOT_BASIS='original-history-import'))).returncode, 0)


if __name__ == '__main__':
    unittest.main()
