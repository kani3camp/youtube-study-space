#!/usr/bin/env python3
"""Synthetic raw State only; no auth, backend, provider, or filesystem calls."""
import copy
import hashlib
import io
import json
from pathlib import Path
import unittest
from unittest.mock import patch

import terraform_history_observation_sync_prototype as prototype
from terraform_baseline_drift_fields import ALIASES, PROVIDER, SCHEMA_VERSIONS

CANARY = 'PRIVATE_CANARY_nonce_member_arn'
ROOT = Path(__file__).resolve().parents[2]


def fixture():
    resources, containers = [], {}
    for _, address, kind in ALIASES:
        prefix, tail = address.rsplit('.', 1)
        name = tail.split('[', 1)[0]
        module = prefix.rsplit('.', 1)[0] if address.startswith('module.') else None
        container = (module, kind, name)
        if container not in containers:
            resource = dict(mode='managed', type=kind, name=name,
                provider='provider["' + PROVIDER + '"]', instances=[])
            if module: resource['module'] = module
            containers[container] = resource; resources.append(resource)
        attributes = dict(project=prototype.PROJECT, id=CANARY)
        if address == prototype.HISTORY:
            attributes.update(id=prototype.HISTORY_ID, dataset_id='firestore_export',
                table_id='user-activity-history', etag='OLD_DUMMY_ETAG', last_modified_time=100)
        instance = dict(schema_version=SCHEMA_VERSIONS[kind], attributes=attributes,
            private=CANARY, dependencies=['DUMMY_DEPENDENCY_1', 'DUMMY_DEPENDENCY_2'], sensitive_attributes=[])
        if '[' in tail: instance['index_key'] = json.loads(tail.split('[', 1)[1][:-1])
        containers[container]['instances'].append(instance)
    before = dict(version=4, terraform_version=prototype.TERRAFORM_VERSION, serial=5,
        lineage='00000000-0000-0000-0000-000000000001', resources=resources,
        outputs={name: dict(type='string', value=value) for name, value in
            (('environment', 'development'), ('project_id', prototype.PROJECT))},
        check_results=[dict(object_kind='var', config_addr='DUMMY_VAR', status='pass', objects=[])])
    after = copy.deepcopy(before); after['serial'] += 1
    def attributes(state):
        _, values = prototype._state(raw(state)); return values
    old, new = attributes(before), attributes(after)
    new[prototype.HISTORY].update(etag='NEW_DUMMY_ETAG', last_modified_time=200)
    # _state parses to a copy, so explicitly update the candidate input.
    history(after).update(etag='NEW_DUMMY_ETAG', last_modified_time=200)
    changes = [dict(address=address, mode='managed', type=kind, provider_name=PROVIDER,
        change=dict(actions=['no-op'], before=copy.deepcopy(new[address]), after=copy.deepcopy(new[address]),
            after_unknown={}, before_sensitive={}, after_sensitive={})) for _, address, kind in ALIASES]
    plan = dict(format_version='1.2', terraform_version=prototype.TERRAFORM_VERSION, complete=True, errored=False,
        resource_changes=changes, resource_drift=[dict(address=prototype.HISTORY, mode='managed',
            type='google_bigquery_table', provider_name=PROVIDER,
            change=dict(actions=['update'], before=copy.deepcopy(old[prototype.HISTORY]),
                after=copy.deepcopy(new[prototype.HISTORY]), after_unknown={}, before_sensitive={}, after_sensitive={}))],
        output_changes={name: dict(actions=['no-op'], before=value, after=value) for name, value in
            (('environment', 'development'), ('project_id', prototype.PROJECT))}, checks=[{'status': 'pass'}])
    return plan, before, after, copy.deepcopy(prototype._CONTEXT)


def raw(value):
    return json.dumps(value, separators=(',', ':')).encode()


def history(state):
    return next(resource for resource in state['resources'] if resource.get('module') == 'module.user_activity_history[0]')['instances'][0]['attributes']


