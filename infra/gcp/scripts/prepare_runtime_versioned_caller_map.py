#!/usr/bin/env python3
"""Offline review candidate: historical immutable task configuration plus current anchors.

No authentication, network, State, legacy capture conversion, or readiness claim.
Pinned source/journal consistency is not a signed execution attestation.
"""
from __future__ import annotations

import argparse
import ast
import copy
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

import collect_aws_runtime_versioned_config as aws
import collect_codex_runtime_metadata as codex
import collect_runtime_metadata as gcp
import prepare_runtime_ownership as ownership

ROOT = Path(__file__).resolve().parents[3]
MAX_BYTES = 4 * 1024 * 1024
MAX_CAPTURE_AGE = timedelta(hours=1)
INPUTS = {'aws_request', 'aws_current', 'owner_task_capture', 'aws_auth_receipt',
          'aws_auth_wrapper', 'aws_diagnostic_harness', 'aws_canonical', 'gcp_capture', 'gcp_manifest'}
SOURCE_FILES = {'producer': Path(__file__), 'aws_collector': Path(aws.__file__),
                'gcp_collector': Path(codex.__file__),
                'gcp_validator': Path(gcp.__file__), 'ownership_validator': Path(ownership.__file__),
                'history_validator': Path(gcp.validate_history.__globals__['__file__'])}
REQUEST_KEYS = {'schema_version', 'kind', 'environment', 'git_sha', 'issued_utc', 'expires_utc',
                'sources', 'inputs', 'aws_source_revision', 'gcp_source_revision', 'gcp_auth_selector_sha256'}
GCP_KEYS = {'schema_version', 'kind', 'source_revision', 'manifest_sha256', 'capture_started_utc',
            'capture_completed_utc', 'complete', 'metadata', 'explicit_auth_requests',
            'explicit_metadata_requests', 'source_identity_verified', 'canonical_auth_selector_sha256',
            'lambda6_source_match', 'ecs_source_match', 'ci_identity_read_verified', 'caller_scope_verified',
            'root_admission_verified', 'migration_ready'}
FALSE_CLAIMS = {'full_seven_caller_fresh', 'direct_task_definition_read_verified',
                'historical_capture_reauthenticated', 'prior_task_describe_denial_retested',
                'task_role_existence_verified', 'role_id_continuity_verified',
                'workload_sts_identity_verified', 'stepfunctions_runtime_overrides_verified',
                'natural_execution_verified', 'ci_identity_read_verified', 'root_admission_verified',
                'state_verified', 'all_wif_member_purposes_resolved', 'migration_ready',
                'cross_cloud_atomic_snapshot_verified', 'lambda_configuration_stability_verified',
                'task_selection_after_observation_verified'}


class PrivateParser(argparse.ArgumentParser):
    def error(self, message):
        raise ValueError('private caller-map arguments STOP')


def need(value):
    if not value:
        raise ValueError('private versioned caller-map STOP')


def now():
    return datetime.now(timezone.utc)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def digest(value):
    need(type(value) is str and re.fullmatch(r'[0-9a-f]{64}', value))
    return value


def decode(raw):
    need(type(raw) is bytes and 0 < len(raw) <= MAX_BYTES)
    return json.loads(raw.decode('utf-8', errors='strict'), object_pairs_hook=aws.unique,
                      parse_constant=lambda _: need(False), parse_float=finite_float)


def finite_float(raw):
    value = float(raw)
    need(value not in (float('inf'), float('-inf')))
    return value


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=False,
                      allow_nan=False).encode('utf-8')


def private_parent(path):
    aws.private_parent(path)


def private_bytes(path, expected_hash):
    digest(expected_hash)
    private_parent(path)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        info = os.fstat(handle.fileno())
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
             and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1
             and 0 < info.st_size <= MAX_BYTES)
        raw = handle.read(MAX_BYTES + 1)
    need(0 < len(raw) <= MAX_BYTES and sha(raw) == expected_hash)
    return raw


