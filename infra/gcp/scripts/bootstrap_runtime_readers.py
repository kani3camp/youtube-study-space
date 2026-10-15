#!/usr/bin/env python3
"""Review a fixed dev reader intent offline; owner execution needs separate approval.

Standalone stdlib helper. Default: no authentication, HTTP or mutation. This is
not a general IAM tool, a CI writer, an import tool or an authorization receipt.
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
import tempfile
import urllib.error
import urllib.parse
import urllib.request

PROJECT = 'test-youtube-study-space'
REPOSITORY = 'kani3camp/youtube-study-space'
BRANCH = 'feature/gcp-terraform-iac'
WORKFLOW = '.github/workflows/gcp-terraform-authenticated.yml'
MAX_BYTES = 4 * 1024 * 1024
POOL_PERMISSIONS = ('iam.workloadIdentityPools.get',
                    'iam.workloadIdentityPools.getAttestationRules',
                    'iam.workloadIdentityPoolProviders.get')
ROLES = {
    'pool': {'roleId': 'terraformDevRuntimePoolReadV1', 'role': {
        'title': 'Terraform dev runtime pool metadata read',
        'description': 'Read the approved development runtime pool metadata; no mutations.',
        'stage': 'GA', 'includedPermissions': list(POOL_PERMISSIONS)}},
    'sa': {'roleId': 'terraformDevRuntimeSaPolicyReadV1', 'role': {
        'title': 'Terraform dev runtime SA policy read',
        'description': 'Read the approved development runtime service account IAM policy; no mutations.',
        'stage': 'GA', 'includedPermissions': ['iam.serviceAccounts.getIamPolicy']}}
}
AUTHORITY = {
    'project': ('iam.roles.create', 'iam.roles.get'),
    'pool': ('iam.workloadIdentityPools.getIamPolicy', 'iam.workloadIdentityPools.setIamPolicy'),
    'sa': ('iam.serviceAccounts.getIamPolicy', 'iam.serviceAccounts.setIamPolicy')}
CLOSED_GATES = ('DEV_HISTORY_POST_NOOP12_READY', 'DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY',
                'DEV_CURRENT_STATE_READ_ENABLED', 'DEV_RUNTIME_METADATA_READ_ENABLED',
                'DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED', 'DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED',
                'DEV_RUNTIME_OWNERSHIP_SPLIT_INPUT_ENABLED')
EMPTY_TIMES = ('DEV_CURRENT_STATE_READ_ISSUED_UTC', 'DEV_CURRENT_STATE_READ_EXPIRES_UTC',
               'DEV_RUNTIME_METADATA_READ_ISSUED_UTC', 'DEV_RUNTIME_METADATA_READ_EXPIRES_UTC')
ACTIVE_RUN_STATES = ('queued', 'in_progress', 'waiting', 'pending', 'requested')
MANIFEST_KEYS = frozenset({'schema_version', 'kind', 'source_usage', 'reviewed_capture_sha256',
    'project_id', 'project_number', 'runtime_pool_resource', 'runtime_provider_resource',
    'runtime_iam_policy_resource', 'ci_plan_service_account', 'ci_apply_service_account',
    'reviewed_revision', 'script_sha256', 'workflow_sha256', 'issued_utc', 'expires_utc',
    'owner_session_scope', 'owner_exception_approved', 'known_iam_writers_quiescent',
    'allow_empty_pool_policy_initialization', 'role_creations', 'binding_additions'})


class Stop(ValueError):
    pass


class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise Stop('bootstrap arguments STOP')


def need(value):
    if not value:
        raise Stop('runtime reader bootstrap STOP')


def now():
    return datetime.now(timezone.utc)


def stamp(value):
    need(type(value) is str and re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ', value))
    return datetime.fromisoformat(value[:-1] + '+00:00')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def digest(value):
    need(type(value) is str and re.fullmatch(r'[0-9a-f]{64}', value))
    return value


def unique(pairs):
    result = {}
    for key, value in pairs:
        need(key not in result)
        result[key] = value
    return result


def decode(raw):
    need(type(raw) is bytes and 0 < len(raw) <= MAX_BYTES)
    value = json.loads(raw, object_pairs_hook=unique,
                       parse_constant=lambda _: need(False))
    need(type(value) is dict)
    return value


def encoded(value):
    raw = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode()
    need(0 < len(raw) <= MAX_BYTES)
    return raw


def private_input(path, expected_hash):
    digest(expected_hash)
    need(path.is_absolute() and path == path.resolve() and not path.is_symlink())
    parent = path.parent.lstat()
    need(stat.S_ISDIR(parent.st_mode) and parent.st_uid == os.geteuid()
         and parent.st_mode & 0o022 == 0)
    for directory in (path.parent, *path.parent.parents):
        marker = directory / '.git'
        need(not (marker.is_file() or (marker / 'HEAD').is_file()))
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        info = os.fstat(handle.fileno())
        mode = stat.S_IMODE(info.st_mode)
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
             and info.st_nlink == 1 and mode & 0o700 == 0o600 and mode & 0o7022 == 0)
        os.fchmod(handle.fileno(), 0o600)  # Uploaded 0644 files become private through their fd.
        raw = handle.read(MAX_BYTES + 1)
    need(sha(raw) == expected_hash)
    return decode(raw)


def fixed_members():
    return ['serviceAccount:terraform-dev-' + identity + '@' + PROJECT + '.iam.gserviceaccount.com'
            for identity in ('plan', 'apply')]


def role_name(alias):
    return 'projects/' + PROJECT + '/roles/' + ROLES[alias]['roleId']


def additions():
    return [{'resource_alias': alias, 'role': role_name(alias), 'members': fixed_members(),
             'condition': None} for alias in ('pool', 'sa')]


def validate_manifest(value):
    need(type(value) is dict and set(value) == MANIFEST_KEYS
         and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == 'private-owner-runtime-reader-bootstrap-intent'
         and value['source_usage'] == 'reviewed-target-identifiers-only'
         and value['project_id'] == PROJECT and type(value['project_number']) is str
         and re.fullmatch(r'[0-9]{1,20}', value['project_number']))
    pool = 'projects/' + value['project_number'] + '/locations/global/workloadIdentityPools/aws-runtime'
    need(value['runtime_pool_resource'] == pool
         and value['runtime_provider_resource'] == pool + '/providers/aws-provider'
         and value['runtime_iam_policy_resource'] == 'projects/' + PROJECT
             + '/serviceAccounts/' + PROJECT + '@appspot.gserviceaccount.com'
         and value['ci_plan_service_account'] == fixed_members()[0].split(':', 1)[1]
         and value['ci_apply_service_account'] == fixed_members()[1].split(':', 1)[1]
         and type(value['reviewed_revision']) is str
         and re.fullmatch(r'[0-9a-f]{40}', value['reviewed_revision'])
         and value['role_creations'] == ROLES and value['binding_additions'] == additions()
         and value['owner_session_scope'] == 'current-normal-gcloud-user')
    for key in ('reviewed_capture_sha256', 'script_sha256', 'workflow_sha256'):
        digest(value[key])
    for key in ('owner_exception_approved', 'known_iam_writers_quiescent',
                'allow_empty_pool_policy_initialization'):
        need(type(value[key]) is bool)
    issued, expires = stamp(value['issued_utc']), stamp(value['expires_utc'])
    need(issued < expires <= issued + timedelta(hours=8))
    return copy.deepcopy(value)


def check_window(manifest, started):
    current = now()
    need(stamp(manifest['issued_utc']) <= current < stamp(manifest['expires_utc'])
         and started <= current < started + timedelta(hours=1))


def source_hash():
    path = Path(__file__)
    need(not path.is_symlink())
    return sha(path.read_bytes())


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise Stop('redirect STOP')


def http(host, path, *, method='GET', body=None, token=None, raw=False):
    need(host in {'iam.googleapis.com', 'cloudresourcemanager.googleapis.com',
                  'api.github.com', 'raw.githubusercontent.com'}
         and path.startswith('/') and not any(c in path for c in ('\r', '\n', '#')))
    need((token is None and method == 'GET' and body is None
          and host in {'api.github.com', 'raw.githubusercontent.com'})
         or (token is not None and host in {'iam.googleapis.com', 'cloudresourcemanager.googleapis.com'}
             and method in {'GET', 'POST'}))
    headers = {'Accept': 'application/json', 'User-Agent': 'yss-runtime-reader-bootstrap'}
    if token is not None:
        headers.update(Authorization='Bearer ' + token, **{'Content-Type': 'application/json'})
    request = urllib.request.Request('https://' + host + path, method=method, headers=headers,
        data=None if body is None else encoded(body))
    try:
        with urllib.request.build_opener(NoRedirect()).open(request, timeout=30) as response:
            need(response.status == 200)
            length = response.headers.get('Content-Length')
            need(length is None or re.fullmatch(r'[0-9]+', length))
            need(length is None or int(length) <= MAX_BYTES)
            content = response.read(MAX_BYTES + 1)
            need(0 < len(content) <= MAX_BYTES and (length is None or len(content) == int(length)))
            return 200, content if raw else decode(content)
    except urllib.error.HTTPError as error:
        code = error.code
        error.close()  # Error body/message/identity are neither read nor logged.
        return code, None


def check_github(manifest, request=http):
    revision = manifest['reviewed_revision']
    status, value = request('api.github.com', '/repos/' + REPOSITORY + '/git/ref/heads/' + BRANCH)
    need(status == 200 and type(value) is dict and value.get('ref') == 'refs/heads/' + BRANCH
         and type(value.get('object')) is dict and value['object'].get('type') == 'commit'
         and value['object'].get('sha') == revision)
    status, raw = request('raw.githubusercontent.com', '/' + REPOSITORY + '/' + revision + '/' + WORKFLOW, raw=True)
    need(status == 200 and type(raw) is bytes and sha(raw) == manifest['workflow_sha256'])
    text = raw.decode('utf-8')
    for key, expected in [(key, '"false"') for key in CLOSED_GATES] + [(key, '""') for key in EMPTY_TIMES]:
        lines = re.findall(r'^([ \t]+)' + key + r':[ \t]*(.*?)[ \t]*$', text, re.MULTILINE)
        need(lines == [('  ', expected)])
    need(re.search(r'^env:\s*$', text, re.MULTILINE) is not None)
    # No protected run retry or cancellation. Zero active dispatches must be
    # established separately for every documented active run state.
    for state in ACTIVE_RUN_STATES:
        query = urllib.parse.urlencode({'branch': BRANCH, 'event': 'workflow_dispatch',
                                       'status': state, 'per_page': 1})
        status, value = request('api.github.com', '/repos/' + REPOSITORY + '/actions/workflows/ci.yml/runs?' + query)
        need(status == 200 and type(value) is dict and set(value) == {'total_count', 'workflow_runs'}
             and type(value['total_count']) is int and value['total_count'] == 0
             and value['workflow_runs'] == [])
    # This confirms known repository activity, never all possible IAM writers.


def owner_token():
    need(not any(value for key, value in os.environ.items() if key.startswith('CLOUDSDK_AUTH_'))
         and not any(os.environ.get(key) for key in ('GOOGLE_APPLICATION_CREDENTIALS',
                     'GOOGLE_GHA_CREDS_PATH', 'GCP_SMOKE_ACCESS_TOKEN')))
    environment = dict(os.environ, CLOUDSDK_CORE_LOG_HTTP='false',
                       CLOUDSDK_CORE_DISABLE_PROMPTS='1', CLOUDSDK_CORE_VERBOSITY='none')
    def cli(arguments):
        result = subprocess.run(['gcloud', *arguments, '--quiet', '--verbosity=none'],
                                capture_output=True, text=True, timeout=60, env=environment)
        need(result.returncode == 0 and len(result.stdout) <= MAX_BYTES)
        return result.stdout.strip()
    active = json.loads(cli(['auth', 'list', '--filter=status:ACTIVE', '--format=json']), object_pairs_hook=unique)
    need(type(active) is list and len(active) == 1 and type(active[0]) is dict
         and set(active[0]) == {'account', 'status'} and active[0]['status'] == 'ACTIVE')
    account = active[0]['account']
    need(type(account) is str and re.fullmatch(r'[A-Za-z0-9.!#$%&\x27*+/=?^_`{|}~-]+@[A-Za-z0-9.-]+', account)
         and not account.lower().endswith('.gserviceaccount.com'))
    for setting in ('impersonate_service_account', 'credential_file_override', 'access_token_file', 'login_config_file'):
        need(cli(['config', 'get-value', 'auth/' + setting]) in {'', '(unset)'})
    token = cli(['auth', 'print-access-token', '--account=' + account])
    need(0 < len(token) < 16384 and not any(c.isspace() for c in token))
    # Account selection is not an independent proof of the human's identity.
    return token


def policy(value, *, empty_pool=False):
    need(type(value) is dict and set(value) <= {'version', 'bindings', 'etag', 'auditConfigs'}
         and type(value.get('version', 1)) is int and value.get('version', 1) in {1, 3}
         and type(value.get('bindings', [])) is list)
    if 'etag' in value:
        need(type(value['etag']) is str and 0 < len(value['etag']) <= 4096)
    if not value.get('etag'):
        need(empty_pool and value == {})
    seen = set()
    for binding in value.get('bindings', []):
        need(type(binding) is dict and {'role', 'members'} <= set(binding)
             and set(binding) <= {'role', 'members', 'condition'}
             and type(binding['role']) is str and bool(binding['role'])
             and type(binding['members']) is list and bool(binding['members'])
             and all(type(member) is str and bool(member) for member in binding['members'])
             and len(binding['members']) == len(set(binding['members'])))
        if 'condition' in binding:
            condition = binding['condition']
            need(value.get('version') == 3 and bool(value.get('etag'))
                 and type(condition) is dict and {'title', 'expression'} <= set(condition)
                 and set(condition) <= {'title', 'description', 'expression', 'location'}
                 and all(type(item) is str for item in condition.values())
                 and condition['title'] and condition['expression'])
        key = (binding['role'], json.dumps(binding.get('condition'), sort_keys=True))
        need(key not in seen)
        seen.add(key)
    if 'auditConfigs' in value:
        need(type(value['auditConfigs']) is list)
        services = set()
        for audit in value['auditConfigs']:
            need(type(audit) is dict and set(audit) == {'service', 'auditLogConfigs'}
                 and type(audit['service']) is str and audit['service'] not in services
                 and type(audit['auditLogConfigs']) is list)
            services.add(audit['service'])
            log_types = set()
            for config in audit['auditLogConfigs']:
                need(type(config) is dict and {'logType'} <= set(config) <= {'logType', 'exemptedMembers'}
                     and config['logType'] in {'ADMIN_READ', 'DATA_READ', 'DATA_WRITE'}
                     and config['logType'] not in log_types
                     and type(config.get('exemptedMembers', [])) is list
                     and all(type(member) is str and member for member in config.get('exemptedMembers', []))
                     and len(config.get('exemptedMembers', [])) == len(set(config.get('exemptedMembers', []))))
                log_types.add(config['logType'])
    return copy.deepcopy(value)


def comparable_policy(value):
    result = copy.deepcopy(value)
    result.pop('etag', None)
    # Responses may downgrade unconditional version3 to version1.
    if not any('condition' in binding for binding in result.get('bindings', [])):
        result.pop('version', None)
    result.setdefault('bindings', [])
    for binding in result['bindings']:
        binding['members'].sort()
    result['bindings'].sort(key=lambda b: json.dumps(b, sort_keys=True))
    return result


def new_policy(before, alias):
    need(not any(binding['role'] == role_name(alias) for binding in before.get('bindings', [])))
    result = copy.deepcopy(before)
    result.setdefault('bindings', []).append({'role': role_name(alias), 'members': fixed_members()})
    result['version'] = 3
    encoded(result)
    return result


def validate_role(value, alias):
    expected = ROLES[alias]['role']
    need(type(value) is dict and set(value) <= {'name', 'title', 'description', 'stage', 'includedPermissions', 'etag', 'deleted'}
         and value.get('name') == role_name(alias) and value.get('deleted', False) is False
         and all(value.get(key) == expected[key] for key in ('title', 'description', 'stage'))
         and type(value.get('includedPermissions')) is list
         and sorted(value['includedPermissions']) == sorted(expected['includedPermissions'])
         and type(value.get('etag')) is str and bool(value['etag']))


class Evidence:
    def __init__(self):
        self.root = Path(tempfile.mkdtemp(prefix='yss-runtime-reader-bootstrap-'))
        self.root.chmod(0o700)
        self.fd = os.open(self.root / 'journal.jsonl', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)

    def event(self, value):
        raw = encoded(value) + b'\n'
        need(os.write(self.fd, raw) == len(raw))
        os.fsync(self.fd)

    def save(self, name, value):
        need(re.fullmatch(r'[a-z0-9-]+\.json', name))
        raw = encoded(value)
        fd = os.open(self.root / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, 'wb') as handle:
            handle.write(raw)
            handle.flush()
            os.fsync(handle.fileno())
        return sha(raw)

    def close(self):
        os.close(self.fd)


class Bootstrap:
    def __init__(self, manifest, evidence, *, execute=False, no_cas=False,
                 request=http, github=check_github, token=None, started=None):
        self.manifest = manifest
        self.evidence = evidence
        self.execute = execute
        self.no_cas = no_cas
        self.request = request
        self.github = github
        self.token = token
        self.started = started or now()
        self.prepared = False
        self.attempts = 0
        self.completed = []
        self.before = None

    def check(self):
        check_window(self.manifest, self.started)

    def save(self, name, value):
        self.check()
        return self.evidence.save(name, value)

    def route(self, alias):
        pool = 'v1/' + self.manifest['runtime_pool_resource']
        sa = 'v1/' + self.manifest['runtime_iam_policy_resource']
        routes = {
            'project': ('cloudresourcemanager.googleapis.com', '/v1/projects/' + PROJECT, 'GET', None),
            'pool': ('iam.googleapis.com', '/' + pool, 'GET', None),
            'provider': ('iam.googleapis.com', '/v1/' + self.manifest['runtime_provider_resource'], 'GET', None),
            'attestation': ('iam.googleapis.com', '/' + pool + ':listAttestationRules', 'GET', None),
            'pool-policy': ('iam.googleapis.com', '/' + pool + ':getIamPolicy', 'POST', {'options': {'requestedPolicyVersion': 3}}),
            'sa-policy': ('iam.googleapis.com', '/' + sa + ':getIamPolicy?options.requestedPolicyVersion=3', 'POST', None),
            'role-pool-get': ('iam.googleapis.com', '/v1/' + role_name('pool'), 'GET', None),
            'role-sa-get': ('iam.googleapis.com', '/v1/' + role_name('sa'), 'GET', None),
            'role-pool-create': ('iam.googleapis.com', '/v1/projects/' + PROJECT + '/roles', 'POST', ROLES['pool']),
            'role-sa-create': ('iam.googleapis.com', '/v1/projects/' + PROJECT + '/roles', 'POST', ROLES['sa'])}
        for resource, path, host in [('project', 'v3/projects/' + PROJECT, 'cloudresourcemanager.googleapis.com'),
                                     ('pool', pool, 'iam.googleapis.com'), ('sa', sa, 'iam.googleapis.com')]:
            routes['authority-' + resource] = (host, '/' + path + ':testIamPermissions', 'POST', {'permissions': list(AUTHORITY[resource])})
        need(alias in routes)
        return copy.deepcopy(routes[alias])

    def call(self, alias, *, absent=False):
        self.check()
        host, path, method, body = self.route(alias)
        need('-create' not in alias)
        status, value = self.request(host, path, method=method, body=body, token=self.token)
        self.check()
        if absent:
            need(status == 404 and value is None)
            return None
        need(status == 200 and type(value) is dict)
        return value

    def metadata(self):
        project = self.call('project')
        need(set(project) <= {'projectId', 'projectNumber', 'lifecycleState', 'name', 'createTime', 'labels', 'parent'}
             and project.get('projectId') == PROJECT and project.get('projectNumber') == self.manifest['project_number']
             and project.get('lifecycleState') == 'ACTIVE')
        need(all(type(project[key]) is str for key in ('name', 'createTime') if key in project))
        if 'labels' in project:
            need(type(project['labels']) is dict
                 and all(type(key) is str and type(item) is str for key, item in project['labels'].items()))
        if 'parent' in project:
            need(type(project['parent']) is dict and set(project['parent']) == {'type', 'id'}
                 and all(type(item) is str for item in project['parent'].values()))
        pool = self.call('pool')
        provider = self.call('provider')
        for value, resource in ((pool, self.manifest['runtime_pool_resource']),
                                (provider, self.manifest['runtime_provider_resource'])):
            need(value.get('name') == resource and value.get('state') == 'ACTIVE'
                 and value.get('disabled', False) is False
                 and all(type(value[key]) is str for key in ('displayName', 'description') if key in value))
        need(set(pool) <= {'name', 'state', 'disabled', 'displayName', 'description', 'mode'}
             and pool.get('mode') in (None, '', 'FEDERATION_ONLY'))
        need(set(provider) <= {'name', 'state', 'disabled', 'displayName', 'description', 'aws', 'attributeMapping', 'attributeCondition'}
             and type(provider.get('aws')) is dict and set(provider['aws']) == {'accountId'}
             and type(provider['aws']['accountId']) is str and re.fullmatch(r'[0-9]{12}', provider['aws']['accountId'])
             and type(provider.get('attributeMapping')) is dict
             and {'google.subject', 'attribute.aws_role', 'attribute.account'} <= set(provider['attributeMapping'])
             and all(type(key) is str and type(item) is str and item.strip() for key, item in provider['attributeMapping'].items())
             and type(provider.get('attributeCondition')) is str and provider['attributeCondition'].strip())
        attestation = self.call('attestation')
        need(set(attestation) <= {'attestationRules', 'nextPageToken'}
             and attestation.get('attestationRules', []) == []
             and type(attestation.get('attestationRules', [])) is list
             and attestation.get('nextPageToken', '') == ''
             and type(attestation.get('nextPageToken', '')) is str)
        return {'project': project, 'pool': pool, 'provider': provider, 'attestation': attestation}

    def prepare(self):
        self.prepared = False
        self.check()
        self.github(self.manifest)
        self.check()
        fresh = self.metadata()  # Fixed dev project identity is verified first.
        for resource, permissions in AUTHORITY.items():
            value = self.call('authority-' + resource)
            need(set(value) <= {'permissions'} and type(value.get('permissions', [])) is list
                 and sorted(value.get('permissions', [])) == sorted(permissions))
        for alias in ROLES:
            self.call('role-' + alias + '-get', absent=True)
        fresh['pool_policy'] = policy(self.call('pool-policy'), empty_pool=True)
        fresh['sa_policy'] = policy(self.call('sa-policy'))
        for alias in ROLES:
            new_policy(fresh[alias + '_policy'], alias)
        self.check()
        self.save('preflight.json', fresh)
        self.save('dry-plan.json', {'role_creations': ROLES, 'binding_additions': additions(),
            'maximum_write_attempts': 4, 'provider_inheritance_verified': False,
            'pool_cas_available': bool(fresh['pool_policy'].get('etag')),
            'all_writers_quiescence_proven': False})
        self.before = fresh
        self.prepared = True
        return fresh

    def write(self, alias, body):
        self.check()
        sequence = ('role-pool-create', 'role-sa-create', 'pool-set', 'sa-set')
        need(self.execute and self.prepared and self.attempts < 4
             and len(self.completed) == self.attempts and alias == sequence[self.attempts]
             and self.manifest['owner_exception_approved'] is True
             and self.manifest['known_iam_writers_quiescent'] is True)
        self.github(self.manifest)
        self.check()
        if alias.startswith('role-'):
            target = alias.split('-')[1]
            need(alias == 'role-' + target + '-create' and body == ROLES[target])
            host, path, method, _ = self.route(alias)
        else:
            need(alias in {'pool-set', 'sa-set'})
            target = alias.split('-')[0]
            expected = new_policy(self.before[target + '_policy'], target)
            need(body == {'policy': expected, 'updateMask': 'bindings,etag'})
            current = policy(self.call(target + '-policy'), empty_pool=(target == 'pool'))
            need(current == self.before[target + '_policy'])
            if target == 'pool' and current == {}:
                need(self.no_cas and self.manifest['allow_empty_pool_policy_initialization']
                     and self.call('pool-policy') == current)
                # This is an expressly reviewed initial write, not a CAS lock.
            host, method = 'iam.googleapis.com', 'POST'
            resource = self.manifest['runtime_pool_resource' if target == 'pool' else 'runtime_iam_policy_resource']
            path = '/v1/' + resource + ':setIamPolicy'
        # Count and durably record the attempt BEFORE sending. A lost response
        # is UNKNOWN, never grounds for a retry or a fifth write.
        self.attempts += 1
        self.evidence.event({'event': 'write-attempt', 'alias': alias, 'attempt': self.attempts})
        try:
            status, value = self.request(host, path, method=method, body=copy.deepcopy(body), token=self.token)
            self.evidence.event({'event': 'write-http', 'alias': alias, 'http_status': status})
            self.check()
            need(status == 200 and type(value) is dict)
            self.save('write-' + str(self.attempts) + '.json', value)
            if alias.startswith('role-'):
                validate_role(value, target)
            else:
                policy(value, empty_pool=False)
                need(comparable_policy(value) == comparable_policy(body['policy']))
            self.completed.append(alias)
            self.evidence.event({'event': 'write-verified', 'alias': alias})
        except Exception:
            self.evidence.event({'event': 'write-unconfirmed', 'alias': alias})
            raise Stop('write STOP') from None

    def run(self):
        need(self.prepared)
        self.check()
        need(self.execute and self.manifest['owner_exception_approved']
             and self.manifest['known_iam_writers_quiescent'])
        empty = self.before['pool_policy'] == {}
        need(not empty or (self.no_cas and self.manifest['allow_empty_pool_policy_initialization']))
        for alias in ROLES:
            self.write('role-' + alias + '-create', copy.deepcopy(ROLES[alias]))
        for alias in ROLES:
            # write() rechecks gates/activity, then reads the exact policy
            # immediately before its only SET attempt. Never merge a new drift.
            current = self.before[alias + '_policy']
            self.write(alias + '-set', {'policy': new_policy(current, alias), 'updateMask': 'bindings,etag'})
            after = policy(self.call(alias + '-policy'))
            need(comparable_policy(after) == comparable_policy(new_policy(current, alias)))
            self.save('after-' + alias + '-policy.json', after)
        after = self.metadata()
        need(after == {key: self.before[key] for key in ('project', 'pool', 'provider', 'attestation')})
        for alias in ROLES:
            validate_role(self.call('role-' + alias + '-get'), alias)
        self.check()
        self.save('after-metadata.json', after)


def main(argv=None):
    invocation_started = now()
    evidence = session = None
    mode = 'offline'
    success = False
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument('--manifest-file', type=Path, required=True)
        parser.add_argument('--manifest-sha256', required=True)
        parser.add_argument('--source-sha256')
        modes = parser.add_mutually_exclusive_group()
        modes.add_argument('--live-plan', action='store_true')
        modes.add_argument('--execute-owner-exception', action='store_true')
        parser.add_argument('--accept-empty-pool-no-cas', action='store_true')
        args = parser.parse_args(argv)
        manifest = validate_manifest(private_input(args.manifest_file, args.manifest_sha256))
        mode = 'execute' if args.execute_owner_exception else 'live-plan' if args.live_plan else 'offline'
        need(not args.accept_empty_pool_no_cas or args.execute_owner_exception)
        if mode != 'offline':
            need(digest(args.source_sha256) == manifest['script_sha256'] == source_hash())
            check_window(manifest, invocation_started)
        if mode == 'execute':
            need(manifest['owner_exception_approved'] and manifest['known_iam_writers_quiescent'])
        evidence = Evidence()
        evidence.event({'event': 'start', 'mode': mode, 'manifest_sha256': args.manifest_sha256,
                        'script_sha256': source_hash()})
        if mode == 'offline':
            evidence.save('offline-intent.json', {'role_creations': ROLES,
                'binding_additions': [{'resource_alias': item['resource_alias'], 'member_count': 2}
                                      for item in additions()], 'maximum_write_attempts': 4,
                'cloud_observed': False, 'execution_ready': False})
        else:
            # Inspect known repository gates/activity before requesting a token.
            check_github(manifest)
            token = owner_token()
            try:
                session = Bootstrap(manifest, evidence, execute=args.execute_owner_exception,
                    no_cas=args.accept_empty_pool_no_cas, token=token, started=invocation_started)
                session.prepare()
                if args.execute_owner_exception:
                    session.run()
            finally:
                token = None
                if session is not None:
                    session.token = None
        success = True
    except Exception:
        pass  # Parser/dependency/API errors must never echo private input.
    finally:
        if evidence is not None:
            try:
                evidence.event({'event': 'finish', 'status': 'PASS' if success else 'STOP',
                    'mode': mode, 'write_attempts': session.attempts if session else 0,
                    'verified_write_aliases': session.completed if session else [],
                    'setup_complete': success and mode == 'execute',
                    'ci_identity_read_verified': False, 'provider_inheritance_verified': False,
                    'owner_identity_independently_verified': False,
                    'all_writers_quiescence_proven': False, 'migration_ready': False})
            except Exception:
                success = False
            try:
                evidence.close()
            except Exception:
                success = False
        print('runtime-reader-bootstrap: ' + ('PASS' if success else 'STOP'))
        print('runtime-reader-bootstrap-mode: ' + mode)
        print('runtime-reader-bootstrap-write-attempts: ' + str(session.attempts if session else 0))
        print('runtime-reader-bootstrap-migration-ready: false')
        if evidence is not None:
            print('RESULT_DIRECTORY=' + str(evidence.root))
    return 0 if success else 1


if __name__ == '__main__':
    raise SystemExit(main())
