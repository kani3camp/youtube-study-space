#!/usr/bin/env python3
"""DEV native Terraform operations. Raw evidence stays on the ephemeral runner.

No resource-type allowlist or per-change receipt. Jobs compare a private keyed
hash of the complete plan (except its generation timestamp), source, inputs and
S3 version, then apply the freshly saved, matching native plan once.
"""
from __future__ import annotations

import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import shutil
import sys
import time
from typing import NamedTuple

REPO = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO / '.github/scripts'))
import terraform_migration as common

PROJECT = 'test-youtube-study-space'
PRIVATE = 'gcp-normal-private'
MODES = {'normal-plan', 'normal-apply', 'normal-e2e'}
ACTIONS = {('no-op',), ('read',), ('create',), ('update',), ('delete',),
           ('delete', 'create'), ('create', 'delete')}
MAX_AGE = 1800


class SavedPlan(NamedTuple):
    path: Path
    digest: bytes


def need(value):
    if not value:
        raise ValueError('normal Terraform operation rejected')


def context(env, role=None):
    exact = {'GITHUB_EVENT_NAME': 'workflow_dispatch',
             'GITHUB_REPOSITORY': 'kani3camp/youtube-study-space',
             'GITHUB_REPOSITORY_ID': '340900071', 'GITHUB_REPOSITORY_OWNER_ID': '54093651',
             'GITHUB_ACTOR': 'kani3camp', 'GITHUB_ACTOR_ID': '54093651',
             'GITHUB_TRIGGERING_ACTOR': 'kani3camp', 'GITHUB_RUN_ATTEMPT': '1',
             'GITHUB_REF': 'refs/heads/dev',
             'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/dev',
             'NORMAL_TARGET': 'dev'}
    need(all(env.get(k) == v for k, v in exact.items()))
    need(env.get('NORMAL_MODE') in MODES)
    need(re.fullmatch('[0-9a-f]{40}', env.get('GITHUB_SHA', '')) is not None)
    need(env.get('NORMAL_EXPECTED_SHA') == env['GITHUB_SHA'])
    if role is not None:
        need(role in {'plan', 'apply'} and env.get('GITHUB_JOB') == 'normal_' + role)
        need(role != 'apply' or env['NORMAL_MODE'] in {'normal-apply', 'normal-e2e'})
        need(env.get('TF_WORKSPACE') == 'default' and env.get('STATE_AWS_REGION') == 'ap-northeast-1')
    return 'dev'


def private(env):
    root = Path(env['RUNNER_TEMP']).resolve() / PRIVATE
    need(not root.is_symlink())
    if root.exists():
        need(root.is_dir() and root.stat().st_uid == os.geteuid()
             and root.stat().st_mode & 0o777 == 0o700)
    return root


def inputs(env):
    # Keep the adopted 31-resource selection/private inventory, without coupling
    # ordinary development to the migration manifest or resource-type catalog.
    value = common.decode_input_transport(env.get('NORMAL_INPUTS_JSON', '').encode())
    need(type(value['tfvars']) is dict and value['tfvars'].get('project_id') == PROJECT)
    need(re.fullmatch('[0-9a-f]{64}', value.get('plan_integrity_key', '')) is not None)
    overlay = REPO / 'infra/gcp/environments/dev/normal.tfvars.json'
    additions = common.decode(overlay.read_bytes()) if overlay.exists() else {}
    need(type(additions) is dict)
    tfvars = value['tfvars'] | additions
    need(tfvars.get('project_id') == PROJECT)
    return tfvars, bytes.fromhex(value['plan_integrity_key'])


def review():
    value = common.decode((REPO / 'infra/gcp/environments/dev/normal-operations.json').read_bytes())
    need(type(value) is dict and set(value) == {'destructive_changes'})
    changes = value['destructive_changes']
    need(type(changes) is dict and all(type(a) is str and type(v) is list
         and tuple(v) in ACTIONS and 'delete' in v for a, v in changes.items()))
    return changes


