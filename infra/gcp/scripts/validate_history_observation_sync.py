"""Strict, separate history observation-sync validation; no execution client.

The only projection with drift=[] is confined to the unchanged 55-guard call.
All public results retain raw drift1. Candidate acceptance never permits apply.
"""
from __future__ import annotations
import copy
import json
from pathlib import Path
import re
import sys
import uuid

sys.path.insert(0, str(Path(__file__).resolve().parents[3] / '.github/scripts'))
import terraform_baseline_drift_diagnostic as diagnostic
from terraform_baseline_drift_fields import ALIASES, FIELDS, PROVIDER, PROVIDER_VERSION, SCHEMA_VERSIONS
from terraform_plan_summary import COUNT_KEYS
from validate_user_activity_history_plan import validate as validate_history

TERRAFORM_VERSION = '1.16.4'
PROJECT = 'test-youtube-study-space'
HISTORY = 'module.user_activity_history[0].google_bigquery_table.retained'
HISTORY_ID = 'projects/test-youtube-study-space/datasets/firestore_export/tables/user-activity-history'
LEAVES = ('etag', 'last_modified_time')
MAX_BYTES = 4194304
MAX_PLAN_BYTES = 16 * 1024 * 1024
_REASONS = frozenset(('SHAPE','PLAN','GRAPH','ACTION','MASK','DRIFT','VALUES','STATE','STATE_DELTA','CONFIGURATION','CANDIDATE','ARCHIVE','APPLY_UNAVAILABLE'))
_PLAN_KEYS = frozenset(('format_version', 'terraform_version', 'complete', 'errored', 'applyable',
    'resource_changes', 'resource_drift', 'output_changes', 'checks', 'deferred_changes',
    'action_invocations', 'deferred_action_invocations', 'prior_state', 'planned_values',
    'configuration', 'variables', 'timestamp', 'relevant_attributes'))
_STATE_KEYS = frozenset(('version', 'terraform_version', 'serial', 'lineage', 'outputs', 'resources', 'check_results'))
_RESOURCE_KEYS = frozenset(('module', 'mode', 'type', 'name', 'provider', 'instances'))
_INSTANCE_KEYS = frozenset(('schema_version', 'attributes', 'sensitive_attributes', 'private',
    'dependencies', 'index_key', 'identity_schema_version', 'identity', 'create_before_destroy'))

class SyncRejected(ValueError):
    def __init__(self, reason):
        self.reason = reason if reason in _REASONS else 'SHAPE'
        super().__init__('History observation sync STOP: ' + self.reason)


def require(condition, reason):
    if not condition:
        raise SyncRejected(reason)

def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False)

def _bounded(value):
    diagnostic._bounded_json(value, [diagnostic.MAX_NODES, MAX_BYTES])
    def integer_budget(item):
        if type(item) is int:
            require(-(1 << 63) <= item < (1 << 63), 'SHAPE')
        elif type(item) is dict:
            for child in item.values(): integer_budget(child)
        elif type(item) is list:
            for child in item: integer_budget(child)
    integer_budget(value)
    require(len(canonical(value).encode()) <= MAX_BYTES, 'SHAPE')

def _change(change, fields, action):
    require(type(change) is dict and set(change) <= diagnostic._CHANGE_KEYS, 'ACTION')
    require(change.get('actions') == [action] and change.get('replace_paths', []) == [], 'ACTION')
    for name in ('before', 'after'):
        value = change.get(name)
        require(type(value) is dict and set(value) <= set(fields) and value.get('project') == PROJECT, 'VALUES')
    unknown = change.get('after_unknown', {})
    diagnostic._top_mask(unknown, fields)
    require(not diagnostic._mask_true(unknown), 'MASK')
    before, after = (change.get(name, {}) for name in ('before_sensitive', 'after_sensitive'))
    diagnostic._top_mask(before, fields); diagnostic._top_mask(after, fields)
    require(canonical(before) == canonical(after), 'MASK')

