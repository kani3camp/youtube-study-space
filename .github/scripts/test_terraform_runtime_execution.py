#!/usr/bin/env python3
"""Adversarial dummy full-state/apply receipts; all cloud transports are synthetic."""
from __future__ import annotations
import copy
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import re
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'infra/gcp/tests'))
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
import terraform_runtime_execution as runtime
import terraform_ownership_receipt as receipt
import terraform_history_plan_receipt as private
import terraform_protected_plan as protected
import runtime_ownership_provenance as provenance
import runtime_ownership_packet as packet_module
import prepare_runtime_ownership as preparer
import validate_runtime_ownership_plan as graph
from test_runtime_ownership_package import fixture, stages, PROJECT, EMAIL
from test_terraform_ownership_receipt import dummy_binding, dummy_catalog, dummy_context, dummy_receipt, GitHubFixture, SHA
from test_terraform_history_plan_receipt import FakeS3, state_fixture


def full_state(plan, selected=None):
    state = state_fixture()
    state['resources'] = []
    grouped = {}
    for r in plan['resource_changes']:
        if selected is not None and r['address'] not in private.BASELINE | {runtime.TABLE} | set(selected):
            continue
        address = r['address']
        module, tail = address.rsplit('.google_', 1) if '.google_' in address else ('', address[7:])
        kind, name = ('google_' + tail).split('.', 1)
        index = None
        if '[' in name:
            name, suffix = name.split('[', 1)
            index = json.loads(suffix[:-1])
        base = (module, kind, name)
        container = grouped.setdefault(base, dict(module=module, mode='managed', type=kind, name=name,
            provider='provider["registry.terraform.io/hashicorp/google"]', instances=[]))
        if not module:
            container.pop('module', None)
        instance = dict(schema_version=4 if kind == 'google_storage_bucket' else 0,
                        attributes=copy.deepcopy(r['change']['after']))
        if r['address'] in private.BASELINE:
            instance['attributes'].setdefault('project', 'test-youtube-study-space')
        if index is not None:
            instance['index_key'] = index
        container['instances'].append(instance)
    state['resources'] = list(grouped.values())
    return state


def malformed_containers(source):
    """Public dummy mutations, including equal flattened-address attacks."""
    def grant(state):
        return next(r for r in state['resources'] if r['type'] == 'google_service_account_iam_member')
    def split(state):
        original = grant(state)
        duplicate = copy.deepcopy(original)
        duplicate['instances'] = [original['instances'].pop()]
        state['resources'].append(duplicate)
    mutations = [
        ('hidden-empty-unapproved', lambda s: s['resources'].append(dict(module='module.runtime_wif', mode='data',
            type='unapproved_resource', name='hidden', provider='unapproved-provider', instances=[]))),
        ('hidden-empty-managed-runtime', lambda s: s['resources'].append(dict(module='module.runtime_wif', mode='managed',
            type='unapproved_resource', name='hidden', provider='provider["registry.terraform.io/hashicorp/google"]', instances=[]))),
        ('hidden-empty-managed-api', lambda s: s['resources'].append(dict(module='module.owned_apis', mode='managed',
            type='unapproved_resource', name='hidden', provider='provider["registry.terraform.io/hashicorp/google"]', instances=[]))),
        ('empty-expected', lambda s: grant(s).update(instances=[])),
        ('split-grant-container', split),
        ('duplicate-instance', lambda s: grant(s)['instances'].append(copy.deepcopy(grant(s)['instances'][0]))),
        ('unknown-header', lambda s: grant(s).update(unapproved=True)),
        ('data-mode', lambda s: grant(s).update(mode='data')),
        ('foreign-provider', lambda s: grant(s).update(provider='unapproved-provider')),
        ('foreign-type', lambda s: grant(s).update(type='unapproved_resource')),
        ('foreign-name', lambda s: grant(s).update(name='unapproved')),
        ('invalid-module', lambda s: grant(s).update(module=None)),
        ('invalid-type', lambda s: grant(s).update(type=[])),
        ('invalid-name', lambda s: grant(s).update(name={})),
        ('non-list-instances', lambda s: grant(s).update(instances={})),
        ('non-object-instance', lambda s: grant(s)['instances'].__setitem__(0, None)),
        ('missing-schema', lambda s: grant(s)['instances'][0].pop('schema_version')),
        ('wrong-schema', lambda s: grant(s)['instances'][0].update(schema_version=1)),
        ('boolean-schema', lambda s: grant(s)['instances'][0].update(schema_version=True)),
        ('invalid-attributes', lambda s: grant(s)['instances'][0].update(attributes=[])),
        ('null-index', lambda s: grant(s)['instances'][0].update(index_key=None)),
        ('boolean-index', lambda s: grant(s)['instances'][0].update(index_key=True)),
        ('compound-index', lambda s: grant(s)['instances'][0].update(index_key=[])),
        ('wrong-index', lambda s: grant(s)['instances'][0].update(index_key='unapproved')),
    ]
    for case, mutate in mutations:
        state = copy.deepcopy(source)
        mutate(state)
        yield case, state