def validate(plan, approved, *, post=False):
    need(type(plan) is dict and plan.get('format_version') == '1.2'
         and plan.get('terraform_version') == common.VERSION)
    need(plan.get('errored') is False and plan.get('complete') is True)
    need(not plan.get('deferred_changes') and not plan.get('resource_drift'))
    for check in plan.get('checks', []):
        need(check.get('status') == 'pass')
    changes = plan.get('resource_changes', [])
    need(type(changes) is list)
    destructive, seen, counts = {}, set(), {k: 0 for k in ('create', 'update', 'delete', 'replace', 'no-op', 'read')}
    for item in changes:
        address, change = item['address'], item['change']
        need(type(address) is str and address not in seen)
        seen.add(address)
        action = tuple(change['actions'])
        need(action in ACTIONS and not change.get('importing'))
        need(item.get('provider_name') in {'registry.terraform.io/hashicorp/google',
                                         'registry.terraform.io/hashicorp/google-beta'})
        need(item.get('mode') in {'managed', 'data'})
        for values in (change.get('before'), change.get('after')):
            if isinstance(values, dict) and values.get('project') is not None:
                need(values['project'] == PROJECT)
        if post:
            need(action == ('no-op',))
        if 'delete' in action:
            destructive[address] = list(action)
        counts['replace' if len(action) == 2 else action[0]] += 1
    if not post:
        need(destructive == approved)
    if post:
        need(all(tuple(c['actions']) == ('no-op',) for c in plan.get('output_changes', {}).values()))
    return counts


def summarize_changes(plan, provider_schema):
    """Publish only pinned-provider type names, native actions and counts.

    Addresses can embed private for_each keys. Attributes, schema descriptions,
    IDs, variables and values never enter this public projection.
    """
    grouped = {}
    for item in plan.get('resource_changes', []):
        actions = tuple(item['change']['actions'])
        need(actions in ACTIONS)
        if actions == ('no-op',):
            continue
        provider = item['provider_name']
        need(provider in {'registry.terraform.io/hashicorp/google',
                          'registry.terraform.io/hashicorp/google-beta'})
        need(item['mode'] in {'managed', 'data'})
        kind = item['type']
        need(type(kind) is str and re.fullmatch(r'google_[a-z0-9_]{1,200}', kind) is not None)
        schemas = provider_schema['provider_schemas'][provider][
            'resource_schemas' if item['mode'] == 'managed' else 'data_source_schemas']
        need(kind in schemas)
        key = kind, actions
        grouped[key] = grouped.get(key, 0) + 1
    return [{'resource_type': kind, 'actions': list(actions), 'count': count}
            for (kind, actions), count in sorted(grouped.items())]


def publish_plan_summary(env, changes):
    print('GCP_NORMAL_PLAN: ' + json.dumps({'changes': changes}, sort_keys=True))
    if env.get('GITHUB_STEP_SUMMARY'):
        lines = ['### Terraform resource changes', '']
        if changes:
            lines += ['| Resource type | Actions | Count |', '| --- | --- | ---: |']
            lines += [f"| `{row['resource_type']}` | {' → '.join(row['actions'])} | {row['count']} |"
                      for row in changes]
        else:
            lines += ['No resource changes.']
        with open(env['GITHUB_STEP_SUMMARY'], 'a', encoding='utf-8') as summary:
            summary.write('\n'.join(lines) + '\n')


def report_plan(env, root, tfroot, value):
    schema = common.decode(common.terraform(env, root, tfroot, 'summary-schema',
        ['providers', 'schema', '-json']).stdout)
    publish_plan_summary(env, summarize_changes(value, schema))


def commitment(env, tfvars, state, plan, key):
    projection = {k: v for k, v in plan.items() if k != 'timestamp'}
    payload = {'sha': env['GITHUB_SHA'], 'run': env['GITHUB_RUN_ID'],
               'mode': env['NORMAL_MODE'], 'tfvars': tfvars, 'review': review(),
               'state': state, 'plan': projection}
    return hmac.new(key, common.canonical(payload), hashlib.sha256).hexdigest()