def _observation(plan):
    _bounded(plan)
    require(type(plan) is dict and set(plan) <= _PLAN_KEYS and plan.get('format_version') == '1.2'
        and plan.get('terraform_version') == TERRAFORM_VERSION, 'PLAN')
    require('applyable' not in plan or type(plan['applyable']) is bool, 'PLAN')
    changes_list = plan.get('resource_changes')
    require(type(changes_list) is list and len(changes_list) == 12, 'GRAPH')
    indices = [diagnostic._identity(item)[0] for item in changes_list]
    require(set(indices) == set(range(12)) and len(set(indices)) == 12, 'GRAPH')
    drift_list = plan.get('resource_drift')
    require(type(drift_list) is list and len(drift_list) == 1, 'DRIFT')
    index, _ = diagnostic._identity(drift_list[0])
    require(ALIASES[index][1] == HISTORY, 'DRIFT')
    for name in ('deferred_changes', 'action_invocations', 'deferred_action_invocations'):
        require(type(plan.get(name, [])) is list and plan.get(name, []) == [], 'ACTION')
    require(plan.get('complete') is True and plan.get('errored') is False, 'PLAN')
    changes = {}
    for item in plan['resource_changes']:
        _, fields = diagnostic._identity(item)
        _change(item['change'], fields, 'no-op')
        require(canonical(item['change']['before']) == canonical(item['change']['after']), 'VALUES')
        changes[item['address']] = item['change']['after']
    outputs = plan.get('output_changes')
    require(type(outputs) is dict and set(outputs) == {'environment', 'project_id'}, 'PLAN')
    for name, expected in (('environment', 'development'), ('project_id', PROJECT)):
        change = outputs[name]
        require(type(change) is dict and {'actions', 'before', 'after'} <= set(change)
            <= {'actions', 'before', 'after', 'after_unknown', 'before_sensitive', 'after_sensitive'}
            and change.get('actions') == ['no-op']
            and change.get('before') == change.get('after') == expected
            and not diagnostic._mask_true(change.get('after_unknown', False))
            and change.get('before_sensitive', False) is False
            and change.get('after_sensitive', False) is False, 'PLAN')
    checks = plan.get('checks', [])
    require(type(checks) is list, 'PLAN')
    for check in checks:
        require(type(check) is dict and {'status'} <= set(check) <= {'address', 'status', 'instances'}
            and check['status'] == 'pass', 'PLAN')
        instances = check.get('instances', [])
        require(type(instances) is list and all(type(item) is dict and {'status'} <= set(item) <= {'address', 'status'}
            and item['status'] == 'pass' for item in instances), 'PLAN')
    drift = plan['resource_drift'][0]['change']
    _change(drift, FIELDS['google_bigquery_table'], 'update')
    for value in (drift['before'], drift['after']):
        require(value.get('id') == HISTORY_ID and value.get('dataset_id') == 'firestore_export'
            and value.get('table_id') == 'user-activity-history', 'VALUES')
        require(type(value.get('etag')) is str and bool(value['etag'])
            and type(value.get('last_modified_time')) is int and value['last_modified_time'] >= 0, 'VALUES')
    require(set(drift['before']) == set(drift['after']), 'DRIFT')
    changed = {key for key in drift['before'] if canonical(drift['before'][key]) != canonical(drift['after'][key])}
    require(changed == set(LEAVES), 'DRIFT')
    require(canonical(changes[HISTORY]) == canonical(drift['after']), 'VALUES')
    require(drift['after']['last_modified_time'] > drift['before']['last_modified_time'], 'VALUES')
    return changes, drift

def _unique(pairs):
    result = {}
    for key, value in pairs:
        require(key not in result, 'SHAPE'); result[key] = value
    return result

def _state(raw):
    require(type(raw) is bytes and 0 < len(raw) <= MAX_BYTES, 'STATE')
    def nonfinite(_):
        raise SyncRejected('SHAPE')
    state = json.loads(raw, object_pairs_hook=_unique, parse_constant=nonfinite)
    _bounded(state)
    require(type(state) is dict and {'version', 'terraform_version', 'serial', 'lineage', 'outputs', 'resources'} <= set(state) <= _STATE_KEYS, 'STATE')
    require(type(state['version']) is int and state['version'] == 4
        and state['terraform_version'] == TERRAFORM_VERSION
        and type(state['serial']) is int and state['serial'] >= 0, 'STATE')
    require(type(state['lineage']) is str and str(uuid.UUID(state['lineage'])) == state['lineage'], 'STATE')
    require(type(state['outputs']) is dict and set(state['outputs']) == {'environment', 'project_id'}, 'STATE')
    for name, expected in (('environment', 'development'), ('project_id', PROJECT)):
        output = state['outputs'][name]
        require(type(output) is dict and {'type', 'value'} <= set(output) <= {'type', 'value', 'sensitive'}
            and output['type'] == 'string' and output['value'] == expected and output.get('sensitive', False) is False, 'STATE')
    require(type(state['resources']) is list, 'STATE')
    addresses, containers = {}, set()
    for resource in state['resources']:
        require(type(resource) is dict and {'mode', 'type', 'name', 'provider', 'instances'} <= set(resource) <= _RESOURCE_KEYS, 'STATE')
        require(resource['mode'] == 'managed' and resource['provider'] == 'provider["' + PROVIDER + '"]', 'STATE')
        container = (resource.get('module', ''), resource['type'], resource['name'])
        require(container not in containers, 'STATE'); containers.add(container)
        require(type(resource['instances']) is list and bool(resource['instances']), 'STATE')
        for instance in resource['instances']:
            require(type(instance) is dict and {'schema_version', 'attributes'} <= set(instance) <= _INSTANCE_KEYS, 'STATE')
            require(type(instance['schema_version']) is int and instance['schema_version'] == SCHEMA_VERSIONS[resource['type']], 'STATE')
            attributes = instance['attributes']
            require(type(attributes) is dict and set(attributes) <= set(FIELDS[resource['type']]) and attributes.get('project') == PROJECT, 'STATE')
            require(type(instance.get('private', '')) is str and type(instance.get('dependencies', [])) is list
                and all(type(item) is str for item in instance.get('dependencies', []))
                and type(instance.get('sensitive_attributes', [])) is list
                and instance.get('create_before_destroy', False) is False
                and type(instance.get('identity_schema_version',0)) is int
                and instance.get('identity_schema_version',0) >= 0
                and (instance.get('identity') is None or type(instance['identity']) is dict), 'STATE')
            index = instance.get('index_key')
            require('index_key' not in instance or type(index) in (str, int), 'STATE')
            suffix = '[' + canonical(index) + ']' if 'index_key' in instance else ''
            address = (resource['module'] + '.' if 'module' in resource else '') + resource['type'] + '.' + resource['name'] + suffix
            require(address not in addresses, 'STATE'); addresses[address] = attributes
    require(set(addresses) == {address for _, address, _ in ALIASES}, 'GRAPH')
    return state, addresses


