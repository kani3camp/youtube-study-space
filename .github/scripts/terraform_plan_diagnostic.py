#!/usr/bin/env python3
"""Bounded failure-only enums/counts; never an admission or state receipt."""
from __future__ import annotations
import json
import os
import stat
from pathlib import Path

from terraform_plan_summary import COUNT_KEYS, normalize_action

FILE = 'runtime-plan-diagnostic.json'
MAX_BYTES = 2048
MAX_COUNT = 10000
STAGES = frozenset(('NOT_OBSERVED', 'INPUT', 'ADMISSION', 'HISTORY', 'PROVENANCE', 'FRESHNESS',
    'PACKET', 'PRIVATE_INPUT', 'RUNTIME_PACKET', 'RUNTIME_PLAN', 'RESOURCE_CHANGE', 'OUTPUT_CHANGE',
    'BASELINE', 'RUNTIME_VALUES', 'PLANNED_VALUES', 'DEPENDENCIES', 'COUNTS', 'APPROVAL', 'SERIALIZATION'))
FIELDS = frozenset(('NOT_OBSERVED', 'UNCLASSIFIED', 'CONTRACT', 'FORMAT', 'COMPLETE', 'EMPTY_LIST',
    'CHECKS', 'SHAPE', 'ADDRESS', 'MOVE', 'ACTION', 'UNKNOWN', 'VALUE', 'SENSITIVITY', 'REPLACE',
    'TYPE', 'IMPORT', 'GRAPH', 'PROVIDER', 'MODE', 'ALLOWED_FIELDS', 'TIMEOUTS', 'ETAG', 'SCHEMA_VERSION',
    'ID', 'NAME', 'PROJECT', 'WORKLOAD_IDENTITY_POOL_ID', 'STATE', 'DISPLAY_NAME', 'DESCRIPTION',
    'DISABLED', 'DELETION_POLICY', 'ATTESTATION_RULES', 'INLINE_CERTIFICATE_ISSUANCE_CONFIG',
    'INLINE_TRUST_CONFIG', 'WORKLOAD_IDENTITY_POOL_PROVIDER_ID', 'ATTRIBUTE_MAPPING', 'ATTRIBUTE_CONDITION',
    'AWS', 'OIDC', 'SAML', 'X509', 'SERVICE_ACCOUNT_ID', 'ROLE', 'MEMBER', 'CONDITION', 'SERVICE',
    'DISABLE_ON_DESTROY', 'DISABLE_DEPENDENT_SERVICES'))


# Keys come only from fixed validator source dictionaries/tuples. Arbitrary
# metadata/plan keys never become a public field code.
_BASELINE_SOURCE_FIELDS = {key: key.upper() for key in (
    'id', 'project', 'dataset_id', 'table_id', 'location', 'deletion_protection',
    'region', 'name', 'runtime', 'entry_point', 'available_memory_mb', 'timeout',
    'min_instances', 'max_instances', 'service_account_email', 'ingress_settings',
    'status', 'version_id', 'environment_variables', 'effective_labels', 'event_trigger',
    'description', 'labels', 'terraform_labels', 'source_archive_bucket', 'source_archive_object',
    'source_repository', 'https_trigger_url', 'trigger_http', 'vpc_connector',
    'vpc_connector_egress_settings', 'kms_key_name', 'docker_repository', 'build_worker_pool',
    'build_environment_variables', 'schedule', 'time_zone', 'friendly_name', 'resource_tags',
    'time_partitioning', 'range_partitioning', 'clustering', 'expiration_time', 'encryption_configuration',
    'external_data_configuration', 'view', 'materialized_view', 'snapshot_definition', 'clone_definition',
    'table_constraints', 'biglake_configuration', 'max_staleness', 'require_partition_filter')}
FIELDS = FIELDS | frozenset(_BASELINE_SOURCE_FIELDS.values()) | frozenset((
    'IDENTITY', 'PAUSED', 'DEADLINE', 'TARGET', 'RETRY_CONFIG', 'DATA', 'OUTPUTS', 'SCHEMA', 'COUNTS'))
STAGES = STAGES | frozenset(('HISTORY_PLAN', 'HISTORY_METADATA', 'HISTORY_INPUTS', 'HISTORY_COUNTS',
    'HISTORY_TABLE', 'HISTORY_SCHEMA', 'HISTORY_FORBIDDEN', 'BASELINE_FUNCTION',
    'BASELINE_SCHEDULER', 'BASELINE_TOPIC', 'BASELINE_OUTPUTS'))
POST_CHECKS = ('Persistent state invariant', 'Stable table metadata invariant',
    'Exact native lock absence', 'Workspace absence', 'Full-root no-op12 and job outcomes',
    'Fresh execution evidence')


def baseline_scope(env, *, operation, environment, phase):
    return (operation == 'plan' and environment == 'dev' and phase == 'post'
        and all(env.get(key) == expected for key, expected in {
            'MODE': 'plan', 'HISTORY_TARGET': 'dev', 'OWNERSHIP_WAVE': 'none',
            'HISTORY_POST_NOOP': 'true', 'CURRENT_STATE_ATTESTATION': 'true',
            'OWNERSHIP_ROOT_BASIS': 'original-history-import', 'GITHUB_RUN_ATTEMPT': '1'}.items()))


