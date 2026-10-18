#!/usr/bin/env python3
"""All fixtures are invented; guard projection cannot become ordinary admission."""
import copy
import io
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
sys.path[:0] = [str(ROOT/'.github/scripts'), str(ROOT/'infra/gcp/scripts')]
import validate_history_observation_sync as gate
import test_terraform_plan_diagnostic as legacy_fixture
import test_terraform_history_observation_sync_prototype as state_fixture

CANARY = state_fixture.CANARY
SHA = 'a'*40


def fixture():
    plan, metadata = legacy_fixture.BaselineDiagnosticTest().source()
    for resource in plan['resource_changes']:
        kind = next(kind for _, address, kind in gate.ALIASES if address == resource['address'])
        resource.setdefault('type', kind); resource['provider_name'] = gate.PROVIDER
        for name in ('before','after'):
            resource['change'].setdefault(name, {})
            resource['change'][name].setdefault('project', gate.PROJECT)
    for name, value in (('environment','development'),('project_id',gate.PROJECT)):
        plan['output_changes'][name].update(before=value, after=value)
    history = next(resource for resource in plan['resource_changes'] if resource['address'] == gate.HISTORY)
    for side in ('before','after'): history['change'][side].update(etag='DUMMY_NEW', last_modified_time=200)
    drift = copy.deepcopy(history); drift['change']['actions'] = ['update']
    drift['change']['before'].update(etag='DUMMY_OLD', last_modified_time=100)
    plan['resource_drift'] = [drift]
    return plan, metadata


def args(metadata):
    return dict(metadata=metadata, execution_email=legacy_fixture.fixture.EMAIL, source_sha=SHA)


def candidate(plan):
    modules = {}; root = {'resources': [], 'child_modules': []}
    for resource in plan['resource_changes']:
        address = resource['address']; module = address.rsplit('.',2)[0] if address.startswith('module.') else None
        target = root
        if module:
            if module not in modules:
                modules[module] = dict(address=module, resources=[]); root['child_modules'].append(modules[module])
            target = modules[module]
        tail = address.rsplit('.',1)[1]
        value = dict(address=address, mode='managed', type=resource['type'], name=tail.split('[',1)[0],
            provider_name=gate.PROVIDER, schema_version=gate.SCHEMA_VERSIONS[resource['type']],
            values=copy.deepcopy(resource['change']['after']), sensitive_values={}, identity_schema_version=0, identity=None)
        target['resources'].append(value)
    outputs = {name:dict(value=item['after'], type='string', sensitive=False) for name,item in plan['output_changes'].items()}
    return dict(format_version='1.2', terraform_version=gate.TERRAFORM_VERSION, complete=True, errored=False,
        applyable=True, resource_drift=copy.deepcopy(plan['resource_drift']), output_changes=copy.deepcopy(plan['output_changes']),
        planned_values={'root_module':{},'outputs':copy.deepcopy(outputs)},
        prior_state=dict(format_version='1.0',terraform_version=gate.TERRAFORM_VERSION, values=dict(root_module=root, outputs=outputs)))


