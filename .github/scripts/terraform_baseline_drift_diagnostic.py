#!/usr/bin/env python3
"""Failure-only baseline drift projection. No value or dynamic path is output."""
from __future__ import annotations

import json
import math
import os
from pathlib import Path
import stat

import terraform_plan_diagnostic as legacy
from terraform_plan_summary import KNOWN_ACTIONS
from terraform_baseline_drift_fields import ALIASES, FIELDS, MAP_VERSION, PROVIDER

KINDS = ('PRESENCE', 'NULL', 'VALUE', 'UNKNOWN', 'SENSITIVITY', 'REPLACE')
MAX_TARGETS = 12
MAX_DEPTH = 32
MAX_NODES = 16384
MAX_TEXT_BYTES = 4 * 1024 * 1024
_TARGETS = {address: (index, resource_type) for index, (_, address, resource_type) in enumerate(ALIASES)}
_RESOURCE_KEYS = frozenset(('address', 'module_address', 'mode', 'type', 'name', 'index', 'provider_name', 'change'))
_CHANGE_KEYS = frozenset(('actions', 'before', 'after', 'after_unknown', 'before_sensitive', 'after_sensitive', 'replace_paths'))
_MISSING = object()


def require_scope(env, *, operation, environment, phase):
    if not legacy.baseline_scope(env, operation=operation, environment=environment, phase=phase):
        raise ValueError('Baseline diagnostic scope rejected')


def _json(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False)


def _bounded_json(value, budget, depth=0):
    budget[0] -= 1
    if depth > MAX_DEPTH or budget[0] < 0:
        raise ValueError('Diagnostic JSON bound rejected')
    if type(value) is str:
        budget[1] -= len(value.encode('utf-8'))
        if budget[1] < 0:
            raise ValueError('Diagnostic JSON bound rejected')
    elif type(value) is dict:
        for key, item in value.items():
            if type(key) is not str:
                raise ValueError('Diagnostic JSON key rejected')
            _bounded_json(key, budget, depth + 1)
            _bounded_json(item, budget, depth + 1)
    elif type(value) is list:
        for item in value:
            _bounded_json(item, budget, depth + 1)
    elif type(value) is float:
        if not math.isfinite(value):
            raise ValueError('Diagnostic JSON number rejected')
    elif value is not None and type(value) not in (bool, int):
        raise ValueError('Diagnostic JSON type rejected')


def _mask_true(value):
    if type(value) is bool:
        return value
    if type(value) is dict:
        return any([_mask_true(item) for item in value.values()])
    if type(value) is list:
        return any([_mask_true(item) for item in value])
    raise ValueError('Diagnostic mask rejected')


def _top_mask(value, fields):
    if value is False:
        return {}
    # Whole-object true does not identify which optional attributes exist.
    if value is True:
        raise ValueError('Diagnostic whole mask rejected')
    if type(value) is not dict or not set(value) <= set(fields):
        raise ValueError('Diagnostic mask rejected')
    # Validate every nested leaf, even when an earlier leaf was true.
    for item in value.values():
        _mask_true(item)
    return value


def _identity(item):
    if type(item) is not dict or not set(item) <= _RESOURCE_KEYS:
        raise ValueError('Diagnostic target rejected')
    index, resource_type = _TARGETS[item['address']]
    if item.get('mode') != 'managed' or item.get('type') != resource_type or item.get('provider_name') != PROVIDER:
        raise ValueError('Diagnostic target rejected')
    address = ALIASES[index][1]
    tail = address.rsplit('.', 1)[1]
    if 'name' in item and item['name'] != tail.split('[', 1)[0]:
        raise ValueError('Diagnostic target rejected')
    module = address.rsplit('.', 2)[0] if address.startswith('module.') else _MISSING
    if 'module_address' in item and (module is _MISSING or item['module_address'] != module):
        raise ValueError('Diagnostic target rejected')
    expected_index = json.loads(tail.split('[', 1)[1][:-1]) if '[' in tail else _MISSING
    if 'index' in item and (expected_index is _MISSING or type(item['index']) is not type(expected_index) or item['index'] != expected_index):
        raise ValueError('Diagnostic target rejected')
    return index, FIELDS[resource_type]


