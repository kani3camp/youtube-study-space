#!/usr/bin/env python3
"""Public dummy-only cumulative graph, private handoff and workflow contracts."""
from __future__ import annotations

import copy
from datetime import datetime, timezone
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import textwrap
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / '.github/scripts'))
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
sys.path.insert(0, str(Path(__file__).parent))
import prepare_runtime_ownership as preparer
import runtime_ownership_packet as packet_module
import validate_runtime_ownership_plan as gate
import terraform_protected_plan as protected
from test_runtime_ownership import inventory
import test_gcp_user_activity_schema_audit_workflow as history_tests
from test_terraform_export_function_gate import EMAIL

SHA = 'a' * 40
NOW = datetime(2026, 10, 10, 4, 0, tzinfo=timezone.utc)
PROJECT = 'example-study-space-dev'
GOOGLE = 'registry.terraform.io/hashicorp/google'


def selection(pool=False, provider=False, grants=(), apis=()):
    return dict(pool=pool, provider=provider, grants=list(grants), apis=list(apis))


def stages():
    states = [selection(), selection(True), selection(True, True),
              selection(True, True, ['grant-01']), selection(True, True, ['grant-01', 'grant-02']),
              selection(True, True, ['grant-01', 'grant-02'], ['pubsub.googleapis.com'])]
    return [(wave, old, new) for wave, old, new in zip(
        ['pool', 'provider', 'grant', 'grant', 'api', 'noop'], states, states[1:] + [states[-1]])]


def packet(index=0):
    wave, old, new = stages()[index]
    return dict(schema_version=1, git_sha=SHA, wave=wave, reviewed_utc='2026-10-10T03:30:00Z',
                expires_utc='2026-10-10T05:30:00Z', history_post_noop_run_id=123,
                inventory=inventory(), adopted=old, selected=new)


def add_resource(plan, address, resource_type, values, import_id=None):
    change = dict(actions=['no-op'], before=copy.deepcopy(values), after=copy.deepcopy(values),
                  after_unknown={}, before_sensitive={}, after_sensitive={})
    if import_id is not None:
        change['importing'] = {'id': import_id}
    plan['resource_changes'].append(dict(address=address, mode='managed', type=resource_type,
                                         provider_name=GOOGLE, change=change))


def planned_values(plan):
    plan['planned_values'] = {'root_module': {'resources': [
        dict(address=r['address'], mode=r['mode'], type=r['type'], provider_name=GOOGLE,
             schema_version=4 if r['type'] == 'google_storage_bucket' else 0,
             values=copy.deepcopy(r['change']['after'])) for r in plan['resource_changes']]}}


