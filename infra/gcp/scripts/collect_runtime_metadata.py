#!/usr/bin/env python3
"""Private dev metadata collector using an existing operator gcloud identity.

Seven exact metadata interfaces only. SA getIamPolicy is a read-only POST.
No IAM changes, business rows, Terraform, backend access or readiness receipts.
The selectors and resulting metadata must stay outside every checkout.
"""
from __future__ import annotations
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import urllib.error
import urllib.parse
import urllib.request

from prepare_runtime_ownership import require_empty_attestation_response
from prepare_user_activity_history_adoption import prepare as validate_history

PROJECT = 'test-youtube-study-space'
MAX_BYTES = 4 * 1024 * 1024
MAX_PAGES = 16
OUTPUT = 'runtime-metadata-capture.json'
HOSTS = frozenset({'iam.googleapis.com', 'serviceusage.googleapis.com',
                   'cloudresourcemanager.googleapis.com', 'bigquery.googleapis.com'})
SELECTOR_KEYS = frozenset({'schema_version', 'project_id', 'project_number',
    'runtime_pool_resource', 'runtime_provider_resource',
    'runtime_service_account_email', 'runtime_iam_policy_resource',
    'runtime_audience', 'lambda6_source_match', 'ecs_source_match'})

class Stop(ValueError):
    pass

class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise Stop('metadata arguments STOP')

def need(value):
    if not value:
        raise Stop('metadata collection STOP')

def unique(pairs):
    result = {}
    for key, value in pairs:
        need(key not in result)
        result[key] = value
    return result

def decode(raw):
    need(0 < len(raw) <= MAX_BYTES)
    result = json.loads(raw, object_pairs_hook=unique,
        parse_constant=lambda value: (_ for _ in ()).throw(Stop()))
    need(type(result) is dict)
    return result

def private_root(path):
    need(path.is_absolute() and not path.is_symlink())
    info = path.stat()
    need(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
         and stat.S_IMODE(info.st_mode) == 0o700)
    root = path.resolve()
    # Managed executors may expose empty read-only .git markers in /tmp.
    # A checkout has a worktree gitfile or a real repository HEAD.
    markers = [parent / '.git' for parent in (root, *root.parents)]
    need(not any(marker.is_file() or (marker / 'HEAD').is_file() for marker in markers))
    return root

def read_selectors(root, path):
    need(path.is_absolute() and path.parent.resolve() == root and not path.is_symlink())
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        info = os.fstat(handle.fileno())
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
             and stat.S_IMODE(info.st_mode) == 0o600)
        result = decode(handle.read(MAX_BYTES + 1))
    need(set(result) == SELECTOR_KEYS and type(result['schema_version']) is int
         and result['schema_version'] == 1 and result['project_id'] == PROJECT
         and type(result['project_number']) is str
         and re.fullmatch(r'[0-9]+', result['project_number']) is not None)
    pool = 'projects/' + result['project_number'] + '/locations/global/workloadIdentityPools/aws-runtime'
    provider = pool + '/providers/aws-provider'
    email = PROJECT + '@appspot.gserviceaccount.com'
    need(result['runtime_pool_resource'] == pool
         and result['runtime_provider_resource'] == provider
         and result['runtime_service_account_email'] == email
         and result['runtime_iam_policy_resource'] == 'projects/' + PROJECT + '/serviceAccounts/' + email
         and result['runtime_audience'] == '//iam.googleapis.com/' + provider
         and result['lambda6_source_match'] is True
         and type(result['ecs_source_match']) is bool)
    return result

class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, msg, headers, newurl):
        raise Stop('metadata redirect rejected')

def google_token():
    # Capture every CLI output. Never inspect credential files or display a token.
    result = subprocess.run(['gcloud', 'auth', 'print-access-token', '--quiet'],
        capture_output=True, text=True, timeout=90)
    need(result.returncode == 0 and 0 < len(result.stdout.strip()) < 16384)
    token = result.stdout.strip()
    need(not any(c.isspace() for c in token))
    return token