class ObservationSyncPrototypeTest(unittest.TestCase):
    def evaluate(self, plan, before, after, context):
        return prototype.evaluate_prototype(plan, raw(before), raw(after), context=context)

    def reject(self, plan, before, after, context):
        with patch('sys.stdout', io.StringIO()) as out, patch('sys.stderr', io.StringIO()) as err:
            with self.assertRaises(prototype.PrototypeRejected) as caught:
                self.evaluate(plan, before, after, context)
        self.assertNotIn(CANARY, str(caught.exception) + out.getvalue() + err.getvalue())

    def test_exact_two_leaf_hypothetical_transition_is_never_execution_authority(self):
        plan, before, after, context = fixture(); original = copy.deepcopy((plan, before, after, context))
        with patch('builtins.open', side_effect=AssertionError('no file IO')):
            result = self.evaluate(plan, before, after, context)
        self.assertFalse(result['execution_permitted']); self.assertFalse(result['native_refresh_only_serialization_proven'])
        self.assertFalse(result['full_configuration_verified']); self.assertFalse(result['fresh_evidence_verified'])
        self.assertTrue(result['normal_baseline_required']); self.assertEqual(result['prototype_checks'], 'PASS')
        self.assertEqual((plan, before, after, context), original)
        self.assertNotIn(CANARY, json.dumps(result))

    def test_context_source_dev_toolchain_lock_and_unknown_keys_are_exact(self):
        for key in prototype._CONTEXT:
            plan, before, after, context = fixture(); context[key] = CANARY
            self.reject(plan, before, after, context)
        plan, before, after, context = fixture(); context['execution_approved'] = True
        self.reject(plan, before, after, context)
        self.assertEqual(hashlib.sha256((ROOT/'infra/gcp/environments/dev/.terraform.lock.hcl').read_bytes()).hexdigest(),
            prototype.PROVIDER_LOCK_SHA256)
        self.assertEqual((ROOT/'infra/gcp/.terraform-version').read_text().strip(), prototype.TERRAFORM_VERSION)

    def test_graph_extra_missing_duplicate_move_provider_type_and_foreign_project_stop(self):
        mutations = [lambda p: p['resource_changes'].pop(), lambda p: p['resource_changes'].append(copy.deepcopy(p['resource_changes'][0])),
            lambda p: p['resource_changes'].__setitem__(0, copy.deepcopy(p['resource_changes'][1])),
            lambda p: p['resource_changes'][0].update(previous_address=CANARY),
            lambda p: p['resource_changes'][0].update(provider_name=CANARY),
            lambda p: p['resource_changes'][0].update(type=CANARY),
            lambda p: p['resource_changes'][0]['change']['after'].update(project='prod')]
        for mutate in mutations:
            plan, before, after, context = fixture(); mutate(plan); self.reject(plan, before, after, context)

    def test_actions_unknown_sensitivity_replacement_import_and_deferred_changes_stop(self):
        mutations = [lambda p: p['resource_changes'][0]['change'].update(actions=['update']),
            lambda p: p['resource_changes'][0]['change'].update(after_unknown={'id': True}),
            lambda p: p['resource_changes'][0]['change'].update(after_sensitive={'id': True}),
            lambda p: p['resource_changes'][0]['change'].update(replace_paths=[['id']]),
            lambda p: p['resource_changes'][0]['change'].update(importing={'id': CANARY}),
            *[lambda p, key=key: p.update({key: [{}]}) for key in
                ('deferred_changes', 'action_invocations', 'deferred_action_invocations')],
            lambda p: p.update(complete=False), lambda p: p.update(errored=True), lambda p: p.update(applyable=1)]
        for mutate in mutations:
            plan, before, after, context = fixture(); mutate(plan); self.reject(plan, before, after, context)

    def test_exact_two_fields_both_known_types_increasing_time_and_correct_target_only(self):
        mutations = [lambda p: p['resource_drift'].clear(),
            lambda p: p['resource_drift'].append(copy.deepcopy(p['resource_drift'][0])),
            lambda p: p['resource_drift'][0].update(address=ALIASES[2][1]),
            lambda p: p['resource_drift'][0]['change']['after'].update(num_rows=3),
            lambda p: p['resource_drift'][0]['change']['after'].update(etag=None),
            lambda p: p['resource_drift'][0]['change']['after'].pop('etag'),
            lambda p: p['resource_drift'][0]['change']['after'].update(etag=''),
            lambda p: p['resource_drift'][0]['change']['after'].update(last_modified_time=True),
            lambda p: p['resource_drift'][0]['change']['after'].update(last_modified_time=200.0),
            lambda p: p['resource_drift'][0]['change']['after'].update(last_modified_time=100),
            lambda p: p['resource_drift'][0]['change']['after'].update(last_modified_time=99),
            lambda p: p['resource_drift'][0]['change'].update(after_unknown={'etag': True})]
        for mutate in mutations:
            plan, before, after, context = fixture(); mutate(plan); self.reject(plan, before, after, context)

    def test_outputs_and_checks_do_not_accept_hidden_extra_actions_or_failed_results(self):
        for key in ('importing', 'replace_paths', CANARY):
            plan, before, after, context = fixture(); plan['output_changes']['environment'][key] = CANARY
            self.reject(plan, before, after, context)
        for change in ({'status': 'fail'}, {'status': 'unknown'}, {'status': 'pass', 'errors': [CANARY]},
                       {'status': 'pass', 'instances': [{'status': 'unknown'}]}):
            plan, before, after, context = fixture(); plan['checks'] = [change]
            self.reject(plan, before, after, context)

    def test_state_serial_type_exact_increment_and_lineage_are_preserved(self):
        for value in (True, 6.0, 5, 4, 7, 1 << 64):
            plan, before, after, context = fixture(); after['serial'] = value
            self.reject(plan, before, after, context)
        plan, before, after, context = fixture(); after['lineage'] = '00000000-0000-0000-0000-000000000002'
        self.reject(plan, before, after, context)

    def test_entire_other_state_outputs_private_dependencies_checks_identity_and_order_are_unchanged(self):
        mutations = [lambda s: s['outputs']['environment'].update(value=CANARY),
            lambda s: s['resources'][0]['instances'][0].update(private=CANARY + '_changed'),
            lambda s: s['resources'][-1]['instances'][0].update(private=CANARY + '_changed'),
            lambda s: s['resources'][0]['instances'][0]['dependencies'].reverse(),
            lambda s: s['resources'][0]['instances'][0].update(identity={'dummy': CANARY}),
            lambda s: s['resources'][0]['instances'][0].update(schema_version=True),
            lambda s: s['resources'][0].update(provider=CANARY),
            lambda s: s['resources'][0]['instances'][0].update(sensitive_attributes=[['id']]),
            lambda s: s['check_results'][0].update(status='unknown'),
            lambda s: s['resources'].reverse(), lambda s: s.update(extra=CANARY)]
        for mutate in mutations:
            plan, before, after, context = fixture(); mutate(after); self.reject(plan, before, after, context)

    def test_state_type_aware_comparison_detects_boolean_integer_and_float_differences(self):
        for value in (True, 1.0):
            plan, before, after, context = fixture()
            for state in (before, after): state['resources'][0]['instances'][0]['identity'] = {'dummy': 1}
            after['resources'][0]['instances'][0]['identity']['dummy'] = value
            self.reject(plan, before, after, context)

    def test_plan_A_to_B_never_accepts_state_A_to_C_or_reversed_expected_values(self):
        for mutate in (lambda s: history(s).update(etag='WRITER_CHANGED_DUMMY'),
                       lambda s: history(s).update(last_modified_time=300),
                       lambda s: history(s).update(etag='OLD_DUMMY_ETAG', last_modified_time=100),
                       lambda s: history(s).pop('etag'), lambda s: history(s).update(etag=None)):
            plan, before, after, context = fixture(); mutate(after); self.reject(plan, before, after, context)
        plan, before, after, context = fixture(); history(before).update(etag='OTHER_OLD_DUMMY')
        self.reject(plan, before, after, context)

    def test_state_same_named_leaf_elsewhere_target_duplicate_and_extra_changes_stop(self):
        plan, before, after, context = fixture(); after['resources'][2]['instances'][0]['attributes'].update(etag='ELSEWHERE')
        self.reject(plan, before, after, context)
        plan, before, after, context = fixture(); after['resources'].append(copy.deepcopy(after['resources'][-1]))
        self.reject(plan, before, after, context)
        for mutate in (lambda s: history(s).update(extra_field=CANARY),
                       lambda s: s['resources'][-1]['instances'].append(copy.deepcopy(s['resources'][-1]['instances'][0]))):
            plan, before, after, context = fixture(); mutate(before); self.reject(plan, before, after, context)

    def test_bad_raw_json_duplicate_nonfinite_budget_and_depth_are_private_stop(self):
        plan, before, after, context = fixture()
        raws = [b'{"serial":1,"serial":2}', ('{"' + CANARY).encode(), b'{"x":NaN}', b'x'*(prototype.MAX_BYTES+1)]
        for value in raws:
            with self.assertRaises(prototype.PrototypeRejected) as error:
                prototype.evaluate_prototype(plan, value, raw(after), context=context)
            self.assertNotIn(CANARY, str(error.exception))
        for value in (float('nan'), float('inf'), 1 << 64, {1: CANARY}, (CANARY,)):
            candidate = copy.deepcopy(plan); candidate['variables'] = {'dummy': value}
            self.reject(candidate, before, after, context)
        nested = True
        for _ in range(34): nested = [nested]
        candidate = copy.deepcopy(plan); candidate['variables'] = {'dummy': nested}
        self.reject(candidate, before, after, context)

    def test_native_refresh_only_candidate_always_stops_instead_of_guessing_a_parser(self):
        for candidate in (None, {}, {'resource_drift': []}, {'saved_plan': CANARY}):
            with self.assertRaises(prototype.PrototypeRejected) as error:
                prototype.validate_refresh_only_candidate(candidate, context=fixture()[3])
            self.assertEqual(error.exception.reason, 'UNPROVEN_REFRESH_ONLY')
            self.assertNotIn(CANARY, str(error.exception))


if __name__ == '__main__': unittest.main()
