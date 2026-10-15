#!/usr/bin/env python3
"""Keep historical ECS configuration separate from fresh bounded AWS observations.

Default mode is offline. Explicit current reads use an already selected exact
read-only session; this helper never logs in, assumes roles, mutates resources,
reads State, invokes workloads, or attests migration readiness.
"""
from __future__ import annotations

import argparse
import copy
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys

MAX_BYTES = 262144
REGION = 'ap-northeast-1'
STACK = 'AwsCdkStack'
PROJECT = 'test-youtube-study-space'
CALLERS = ('sns_notify_discord', 'set_desired_max_seats',
           'youtube_organize_database', 'check_live_stream_status',
           'update_work_name_trend', 'error_log_notify_discord')
AUTH_KEYS = ('GOOGLE_CLOUD_PROJECT', 'GCP_WIF_AUDIENCE',
             'GCP_WIF_SERVICE_ACCOUNT_EMAIL')
PARAMETERS = dict(zip(('GoogleCloudProject', 'GcpWifAudience',
                       'GcpWifServiceAccountEmail'), AUTH_KEYS))
REQUEST_KEYS = frozenset({'schema_version', 'kind', 'environment', 'aws_account_id',
    'aws_region', 'stack_name', 'readonly_role_arn', 'readonly_session_name',
    'owner_capture_sha256', 'issued_utc', 'expires_utc'})
CAPTURE_KEYS = frozenset({'schema_version', 'capture_type', 'captured_at', 'environment',
    'aws_region', 'task_definition_arn', 'task_role_arn', 'execution_role_arn',
    'runtime_auth', 'stack_reference_match', 'three_auth_env_match', 'metadata_reads',
    'cloud_mutations', 'readiness_attested'})
STACK_QUERY = ('Stacks[].{name:StackName,status:StackStatus,'
    'outputs:Outputs[?OutputKey==`DailyBatchTaskDefinitionArn`],'
    'parameters:Parameters[?ParameterKey==`GoogleCloudProject` || '
    'ParameterKey==`GcpWifAudience` || ParameterKey==`GcpWifServiceAccountEmail`].'
    '{key:ParameterKey,value:ParameterValue}}')
FUNCTION_QUERY = ('{name:FunctionName,arn:FunctionArn,role:Role,version:Version,'
    'revision_id:RevisionId,last_modified:LastModified,'
    'auth:{GOOGLE_CLOUD_PROJECT:Environment.Variables.GOOGLE_CLOUD_PROJECT,'
    'GCP_WIF_AUDIENCE:Environment.Variables.GCP_WIF_AUDIENCE,'
    'GCP_WIF_SERVICE_ACCOUNT_EMAIL:Environment.Variables.GCP_WIF_SERVICE_ACCOUNT_EMAIL}}')


class Stop(ValueError):
    pass


class PrivateParser(argparse.ArgumentParser):
    def error(self, message):
        raise Stop('private arguments STOP')


def need(value):
    if not value:
        raise Stop('private versioned configuration STOP')


def now():
    return datetime.now(timezone.utc)


def stamp(value):
    need(type(value) is str and len(value) <= 40)
    parsed = datetime.fromisoformat(value.replace('Z', '+00:00'))
    need(parsed.utcoffset() == timedelta(0))
    return parsed


def digest(value):
    need(type(value) is str and re.fullmatch(r'[0-9a-f]{64}', value))
    return value


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def unique(pairs):
    result = {}
    for key, value in pairs:
        need(key not in result)
        result[key] = value
    return result


def decode(raw):
    need(type(raw) is bytes and 0 < len(raw) <= MAX_BYTES)
    value = json.loads(raw, object_pairs_hook=unique, parse_constant=lambda _: need(False))
    need(type(value) is dict)
    return value