class WaveFixture:
    def __init__(self, index=0, *, previous=None):
        self.index = index
        self.plan, self.metadata, self.packet = fixture(index)
        now = datetime.now(timezone.utc)
        self.packet.pop('history_post_noop_run_id')
        self.packet.update(schema_version=2, receipt_binding=dummy_binding(), adoption_receipts=[],
            reviewed_utc=(now-timedelta(minutes=1)).strftime('%Y-%m-%dT%H:%M:%SZ'),
            expires_utc=(now+timedelta(hours=1)).strftime('%Y-%m-%dT%H:%M:%SZ'))
        p = self.packet
        candidate = preparer.prepare(p['inventory'])
        chosen = p['adopted']
        candidate.update(own_runtime_wif_pool=chosen['pool'], own_runtime_wif_provider=chosen['provider'],
            runtime_wif_grant_keys=chosen['grants'], owned_api_keys=chosen['apis'])
        self.old = full_state(self.plan, graph.identities(candidate))
        self.new = full_state(self.plan)
        self.new['serial'] = self.old['serial'] + int(index != 5)
        if previous is None:
            # Shape/authenticated dummy root is explicit, never actual adoption proof.
            baseline = full_state(self.plan, {})
            history = dummy_receipt()
            history['state_after_commitment'] = receipt.state_commitment(p['receipt_binding'], baseline)
            self.fakes = [GitHubFixture(history)]
            for step, (wave, _, selected) in enumerate(stages()[:index]):
                prior = self.fakes[-1].value
                current = dummy_receipt(run_id=201+step, wave=wave, count=12+step+1,
                    scope=receipt.commitment(p['receipt_binding'], provenance.scope(p['receipt_binding'], preparer.prepare(p['inventory']), selected)),
                    prior=receipt.digest(prior), prior_scope=prior['scope_commitment'])
                current['prior_state_commitment'] = prior['state_after_commitment']
                stage_plan = fixture(step)[0]
                stage_state = full_state(stage_plan)
                current['state_after_commitment'] = receipt.state_commitment(p['receipt_binding'], stage_state)
                self.fakes.append(GitHubFixture(current, minute=(step+1)*5))
        else:
            self.fakes = previous
        p['history_receipt'] = self.fakes[0].ref
        for step, fake in enumerate(self.fakes[1:]):
            p['adoption_receipts'].append(dict(reference=fake.ref, wave=stages()[step][0], selected=stages()[step][2]))
        # Chain test chooses actual state carried by preceding runtime emitter.
        if previous is not None and len(previous)>1:
            self.old['serial'] = 27 + index
            self.new['serial'] = self.old['serial'] + int(index != 5)
        self.aws = FakeS3()
        self.aws.body = json.dumps(self.old).encode()
        self.metadata_calls = []
        self.metadata_mutation = None

    def github(self, path, *, archive=False):
        responses = {k: v for fake in self.fakes for k, v in fake.responses.items()}
        return copy.deepcopy(responses[path])

    def google(self, path, token, *, host):
        self.metadata_calls.append((host, path))
        inventory = self.packet['inventory']
        if host == 'cloudresourcemanager.googleapis.com':
            value = inventory['project']
        elif host == 'serviceusage.googleapis.com':
            value = inventory['enabled_services']
        elif host == 'bigquery.googleapis.com':
            value = self.metadata
        elif ':listAttestationRules' in path:
            value = inventory['pool_attestation_rules']
        elif ':getIamPolicy' in path:
            value = inventory['iam_policy']
        elif '/providers/' in path:
            value = inventory['provider']
        else:
            value = inventory['pool']
        value = copy.deepcopy(value)
        if self.metadata_mutation:
            self.metadata_mutation(host, path, value)
        return 200, value

    def stage(self, root, *, job='apply', mode=None, expected=None):
        root = Path(root)
        p = self.packet
        env = dummy_context() | dict(RUNNER_TEMP=str(root), GITHUB_JOB=job, GITHUB_RUN_ID=str(201+self.index),
            MODE=mode or ('apply' if job=='apply' else 'plan'), OWNERSHIP_WAVE=p['wave'], HISTORY_POST_NOOP='false',
            STATE_ACCOUNT_ID='111111111111', STATE_BUCKET='dummy-state-bucket', STATE_KEY=private.STATE_KEY,
            STATE_AWS_REGION='ap-northeast-1', AWS_MAX_ATTEMPTS='1', AWS_RETRY_MODE='standard', TF_WORKSPACE='default',
            TF_VAR_manage_user_activity_history='true', TF_VAR_export_function_execution_service_account_email=EMAIL,
            TF_VAR_manage_primary_email='true', TF_VAR_manage_youtube_quota_alerts='true',
            TF_VAR_primary_email_address=EMAIL, TF_VAR_primary_email_channel_name='dummy-channel',
            DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED='true', DEV_HISTORY_POST_NOOP12_READY='true',
            DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED='true', GCP_SMOKE_ACCESS_TOKEN='DUMMY_PRIVATE_TOKEN',
            GITHUB_OUTPUT=str(root/'output'), GITHUB_STEP_SUMMARY=str(root/'summary'),
            **{'TF_VAR_manage_export_'+kind:'true' for kind in ('function','scheduler','topic')})
        (root/'output').write_text('')
        env['RUNTIME_OWNERSHIP_PACKET_JSON'] = json.dumps(p)
        packet_module.prepare_workflow(env)
        history = protected.prepare(self.metadata)
        history.pop('manage_user_activity_history')
        private.write_private(root/'user-history-before.json', self.metadata)
        private.write_private(root/'user-history-before.tfvars.json', history)
        catalog = root/'catalog.json'
        catalog.write_text(json.dumps(dummy_catalog()))
        with patch.object(provenance, 'CATALOG', catalog):
            provenance.verify_workflow(env, request=self.github)
        commitment = receipt.commitment(p['receipt_binding'], runtime.bundle(env, root, p))
        env['EXPECTED_RUNTIME_INPUT_COMMITMENT'] = expected or commitment
        return env, catalog

    def begin(self, env):
        runtime.before(env, request=self.aws, metadata_request=self.google)

    def approve(self, env, *, phase='before'):
        plan = copy.deepcopy(self.plan)
        if phase=='post':
            for resource in plan['resource_changes']:
                resource['change'].pop('importing',None)
        root = Path(env['RUNNER_TEMP'])
        name = 'post-apply-summary.json' if phase=='post' else 'sanitized-replan.json'
        if phase=='post':
            private.write_private(root/'user-history-post.json', self.metadata)
            private.write_private(root/'user-history-post.tfvars.json', private.private_json(root/'user-history-before.tfvars.json'))
        args = ['protected', '--operation', env['MODE'], '--phase', phase, '--environment', 'dev',
                '--git-sha', SHA, '--policy', 'import-only', '--json-output', str(root/name),
                '--markdown-output', str(root/(name+'.md'))]
        with patch.dict(os.environ,env,clear=True), patch('sys.argv',args), patch('sys.stdin',io.StringIO(json.dumps(plan))):
            if protected.main() != 0:
                raise ValueError('dummy protected consumer failed')
        summary = private.private_json(root/name)
        if summary['resources'] != []:
            raise AssertionError('runtime summary cannot expose aliases/API inventory')
        if phase=='before':
            private.write_private(root/'tfplan', 'DUMMY_SAVED_PLAN')
            env['APPROVED_SUMMARY_DIGEST'] = hashlib.sha256(private.private_bytes(root/name)).hexdigest()
        return summary

    def seal(self, env):
        runtime.preapply(env, request=self.aws, metadata_request=self.google)
        runtime.check_seal(env)

    def finish(self, env):
        self.aws.body = json.dumps(self.new).encode()
        self.approve(env, phase='post')
        env.update(APPLY_OUTCOME='success', POST_PLAN_OUTCOME='success', POST_PLAN_EXIT_CODE='0')
        runtime.after(env, request=self.aws, metadata_request=self.google)