def validate_request(request, current_time):
    need(type(request) is dict and set(request) == REQUEST_KEYS
         and type(request['schema_version']) is int and request['schema_version'] == 1
         and request['kind'] == 'private-runtime-versioned-caller-map-request'
         and request['environment'] == 'dev')
    for key in ('git_sha', 'aws_source_revision', 'gcp_source_revision'):
        need(type(request[key]) is str and re.fullmatch(r'[0-9a-f]{40}', request[key]))
    issued, expires = aws.stamp(request['issued_utc']), aws.stamp(request['expires_utc'])
    need(issued <= current_time < expires <= issued + timedelta(hours=1))
    need(type(request['sources']) is dict and set(request['sources']) == set(SOURCE_FILES))
    for key, path in SOURCE_FILES.items():
        need(not path.is_symlink() and sha(path.read_bytes()) == digest(request['sources'][key]))
    digest(request['gcp_auth_selector_sha256'])
    need(type(request['inputs']) is dict and set(request['inputs']) == INPUTS)
    for entry in request['inputs'].values():
        need(type(entry) is dict and set(entry) == {'path', 'sha256'} and type(entry['path']) is str)
        digest(entry['sha256'])
    return issued, expires


def expected_journal(plan):
    rows = [dict(event='diagnostic-sink-reserved', authentication_call_limit=2, metadata_call_limit=4)]
    for index, alias, checked in ((1, 'source-identity', 'source-identity-validated'),
                                  (2, 'readonly-session', 'returned-session-validated')):
        rows += [dict(event='auth-cli-intent', alias=alias, index=index),
                 dict(event='auth-cli-returned', alias=alias, index=index, return_code=0),
                 dict(event=checked, outcome='PASS')]
    rows.append(dict(event='diagnostic-start', metadata_call_limit=4,
                     read_returned_means='aws-read-function-returned-not-semantic-or-http-attestation'))
    for index, call in enumerate(plan, 1):
        fields = dict(alias=call['alias'], index=index)
        rows += [dict(event='read-attempted', **fields), dict(event='cli-send-intent', **fields),
                 dict(event='cli-returned', return_code=0, aws_error_code='NONE', **fields),
                 dict(event='read-function-returned', **fields)]
    rows.append(dict(event='collector-finished', collector_exit_code=0, metadata_reads_attempted=4,
                     read_functions_returned=4, cli_subprocesses_attempted=4, cli_subprocesses_returned=4))
    return rows


def validate_aws(raw, request, current_time):
    req, current, historical, canon = (decode(raw[k]) for k in
        ('aws_request', 'aws_current', 'owner_task_capture', 'aws_canonical'))
    need(sha(raw['owner_task_capture']) == req['owner_capture_sha256'])
    issued, expires = aws.validate(req, historical)
    need(issued <= current_time < expires)
    need(type(canon) is dict and set(canon) == {'schema_version', 'canonical_page', 'source_account',
         'source_arn', 'source_user_id', 'environments'}
         and type(canon['schema_version']) is str and bool(canon['schema_version'])
         and type(canon['canonical_page']) is str and bool(canon['canonical_page'])
         and type(canon['environments']) is dict)
    source_account = canon['source_account']
    need(type(source_account) is str and re.fullmatch(r'[0-9]{12}', source_account)
         and type(canon['source_arn']) is str and re.fullmatch(
             r'arn:aws:iam::' + source_account + r':user/[A-Za-z0-9+=,.@_/-]{1,512}', canon['source_arn'])
         and type(canon['source_user_id']) is str and re.fullmatch(r'[A-Z0-9]{16,128}', canon['source_user_id']))
    need(canon['environments']['dev'] == dict(account=req['aws_account_id'], region=aws.REGION,
         stack_name=aws.STACK, readonly_role_arn=req['readonly_role_arn']))
    need(type(current) is dict and current.get('mode') == 'current-read'
         and current.get('historical_capture', {}).get('body') == historical)
    observations = current.get('observations')
    need(type(observations) is list and len(observations) == 4)
    values = iter(copy.deepcopy(observations))
    times = [current['observed_started_at_utc']]
    for item in observations:
        need(type(item) is dict and set(item) == {'alias', 'started_at_utc', 'finished_at_utc', 'value'})
        times += [item['started_at_utc'], item['finished_at_utc']]
    times.append(current['observed_finished_at_utc'])
    clock = iter(map(aws.stamp, times))
    def replay(call):
        item = next(values)
        need(call['alias'] == item['alias'])
        return item['value']
    need(canonical(aws.collect(req, historical, current=True, read=replay, clock=lambda: next(clock))) == canonical(current)
         and aws.stamp(times[-1]) <= current_time)
    journal = raw['aws_auth_receipt']
    need(len(journal) <= 65536)
    rows = [decode(line) for line in journal.splitlines()]
    need(canonical(rows) == canonical(expected_journal(current['call_plan'])))
    need(historical['task_role_arn'] != historical['execution_role_arn'])
    return req, current, historical, canon