def private_parent(path):
    need(path.is_absolute() and path == path.resolve() and not path.is_symlink())
    info = path.parent.lstat()
    need(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
         and stat.S_IMODE(info.st_mode) == 0o700)
    # A real checkout/worktree has .git/HEAD or a .git file. Empty read-only
    # sandbox root guards are not checkouts and are neither changed nor removed.
    for directory in (path.parent, *path.parent.parents):
        marker = directory / '.git'
        if marker.exists() or marker.is_symlink():
            info = marker.lstat()
            empty_root_guard = (marker in (Path('/tmp/.git'), Path('/workspace/.git'))
                and stat.S_ISDIR(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o555
                and not any(marker.iterdir()))
            need(empty_root_guard)


def private_input(path, expected_hash):
    digest(expected_hash)
    private_parent(path)
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor, 'rb') as handle:
        info = os.fstat(handle.fileno())
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
             and stat.S_IMODE(info.st_mode) == 0o600 and info.st_nlink == 1
             and info.st_size <= MAX_BYTES)
        raw = handle.read(MAX_BYTES + 1)
    need(sha(raw) == expected_hash)
    return decode(raw)


def role_arn(value, account):
    need(type(value) is str and re.fullmatch(
        r'arn:aws:iam::' + account + r':role/[A-Za-z0-9+=,.@_/-]{1,512}', value))


def validate(request, capture):
    need(type(request) is dict and set(request) == REQUEST_KEYS
         and type(request['schema_version']) is int and request['schema_version'] == 1
         and request['kind'] == 'private-aws-runtime-versioned-config-request'
         and request['environment'] == 'dev' and request['aws_region'] == REGION
         and request['stack_name'] == STACK)
    account = request['aws_account_id']
    need(type(account) is str and re.fullmatch(r'[0-9]{12}', account))
    role_arn(request['readonly_role_arn'], account)
    need(type(request['readonly_session_name']) is str and re.fullmatch(
        r'[A-Za-z0-9+=,.@_-]{2,64}', request['readonly_session_name']))
    digest(request['owner_capture_sha256'])
    issued, expires = stamp(request['issued_utc']), stamp(request['expires_utc'])
    need(issued < expires <= issued + timedelta(hours=1))
    need(type(capture) is dict and set(capture) == CAPTURE_KEYS
         and type(capture['schema_version']) is int and capture['schema_version'] == 1
         and capture['capture_type'] == 'aws-runtime-task-metadata-v1'
         and capture['environment'] == 'dev' and capture['aws_region'] == REGION
         and capture['stack_reference_match'] is True
         and capture['three_auth_env_match'] is True
         and type(capture['metadata_reads']) is int and capture['metadata_reads'] == 4
         and type(capture['cloud_mutations']) is int and capture['cloud_mutations'] == 0
         and capture['readiness_attested'] is False)
    need(stamp(capture['captured_at']) <= issued)
    arn = capture['task_definition_arn']
    need(type(arn) is str and re.fullmatch(
        r'arn:aws:ecs:' + REGION + ':' + account
        + r':task-definition/[A-Za-z0-9_-]{1,255}:[1-9][0-9]*', arn))
    for key in ('task_role_arn', 'execution_role_arn'):
        role_arn(capture[key], account)
    auth = capture['runtime_auth']
    need(type(auth) is dict and set(auth) == set(AUTH_KEYS)
         and all(type(value) is str and 0 < len(value) <= 512 for value in auth.values())
         and auth['GOOGLE_CLOUD_PROJECT'] == PROJECT
         and auth['GCP_WIF_SERVICE_ACCOUNT_EMAIL'] == PROJECT + '@appspot.gserviceaccount.com'
         and re.fullmatch(r'//iam.googleapis.com/projects/[0-9]+/locations/global/'
            r'workloadIdentityPools/aws-runtime/providers/aws-provider', auth['GCP_WIF_AUDIENCE']))
    return issued, expires