def match(env, digest, now):
    need(re.fullmatch('[0-9a-f]{64}', env.get('NORMAL_EXPECTED_DIGEST', '')) is not None)
    need(hmac.compare_digest(digest, env['NORMAL_EXPECTED_DIGEST']))
    issued = int(env.get('NORMAL_PLAN_TIME', '0'))
    need(0 <= now - issued <= MAX_AGE)


def snapshot(env, root, name):
    # Reuse bounded native-S3 state/lock/workspace checks, not migration contracts.
    return common.snapshot(env | {'MIGRATION_TARGET': 'dev'}, root, name)


def identity(env):
    role = env['NORMAL_ROLE']
    account, arn = env['STATE_ACCOUNT_ID'], env['BACKEND_ROLE_ARN']
    need(re.fullmatch('[0-9]{12}', account) and arn.startswith('arn:aws:iam::' + account + ':role/')
         and arn.endswith('-' + role))
    result = common.aws(env, 'sts', 'get-caller-identity')
    need(result.get('Account') == account
         and result.get('UserId', '').startswith(env['BACKEND_ROLE_ID'] + ':')
         and result.get('Arn', '').startswith('arn:aws:sts::' + account + ':assumed-role/' + arn.rsplit('/', 1)[1] + '/'))
    need(env['GCP_SERVICE_ACCOUNT'] == f'terraform-dev-normal-{role}@{PROJECT}.iam.gserviceaccount.com')
    current = common.google(env, 'cloudresourcemanager.googleapis.com', 'v3/projects/' + PROJECT)
    need(current.get('projectId') == PROJECT and current.get('state') == 'ACTIVE')
    permissions = ['resourcemanager.projects.get', 'pubsub.topics.create', 'pubsub.topics.update', 'pubsub.topics.delete']
    own = common.google(env, 'cloudresourcemanager.googleapis.com', 'v3/projects/' + PROJECT + ':testIamPermissions', {'permissions': permissions})
    need(set(own.get('permissions', [])) == set(permissions if role == 'apply' else permissions[:1]))
    # Explicitly probe the production boundary, without reading production state.
    try:
        other = common.google(env, 'cloudresourcemanager.googleapis.com', 'v3/projects/youtube-study-space:testIamPermissions', {'permissions': permissions})
    except common.urllib.error.HTTPError as error:
        need(error.code == 403)
        error.close()
    else:
        need(other.get('permissions', []) == [])


def oidc(env):
    context(env, env['NORMAL_ROLE'])
    # Claims and subject are identical to existing environment-specific jobs;
    # job ID and GCP service account are verified separately above.
    request = common.urllib.request.Request(env['ACTIONS_ID_TOKEN_REQUEST_URL'] + '&audience=sts.amazonaws.com',
        headers={'Authorization': 'Bearer ' + env['ACTIONS_ID_TOKEN_REQUEST_TOKEN']})
    with common.urllib.request.urlopen(request, timeout=30) as response:
        token = json.load(response)['value']
    part = token.split('.')[1]
    claims = common.decode(common.base64.urlsafe_b64decode(part + '=' * (-len(part) % 4)))
    expected = {
        'repository_id': '340900071', 'repository_owner_id': '54093651',
        'environment': 'terraform-dev-' + env['NORMAL_ROLE'], 'ref': 'refs/heads/dev',
        'workflow_ref': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/dev',
        'job_workflow_ref': 'kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/dev',
        'event_name': 'workflow_dispatch',
    }
    subject = ':'.join(f'{k}:{v.replace(":", "%3A")}' for k, v in expected.items())
    need(claims.get('sub') == subject and all(claims.get(k) == v for k, v in expected.items()))
    need(all(claims.get(k) == v for k, v in {
        'actor_id': '54093651', 'actor': 'kani3camp', 'run_attempt': '1',
        'run_id': env['GITHUB_RUN_ID'], 'workflow_sha': env['GITHUB_SHA'],
        'job_workflow_sha': env['GITHUB_SHA']}.items()))