def validate_gcp(raw, request, canonical_source, current_time):
    capture, manifest = decode(raw['gcp_capture']), decode(raw['gcp_manifest'])
    codex.validate_manifest(manifest)
    need(manifest['execute_approved'] is True
         and manifest['script_sha256'] == request['sources']['gcp_collector']
         and manifest['collector_sha256'] == request['sources']['gcp_validator']
         and manifest['requirements_sha256'] == sha(codex.REQUIREMENTS.read_bytes()))
    need(type(capture) is dict and set(capture) == GCP_KEYS
         and type(capture['schema_version']) is int and capture['schema_version'] == 1
         and capture['kind'] == 'private-codex-runtime-metadata-capture'
         and capture['source_revision'] == request['gcp_source_revision'] == manifest['source_revision']
         and capture['manifest_sha256'] == sha(raw['gcp_manifest'])
         and capture['canonical_auth_selector_sha256'] == request['gcp_auth_selector_sha256'] == manifest['auth_selector_sha256']
         and capture['complete'] is True and capture['source_identity_verified'] is True
         and capture['lambda6_source_match'] is True and type(capture['ecs_source_match']) is bool
         and all(capture[k] is False for k in ('ci_identity_read_verified', 'caller_scope_verified',
                                             'root_admission_verified', 'migration_ready'))
         and type(capture['explicit_auth_requests']) is int and capture['explicit_auth_requests'] == 3
         and type(capture['explicit_metadata_requests']) is int and 7 <= capture['explicit_metadata_requests'] <= 22)
    need(manifest['source_identity'] == dict(account_id=canonical_source['source_account'],
         arn=canonical_source['source_arn'], user_id=canonical_source['source_user_id']))
    start, end = aws.stamp(capture['capture_started_utc']), aws.stamp(capture['capture_completed_utc'])
    need(aws.stamp(manifest['issued_utc']) <= start <= end < aws.stamp(manifest['expires_utc'])
         and end < start + codex.MAX_RUN
         and end <= current_time < start + MAX_CAPTURE_AGE)
    data = capture['metadata']
    project = data['project']
    number = project['projectNumber']
    pool = 'projects/' + number + '/locations/global/workloadIdentityPools/aws-runtime'
    selectors = dict(project_number=number, runtime_pool_resource=pool,
        runtime_provider_resource=pool + '/providers/aws-provider',
        runtime_service_account_email=aws.PROJECT + '@appspot.gserviceaccount.com',
        runtime_iam_policy_resource='projects/' + aws.PROJECT + '/serviceAccounts/' + aws.PROJECT + '@appspot.gserviceaccount.com',
        runtime_audience='//iam.googleapis.com/' + pool + '/providers/aws-provider')
    expected = [data[k] for k in ('project', 'pool', 'provider', 'pool_attestation_rules', 'iam_policy', 'enabled_services', 'history_table')]
    calls = iter(copy.deepcopy(expected))
    need(gcp.collect(selectors, lambda *args, **kwargs: next(calls)) == data)
    return capture, data


