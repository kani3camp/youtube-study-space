#!/usr/bin/env python3
"""Owner-dispatched, import-only GCP operations with private full-root evidence.

The fixed Environment secret is desired configuration, never refreshed from live
metadata. No raw state, plan, API response, exception or credential is published.
Historical receipts and approvals are not used as current execution authority.
"""
from __future__ import annotations

import base64
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

import terraform_migration_contract as contract

ROOT = Path(__file__).resolve().parents[2]
PRIVATE = 'gcp-migration-private'
MAX_BYTES = 4 * 1024 * 1024
PROJECTS = {'dev': 'test-youtube-study-space', 'prod': 'youtube-study-space'}
GOOGLE = 'registry.terraform.io/hashicorp/google'
VERSION = (ROOT / 'infra/gcp/.terraform-version').read_text().strip()
TABLE_OBSERVATIONS = {
    'etag', 'lastModifiedTime', 'streamingBuffer', 'numRows', 'numBytes',
    'numLongTermBytes', 'numTimeTravelPhysicalBytes', 'numTotalLogicalBytes',
    'numActiveLogicalBytes', 'numLongTermLogicalBytes', 'numTotalPhysicalBytes',
    'numActivePhysicalBytes', 'numLongTermPhysicalBytes', 'numPartitions',
    'numCurrentPhysicalBytes',
}


def need(condition):
    if not condition:
        raise ValueError('migration contract rejected')


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), allow_nan=False).encode()


def pairs(items):
    result = {}
    for key, value in items:
        need(key not in result)
        result[key] = value
    return result


def decode(raw):
    need(0 < len(raw) <= 16 * MAX_BYTES)
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda _: need(False))


def run(args, *, cwd=None, env=None, codes=(0,), timeout=90):
    result = subprocess.run(args, cwd=cwd, env=env, capture_output=True, timeout=timeout)
    need(len(result.stdout) <= 16 * MAX_BYTES and len(result.stderr) <= MAX_BYTES
         and result.returncode in codes)
    return result


def context(env, role=None):
    target = env.get('MIGRATION_TARGET')
    need(target in PROJECTS and env.get('MIGRATION_MODE') in {'migration-plan', 'migration-apply'})
    exact = {
        'GITHUB_EVENT_NAME': 'workflow_dispatch',
        'GITHUB_REPOSITORY': 'kani3camp/youtube-study-space',
        'GITHUB_REPOSITORY_ID': '340900071', 'GITHUB_REPOSITORY_OWNER_ID': '54093651',
        'GITHUB_ACTOR_ID': '54093651', 'GITHUB_ACTOR': 'kani3camp',
        'GITHUB_TRIGGERING_ACTOR': 'kani3camp', 'GITHUB_RUN_ATTEMPT': '1',
        'GITHUB_REF': 'refs/heads/feature/gcp-terraform-iac',
        'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac',
    }
    need(all(env.get(key) == value for key, value in exact.items()))
    need(re.fullmatch('[0-9a-f]{40}', env.get('GITHUB_SHA', '')) is not None)
    if target == 'prod':
        need(env.get('MIGRATION_PRODUCTION_READY') == 'true')
    if role is not None:
        need(role in {'plan', 'apply'} and env.get('GITHUB_JOB') == 'migration_' + role)
        need(role != 'apply' or env['MIGRATION_MODE'] == 'migration-apply')
        need(env.get('TF_WORKSPACE') == 'default' and env.get('STATE_AWS_REGION') == 'ap-northeast-1')
        need(env.get('AWS_MAX_ATTEMPTS') == '1' and env.get('AWS_RETRY_MODE') == 'standard')
    return target


def private_root(env):
    root = Path(env['RUNNER_TEMP']).resolve() / PRIVATE
    need(not root.is_symlink())
    if root.exists():
        info = root.stat()
        need(root.is_dir() and info.st_uid == os.geteuid() and info.st_mode & 0o777 == 0o700)
    return root


def write(path, raw):
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())