def fixture(index=0, phase='before'):
    history = history_tests.UserActivityHistoryPlanTest()
    history.setUp()
    plan, metadata = history.fixture(False)
    plan.update(format_version='1.2', terraform_version='1.16.4', complete=True, errored=False)
    for r in plan['resource_changes']:
        r['provider_name'] = GOOGLE
        r['change'].setdefault('before', {})
        r['change'].setdefault('after', {})
        r.setdefault('type', re.search(r'(google_[a-z_]+)\.', r['address']).group(1))
    p = packet(index)
    old, new = p['adopted'], p['selected']
    inv = p['inventory']
    pool_id = f'projects/{PROJECT}/locations/global/workloadIdentityPools/aws-runtime'
    pool_name = inv['pool']['name']
    if new['pool']:
        values = dict(id=pool_id, name=pool_name, project=PROJECT, workload_identity_pool_id='aws-runtime',
                      state='ACTIVE', display_name='dummy pool', description='dummy private sentinel',
                      disabled=False, mode='FEDERATION_ONLY', deletion_policy='DELETE',
                      attestation_rules=[], inline_certificate_issuance_config=[], inline_trust_config=[])
        add_resource(plan, gate.POOL, 'google_iam_workload_identity_pool', values,
                     pool_id if phase == 'before' and not old['pool'] else None)
    if new['provider']:
        values = dict(id=pool_id + '/providers/aws-provider', name=pool_name + '/providers/aws-provider',
                      project=PROJECT, workload_identity_pool_id='aws-runtime', workload_identity_pool_provider_id='aws-provider',
                      state='ACTIVE', display_name='', description='', disabled=False, deletion_policy='DELETE',
                      attribute_mapping=inv['provider']['attributeMapping'], attribute_condition=inv['provider']['attributeCondition'],
                      aws=[{'account_id': '222222222222'}], oidc=[], saml=[], x509=[])
        add_resource(plan, gate.PROVIDER, 'google_iam_workload_identity_pool_provider', values,
                     pool_id + '/providers/aws-provider' if phase == 'before' and not old['provider'] else None)
    for alias in new['grants']:
        member = inv['runtime_grants'][alias]['member']
        condition = inv['iam_policy']['bindings'][1]['condition'] if alias == 'grant-02' else None
        sa = inv['iam_policy_resource']
        state_id = sa + '/roles/iam.workloadIdentityUser/' + member
        if condition:
            state_id += '/dummy-condition/dummy private sentinel/' + condition['expression']
        values = dict(id=state_id, service_account_id=sa, role='roles/iam.workloadIdentityUser',
                      member=member, condition=[condition] if condition else [], etag='dummy-etag')
        import_id = sa + ' roles/iam.workloadIdentityUser ' + member + (' dummy-condition' if condition else '')
        add_resource(plan, f'module.runtime_wif.google_service_account_iam_member.runtime["{alias}"]',
                     'google_service_account_iam_member', values,
                     import_id if phase == 'before' and alias not in old['grants'] else None)
    for service in new['apis']:
        resource_id = PROJECT + '/' + service
        values = dict(id=resource_id, project=PROJECT, service=service, deletion_policy='DELETE',
                      disable_on_destroy=False, disable_dependent_services=False)
        add_resource(plan, f'module.owned_apis.google_project_service.owned["{service}"]', 'google_project_service', values,
                     resource_id if phase == 'before' and service not in old['apis'] else None)
    planned_values(plan)
    return plan, metadata, p