def source(env, root):
    need(common.run(['git', 'rev-parse', 'HEAD'], cwd=REPO).stdout.decode().strip() == env['GITHUB_SHA'])
    need(not common.run(['git', 'status', '--porcelain', '--untracked-files=no'], cwd=REPO).stdout)
    names = common.run(['git', 'ls-files', 'infra/gcp'], cwd=REPO).stdout.decode().splitlines()
    for name in names:
        path = REPO / name
        need(not path.is_symlink())
        raw = common.run(['git', 'show', 'HEAD:' + name], cwd=REPO).stdout
        need(path.read_bytes() == raw)
        common.write(root / 'checkout' / name, raw)
    return root / 'checkout/infra/gcp/environments/dev'


def setup(env, root, tfvars):
    tfroot = source(env, root)
    common.write(root / 'terraform.rc', b'disable_checkpoint = true\n')
    common.write(root / 'inputs.tfvars.json', common.canonical(tfvars))
    backend = dict(bucket=env['STATE_BUCKET'], key='youtube-study-space/dev/terraform.tfstate',
                   region='ap-northeast-1', encrypt=True, use_lockfile=True,
                   workspace_key_prefix='youtube-study-space/dev/workspaces',
                   allowed_account_ids=[env['STATE_ACCOUNT_ID']])
    common.write(root / 'backend.hcl', ''.join(f'{k} = {json.dumps(v)}\n' for k, v in backend.items()).encode())
    common.terraform(env, root, tfroot, 'init', ['init', '-input=false', '-lockfile=readonly', '-backend-config=' + str(root / 'backend.hcl')])
    return tfroot


def plan(env, root, tfroot, name, approved, *, post=False):
    binary = root / (name + '.tfplan')
    result = common.terraform(env, root, tfroot, name, ['plan', '-input=false', '-detailed-exitcode', '-lock-timeout=60s',
        '-var-file=' + str(root / 'inputs.tfvars.json'), '-out=' + str(binary)], codes=(0, 2))
    value = common.decode(common.terraform(env, root, tfroot, name + '-show', ['show', '-json', str(binary)]).stdout)
    counts = validate(value, approved, post=post)
    if post:
        need(result.returncode == 0)
    return SavedPlan(binary, hashlib.sha256(binary.read_bytes()).digest()), value, counts


def apply_saved(env, root, tfroot, binary, before, name):
    need(hashlib.sha256(binary.path.read_bytes()).digest() == binary.digest)
    need(snapshot(env, root, name + '-pre') == before)
    need(hashlib.sha256(binary.path.read_bytes()).digest() == binary.digest)
    common.terraform(env, root, tfroot, name + '-apply', ['apply', '-input=false', '-lock-timeout=60s', str(binary.path)])
    after = snapshot(env, root, name + '-after')
    need(after['state']['lineage'] == before['state']['lineage']
         and after['state']['serial'] >= before['state']['serial'])
    plan(env, root, tfroot, name + '-post', {}, post=True)
    need(snapshot(env, root, name + '-verified') == after)
    return after