def inputs(env):
    raw = env.get('MIGRATION_INPUTS_JSON', '').encode()
    need(0 < len(raw) <= MAX_BYTES)
    value = decode(raw)
    need(type(value) is dict and set(value) == {
        'schema_version', 'tfvars', 'manifest', 'plan_integrity_key'})
    need(value['schema_version'] == 'gcp-migration-inputs-v1'
         and type(value['tfvars']) is dict
         and re.fullmatch('[0-9a-f]{64}', value['plan_integrity_key']) is not None)
    target = env['MIGRATION_TARGET']
    contract.validate_manifest(value['manifest'], target)
    need(value['tfvars'].get('project_id') == PROJECTS[target])
    return value


def stage(env):
    context(env, env['MIGRATION_ROLE'])
    inputs(env)
    need(run(['git', 'rev-parse', 'HEAD'], cwd=ROOT).stdout.decode().strip() == env['GITHUB_SHA'])
    need(not run(['git', 'status', '--porcelain', '--untracked-files=no'], cwd=ROOT).stdout)
    root = private_root(env)
    need(not root.exists())
    root.mkdir(mode=0o700)


def verify_oidc(env):
    role, target = env['MIGRATION_ROLE'], context(env, env['MIGRATION_ROLE'])
    request = urllib.request.Request(env['ACTIONS_ID_TOKEN_REQUEST_URL'] + '&audience=sts.amazonaws.com',
        headers={'Authorization': 'Bearer ' + env['ACTIONS_ID_TOKEN_REQUEST_TOKEN']})
    with urllib.request.urlopen(request, timeout=30) as response:
        token = json.load(response)['value']
    part = token.split('.')[1]
    claims = decode(base64.urlsafe_b64decode(part + '=' * (-len(part) % 4)))
    expected = {
        'repository_id': '340900071', 'repository_owner_id': '54093651',
        'environment': f'terraform-{target}-{role}',
        'ref': 'refs/heads/feature/gcp-terraform-iac',
        'workflow_ref': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac',
        'job_workflow_ref': 'kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/feature/gcp-terraform-iac',
        'event_name': 'workflow_dispatch',
    }
    subject = ':'.join(f'{key}:{value.replace(":", "%3A")}' for key, value in expected.items())
    need(claims.get('sub') == subject and all(claims.get(key) == value for key, value in expected.items()))
    need(claims.get('actor_id') == '54093651' and claims.get('actor') == 'kani3camp'
         and claims.get('run_attempt') == '1'
         and claims.get('job_workflow_sha') == env['GITHUB_SHA']
         and claims.get('workflow_sha') == env['GITHUB_SHA'])


def aws(env, *args):
    return decode(run(['aws', *args, '--output', 'json'], env=env).stdout)


def verify_identity(env):
    role, target = env['MIGRATION_ROLE'], context(env, env['MIGRATION_ROLE'])
    account, arn, role_id = (env[key] for key in ('STATE_ACCOUNT_ID', 'BACKEND_ROLE_ARN', 'BACKEND_ROLE_ID'))
    need(re.fullmatch('[0-9]{12}', account) is not None
         and arn.startswith('arn:aws:iam::' + account + ':role/')
         and arn.endswith('-' + role) and re.fullmatch('AROA[A-Z0-9]{17}', role_id) is not None)
    actual = aws(env, 'sts', 'get-caller-identity')
    need(actual.get('Account') == account and actual.get('UserId', '').startswith(role_id + ':')
         and actual.get('Arn', '').startswith('arn:aws:sts::' + account + ':assumed-role/' + arn.rsplit('/', 1)[1] + '/'))
    need(env['GCP_SERVICE_ACCOUNT'] == f'terraform-{target}-{role}@{PROJECTS[target]}.iam.gserviceaccount.com')
    project = google(env, 'cloudresourcemanager.googleapis.com', 'v3/projects/' + PROJECTS[target])
    need(project.get('projectId') == PROJECTS[target] and project.get('state') == 'ACTIVE'
         and project.get('name') == 'projects/' + inputs(env)['manifest'].get('project_number', ''))


