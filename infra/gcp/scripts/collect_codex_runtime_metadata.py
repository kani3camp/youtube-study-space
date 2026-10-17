#!/usr/bin/env python3
"""Optional original-user AWS/WIF dev metadata read. Offline unless --execute.

Private reviewed pins are not an authorization receipt. This collector never
uses the runtime workload SA to authenticate and never exports receipt_binding.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import importlib
import importlib.metadata
import json
import os
from pathlib import Path
import platform
import re
import stat
import subprocess
import sys
from types import ModuleType
import urllib.error
import urllib.parse
import urllib.request
import xml.etree.ElementTree as ET

ROOT = Path(__file__).resolve().parents[3]
REQUIREMENTS = ROOT / 'infra/gcp/requirements/codex-runtime-metadata.txt'
PROJECT = 'test-youtube-study-space'
MAX_BYTES = 4 * 1024 * 1024
MAX_PAGES = 16
HOSTS = frozenset({'iam.googleapis.com', 'serviceusage.googleapis.com',
                   'cloudresourcemanager.googleapis.com', 'bigquery.googleapis.com'})
SELECTOR_KEYS = frozenset({'schema_version', 'project_id', 'project_number',
    'runtime_pool_resource', 'runtime_provider_resource', 'runtime_service_account_email',
    'runtime_iam_policy_resource', 'runtime_audience', 'lambda6_source_match', 'ecs_source_match'})
COMPANIONS = ('collect_runtime_metadata', 'prepare_runtime_ownership', 'prepare_user_activity_history_adoption')
MAX_WINDOW = timedelta(minutes=15)
MAX_RUN = timedelta(minutes=10)
REGION = 'ap-northeast-1'
SUBJECT_TYPE = 'urn:ietf:params:aws:token-type:aws4_request'
ACCESS_TYPE = 'urn:ietf:params:oauth:token-type:access_token'
SCOPE = 'https://www.googleapis.com/auth/cloud-platform'
IAM_SCOPE = 'https://www.googleapis.com/auth/iam'
GOOGLE_STS = 'https://sts.googleapis.com/v1/token'
AWS_STS = 'https://sts.' + REGION + '.amazonaws.com?Action=GetCallerIdentity&Version=2011-06-15'
AUTH_KEYS = {'schema_version', 'kind', 'service_account_email', 'project_number', 'pool_id',
             'provider_id', 'source_page', 'source_section'}
MANIFEST_KEYS = {'schema_version', 'kind', 'source_revision', 'script_sha256', 'collector_sha256',
                 'requirements_sha256', 'auth_selector_sha256', 'runtime_selector_sha256',
                 'dependency_selector_sha256', 'issued_utc', 'expires_utc', 'source_identity',
                 'execute_approved'}
DEPENDENCY_KEYS = {'schema_version', 'kind', 'root', 'tree_sha256', 'python_version', 'platform'}
FORBIDDEN_ENV = ('AWS_SESSION_TOKEN', 'AWS_SECURITY_TOKEN', 'AWS_PROFILE', 'AWS_DEFAULT_PROFILE',
                 'AWS_ROLE_ARN', 'AWS_WEB_IDENTITY_TOKEN_FILE', 'AWS_CONTAINER_CREDENTIALS_FULL_URI',
                 'AWS_CONTAINER_CREDENTIALS_RELATIVE_URI')


class Stop(ValueError):
    pass


class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise Stop('codex metadata arguments STOP')


def need(value):
    if not value:
        raise Stop('codex metadata read STOP')


def now():
    return datetime.now(timezone.utc)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def hash_value(value):
    need(type(value) is str and re.fullmatch(r'[0-9a-f]{64}', value) and value != '0' * 64)
    return value


def decode(raw):
    need(type(raw) is bytes and 0 < len(raw) <= MAX_BYTES)
    def unique(pairs):
        result = {}
        for key, value in pairs:
            need(key not in result)
            result[key] = value
        return result
    try:
        value = json.loads(raw, object_pairs_hook=unique, parse_constant=lambda _: need(False))
    except Exception:
        raise Stop('codex metadata JSON STOP') from None
    need(type(value) is dict)
    return value


def stamp(value):
    need(type(value) is str and re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ', value))
    return datetime.fromisoformat(value.replace('Z', '+00:00'))


def private_root(path):
    need(path.is_absolute() and path == path.resolve() and not path.is_symlink())
    info = path.stat()
    need(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid()
         and stat.S_IMODE(info.st_mode) == 0o700)
    for parent in (path, *path.parents):
        marker = parent / '.git'
        need(not (marker.is_file() or (marker / 'HEAD').is_file()))
    return path


def private_bytes(path, expected):
    hash_value(expected)
    private_root(path.parent)
    need(path.is_absolute() and path == path.resolve() and not path.is_symlink())
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'rb') as handle:
        info = os.fstat(handle.fileno())
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid()
             and info.st_nlink == 1 and stat.S_IMODE(info.st_mode) == 0o600)
        raw = handle.read(MAX_BYTES + 1)
    need(0 < len(raw) <= MAX_BYTES and digest(raw) == expected)
    return raw


def validate_manifest(value):
    need(set(value) == MANIFEST_KEYS and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == 'private-codex-dev-runtime-metadata-read'
         and type(value['source_revision']) is str and re.fullmatch(r'[0-9a-f]{40}', value['source_revision'])
         and value['source_revision'] != '0' * 40 and type(value['execute_approved']) is bool)
    for key in MANIFEST_KEYS:
        if key.endswith('_sha256'):
            hash_value(value[key])
    issued, expires = stamp(value['issued_utc']), stamp(value['expires_utc'])
    need(issued < expires <= issued + MAX_WINDOW)
    identity = value['source_identity']
    need(type(identity) is dict and set(identity) == {'account_id', 'arn', 'user_id'}
         and type(identity['account_id']) is str and re.fullmatch(r'[0-9]{12}', identity['account_id'])
         and type(identity['arn']) is str and re.fullmatch('arn:aws:iam::' + identity['account_id']
             + r':user/[A-Za-z0-9+=,.@_/-]{1,512}', identity['arn'])
         and type(identity['user_id']) is str and re.fullmatch(r'AIDA[A-Z0-9]{17}', identity['user_id']))
    return value


def validate_auth(value):
    need(set(value) == AUTH_KEYS and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == 'canonical-dedicated-codex-dev-metadata-identity-selector'
         and value['pool_id'] == 'codex-cloud' and value['provider_id'] == 'aws-state-reader'
         and value['source_page'] == '3d3357a8d0ce8191a8b1d8876b6840a9'
         and value['source_section'] == 'Codex Cloud：開発・インフラ調査アクセス（2026-10-12）'
         and type(value['project_number']) is str and re.fullmatch(r'[0-9]{1,20}', value['project_number']))
    email = value['service_account_email']
    need(type(email) is str and re.fullmatch(r'[a-z][a-z0-9-]{4,28}[a-z0-9]@' + re.escape(PROJECT)
                                           + r'\.iam\.gserviceaccount\.com', email)
         and email not in {f'terraform-dev-{role}@{PROJECT}.iam.gserviceaccount.com' for role in ('plan', 'apply')})
    return value


def audience(auth):
    return '//iam.googleapis.com/projects/' + auth['project_number'] + '/locations/global/workloadIdentityPools/codex-cloud/providers/aws-state-reader'


def impersonation(auth):
    return 'https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/' + auth['service_account_email'] + ':generateAccessToken'


def validate_runtime(value, auth):
    need(set(value) == SELECTOR_KEYS and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['project_id'] == PROJECT and value['project_number'] == auth['project_number'])
    pool = 'projects/' + auth['project_number'] + '/locations/global/workloadIdentityPools/aws-runtime'
    provider = pool + '/providers/aws-provider'
    email = PROJECT + '@appspot.gserviceaccount.com'
    need(value['runtime_pool_resource'] == pool and value['runtime_provider_resource'] == provider
         and value['runtime_iam_policy_resource'] == 'projects/' + PROJECT + '/serviceAccounts/' + email
         and value['runtime_service_account_email'] == email
         and value['runtime_audience'] == '//iam.googleapis.com/' + provider
         and value['lambda6_source_match'] is True and type(value['ecs_source_match']) is bool
         and auth['service_account_email'] != email)
    return value


def requirements(raw):
    versions = {}
    for line in raw.decode().splitlines():
        if not line or line.startswith('#'):
            continue
        match = re.fullmatch(r'([a-z0-9-]+)==([^ ]+) --hash=sha256:([0-9a-f]{64})', line)
        need(match is not None and match[1] not in versions)
        versions[match[1]] = match[2]
    need(set(versions) == {'google-auth', 'pyasn1-modules', 'pyasn1', 'cryptography', 'cffi', 'pycparser'})
    return versions


def dependency_tree(root):
    need(root.is_absolute() and root == root.resolve() and not root.is_symlink())
    info = root.stat()
    need(stat.S_ISDIR(info.st_mode) and info.st_uid == os.geteuid() and info.st_mode & 0o022 == 0)
    rows, total = [], 0
    for path in sorted(root.rglob('*')):
        info = path.lstat()
        need(not path.is_symlink() and info.st_uid == os.geteuid() and info.st_mode & 0o022 == 0)
        if path.is_dir():
            need(path.name != '__pycache__')
            continue
        need(stat.S_ISREG(info.st_mode) and info.st_nlink == 1 and not path.name.endswith(('.pyc', '.pyo')))
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
        with os.fdopen(fd, 'rb') as handle:
            raw = handle.read(32 * MAX_BYTES + 1)
        total += len(raw)
        need(len(raw) <= 32 * MAX_BYTES and total <= 64 * MAX_BYTES and len(rows) < 10000)
        rows.append([path.relative_to(root).as_posix(), digest(raw)])
    need(rows)
    return digest(json.dumps(rows, separators=(',', ':')).encode())


def validate_dependencies(value, lock):
    need(set(value) == DEPENDENCY_KEYS and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == 'private-isolated-codex-metadata-dependencies'
         and value['python_version'] == '3.12' and value['platform'] == 'linux-x86_64'
         and sys.version_info[:2] == (3, 12) and platform.system() == 'Linux' and platform.machine() == 'x86_64'
         and type(value['root']) is str)
    hash_value(value['tree_sha256'])
    root = Path(value['root'])
    need(dependency_tree(root) == value['tree_sha256'])
    found = {}
    for distribution in importlib.metadata.distributions(path=[str(root)]):
        name = distribution.metadata['Name'].lower().replace('_', '-')
        need(name not in found)
        found[name] = distribution.version
    need(found == requirements(lock))
    return root


def source_pins(manifest):
    def git(*args):
        result = subprocess.run(['git', '-C', str(ROOT), *args], stdin=subprocess.DEVNULL,
                                stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=10)
        need(result.returncode == 0 and len(result.stdout) <= MAX_BYTES)
        return result.stdout
    need(git('rev-parse', 'HEAD').decode().strip() == manifest['source_revision'])
    git('diff', '--quiet', 'HEAD', '--', 'infra/gcp')
    sources = {}
    for path, key in ((Path(__file__), 'script_sha256'),
                      (Path(__file__).with_name('collect_runtime_metadata.py'), 'collector_sha256'),
                      (REQUIREMENTS, 'requirements_sha256'),
                      *((Path(__file__).with_name(name + '.py'), None) for name in COMPANIONS[1:])):
        need(not path.is_symlink())
        raw = path.read_bytes()
        need((key is None or digest(raw) == manifest[key])
             and git('show', manifest['source_revision'] + ':' + path.relative_to(ROOT).as_posix()) == raw)
        if path.stem in COMPANIONS:
            sources[path.stem] = raw
    need(set(sources) == set(COMPANIONS))
    return sources


def load_legacy(sources):
    # Execute the verified bytes, bypassing ambient modules and bytecode caches.
    need(type(sources) is dict and set(sources) == set(COMPANIONS)
         and all(type(raw) is bytes and 0 < len(raw) <= MAX_BYTES for raw in sources.values()))
    scripts = Path(__file__).resolve().parent
    sys.dont_write_bytecode = True
    sys.path.insert(0, str(scripts))
    for name in (*COMPANIONS[1:], COMPANIONS[0]):
        path = str(scripts / (name + '.py'))
        module = ModuleType(name)
        module.__file__ = path
        sys.modules[name] = module
        exec(compile(sources[name], path, 'exec'), module.__dict__)
    module = sys.modules['collect_runtime_metadata']
    need(module.PROJECT == PROJECT and module.MAX_BYTES == MAX_BYTES and module.MAX_PAGES == MAX_PAGES
         and module.HOSTS == HOSTS and module.SELECTOR_KEYS == SELECTOR_KEYS)
    return module


class Admission:
    def __init__(self, manifest, checks, *, clock=now):
        self.manifest, self.checks, self.clock = manifest, checks, clock
        self.started = clock()
        self.previous = self.started

    def sample(self):
        observed = self.clock()
        need(stamp(self.manifest['issued_utc']) <= self.previous <= observed < stamp(self.manifest['expires_utc'])
             and self.started <= observed < self.started + MAX_RUN)
        self.previous = observed
        return observed

    def check(self):
        self.sample()
        self.checks()
        return self.sample()  # Hash/path IO must not cross the admission deadline.


def source_keys(env):
    # Called only after all reviewed pins/private paths and explicit execution.
    # No provider chain, credential files, session/profile or metadata fallback.
    need(all(not env.get(key) for key in FORBIDDEN_ENV))
    access = env.get('AWS_ACCESS_KEY_ID')
    secret = env.get('AWS_SECRET_ACCESS_KEY')
    need(type(access) is str and re.fullmatch(r'AKIA[A-Z0-9]{16}', access)
         and type(secret) is str and re.fullmatch(r'[A-Za-z0-9/+=]{40}', secret))
    return access, secret


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        raise Stop('redirect STOP')


class Response:
    def __init__(self, status, data, headers):
        self.status, self.data, self.headers = status, data, headers


class Wire:
    def __init__(self, admission, auth, selectors, *, opener=None):
        self.admission, self.auth, self.selectors = admission, auth, selectors
        self.opener = opener or urllib.request.build_opener(NoRedirect())
        self.auth_calls = 0
        self.metadata_calls = 0

    def call(self, url, method='GET', body=None, headers=None):
        self.admission.check()
        need(type(url) is str and not any(char in url for char in ('\r', '\n', '#')))
        auth_urls = (AWS_STS, GOOGLE_STS, impersonation(self.auth))
        if url in auth_urls:
            need(self.auth_calls < len(auth_urls) and url == auth_urls[self.auth_calls] and method == 'POST')
            self.auth_calls += 1  # Attempt counted before sending; no retry.
        else:
            need(self.auth_calls == 3 and self.metadata_calls < MAX_PAGES + 6)
            parsed = urllib.parse.urlsplit(url)
            need(parsed.scheme == 'https' and parsed.netloc in HOSTS and body is None
                 and allowed_metadata(self.selectors, parsed.netloc,
                     parsed.path[1:] + ('?' + parsed.query if parsed.query else ''), method))
            self.metadata_calls += 1
        request = urllib.request.Request(url, method=method, data=body, headers=headers or {})
        try:
            with self.opener.open(request, timeout=30) as response:
                status = response.status
                need(type(status) is int and status == 200)
                length = response.headers.get('Content-Length')
                need(length is None or re.fullmatch(r'[0-9]+', length))
                need(length is None or int(length) <= MAX_BYTES)
                raw = response.read(MAX_BYTES + 1)
                need(0 < len(raw) <= MAX_BYTES and (length is None or len(raw) == int(length)))
                result = Response(status, raw, dict(response.headers))
        except urllib.error.HTTPError as error:
            error.close()  # Never read or format the dependency error body.
            raise Stop('HTTP STOP') from None
        except Exception:
            raise Stop('transport STOP') from None
        self.admission.check()
        return result

    def sdk_request(self, url, method='GET', body=None, headers=None, timeout=None, **kwargs):
        need(not kwargs and url in (GOOGLE_STS, impersonation(self.auth)) and method == 'POST')
        if url == GOOGLE_STS:
            need(type(body) in (str, bytes))
            fields = urllib.parse.parse_qs(body.decode() if type(body) is bytes else body, strict_parsing=True)
            need(set(fields) <= {'grant_type', 'requested_token_type', 'subject_token_type', 'subject_token', 'audience', 'scope'}
                 and all(len(values) == 1 for values in fields.values())
                 and fields.get('audience') == [audience(self.auth)]
                 and fields.get('subject_token_type') == [SUBJECT_TYPE]
                 and fields.get('requested_token_type') == [ACCESS_TYPE]
                 and fields.get('grant_type') == ['urn:ietf:params:oauth:grant-type:token-exchange']
                 and fields.get('scope') in ([IAM_SCOPE], [SCOPE]) and bool(fields.get('subject_token')))
        else:
            value = decode(body.encode() if type(body) is str else body)
            need(set(value) == {'delegates', 'scope', 'lifetime'} and value['delegates'] is None
                 and value['scope'] == [SCOPE] and value['lifetime'] == '900s')
        response = self.call(url, method, body.encode() if type(body) is str else body, headers)
        value = decode(response.data)
        if url == GOOGLE_STS:
            need(set(value) <= {'access_token', 'issued_token_type', 'token_type', 'expires_in', 'scope'}
                 and value.get('issued_token_type') == ACCESS_TYPE and value.get('token_type') == 'Bearer'
                 and type(value.get('expires_in')) is int and 0 < value['expires_in'] <= 3600)
            token = value.get('access_token')
        else:
            need(set(value) == {'accessToken', 'expireTime'} and type(value['expireTime']) is str)
            expires = datetime.fromisoformat(value['expireTime'].replace('Z', '+00:00'))
            need(expires.tzinfo is not None and now() < expires <= now() + timedelta(minutes=16))
            token = value['accessToken']
        need(type(token) is str and 0 < len(token) < 16384 and not any(char.isspace() for char in token))
        return response


def load_sdk(root):
    # No ADC/default/from_file helpers are used. Import only after tree/hash pins.
    sys.dont_write_bytecode = True
    sys.path.insert(0, str(root))
    def origins():
        namespaces = ('google', 'pyasn1', 'pyasn1_modules', 'cryptography', 'cffi', '_cffi_backend', 'pycparser')
        for name, module in tuple(sys.modules.items()):
            if name.split('.')[0] not in namespaces or module is None:
                continue
            filename = getattr(module, '__file__', None)
            if filename is not None:
                need(Path(filename).resolve().is_relative_to(root))
            else:
                paths = getattr(module, '__path__', ())
                need(paths and all(Path(path).resolve().is_relative_to(root) for path in paths))
    origins()  # Reject an already imported ambient SDK/dependency.
    aws = importlib.import_module('google.auth.aws')
    origins()
    return aws


def official_supplier(sdk, auth, keys):
    class Supplier(sdk.AwsSecurityCredentialsSupplier):
        def verify(self, context):
            need(context.audience == audience(auth) and context.subject_token_type == SUBJECT_TYPE)
        def get_aws_region(self, context, request):
            self.verify(context)
            return REGION
        def get_aws_security_credentials(self, context, request):
            self.verify(context)
            return sdk.AwsSecurityCredentials(keys[0], keys[1], None)
    return Supplier()


def source_identity(raw):
    need(type(raw) is bytes and 0 < len(raw) <= 65536)
    need(b'<!' not in raw)  # No DTD/entities in the fixed AWS response.
    root = ET.fromstring(raw)
    tag = '{https://sts.amazonaws.com/doc/2011-06-15/}'
    need(root.tag == tag + 'GetCallerIdentityResponse')
    result = root.findall(tag + 'GetCallerIdentityResult')
    need(len(result) == 1 and {node.tag for node in result[0]} == {tag + key for key in ('Account', 'Arn', 'UserId')}
         and len(result[0]) == 3)
    return {target: result[0].findtext(tag + source) for target, source in
            (('account_id', 'Account'), ('arn', 'Arn'), ('user_id', 'UserId'))}


def token(sdk, auth, keys, expected, wire):
    options = sdk.RequestSigner(REGION).get_request_options(sdk.AwsSecurityCredentials(keys[0], keys[1], None), AWS_STS, 'POST')
    need(set(options) == {'url', 'method', 'headers'} and options['url'] == AWS_STS and options['method'] == 'POST')
    response = wire.call(options['url'], options['method'], headers=options['headers'])
    need(source_identity(response.data) == expected)
    supplier = official_supplier(sdk, auth, keys)
    credentials = sdk.Credentials(audience=audience(auth), subject_token_type=SUBJECT_TYPE,
        token_url=GOOGLE_STS, credential_source=None, aws_security_credentials_supplier=supplier,
        service_account_impersonation_url=impersonation(auth),
        service_account_impersonation_options={'token_lifetime_seconds': 900}, scopes=[SCOPE])
    try:
        credentials.refresh(wire.sdk_request)
        value = credentials.token
        need(wire.auth_calls == 3 and type(value) is str and 0 < len(value) < 16384
             and not any(char.isspace() for char in value))
        return value
    except Exception:
        raise Stop('codex metadata authentication STOP') from None
    finally:
        credentials.token = None
        keys = None


def allowed_metadata(selectors, host, path, method):
    fixed = {
        ('cloudresourcemanager.googleapis.com', 'v1/projects/' + PROJECT, 'GET'),
        ('iam.googleapis.com', 'v1/' + selectors['runtime_pool_resource'], 'GET'),
        ('iam.googleapis.com', 'v1/' + selectors['runtime_provider_resource'], 'GET'),
        ('iam.googleapis.com', 'v1/' + selectors['runtime_pool_resource'] + ':listAttestationRules', 'GET'),
        ('iam.googleapis.com', 'v1/' + selectors['runtime_iam_policy_resource'] + ':getIamPolicy?options.requestedPolicyVersion=3', 'POST'),
        ('bigquery.googleapis.com', 'bigquery/v2/projects/' + PROJECT + '/datasets/firestore_export/tables/user-activity-history', 'GET'),
    }
    services = 'v1/projects/' + selectors['project_number'] + '/services?filter=state%3AENABLED&pageSize=200'
    allowed = (host, path, method) in fixed
    if host == 'serviceusage.googleapis.com' and method == 'GET':
        allowed = path == services or (path.startswith(services + '&pageToken=')
            and bool(path[len(services + '&pageToken='):]) and '&' not in path[len(services + '&pageToken='):])
    return allowed


def metadata_request(wire, selectors, access_token):
    def request(host, path, method='GET'):
        need(allowed_metadata(selectors, host, path, method))
        response = wire.call('https://' + host + '/' + path, method,
                             headers={'Authorization': 'Bearer ' + access_token, 'Content-Type': 'application/json'})
        return decode(response.data)
    return request


def run(args, *, env=None, sdk_loader=None, opener=None, clock=now, check_source=source_pins):
    manifest_raw = private_bytes(args.manifest_file, args.manifest_sha256)
    manifest = validate_manifest(decode(manifest_raw))
    auth = validate_auth(decode(private_bytes(args.auth_selector_file, manifest['auth_selector_sha256'])))
    selectors = validate_runtime(decode(private_bytes(args.runtime_selector_file, manifest['runtime_selector_sha256'])), auth)
    dependency = decode(private_bytes(args.dependency_selector_file, manifest['dependency_selector_sha256']))
    companions = check_source(manifest)
    lock = REQUIREMENTS.read_bytes()
    need(digest(lock) == manifest['requirements_sha256'])
    dependency_root = validate_dependencies(dependency, lock)
    output = args.output
    private_root(output.parent)
    need(output.is_absolute() and output == output.resolve() and not output.is_symlink() and not output.exists())
    if not args.execute:
        return False
    need(manifest['execute_approved'] is True)
    def checks():
        need(private_bytes(args.manifest_file, args.manifest_sha256) == manifest_raw)
        private_bytes(args.auth_selector_file, manifest['auth_selector_sha256'])
        private_bytes(args.runtime_selector_file, manifest['runtime_selector_sha256'])
        private_bytes(args.dependency_selector_file, manifest['dependency_selector_sha256'])
    admission = Admission(manifest, checks, clock=clock)
    admission.check()
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    reserved = os.fstat(fd)
    def output_identity():
        private_root(output.parent)
        info, path_info = os.fstat(fd), output.lstat()
        need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid() and info.st_nlink == 1
             and stat.S_IMODE(info.st_mode) == 0o600
             and (info.st_dev, info.st_ino) == (path_info.st_dev, path_info.st_ino)
             and (info.st_dev, info.st_ino) == (reserved.st_dev, reserved.st_ino))
    success, keys, access_token = False, None, None
    try:
        output_identity()
        legacy = load_legacy(companions)
        sdk = (sdk_loader or load_sdk)(dependency_root)
        admission.check()
        keys = source_keys(os.environ if env is None else env)
        wire = Wire(admission, auth, selectors, opener=opener)
        access_token = token(sdk, auth, keys, manifest['source_identity'], wire)
        keys = None
        data = legacy.collect(selectors, metadata_request(wire, selectors, access_token))
        access_token = None
        admission.check()
        need(dependency_tree(dependency_root) == dependency['tree_sha256'])
        check_source(manifest)
        completed = admission.check()
        value = {'schema_version': 1, 'kind': 'private-codex-runtime-metadata-capture',
            'source_revision': manifest['source_revision'], 'manifest_sha256': args.manifest_sha256,
            'capture_started_utc': admission.started.isoformat().replace('+00:00', 'Z'),
            'capture_completed_utc': completed.isoformat().replace('+00:00', 'Z'),
            'complete': True, 'metadata': data, 'explicit_auth_requests': wire.auth_calls,
            'explicit_metadata_requests': wire.metadata_calls, 'source_identity_verified': True,
            'canonical_auth_selector_sha256': manifest['auth_selector_sha256'],
            'lambda6_source_match': selectors['lambda6_source_match'], 'ecs_source_match': selectors['ecs_source_match'],
            'ci_identity_read_verified': False, 'caller_scope_verified': False,
            'root_admission_verified': False, 'migration_ready': False}
        raw = json.dumps(value, ensure_ascii=False, allow_nan=False).encode() + b'\n'
        need(len(raw) <= MAX_BYTES)
        output_identity()
        with os.fdopen(fd, 'wb', closefd=False) as handle:
            handle.write(raw)
            handle.flush()
            os.fsync(handle.fileno())
        admission.check()
        output_identity()
        success = True
        return True
    finally:
        keys = access_token = None
        os.close(fd)
        if not success:
            try:
                info = output.lstat()
                if (info.st_dev, info.st_ino) == (reserved.st_dev, reserved.st_ino):
                    output.unlink()
            except FileNotFoundError:
                pass


def main(argv=None):
    execute = complete = success = False
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument('--manifest-file', type=Path, required=True)
        parser.add_argument('--manifest-sha256', required=True)
        parser.add_argument('--auth-selector-file', type=Path, required=True)
        parser.add_argument('--runtime-selector-file', type=Path, required=True)
        parser.add_argument('--dependency-selector-file', type=Path, required=True)
        parser.add_argument('--output', type=Path, required=True)
        parser.add_argument('--execute', action='store_true')
        args = parser.parse_args(argv)
        execute = args.execute
        complete = run(args)
        success = True
    except Exception:
        pass  # No exception/resource/token/credential/argv text is public.
    print('codex-runtime-metadata-read: ' + ('PASS' if success else 'STOP'))
    print('codex-runtime-metadata-mode: ' + ('read' if execute else 'offline'))
    print('codex-runtime-metadata-complete: ' + ('true' if complete else 'false'))
    print('codex-runtime-metadata-migration-ready: false')
    return 0 if success else 1


if __name__ == '__main__':
    raise SystemExit(main())
