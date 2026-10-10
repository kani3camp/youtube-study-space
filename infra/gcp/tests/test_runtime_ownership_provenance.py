#!/usr/bin/env python3
"""Cumulative provenance with public dummy GitHub execution fixtures only."""
from __future__ import annotations
import copy
from datetime import datetime, timedelta, timezone
from io import StringIO
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
for path in (ROOT / '.github/scripts', ROOT / 'infra/gcp/scripts', Path(__file__).parent):
    sys.path.insert(0, str(path))
import runtime_ownership_provenance as gate
import runtime_ownership_packet as packet_module
import prepare_runtime_ownership as preparer
import validate_runtime_ownership_plan as graph
import terraform_history_plan_receipt as private
import terraform_ownership_receipt as receipt
from test_terraform_ownership_receipt import dummy_binding, dummy_catalog, dummy_context, dummy_receipt, GitHubFixture, SHA
from test_runtime_ownership_package import packet, stages, PROJECT


def fixture(index=0):
    value = packet(index)
    now = datetime.now(timezone.utc)
    value.update(schema_version=2, receipt_binding=dummy_binding(), adoption_receipts=[],
                 reviewed_utc=(now-timedelta(minutes=1)).strftime('%Y-%m-%dT%H:%M:%SZ'),
                 expires_utc=(now+timedelta(hours=1)).strftime('%Y-%m-%dT%H:%M:%SZ'))
    value.pop('history_post_noop_run_id')
    history = GitHubFixture()
    value['history_receipt'] = history.ref
    candidate = preparer.prepare(value['inventory'])
    previous = history.value
    fakes = [history]
    for step, (wave, _, selected) in enumerate(stages()[:index]):
        current = dummy_receipt(run_id=201+step, wave=wave,
            scope=receipt.commitment(value['receipt_binding'], gate.scope(value['receipt_binding'], candidate, selected)),
            prior=receipt.digest(previous), prior_scope=previous['scope_commitment'],
            count=12+int(selected['pool'])+int(selected['provider'])+len(selected['grants'])+len(selected['apis']))
        fake = GitHubFixture(current, minute=5*(step+1))
        value['adoption_receipts'].append(dict(reference=fake.ref, wave=wave, selected=copy.deepcopy(selected)))
        fakes.append(fake)
        previous = current
    responses = {path: response for fake in fakes for path, response in fake.responses.items()}
    calls = []
    def request(path, *, archive=False):
        calls.append((path, archive))
        response = responses[path]
        if isinstance(response, Exception):
            raise response
        return copy.deepcopy(response)
    return value, request, responses, calls