def _entry(item, budget):
    index, fields = _identity(item)
    change = item['change']
    if type(change) is not dict or not set(change) <= _CHANGE_KEYS:
        raise ValueError('Diagnostic change rejected')
    _bounded_json(change, budget)
    actions = change['actions']
    if type(actions) is not list or not all(type(action) is str for action in actions) or tuple(actions) not in KNOWN_ACTIONS:
        raise ValueError('Diagnostic action rejected')
    before, after = change['before'], change['after']
    if not all(value is None or type(value) is dict for value in (before, after)):
        raise ValueError('Diagnostic object rejected')
    sides = [value if value is not None else {} for value in (before, after)]
    if not all(set(value) <= set(fields) for value in sides):
        raise ValueError('Diagnostic field rejected')
    unknown = _top_mask(change.get('after_unknown', {}), fields)
    sensitive = [_top_mask(change.get(key, {}), fields) for key in ('before_sensitive', 'after_sensitive')]
    paths = change.get('replace_paths', [])
    if type(paths) is not list or len(paths) > legacy.MAX_COUNT:
        raise ValueError('Diagnostic replacement rejected')
    replaced = set()
    for path in paths:
        if type(path) is not list or not all(type(part) in (str, int) for part in path):
            raise ValueError('Diagnostic replacement rejected')
        if path and type(path[0]) is str and path[0] in fields:
            replaced.add(path[0])
        else:
            raise ValueError('Diagnostic replacement rejected')
    bits = [0] * len(KINDS)
    for position, field in enumerate(fields):
        b, a = (value.get(field, _MISSING) for value in sides)
        present = (b is not _MISSING, a is not _MISSING)
        unknown_field = _mask_true(unknown.get(field, False))
        # Terraform may omit an unknown value or serialize its placeholder as
        # null. Neither proves a known presence/null/value change at this field.
        changed = not unknown_field and (present[0] != present[1] or (all(present) and _json(b) != _json(a)))
        flags = (not unknown_field and present[0] != present[1], changed and (b is None or a is None), changed,
            unknown_field,
            _json(sensitive[0].get(field, False)) != _json(sensitive[1].get(field, False)), field in replaced)
        for kind, observed in enumerate(flags):
            if observed:
                bits[kind] |= 1 << position
    # Object presence is finite, separate from top-level field presence.
    objects = int(before is not None) + 2 * int(after is not None)
    return [index, KNOWN_ACTIONS[tuple(actions)], objects, *[format(bits, 'x') for bits in bits]]


def project(plan):
    """All targets or none; diagnostic rejection never changes plan admission."""
    try:
        if (type(plan) is not dict or plan.get('complete') is not True or plan.get('errored') is not False
                or legacy.counts(plan) is None):
            return None
        drift = plan.get('resource_drift', [])
        if type(drift) is not list or len(drift) > MAX_TARGETS:
            return None
        changes = plan['resource_changes']
        if len(changes) != MAX_TARGETS:
            return None
        graph = [_identity(item)[0] for item in changes]
        if len(set(graph)) != MAX_TARGETS or set(graph) != set(range(MAX_TARGETS)):
            return None
        budget = [MAX_NODES, MAX_TEXT_BYTES]
        entries = [_entry(item, budget) for item in drift]
        if len({entry[0] for entry in entries}) != len(entries):
            return None
        value = {'map_version': MAP_VERSION, 'items': sorted(entries)}
        validate_projection(value)
        return value
    except Exception:
        return None


def validate_projection(value):
    if value is None:
        return
    if type(value) is not dict or set(value) != {'map_version', 'items'} or value['map_version'] != MAP_VERSION:
        raise ValueError('Diagnostic map rejected')
    items = value['items']
    if type(items) is not list or len(items) > MAX_TARGETS:
        raise ValueError('Diagnostic targets rejected')
    previous = -1
    for item in items:
        if (type(item) is not list or len(item) != 3 + len(KINDS) or type(item[0]) is not int
                or not previous < item[0] < MAX_TARGETS or type(item[1]) is not str
                or item[1] not in KNOWN_ACTIONS.values() or type(item[2]) is not int or not 0 <= item[2] <= 3):
            raise ValueError('Diagnostic entry rejected')
        previous = item[0]
        width = len(FIELDS[ALIASES[item[0]][2]])
        for bits in item[3:]:
            if type(bits) is not str or not 1 <= len(bits) <= (width + 3) // 4 or any(c not in '0123456789abcdef' for c in bits):
                raise ValueError('Diagnostic bits rejected')
            number = int(bits, 16)
            if number >= 1 << width or format(number, 'x') != bits:
                raise ValueError('Diagnostic bits rejected')


