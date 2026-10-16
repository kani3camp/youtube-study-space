"""Bounded sealed-plan inspection; verification objects never reach Terraform.

TF1.16.4 Core writes a stub header into tfstate-prev and replaces check_results
from Plan.Checks during apply. Only those documented transformations are
interpreted here. Every other JSON element and source snapshot byte is pinned.
"""
from __future__ import annotations
import copy
import io
from pathlib import PurePosixPath
import stat
import zipfile

import validate_history_observation_sync as gate
import history_observation_sync_checks as checks

MAX_ARCHIVE_BYTES = 8 * 1024 * 1024
MAX_EXPANDED_BYTES = 16 * 1024 * 1024
MAX_MEMBERS = 128


def read_archive(raw):
    gate.require(type(raw) is bytes and 0 < len(raw) <= MAX_ARCHIVE_BYTES
        and raw.startswith(b'PK\x03\x04'), 'ARCHIVE')
    result = {}; total = 0
    with zipfile.ZipFile(io.BytesIO(raw)) as archive:
        entries = archive.infolist()
        gate.require(3 <= len(entries) <= MAX_MEMBERS, 'ARCHIVE')
        for entry in entries:
            name = entry.filename; path = PurePosixPath(name)
            mode = (entry.external_attr >> 16) & 0xffff
            gate.require(type(name) is str and 0 < len(name) <= 256 and name not in result
                and '\\' not in name and '\x00' not in name and not path.is_absolute()
                and str(path) == name and all(part not in ('.','..') for part in path.parts)
                and not entry.is_dir() and stat.S_IFMT(mode) in (0,stat.S_IFREG)
                and not entry.flag_bits & 1 and entry.compress_type in (0,8), 'ARCHIVE')
            gate.require(name in ('tfplan','tfstate','tfstate-prev','.terraform.lock.hcl')
                or name.startswith('tfconfig/'), 'ARCHIVE')
            total += entry.file_size
            gate.require(0 <= entry.file_size <= gate.MAX_BYTES
                and entry.file_size <= max(entry.compress_size,1)*256
                and total <= MAX_EXPANDED_BYTES, 'ARCHIVE')
            with archive.open(entry) as stream: value = stream.read(gate.MAX_BYTES+1)
            gate.require(len(value) == entry.file_size, 'ARCHIVE')
            result[name] = value
        gate.require({'tfplan','tfstate','tfstate-prev','.terraform.lock.hcl','tfconfig/modules.json'} <= set(result), 'ARCHIVE')
        gate.require(archive.testzip() is None, 'ARCHIVE')
    return result


def json_value(raw):
    # Raw JSON parsing shares the duplicate/type/depth/node/finite budgets.
    value = gate.json.loads(raw, object_pairs_hook=gate._unique,
        parse_constant=lambda _: (_ for _ in ()).throw(gate.SyncRejected('SHAPE')))
    gate._bounded(value)
    return value


def normalized_state(state, *, catalog):
    result = copy.deepcopy(state)
    result['check_results'] = checks.canonical_checks(result['check_results'],catalog=catalog)
    return result


def _instances(state):
    found = {}
    for resource in state['resources']:
        base = (resource['module']+'.' if 'module' in resource else '') + resource['type']+'.'+resource['name']
        for instance in resource['instances']:
            address = base + ('['+gate.canonical(instance['index_key'])+']' if 'index_key' in instance else '')
            gate.require(address not in found, 'ARCHIVE'); found[address] = instance
    return found


def mask_paths(mask, value, path=()):
    if mask is True: return {path}
    if mask is False: return set()
    result=set()
    if type(mask) is dict:
        gate.require(type(value) is dict and set(mask)<=set(value), 'ARCHIVE')
        for key,item in mask.items(): result |= mask_paths(item,value[key],path+(key,))
    elif type(mask) is list:
        gate.require(type(value) is list and len(mask)<=len(value), 'ARCHIVE')
        for index,item in enumerate(mask):result |= mask_paths(item,value[index],path+(index,))
    else: raise gate.SyncRejected('ARCHIVE')
    return result


def sensitive_paths(items,attributes):
    # Core statefile/version4.go marshalPaths and go-cty v1.18.1
    # json/marshal.go marshalDynamic: index value is {type,value}.
    gate.require(type(items)is list and len(items)<=4096,'ARCHIVE')
    result=set()
    for item in items:
        gate.require(type(item)is list and 0<len(item)<=32,'ARCHIVE')
        path=[];value=attributes
        for step in item:
            gate.require(type(step)is dict and set(step)=={'type','value'},'ARCHIVE')
            if step['type']=='get_attr':
                key=step['value'];gate.require(type(key)is str and type(value)is dict and key in value,'ARCHIVE')
            elif step['type']=='index':
                dynamic=step['value']
                gate.require(type(dynamic)is dict and set(dynamic)=={'type','value'},'ARCHIVE')
                key=dynamic['value']
                if dynamic['type']=='string':gate.require(type(key)is str and type(value)is dict and key in value,'ARCHIVE')
                elif dynamic['type']=='number':gate.require(type(key)is int and type(value)is list and 0<=key<len(value),'ARCHIVE')
                else:raise gate.SyncRejected('ARCHIVE')
            else:raise gate.SyncRejected('ARCHIVE')
            path.append(key);value=value[key]
        path=tuple(path);gate.require(path not in result,'ARCHIVE');result.add(path)
    return result