def caller_grants(data, aws_request, current, historical):
    need(data['provider']['aws']['accountId'] == aws_request['aws_account_id'])
    auth = historical['runtime_auth']
    need(auth == dict(GOOGLE_CLOUD_PROJECT=data['project']['projectId'],
         GCP_WIF_AUDIENCE=data['runtime_config']['audience'],
         GCP_WIF_SERVICE_ACCOUNT_EMAIL=data['runtime_config']['service_account_email']))
    mapping = data['provider']['attributeMapping']
    need(set(mapping) == {'google.subject', 'attribute.account', 'attribute.aws_role'}
         and mapping['attribute.account'] == 'assertion.account'
         and mapping['attribute.aws_role'] == "assertion.arn.extract('assumed-role/{role_name}/')"
         and mapping['google.subject'] in {'assertion.arn', "assertion.account + ':' + assertion.userid.split(':')[0]"})
    functions = {item['name']: item for item in current['observations'][3]['value']['selected']}
    callers = [dict(caller='lambda:' + name, role_arn=functions[name]['role'],
                    basis='current-lambda-configuration-point-observation') for name in aws.CALLERS]
    callers.append(dict(caller='ecs:daily-batch', role_arn=historical['task_role_arn'],
                        basis='historical-immutable-revision-with-current-selection-anchors'))
    roles = {item['role_arn'].rsplit('/', 1)[1] for item in callers}
    condition = data['provider']['attributeCondition']
    match = re.fullmatch(r"attribute\.account == '([0-9]{12})' && attribute\.aws_role in (\[[^\r\n]*\])", condition)
    need(match and match[1] == aws_request['aws_account_id'])
    allowlist = ast.literal_eval(match[2])
    need(type(allowlist) is list and all(type(role) is str and re.fullmatch(r'[A-Za-z0-9_+=,.@-]+', role) for role in allowlist)
         and len(allowlist) == len(set(allowlist)) and set(allowlist) == roles)
    prefix = 'principalSet://iam.googleapis.com/' + data['pool']['name'] + '/attribute.aws_role/'
    grants = {}
    for role in sorted(roles):
        member = prefix + role
        matches = [binding for binding in data['iam_policy']['bindings']
                   if binding['role'] == ownership.ROLE and member in binding['members']]
        need(len(matches) == 1 and len(matches[0]['members']) == len(set(matches[0]['members'])))
        actual = matches[0].get('condition')
        alias = 'grant-' + str(len(grants) + 1).zfill(2)
        grants[alias] = dict(member=member, condition_title=actual['title'] if actual else '')
        for caller in callers:
            if caller['role_arn'].rsplit('/', 1)[1] == role:
                caller['grant_alias'] = alias
    need(len(callers) == 7 and len({item['caller'] for item in callers}) == 7)
    remaining = [dict(member=member, condition=copy.deepcopy(binding.get('condition')))
                 for binding in data['iam_policy']['bindings'] if binding['role'] == ownership.ROLE
                 for member in binding['members'] if member not in {item['member'] for item in grants.values()}]
    inventory = copy.deepcopy(data)
    inventory['runtime_grants'] = grants
    inventory['api_classification'] = {item['config']['name']: dict(classification='Investigate',
        dependency_addresses=[], reason='Unreviewed API ownership; candidate only.')
        for item in data['enabled_services']['services']}
    ownership.prepare(inventory)
    return callers, inventory, remaining