class PackageContracts(unittest.TestCase):
    def setUp(self):
        self.patches = [patch.object(preparer, 'PROJECT', PROJECT), patch.object(gate, 'PROJECT', PROJECT)]
        for p in self.patches:
            p.start()
            self.addCleanup(p.stop)

    def review(self, plan, metadata, p, phase='before'):
        return gate.validate(plan, packet=p, wave=p['wave'], git_sha=SHA, metadata=metadata,
                             phase=phase, execution_email=EMAIL, now=NOW)

    def test_each_wave_is_cumulative_one_import_then_full_post_noop(self):
        for index, count in enumerate([13, 14, 15, 16, 17, 17]):
            for phase in ['before', 'post']:
                with self.subTest(index=index, phase=phase):
                    plan, metadata, p = fixture(index, phase)
                    result = self.review(plan, metadata, p, phase)
                    self.assertEqual(result['counts']['no-op'], count)
                    self.assertEqual(result['counts']['import'], int(phase == 'before' and index < 5))
                    self.assertNotIn('dummy private sentinel', json.dumps(result))
                    self.assertNotIn('222222222222', json.dumps(result))

    def test_selection_rejects_skips_shrinks_batches_unknown_or_unreviewed_apis(self):
        changes = [
            (0, lambda p: p['selected'].update(provider=True)),
            (1, lambda p: p['adopted'].update(pool=False)),
            (2, lambda p: p['selected'].update(grants=['grant-01', 'grant-02'])),
            (3, lambda p: p['selected'].update(grants=['grant-02'])),
            (4, lambda p: p['adopted'].update(grants=['grant-01'])),
            (4, lambda p: p['selected'].update(apis=['firebase.googleapis.com'])),
            (4, lambda p: p['selected'].update(apis=['invented.googleapis.com'])),
            (4, lambda p: p['selected'].update(apis=['pubsub.googleapis.com', 'pubsub.googleapis.com'])),
            (0, lambda p: p.update(history_post_noop_run_id=0)),
            (0, lambda p: p.update(history_post_noop_run_id=True)),
            (0, lambda p: p.update(git_sha='b' * 40)),
            (0, lambda p: p.update(expires_utc='2026-10-10T03:59:59Z')),
            (0, lambda p: p.update(expires_utc='2026-10-12T03:30:00Z')),
            (0, lambda p: p.update(reviewed_utc='2026-10-10T04:30:00Z')),
            (0, lambda p: p.update(extra='dummy private sentinel')),
        ]
        for index, mutation in changes:
            p = packet(index)
            mutation(p)
            with self.subTest(index=index, mutation=changes.index((index, mutation))), self.assertRaises((ValueError, KeyError)):
                packet_module.validate_packet(p, wave=p['wave'], git_sha=SHA, now=NOW)

    def test_inventory73_candidates10_never_selects_actual_scope(self):
        p = packet(4)
        for index in range(71):
            service = f'dummy{index:03d}.googleapis.com'
            p['inventory']['enabled_services']['services'].append({
                'name': 'projects/111111111111/services/' + service,
                'config': {'name': service}, 'state': 'ENABLED'})
            p['inventory']['api_classification'][service] = {
                'classification': 'Own' if index < 9 else 'Investigate',
                'reason': 'Dummy review only',
                'dependency_addresses': ['module.export_topic[0].google_pubsub_topic.export'] if index < 9 else []}
        candidate = packet_module.validate_packet(p, wave='api', git_sha=SHA, now=NOW)
        self.assertEqual(len(p['inventory']['enabled_services']['services']), 73)
        self.assertEqual(sum(v['classification'] == 'Own' for v in candidate['api_classification'].values()), 10)
        self.assertEqual(candidate['owned_api_keys'], ['pubsub.googleapis.com'])
        self.assertEqual(preparer.prepare(p['inventory'])['owned_api_keys'], [])

    def test_every_graph_action_identity_schema_and_drift_failure_stops(self):
        changes = [
            lambda p: p.update(resource_drift=[{}]), lambda p: p.update(resource_drift={}),
            lambda p: p.update(action_invocations=[{}]), lambda p: p.update(deferred_action_invocations=[{}]),
            lambda p: p.update(deferred_changes=[{}]), lambda p: p.update(complete=False),
            lambda p: p.update(errored=True), lambda p: p.update(format_version='2.0'),
            lambda p: p.update(terraform_version='0.0.0'), lambda p: p.update(checks=[{'status': 'unknown'}]),
            lambda p: p['resource_changes'].append(copy.deepcopy(p['resource_changes'][-1])),
            lambda p: p['resource_changes'].pop(0),
            lambda p: p['resource_changes'][-1].update(previous_address='dummy'),
            lambda p: p['resource_changes'][-1].update(deposed='dummy'),
            lambda p: p['resource_changes'][-1].update(mode='data'),
            lambda p: p['resource_changes'][-1].update(type='google_project_iam_policy'),
            lambda p: p['resource_changes'][-1].update(provider_name='registry.terraform.io/hashicorp/google-beta'),
            lambda p: p['resource_changes'][-1]['change'].update(after_unknown={'id': True}),
            lambda p: p['resource_changes'][-1]['change'].update(importing={'id': 'dummy wrong import'}),
            lambda p: p['resource_changes'][-1]['change'].update(importing={'id': PROJECT + '/pubsub.googleapis.com', 'unknown': False}),
            lambda p: p['resource_changes'][-1]['change'].update(after_sensitive={'id': True}),
            lambda p: p['planned_values']['root_module']['resources'][-1].update(schema_version=1),
            lambda p: p['planned_values']['root_module']['resources'][-1].update(schema_version=False),
            lambda p: p['planned_values']['root_module']['resources'][-1]['values'].update(service='wrong.googleapis.com'),
            lambda p: p['resource_changes'][0]['change'].update(importing={'id': 'dummy unexpected import'}),
            lambda p: p['resource_changes'][0]['change'].update(importing=None),
            lambda p: p['resource_changes'][0].update(type='google_project_iam_policy'),
            lambda p: p['resource_changes'][0]['change'].update(after=None),
            lambda p: p.update(output_changes={'dummy': {'actions': ['update']}}),
            lambda p: p.update(output_changes={'dummy': {'actions': ['no-op'], 'before': 'safe', 'after': 'changed'}}),
        ]
        for action in [['create'], ['read'], ['update'], ['delete'], ['delete', 'create'], ['no-op', 'read'], []]:
            changes.append(lambda p, action=action: p['resource_changes'][-1]['change'].update(actions=action))
        for mutation in changes:
            plan, metadata, p = fixture(4)
            mutation(plan)
            with self.subTest(mutation=changes.index(mutation)), self.assertRaises((ValueError, KeyError)):
                self.review(plan, metadata, p)

    def test_noop_values_cannot_hide_identity_trust_condition_or_disable_changes(self):
        for index, resource_index, field, value in [
            (0, 12, 'attestation_rules', [{'google_cloud_resource': 'dummy'}]),
            (0, 12, 'name', 'projects/999/locations/global/workloadIdentityPools/aws-runtime'),
            (0, 12, 'mode', 'TRUST_DOMAIN'), (0, 12, 'state', 'DELETED'),
            (1, 13, 'attribute_condition', 'true'), (1, 13, 'aws', [{'account_id': '999999999999'}]),
            (1, 13, 'oidc', [{'issuer_uri': 'https://example.invalid'}]),
            (2, 14, 'member', 'allUsers'), (2, 14, 'role', 'roles/editor'),
            (3, 15, 'condition', []), (3, 15, 'id', 'dummy wrong canonical id'),
            (4, 16, 'disable_on_destroy', True), (4, 16, 'disable_dependent_services', True),
            (4, 16, 'deletion_policy', 'ABANDON'), (4, 16, 'unknown_schema_field', None),
        ]:
            plan, metadata, p = fixture(index)
            for side in ['before', 'after']:
                plan['resource_changes'][resource_index]['change'][side][field] = value
            planned_values(plan)
            with self.subTest(index=index, field=field), self.assertRaises(ValueError):
                self.review(plan, metadata, p)

    def test_history_import_or_schema_change_and_unowned_dependency_are_rejected(self):
        for mutation in [
            lambda plan, p: plan['resource_changes'][11]['change'].update(importing={'id': gate.validate_history.__globals__['TABLE_ID']}),
            lambda plan, p: plan['resource_changes'][11]['change']['after'].update(schema='[]'),
            lambda plan, p: p['inventory']['api_classification']['pubsub.googleapis.com'].update(dependency_addresses=['module.unowned']),
        ]:
            plan, metadata, p = fixture(4)
            mutation(plan, p)
            with self.assertRaises(ValueError):
                self.review(plan, metadata, p)

    def test_post_rejects_import_marker_and_missing_selected_resource(self):
        plan, metadata, p = fixture(3)
        with self.assertRaises(ValueError):
            self.review(plan, metadata, p, 'post')
        plan, metadata, p = fixture(3, 'post')
        plan['resource_changes'].pop()
        planned_values(plan)
        with self.assertRaises(ValueError):
            self.review(plan, metadata, p, 'post')

    def test_private_handoff_is_exclusive_owner_only_and_never_emits_values(self):
        with tempfile.TemporaryDirectory() as directory:
            env = dict(MODE='plan', TF_VAR_project_id='test-youtube-study-space',
                       TF_VAR_manage_user_activity_history='true', OWNERSHIP_WAVE='pool',
                       GITHUB_SHA=SHA, RUNNER_TEMP=directory, RUNTIME_OWNERSHIP_PACKET_JSON=json.dumps(staging_packet()))
            with patch.object(packet_module, 'validate_packet', side_effect=lambda p, **kw: packet_module_validate(p, now=NOW, **kw)):
                packet_module.prepare_workflow(env)
                for name in ['runtime-ownership-packet.json', 'runtime-ownership.tfvars.json']:
                    self.assertEqual((Path(directory) / name).stat().st_mode & 0o777, 0o600)
                with self.assertRaises(OSError):
                    packet_module.prepare_workflow(env)
            env['RUNTIME_OWNERSHIP_PACKET_JSON'] = '{"private sentinel": 1, "private sentinel": 2}'
            with patch.dict(os.environ, env, clear=True), patch('sys.stdout', io.StringIO()) as out, patch('sys.stderr', io.StringIO()) as err:
                self.assertEqual(packet_module.main(), 3)
                self.assertNotIn('private sentinel', out.getvalue() + err.getvalue())

    def test_protected_consumer_checks_private_var_file_and_sanitizes_all_values(self):
        plan, metadata, p = fixture(4)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            candidate = packet_module.validate_packet(p, wave='api', git_sha=SHA, now=NOW)
            inputs = protected.prepare(metadata)
            inputs.pop('manage_user_activity_history')
            for name, value in [('user-history-before.json', metadata), ('user-history-before.tfvars.json', inputs),
                                ('runtime-ownership-packet.json', p), ('runtime-ownership.tfvars.json', candidate)]:
                (root / name).write_text(json.dumps(value))
                (root / name).chmod(0o600)
            args = ['protected', '--operation', 'plan', '--phase', 'before', '--environment', 'dev',
                    '--git-sha', SHA, '--policy', 'import-only', '--json-output', str(root / 'summary.json'),
                    '--markdown-output', str(root / 'summary.md')]
            env = dict(TF_VAR_manage_user_activity_history='true', OWNERSHIP_WAVE='api', RUNNER_TEMP=directory,
                       TF_VAR_export_function_execution_service_account_email=EMAIL,
                       **{'TF_VAR_manage_export_' + kind: 'true' for kind in ['function', 'scheduler', 'topic']})
            with patch.dict(os.environ, env, clear=True), patch('sys.argv', args), patch('sys.stdin', io.StringIO(json.dumps(plan))), \
                 patch.object(protected, 'require_verified_inputs'), \
                 patch('terraform_runtime_execution.require_fresh_inputs'), patch('terraform_runtime_execution.approve_plan'), \
                 patch.object(protected, 'validate_packet', side_effect=lambda p, **kw: packet_module_validate(p, now=NOW, **kw)), \
                 patch.object(protected, 'validate_runtime', side_effect=lambda p, **kw: gate_validate(p, now=NOW, **kw)):
                self.assertEqual(protected.main(), 0)
            public = (root / 'summary.json').read_text() + (root / 'summary.md').read_text()
            for sentinel in ['dummy private sentinel', '222222222222', 'DummyLambda', 'dummy-etag']:
                self.assertNotIn(sentinel, public)
            candidate['owned_api_keys'] = ['firebase.googleapis.com']
            (root / 'runtime-ownership.tfvars.json').write_text(json.dumps(candidate))
            with patch.dict(os.environ, env, clear=True), patch('sys.argv', args), patch('sys.stdin', io.StringIO(json.dumps(plan))), \
                 patch.object(protected, 'require_verified_inputs'), \
                 patch('terraform_runtime_execution.require_fresh_inputs'), patch('terraform_runtime_execution.approve_plan'), \
                 patch.object(protected, 'validate_packet', side_effect=lambda p, **kw: packet_module_validate(p, now=NOW, **kw)), patch('sys.stderr', io.StringIO()):
                self.assertEqual(protected.main(), 3)