def _prior_join(refresh, candidate):
    found = {}; gate._module_resources(refresh['prior_state']['values']['root_module'],found,root=True)
    instances = _instances(candidate)
    gate.require(set(instances) == set(found), 'ARCHIVE')
    for address, resource in found.items():
        instance = instances[address]
        gate.require(gate.canonical(resource['values']) == gate.canonical(instance['attributes'])
            and resource['schema_version'] == instance['schema_version']
            and resource.get('identity_schema_version',0) == instance.get('identity_schema_version',0)
            and gate.canonical(resource.get('identity')) == gate.canonical(instance.get('identity'))
            and gate.canonical(resource.get('depends_on',[]))==gate.canonical(instance.get('dependencies',[])), 'ARCHIVE')
        # jsonstate/state.go unions raw marks with provider schema sensitivity.
        # The pinned Google8.5 nine-type schema has exactly these three schema
        # marks (notification_channel.sensitive_labels); no mark is ignored.
        expected_marks=sensitive_paths(instance.get('sensitive_attributes',[]),instance['attributes'])
        if resource['type']=='google_monitoring_notification_channel':
            blocks=instance['attributes'].get('sensitive_labels',[])
            gate.require(type(blocks)is list and len(blocks)<=1,'ARCHIVE')
            for index,block in enumerate(blocks):
                gate.require(type(block)is dict and set(block)<={'auth_token','password','service_key'}
                    and all(value is None or type(value)is str for value in block.values()),'ARCHIVE')
                expected_marks|={('sensitive_labels',index,key)for key in block}
        expected_marks={path for path in expected_marks if not any(
            len(prefix)<len(path)and path[:len(prefix)]==prefix for prefix in expected_marks)}
        gate.require(expected_marks==mask_paths(resource['sensitive_values'],instance['attributes']),'ARCHIVE')
    gate.require(gate.canonical(refresh['prior_state']['values']['outputs']) ==
        gate.canonical({name:dict(type=item['type'],value=item['value'],sensitive=item.get('sensitive',False))
                        for name,item in candidate['outputs'].items()}), 'ARCHIVE')


def inspect_candidate(raw, before_raw, observation, refresh, *, expected_snapshot,catalog):
    """Return a private expected object, never a serialized State or CLI input.

    expected_snapshot is generated from the same-R checkout and initialized
    module manifest, including every config snapshot member and lockfile.
    """
    try:
        members = read_archive(raw)
        gate.require(type(expected_snapshot) is dict and bool(expected_snapshot)
            and all(type(key) is str and type(value) is bytes for key,value in expected_snapshot.items()), 'ARCHIVE')
        actual = {key:value for key,value in members.items() if key not in ('tfplan','tfstate','tfstate-prev')}
        gate.require(actual == expected_snapshot, 'ARCHIVE')
        before, _ = gate._state(before_raw)
        previous = json_value(members['tfstate-prev'])
        candidate, _ = gate._state(members['tfstate'])
        _,plan_info=checks.decode_plan(members['tfplan'],expected_mode=2,catalog=catalog)
        gate.require(plan_info['resource_change_count']==0 and plan_info['drift_count']==1,'ARCHIVE')
        typed = checks.verify_checks(members['tfplan'], refresh.get('checks'), before.get('check_results'),catalog=catalog)
        gate.require(checks.raw_checks(previous.get('check_results'),catalog=catalog) == typed, 'ARCHIVE')
        gate.require(previous.get('lineage') == '' and type(previous.get('serial')) is int
            and previous['serial'] == 0, 'ARCHIVE')
        previous = copy.deepcopy(previous)
        previous['lineage'] = before['lineage']; previous['serial'] = before['serial']
        gate.require(gate.canonical(normalized_state(previous,catalog=catalog)) == gate.canonical(normalized_state(before,catalog=catalog)), 'ARCHIVE')
        gate.require(candidate['lineage'] == before['lineage'] and candidate['serial'] == before['serial'], 'ARCHIVE')
        # Core replaces the archive candidate's Checks using protobuf field19.
        # Native witnessed null is allowed only here and only after the exact
        # proto/show/raw-before full check-set join above.
        gate.require(candidate.get('check_results') is None
            or checks.raw_checks(candidate['check_results'],catalog=catalog) == typed, 'ARCHIVE')
        expected = copy.deepcopy(candidate)
        expected['check_results'] = checks.planned_checks(before['check_results'],typed,catalog=catalog)
        gate.require(gate.canonical(checks.canonical_checks(expected['check_results'],catalog=catalog)) ==
            gate.canonical(checks.canonical_checks(before['check_results'],catalog=catalog)), 'ARCHIVE')
        _prior_join(refresh,candidate)
        normalized_before = normalized_state(before,catalog=catalog); normalized_expected = normalized_state(expected,catalog=catalog)
        gate.validate_raw_transition(gate.canonical(normalized_before).encode(),
            gate.canonical(normalized_expected).encode(),observation,serial_delta=0)
        expected['serial'] += 1
        gate._bounded(expected)
        return expected
    except gate.SyncRejected:
        raise
    except Exception:
        raise gate.SyncRejected('ARCHIVE') from None


def verify_after(raw, expected, *, catalog):
    try:
        after,_ = gate._state(raw)
        gate.require(gate.canonical(normalized_state(after,catalog=catalog)) == gate.canonical(normalized_state(expected,catalog=catalog)), 'STATE_DELTA')
    except gate.SyncRejected:
        raise
    except Exception:
        raise gate.SyncRejected('STATE_DELTA') from None
