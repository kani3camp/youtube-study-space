#!/usr/bin/env python3
"""Synthetic data for legacy observation guard regression tests; no prototype runtime."""
import copy
import json

import validate_history_observation_sync as gate
from terraform_baseline_drift_fields import ALIASES, PROVIDER, SCHEMA_VERSIONS

CANARY = 'PRIVATE_CANARY_nonce_member_arn'
HISTORY_ID = ('projects/' + gate.PROJECT + '/datasets/firestore_export/'
              'tables/user-activity-history')
TEST_ONLY_CONTEXT = {'kind': 'synthetic-fixture', 'execution_permitted': False}


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
        attributes = dict(project=gate.PROJECT, id=CANARY)
        if address == gate.HISTORY:
            attributes.update(id=HISTORY_ID, dataset_id='firestore_export',
                table_id='user-activity-history', etag='OLD_DUMMY_ETAG', last_modified_time=100)
        instance = dict(schema_version=SCHEMA_VERSIONS[kind], attributes=attributes,
            private=CANARY, dependencies=['DUMMY_DEPENDENCY_1', 'DUMMY_DEPENDENCY_2'], sensitive_attributes=[])
        if '[' in tail: instance['index_key'] = json.loads(tail.split('[', 1)[1][:-1])
        containers[container]['instances'].append(instance)
    before = dict(version=4, terraform_version=gate.TERRAFORM_VERSION, serial=5,
        lineage='00000000-0000-0000-0000-000000000001', resources=resources,
        outputs={name: dict(type='string', value=value) for name, value in
            (('environment', 'development'), ('project_id', gate.PROJECT))},
        check_results=[dict(object_kind='var', config_addr='DUMMY_VAR', status='pass', objects=[])])
    after = copy.deepcopy(before); after['serial'] += 1
    def attributes(state):
        _, values = gate._state(raw(state)); return values
    old, new = attributes(before), attributes(after)
    new[gate.HISTORY].update(etag='NEW_DUMMY_ETAG', last_modified_time=200)
    # _state parses to a copy, so explicitly update the candidate input.
    history(after).update(etag='NEW_DUMMY_ETAG', last_modified_time=200)
    changes = [dict(address=address, mode='managed', type=kind, provider_name=PROVIDER,
        change=dict(actions=['no-op'], before=copy.deepcopy(new[address]), after=copy.deepcopy(new[address]),
            after_unknown={}, before_sensitive={}, after_sensitive={})) for _, address, kind in ALIASES]
    plan = dict(format_version='1.2', terraform_version=gate.TERRAFORM_VERSION, complete=True, errored=False,
        resource_changes=changes, resource_drift=[dict(address=gate.HISTORY, mode='managed',
            type='google_bigquery_table', provider_name=PROVIDER,
            change=dict(actions=['update'], before=copy.deepcopy(old[gate.HISTORY]),
                after=copy.deepcopy(new[gate.HISTORY]), after_unknown={}, before_sensitive={}, after_sensitive={}))],
        output_changes={name: dict(actions=['no-op'], before=value, after=value) for name, value in
            (('environment', 'development'), ('project_id', gate.PROJECT))}, checks=[{'status': 'pass'}])
    return plan, before, after, copy.deepcopy(TEST_ONLY_CONTEXT)


def raw(value):
    return json.dumps(value, separators=(',', ':')).encode()


def history(state):
    return next(resource for resource in state['resources'] if resource.get('module') == 'module.user_activity_history[0]')['instances'][0]['attributes']


