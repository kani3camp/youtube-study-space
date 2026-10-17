"""Pure, version-pinned check-set joins for the separate observation-sync mode.

Official Terraform v1.16.4: planproto/planfile.proto fields 17/19,
command/jsonchecks/{checks,objects}.go, statefile/version4.go:574-599, and
terraform/context_apply.go:269-271. Checks are unordered configuration/object
maps. Failure-message order and every other State representation remain strict.
This module performs no IO and never grants execution/readiness authority.
"""
from __future__ import annotations

from dataclasses import dataclass
import copy
from functools import wraps
import json
import re

MAX_BYTES = 4 * 1024 * 1024
MAX_CHECKS = 512
MAX_FIELDS = 8192
CORE_PROTO_SHA256 = '7908c61954fa7f5be14dfb797860b4e8d32891c88fa8ae21ac66aa75e64b2937'
CORE_OBJECTS_SHA256 = 'a058bf922e463f75e055ec78ab0d48185446938490da1cd0cf51d8a62c0406f0'
CORE_CHECKS_SHA256 = 'e1ed098f6d8830aa69d43ec497d097a178bd3ab0eec7253cc32070deaffd3a59'
_IDENTIFIER = re.compile(r'[A-Za-z_][A-Za-z0-9_-]*\Z')
_MODULE_STEP = re.compile(r'module\.([A-Za-z_][A-Za-z0-9_-]*)')
_KINDS = {1: 'resource', 2: 'output', 3: 'check', 4: 'var'}
_STATUS = {0: 'unknown', 1: 'pass', 2: 'fail', 3: 'error'}
_PLAN = {1: (0, False), 2: (2, True), 3: (2, True), 4: (2, True),
         5: (2, True), 13: (2, False), 14: (2, False), 15: (2, True),
         16: (2, True), 17: (0, False), 18: (2, True), 19: (2, True),
         20: (0, False), 21: (2, False), 22: (2, True), 25: (0, False),
         26: (0, False), 27: (2, True), 28: (2, True), 29: (2, False),
         30: (2, True), 31: (2, True), 32: (2, True)}
_AGGREGATE = {1: (0, False), 2: (2, False), 3: (0, False), 4: (2, True)}
_OBJECT = {1: (2, False), 2: (0, False), 3: (2, True)}


class ChecksRejected(ValueError):
    def __init__(self):
        super().__init__('History observation checks STOP')


def _require(condition):
    if not condition:
        raise ChecksRejected()


def _safe(function):
    @wraps(function)
    def wrapped(*args, **kwargs):
        try:
            return function(*args, **kwargs)
        except ChecksRejected:
            raise
        except (ValueError, TypeError, KeyError, IndexError, OverflowError, RecursionError):
            raise ChecksRejected() from None
    return wrapped


@dataclass(frozen=True)
class CheckCatalog:
    """Caller supplies the exact reviewed source closure, never an input policy.

    configs/zero_instances contain (raw-kind, static configuration address).
    module_instances includes '' and exact source-owned module instance paths.
    resource_instances contains exact managed resource instance addresses.
    The controller must bind these to its immutable HCL/source graph and first
    run the existing receipt checks/state validator. No catalog is inferred from
    a response or accepted by a default/fallback path in this module.
    """
    configs: frozenset
    zero_instances: frozenset
    module_instances: frozenset
    resource_instances: frozenset


def _string(value, limit=2048):
    _require(type(value) is str and 0 < len(value) <= limit
             and not any(ord(c) < 32 or ord(c) == 127 for c in value))
    return value


def _index(raw):
    value = json.loads(raw)
    _require((type(value) is int and 0 <= value < (1 << 63))
             or (type(value) is str and len(value) <= 256
                 and not any(ord(c) < 32 or 0xD800 <= ord(c) <= 0xDFFF for c in value)))
    _require(json.dumps(value, separators=(',', ':'), ensure_ascii=False) == raw)
    return value