def google(env, host, path, body=None):
    request = urllib.request.Request('https://' + host + '/' + path,
        headers={'Authorization': 'Bearer ' + env['GCP_ACCESS_TOKEN'], 'Content-Type': 'application/json'},
        data=None if body is None else canonical(body))
    with urllib.request.urlopen(request, timeout=30) as response:
        raw = response.read(MAX_BYTES + 1)
        need(len(raw) <= MAX_BYTES)
    return decode(raw)


def metadata_route(descriptor, project, project_number):
    kind, identifier = descriptor['type'], descriptor['import_id']
    if kind == 'google_storage_bucket':
        need(re.fullmatch('[a-z0-9][a-z0-9._-]{1,221}[a-z0-9]', identifier) is not None)
        return 'storage.googleapis.com', 'storage/v1/b/' + identifier, None
    if kind in {'google_bigquery_dataset', 'google_bigquery_table'}:
        need(identifier.startswith('projects/' + project + '/datasets/'))
        suffix = r'/tables/[A-Za-z0-9_-]+' if kind == 'google_bigquery_table' else ''
        need(re.fullmatch(r'projects/[^/]+/datasets/[A-Za-z0-9_]+' + suffix, identifier) is not None)
        return 'bigquery.googleapis.com', 'bigquery/v2/' + identifier, None
    if kind == 'google_project_service':
        expected_project, service = identifier.split('/', 1)
        need(expected_project == project and re.fullmatch('[a-z][a-z0-9-]*\\.googleapis\\.com', service) is not None)
        need(re.fullmatch('[0-9]{6,20}', project_number) is not None)
        return 'serviceusage.googleapis.com', 'v1/projects/' + project_number + '/services/' + service, None
    if kind == 'google_service_account_iam_member':
        account = identifier.split(' ', 1)[0]
        need(account == f'projects/{project}/serviceAccounts/{project}@appspot.gserviceaccount.com')
        return 'iam.googleapis.com', 'v1/' + account + ':getIamPolicy', {'options': {'requestedPolicyVersion': 3}}
    routes = {
        'google_firestore_backup_schedule': ('firestore.googleapis.com', r'databases/\(default\)/backupSchedules/[0-9a-f-]{36}'),
        'google_monitoring_notification_channel': ('monitoring.googleapis.com', r'notificationChannels/[0-9]+'),
        'google_monitoring_alert_policy': ('monitoring.googleapis.com', r'alertPolicies/[0-9]+'),
        'google_pubsub_topic': ('pubsub.googleapis.com', r'topics/[A-Za-z][A-Za-z0-9._~+-]{2,254}'),
        'google_cloud_scheduler_job': ('cloudscheduler.googleapis.com', r'locations/[a-z]+-[a-z0-9]+[0-9]/jobs/[A-Za-z0-9_-]+'),
        'google_cloudfunctions_function': ('cloudfunctions.googleapis.com', r'locations/[a-z]+-[a-z0-9]+[0-9]/functions/[A-Za-z][A-Za-z0-9_-]*'),
        'google_iam_workload_identity_pool': ('iam.googleapis.com', r'locations/global/workloadIdentityPools/aws-runtime'),
        'google_iam_workload_identity_pool_provider': ('iam.googleapis.com', r'locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider'),
    }
    need(kind in routes and re.fullmatch('projects/' + re.escape(project) + '/' + routes[kind][1], identifier) is not None)
    if kind in {'google_iam_workload_identity_pool', 'google_iam_workload_identity_pool_provider'}:
        need(re.fullmatch('[0-9]{6,20}', project_number) is not None)
        identifier = 'projects/' + project_number + '/' + identifier.split('/', 2)[2]
    return routes[kind][0], ('v3/' if kind.startswith('google_monitoring_') else 'v1/') + identifier, None