class RuntimeExecutionTest(unittest.TestCase):
    def setUp(self):
        for target in (preparer,graph):
            p=patch.object(target,'PROJECT',PROJECT);p.start();self.addCleanup(p.stop)

    def run_wave(self, index=0):
        f=WaveFixture(index)
        temporary=tempfile.TemporaryDirectory();self.addCleanup(temporary.cleanup)
        env,catalog=f.stage(temporary.name)
        active=patch.object(provenance,'CATALOG',catalog);active.start();self.addCleanup(active.stop)
        f.begin(env);f.approve(env);f.seal(env)
        return f,env

    def test_all_waves_real_freshness_seal_emitter_and_next_authentic_consumer(self):
        previous=None
        for index in range(6):
            with self.subTest(wave=stages()[index][0]), tempfile.TemporaryDirectory() as directory:
                f=WaveFixture(index,previous=previous)
                env,catalog=f.stage(directory)
                with patch.object(provenance,'CATALOG',catalog):
                    f.begin(env);f.approve(env);f.seal(env);f.finish(env)
                    with patch('sys.stdout',io.StringIO()) as out:
                        receipt.emit_runtime(env | dict(DEV_OWNERSHIP_RECEIPT_ARTIFACT_ENABLED='true'))
                    machine=receipt.decode(out.getvalue()[len(receipt.MARKER):])
                    self.assertEqual((Path(directory)/receipt.PUBLIC_DIRECTORY/receipt.PUBLIC_FILE).read_bytes(),
                                     receipt.canonical(machine)+b'\n')
                    emitted=GitHubFixture(machine,minute=5*(index+1))
                    self.assertEqual(receipt.authenticate(emitted.ref,dummy_catalog(),request=emitted)[0],machine)
                    self.assertEqual(machine['resource_count'],min(13+index,17))
                    self.assertEqual(machine['accounting']['post_state_reads'],5)
                    self.assertEqual(machine['accounting']['post_metadata_reads'],7)
                    self.assertEqual(machine['prior_state_commitment'],f.fakes[-1].value['state_after_commitment'])
                    for secret in ['DUMMY_PRIVATE_TOKEN','dummy private sentinel','222222222222',f.packet['receipt_binding']['nonce'],'pubsub.googleapis.com']:
                        self.assertNotIn(secret,out.getvalue()+(Path(directory)/'summary').read_text())
                    for name in runtime.FILES:
                        if (Path(directory)/name).exists():
                            self.assertEqual((Path(directory)/name).stat().st_mode & 0o777,0o600)
                    if index<5:
                        previous=f.fakes+[GitHubFixture(machine,minute=5*(index+1))]
                self.assertTrue(all(call[:2] in {('s3api','head-object'),('s3api','get-object'),('s3api','list-objects-v2')} for call in f.aws.calls))

    def test_full_state_same_count_identity_trust_api_checks_schema_deposed_denied(self):
        f=WaveFixture(5)
        changes=[lambda s:s.update(serial=True),lambda s:s.update(lineage='bad'),lambda s:s.update(terraform_version='1.0.0'),
            lambda s:s['outputs']['project_id'].update(value='swapped'),lambda s:s['resources'].pop(),
            lambda s:s['resources'][0]['instances'][0].update(deposed='bad'),lambda s:s['resources'][0].update(provider='other'),
            lambda s:s['resources'][-1]['instances'][0]['attributes'].update(service='swap.googleapis.com'),
            lambda s:s['resources'][-2]['instances'][0]['attributes'].update(member='swapped'),
            lambda s:s['resources'][-3]['instances'][0].update(schema_version=1),
            lambda s:s['check_results'][0].update(status='unknown')]
        env=dummy_context()|dict(TF_VAR_export_function_execution_service_account_email=EMAIL)
        for index,mutate in enumerate(changes):
            state=copy.deepcopy(f.old);mutate(state)
            with self.subTest(index=index),self.assertRaises(Exception):
                runtime.state_shape(json.dumps(state),f.packet,f.metadata,env,selection='adopted')

    def test_malformed_pre_containers_reject_even_matching_authenticated_state_commitment(self):
        # An authenticated dummy predecessor commits the malformed body:
        # rejection must be semantic, rather than an incidental HMAC mismatch.
        for job in ('plan', 'apply'):
            for case, state in malformed_containers(WaveFixture(5).old):
                with self.subTest(job=job, case=case), tempfile.TemporaryDirectory() as directory:
                    f = WaveFixture(5)
                    predecessor = copy.deepcopy(f.fakes[-1].value)
                    predecessor['state_after_commitment'] = receipt.state_commitment(f.packet['receipt_binding'], state)
                    f.fakes[-1] = GitHubFixture(predecessor, minute=25)
                    f.packet['adoption_receipts'][-1]['reference'] = f.fakes[-1].ref
                    f.aws.body = json.dumps(state).encode()
                    env, catalog = f.stage(directory, job=job)
                    with patch.object(provenance, 'CATALOG', catalog):
                        self.assertEqual(private.private_json(Path(directory)/provenance.VERIFIED)['last_state_after_commitment'],
                                         receipt.state_commitment(f.packet['receipt_binding'], state))
                        with self.assertRaises(Exception): f.begin(env)
                        with patch('sys.stdout', io.StringIO()) as out, self.assertRaises(Exception): receipt.emit_runtime(env)
                        self.assertEqual(out.getvalue(), '')
                    self.assertFalse((Path(directory)/runtime.BEFORE).exists())
                    self.assertFalse((Path(directory)/runtime.RECORD).exists())

    def test_malformed_post_containers_never_create_or_emit_authentic_receipt(self):
        for case, state in malformed_containers(WaveFixture(3).new):
            with self.subTest(case=case):
                f, env = self.run_wave(3)
                f.new = state
                with self.assertRaises(Exception): f.finish(env)
                root = Path(env['RUNNER_TEMP'])
                ledger = private.private_json(root/'runtime-execution-after.json')
                self.assertEqual(ledger['checks'], dict(state=False, metadata=True, lock=True, workspace=True))
                self.assertFalse((root/runtime.RECORD).exists())
                with patch('sys.stdout', io.StringIO()) as out, self.assertRaises(Exception):
                    receipt.emit_runtime(env | dict(DEV_OWNERSHIP_RECEIPT_ARTIFACT_ENABLED='true'))
                self.assertEqual(out.getvalue(), '')
                self.assertFalse((root/receipt.PUBLIC_DIRECTORY).exists())
                # The real artifact consumer cannot authenticate a failed emitter.
                fake = GitHubFixture(dummy_receipt(run_id=int(env['GITHUB_RUN_ID']), wave='grant', count=16), minute=20)
                fake.install_archive(out.getvalue().encode())
                with self.assertRaises(ValueError): receipt.authenticate(fake.ref, dummy_catalog(), request=fake)

    def test_emitter_revalidates_malformed_post_even_when_private_receipt_hmac_matches(self):
        for case, state in malformed_containers(WaveFixture(3).new):
            with self.subTest(case=case):
                f, env = self.run_wave(3)
                f.finish(env)
                root = Path(env['RUNNER_TEMP'])
                machine = private.private_json(root/runtime.RECORD)
                machine['state_after_commitment'] = receipt.state_commitment(f.packet['receipt_binding'], state)
                # Model a legacy erroneous success record, with a matching body
                # and commitment. It cannot become a new source-pinned marker.
                (root/'runtime-state-after.json').unlink(); (root/runtime.RECORD).unlink()
                private.write_private(root/'runtime-state-after.json', state)
                private.write_private(root/runtime.RECORD, machine)
                with patch('sys.stdout', io.StringIO()) as out, self.assertRaises(Exception):
                    receipt.emit_runtime(env | dict(DEV_OWNERSHIP_RECEIPT_ARTIFACT_ENABLED='true'))
                self.assertEqual(out.getvalue(), '')
                self.assertFalse((root/receipt.PUBLIC_DIRECTORY).exists())
                fake = GitHubFixture(machine, minute=20)
                fake.install_archive(out.getvalue().encode())
                with self.assertRaises(ValueError): receipt.authenticate(fake.ref, dummy_catalog(), request=fake)

    def test_stale_full_state_and_wrong_nonce_prior_receipt_fail_before_plan(self):
        for field,value in [('serial',29),('lineage','bbbbbbbb-bbbb-cccc-dddd-eeeeeeeeeeee')]:
            f=WaveFixture()
            changed=copy.deepcopy(f.old);changed[field]=value;f.aws.body=json.dumps(changed).encode()
            with tempfile.TemporaryDirectory() as directory:
                env,catalog=f.stage(directory)
                with patch.object(provenance,'CATALOG',catalog),self.assertRaises(Exception):f.begin(env)
                self.assertFalse((Path(directory)/runtime.BEFORE).exists())

    def test_plan_to_apply_private_binding_rejects_changes_before_state_read(self):
        f=WaveFixture()
        with tempfile.TemporaryDirectory() as directory:
            env,catalog=f.stage(directory,job='plan',mode='apply')
            with patch.object(provenance,'CATALOG',catalog):f.begin(env)
            commit=(Path(directory)/'output').read_text().split('=')[1].strip()
        for changed in [dict(EXPECTED_RUNTIME_INPUT_COMMITMENT='b'*64),dict(TF_VAR_primary_email_channel_name='different')]:
            f=WaveFixture()
            with tempfile.TemporaryDirectory() as directory:
                env,catalog=f.stage(directory,expected=commit);env.update(changed)
                with patch.object(provenance,'CATALOG',catalog),self.assertRaises(Exception):f.begin(env)
                self.assertEqual(f.aws.calls,[])

    def test_saved_binary_summary_input_and_state_races_stop_preapply(self):
        mutations=[lambda f,e:(Path(e['RUNNER_TEMP'])/'tfplan').write_text('swapped'),
            lambda f,e:(Path(e['RUNNER_TEMP'])/'sanitized-replan.json').write_text('{}'),
            lambda f,e:e.update(TF_VAR_primary_email_address='different'),
            lambda f,e:f.aws.__setattr__('version','raced')]
        for index,mutation in enumerate(mutations):
            f,env=self.run_wave();mutation(f,env)
            with self.subTest(index=index),self.assertRaises(Exception):
                if index==3:
                    (Path(env['RUNNER_TEMP'])/'runtime-state-preapply.json').unlink()
                    (Path(env['RUNNER_TEMP'])/runtime.SEALED).unlink()
                    f.seal(env)
                else:runtime.check_seal(env)

    def test_post_failures_preserve_independent_safety_reads_and_emit_no_receipt(self):
        mutations=[lambda f,e:f.aws.__setattr__('denied','get-object'),lambda f,e:f.aws.__setattr__('lock',True),
            lambda f,e:f.aws.__setattr__('workspaces',True),lambda f,e:e.update(APPLY_OUTCOME='failure'),
            lambda f,e:e.update(POST_PLAN_OUTCOME='failure'),lambda f,e:f.new.update(serial=27),
            lambda f,e:f.new['resources'][0]['instances'][0]['attributes'].update(id='same-count-swap'),
            lambda f,e:f.new['outputs']['environment'].update(value='changed')]
        for index,mutate in enumerate(mutations):
            f,env=self.run_wave();f.approve(env,phase='post');env.update(APPLY_OUTCOME='success',POST_PLAN_OUTCOME='success',POST_PLAN_EXIT_CODE='0')
            mutate(f,env);f.aws.body=json.dumps(f.new).encode();f.metadata_calls=[]
            with self.subTest(index=index),self.assertRaises(Exception):runtime.after(env,request=f.aws,metadata_request=f.google)
            self.assertTrue(any(host=='bigquery.googleapis.com' for host,path in f.metadata_calls))
            self.assertFalse((Path(env['RUNNER_TEMP'])/runtime.RECORD).exists())
            self.assertTrue((Path(env['RUNNER_TEMP'])/'runtime-execution-after.json').exists())

    def test_unrelated_iam_binding_wif_trust_all_enabled_apis_and_table_swap_rejected(self):
        mutations=[lambda h,p,v:v['bindings'].append(dict(role='roles/viewer',members=['dummy-swapped'])) if ':getIamPolicy' in p else None,
            lambda h,p,v:v.update(attributeCondition='changed') if '/providers/' in p else None,
            lambda h,p,v:v['services'][0]['config'].update(name='swapped.googleapis.com') if h=='serviceusage.googleapis.com' else None,
            lambda h,p,v:v['schema']['fields'].reverse() if h=='bigquery.googleapis.com' else None,
            lambda h,p,v:v.update(attestationRules=[{}]) if ':listAttestationRules' in p else None]
        for index,mutate in enumerate(mutations):
            f,env=self.run_wave();f.metadata_mutation=mutate
            with self.subTest(index=index),self.assertRaises(Exception):f.finish(env)
            self.assertFalse((Path(env['RUNNER_TEMP'])/runtime.RECORD).exists())

    def test_empty_catalog_closed_gates_mixed_selectors_no_public_values(self):
        f,env=self.run_wave()
        for mutation in [dict(DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED='false'),dict(DEV_HISTORY_POST_NOOP12_READY='false'),
                         dict(DEV_HISTORY_RECEIPT_EMITTER_ENABLED='false'),dict(DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED='false'),
                         dict(HISTORY_POST_NOOP='true'),dict(GITHUB_JOB='other'),dict(MODE='security-probe')]:
            with patch.dict(os.environ,env|mutation,clear=True),patch('sys.argv',['runtime','--phase','check-seal']),patch('sys.stdout',io.StringIO()) as out,patch('sys.stderr',io.StringIO()) as err:
                self.assertEqual(runtime.main(),3)
                self.assertNotIn('DUMMY_PRIVATE_TOKEN',out.getvalue()+err.getvalue())

    def test_plan_after_requires_no_state_version_write_or_metadata_change(self):
        for changed in (False,True):
            f=WaveFixture()
            with tempfile.TemporaryDirectory() as directory:
                env,catalog=f.stage(directory,job='plan')
                with patch.object(provenance,'CATALOG',catalog):
                    f.begin(env);f.approve(env)
                    env.update(BACKEND_INIT_OUTCOME='success',TERRAFORM_PLAN_OUTCOME='success',PLAN_VALIDATION_OUTCOME='success')
                    if changed:f.aws.version='DUMMY_NEW_VERSION'
                    if changed:
                        with self.assertRaises(Exception):runtime.after(env,request=f.aws,metadata_request=f.google)
                    else:runtime.after(env,request=f.aws,metadata_request=f.google)
                    self.assertFalse((Path(directory)/runtime.RECORD).exists())

    def test_plan_and_apply_jobs_share_exact_packet_commitment(self):
        f=WaveFixture()
        with tempfile.TemporaryDirectory() as plan_dir, tempfile.TemporaryDirectory() as apply_dir:
            planned,catalog=f.stage(plan_dir,job='plan',mode='apply')
            with patch.object(provenance,'CATALOG',catalog):
                f.begin(planned);f.approve(planned)
                planned.update(BACKEND_INIT_OUTCOME='success',TERRAFORM_PLAN_OUTCOME='success',PLAN_VALIDATION_OUTCOME='success')
                runtime.after(planned,request=f.aws,metadata_request=f.google)
            commitment=(Path(plan_dir)/'output').read_text().split('=')[1].strip()
            applied,catalog=f.stage(apply_dir,expected=commitment)
            with patch.object(provenance,'CATALOG',catalog):
                f.begin(applied);f.approve(applied);f.seal(applied);f.finish(applied)
            self.assertEqual(private.private_json(Path(apply_dir)/runtime.BEFORE)['input_commitment'],commitment)

    def test_runtime_identity_never_uses_baseline11_reader_or_negative_probe(self):
        import terraform_identity_smoke as identity
        for job in ('plan','apply'):
            f=WaveFixture()
            with tempfile.TemporaryDirectory() as directory:
                env,catalog=f.stage(directory,job=job)
                with patch.object(provenance,'CATALOG',catalog), patch.object(identity,'verify_aws_identity') as sts, \
                     patch.object(identity,'aws',side_effect=AssertionError('no extra AWS calls')), \
                     patch.object(private,'snapshot_before',side_effect=AssertionError('baseline11 forbidden')):
                    labels=identity.verify_aws(env,plan_read_only=job=='plan',apply_read_only=job=='apply')
                sts.assert_called_once()
                self.assertTrue(any('Runtime state verification' in label for label in labels))

    def test_wrong_nonce_reference_skip_batch_and_copied_success_cannot_enter_runtime(self):
        changes=[lambda p:p['receipt_binding'].update(nonce='cd'*32),lambda p:p['history_receipt'].update(receipt_sha256='e'*64),
                 lambda p:p['selected'].update(provider=True),lambda p:p.update(adopted=p['selected'])]
        for index,change in enumerate(changes):
            f=WaveFixture();change(f.packet)
            with self.subTest(index=index),tempfile.TemporaryDirectory() as directory,self.assertRaises(Exception):
                f.stage(directory)

    def test_runtime_authentic_consumer_requires_critical_steps_and_cleanup(self):
        f,env=self.run_wave();f.finish(env)
        with patch('sys.stdout',io.StringIO()) as out:receipt.emit_runtime(env)
        machine=receipt.decode(out.getvalue()[len(receipt.MARKER):])
        for name in receipt.RUNTIME_STEPS['apply']+['Cleanup sensitive temporary files']:
            fake=GitHubFixture(machine,minute=5)
            for step in fake.jobs[1]['steps']:
                if step['name']==name:step['conclusion']='skipped'
            with self.subTest(name=name),self.assertRaises(ValueError):receipt.authenticate(fake.ref,dummy_catalog(),request=fake)

    def test_post_network_metadata_failure_still_attempts_table_lock_and_workspace(self):
        f,env=self.run_wave();f.approve(env,phase='post');f.aws.denied='get-object'
        calls=[]
        def broken(path,token,*,host):
            calls.append((host,path))
            raise RuntimeError('DUMMY_PRIVATE_ERROR')
        with self.assertRaises(Exception):runtime.after(env,request=f.aws,metadata_request=broken)
        self.assertTrue(any(host=='bigquery.googleapis.com' for host,path in calls))
        self.assertTrue(any(host=='iam.googleapis.com' and ':getIamPolicy' in path for host,path in calls))
        evidence=private.private_json(Path(env['RUNNER_TEMP'])/'runtime-execution-after.json')
        self.assertTrue(evidence['checks']['lock']);self.assertTrue(evidence['checks']['workspace'])
        self.assertFalse((Path(env['RUNNER_TEMP'])/runtime.RECORD).exists())

    def test_private_files_symlink_modes_exclusive_and_cleanup_failure(self):
        for kind in ('symlink','public'):
            f,env=self.run_wave();target=Path(env['RUNNER_TEMP'])/'tfplan'
            if kind=='symlink':
                value=target.read_bytes();target.unlink();other=target.parent/'other';other.write_bytes(value);other.chmod(0o600);target.symlink_to(other)
            else:target.chmod(0o644)
            with self.subTest(kind=kind),self.assertRaises(Exception):runtime.check_seal(env)
        f,env=self.run_wave()
        with self.assertRaises(Exception):f.begin(env)
        workflow=(ROOT/'.github/workflows/gcp-terraform-authenticated.yml').read_text()
        for job,end in [('plan','apply'),('apply','security-probe')]:
            section=workflow.split('  '+job+':\n')[1].split('  '+end+':\n')[0]
            cleanup=section.split('      - name: Cleanup sensitive temporary files')[1]
            self.assertIn('if: always()',cleanup)
            for name in runtime.FILES:self.assertIn('"${RUNNER_TEMP}/'+name+'"',cleanup)
            # Run the actual rm list in a dummy isolated directory, never a cloud runner.
            import subprocess,textwrap
            script=textwrap.dedent(cleanup.split('        run: |\n')[1])
            with tempfile.TemporaryDirectory() as directory:
                for name in runtime.FILES:(Path(directory)/name).write_text('DUMMY_PRIVATE')
                run=subprocess.run(['bash','-c',script],env={'PATH':os.environ['PATH'],'RUNNER_TEMP':directory,'GITHUB_WORKSPACE':directory},capture_output=True,text=True)
                self.assertEqual(run.returncode,0)
                self.assertTrue(all(not (Path(directory)/name).exists() for name in runtime.FILES))

    def test_packet_expiry_after_apply_retains_bound_safety_reads_but_no_success(self):
        f,env=self.run_wave();f.approve(env,phase='post');f.aws.body=json.dumps(f.new).encode()
        expires=receipt.timestamp(f.packet['expires_utc'])
        class LateTime(datetime):
            @classmethod
            def now(cls,tz=None):return expires+timedelta(seconds=1)
        with patch.object(packet_module,'datetime',LateTime),self.assertRaises(Exception):
            runtime.after(env,request=f.aws,metadata_request=f.google)
        evidence=private.private_json(Path(env['RUNNER_TEMP'])/'runtime-execution-after.json')
        self.assertTrue(evidence['checks']['metadata']);self.assertTrue(evidence['checks']['lock'])
        self.assertGreater(evidence['observed_reads']['state'],0)
        self.assertFalse((Path(env['RUNNER_TEMP'])/runtime.RECORD).exists())

    def test_complete_api_pagination_and_loop_or_truncation_rejection(self):
        f,env=self.run_wave()
        services=f.packet['inventory']['enabled_services']['services']
        def paged(path,token,*,host):
            if host=='serviceusage.googleapis.com':
                if 'pageToken=' in path:return 200,{'services':services[1:]}
                return 200,{'services':services[:1],'nextPageToken':'dummy page&token'}
            return f.google(path,token,host=host)
        observed=runtime.metadata_snapshot(env,f.packet,request=paged)
        self.assertEqual(len(observed['inventory']['enabled_services']['services']),len(services))
        def loop(path,token,*,host):
            if host=='serviceusage.googleapis.com':return 200,{'services':services[:1],'nextPageToken':'same'}
            return f.google(path,token,host=host)
        with self.assertRaises(Exception):runtime.metadata_snapshot(env,f.packet,request=loop)


if __name__=='__main__':unittest.main()