def transport(token, host, path, method='GET'):
    need(host in HOSTS and path.startswith(('v1/', 'bigquery/v2/'))
         and not any(c in path for c in ('\r', '\n', '#')))
    need(method == 'GET' or (method == 'POST' and host == 'iam.googleapis.com'
         and path.endswith(':getIamPolicy?options.requestedPolicyVersion=3')))
    request = urllib.request.Request('https://' + host + '/' + path,
        headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'}, method=method)
    # urllib's normal proxy and TLS trust remain enabled; cross-host redirects fail.
    opener = urllib.request.build_opener(NoRedirect())
    try:
        with opener.open(request, timeout=45) as response:
            need(response.status == 200)
            return decode(response.read(MAX_BYTES + 1))
    except urllib.error.HTTPError as error:
        error.read(MAX_BYTES + 1)  # Never print dependency error/resource bodies.
        raise Stop('metadata HTTP STOP') from None

def collect(selectors, request):
    number = selectors['project_number']
    project = request('cloudresourcemanager.googleapis.com', 'v1/projects/' + PROJECT)
    need(project.get('projectId') == PROJECT and project.get('projectNumber') == number
         and project.get('lifecycleState') == 'ACTIVE')
    pool = request('iam.googleapis.com', 'v1/' + selectors['runtime_pool_resource'])
    provider = request('iam.googleapis.com', 'v1/' + selectors['runtime_provider_resource'])
    for observed, expected in ((pool, selectors['runtime_pool_resource']),
                               (provider, selectors['runtime_provider_resource'])):
        need(type(observed) is dict
             and all(type(observed[key]) is str for key in ('displayName', 'description') if key in observed)
             and observed.get('name') == expected and observed.get('state') == 'ACTIVE'
             and observed.get('disabled', False) is False and 'expireTime' not in observed)
    need(set(pool) <= {'name', 'displayName', 'description', 'state', 'disabled', 'mode'}
         and pool.get('mode') in (None, '', 'FEDERATION_ONLY')
         and not any(k in pool for k in ('inlineCertificateIssuanceConfig', 'inlineTrustConfig')))
    need(set(provider) <= {'name', 'displayName', 'description', 'state', 'disabled',
                              'aws', 'attributeMapping', 'attributeCondition'}
         and type(provider.get('aws')) is dict and set(provider['aws']) == {'accountId'}
         and not any(k in provider for k in ('oidc', 'saml', 'x509'))
         and type(provider.get('attributeMapping')) is dict
         and type(provider.get('attributeCondition')) is str and provider['attributeCondition'].strip()
         and type(provider['aws']['accountId']) is str
         and re.fullmatch(r'[0-9]{12}', provider['aws']['accountId'])
         and all(type(k) is str and type(v) is str and v.strip()
                 for k, v in provider['attributeMapping'].items())
         and {'google.subject', 'attribute.aws_role', 'attribute.account'} <= set(provider['attributeMapping']))
    attestation = request('iam.googleapis.com', 'v1/' + selectors['runtime_pool_resource'] + ':listAttestationRules')
    require_empty_attestation_response(attestation)
    policy_resource = selectors['runtime_iam_policy_resource']
    policy = request('iam.googleapis.com', 'v1/' + policy_resource
        + ':getIamPolicy?options.requestedPolicyVersion=3', method='POST')
    need(set(policy) <= {'version', 'bindings', 'etag', 'auditConfigs'}
         and type(policy.get('version', 1)) is int and policy.get('version', 1) in (1, 3)
         and type(policy.get('bindings')) is list)
    for binding in policy['bindings']:
        need(type(binding) is dict and {'role', 'members'} <= set(binding)
             and set(binding) <= {'role', 'members', 'condition'}
             and type(binding['role']) is str and binding['role']
             and type(binding['members']) is list
             and all(type(member) is str and member for member in binding['members']))
        if 'condition' in binding:
            condition = binding['condition']
            need(policy.get('version', 1) == 3 and type(condition) is dict
                 and {'title', 'expression'} <= set(condition)
                 and set(condition) <= {'title', 'description', 'expression'}
                 and all(type(value) is str for value in condition.values()))
    services, seen_tokens, service_names, token = [], set(), set(), ''
    for _ in range(MAX_PAGES):
        path = 'v1/projects/' + number + '/services?filter=state%3AENABLED&pageSize=200'
        if token:
            path += '&pageToken=' + urllib.parse.quote(token, safe='')
        page = request('serviceusage.googleapis.com', path)
        need(type(page) is dict and set(page) <= {'services', 'nextPageToken'}
             and type(page.get('services', [])) is list)
        for service in page.get('services', []):
            need(type(service) is dict and type(service.get('config')) is dict)
            name = service['config'].get('name')
            need(type(name) is str and re.fullmatch(r'[a-z][a-z0-9-]*\.googleapis\.com', name)
                 and service.get('name') == 'projects/' + number + '/services/' + name
                 and name not in service_names and service.get('state') == 'ENABLED')
            service_names.add(name)
            services.append(service)
        token = page.get('nextPageToken', '')
        need(type(token) is str and len(token) <= 2048)
        if not token:
            break
        need(token not in seen_tokens)
        seen_tokens.add(token)
    need(not token)
    table = request('bigquery.googleapis.com', 'bigquery/v2/projects/' + PROJECT
        + '/datasets/firestore_export/tables/user-activity-history')
    validate_history(table)
    need(table.get('tableReference') == {'projectId': PROJECT, 'datasetId': 'firestore_export',
                                       'tableId': 'user-activity-history'})
    need(type(table.get('schema')) is dict and type(table['schema'].get('fields')) is list)
    return {'project': project, 'pool': pool, 'provider': provider,
        'pool_attestation_rules': attestation, 'iam_policy_resource': policy_resource,
        'runtime_config': {'audience': selectors['runtime_audience'],
                           'service_account_email': selectors['runtime_service_account_email']},
        'iam_policy': policy, 'enabled_services': {'services': services}, 'history_table': table}

def now():
    return datetime.now(timezone.utc).isoformat().replace('+00:00', 'Z')

def main(argv=None):
    parser = PrivateArgumentParser(description=__doc__)
    parser.add_argument('--private-root', type=Path, required=True)
    parser.add_argument('--selectors-file', type=Path, required=True)
    fd = None
    created = False
    try:
        args = parser.parse_args(argv)
        root = private_root(args.private_root)
        selectors = read_selectors(root, args.selectors_file)
        target = root / OUTPUT
        # Reserve a fresh 0600 regular file before obtaining a credential.
        fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        created = True
        started = now()
        token = google_token()
        try:
            data = collect(selectors, lambda host, path, **kwargs: transport(token, host, path, **kwargs))
        finally:
            token = ''
        capture = {'schema_version': 1, 'kind': 'private-operator-runtime-metadata-capture',
            'capture_started_utc': started, 'capture_completed_utc': now(),
            'complete': True, 'metadata': data,
            'lambda6_source_match': selectors['lambda6_source_match'],
            'ecs_source_match': selectors['ecs_source_match'],
            'ci_identity_read_verified': False, 'migration_ready': False}
        raw = (json.dumps(capture, ensure_ascii=False, allow_nan=False) + '\n').encode()
        need(len(raw) <= MAX_BYTES)
        with os.fdopen(fd, 'wb') as handle:
            fd = None
            handle.write(raw)
            handle.flush()
            os.fsync(handle.fileno())
    except Exception:
        try:
            if fd is not None:
                os.close(fd)
            if created:
                target.unlink(missing_ok=True)
        except OSError:
            print('STOP: 非公開ファイルの削除に失敗しました。既存の保管・削除手順で処理してください。', file=sys.stderr)
        print('STOP: 設定を読めませんでした。クラウド設定は変更していません。', file=sys.stderr)
        return 1
    print('PASS: 対象7種類の設定を非公開ファイルへ保存しました。移行準備の確認は未完了です。')
    return 0

if __name__ == '__main__':
    raise SystemExit(main())