def validate(value):
    if type(value) is not dict or set(value) != {'schema_version', 'outcome', 'stage', 'field', 'counts', 'drift'} or type(value['schema_version']) is not int or value['schema_version'] != 2:
        raise ValueError('Baseline diagnostic rejected')
    legacy.validate(dict(schema_version=1, **{key: value[key] for key in ('outcome', 'stage', 'field', 'counts')}))
    validate_projection(value['drift'])
    if value['drift'] is not None and (value['counts'] is None or value['counts']['drift'] != len(value['drift']['items'])):
        raise ValueError('Baseline diagnostic count rejected')
    return value


def record(directory, plan, context, error, *, env, operation, environment, phase):
    require_scope(env, operation=operation, environment=environment, phase=phase)
    value = validate(dict(schema_version=2, outcome='STOP', stage=context.stage,
        field=error.field if type(error) is legacy.Rejected else (context.field or 'UNCLASSIFIED'),
        counts=legacy.counts(plan), drift=project(plan)))
    raw = (_json(value) + '\n').encode()
    if len(raw) > legacy.MAX_BYTES:
        value['drift'] = None
        raw = (_json(value) + '\n').encode()
    if len(raw) > legacy.MAX_BYTES:
        raise ValueError('Baseline diagnostic bound rejected')
    fd = os.open(Path(directory) / legacy.FILE, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(raw)


def read(directory, *, env, operation, environment, phase):
    require_scope(env, operation=operation, environment=environment, phase=phase)
    fd = os.open(Path(directory) / legacy.FILE, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.getuid() or stat.S_IMODE(info.st_mode) != 0o600 or info.st_nlink != 1 or not 0 < info.st_size <= legacy.MAX_BYTES:
            raise ValueError('Baseline diagnostic file rejected')
        raw = stream.read(legacy.MAX_BYTES + 1)
    if len(raw) > legacy.MAX_BYTES:
        raise ValueError('Baseline diagnostic bound rejected')
    value = json.loads(raw, object_pairs_hook=legacy.unique,
        parse_constant=lambda _: (_ for _ in ()).throw(ValueError('Diagnostic number rejected')))
    if type(value) is dict and type(value.get('schema_version')) is int and value['schema_version'] == 1:
        return legacy.validate(value)
    return validate(value)


def render(value, *, env, operation, environment, phase):
    require_scope(env, operation=operation, environment=environment, phase=phase)
    if type(value) is dict and type(value.get('schema_version')) is int and value['schema_version'] == 1:
        return legacy.render(value)
    value = validate(value)
    lines = [legacy.render(dict(schema_version=1, **{key: value[key] for key in ('outcome', 'stage', 'field', 'counts')}))]
    if value['drift'] is None:
        lines.append('Baseline drift diagnostic: NOT_OBSERVED')
    else:
        lines.append('Baseline drift diagnostic: OBSERVED targets=' + str(len(value['drift']['items'])))
        for item in value['drift']['items']:
            alias, _, resource_type = ALIASES[item[0]]
            fields = FIELDS[resource_type]
            parts = []
            for kind, bits in zip(KINDS, item[3:]):
                codes = [field.upper() for position, field in enumerate(fields) if int(bits, 16) & 1 << position]
                parts.append(kind + '=' + (','.join(codes) if codes else 'NONE'))
            lines.append('Baseline drift target: role=' + alias + ' action=' + item[1]
                + ' object_presence=' + ('NEITHER', 'BEFORE', 'AFTER', 'BOTH')[item[2]] + ' ' + ' '.join(parts))
    return '\n'.join(lines)


def main():
    env = dict(os.environ)
    scope = dict(env=env, operation=env.get('MODE'), environment=env.get('HISTORY_TARGET'), phase='post')
    try:
        require_scope(**scope)
    except Exception:
        return 3  # No new public output for any other mode.
    try:
        value = read(env['RUNNER_TEMP'], **scope)
        print(render(value, **scope))
        return 0
    except Exception:
        print('Runtime plan diagnostic: outcome=STOP stage=NOT_OBSERVED field=NOT_OBSERVED counts=NOT_OBSERVED')
        print('Baseline drift diagnostic: NOT_OBSERVED')
        return 3


if __name__ == '__main__':
    raise SystemExit(main())