def _module_static(module):
    if module == '':
        return ''
    _string(module)
    pos, names = 0, []
    while pos < len(module):
        match = _MODULE_STEP.match(module, pos)
        _require(match is not None and len(names) < 16)
        names.append('module.' + match.group(1))
        pos = match.end()
        if module[pos:pos + 1] == '[':
            _, consumed = json.JSONDecoder().raw_decode(module[pos + 1:])
            _require(module[pos + 1 + consumed:pos + 2 + consumed] == ']')
            _index(module[pos + 1:pos + 1 + consumed])
            pos += consumed + 2
        if pos < len(module):
            _require(module[pos:pos + 1] == '.')
            pos += 1
            _require(pos < len(module))
    return '.'.join(names)


def _static(kind, address):
    _require(kind in _KINDS.values())
    _string(address)
    parts = address.split('.')
    _require(len(parts) >= 2)
    first, name = parts[-2:]
    module = '.'.join(parts[:-2])
    _require(_IDENTIFIER.fullmatch(name) is not None and _module_static(module) == module)
    result = {'to_display': address, 'kind': 'output_value' if kind == 'output' else kind, 'name': name}
    if module:
        result['module'] = module
    if kind == 'resource':
        _require(_IDENTIFIER.fullmatch(first) is not None)
        result.update(mode='managed', type=first)
    else:
        _require(first == kind)
    return result, module, first, name


def _catalog(catalog):
    _require(type(catalog) is CheckCatalog)
    for value in (catalog.configs, catalog.zero_instances, catalog.module_instances, catalog.resource_instances):
        _require(type(value) is frozenset and len(value) <= MAX_CHECKS)
    _require('' in catalog.module_instances and catalog.zero_instances <= catalog.configs)
    _require(all(type(instance) is str for instance in catalog.resource_instances))
    modules = {module: _module_static(module) for module in catalog.module_instances}
    result, matched_resources = {}, set()
    for key in catalog.configs:
        _require(type(key) is tuple and len(key) == 2 and all(type(x) is str for x in key))
        kind, address = key
        static, module, first, name = _static(kind, address)
        allowed = {}
        for instance_module, config_module in modules.items():
            if module != config_module:
                continue
            base = (instance_module + '.' if instance_module else '') + first + '.' + name
            if kind != 'resource':
                value = {'to_display': base}
                if instance_module:
                    value['module'] = instance_module
                allowed[base] = value
                continue
            for instance in catalog.resource_instances:
                _string(instance)
                if instance == base:
                    value = {'to_display': instance}
                elif instance.startswith(base + '[') and instance.endswith(']'):
                    value = {'to_display': instance, 'instance_key': _index(instance[len(base) + 1:-1])}
                else:
                    continue
                if instance_module:
                    value['module'] = instance_module
                allowed[instance] = value
                matched_resources.add(instance)
        _require(bool(allowed) or key in catalog.zero_instances)
        result[key] = (static, allowed)
    _require(matched_resources == set(catalog.resource_instances))
    return result


def _bounded(value):
    remaining = [65536]
    def visit(item, depth):
        remaining[0] -= 1
        _require(depth <= 32 and remaining[0] >= 0)
        if type(item) is dict:
            _require(all(type(k) is str for k in item))
            for key, child in item.items():
                _require(len(key) <= 2048)
                visit(child, depth + 1)
        elif type(item) is list:
            for child in item:
                visit(child, depth + 1)
        else:
            _require(item is None or type(item) in (str, int, bool))
            if type(item) is str:
                _require(len(item) <= MAX_BYTES)
            if type(item) is int:
                _require(-(1 << 63) <= item < (1 << 63))
    visit(value, 0)
    _require(len(json.dumps(value, separators=(',', ':'), ensure_ascii=False).encode()) <= MAX_BYTES)


def _map_valid(result, catalog):
    definitions = _catalog(catalog)
    _require(type(result) is dict and len(result) <= MAX_CHECKS)
    for key, value in result.items():
        _require(key in definitions and type(value) is tuple and len(value) == 2)
        status, objects = value
        _require(type(status) is str and status == 'pass' and type(objects) is dict and len(objects) <= MAX_CHECKS)
        _require(bool(objects) or key in catalog.zero_instances)
        allowed = definitions[key][1]
        for address, observed in objects.items():
            _require(type(address) is str and address in allowed and type(observed) is tuple
                     and len(observed) == 2 and observed[0] == 'pass' and type(observed[0]) is str
                     and type(observed[1]) is tuple and observed[1] == ())
    return result