def call_plan(request, capture):
    validate(request, capture)
    account = request['aws_account_id']
    family = capture['task_definition_arn'].rsplit('/', 1)[1].rsplit(':', 1)[0]
    calls = [
        {'alias': 'reader-identity', 'service': 'sts', 'operation': 'get-caller-identity',
         'arguments': [], 'query': '{account:Account,arn:Arn}'},
        {'alias': 'stack-version', 'service': 'cloudformation', 'operation': 'describe-stacks',
         'arguments': ['--stack-name', STACK, '--no-paginate'], 'query': STACK_QUERY},
        {'alias': 'family-active-head', 'service': 'ecs', 'operation': 'list-task-definitions',
         'arguments': ['--cli-input-json', json.dumps({'familyPrefix': family, 'status': 'ACTIVE',
             'sort': 'DESC', 'maxResults': 1}, sort_keys=True, separators=(',', ':')),
             '--no-paginate'], 'query': 'taskDefinitionArns'},
    ]
    for name in CALLERS:
        calls.append({'alias': name, 'service': 'lambda', 'operation': 'get-function-configuration',
            'arguments': ['--function-name', 'arn:aws:lambda:' + REGION + ':' + account
                + ':function:' + name], 'query': FUNCTION_QUERY})
    return calls


def aws_read(call):
    need(type(call) is dict and set(call) == {'alias', 'service', 'operation', 'arguments', 'query'})
    need((call['service'], call['operation']) in {
        ('sts', 'get-caller-identity'), ('cloudformation', 'describe-stacks'),
        ('ecs', 'list-task-definitions'), ('lambda', 'get-function-configuration')})
    command = ['aws', call['service'], call['operation'], *call['arguments'], '--region', REGION,
               '--output', 'json', '--no-cli-pager', '--cli-connect-timeout', '10',
               '--cli-read-timeout', '20', '--query', call['query']]
    environment = os.environ.copy()
    environment.update(AWS_PAGER='', AWS_CLI_AUTO_PROMPT='off', AWS_MAX_ATTEMPTS='1')
    result = subprocess.run(command, env=environment, capture_output=True, text=True,
                            check=False, timeout=45)
    need(result.returncode == 0 and len(result.stdout.encode()) <= MAX_BYTES)
    return json.loads(result.stdout, object_pairs_hook=unique, parse_constant=lambda _: need(False))


def check_observation(call, value, request, capture):
    account = request['aws_account_id']
    if call['alias'] == 'reader-identity':
        role_name = request['readonly_role_arn'].rsplit('/', 1)[1]
        need(value == {'account': account, 'arn': 'arn:aws:sts::' + account + ':assumed-role/'
            + role_name + '/' + request['readonly_session_name']})
    elif call['alias'] == 'stack-version':
        need(type(value) is list and len(value) == 1)
        stack = value[0]
        need(type(stack) is dict and set(stack) == {'name', 'status', 'outputs', 'parameters'}
             and stack['name'] == STACK and type(stack['status']) is str and stack['status'])
        outputs = stack['outputs']
        need(type(outputs) is list and len(outputs) == 1
             and type(outputs[0]) is dict
             and set(outputs[0]) <= {'OutputKey', 'OutputValue', 'Description', 'ExportName'}
             and outputs[0].get('OutputKey') == 'DailyBatchTaskDefinitionArn'
             and outputs[0].get('OutputValue') == capture['task_definition_arn'])
        pairs = stack['parameters']
        need(type(pairs) is list and len(pairs) == 3
             and all(type(pair) is dict and set(pair) == {'key', 'value'} for pair in pairs))
        parameters = unique((pair['key'], pair['value']) for pair in pairs)
        need(set(parameters) == set(PARAMETERS)
             and all(parameters[key] == capture['runtime_auth'][auth] for key, auth in PARAMETERS.items()))
    elif call['alias'] == 'family-active-head':
        # This is the first ACTIVE/DESC page (maxResults=1), not a complete list.
        need(type(value) is list and value == [capture['task_definition_arn']])
    else:
        name = call['alias']
        need(name in CALLERS and type(value) is dict and set(value) == {
            'name', 'arn', 'role', 'version', 'revision_id', 'last_modified', 'auth'}
            and value['name'] == name
            and value['arn'] == 'arn:aws:lambda:' + REGION + ':' + account + ':function:' + name
            and value['version'] == '$LATEST'
            and value['auth'] == capture['runtime_auth'])
        role_arn(value['role'], account)
        need(all(type(value[key]) is str and 0 < len(value[key]) <= 128
                 for key in ('revision_id', 'last_modified')))