def collect_metadata(env, manifest):
    result = {}
    for descriptor in manifest['resources']:
        host, path, body = metadata_route(descriptor, PROJECTS[env['MIGRATION_TARGET']], manifest.get('project_number', ''))
        value = google(env, host, path, body)
        if descriptor['type'] == 'google_bigquery_table':
            need('schema' in value and 'tableReference' in value)
            value = {key: item for key, item in value.items() if key not in TABLE_OBSERVATIONS}
        elif descriptor['type'] == 'google_cloud_scheduler_job':
            value = {key: item for key, item in value.items() if key not in {'lastAttemptTime', 'scheduleTime', 'status'}}
        if descriptor['type'] == 'google_project_service':
            need(value.get('state') == 'ENABLED')
        result[descriptor['address']] = value
    return result


def snapshot(env, root, name):
    target = env['MIGRATION_TARGET']
    bucket, key = env['STATE_BUCKET'], f'youtube-study-space/{target}/terraform.tfstate'
    need(re.fullmatch('[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]', bucket) is not None)
    common = ['--bucket', bucket, '--expected-bucket-owner', env['STATE_ACCOUNT_ID']]
    for prefix in (key + '.tflock', key.rsplit('/', 1)[0] + '/workspaces/'):
        listing = aws(env, 's3api', 'list-objects-v2', *common, '--prefix', prefix, '--max-keys', '2')
        need(listing.get('Name') == bucket and listing.get('Prefix') == prefix
             and type(listing.get('KeyCount')) is int and listing['KeyCount'] == 0
             and listing.get('IsTruncated') is False and listing.get('Contents', []) == []
             and listing.get('CommonPrefixes', []) == [] and 'NextContinuationToken' not in listing)
    listing = aws(env, 's3api', 'list-objects-v2', *common, '--prefix', key, '--max-keys', '2')
    need(listing.get('Name') == bucket and listing.get('Prefix') == key
         and listing.get('IsTruncated') is False and type(listing.get('KeyCount')) is int
         and listing.get('CommonPrefixes', []) == [] and 'NextContinuationToken' not in listing)
    if listing.get('KeyCount') == 0:
        need(target == 'prod' and listing.get('Contents', []) == [])
        return {'head': None, 'raw': None, 'state': None}
    need(listing.get('KeyCount') == 1 and [item['Key'] for item in listing.get('Contents', [])] == [key])
    head = aws(env, 's3api', 'head-object', *common, '--key', key)
    need(type(head.get('ContentLength')) is int and 0 < head['ContentLength'] <= MAX_BYTES
         and head.get('ServerSideEncryption') == 'AES256'
         and type(head.get('VersionId')) is str and head['VersionId'] not in {'', 'null'})
    need(listing['Contents'][0].get('Size') == head['ContentLength']
         and listing['Contents'][0].get('ETag') == head.get('ETag'))
    path = root / (name + '.tfstate')
    result = aws(env, 's3api', 'get-object', *common, '--key', key,
                 '--range', f'bytes=0-{MAX_BYTES-1}', str(path))
    raw = path.read_bytes()
    os.chmod(path, 0o600)
    need(len(raw) == head['ContentLength'] == result.get('ContentLength')
         and result.get('VersionId') == head['VersionId'] and result.get('ETag') == head.get('ETag')
         and aws(env, 's3api', 'head-object', *common, '--key', key) == head)
    state = decode(raw)
    need(state.get('version') == 4 and state.get('terraform_version') == VERSION
         and type(state.get('serial')) is int and state['serial'] >= 0
         and type(state.get('lineage')) is str)
    return {'head': head, 'raw': raw.decode(), 'state': state}


def copy_source(env, root):
    names = run(['git', 'ls-files', 'infra/gcp'], cwd=ROOT).stdout.decode().splitlines()
    closure = {}
    for name in names:
        path = Path(name)
        if not (path.suffix in {'.tf', '.tftpl'} or path.name in {'.terraform.lock.hcl', 'schema.json'}):
            continue
        need(not (ROOT / path).is_symlink())
        raw = run(['git', 'show', 'HEAD:' + name], cwd=ROOT).stdout
        need((ROOT / path).read_bytes() == raw)
        write(root / 'checkout' / path, raw)
        closure[name] = hashlib.sha256(raw).hexdigest()
    need(bool(closure))
    return root / 'checkout/infra/gcp/environments' / env['MIGRATION_TARGET'], closure