def _varint(raw, pos):
    value = 0
    for index in range(10):
        _require(pos < len(raw))
        byte = raw[pos]
        pos += 1
        _require(index < 9 or byte <= 1)
        value |= (byte & 127) << (7 * index)
        if byte < 128:
            _require(index == 0 or byte != 0)
            return value, pos
    raise ChecksRejected()


def _fields(raw, schema):
    _require(type(raw) is bytes and len(raw) <= MAX_BYTES)
    result, pos, count = {}, 0, 0
    while pos < len(raw):
        count += 1
        _require(count <= MAX_FIELDS)
        tag, pos = _varint(raw, pos)
        number, wire = tag >> 3, tag & 7
        _require(number in schema and wire == schema[number][0]
                 and (number not in result or schema[number][1]))
        if wire == 0:
            value, pos = _varint(raw, pos)
        else:
            length, pos = _varint(raw, pos)
            _require(length <= len(raw) - pos)
            value = raw[pos:pos + length]
            pos += length
        result.setdefault(number, []).append(value)
    return result


def _one(message, number, default=None):
    return message[number][0] if number in message else default


def _enum(value, allowed):
    _require(type(value) is int and value in allowed)
    return value


def _text(raw):
    _require(type(raw) is bytes and len(raw) <= 2048)
    return _string(raw.decode('utf-8', errors='strict'))


@_safe
def decode_plan(raw, expected_mode=2, *, catalog):
    """Decode only the pinned root metadata and complete field-19 check schema.

    Other known nested Plan messages are not interpreted here; the separate
    full graph/action/state validator remains mandatory. No future fields or
    alternate Terraform/serialized-plan versions are silently accepted.
    """
    _enum(expected_mode, {0, 2})
    plan = _fields(raw, _PLAN)
    _require(_one(plan, 1) == 3 and _one(plan, 14) == b'1.16.4')
    mode = _enum(_one(plan, 17, 0), {0, 1, 2})
    _require(mode == expected_mode)
    flags = {field: _enum(_one(plan, field, 0), {0, 1}) for field in (20, 25, 26)}
    _require(flags[20] == 0 and flags[25] == (1 if mode == 2 else 0) and flags[26] == 1)
    groups = plan.get(19, [])
    _require(len(groups) <= MAX_CHECKS)
    result = {}
    for encoded in groups:
        group = _fields(encoded, _AGGREGATE)
        kind = _KINDS[_enum(_one(group, 1, 0), _KINDS)]
        address = _text(_one(group, 2, b''))
        status = _STATUS[_enum(_one(group, 3, 0), _STATUS)]
        objects = {}
        _require(len(group.get(4, [])) <= MAX_CHECKS)
        for encoded_object in group.get(4, []):
            obj = _fields(encoded_object, _OBJECT)
            member = _text(_one(obj, 1, b''))
            obj_status = _STATUS[_enum(_one(obj, 2, 0), _STATUS)]
            messages = tuple(_text(message) for message in obj.get(3, []))
            _require(member not in objects)
            objects[member] = (obj_status, messages)
        key = (kind, address)
        _require(key not in result)
        result[key] = (status, objects)
    _map_valid(result, catalog)
    return result, {'format_version': 3, 'mode': mode, 'check_count': len(result),
                    'resource_change_count': len(plan.get(3, [])), 'drift_count': len(plan.get(18, []))}