def e2e(env, root, tfroot, before):
    # Only a generated, uniquely named DEV topic; no workload is altered for tests.
    probe = tfroot / 'normal_probe.tf.json'
    address = 'google_pubsub_topic.normal_ops_probe'
    baseline = before['state']['resources']
    need(all(r.get('name') != 'normal_ops_probe' for r in baseline))
    try:
        for phase, suffix, label, expected in [('create', 'a', 'one', 'create'),
                ('update', 'a', 'two', 'update'), ('replace', 'b', 'two', 'replace'),
                ('delete', None, None, 'delete')]:
            if suffix is None:
                probe.unlink()
            else:
                value = {'resource': {'google_pubsub_topic': {'normal_ops_probe': {
                    'project': PROJECT, 'name': 'terraform-normal-probe-' + env['GITHUB_RUN_ID'] + '-' + suffix,
                    'labels': {'verification': label}}}}}
                if probe.exists():
                    probe.unlink()
                common.write(probe, common.canonical(value))
            approved = {} if phase in {'create', 'update'} else {address: ['delete', 'create'] if phase == 'replace' else ['delete']}
            binary, value, counts = plan(env, root, tfroot, phase, approved)
            changed = [c for c in value.get('resource_changes', []) if c['change']['actions'] != ['no-op']]
            need(len(changed) == 1 and changed[0]['address'] == address and counts[expected] == 1)
            before = apply_saved(env, root, tfroot, binary, before, phase)
            print('GCP_NORMAL_E2E: PASS; phase=' + phase)
    except Exception:
        # Recovery may delete only this run's probe. Never apply changes to any
        # existing workload as an incidental cleanup operation.
        if probe.exists():
            probe.unlink()
        try:
            binary, value, _ = plan(env, root, tfroot, 'recovery', {address: ['delete']})
            changed = [c for c in value.get('resource_changes', []) if c['change']['actions'] != ['no-op']]
            need(len(changed) == 1 and changed[0]['address'] == address
                 and changed[0]['change']['actions'] == ['delete'])
            current = snapshot(env, root, 'recovery-current')
            apply_saved(env, root, tfroot, binary, current, 'recovery')
            print('GCP_NORMAL_E2E: recovery_probe_deleted=true')
        except Exception as error:
            common.close_http_error(error)
            print('GCP_NORMAL_E2E: STOP; probe cleanup requires documented operator recovery')
        raise
    # Compare identities and all managed attributes after removing the probe.
    need(before['state']['resources'] == baseline)
    print('GCP_NORMAL_E2E: PASS; baseline_restored=true')


def execute(env):
    role = env['NORMAL_ROLE']
    context(env, role)
    root = private(env)
    tfvars, key = inputs(env)
    print('GCP_NORMAL: phase=identity')
    identity(env)
    print('GCP_NORMAL: phase=state-and-plan')
    before = snapshot(env, root, 'before')
    need(before['state'] is not None)
    tfroot = setup(env, root, tfvars)
    binary, value, counts = plan(env, root, tfroot, 'normal', review())
    need(snapshot(env, root, 'after-plan') == before)
    report_plan(env, root, tfroot, value)
    digest = commitment(env, tfvars, before, value, key)
    if role == 'plan':
        if env['NORMAL_MODE'] == 'normal-e2e':
            need(all(k in {'no-op', 'read'} or v == 0 for k, v in counts.items()))
        with open(env['GITHUB_OUTPUT'], 'a') as output:
            output.write('digest=' + digest + '\nplan_time=' + str(int(time.time())) + '\n')
    else:
        print('GCP_NORMAL: phase=replan-match')
        match(env, digest, int(time.time()))
        print('GCP_NORMAL: phase=apply-and-verify')
        after = apply_saved(env, root, tfroot, binary, before, 'normal')
        if env['NORMAL_MODE'] == 'normal-e2e':
            need(all(k in {'no-op', 'read'} or v == 0 for k, v in counts.items()))
            e2e(env, root, tfroot, after)
    print('GCP_NORMAL: PASS; role=' + role + '; counts=' + json.dumps(counts, sort_keys=True))


def main():
    env = dict(os.environ)
    try:
        operation = sys.argv[1]
        if operation == 'cleanup':
            root = private(env)
            if root.exists():
                shutil.rmtree(root)
        elif operation == 'preflight':
            context(env)
        elif operation == 'stage':
            context(env, env['NORMAL_ROLE'])
            inputs(env)
            review()
            root = private(env)
            need(not root.exists())
            root.mkdir(mode=0o700)
        elif operation == 'oidc':
            oidc(env)
        elif operation == 'execute':
            execute(env)
        else:
            need(False)
        return 0
    except Exception as error:
        common.close_http_error(error)
        # Constant diagnostics: provider/API stderr and exception values are private.
        print('GCP_NORMAL: STOP; operation failed; provider output suppressed; use the documented recovery procedure')
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