def verify_source_copy(root, closure):
    need(bool(closure) and all(
        not (root / 'checkout' / name).is_symlink()
        and hashlib.sha256((root / 'checkout' / name).read_bytes()).hexdigest() == digest
        for name, digest in closure.items()))


def terraform(env, root, tfroot, name, args, *, codes=(0,)):
    clean = {key: value for key, value in env.items()
             if not key.startswith(('TF_VAR_', 'TF_CLI_ARGS', 'TF_LOG'))}
    clean.update(TF_IN_AUTOMATION='true', TF_INPUT='false', TF_WORKSPACE='default',
                 TF_CLI_CONFIG_FILE=str(root / 'terraform.rc'), CHECKPOINT_DISABLE='1')
    result = run(['terraform', *args], cwd=tfroot, env=clean, codes=codes, timeout=600)
    write(root / (name + '.stdout'), result.stdout)
    write(root / (name + '.stderr'), result.stderr)
    return result


def execute(env):
    role, target = env['MIGRATION_ROLE'], context(env, env['MIGRATION_ROLE'])
    value, root = inputs(env), private_root(env)
    manifest = value['manifest']
    stage_name = 'IDENTITY'
    try:
        verify_identity(env)
        stage_name = 'METADATA'
        before_metadata = collect_metadata(env, manifest)
        stage_name = 'STATE_BEFORE'
        before = snapshot(env, root, 'before')
        need(before['state'] is not None or not manifest['resources'])
        stage_name = 'SOURCE'
        tfroot, closure = copy_source(env, root)
        write(root / 'terraform.rc', b'disable_checkpoint = true\n')
        write(root / 'inputs.tfvars.json', canonical(value['tfvars']))
        key = f'youtube-study-space/{target}/terraform.tfstate'
        backend = dict(bucket=env['STATE_BUCKET'], key=key, region='ap-northeast-1',
                       encrypt=True, use_lockfile=True,
                       workspace_key_prefix=f'youtube-study-space/{target}/workspaces',
                       allowed_account_ids=[env['STATE_ACCOUNT_ID']])
        write(root / 'backend.hcl', ''.join(f'{key} = {json.dumps(item)}\n' for key, item in backend.items()).encode())
        stage_name = 'INIT'
        terraform(env, root, tfroot, 'init', ['init', '-input=false', '-lockfile=readonly', '-backend-config=' + str(root / 'backend.hcl')])
        schema = decode(terraform(env, root, tfroot, 'schema', ['providers', 'schema', '-json']).stdout)
        stage_name = 'PLAN'
        binary = root / 'saved.tfplan'
        terraform(env, root, tfroot, 'plan', ['plan', '-input=false', '-detailed-exitcode', '-lock-timeout=60s',
                  '-var-file=' + str(root / 'inputs.tfvars.json'), '-out=' + str(binary)], codes=(0, 2))
        need(binary.is_file() and 0 < binary.stat().st_size <= 16 * MAX_BYTES)
        plan = decode(terraform(env, root, tfroot, 'show', ['show', '-json', str(binary)]).stdout)
        stage_name = 'VALIDATE'
        summary = contract.validate_plan(plan, manifest=manifest, environment=target,
                                         provider_schema=schema, state=before['state'])
        need(summary['imports'] <= 1)
        stage_name = 'FRESH_RECHECK'
        need(snapshot(env, root, 'after-plan') == before)
        need(collect_metadata(env, manifest) == before_metadata)
        payload = {'source': env['GITHUB_SHA'], 'closure': closure,
                   'inputs': value, 'state': before,
                   'metadata': before_metadata,
                   'plan': contract.plan_commitment_payload(plan, manifest=manifest,
                        environment=target, provider_schema=schema, state=before['state'])}
        commitment = hmac.new(bytes.fromhex(value['plan_integrity_key']), b'gcp-migration-execution-v1\0' + canonical(payload), hashlib.sha256).hexdigest()
        if role == 'plan':
            with open(env['GITHUB_OUTPUT'], 'a') as stream:
                stream.write('commitment=' + commitment + '\n')
        else:
            stage_name = 'REPLAN_MATCH'
            need(hmac.compare_digest(commitment, env.get('MIGRATION_EXPECTED_COMMITMENT', '')))
            stage_name = 'SEAL'
            seal = hashlib.sha256(binary.read_bytes()).hexdigest()
            started = time.monotonic()
            need(snapshot(env, root, 'pre-apply') == before)
            need(collect_metadata(env, manifest) == before_metadata)
            need(inputs(env) == value and run(['git', 'rev-parse', 'HEAD'], cwd=ROOT).stdout.decode().strip() == env['GITHUB_SHA'])
            verify_source_copy(root, closure)
            need(hashlib.sha256(binary.read_bytes()).hexdigest() == seal and time.monotonic() - started < 60)
            stage_name = 'APPLY'
            write(root / 'apply-attempt', b'1\n')
            terraform(env, root, tfroot, 'apply', ['apply', '-input=false', '-lock-timeout=60s', str(binary)])
            stage_name = 'STATE_AFTER'
            after = snapshot(env, root, 'after-apply')
            contract.validate_state_delta(before['state'], after['state'], manifest=manifest,
                environment=target, provider_schema=schema, plan=plan,
                expected_imports={item['address'] for item in plan.get('resource_changes', []) if 'importing' in item['change']})
            need(collect_metadata(env, manifest) == before_metadata)
            if before['state'] is not None:
                if after['raw'] == before['raw']:
                    need(after == before)
                else:
                    # The pure validator above already permits only exact
                    # imports, computed observations and saved-plan check sets.
                    need(after['head']['VersionId'] != before['head']['VersionId']
                         and after['state']['serial'] > before['state']['serial'])
            stage_name = 'POST_PLAN'
            post_binary = root / 'post.tfplan'
            terraform(env, root, tfroot, 'post-plan', ['plan', '-input=false', '-detailed-exitcode', '-lock-timeout=60s',
                      '-var-file=' + str(root / 'inputs.tfvars.json'), '-out=' + str(post_binary)], codes=(0, 2))
            post = decode(terraform(env, root, tfroot, 'post-show', ['show', '-json', str(post_binary)]).stdout)
            summary = contract.validate_plan(post, manifest=manifest, environment=target,
                provider_schema=schema, state=after['state'], phase='post')
            need(snapshot(env, root, 'after-post') == after)
            need(collect_metadata(env, manifest) == before_metadata)
        print('GCP_MIGRATION: PASS; role=' + role + '; target=' + target
              + '; summary=' + json.dumps(summary, sort_keys=True, separators=(',', ':')))
    except Exception as error:
        # Stage codes are source constants. Never expose provider/API exceptions.
        reason = error.reason if type(error) is contract.ContractError else 'EXECUTION'
        print('GCP_MIGRATION: STOP; stage=' + stage_name + '; reason=' + reason)
        raise


def main():
    os.umask(0o077)
    env = dict(os.environ)
    try:
        phase = sys.argv[1]
        if phase == 'preflight':
            context(env)
        elif phase == 'stage':
            stage(env)
        elif phase == 'oidc':
            verify_oidc(env)
        elif phase == 'execute':
            execute(env)
        elif phase == 'cleanup':
            root = private_root(env)
            if root.exists():
                shutil.rmtree(root)
            need(not root.exists())
        else:
            need(False)
    except Exception:
        print('GCP_MIGRATION: STOP; phase=' + (sys.argv[1] if len(sys.argv) == 2 and sys.argv[1] in
              {'preflight', 'stage', 'oidc', 'execute', 'cleanup'} else 'UNKNOWN'))
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
