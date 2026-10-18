"""Owner-only metadata reads; never changes cloud settings or IAM policies."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import tempfile
from datetime import datetime, timedelta, timezone
import urllib.error
import urllib.request

PROJECT = 'test-youtube-study-space'
MAX_BYTES = 4 * 1024 * 1024
MAX_PAGES = 16
MAX_CANDIDATES = 32
PAGE_SIZE = 1000
OUTPUT = 'runtime-read-scope.json'
QUERY_PATH = 'v1/permissions:queryTestablePermissions'
NEEDED = ('iam.workloadIdentityPools.get', 'iam.workloadIdentityPools.getAttestationRules',
          'iam.workloadIdentityPoolProviders.get')
CAPTURE_KEYS = {'schema_version', 'kind', 'capture_started_utc', 'capture_completed_utc', 'complete',
                'metadata', 'lambda6_source_match', 'ecs_source_match',
                'ci_identity_read_verified', 'migration_ready'}
METADATA_KEYS = {'project', 'pool', 'provider', 'pool_attestation_rules', 'iam_policy_resource',
                 'runtime_config', 'iam_policy', 'enabled_services', 'history_table'}
PERMISSION_KEYS = {'name', 'title', 'description', 'onlyInPredefinedRoles', 'stage',
                   'customRolesSupportLevel', 'apiDisabled', 'primaryPermission'}

class Stop(ValueError):
    pass

class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise Stop('read-scope arguments STOP')

def need(value):
    if not value:
        raise Stop('read-scope STOP')

def utc_now():
    return datetime.now(timezone.utc)

def timestamp(value):
    need(type(value) is str and value.endswith('Z'))
    parsed = datetime.fromisoformat(value[:-1] + '+00:00')
    need(parsed.tzinfo is not None)
    return parsed

def unique(pairs):
    result = {}
    for key, value in pairs:
        need(key not in result)
        result[key] = value
    return result

def decode(raw):
    need(0 < len(raw) <= MAX_BYTES)
    value = json.loads(raw, object_pairs_hook=unique,
                       parse_constant=lambda _value: (_ for _ in ()).throw(Stop()))
    need(type(value) is dict)
    return value

def owned(path, directory=False):
    need(not path.is_symlink())
    info = path.lstat()
    need(info.st_uid == os.geteuid()
         and (stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode))
         and stat.S_IMODE(info.st_mode) == (0o700 if directory else 0o600))

def read_capture(path, capture_sha256):
    owned(path.parent, directory=True)
    # The previous command creates /tmp/yss-metadata-*/private; neither directory may redirect.
    owned(path.parent.parent, directory=True)
    owned(path)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        info = os.fstat(handle.fileno())
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
             and stat.S_IMODE(info.st_mode) == 0o600)
        raw = handle.read(MAX_BYTES + 1)
    need(hashlib.sha256(raw).hexdigest() == capture_sha256)
    return decode(raw)

def read_uploaded_capture(path, capture_sha256):
    # Explicit uploads may be 0644 in an owned, non-shared-write home directory.
    need(path.is_absolute() and path == path.resolve() and not path.is_symlink())
    parent = path.parent.lstat()
    need(stat.S_ISDIR(parent.st_mode) and parent.st_uid == os.geteuid()
         and parent.st_mode & 0o022 == 0)
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        info = os.fstat(handle.fileno())
        mode = stat.S_IMODE(info.st_mode)
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
             and mode & 0o700 == 0o600 and mode & 0o7022 == 0)
        os.fchmod(handle.fileno(), 0o600)
        raw = handle.read(MAX_BYTES + 1)
    need(hashlib.sha256(raw).hexdigest() == capture_sha256)
    return decode(raw)

def find_capture(capture_sha256):
    candidates = list(Path('/tmp').glob('yss-metadata-*/private/runtime-metadata-capture.json'))
    need(0 < len(candidates) <= MAX_CANDIDATES)
    matches = []
    for candidate in candidates:
        try:
            value = read_capture(candidate, capture_sha256)
            matches.append((candidate, value))
        except Exception:
            pass
    need(len(matches) == 1)
    return matches[0]

def validate_capture(value, now=None):
    current = now or utc_now()
    need(type(value) is dict and set(value) == CAPTURE_KEYS
         and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == 'private-operator-runtime-metadata-capture' and value['complete'] is True
         and value['lambda6_source_match'] is True and value['ecs_source_match'] is False
         and value['ci_identity_read_verified'] is False and value['migration_ready'] is False)
    start, end = timestamp(value['capture_started_utc']), timestamp(value['capture_completed_utc'])
    need(start <= end <= current and timedelta(0) <= current - start <= timedelta(hours=1))
    metadata = value['metadata']
    need(type(metadata) is dict and set(metadata) == METADATA_KEYS)
    project = metadata['project']
    need(type(project) is dict and project.get('projectId') == PROJECT
         and project.get('lifecycleState') == 'ACTIVE'
         and type(project.get('projectNumber')) is str and re.fullmatch(r'[0-9]+', project['projectNumber']))
    pool = 'projects/' + project['projectNumber'] + '/locations/global/workloadIdentityPools/aws-runtime'
    provider = pool + '/providers/aws-provider'
    need(type(metadata['pool']) is dict and metadata['pool'].get('name') == pool
         and metadata['pool'].get('state') == 'ACTIVE' and metadata['pool'].get('disabled', False) is False
         and metadata['pool'].get('mode') in (None, '', 'FEDERATION_ONLY')
         and type(metadata['provider']) is dict and metadata['provider'].get('name') == provider
         and metadata['provider'].get('state') == 'ACTIVE' and metadata['provider'].get('disabled', False) is False)
    attest = metadata['pool_attestation_rules']
    need(type(attest) is dict and set(attest) <= {'attestationRules', 'nextPageToken'}
         and type(attest.get('attestationRules', [])) is list and attest.get('attestationRules', []) == []
         and type(attest.get('nextPageToken', '')) is str and attest.get('nextPageToken', '') == '')
    email = PROJECT + '@appspot.gserviceaccount.com'
    need(metadata['iam_policy_resource'] == 'projects/' + PROJECT + '/serviceAccounts/' + email
         and metadata['runtime_config'] == {'audience': '//iam.googleapis.com/' + provider,
                                           'service_account_email': email})
    need(type(metadata['history_table']) is dict
         and metadata['history_table'].get('tableReference') == {'projectId': PROJECT, 'datasetId': 'firestore_export', 'tableId': 'user-activity-history'}
         and type(metadata['history_table'].get('schema')) is dict
         and type(metadata['history_table']['schema'].get('fields')) is list
         and len(metadata['history_table']['schema']['fields']) == 8)
    return {'pool': pool, 'provider': provider, 'source_capture_started_utc': value['capture_started_utc']}

def reviewed_targets(value, now=None):
    """Read an address list, never reuse old observations as current evidence."""
    current = now or utc_now()
    need(type(value) is dict and set(value) == CAPTURE_KEYS
         and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == 'private-operator-runtime-metadata-capture')
    for key in ('complete', 'lambda6_source_match', 'ecs_source_match',
                'ci_identity_read_verified', 'migration_ready'):
        need(type(value[key]) is bool)  # Shape only; no old success flag is trusted.
    start, end = timestamp(value['capture_started_utc']), timestamp(value['capture_completed_utc'])
    need(start <= end <= current)
    metadata = value['metadata']
    need(type(metadata) is dict and set(metadata) == METADATA_KEYS)
    project = metadata['project']
    need(type(project) is dict and project.get('projectId') == PROJECT
         and type(project.get('projectNumber')) is str and re.fullmatch(r'[0-9]+', project['projectNumber']))
    pool = 'projects/' + project['projectNumber'] + '/locations/global/workloadIdentityPools/aws-runtime'
    provider = pool + '/providers/aws-provider'
    need(type(metadata['pool']) is dict and metadata['pool'].get('name') == pool
         and type(metadata['provider']) is dict and metadata['provider'].get('name') == provider)
    # Do not inspect state, trust, policies, attestations, services, history or success values.
    return {'project_id': PROJECT, 'project_number': project['projectNumber'],
            'pool': pool, 'provider': provider,
            'source_capture_started_utc': value['capture_started_utc'],
            'source_capture_completed_utc': value['capture_completed_utc'],
            'source_capture_age_seconds_at_new_read': int((current - start).total_seconds())}

def observation_window(started, now=None):
    current = now or utc_now()
    need(started <= current < started + timedelta(hours=1))

def project_transport(token):
    # The additional interface is a fixed dev project GET, never a caller-selected URL.
    request = urllib.request.Request('https://cloudresourcemanager.googleapis.com/v1/projects/' + PROJECT,
        method='GET', headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
    try:
        with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
            need(response.status == 200)
            return 200, decode(response.read(MAX_BYTES + 1))
    except urllib.error.HTTPError as error:
        error.read(MAX_BYTES + 1)
        return error.code, None

def read_token():
    result = subprocess.run(['gcloud', 'auth', 'print-access-token', '--quiet'],
                            capture_output=True, text=True, timeout=60)
    token = result.stdout.strip()
    need(result.returncode == 0 and 0 < len(token) < 16384 and not any(c.isspace() for c in token))
    return token

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, newurl):
        raise Stop('redirect STOP')

def transport(token, selectors, path, body):
    pool_path = 'v1/' + selectors['pool'] + ':getIamPolicy'
    if path == pool_path:
        need(body == {'options': {'requestedPolicyVersion': 3}})
    elif path == QUERY_PATH:
        need(type(body) is dict and {'fullResourceName', 'pageSize'} <= set(body)
             and set(body) <= {'fullResourceName', 'pageSize', 'pageToken'}
             and body['fullResourceName'] in {'//iam.googleapis.com/' + selectors['pool'],
                                              '//iam.googleapis.com/' + selectors['provider']}
             and type(body['pageSize']) is int and body['pageSize'] == PAGE_SIZE
             and ('pageToken' not in body or (type(body['pageToken']) is str and 0 < len(body['pageToken']) <= 2048)))
    else:
        raise Stop('request scope STOP')
    request = urllib.request.Request('https://iam.googleapis.com/' + path,
        data=json.dumps(body, allow_nan=False).encode(), method='POST',
        headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
    try:
        with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
            need(response.status == 200)
            return 200, decode(response.read(MAX_BYTES + 1))
    except urllib.error.HTTPError as error:
        # A failed/unsupported resource is recorded only by numeric status, without its body.
        error.read(MAX_BYTES + 1)
        return error.code, None

def check_policy(value):
    need(type(value) is dict and set(value) <= {'version', 'bindings', 'etag', 'auditConfigs'}
         and type(value.get('version', 1)) is int and value.get('version', 1) in (1, 3)
         and type(value.get('bindings', [])) is list
         and type(value.get('etag', '')) is str and type(value.get('auditConfigs', [])) is list)
    for binding in value.get('bindings', []):
        need(type(binding) is dict and {'role', 'members'} <= set(binding)
             and set(binding) <= {'role', 'members', 'condition'}
             and type(binding['role']) is str and binding['role']
             and type(binding['members']) is list
             and all(type(member) is str and member for member in binding['members']))
        if 'condition' in binding:
            condition = binding['condition']
            need(value.get('version', 1) == 3 and type(condition) is dict
                 and 'expression' in condition and set(condition) <= {'expression', 'title', 'description', 'location'}
                 and all(type(text) is str for text in condition.values()))
    for config in value.get('auditConfigs', []):
        need(type(config) is dict and set(config) <= {'service', 'auditLogConfigs'}
             and type(config.get('service', '')) is str and type(config.get('auditLogConfigs', [])) is list)
        for entry in config.get('auditLogConfigs', []):
            need(type(entry) is dict and set(entry) <= {'logType', 'exemptedMembers'}
                 and entry.get('logType', 'LOG_TYPE_UNSPECIFIED') in {'LOG_TYPE_UNSPECIFIED', 'ADMIN_READ', 'DATA_WRITE', 'DATA_READ'}
                 and type(entry.get('exemptedMembers', [])) is list
                 and all(type(member) is str for member in entry.get('exemptedMembers', [])))

def permission_key(name):
    if name.startswith('iam.googleapis.com/'):
        return 'iam.' + name[len('iam.googleapis.com/'):]
    return name

def check_permission(value):
    need(type(value) is dict and 'name' in value and set(value) <= PERMISSION_KEYS)
    for key in ('name', 'title', 'description', 'primaryPermission'):
        need(key not in value or type(value[key]) is str)
    need(bool(value['name']) and re.fullmatch(r'[A-Za-z0-9_./-]+', value['name']))
    for key in ('onlyInPredefinedRoles', 'apiDisabled'):
        need(key not in value or type(value[key]) is bool)
    need(value.get('customRolesSupportLevel', 'SUPPORTED') in {'SUPPORTED', 'TESTING', 'NOT_SUPPORTED'}
         and value.get('stage', 'GA') in {'ALPHA', 'BETA', 'GA', 'DEPRECATED'})
    if value.get('primaryPermission'):
        need(re.fullmatch(r'[A-Za-z0-9_./-]+', value['primaryPermission']))

def support(permissions):
    result = {}
    for name in NEEDED:
        matches = [item for item in permissions
                   if name in {permission_key(item['name']), permission_key(item.get('primaryPermission', ''))}]
        levels = sorted({item.get('customRolesSupportLevel', 'SUPPORTED') for item in matches})
        result[name] = {'testable_on_this_scope': bool(matches),
            'matching_entries': matches, 'custom_role_support_levels': levels,
            'supported_for_custom_role': bool(matches) and levels == ['SUPPORTED']
                and all(item.get('onlyInPredefinedRoles', False) is False for item in matches)}
    return result

def query_scope(resource, request, fresh):
    permissions, names, tokens, token, statuses = [], set(), set(), '', []
    try:
        for _ in range(MAX_PAGES):
            fresh()
            body = {'fullResourceName': '//iam.googleapis.com/' + resource, 'pageSize': PAGE_SIZE}
            if token:
                body['pageToken'] = token
            status, page = request(QUERY_PATH, body)
            need(type(status) is int)
            statuses.append(status)
            if status != 200:
                return {'available': False, 'complete': False, 'page_http_statuses': statuses,
                        'failure_kind': 'http_unavailable', 'permissions': permissions,
                        'required_read_support': None}
            need(type(page) is dict and set(page) <= {'permissions', 'nextPageToken'}
                 and type(page.get('permissions', [])) is list)
            for item in page.get('permissions', []):
                check_permission(item)
                need(item['name'] not in names)
                names.add(item['name'])
                permissions.append(item)
            need(len(json.dumps(permissions, allow_nan=False).encode()) <= MAX_BYTES)
            token = page.get('nextPageToken', '')
            need(type(token) is str and len(token) <= 2048)
            if not token:
                return {'available': True, 'complete': True, 'page_http_statuses': statuses,
                        'empty': not bool(permissions), 'permissions': permissions,
                        'required_read_support': support(permissions)}
            need(token not in tokens)
            tokens.add(token)
    except Exception:
        return {'available': False, 'complete': False, 'page_http_statuses': statuses,
                'failure_kind': 'invalid_or_incomplete', 'permissions': permissions,
                'required_read_support': None}
    return {'available': False, 'complete': False, 'page_http_statuses': statuses,
            'failure_kind': 'page_limit', 'permissions': permissions, 'required_read_support': None}

def collect(selectors, request, fresh, capture_sha256):
    started = utc_now().isoformat().replace('+00:00', 'Z')
    policy = {'available': False, 'complete': False, 'http_status': None, 'policy': None}
    try:
        fresh()
        status, value = request('v1/' + selectors['pool'] + ':getIamPolicy',
                                {'options': {'requestedPolicyVersion': 3}})
        need(type(status) is int)
        policy['http_status'] = status
        if status == 200:
            check_policy(value)
            policy.update(available=True, complete=True, policy=value)
        else:
            policy['failure_kind'] = 'http_unavailable'
    except Exception:
        policy['failure_kind'] = 'invalid_or_unavailable'
    scopes = {alias: query_scope(selectors[alias], request, fresh) for alias in ('pool', 'provider')}
    fresh()
    return {'schema_version': 1, 'kind': 'private-owner-runtime-read-scope',
        'source_capture_sha256': capture_sha256,
        'source_capture_started_utc': selectors['source_capture_started_utc'],
        'capture_started_utc': started, 'capture_completed_utc': utc_now().isoformat().replace('+00:00', 'Z'),
        'complete': policy['complete'] and all(item['complete'] for item in scopes.values()),
        'pool_iam_policy': policy, 'scopeTestable': scopes,
        'interpretation': {'proves_caller_has_permissions': False, 'proves_caller_can_change_iam': False,
            'proves_pool_binding_acceptance': False, 'proves_provider_inheritance': False,
            'project_or_resource_policy_changed': False, 'migration_ready': False}}

def collect_reviewed_targets(targets, request, project_request, fresh, capture_sha256, started):
    fresh()
    status, project = project_request()
    fresh()
    # No IAM request is made until the fresh, fixed-project result matches the reviewed IDs.
    need(type(status) is int and status == 200 and type(project) is dict
         and project.get('projectId') == PROJECT and project.get('projectId') == targets['project_id']
         and project.get('projectNumber') == targets['project_number']
         and project.get('lifecycleState') == 'ACTIVE')
    captured = collect(targets, request, fresh, capture_sha256)
    captured.update(schema_version=2, kind='private-owner-runtime-read-scope-reviewed-targets',
        source_usage='reviewed-target-identifiers-only',
        source_capture_completed_utc=targets['source_capture_completed_utc'],
        source_capture_age_seconds_at_new_read=targets['source_capture_age_seconds_at_new_read'],
        capture_started_utc=started.isoformat().replace('+00:00', 'Z'),
        capture_expires_utc=(started + timedelta(hours=1)).isoformat().replace('+00:00', 'Z'),
        fresh_project={'complete': True, 'http_status': 200, 'metadata': project})
    captured['interpretation'].update(proves_ci_identity_reads=False,
        reuses_source_metadata_as_fresh=False, proves_runtime_trust_state=False)
    fresh()
    return captured

def write_capture(value):
    raw = (json.dumps(value, ensure_ascii=False, allow_nan=False) + '\n').encode()
    need(len(raw) <= MAX_BYTES)
    root = Path(tempfile.mkdtemp(prefix='yss-gcp-read-scope-', dir='/tmp'))
    root.chmod(0o700)
    path = root / OUTPUT
    try:
        fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, 'wb') as handle:
            handle.write(raw)
            handle.flush()
            os.fsync(handle.fileno())
        owned(root, directory=True)
        owned(path)
    except Exception:
        try:
            path.unlink(missing_ok=True)
            root.rmdir()
        except OSError:
            pass
        raise Stop('private output STOP') from None
    return path

def main(argv=None):
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument('--capture-sha256', required=True)
        inputs = parser.add_mutually_exclusive_group()
        inputs.add_argument('--capture-file', type=Path)
        inputs.add_argument('--reviewed-targets-file', type=Path)
        args = parser.parse_args(argv)
        need(re.fullmatch(r'[0-9a-fA-F]{64}', args.capture_sha256) is not None)
        capture_sha256 = args.capture_sha256.lower()
        if args.reviewed_targets_file is not None:
            source = read_uploaded_capture(args.reviewed_targets_file, capture_sha256)
            started = utc_now()
            selectors = reviewed_targets(source, now=started)
            def fresh():
                observation_window(started)
        elif args.capture_file is not None:
            source = read_uploaded_capture(args.capture_file, capture_sha256)
        else:
            _path, source = find_capture(capture_sha256)
        if args.reviewed_targets_file is None:
            selectors = validate_capture(source)
            def fresh():
                validate_capture(source)
        fresh()
        token = read_token()
        try:
            request = lambda path, body: transport(token, selectors, path, body)
            if args.reviewed_targets_file is None:
                captured = collect(selectors, request, fresh, capture_sha256)
            else:
                captured = collect_reviewed_targets(selectors, request, lambda: project_transport(token),
                                                     fresh, capture_sha256, started)
        finally:
            token = ''
        fresh()
        path = write_capture(captured)
        print('確認結果を非公開ファイルへ保存しました。クラウド設定は変更していません。')
        print('RESULT_FILE=' + str(path))
        if not captured['complete']:
            print('一部の読み取りは未確認です。取得できない設定を空として扱っていません。')
            return 1
        print('必要な読み取りは完了しました。権限の追加や移行の許可を確認した結果ではありません。')
        return 0
    except Exception:
        print('STOP: 入力の期限・一致、または読み取りを確認できませんでした。クラウド設定は変更していません。')
        return 1

if __name__ == '__main__':
    raise SystemExit(main())