COUNTS = dict.fromkeys(COUNT_KEYS, 0) | {'no-op': 12, 'drift': 1}


def validate_observation(plan, *, metadata, execution_email, source_sha):
    """Validate exact drift first, then delegate only a discarded validation copy."""
    try:
        require(type(source_sha) is str and re.fullmatch('[0-9a-f]{40}', source_sha), 'PLAN')
        _observation(plan)
        projection = copy.deepcopy(plan)
        projection['resource_drift'] = []
        try:
            validate_history(projection, metadata=metadata, phase='post', execution_email=execution_email)
        except Exception:
            raise SyncRejected('CONFIGURATION') from None
        # Never return projection, build a normal summary from it, or issue a
        # receipt. Only this dedicated kind retains the original drift count.
        return dict(schema_version=1, kind='dev-history-observation-sync-observation-v1',
            source_sha=source_sha, target='dev', resource_count=12, counts=dict(COUNTS),
            drift_alias='USER_ACTIVITY_HISTORY', field_codes=['ETAG','LAST_MODIFIED_TIME'],
            existing_guard_validation='PASS', execution_permitted=False)
    except SyncRejected:
        raise
    except Exception:
        raise SyncRejected('SHAPE') from None


def _module_resources(module, result, *, root=False, seen=None):
    if seen is None:
        seen = set()
    known_modules = {address.rsplit('.', 2)[0] for _, address, _ in ALIASES if address.startswith('module.')}
    require(type(module) is dict and set(module) <= ({'resources','child_modules'} if root else {'address','resources','child_modules'}), 'CANDIDATE')
    if not root:
        require(type(module.get('address')) is str and module['address'] in known_modules
            and module['address'] not in seen, 'CANDIDATE')
        seen.add(module['address'])
    resources = module.get('resources', [])
    require(type(resources) is list, 'CANDIDATE')
    for resource in resources:
        require(type(resource) is dict and {'address','mode','type','name','provider_name','schema_version','values','sensitive_values'} <= set(resource)
            <= {'address','module_address','mode','type','name','index','provider_name','schema_version','values','sensitive_values','identity','identity_schema_version','depends_on'}, 'CANDIDATE')
        address = resource['address']
        index = next((i for i, (_, expected, _) in enumerate(ALIASES) if expected == address), None)
        require(index is not None and address not in result and resource['mode'] == 'managed'
            and resource['type'] == ALIASES[index][2] and resource['provider_name'] == PROVIDER
            and type(resource['schema_version']) is int and resource['schema_version'] == SCHEMA_VERSIONS[resource['type']], 'CANDIDATE')
        identity = {key: resource[key] for key in diagnostic._RESOURCE_KEYS if key in resource}
        diagnostic._identity(identity)
        expected_module = address.rsplit('.', 2)[0] if address.startswith('module.') else None
        require(expected_module == (None if root else module['address']), 'CANDIDATE')
        require(type(resource['values']) is dict, 'CANDIDATE')
        diagnostic._top_mask(resource['sensitive_values'], FIELDS[resource['type']])
        require('identity_schema_version' not in resource or type(resource['identity_schema_version']) is int
            and resource['identity_schema_version'] >= 0, 'CANDIDATE')
        require('identity' not in resource or resource['identity'] is None or type(resource['identity']) is dict, 'CANDIDATE')
        require(type(resource.get('depends_on',[]))is list
            and all(type(item)is str for item in resource.get('depends_on',[])), 'CANDIDATE')
        result[address] = resource
    children = module.get('child_modules', [])
    require(type(children) is list, 'CANDIDATE')
    # All currently-owned source modules are one level below the root.
    require(root or not children, 'CANDIDATE')
    for child in children: _module_resources(child, result, seen=seen)
    if root:
        require(seen == known_modules, 'CANDIDATE')


