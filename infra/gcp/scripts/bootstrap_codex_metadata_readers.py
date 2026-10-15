#!/usr/bin/env python3
"""Review a fixed dev metadata-reader extension; default is entirely offline.

The adjacent, hash-pinned bootstrap_runtime_readers.py is an unchanged dependency.
No credentials, selectors or owner approval are distributed with this source.
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
import types
import urllib.error
import urllib.request

BASE_SHA256 = 'afc5feaf75365c4777c698d9ef64ce5b6f50547310a5d47c9dabc49d1030ac85'
PROJECT = 'test-youtube-study-space'
TABLE = 'projects/' + PROJECT + '/datasets/firestore_export/tables/user-activity-history'
HISTORY_ROLE = {'roleId': 'terraformDevHistoryMetadataReadV1', 'role': {
    'title': 'Terraform dev retained history metadata read',
    'description': 'Read metadata of the approved retained development history table; no rows or mutations.',
    'stage': 'GA', 'includedPermissions': ['bigquery.tables.get']}}
MAX_BYTES = 4 * 1024 * 1024
MANIFEST_KEYS = frozenset({'schema_version', 'kind', 'source_usage',
    'reviewed_capture_sha256', 'reviewed_metadata_identity_sha256', 'project_id', 'project_number',
    'runtime_pool_resource', 'runtime_provider_resource', 'runtime_iam_policy_resource',
    'metadata_service_account_email', 'metadata_service_account_resource', 'history_table_resource',
    'ci_plan_service_account', 'ci_apply_service_account', 'existing_role_definitions',
    'role_creation', 'binding_additions', 'reviewed_revision', 'script_sha256',
    'base_script_sha256', 'workflow_sha256', 'issued_utc', 'expires_utc',
    'owner_session_scope', 'owner_route_acknowledged', 'known_iam_writers_quiescent'})


class Stop(ValueError):
    pass


def need(value):
    if not value:
        raise Stop('metadata reader extension STOP')


class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise Stop('metadata reader arguments STOP')


def now():
    return datetime.now(timezone.utc)


def utc():
    return now().isoformat().replace('+00:00', 'Z')


def source_hash():
    path = Path(__file__)
    need(not path.is_symlink())
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_base():
    # Execute only the bytes just hashed, never import an unchecked local module
    # or reopen the dependency after the hash check.
    path = Path(__file__).resolve().with_name('bootstrap_runtime_readers.py')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        info = os.fstat(handle.fileno())
        need(stat.S_ISREG(info.st_mode))
        raw = handle.read(MAX_BYTES + 1)
    need(0 < len(raw) <= MAX_BYTES and hashlib.sha256(raw).hexdigest() == BASE_SHA256)
    module = types.ModuleType('pinned_runtime_reader_bootstrap')
    module.__file__ = str(path)
    exec(compile(raw, str(path), 'exec'), module.__dict__)
    return module


def history_role_name():
    return 'projects/' + PROJECT + '/roles/' + HISTORY_ROLE['roleId']


def additions(base, email):
    member = 'serviceAccount:' + email
    return [{'resource_alias': alias,
             'role': history_role_name() if alias == 'history' else base.role_name(alias),
             'members_to_add': [member], 'condition': None}
            for alias in ('pool', 'sa', 'history')]


def validate_manifest(base, value):
    need(type(value) is dict and set(value) == MANIFEST_KEYS
         and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == 'private-owner-codex-metadata-reader-extension-intent'
         and value['source_usage'] == 'reviewed-target-identifiers-only'
         and value['project_id'] == PROJECT and type(value['project_number']) is str
         and re.fullmatch(r'[0-9]{1,20}', value['project_number']))
    pool = 'projects/' + value['project_number'] + '/locations/global/workloadIdentityPools/aws-runtime'
    runtime_sa = PROJECT + '@appspot.gserviceaccount.com'
    email = value['metadata_service_account_email']
    need(type(email) is str and re.fullmatch(r'[a-z][a-z0-9-]{4,28}[a-z0-9]@'
         + re.escape(PROJECT) + r'\.iam\.gserviceaccount\.com', email)
         and email not in [member.split(':', 1)[1] for member in base.fixed_members()] + [runtime_sa]
         and value['metadata_service_account_resource'] == 'projects/' + PROJECT + '/serviceAccounts/' + email
         and value['runtime_pool_resource'] == pool
         and value['runtime_provider_resource'] == pool + '/providers/aws-provider'
         and value['runtime_iam_policy_resource'] == 'projects/' + PROJECT + '/serviceAccounts/' + runtime_sa
         and value['history_table_resource'] == TABLE
         and value['ci_plan_service_account'] == base.fixed_members()[0].split(':', 1)[1]
         and value['ci_apply_service_account'] == base.fixed_members()[1].split(':', 1)[1]
         and value['existing_role_definitions'] == base.ROLES and value['role_creation'] == HISTORY_ROLE
         and value['binding_additions'] == additions(base, email)
         and value['base_script_sha256'] == BASE_SHA256
         and value['owner_session_scope'] == 'current-normal-gcloud-user'
         and type(value['reviewed_revision']) is str and re.fullmatch(r'[0-9a-f]{40}', value['reviewed_revision']))
    for key in ('reviewed_capture_sha256', 'reviewed_metadata_identity_sha256',
                'script_sha256', 'base_script_sha256', 'workflow_sha256'):
        base.digest(value[key])
    for key in ('owner_route_acknowledged', 'known_iam_writers_quiescent'):
        need(type(value[key]) is bool)
    issued, expires = base.stamp(value['issued_utc']), base.stamp(value['expires_utc'])
    need(issued < expires <= issued + timedelta(hours=8))
    return copy.deepcopy(value)


def check_window(base, manifest, started):
    current = now()
    need(base.stamp(manifest['issued_utc']) <= current < base.stamp(manifest['expires_utc'])
         and started <= current < started + timedelta(hours=1))


def planned_policy(base, before, alias, email):
    before = base.policy(before)  # All three etags are mandatory, including Pool.
    result = copy.deepcopy(before)
    role = history_role_name() if alias == 'history' else base.role_name(alias)
    matches = [binding for binding in result.get('bindings', []) if binding['role'] == role]
    member = 'serviceAccount:' + email
    if alias == 'history':
        need(not matches)
        result.setdefault('bindings', []).append({'role': role, 'members': [member]})
    else:
        need(len(matches) == 1 and 'condition' not in matches[0]
             and sorted(matches[0]['members']) == sorted(base.fixed_members()))
        matches[0]['members'].append(member)
    result['version'] = 3
    base.encoded(result)
    return result


def validate_history_role(base, value):
    need(type(value) is dict and set(value) <= {'name', 'title', 'description', 'stage', 'includedPermissions', 'etag', 'deleted'}
         and value.get('name') == history_role_name() and value.get('deleted', False) is False
         and all(value.get(key) == expected for key, expected in HISTORY_ROLE['role'].items())
         and type(value.get('etag')) is str and bool(value['etag']))


def bq_http(base, path, *, method, body, token):
    # No arbitrary host/path/verb fallback. These are the exact table's IAM APIs.
    need(path in {'/bigquery/v2/' + TABLE + ':' + action
                  for action in ('getIamPolicy', 'setIamPolicy', 'testIamPermissions')}
         and method == 'POST' and type(body) is dict and type(token) is str and bool(token))
    request = urllib.request.Request('https://bigquery.googleapis.com' + path, method='POST',
        headers={'Accept': 'application/json', 'Content-Type': 'application/json',
                 'User-Agent': 'yss-codex-metadata-reader-bootstrap', 'Authorization': 'Bearer ' + token},
        data=base.encoded(body))
    try:
        with urllib.request.build_opener(base.NoRedirect()).open(request, timeout=30) as response:
            need(response.status == 200)
            length = response.headers.get('Content-Length')
            need(length is None or re.fullmatch(r'[0-9]+', length))
            need(length is None or int(length) <= MAX_BYTES)
            raw = response.read(MAX_BYTES + 1)
            need(0 < len(raw) <= MAX_BYTES and (length is None or len(raw) == int(length)))
            return 200, base.decode(raw)
    except urllib.error.HTTPError as error:
        status = error.code
        error.close()  # Never read/save/print error bodies, messages or identities.
        return status, None


class Evidence:
    def __init__(self, base):
        self.base = base
        self.inner = base.Evidence()
        self.root = self.inner.root

    def event(self, value):
        self.inner.event(dict(value, recorded_utc=utc(), timestamp_source='local-clock-self-report'))

    def save(self, name, value):
        return self.inner.save(name, value)

    def close(self):
        self.inner.close()


class Bootstrap:
    def __init__(self, base, manifest, evidence, *, execute=False, request=None,
                 github=None, token=None, started=None):
        self.base, self.manifest, self.evidence = base, manifest, evidence
        self.execute, self.token = execute, token
        self.started = started or now()
        self.request = request or self.transport
        self.github = github or base.check_github
        self.prepared = False
        self.attempts = 0
        self.completed = []
        self.before = None

    def transport(self, host, path, *, method='GET', body=None, token=None):
        if host == 'bigquery.googleapis.com':
            return bq_http(self.base, path, method=method, body=body, token=token)
        return self.base.http(host, path, method=method, body=body, token=token)

    def check(self):
        check_window(self.base, self.manifest, self.started)

    def event(self, value):
        self.evidence.event(value)

    def save(self, name, value):
        self.check()
        result = self.evidence.save(name, value)
        self.check()
        return result

    def route(self, alias):
        pool = '/v1/' + self.manifest['runtime_pool_resource']
        sa = '/v1/' + self.manifest['runtime_iam_policy_resource']
        table = '/bigquery/v2/' + TABLE
        routes = {
            'project': ('cloudresourcemanager.googleapis.com', '/v1/projects/' + PROJECT, 'GET', None),
            'metadata-sa': ('iam.googleapis.com', '/v1/' + self.manifest['metadata_service_account_resource'], 'GET', None),
            'role-history-get': ('iam.googleapis.com', '/v1/' + history_role_name(), 'GET', None),
            'role-history-create': ('iam.googleapis.com', '/v1/projects/' + PROJECT + '/roles', 'POST', HISTORY_ROLE),
            'pool-policy': ('iam.googleapis.com', pool + ':getIamPolicy', 'POST', {'options': {'requestedPolicyVersion': 3}}),
            'sa-policy': ('iam.googleapis.com', sa + ':getIamPolicy?options.requestedPolicyVersion=3', 'POST', None),
            'history-policy': ('bigquery.googleapis.com', table + ':getIamPolicy', 'POST', {'options': {'requestedPolicyVersion': 3}})}
        for target in ('pool', 'sa'):
            routes['role-' + target + '-get'] = ('iam.googleapis.com', '/v1/' + self.base.role_name(target), 'GET', None)
        for target, host, path, permissions in (
                ('project', 'cloudresourcemanager.googleapis.com', '/v3/projects/' + PROJECT, ['iam.roles.create', 'iam.roles.get']),
                ('pool', 'iam.googleapis.com', pool, list(self.base.AUTHORITY['pool'])),
                ('sa', 'iam.googleapis.com', sa, list(self.base.AUTHORITY['sa'])),
                ('history', 'bigquery.googleapis.com', table,
                 ['bigquery.tables.get', 'bigquery.tables.getIamPolicy', 'bigquery.tables.setIamPolicy'])):
            routes['authority-' + target] = (host, path + ':testIamPermissions', 'POST', {'permissions': permissions})
        need(alias in routes)
        return copy.deepcopy(routes[alias])

    def call(self, alias, *, absent=False):
        self.check()
        need(alias != 'role-history-create')
        host, path, method, body = self.route(alias)
        status, value = self.request(host, path, method=method, body=body, token=self.token)
        self.check()
        if absent:
            need(status == 404 and value is None)
            return None
        need(status == 200 and type(value) is dict)
        return value

    def identity(self):
        project = self.call('project')
        need(set(project) <= {'projectId', 'projectNumber', 'lifecycleState', 'name', 'createTime', 'labels', 'parent'}
             and project.get('projectId') == PROJECT and project.get('projectNumber') == self.manifest['project_number']
             and project.get('lifecycleState') == 'ACTIVE')
        account = self.call('metadata-sa')
        need(set(account) <= {'name', 'projectId', 'uniqueId', 'email', 'displayName', 'description', 'etag', 'oauth2ClientId', 'disabled'}
             and account.get('name') == self.manifest['metadata_service_account_resource']
             and account.get('projectId') == PROJECT and account.get('email') == self.manifest['metadata_service_account_email']
             and account.get('disabled', False) is False
             and type(account.get('uniqueId')) is str and re.fullmatch(r'[0-9]{1,30}', account['uniqueId']))
        return {'project': project, 'metadata_service_account': account}

    def prepare(self):
        self.prepared = False
        self.check()
        self.github(self.manifest)
        self.check()
        fresh = self.identity()  # Fixed dev identity verified before IAM target reads.
        for alias in ('project', 'pool', 'sa', 'history'):
            expected = self.route('authority-' + alias)[3]['permissions']
            value = self.call('authority-' + alias)
            need(set(value) <= {'permissions'} and type(value.get('permissions', [])) is list
                 and sorted(value.get('permissions', [])) == sorted(expected))
        # Permission probes are observations, never independent identity proof
        # or a replacement for the API's own enforcement on a later request.
        for alias in ('pool', 'sa'):
            value = self.call('role-' + alias + '-get')
            self.base.validate_role(value, alias)
            fresh['role_' + alias] = value
        self.call('role-history-get', absent=True)
        for alias in ('pool', 'sa', 'history'):
            value = self.base.policy(self.call(alias + '-policy'))
            planned_policy(self.base, value, alias, self.manifest['metadata_service_account_email'])
            fresh[alias + '_policy'] = value
        self.save('preflight.json', fresh)
        self.save('dry-plan.json', {'role_creation': HISTORY_ROLE,
            'binding_additions': self.manifest['binding_additions'], 'maximum_write_attempts': 4,
            'all_policy_etags_required': True, 'all_writers_quiescence_proven': False,
            'dedicated_identity_read_verified': False, 'migration_ready': False})
        self.before = fresh
        self.check()
        self.prepared = True
        return fresh

    def write(self, alias, body):
        self.check()
        sequence = ('role-history-create', 'pool-set', 'sa-set', 'history-set')
        need(self.execute and self.prepared and self.attempts < 4 and len(self.completed) == self.attempts
             and alias == sequence[self.attempts] and self.manifest['owner_route_acknowledged'] is True
             and self.manifest['known_iam_writers_quiescent'] is True)
        self.github(self.manifest)
        self.check()
        account = self.call('metadata-sa')
        need(account == self.before['metadata_service_account'])
        if alias == 'role-history-create':
            self.call('role-history-get', absent=True)
            need(body == HISTORY_ROLE)
            host, path, method, _ = self.route(alias)
        else:
            target = alias.split('-')[0]
            role = self.call('role-' + target + '-get')
            if target == 'history':
                validate_history_role(self.base, role)
            else:
                self.base.validate_role(role, target)
            expected = planned_policy(self.base, self.before[target + '_policy'], target,
                                      self.manifest['metadata_service_account_email'])
            need(body == {'policy': expected, 'updateMask': 'bindings,etag'})
            current = self.base.policy(self.call(target + '-policy'))
            need(current == self.before[target + '_policy'])  # Full policy and exact IAM etag.
            host = 'bigquery.googleapis.com' if target == 'history' else 'iam.googleapis.com'
            resource = TABLE if target == 'history' else self.manifest[
                'runtime_pool_resource' if target == 'pool' else 'runtime_iam_policy_resource']
            path = ('/bigquery/v2/' if target == 'history' else '/v1/') + resource + ':setIamPolicy'
            method = 'POST'
        self.attempts += 1
        self.event({'event': 'write-attempt', 'alias': alias, 'attempt': self.attempts})
        try:
            self.check()  # Durable fsync can consume the remaining approval window.
            status, value = self.request(host, path, method=method, body=copy.deepcopy(body), token=self.token)
            self.event({'event': 'write-http', 'alias': alias, 'http_status': status})
            self.check()
            need(status == 200 and type(value) is dict)
            if alias == 'role-history-create':
                validate_history_role(self.base, value)
                validate_history_role(self.base, self.call('role-history-get'))
            else:
                self.base.policy(value)
                need(self.base.comparable_policy(value) == self.base.comparable_policy(body['policy']))
                after = self.base.policy(self.call(target + '-policy'))
                need(self.base.comparable_policy(after) == self.base.comparable_policy(body['policy']))
                self.save('after-' + target + '-policy.json', after)
            self.save('write-' + str(self.attempts) + '.json', value)
            self.completed.append(alias)
            self.event({'event': 'write-verified', 'alias': alias})
        except Exception:
            self.event({'event': 'write-unconfirmed', 'alias': alias})
            raise Stop('write STOP') from None

    def run(self):
        need(self.prepared)
        self.check()
        self.write('role-history-create', copy.deepcopy(HISTORY_ROLE))
        for alias in ('pool', 'sa', 'history'):
            self.write(alias + '-set', {'policy': planned_policy(self.base, self.before[alias + '_policy'],
                alias, self.manifest['metadata_service_account_email']), 'updateMask': 'bindings,etag'})
        # Recheck exact roles; never update/reuse a conflicting role definition.
        after = {'roles': {}, 'policies': {}}
        for alias in ('pool', 'sa'):
            value = self.call('role-' + alias + '-get')
            self.base.validate_role(value, alias)
            after['roles'][alias] = value
        value = self.call('role-history-get')
        validate_history_role(self.base, value)
        after['roles']['history'] = value
        for alias in ('pool', 'sa', 'history'):
            value = self.base.policy(self.call(alias + '-policy'))
            expected = planned_policy(self.base, self.before[alias + '_policy'], alias,
                                      self.manifest['metadata_service_account_email'])
            need(self.base.comparable_policy(value) == self.base.comparable_policy(expected))
            after['policies'][alias] = value
        account = self.call('metadata-sa')
        need(account == self.before['metadata_service_account'])
        after['metadata_service_account'] = account
        self.save('after-final.json', after)


def main(argv=None):
    started = now()
    evidence = session = None
    mode, success = 'offline', False
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument('--manifest-file', type=Path, required=True)
        parser.add_argument('--manifest-sha256', required=True)
        parser.add_argument('--source-sha256')
        modes = parser.add_mutually_exclusive_group()
        modes.add_argument('--live-plan', action='store_true')
        modes.add_argument('--execute-owner-route', action='store_true')
        args = parser.parse_args(argv)
        base = load_base()
        manifest = validate_manifest(base, base.private_input(args.manifest_file, args.manifest_sha256))
        need(manifest['script_sha256'] == source_hash())
        mode = 'execute' if args.execute_owner_route else 'live-plan' if args.live_plan else 'offline'
        if mode != 'offline':
            need(base.digest(args.source_sha256) == manifest['script_sha256'])
            check_window(base, manifest, started)
            need(manifest['owner_route_acknowledged'] and manifest['known_iam_writers_quiescent'])
        evidence = Evidence(base)
        evidence.event({'event': 'start', 'mode': mode, 'started_utc': started.isoformat().replace('+00:00', 'Z'),
            'manifest_sha256': args.manifest_sha256, 'script_sha256': source_hash(), 'base_script_sha256': BASE_SHA256})
        if mode == 'offline':
            evidence.save('offline-intent.json', {'role_creation': HISTORY_ROLE,
                'binding_additions': manifest['binding_additions'], 'maximum_write_attempts': 4,
                'cloud_observed': False, 'execution_ready': False})
        else:
            base.check_github(manifest)
            check_window(base, manifest, started)
            token = base.owner_token()
            try:
                session = Bootstrap(base, manifest, evidence, execute=args.execute_owner_route,
                                    token=token, started=started)
                session.prepare()
                if args.execute_owner_route:
                    session.run()
            finally:
                token = None
                if session is not None:
                    session.token = None
        success = True
    except Exception:
        pass  # Never echo private arguments, dependency paths, API errors or bodies.
    finally:
        if evidence is not None:
            try:
                evidence.event({'event': 'finish', 'mode': mode, 'status': 'PASS' if success else 'STOP',
                    'completed_utc': utc(), 'write_attempts': session.attempts if session else 0,
                    'verified_write_aliases': session.completed if session else [],
                    'setup_complete': success and mode == 'execute', 'dedicated_identity_read_verified': False,
                    'fresh_full7_verified': False, 'owner_identity_independently_verified': False,
                    'all_writers_quiescence_proven': False, 'migration_ready': False})
            except Exception:
                success = False
            try:
                evidence.close()
            except Exception:
                success = False
        print('codex-metadata-reader-bootstrap: ' + ('PASS' if success else 'STOP'))
        print('codex-metadata-reader-bootstrap-mode: ' + mode)
        print('codex-metadata-reader-bootstrap-write-attempts: ' + str(session.attempts if session else 0))
        print('codex-metadata-reader-bootstrap-migration-ready: false')
        if evidence is not None:
            print('RESULT_DIRECTORY=' + str(evidence.root))
    return 0 if success else 1


if __name__ == '__main__':
    raise SystemExit(main())