class RuntimeProvenanceTest(unittest.TestCase):
    def setUp(self):
        for target in (preparer, graph):
            p = patch.object(target, 'PROJECT', PROJECT)
            p.start()
            self.addCleanup(p.stop)

    def test_all_cumulative_edges_authenticate_scope_and_prior_receipts(self):
        for index in range(6):
            value, request, _, calls = fixture(index)
            with self.subTest(wave=value['wave']):
                result = gate.verify_chain(value, dummy_catalog(), request=request)
                refs = [value['history_receipt']] + [i['reference'] for i in value['adoption_receipts']]
                self.assertEqual(result['last_receipt_sha256'], refs[-1]['receipt_sha256'])
                self.assertEqual(len(calls), 10 * len(refs))

    def test_history_reference_plan_only_wrong_scope_and_nonce_rejected(self):
        for mutate in [lambda p: p.update(schema_version=1),
                       lambda p: p['history_receipt'].update(receipt_sha256='b'*64),
                       lambda p: p['receipt_binding'].update(nonce='cd'*32),
                       lambda p: p['receipt_binding']['backend'].update(bucket='dummy-other-state'),
                       lambda p: p['receipt_binding']['history_metadata']['schema']['fields'].reverse()]:
            value, request, _, _ = fixture()
            mutate(value)
            with self.subTest(case=mutate), self.assertRaises(ValueError):
                gate.verify_chain(value, dummy_catalog(), request=request)

    def test_skip_duplicate_cycle_batch_removal_and_early_api_rejected(self):
        mutations = [lambda p: p['adoption_receipts'].pop(0),
            lambda p: p['adoption_receipts'][1].update(reference=p['history_receipt']),
            lambda p: p['adoption_receipts'].reverse(),
            lambda p: p['adoption_receipts'][2]['selected'].update(grants=['grant-01', 'grant-02']),
            lambda p: p['adoption_receipts'][3]['selected'].update(grants=['grant-02']),
            lambda p: p['adoption_receipts'][1].update(wave='api'),
            lambda p: p['adopted'].update(apis=[])]
        for mutate in mutations:
            value, request, _, _ = fixture(5)
            mutate(value)
            with self.subTest(case=mutations.index(mutate)), self.assertRaises(ValueError):
                gate.verify_chain(value, dummy_catalog(), request=request)

    def test_same_count_member_condition_trust_and_dependency_swaps_rejected(self):
        mutations = [lambda p: p['inventory']['runtime_grants']['grant-01'].update(member=p['inventory']['runtime_grants']['grant-02']['member']),
            lambda p: p['inventory']['iam_policy']['bindings'][1]['condition'].update(expression='true'),
            lambda p: p['inventory']['pool'].update(description='dummy changed description'),
            lambda p: p['inventory']['api_classification']['pubsub.googleapis.com'].update(dependency_addresses=['module.other'])]
        for mutate in mutations:
            value, request, _, _ = fixture(5)
            mutate(value)
            with self.subTest(case=mutations.index(mutate)), self.assertRaises(ValueError):
                gate.verify_chain(value, dummy_catalog(), request=request)

    def test_prior_digest_scope_count_and_chronology_rejected_even_with_matching_reference_digest(self):
        for mutation in [dict(prior_receipt_sha256='b'*64), dict(prior_scope_commitment='b'*64),
                         dict(resource_count=99), dict(wave='api'), dict(prior_state_commitment='b'*64)]:
            value, request, responses, _ = fixture(1)
            edge = value['adoption_receipts'][0]
            ref = edge['reference']
            artifact_path = f"/repos/{receipt.REPOSITORY}/actions/artifacts/{ref['artifact_id']}"
            machine = receipt.archive_receipt(responses[artifact_path+'/zip'], ref['artifact_sha256'], ref['receipt_sha256']) | mutation
            changed = GitHubFixture(machine, minute=5)
            edge['reference'].update(changed.ref)
            for path in (artifact_path, artifact_path+'/zip', changed.base+'/artifacts?per_page=100&page=1'):
                responses[path] = changed.responses[path]
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                gate.verify_chain(value, dummy_catalog(), request=request)
        value, request, responses, _ = fixture(1)
        jobs = next(v['jobs'] for k, v in responses.items() if '/runs/201/' in k and '/jobs?' in k)
        jobs[1]['started_at'] = '2026-10-10T00:00:00Z'
        with self.assertRaises(ValueError):
            gate.verify_chain(value, dummy_catalog(), request=request)

    def staged(self, root, value):
        candidate = packet_module.validate_packet(value, wave=value['wave'], git_sha=SHA)
        for name, data in [('runtime-ownership-packet.json', value), ('runtime-ownership.tfvars.json', candidate)]:
            private.write_private(Path(root)/name, data)
        return dummy_context() | dict(MODE='plan', GITHUB_JOB='plan', GITHUB_RUN_ID='999', OWNERSHIP_WAVE=value['wave'],
            RUNNER_TEMP=root, STATE_ACCOUNT_ID='111111111111', STATE_BUCKET='dummy-state-bucket',
            STATE_KEY='youtube-study-space/dev/terraform.tfstate', STATE_AWS_REGION='ap-northeast-1', TF_WORKSPACE='default')

    def test_workflow_record_is_private_bound_and_rechecks_remote_metadata(self):
        value, request, responses, calls = fixture(5)
        with tempfile.TemporaryDirectory() as root:
            env = self.staged(root, value)
            catalog = Path(root)/'catalog.json'
            catalog.write_text(json.dumps(dummy_catalog()))
            with patch.object(gate, 'CATALOG', catalog):
                first = gate.verify_workflow(env, request=request)
                self.assertEqual((Path(root)/gate.VERIFIED).stat().st_mode & 0o777, 0o600)
                self.assertEqual(gate.verify_workflow(env, request=request, recheck=True), first)
                self.assertEqual(len(calls), 120)
                gate.require_verified_inputs(env, value, value['receipt_binding']['history_metadata'])
                responses[f'/repos/{receipt.REPOSITORY}/actions/runs/201']['run_attempt'] = 2
                with self.assertRaises(ValueError):
                    gate.verify_workflow(env, request=request, recheck=True)
                for change in [dict(GITHUB_RUN_ID='998'), dict(GITHUB_SHA='b'*40), dict(STATE_BUCKET='dummy-other-state')]:
                    with self.assertRaises(ValueError):
                        gate.require_verified_inputs(env | change, value, value['receipt_binding']['history_metadata'])

    def test_empty_issuer_catalog_backend_mismatch_and_legacy_stop_without_token_or_network(self):
        value, request, _, calls = fixture()
        with tempfile.TemporaryDirectory() as root:
            env = self.staged(root, value)
            with self.assertRaises(ValueError):
                gate.verify_workflow(env, request=request)
            self.assertEqual(calls, [])
            with self.assertRaises(ValueError):
                gate.verify_workflow(env | dict(STATE_BUCKET='dummy-other-state'), request=request)
            self.assertEqual(calls, [])
            with self.assertRaises(ValueError):
                gate.require_verified_inputs(env, packet(), value['receipt_binding']['history_metadata'])
            self.assertFalse((Path(root)/gate.VERIFIED).exists())

    def test_submission_of_fake_verified_boolean_or_copied_receipt_is_rejected(self):
        value, request, _, _ = fixture()
        with tempfile.TemporaryDirectory() as root:
            env = self.staged(root, value)
            catalog = Path(root)/'catalog.json'
            catalog.write_text(json.dumps(dummy_catalog()))
            private.write_private(Path(root)/gate.VERIFIED, dict(authenticated=True, packet=value))
            with patch.object(gate, 'CATALOG', catalog), self.assertRaises(ValueError):
                gate.require_verified_inputs(env, value, value['receipt_binding']['history_metadata'])
            value['history_receipt']['receipt_json'] = dummy_receipt()
            with self.assertRaises(ValueError):
                packet_module.validate_packet(value, wave=value['wave'], git_sha=SHA)

    def test_cli_failure_is_value_free_and_has_no_transport_for_empty_catalog(self):
        value, _, _, _ = fixture()
        with tempfile.TemporaryDirectory() as root:
            env = self.staged(root, value) | dict(GH_TOKEN='DUMMY_PRIVATE_TOKEN')
            with patch.dict(os.environ, env, clear=True), patch('sys.argv', ['receipt']), \
                 patch.object(receipt, 'GitHubRead', side_effect=AssertionError('no network')), \
                 patch('sys.stdout', StringIO()) as out, patch('sys.stderr', StringIO()) as err:
                self.assertEqual(gate.main(), 3)
            for secret in ['DUMMY_PRIVATE_TOKEN', 'dummy-state-bucket', value['receipt_binding']['nonce'], '111111111111']:
                self.assertNotIn(secret, out.getvalue()+err.getvalue())


    def test_legacy_packet_stays_offline_and_cannot_stage_protected_inputs(self):
        old = packet()
        packet_module.validate_packet(old, wave=old['wave'], git_sha=SHA, now=__import__('test_runtime_ownership_package').NOW)
        with tempfile.TemporaryDirectory() as root, self.assertRaises(ValueError):
            packet_module.prepare_workflow(dict(MODE='plan', TF_VAR_project_id='test-youtube-study-space',
                TF_VAR_manage_user_activity_history='true', OWNERSHIP_WAVE='pool', GITHUB_SHA=SHA,
                RUNNER_TEMP=root, RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(old)))

    def test_fresh_history_metadata_must_join_the_authentic_private_binding(self):
        value, request, _, _ = fixture()
        with tempfile.TemporaryDirectory() as root:
            env = self.staged(root, value)
            catalog = Path(root)/'catalog.json'
            catalog.write_text(json.dumps(dummy_catalog()))
            with patch.object(gate, 'CATALOG', catalog):
                gate.verify_workflow(env, request=request)
                changed = copy.deepcopy(value['receipt_binding']['history_metadata'])
                changed['schema']['fields'].reverse()
                with self.assertRaises(ValueError):
                    gate.require_verified_inputs(env, value, changed)


    def test_authenticated_v2_chain_reaches_full_root_consumer_without_mocking_provenance(self):
        from test_terraform_runtime_execution import WaveFixture
        import terraform_runtime_execution as runtime
        value = WaveFixture(4)
        with tempfile.TemporaryDirectory() as root:
            env, catalog = value.stage(root, job='plan')
            with patch.object(gate, 'CATALOG', catalog):
                value.begin(env)
                value.approve(env)
                public = (Path(root)/'sanitized-replan.json.md').read_text()
                for sentinel in [value.packet['receipt_binding']['nonce'], 'dummy private sentinel', '222222222222', 'pubsub.googleapis.com']:
                    self.assertNotIn(sentinel, public)
                saved = private.private_json(str(Path(root)/gate.VERIFIED))
                saved['packet_sha256'] = 'b'*64
                (Path(root)/gate.VERIFIED).write_text(json.dumps(saved))
                with self.assertRaises(ValueError):
                    runtime.require_fresh_inputs(env, value.packet)


if __name__ == '__main__':
    unittest.main()