def validate_refresh_candidate(candidate, observation, *, metadata, execution_email, source_sha):
    """Check native show-JSON structure; opaque/raw State proof is separate."""
    try:
        validate_observation(observation, metadata=metadata, execution_email=execution_email, source_sha=source_sha)
        changes, drift = _observation(observation)
        _bounded(candidate)
        allowed = _PLAN_KEYS - {'resource_changes'}
        require(type(candidate) is dict and set(candidate) <= allowed and candidate.get('format_version') == '1.2'
            and candidate.get('terraform_version') == TERRAFORM_VERSION and candidate.get('applyable') is True
            and candidate.get('complete') is True and candidate.get('errored') is False, 'CANDIDATE')
        require(canonical(candidate.get('resource_drift')) == canonical(observation['resource_drift'])
            and canonical(candidate.get('output_changes')) == canonical(observation['output_changes']), 'CANDIDATE')
        for name in ('deferred_changes','action_invocations','deferred_action_invocations'):
            require(type(candidate.get(name, [])) is list and candidate.get(name, []) == [], 'CANDIDATE')
        for name in ('configuration','variables'):
            require(canonical(candidate.get(name)) == canonical(observation.get(name)), 'CANDIDATE')
        require(canonical(candidate.get('checks', [])) == canonical(observation.get('checks', [])), 'CANDIDATE')
        prior = candidate.get('prior_state')
        require(type(prior) is dict and set(prior) == {'format_version','terraform_version','values'}
            and prior['format_version'] == '1.0' and prior['terraform_version'] == TERRAFORM_VERSION, 'CANDIDATE')
        values = prior['values']
        require(type(values) is dict and {'root_module'} <= set(values) <= {'root_module','outputs'}, 'CANDIDATE')
        outputs = values.get('outputs', {})
        require(type(outputs) is dict and set(outputs) == {'environment','project_id'}, 'CANDIDATE')
        for name, expected in (('environment','development'),('project_id',PROJECT)):
            item = outputs[name]
            require(type(item) is dict and set(item) == {'value','type','sensitive'}
                and item['value'] == expected and item['type'] == 'string' and item['sensitive'] is False, 'CANDIDATE')
        found = {}; _module_resources(values['root_module'], found, root=True)
        require(set(found) == set(changes) and all(canonical(found[address]['values']) == canonical(expected)
            for address, expected in changes.items()), 'CANDIDATE')
        planned = candidate.get('planned_values')
        # Native module/root-output witness: refresh-only omits resources but
        # retains the two root output values and their exact no-op changes.
        require(type(planned) is dict and set(planned) == {'root_module','outputs'}
            and planned['root_module'] == {} and canonical(planned['outputs']) == canonical(outputs), 'CANDIDATE')
        return dict(kind='dev-history-observation-sync-candidate-v1', counts=dict(COUNTS),
            show_json_verified=True, raw_candidate_state_verified=False,
            execution_permitted=False)
    except SyncRejected:
        raise
    except Exception:
        raise SyncRejected('SHAPE') from None


def validate_raw_transition(before_raw, after_raw, observation, *, serial_delta):
    """Strict full-payload delta; no schema upgrades or arbitrary normalizations."""
    try:
        require(type(serial_delta) is int and serial_delta in (0,1), 'STATE_DELTA')
        changes, drift = _observation(observation)
        before, old = _state(before_raw); after, new = _state(after_raw)
        require(after['serial'] == before['serial'] + serial_delta, 'STATE_DELTA')
        for address in old:
            expected_old = drift['before'] if address == HISTORY else changes[address]
            require(canonical(old[address]) == canonical(expected_old)
                and canonical(new[address]) == canonical(changes[address]), 'STATE_DELTA')
        for state, attributes in ((before,old),(after,new)):
            state.pop('serial')
            for name in LEAVES: attributes[HISTORY].pop(name)
        require(canonical(before) == canonical(after), 'STATE_DELTA')
        return None
    except SyncRejected:
        raise
    except Exception:
        raise SyncRejected('SHAPE') from None