def prepare_candidate(request, raw, *, current_time=None, request_raw=None):
    current_time = current_time or now()
    request_raw = canonical(request) if request_raw is None else request_raw
    need(canonical(decode(request_raw)) == canonical(request))
    validate_request(request, current_time)
    need(type(raw) is dict and set(raw) == INPUTS)
    for key in INPUTS:
        need(type(raw[key]) is bytes and 0 < len(raw[key]) <= MAX_BYTES
             and sha(raw[key]) == request['inputs'][key]['sha256'])
    aws_request, current, historical, source = validate_aws(raw, request, current_time)
    capture, data = validate_gcp(raw, request, source, current_time)
    callers, inventory, remaining = caller_grants(data, aws_request, current, historical)
    evidence_expiry = min(aws.stamp(request['expires_utc']), aws.stamp(aws_request['expires_utc']),
                          aws.stamp(capture['capture_started_utc']) + MAX_CAPTURE_AGE)
    result = dict(schema_version=1, kind='private-runtime-versioned-caller-grant-review-candidate',
        environment='dev', git_sha=request['git_sha'], prepared_utc=current_time.isoformat(),
        request_window=dict(issued_utc=request['issued_utc'], expires_utc=request['expires_utc']),
        evidence_expires_utc=evidence_expiry.isoformat(),
        configuration_map_complete=True, caller_count=7, required_grant_candidate_count=len(inventory['runtime_grants']),
        callers=callers, inventory=inventory, inventory_sha256=sha(canonical(inventory)),
        remaining_wif_members_requiring_purpose_review=remaining,
        historical_task_projection=copy.deepcopy(current['historical_capture']),
        evidence=dict(review_request_raw_sha256=sha(request_raw),
            input_sha256={key: request['inputs'][key]['sha256'] for key in INPUTS},
            source_sha256=copy.deepcopy(request['sources']), aws_source_revision=request['aws_source_revision'],
            gcp_source_revision=request['gcp_source_revision'],
            aws_observed_started_utc=current['observed_started_at_utc'], aws_observed_completed_utc=current['observed_finished_at_utc'],
            gcp_capture_started_utc=capture['capture_started_utc'], gcp_capture_completed_utc=capture['capture_completed_utc'],
            gcp_execution_window={key: decode(raw['gcp_manifest'])[key] for key in ('issued_utc', 'expires_utc')},
            gcp_capture_ecs_source_match=capture['ecs_source_match'],
            fixed_auth_journal_consistency=True, signed_execution_attestation=False),
        interpretation={key: False for key in FALSE_CLAIMS})
    need(len(canonical(result)) <= MAX_BYTES)
    return result


def main(argv=None):
    output = created = descriptor = None
    try:
        parser = PrivateParser(description=__doc__)
        parser.add_argument('--request-file', type=Path, required=True)
        parser.add_argument('--request-sha256', required=True)
        parser.add_argument('--prepare', action='store_true')
        parser.add_argument('--output', type=Path)
        args = parser.parse_args(argv)
        need(args.prepare == (args.output is not None))
        request_raw = private_bytes(args.request_file, args.request_sha256)
        request = decode(request_raw)
        started = now()
        validate_request(request, started)
        raw = {key: private_bytes(Path(item['path']), item['sha256']) for key, item in request['inputs'].items()}
        candidate = prepare_candidate(request, raw, request_raw=request_raw)
        need(started <= aws.stamp(candidate['prepared_utc']))
        if args.prepare:
            output = args.output
            private_parent(output)
            descriptor = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
            info = os.fstat(descriptor)
            created = info.st_dev, info.st_ino
            handle = os.fdopen(descriptor, 'wb')
            descriptor = None
            with handle:
                need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
                     and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1)
                encoded = canonical(candidate)
                need(len(encoded) <= MAX_BYTES)
                handle.write(encoded)
            # All evidence windows are checked again after serialization/save.
            checked = prepare_candidate(request, raw, request_raw=request_raw)
            need(aws.stamp(candidate['prepared_utc']) <= aws.stamp(checked['prepared_utc']))
        print('Offline caller configuration candidate checked; admission and readiness remain unverified.')
        return 0
    except Exception:
        if descriptor is not None:
            os.close(descriptor)
        if output is not None and created is not None:
            try:
                info = output.lstat()
                if (info.st_dev, info.st_ino) == created:
                    output.unlink()
            except OSError:
                pass
        print('STOP: private caller-map rejected; diagnostic suppressed.', file=sys.stderr)
        return 3


if __name__ == '__main__':
    raise SystemExit(main())