def collect(request, capture, *, current=False, read=None, clock=None):
    read = aws_read if read is None else read
    clock = now if clock is None else clock
    issued, expires = validate(request, capture)
    plan = call_plan(request, capture)
    result = {
        'schema_version': 1, 'kind': 'private-aws-runtime-versioned-config-observations',
        'source_usage': 'historical-immutable-task-projection-with-current-version-anchors',
        'environment': 'dev', 'mode': 'current-read' if current else 'offline',
        'historical_capture': {'sha256': request['owner_capture_sha256'],
            'usage': 'historical-task-role-and-three-auth-projection', 'body': copy.deepcopy(capture)},
        'requested_observation_window': {'issued_utc': request['issued_utc'], 'expires_utc': request['expires_utc']},
        'call_plan': plan, 'observations': [], 'metadata_reads': 0, 'cloud_mutations': 0,
        'state_accesses': 0,
        'interpretation': {
            'fresh_stack_output_matches_historical_revision': False,
            'fresh_active_family_head_matches_historical_revision': False,
            'fresh_lambda_six_configuration_auth_matches_historical_auth': False,
            'reader_identity_verified': False,
            'direct_task_definition_read_verified': False,
            'historical_capture_reauthenticated': False,
            'prior_task_describe_denial_retested': False,
            'task_role_existence_verified': False, 'role_id_continuity_verified': False,
            'workload_sts_identity_verified': False, 'stepfunctions_runtime_overrides_verified': False,
            'natural_execution_verified': False, 'full_seven_caller_fresh': False,
            'ci_identity_read_verified': False, 'migration_ready': False,
        },
    }
    if not current:
        return result

    previous = issued

    def window():
        nonlocal previous
        observed = clock()
        need(issued <= previous <= observed < expires)
        previous = observed
        return observed.isoformat()

    result['observed_started_at_utc'] = window()
    for call in plan:
        started = window()
        value = read(copy.deepcopy(call))
        finished = window()
        check_observation(call, value, request, capture)
        result['observations'].append({'alias': call['alias'], 'started_at_utc': started,
            'finished_at_utc': finished, 'value': copy.deepcopy(value)})
    result['observed_finished_at_utc'] = window()
    result['metadata_reads'] = len(plan)
    result['interpretation']['reader_identity_verified'] = True
    result['interpretation']['fresh_stack_output_matches_historical_revision'] = True
    result['interpretation']['fresh_active_family_head_matches_historical_revision'] = True
    result['interpretation']['fresh_lambda_six_configuration_auth_matches_historical_auth'] = True
    return result


def main(argv=None):
    output = descriptor = created_identity = None
    current = False
    request = None
    try:
        parser = PrivateParser(description=__doc__)
        parser.add_argument('--request-file', type=Path, required=True)
        parser.add_argument('--request-sha256', required=True)
        parser.add_argument('--owner-capture-file', type=Path, required=True)
        parser.add_argument('--output', type=Path, required=True)
        parser.add_argument('--read-current', action='store_true')
        args = parser.parse_args(argv)
        current = args.read_current
        request = private_input(args.request_file, args.request_sha256)
        capture = private_input(args.owner_capture_file, request['owner_capture_sha256'])
        issued, expires = validate(request, capture)
        private_parent(args.output)
        output = args.output
        descriptor = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        info = os.fstat(descriptor)
        created_identity = (info.st_dev, info.st_ino)
        result = collect(request, capture, current=current)
        if current:
            before_save = now()
            need(issued <= stamp(result['observed_finished_at_utc']) <= before_save < expires)
        raw = json.dumps(result, sort_keys=True, indent=2).encode() + b'\n'
        need(len(raw) <= MAX_BYTES)
        with os.fdopen(descriptor, 'wb') as handle:
            descriptor = None
            os.fchmod(handle.fileno(), 0o600)
            handle.write(raw)
        if current:
            need(before_save <= now() < expires)
        print('AWS versioned configuration saved privately; migration readiness remains false.')
        return 0
    except Exception:
        if descriptor is not None:
            os.close(descriptor)
        if output is not None and created_identity is not None:
            try:
                info = output.lstat()
                if (info.st_dev, info.st_ino) == created_identity:
                    output.unlink()
            except OSError:
                pass
        print('AWS versioned configuration STOP; private diagnostic suppressed.', file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
