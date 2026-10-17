"""Pure, disconnected review prototype. It cannot authorize or execute writes.

Only an ordinary observed plan and hypothetical raw State transition are modeled.
The real refresh-only saved-plan serializer is unproven and always rejected.
"""
from __future__ import annotations

import json
import uuid

import terraform_baseline_drift_diagnostic as diagnostic
from terraform_baseline_drift_fields import ALIASES, FIELDS, PROVIDER, PROVIDER_VERSION, SCHEMA_VERSIONS
from terraform_plan_summary import COUNT_KEYS

# Source of the ordinary observation being modeled, not this prototype's
# eventual consumer SHA or any execution admission.
SOURCE_SHA = '8616c732cb57f34c48e8fc2eccb40a9afba97653'
TERRAFORM_VERSION = '1.16.4'
PROVIDER_LOCK_SHA256 = '37b583f275a3592eb5b13c81005949ce6f9ef611420d3108398f046bcd12c93e'
PROJECT = 'test-youtube-study-space'
HISTORY = 'module.user_activity_history[0].google_bigquery_table.retained'
HISTORY_ID = 'projects/test-youtube-study-space/datasets/firestore_export/tables/user-activity-history'
LEAVES = ('etag', 'last_modified_time')
MAX_BYTES = 4 * 1024 * 1024
_CONTEXT = dict(schema_version=1, kind='dev-history-observation-sync-prototype', environment='dev',
    source_sha=SOURCE_SHA, terraform_version=TERRAFORM_VERSION, provider_version=PROVIDER_VERSION,
    provider_lock_sha256=PROVIDER_LOCK_SHA256, operation='offline-prototype')
_PLAN_KEYS = frozenset(('format_version', 'terraform_version', 'complete', 'errored', 'applyable',
    'resource_changes', 'resource_drift', 'output_changes', 'checks', 'deferred_changes',
    'action_invocations', 'deferred_action_invocations', 'prior_state', 'planned_values',
    'configuration', 'variables', 'timestamp', 'relevant_attributes'))
_STATE_KEYS = frozenset(('version', 'terraform_version', 'serial', 'lineage', 'outputs', 'resources', 'check_results'))
_RESOURCE_KEYS = frozenset(('module', 'mode', 'type', 'name', 'provider', 'instances'))
_INSTANCE_KEYS = frozenset(('schema_version', 'attributes', 'sensitive_attributes', 'private',
    'dependencies', 'index_key', 'identity_schema_version', 'identity', 'create_before_destroy'))
_REASONS = frozenset(('CONTEXT', 'SHAPE', 'PLAN', 'GRAPH', 'ACTION', 'MASK', 'DRIFT',
    'VALUES', 'STATE', 'STATE_DELTA', 'UNPROVEN_REFRESH_ONLY'))


class PrototypeRejected(ValueError):
    def __init__(self, reason):
        self.reason = reason if reason in _REASONS else 'SHAPE'
        super().__init__('State observation prototype STOP: ' + self.reason)


def require(condition, reason):
    if not condition:
        raise PrototypeRejected(reason)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False)


def _context(value):
    _bounded(value)
    require(type(value) is dict and canonical(value) == canonical(_CONTEXT), 'CONTEXT')


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
    projection = diagnostic.project(plan)
    require(projection is not None and len(projection['items']) == 1, 'DRIFT')
    history_index = next(i for i, (_, address, _) in enumerate(ALIASES) if address == HISTORY)
    fields = FIELDS['google_bigquery_table']
    bits = format(sum(1 << fields.index(name) for name in LEAVES), 'x')
    require(projection['items'] == [[history_index, 'update', 3, '0', '0', bits, '0', '0', '0']], 'DRIFT')
    expected_counts = dict.fromkeys(COUNT_KEYS, 0) | {'no-op': 12, 'drift': 1}
    require(diagnostic.legacy.counts(plan) == expected_counts, 'ACTION')
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
        raise PrototypeRejected('SHAPE')
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
                and instance.get('create_before_destroy', False) is False, 'STATE')
            index = instance.get('index_key')
            require('index_key' not in instance or type(index) in (str, int), 'STATE')
            suffix = '[' + canonical(index) + ']' if 'index_key' in instance else ''
            address = (resource['module'] + '.' if 'module' in resource else '') + resource['type'] + '.' + resource['name'] + suffix
            require(address not in addresses, 'STATE'); addresses[address] = attributes
    require(set(addresses) == {address for _, address, _ in ALIASES}, 'GRAPH')
    return state, addresses


def evaluate_prototype(observation, before_raw, after_raw, *, context):
    """Check synthetic evidence only; even success permits no execution."""
    try:
        _context(context)
        changes, drift = _observation(observation)
        before, old = _state(before_raw); after, new = _state(after_raw)
        require(after['serial'] == before['serial'] + 1, 'STATE_DELTA')
        for address in old:
            expected_old = drift['before'] if address == HISTORY else changes[address]
            require(canonical(old[address]) == canonical(expected_old)
                and canonical(new[address]) == canonical(changes[address]), 'STATE_DELTA')
        # Compare all root/resource/instance fields, including opaque provider
        # private, dependencies, identity, checks and outputs. No list reordering.
        for state, attributes in ((before, old), (after, new)):
            state.pop('serial')
            for name in LEAVES: attributes[HISTORY].pop(name)
        require(canonical(before) == canonical(after), 'STATE_DELTA')
        return dict(kind='dev-history-observation-sync-prototype-result', prototype_checks='PASS',
            execution_permitted=False, native_refresh_only_serialization_proven=False,
            full_configuration_verified=False, fresh_evidence_verified=False,
            normal_baseline_required=True)
    except PrototypeRejected:
        raise
    except Exception:
        raise PrototypeRejected('SHAPE') from None


def validate_refresh_only_candidate(_candidate, *, context):
    """Intentionally no guessed parser for unproven native serialization."""
    try:
        _context(context)
    except PrototypeRejected:
        raise
    except Exception:
        raise PrototypeRejected('SHAPE') from None
    raise PrototypeRejected('UNPROVEN_REFRESH_ONLY')