class HistoryObservationSyncValidationTest(unittest.TestCase):
    def reject(self, call):
        with patch('sys.stdout',io.StringIO()) as out, patch('sys.stderr',io.StringIO()) as err:
            with self.assertRaises(gate.SyncRejected) as caught: call()
        self.assertNotIn(CANARY,str(caught.exception)+out.getvalue()+err.getvalue())

    def test_strict_ordinary_drift_remains_and_projection_is_discarded(self):
        plan, metadata = fixture(); before = copy.deepcopy(plan)
        delegated = gate.validate_history
        seen = []
        def verify(projection, **kwargs):
            expected = copy.deepcopy(plan); expected['resource_drift'] = []
            self.assertEqual(projection,expected); self.assertIsNot(projection,plan)
            seen.append(True); return delegated(projection,**kwargs)
        with patch.object(gate,'validate_history',side_effect=verify):
            value=gate.validate_observation(plan,**args(metadata))
        self.assertEqual(plan,before);self.assertEqual(seen,[True]);self.assertEqual(value['counts']['drift'],1)
        self.assertFalse(value['execution_permitted']);self.assertNotIn('resources',value)
        self.assertEqual(value['kind'],'dev-history-observation-sync-observation-v1')
        with self.assertRaises(ValueError): delegated(plan,metadata=metadata,phase='post',execution_email=legacy_fixture.fixture.EMAIL)

    def test_existing_guard_rejects_same_value_configuration_change_and_bad_metadata(self):
        plan, metadata = fixture()
        for resource in [plan['resource_changes'][-1],plan['resource_drift'][0]]:
            for side in ('before','after'):resource['change'][side]['deletion_protection']=False
        self.reject(lambda:gate.validate_observation(plan,**args(metadata)))
        plan,metadata=fixture();metadata['schema']['fields'].pop()
        self.reject(lambda:gate.validate_observation(plan,**args(metadata)))

    def test_raw_drift_graph_actions_unknown_marks_and_unexpected_field_cannot_be_hidden(self):
        mutations=[lambda p:p['resource_changes'].pop(),lambda p:p['resource_changes'][0].update(previous_address=CANARY),
            lambda p:p['resource_changes'][0]['change'].update(actions=['update']),
            lambda p:p['resource_drift'][0]['change']['after'].update(num_rows=3),
            lambda p:p['resource_drift'][0]['change'].update(after_unknown={'etag':True}),
            lambda p:p['resource_drift'][0]['change'].update(after_sensitive={'etag':True}),
            lambda p:p['resource_drift'][0]['change'].update(replace_paths=[['etag']]),
            lambda p:p.update(deferred_action_invocations=[{}])]
        for mutate in mutations:
            plan,metadata=fixture();mutate(plan)
            with patch.object(gate,'validate_history') as delegated:
                self.reject(lambda:gate.validate_observation(plan,**args(metadata)));delegated.assert_not_called()

    def test_refresh_show_shape_full12_synthetic_join_stays_execution_false(self):
        plan,metadata=fixture();refresh=candidate(plan)
        result=gate.validate_refresh_candidate(refresh,plan,**args(metadata))
        self.assertTrue(result['show_json_verified']);self.assertFalse(result['raw_candidate_state_verified'])
        self.assertFalse(result['execution_permitted'])

    def test_refresh_candidate_never_normalizes_unknown_resource_changes_or_plan_values(self):
        mutations=[lambda p:p.update(resource_changes=[]),lambda p:p.update(output_changes={}),
            lambda p:p.update(applyable=False),lambda p:p.update(complete=1),
            lambda p:p['planned_values']['root_module'].update(resources=[]),
            lambda p:p['resource_drift'][0]['change']['after'].update(etag=CANARY),
            lambda p:p['prior_state']['values']['root_module']['resources'][0]['values'].update(project='prod'),
            lambda p:p['prior_state']['values']['root_module']['child_modules'][0]['resources'][0].update(identity_schema_version=True)]
        for mutate in mutations:
            plan,metadata=fixture();refresh=candidate(plan);mutate(refresh)
            self.reject(lambda:gate.validate_refresh_candidate(refresh,plan,**args(metadata)))

    def test_full_raw_delta_includes_provider_private_outputs_check_results_and_types(self):
        plan,metadata=fixture();_,old,new,_=state_fixture.fixture()
        changes,drift=gate._observation(plan)
        for state,after in ((old,False),(new,True)):
            _,attrs=gate._state(state_fixture.raw(state))
            # Set input objects through their native container/instance paths.
            for resource in state['resources']:
                base=(resource['module']+'.' if 'module' in resource else '')+resource['type']+'.'+resource['name']
                for instance in resource['instances']:
                    address=base+('['+gate.canonical(instance['index_key'])+']' if 'index_key'in instance else '')
                    instance['attributes']=copy.deepcopy(drift['before'] if address==gate.HISTORY and not after else changes[address])
        self.assertIsNone(gate.validate_raw_transition(state_fixture.raw(old),state_fixture.raw(new),plan,serial_delta=1))
        for mutation in (lambda s:s['resources'][0]['instances'][0].update(private=CANARY+'_changed'),
                         lambda s:s.update(serial=7),lambda s:s['check_results'][0].update(status='fail'),
                         lambda s:s['resources'][-1]['instances'][0]['attributes'].update(etag='C')):
            bad=copy.deepcopy(new);mutation(bad)
            self.reject(lambda:gate.validate_raw_transition(state_fixture.raw(old),state_fixture.raw(bad),plan,serial_delta=1))


if __name__=='__main__':unittest.main()
