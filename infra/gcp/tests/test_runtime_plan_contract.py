#!/usr/bin/env python3
"""Public dummy-only graph, value and import-fidelity contracts."""
from __future__ import annotations

import copy
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
import runtime_plan_selection as selection_module
import validate_runtime_ownership_plan as gate
from test_runtime_ownership import inventory
import test_gcp_user_activity_schema_audit_workflow as history_tests
from test_terraform_export_function_gate import EMAIL

SHA = 'a' * 40
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
    return dict(wave=wave, inventory=inventory(), adopted=old, selected=new)


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


def fixture(index=0, phase='before', *, noop=False):
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
    if noop:
        p.update(wave='noop', adopted=copy.deepcopy(p['selected']))
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
                             phase=phase, execution_email=EMAIL)

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

    def test_noop_accepts_each_nonempty_adopted_stage_without_imports(self):
        for index, count in enumerate([13, 14, 15, 16, 17]):
            for phase in ['before', 'post']:
                with self.subTest(index=index, phase=phase):
                    plan, metadata, p = fixture(index, phase, noop=True)
                    result = self.review(plan, metadata, p, phase)
                    self.assertEqual(p['selected'], p['adopted'])
                    self.assertEqual(result['counts']['no-op'], count)
                    self.assertEqual(result['counts']['import'], 0)

    def test_noop_rejects_empty_invalid_duplicate_or_changed_selection(self):
        mutations = [
            lambda p: p.update(adopted=selection(), selected=selection()),
            lambda p: p['selected'].update(grants=[]),
            lambda p: p['selected'].update(grants=['grant-01', 'grant-02']),
            lambda p: p['selected'].update(grants=['grant-02']),
            lambda p: p['selected'].update(grants=['grant-01', 'grant-01']),
            lambda p: p['selected'].update(pool=False),
            lambda p: p['selected'].update(apis=['pubsub.googleapis.com']),
        ]
        for index, mutate in enumerate(mutations):
            _, _, p = fixture(2, noop=True)
            mutate(p)
            with self.subTest(case=index), self.assertRaises(ValueError):
                selection_module.validate_selection(p, wave='noop')

    def test_api_adoption_still_requires_provider_and_all_required_grants(self):
        for index in range(3):
            _, _, p = fixture(index, noop=True)
            p.update(wave='api', selected=dict(p['selected'], apis=['pubsub.googleapis.com']))
            with self.subTest(index=index), self.assertRaises(ValueError):
                selection_module.validate_selection(p, wave='api')

    def test_partial_noop_rejects_imports_actions_and_mismatched_complete_graphs(self):
        mutations = [
            lambda plan: plan['resource_changes'][-1]['change'].update(importing={'id': 'dummy'}),
            lambda plan: plan['resource_changes'][-1]['change'].update(actions=['create']),
            lambda plan: plan['resource_changes'].pop(),
            lambda plan: plan['resource_changes'].append(copy.deepcopy(plan['resource_changes'][-1])),
            lambda plan: plan['resource_changes'][12]['change']['after'].update(name='dummy swapped pool'),
            lambda plan: plan['resource_changes'].append(dict(copy.deepcopy(plan['resource_changes'][-1]),
                address='module.unapproved.google_project_service.owned["dummy.googleapis.com"]')),
        ]
        for index in range(5):
            for case, mutate in enumerate(mutations):
                plan, metadata, p = fixture(index, noop=True)
                mutate(plan)
                planned_values(plan)
                with self.subTest(index=index, case=case), self.assertRaises(ValueError):
                    self.review(plan, metadata, p)

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
            (0, lambda p: p.update(extra='dummy private sentinel')),
        ]
        for index, mutation in changes:
            p = packet(index)
            mutation(p)
            with self.subTest(index=index, mutation=changes.index((index, mutation))), self.assertRaises((ValueError, KeyError)):
                selection_module.validate_selection(p, wave=p['wave'])

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
        candidate = selection_module.validate_selection(p, wave='api')
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

    def test_own_api_dependency_requires_consumer_instead_of_api_ownership(self):
        service = 'pubsub.googleapis.com'
        self_address = f'module.owned_apis.google_project_service.owned["{service}"]'
        consumer = 'module.export_topic[0].google_pubsub_topic.export'
        api_dependencies = [
            [self_address],
            ['google_project_service.other'],
            ['module.other.google_project_service.owned["dummy.googleapis.com"]'],
            [consumer, self_address],
        ]
        for phase in ('before', 'post'):
            for dependencies in api_dependencies:
                plan, metadata, p = fixture(4, phase)
                p['inventory']['api_classification'][service]['dependency_addresses'] = dependencies
                with self.subTest(phase=phase, dependencies=dependencies), self.assertRaises(ValueError):
                    # Fails while staging the reviewed packet, before cloud
                    # auth or any protected plan can treat it as evidence.
                    selection_module.validate_selection(p, wave='api')
                with self.subTest(phase=phase, protected=True), self.assertRaises(ValueError):
                    self.review(plan, metadata, p, phase)
            plan, metadata, p = fixture(4, phase)
            p['inventory']['api_classification'][service]['dependency_addresses'] = [consumer]
            result = self.review(plan, metadata, p, phase)
            self.assertEqual(result['counts']['import'], int(phase == 'before'))
            self.assertEqual(result['counts']['no-op'], 17)

    def test_review_classification_never_promotes_external_or_investigate_api(self):
        for classification in ('Platform/External', 'Investigate', 'Do not own'):
            p = packet(4)
            p['inventory']['api_classification']['pubsub.googleapis.com']['classification'] = classification
            # Classification can stay in the inventory, but selection of its
            # one enabled API must still be explicitly rejected.
            candidate = preparer.prepare(p['inventory'])
            self.assertEqual(candidate['owned_api_keys'], [])
            with self.subTest(classification=classification), self.assertRaises(ValueError):
                selection_module.validate_selection(p, wave='api')

    def test_protected_graph_independently_requires_non_api_consumers(self):
        plan, metadata, p = fixture(4)
        candidate = selection_module.validate_selection(p, wave='api')
        candidate['api_classification']['pubsub.googleapis.com']['dependency_addresses'] = [
            'module.owned_apis.google_project_service.owned["pubsub.googleapis.com"]']
        # Inject a synthetic already-staged candidate to exercise the graph's
        # second guard independently of pre-credential packet validation. This
        # API address exists in the real fixture graph, but is not a consumer.
        with patch.object(gate, 'validate_selection', return_value=candidate), self.assertRaises(ValueError):
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



if __name__ == '__main__':
    unittest.main()