def source_field(key):
    return _BASELINE_SOURCE_FIELDS.get(key, 'VALUE')


def render_post_checks(checks):
    if type(checks) is not dict or set(checks) != set(POST_CHECKS) or not all(type(checks[key]) is bool for key in POST_CHECKS):
        raise ValueError('Fixed safety checks rejected')
    return ['Current-state safety: '+key+': '+('PASS' if checks[key] else 'STOP') for key in POST_CHECKS]

class Rejected(ValueError):
    def __init__(self, stage, field):
        super().__init__('Runtime plan contract rejected')
        self.stage, self.field = stage, field

class Context:
    def __init__(self):
        self.stage = 'INPUT'
        self.field = None
    def mark(self, stage, field=None):
        if stage not in STAGES:
            raise ValueError('Diagnostic stage rejected')
        if field is not None and field not in FIELDS:
            raise ValueError('Diagnostic field rejected')
        self.stage, self.field = stage, field
    def note_failure(self, field):
        if field not in FIELDS:
            raise ValueError('Diagnostic field rejected')
        self.field = field
    def require(self, condition, field='CONTRACT'):
        if field not in FIELDS:
            raise ValueError('Diagnostic field rejected')
        if not condition:
            raise Rejected(self.stage, field)


def counts(plan):
    # Missing, partial or overlarge projections stay unobserved; no addresses
    # or values enter the diagnostic. These counts never validate a plan.
    if (type(plan) is not dict or plan.get('complete') is not True or plan.get('errored', False) is not False
            or type(plan.get('resource_changes')) is not list):
        return None
    for key in ('deferred_changes', 'action_invocations', 'deferred_action_invocations'):
        if type(plan.get(key, [])) is not list or plan.get(key, []) != []:
            return None
    changes, drift = plan['resource_changes'], plan.get('resource_drift', [])
    if len(changes) > MAX_COUNT or type(drift) is not list or len(drift) > MAX_COUNT:
        return None
    result = dict.fromkeys(COUNT_KEYS, 0)
    for resource in changes:
        if type(resource) is not dict or type(resource.get('change')) is not dict:
            return None
        change = resource['change']
        if type(change.get('actions')) is not list or not all(type(x) is str for x in change['actions']):
            return None
        if 'importing' in change and type(change['importing']) is not dict:
            return None
        result[normalize_action(change['actions'])] += 1
        result['import'] += int('importing' in change)
    if not all(type(item) is dict for item in drift):
        return None
    result['drift'] = len(drift)
    return result


def validate(value):
    if type(value) is not dict or set(value) != {'schema_version', 'outcome', 'stage', 'field', 'counts'}:
        raise ValueError('Diagnostic shape rejected')
    if type(value['schema_version']) is not int or value['schema_version'] != 1 or value['outcome'] != 'STOP':
        raise ValueError('Diagnostic outcome rejected')
    if type(value['stage']) is not str or value['stage'] not in STAGES or type(value['field']) is not str or value['field'] not in FIELDS:
        raise ValueError('Diagnostic enum rejected')
    observed = value['counts']
    if observed is not None and (type(observed) is not dict or set(observed) != set(COUNT_KEYS)
            or not all(type(n) is int and 0 <= n <= MAX_COUNT for n in observed.values())):
        raise ValueError('Diagnostic counts rejected')
    return value


def record(directory, plan, context, error):
    value = validate(dict(schema_version=1, outcome='STOP', stage=context.stage,
        field=error.field if type(error) is Rejected else (context.field or 'UNCLASSIFIED'), counts=counts(plan)))
    raw = (json.dumps(value, sort_keys=True, separators=(',', ':'))+'\n').encode()
    if len(raw) > MAX_BYTES:
        raise ValueError('Diagnostic bound rejected')
    fd = os.open(Path(directory)/FILE, os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(raw)


def unique(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError('Duplicate diagnostic key')
        result[key] = value
    return result


def read(directory):
    fd = os.open(Path(directory)/FILE, os.O_RDONLY|os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600 or info.st_nlink != 1 or not 0 < info.st_size <= MAX_BYTES:
            raise ValueError('Diagnostic file rejected')
        raw = stream.read(MAX_BYTES+1)
    if len(raw) > MAX_BYTES:
        raise ValueError('Diagnostic bound rejected')
    return validate(json.loads(raw, object_pairs_hook=unique, parse_constant=lambda _: (_ for _ in ()).throw(ValueError('Diagnostic number rejected'))))


def render(value):
    value = validate(value)
    suffix = ('counts=NOT_OBSERVED' if value['counts'] is None else
        'counts=' + ','.join(key+':'+str(value['counts'][key]) for key in COUNT_KEYS))
    return 'Runtime plan diagnostic: outcome=STOP stage='+value['stage']+' field='+value['field']+' '+suffix


def main():
    try:
        value = read(os.environ['RUNNER_TEMP'])
        line = render(value)
    except Exception:
        print('Runtime plan diagnostic: outcome=STOP stage=NOT_OBSERVED field=NOT_OBSERVED counts=NOT_OBSERVED')
        return 3
    print(line)
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