def staging_packet():
    from test_terraform_ownership_receipt import dummy_binding, GitHubFixture
    value = packet()
    value.pop('history_post_noop_run_id')
    value.update(schema_version=2, receipt_binding=dummy_binding(), history_receipt=GitHubFixture().ref, adoption_receipts=[])
    return value


packet_module_validate = packet_module.validate_packet
gate_validate = gate.validate


class WorkflowContracts(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.text = (ROOT / '.github/workflows/gcp-terraform-authenticated.yml').read_text()
        cls.caller = (ROOT / '.github/workflows/ci.yml').read_text()

    def preflight(self, **overrides):
        script = self.text.split('      - name: Enforce trusted execution surface', 1)[1].split('        run: |\n', 1)[1].split('\n  plan:\n', 1)[0]
        env = dict(PATH=os.environ['PATH'], GITHUB_EVENT_NAME='workflow_dispatch', GITHUB_REPOSITORY='kani3camp/youtube-study-space',
                   GITHUB_REPOSITORY_ID='340900071', GITHUB_REPOSITORY_OWNER_ID='54093651',
                   GITHUB_REF='refs/heads/feature/gcp-terraform-iac',
                   GITHUB_WORKFLOW_REF='kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac',
                   GITHUB_SHA=SHA, TARGET='dev', MODE='plan', OWNERSHIP_WAVE='pool',
                   HISTORY_POST_NOOP='false')
        env.update(re.findall(r'^  ([A-Z_]+): "([^"]*)"$', self.text, re.M))
        env.update(overrides)
        with tempfile.TemporaryDirectory() as directory:
            env['GITHUB_OUTPUT'] = str(Path(directory) / 'output')
            return subprocess.run(['bash', '-c', textwrap.dedent(script)], env=env, capture_output=True, text=True)

    def test_new_live_gates_closed_before_any_auth_all_selections(self):
        for wave in ['pool', 'provider', 'grant', 'api', 'noop']:
            with self.subTest(wave=wave):
                self.assertNotEqual(self.preflight(OWNERSHIP_WAVE=wave).returncode, 0)
                self.assertNotEqual(self.preflight(OWNERSHIP_WAVE=wave, DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED='true').returncode, 0)
        self.assertEqual(self.preflight(OWNERSHIP_WAVE='none').returncode, 0)

    def test_test_only_activation_still_rejects_apply_prod_probes_exceptions_and_missing_history(self):
        activated = dict(DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED='true', DEV_HISTORY_POST_NOOP12_READY='true')
        self.assertEqual(self.preflight(**activated).returncode, 0)
        apply_gates=activated | dict(DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED='true',DEV_HISTORY_RECEIPT_EMITTER_ENABLED='true')
        self.assertEqual(self.preflight(**apply_gates,MODE='apply').returncode,0)
        self.assertNotEqual(self.preflight(**apply_gates,MODE='apply',HISTORY_POST_NOOP='true',DEV_HISTORY_POST_NOOP_ENABLED='true').returncode,0)
        for overrides in [dict(MODE=mode) for mode in ['apply', 'security-probe', 'email-adoption', 'quota-create', 'quota-plan', 'quota-refresh']] + [
            dict(TARGET='prod'), dict(DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED='false'),
            dict(OWNERSHIP_WAVE='wrong'), dict(GITHUB_EVENT_NAME='pull_request')]:
            self.assertNotEqual(self.preflight(**dict(activated, **overrides)).returncode, 0)

    def test_private_input_and_full_root_plan_wiring_cleanup_and_ci_routing(self):
        plan = self.text.split('  plan:\n', 1)[1].split('  apply:\n', 1)[0]
        self.assertLess(plan.index('Stage reviewed private cumulative'), plan.index('Configure AWS backend credential'))
        self.assertIn('RUNTIME_OWNERSHIP_PACKET_JSON: ${{ secrets.GCP_RUNTIME_OWNERSHIP_PACKET_JSON }}', plan)
        self.assertIn('${ownership_args[@]+"${ownership_args[@]}"}', plan)
        self.assertIn('ownership_wave: ${{ inputs.terraform_ownership_wave }}', self.caller)
        self.assertIn('test_runtime_ownership_package.py', self.caller)
        self.assertNotIn('-target=', plan)
        self.assertNotIn('upload-artifact', self.text)
        for name in ['runtime-ownership-packet.json', 'runtime-ownership.tfvars.json']:
            self.assertIn('"${RUNNER_TEMP}/' + name + '"', plan.split('Cleanup sensitive temporary files')[1])
        for name in ['DEV_HISTORY_POST_NOOP12_READY', 'DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED']:
            self.assertIn(name + ': "false"', self.text)
        self.assertNotIn('plan_cost_evidence', plan.split('Stage reviewed private cumulative', 1)[1].split('Configure AWS backend credential')[0])

    def test_plan_passes_private_cumulative_inputs_only_for_selected_wave(self):
        step = self.text.split('      - name: Create saved plan without public output', 1)[1].split('      - name:', 1)[0]
        script = textwrap.dedent(step.split('        run: |\n', 1)[1])
        for wave in ['none', 'pool', 'provider', 'grant', 'api', 'noop']:
            with self.subTest(wave=wave), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                fake = root / 'terraform'
                fake.write_text('#!/bin/bash\nprintf "%s\\n" "$@" > "$ARGUMENT_RECORD"\nexit 0\n')
                fake.chmod(0o700)
                env = dict(PATH=directory + ':' + os.environ['PATH'], RUNNER_TEMP=directory,
                           TF_ROOT='dummy-root', MODE='plan', TF_VAR_manage_user_activity_history='true',
                           OWNERSHIP_WAVE=wave, HISTORY_POST_NOOP='false',
                           GITHUB_OUTPUT=str(root / 'output'), ARGUMENT_RECORD=str(root / 'arguments'))
                result = subprocess.run(['bash', '-c', script], env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                arguments = (root / 'arguments').read_text().splitlines()
                self.assertEqual(f'-var-file={directory}/runtime-ownership.tfvars.json' in arguments, wave != 'none')
                self.assertIn(f'-var-file={directory}/user-history-before.tfvars.json', arguments)
                self.assertNotIn(directory, result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