@_safe
def raw_checks(checks, *, catalog):
    """Validate all passing raw State checks; source-owned empty instances only."""
    _bounded(checks)
    if checks is None:
        return _map_valid({}, catalog)
    _require(type(checks) is list and len(checks) <= MAX_CHECKS)
    result = {}
    for group in checks:
        _require(type(group) is dict and set(group) == {'object_kind', 'config_addr', 'status', 'objects'})
        kind, address, status = (group[name] for name in ('object_kind', 'config_addr', 'status'))
        _require(type(kind) is str and type(address) is str and type(status) is str)
        key = (kind, address)
        _require(key not in result)
        members = group['objects']
        _require(members is None or type(members) is list)
        objects = {}
        for obj in members or []:
            _require(type(obj) is dict and {'object_addr', 'status'} <= set(obj)
                     <= {'object_addr', 'status', 'failure_messages'})
            member, obj_status = obj['object_addr'], obj['status']
            messages = obj.get('failure_messages', [])
            _require(type(member) is str and member not in objects and type(obj_status) is str
                     and type(messages) is list and all(type(message) is str for message in messages))
            objects[member] = (obj_status, tuple(messages))
        result[key] = (status, objects)
    return _map_valid(result, catalog)


@_safe
def shown_checks(checks, expected_raw_map, *, catalog):
    """Official show address fields; empty instances must be omitted, not null."""
    _bounded(checks)
    _map_valid(expected_raw_map, catalog)
    definitions = _catalog(catalog)
    _require(type(checks) is list and len(checks) <= MAX_CHECKS)
    result = {}
    for group in checks:
        _require(type(group) is dict and {'address', 'status'} <= set(group)
                 <= {'address', 'status', 'instances'})
        address = group['address']
        _require(type(address) is dict and type(address.get('kind')) is str
                 and type(address.get('to_display')) is str)
        kind = 'output' if address['kind'] == 'output_value' else address['kind']
        key = (kind, address['to_display'])
        _require(key in definitions and address == definitions[key][0] and key not in result)
        status = group['status']
        _require(type(status) is str)
        objects = {}
        if 'instances' in group:
            members = group['instances']
            _require(type(members) is list and 0 < len(members) <= MAX_CHECKS)
            for obj in members:
                _require(type(obj) is dict and set(obj) == {'address', 'status'})
                dynamic = obj['address']
                _require(type(dynamic) is dict and type(dynamic.get('to_display')) is str)
                member = dynamic['to_display']
                _require(member in definitions[key][1] and member not in objects
                         and dynamic == definitions[key][1][member] and type(obj['status']) is str)
                if 'instance_key' in dynamic:
                    _require(type(dynamic['instance_key']) in (int, str))
                objects[member] = (obj['status'], ())
        result[key] = (status, objects)
    _map_valid(result, catalog)
    _require(result == expected_raw_map)
    return result


@_safe
def canonical_checks(checks, *, catalog):
    """Sort only config/object check order; retain optional fields and null/[] ."""
    raw_checks(checks, catalog=catalog)
    result = copy.deepcopy(checks)
    if result is None:
        return None
    result.sort(key=lambda group: (group['object_kind'], group['config_addr']))
    for group in result:
        if type(group['objects']) is list:
            group['objects'].sort(key=lambda obj: obj['object_addr'])
    return result


@_safe
def verify_checks(proto_bytes, show_checks, raw_before_checks, *, catalog, expected_mode=2):
    """Exact three-way typed-set equality; no missing/new checks or statuses."""
    expected = raw_checks(raw_before_checks, catalog=catalog)
    observed, _ = decode_plan(proto_bytes, expected_mode=expected_mode, catalog=catalog)
    _require(observed == expected)
    shown_checks(show_checks, expected, catalog=catalog)
    return observed


@_safe
def planned_checks(raw_before_checks, typed_map, *, catalog):
    """Core-writer representation for a validation-only copy, never apply input.

    Core restores Plan.Checks and omits empty failure_messages, serializes an
    empty objects set as null, and an empty whole set as null. Representation
    differences from raw-before (for example [] instead of null) remain visible
    to the caller's complete State comparison and must not be normalized away.
    """
    _require(raw_checks(raw_before_checks, catalog=catalog) == _map_valid(typed_map, catalog))
    if not typed_map:
        return None
    result = []
    for kind, address in sorted(typed_map):
        status, objects = typed_map[(kind, address)]
        serialized = [{'object_addr': member, 'status': objects[member][0]}
                      for member in sorted(objects)] or None
        result.append({'object_kind': kind, 'config_addr': address, 'status': status, 'objects': serialized})
    return result
