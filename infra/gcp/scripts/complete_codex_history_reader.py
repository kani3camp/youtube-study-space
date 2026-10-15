#!/usr/bin/env python3
"""Complete only the reviewed dev history table reader; default is offline.

No replay of previous role/Pool/SA changes. At most one Table IAM SET attempt.
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
import tempfile
import types
import urllib.error
import urllib.request

READER_SHA256 = '5e7e3ef0f9e017f56c7847a862ae4c1d28a4163b3d30bb73f289b8fb96712398'
BASE_SHA256 = 'afc5feaf75365c4777c698d9ef64ce5b6f50547310a5d47c9dabc49d1030ac85'
PRIOR_READER_SHA256 = '56fadba1a8b668c4cd12e76338e47bea8079dbeb1ea58fefd361bd2c6a6d7182'
MAX_ERROR_BYTES = 64 * 1024
MAX_WRITES = 1
MANIFEST_KEYS = frozenset({'schema_version', 'kind', 'source_usage', 'prior_result_sha256',
    'prior_intent_sha256', 'prior_intent', 'expected_snapshot', 'reviewed_revision',
    'script_sha256', 'reader_script_sha256', 'base_script_sha256', 'workflow_sha256',
    'issued_utc', 'expires_utc', 'owner_session_scope', 'owner_route_acknowledged',
    'known_iam_writers_quiescent'})
STATUS_ENUM = frozenset({'INVALID_ARGUMENT', 'UNAUTHENTICATED', 'PERMISSION_DENIED',
    'NOT_FOUND', 'ALREADY_EXISTS', 'ABORTED', 'FAILED_PRECONDITION', 'RESOURCE_EXHAUSTED',
    'INTERNAL', 'UNAVAILABLE', 'DEADLINE_EXCEEDED', 'UNKNOWN'})
REASON_ENUM = frozenset({'IAM_PERMISSION_DENIED', 'ACCESS_TOKEN_EXPIRED', 'SERVICE_DISABLED',
    'ORG_RESTRICTION_VIOLATION', 'SECURITY_POLICY_VIOLATED', 'CONDITION_NOT_MET',
    'RESOURCE_NOT_FOUND', 'INVALID_ARGUMENT', 'INVALID_POLICY_VERSION', 'INVALID_ROLE',
    'CONCURRENT_POLICY_UPDATE', 'INVALID', 'BAD_REQUEST', 'ACCESS_DENIED', 'UNKNOWN'})
REASON_ALIASES = {'invalid': 'INVALID', 'badRequest': 'BAD_REQUEST', 'accessDenied': 'ACCESS_DENIED'}


class Stop(ValueError):
    pass


def need(value):
    if not value:
        raise Stop('history completion STOP')


class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise Stop('history completion arguments STOP')


def now():
    return datetime.now(timezone.utc)


def utc():
    return now().isoformat().replace('+00:00', 'Z')


def source_hash():
    path = Path(__file__)
    need(not path.is_symlink())
    return hashlib.sha256(path.read_bytes()).hexdigest()


def load_dependencies():
    path = Path(__file__).resolve().with_name('bootstrap_codex_metadata_readers.py')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        need(stat.S_ISREG(os.fstat(handle.fileno()).st_mode))
        raw = handle.read(4 * 1024 * 1024 + 1)
    need(0 < len(raw) <= 4 * 1024 * 1024 and hashlib.sha256(raw).hexdigest() == READER_SHA256)
    reader = types.ModuleType('pinned_codex_metadata_reader')
    reader.__file__ = str(path)
    exec(compile(raw, str(path), 'exec'), reader.__dict__)
    base = reader.load_base()  # Verifies and compiles the unchanged afc5 bytes.
    need(reader.BASE_SHA256 == BASE_SHA256)
    return base, reader


def check_window(base, manifest, started):
    current = now()
    need(base.stamp(manifest['issued_utc']) <= current < base.stamp(manifest['expires_utc'])
         and started <= current < started + timedelta(hours=1))


def history_plan(base, reader, policy, email):
    # The pinned reader now requires native v1/no conditions, preserving etag.
    return reader.planned_policy(base, policy, 'history', email)


def validate_manifest(base, reader, value):
    need(type(value) is dict and set(value) == MANIFEST_KEYS
         and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == 'private-owner-history-reader-completion-intent'
         and value['source_usage'] == 'reviewed-partial-setup-and-target-identifiers'
         and value['reader_script_sha256'] == READER_SHA256
         and value['base_script_sha256'] == BASE_SHA256
         and value['owner_session_scope'] == 'current-normal-gcloud-user'
         and type(value['reviewed_revision']) is str and re.fullmatch(r'[0-9a-f]{40}', value['reviewed_revision']))
    for key in ('prior_result_sha256', 'prior_intent_sha256', 'script_sha256',
                'reader_script_sha256', 'base_script_sha256', 'workflow_sha256'):
        base.digest(value[key])
    for key in ('owner_route_acknowledged', 'known_iam_writers_quiescent'):
        need(type(value[key]) is bool)
    issued, expires = base.stamp(value['issued_utc']), base.stamp(value['expires_utc'])
    need(issued < expires <= issued + timedelta(hours=8))
    prior = reader.validate_manifest(base, value['prior_intent'])
    need(base.sha(base.encoded(prior)) == value['prior_intent_sha256']
         and prior['script_sha256'] == PRIOR_READER_SHA256)
    expected = value['expected_snapshot']
    need(type(expected) is dict and set(expected) == {'project', 'metadata_service_account', 'roles', 'policies'}
         and type(expected['roles']) is dict and set(expected['roles']) == {'pool', 'sa', 'history'}
         and type(expected['policies']) is dict and set(expected['policies']) == {'pool', 'sa', 'history'})
    validate_identity(prior, expected['project'], expected['metadata_service_account'])
    for alias in ('pool', 'sa'):
        base.validate_role(expected['roles'][alias], alias)
        current = base.policy(expected['policies'][alias])
        bindings = [binding for binding in current.get('bindings', []) if binding['role'] == base.role_name(alias)]
        need(len(bindings) == 1 and 'condition' not in bindings[0]
             and sorted(bindings[0]['members']) == sorted(base.fixed_members() + ['serviceAccount:' + prior['metadata_service_account_email']]))
    reader.validate_history_role(base, expected['roles']['history'])
    history_plan(base, reader, expected['policies']['history'], prior['metadata_service_account_email'])
    return copy.deepcopy(value)


def validate_identity(prior, project, account):
    need(type(project) is dict
         and set(project) <= {'projectId', 'projectNumber', 'lifecycleState', 'name', 'createTime', 'labels', 'parent'}
         and project.get('projectId') == prior['project_id']
         and project.get('projectNumber') == prior['project_number'] and project.get('lifecycleState') == 'ACTIVE'
         and type(account) is dict
         and set(account) <= {'name', 'projectId', 'uniqueId', 'email', 'displayName', 'description', 'etag', 'oauth2ClientId', 'disabled'}
         and account.get('name') == prior['metadata_service_account_resource']
         and account.get('projectId') == prior['project_id'] and account.get('email') == prior['metadata_service_account_email']
         and account.get('disabled', False) is False and type(account.get('uniqueId')) is str
         and re.fullmatch(r'[0-9]{1,30}', account['uniqueId']))


def classify_error(base, http_status, raw):
    """Only fixed enums/codes leave this function, never messages or raw strings."""
    result = {'http_status': http_status if type(http_status) is int and 400 <= http_status <= 599 else 0,
              'error_code': 0, 'status': 'UNKNOWN', 'reason': 'UNKNOWN'}
    try:
        need(type(raw) is bytes and 0 < len(raw) <= MAX_ERROR_BYTES)
        error = base.decode(raw).get('error')
        need(type(error) is dict and type(error.get('code')) is int and error['code'] == result['http_status'])
        result['error_code'] = error['code']
        status = error.get('status')
        if type(status) is str and status in STATUS_ENUM:
            result['status'] = status
        reasons = set()
        details = error.get('details', [])
        need(type(details) is list)
        for detail in details:
            if type(detail) is dict and detail.get('@type') == 'type.googleapis.com/google.rpc.ErrorInfo':
                reason = detail.get('reason')
                reasons.add(reason if type(reason) is str and reason in REASON_ENUM else 'UNKNOWN')
        entries = error.get('errors', [])
        need(type(entries) is list)
        for entry in entries:
            if type(entry) is dict:
                reason = entry.get('reason')
                reasons.add(REASON_ALIASES.get(reason, 'UNKNOWN') if type(reason) is str else 'UNKNOWN')
        if len(reasons) == 1:
            result['reason'] = reasons.pop()
    except Exception:
        pass
    return result


def set_history_policy(base, reader, body, token):
    need(type(body) is dict and set(body) == {'policy'} and type(token) is str and bool(token))
    base.policy(body['policy'])
    need(body['policy'].get('version') == 1
         and not any('condition' in binding for binding in body['policy'].get('bindings', [])))
    request = urllib.request.Request('https://bigquery.googleapis.com/bigquery/v2/' + reader.TABLE + ':setIamPolicy',
        method='POST', headers={'Accept': 'application/json', 'Content-Type': 'application/json',
            'User-Agent': 'yss-history-reader-completion', 'Authorization': 'Bearer ' + token}, data=base.encoded(body))
    try:
        with urllib.request.build_opener(base.NoRedirect()).open(request, timeout=30) as response:
            need(response.status == 200)
            length = response.headers.get('Content-Length')
            need(length is None or re.fullmatch(r'[0-9]+', length))
            need(length is None or int(length) <= base.MAX_BYTES)
            raw = response.read(base.MAX_BYTES + 1)
            need(0 < len(raw) <= base.MAX_BYTES and (length is None or len(raw) == int(length)))
            return 200, base.decode(raw), None
    except urllib.error.HTTPError as error:
        status = error.code
        try:
            raw = error.read(MAX_ERROR_BYTES + 1)
        except Exception:
            raw = b''
        finally:
            error.close()
        return status, None, classify_error(base, status, raw)


class Evidence:
    def __init__(self, base):
        self.base = base
        self.root = Path(tempfile.mkdtemp(prefix='yss-history-reader-completion-'))
        self.root.chmod(0o700)
        self.fd = os.open(self.root / 'journal.jsonl', os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)

    def event(self, value):
        # Reuse the reviewed exclusive/bounded durable implementation.
        self.base.Evidence.event(self, dict(value, recorded_utc=utc(), timestamp_source='local-clock-self-report'))

    def save(self, name, value):
        return self.base.Evidence.save(self, name, value)

    def close(self):
        self.base.Evidence.close(self)


class Completion:
    READ_ALIASES = frozenset({'project', 'metadata-sa', 'role-pool-get', 'role-sa-get',
        'role-history-get', 'pool-policy', 'sa-policy', 'history-policy', 'authority-history'})

    def __init__(self, base, reader, manifest, evidence, *, execute=False, request=None,
                 write_request=None, github=None, token=None, started=None):
        self.base, self.reader, self.manifest, self.evidence = base, reader, manifest, evidence
        self.execute, self.token = execute, token
        self.started = started or now()
        self.context = copy.deepcopy(manifest['prior_intent'])
        for key in ('reviewed_revision', 'workflow_sha256', 'issued_utc', 'expires_utc'):
            self.context[key] = manifest[key]
        self.read_session = reader.Bootstrap(base, self.context, evidence, execute=False,
            request=request, token=token, started=self.started)
        self.read_session.check = self.check
        self.write_request = write_request or (lambda body, token: set_history_policy(base, reader, body, token))
        self.github = github or base.check_github
        self.attempts, self.write_verified = 0, False
        self.prepared = self.already_complete = self.observed_complete = False

    def check(self):
        check_window(self.base, self.manifest, self.started)

    def save(self, name, value):
        self.check(); result = self.evidence.save(name, value); self.check(); return result

    def read(self, alias):
        need(alias in self.READ_ALIASES)
        if alias != 'authority-history':
            return self.read_session.call(alias)
        self.check()
        host, path, method, _ = self.read_session.route(alias)
        status, value = self.read_session.request(host, path, method=method, body={
            'permissions': ['bigquery.tables.getIamPolicy', 'bigquery.tables.setIamPolicy']}, token=self.token)
        self.check()
        need(status == 200 and type(value) is dict and set(value) <= {'permissions'}
             and sorted(value.get('permissions', [])) == ['bigquery.tables.getIamPolicy', 'bigquery.tables.setIamPolicy'])
        return value

    def snapshot(self, *, final=False):
        expected = self.manifest['expected_snapshot']
        project, account = self.read('project'), self.read('metadata-sa')
        validate_identity(self.context, project, account)
        need(project == expected['project'] and account == expected['metadata_service_account'])
        result = {'project': project, 'metadata_service_account': account, 'roles': {}, 'policies': {}}
        for alias in ('pool', 'sa', 'history'):
            value = self.read('role-' + alias + '-get')
            if alias == 'history': self.reader.validate_history_role(self.base, value)
            else: self.base.validate_role(value, alias)
            need(value == expected['roles'][alias])
            result['roles'][alias] = value
        for alias in ('pool', 'sa', 'history'):
            value = self.base.policy(self.read(alias + '-policy'))
            if alias != 'history':
                need(value == expected['policies'][alias])
            else:
                need(value.get('version') == 1 and not any('condition' in binding for binding in value.get('bindings', [])))
                desired = history_plan(self.base, self.reader, expected['policies']['history'], self.context['metadata_service_account_email'])
                is_desired = self.base.comparable_policy(value) == self.base.comparable_policy(desired)
                if final or self.already_complete:
                    need(is_desired)
                else:
                    need(value == expected['policies']['history'] or is_desired)
                if not final and not self.prepared:
                    self.already_complete = is_desired
            result['policies'][alias] = value
        return result

    def prepare(self):
        self.check(); self.github(self.manifest); self.check()
        self.before = self.snapshot()
        self.read('authority-history')
        self.save('preflight.json', self.before)
        self.save('dry-plan.json', {'maximum_write_attempts': 1,
            'table_already_matches_desired': self.already_complete, 'native_policy_version': 1,
            'dedicated_identity_read_verified': False, 'migration_ready': False})
        self.prepared = True

    def run(self):
        need(self.execute and self.prepared and self.attempts < MAX_WRITES and not self.observed_complete
             and self.manifest['owner_route_acknowledged'] and self.manifest['known_iam_writers_quiescent'])
        self.check(); self.github(self.manifest); self.check()
        current = self.snapshot()
        if self.already_complete:
            need(current == self.before)
            self.save('after-final.json', self.snapshot(final=True))
            self.observed_complete = True
            return
        need(current == self.before)
        desired = history_plan(self.base, self.reader, current['policies']['history'], self.context['metadata_service_account_email'])
        self.attempts = 1
        self.evidence.event({'event': 'write-attempt', 'alias': 'history-set', 'attempt': 1})
        try:
            self.check()  # The durable reservation may have consumed the window.
            status, value, classification = self.write_request({'policy': desired}, self.token)
            need(type(status) is int and 100 <= status <= 599)
            self.evidence.event({'event': 'write-http', 'alias': 'history-set', 'http_status': status})
            if status != 200 and classification is not None:
                # Only the same fixed projection is accepted, including from mocks.
                need(type(classification) is dict and set(classification) == {'http_status', 'error_code', 'status', 'reason'}
                     and type(classification['http_status']) is int and 400 <= classification['http_status'] <= 599
                     and classification['http_status'] == status
                     and type(classification['error_code']) is int and classification['error_code'] in {0, status}
                     and classification['status'] in STATUS_ENUM and classification['reason'] in REASON_ENUM)
                self.evidence.event({'event': 'write-error-classification', **classification})
            self.check()
            need(status == 200 and type(value) is dict)
            self.base.policy(value)
            need(value.get('version') == 1 and self.base.comparable_policy(value) == self.base.comparable_policy(desired))
            after = self.base.policy(self.read('history-policy'))
            need(after.get('version') == 1 and self.base.comparable_policy(after) == self.base.comparable_policy(desired))
            self.save('write-1.json', value)
            self.save('after-history-policy.json', after)
            self.write_verified = True
            self.evidence.event({'event': 'write-verified', 'alias': 'history-set'})
        except Exception:
            self.evidence.event({'event': 'write-unconfirmed', 'alias': 'history-set'})
            raise Stop('history SET STOP') from None
        self.save('after-final.json', self.snapshot(final=True))
        self.observed_complete = True


def main(argv=None):
    started = now()
    evidence = session = None
    success, mode = False, 'offline'
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument('--manifest-file', type=Path, required=True)
        parser.add_argument('--manifest-sha256', required=True)
        parser.add_argument('--source-sha256')
        modes = parser.add_mutually_exclusive_group()
        modes.add_argument('--live-plan', action='store_true')
        modes.add_argument('--execute-owner-route', action='store_true')
        args = parser.parse_args(argv)
        base, reader = load_dependencies()
        manifest = validate_manifest(base, reader, base.private_input(args.manifest_file, args.manifest_sha256))
        need(manifest['script_sha256'] == source_hash())
        mode = 'execute' if args.execute_owner_route else 'live-plan' if args.live_plan else 'offline'
        if mode != 'offline':
            need(base.digest(args.source_sha256) == manifest['script_sha256'])
            check_window(base, manifest, started)
            need(manifest['owner_route_acknowledged'] and manifest['known_iam_writers_quiescent'])
        evidence = Evidence(base)
        evidence.event({'event': 'start', 'mode': mode, 'started_utc': utc(),
            'manifest_sha256': args.manifest_sha256, 'script_sha256': source_hash(),
            'prior_result_sha256': manifest['prior_result_sha256'], 'prior_intent_sha256': manifest['prior_intent_sha256']})
        if mode == 'offline':
            evidence.save('offline-intent.json', {'maximum_write_attempts': 1, 'resource_alias': 'history',
                'role': reader.HISTORY_ROLE, 'member_count_to_add': 1, 'native_policy_version': 1,
                'cloud_observed': False, 'execution_ready': False})
        else:
            base.check_github(manifest); check_window(base, manifest, started)
            token = base.owner_token()
            try:
                session = Completion(base, reader, manifest, evidence, execute=args.execute_owner_route, token=token, started=started)
                session.prepare()
                if args.execute_owner_route: session.run()
            finally:
                token = None
                if session is not None:
                    session.token = session.read_session.token = None
        success = True
    except Exception:
        pass
    finally:
        if evidence is not None:
            try:
                evidence.event({'event': 'finish', 'status': 'PASS' if success else 'STOP', 'mode': mode,
                    'completed_utc': utc(), 'write_attempts': session.attempts if session else 0,
                    'current_owner_observed_complete': bool(success and session and session.observed_complete),
                    'current_write_verified': session.write_verified if session else False,
                    'prior_failed_write_reclassified_as_success': False,
                    'dedicated_identity_read_verified': False, 'fresh_full7_verified': False,
                    'owner_identity_independently_verified': False, 'all_writers_quiescence_proven': False,
                    'migration_ready': False})
            except Exception: success = False
            try: evidence.close()
            except Exception: success = False
        print('codex-history-reader-completion: ' + ('PASS' if success else 'STOP'))
        print('codex-history-reader-completion-mode: ' + mode)
        print('codex-history-reader-completion-write-attempts: ' + str(session.attempts if session else 0))
        print('codex-history-reader-completion-migration-ready: false')
        if evidence is not None: print('RESULT_DIRECTORY=' + str(evidence.root))
    return 0 if success else 1


if __name__ == '__main__':
    raise SystemExit(main())
