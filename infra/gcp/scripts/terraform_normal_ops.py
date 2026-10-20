#!/usr/bin/env python3
"""Environment-scoped GCP native Terraform operations. Raw evidence stays private.

No resource-type allowlist or per-change receipt. Jobs compare a private keyed
hash of the complete plan (except its generation timestamp), source, inputs and
S3 version, then apply the freshly saved, matching native plan once.
"""
from __future__ import annotations

from copy import deepcopy
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

PROJECT = 'test-youtube-study-space'  # Backwards-compatible DEV test fixture
PROJECTS = {'dev': PROJECT, 'prod': 'youtube-study-space'}
BRANCH_REF = re.compile(r'refs/heads/[A-Za-z0-9][A-Za-z0-9._/-]*')
CALLER = 'kani3camp/youtube-study-space/.github/workflows/ci.yml@'
REUSABLE = 'kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@'
DIAGNOSTIC_SECTIONS = ('source_run_mode', 'inputs', 'review', 'state', 'resource_changes',
                       'planned_values', 'prior_state', 'checks', 'relevant_attributes', 'other_plan_fields')
PHASES = frozenset({'identity', 'initial-state', 'source-copy', 'terraform-init',
                   'native-plan', 'plan-json', 'plan-schema', 'plan-validation', 'storage-ownership',
                   'state-readback', 'plan-summary', 'plan-binding'})
PLAN_REJECTIONS = frozenset({'format', 'incomplete', 'deferred', 'refresh-drift',
                            'checks', 'resource-contract', 'refresh-kind', 'refresh-actions',
                            'refresh-fields', 'refresh-state-before', 'refresh-plan-before',
                            'refresh-unknowns', 'refresh-masks', 'refresh-identity'})
NORMAL_OBSERVATIONS = frozenset({'etag', 'last_modified_time', 'num_bytes',
                                'num_long_term_bytes', 'num_rows'})
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


class PlanRejected(ValueError):
    def __init__(self, reason):
        need(reason in PLAN_REJECTIONS)
        self.reason = reason
        super().__init__('normal Plan validation rejected')


def plan_need(value, reason):
    if not value:
        raise PlanRejected(reason)


def target_project(env):
    target = env.get('NORMAL_TARGET')
    need(target in PROJECTS)
    return PROJECTS[target]


def development_ref(value):
    need(type(value) is str and BRANCH_REF.fullmatch(value) is not None
         and value != 'refs/heads/main')
    return value


def context(env, role=None):
    exact = {'GITHUB_EVENT_NAME': 'workflow_dispatch',
             'GITHUB_REPOSITORY': 'kani3camp/youtube-study-space',
             'GITHUB_REPOSITORY_ID': '340900071', 'GITHUB_REPOSITORY_OWNER_ID': '54093651',
             'GITHUB_ACTOR': 'kani3camp', 'GITHUB_ACTOR_ID': '54093651',
             'GITHUB_TRIGGERING_ACTOR': 'kani3camp', 'GITHUB_RUN_ATTEMPT': '1'}
    need(all(env.get(k) == v for k, v in exact.items()))
    target = env.get('NORMAL_TARGET')
    need(target in PROJECTS and env.get('NORMAL_MODE') in MODES)
    if target == 'dev':
        ref = development_ref(env.get('GITHUB_REF'))
    else:
        # No feature/integration branch may ever operate on Production.
        ref = 'refs/heads/main'
        need(env.get('GITHUB_REF') == ref and env['NORMAL_MODE'] != 'normal-e2e')
    need(env.get('GITHUB_WORKFLOW_REF') == CALLER + ref)
    need(re.fullmatch('[0-9a-f]{40}', env.get('GITHUB_SHA', '')) is not None)
    need(env.get('NORMAL_EXPECTED_SHA') == env['GITHUB_SHA'])
    if role is not None:
        need(role in {'plan', 'apply'} and env.get('GITHUB_JOB') == 'normal_' + role)
        need(role != 'apply' or env['NORMAL_MODE'] in {'normal-apply', 'normal-e2e'})
        need(env.get('TF_WORKSPACE') == 'default' and env.get('STATE_AWS_REGION') == 'ap-northeast-1')
    return target


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
    project = target_project(env)
    value = common.decode_input_transport(env.get('NORMAL_INPUTS_JSON', '').encode())
    need(type(value['tfvars']) is dict and value['tfvars'].get('project_id') == project)
    need(re.fullmatch('[0-9a-f]{64}', value.get('plan_integrity_key', '')) is not None)
    overlay = REPO / 'infra/gcp/environments' / env['NORMAL_TARGET'] / 'normal.tfvars.json'
    additions = common.decode(overlay.read_bytes()) if overlay.exists() else {}
    need(type(additions) is dict)
    tfvars = value['tfvars'] | additions
    need(tfvars.get('project_id') == project)
    return tfvars, bytes.fromhex(value['plan_integrity_key'])


def review(target='dev'):
    need(target in PROJECTS)
    value = common.decode((REPO / 'infra/gcp/environments' / target / 'normal-operations.json').read_bytes())
    need(type(value) is dict and set(value) == {'destructive_changes'})
    changes = value['destructive_changes']
    need(type(changes) is dict and all(type(a) is str and type(v) is list
         and tuple(v) in ACTIONS and 'delete' in v for a, v in changes.items()))
    return changes


def production_references(values, project, project_number=None):
    """Check project-bearing IDs at every nesting level, not only `project`.

    Google full/relative resource names, principalSet IDs and service-account
    emails carry their own project. Numeric aliases come only from the live
    authenticated Resource Manager response, never private input assertions.
    """
    aliases = {project}
    if project_number is not None:
        need(type(project_number) is str and re.fullmatch(r"[0-9]+", project_number))
        aliases.add(project_number)
    if isinstance(values, dict):
        for key, value in values.items():
            if key in {"project", "project_id", "project_number", "billing_project"} and value is not None:
                need(str(value) in aliases)
            production_references(value, project, project_number)
    elif isinstance(values, list):
        for value in values:
            production_references(value, project, project_number)
    elif isinstance(values, str):
        need(re.search(r"(?<![a-z0-9-])test-youtube-study-space(?![a-z0-9-])", values) is None)
        # Container registry paths encode a project without a projects/ prefix.
        for found in re.finditer(r'''(?:^|[/\s"':=])(?:[a-z0-9-]+-docker\.pkg\.dev|(?:[a-z0-9-]+\.)?gcr\.io)\.?(?::[0-9]+)?/([^/\s"':@]+)''', values, re.IGNORECASE):
            need(found.group(1) in aliases)
        for found in re.finditer(r'''(?:^|[/\s"':=])projects/([^/\s"':]+)''', values):
            identifier = found.group(1)
            # IAM allows '-' only for an unambiguous SA email in this project.
            if identifier == "-":
                suffix = values[found.end():]
                # Require an email; the ownership check below also covers the
                # App Engine default account's project-ID local part.
                need(re.match(r"/serviceAccounts/[A-Za-z0-9._-]+@[A-Za-z0-9.-]+" +
                              r'''\.gserviceaccount\.com(?:$|[/\s"'])''', suffix))
            else:
                need(identifier in aliases)
        for email in re.findall(r"[A-Za-z0-9._-]+@[A-Za-z0-9.-]+\.gserviceaccount\.com", values):
            local, domain = email.split("@")
            if (domain == project + ".iam.gserviceaccount.com"
                    or (domain == "appspot.gserviceaccount.com" and local == project)):
                continue
            if project_number is not None:
                if domain in {"developer.gserviceaccount.com", "cloudservices.gserviceaccount.com"}:
                    need(local == (project_number + '-compute' if domain == 'developer.gserviceaccount.com' else project_number))
                    continue
                if domain.startswith("gcp-sa-") and domain.endswith(".iam.gserviceaccount.com"):
                    need(local == "service-" + project_number)
                    continue
            need(False)


def verify_storage_ownership(env, plan):
    """Resolve globally named buckets; their names do not encode a project."""
    if env['NORMAL_TARGET'] != 'prod':
        return
    number = env['NORMAL_PROJECT_NUMBER']
    created, existing, references = set(), set(), set()

    def collect(value):
        if isinstance(value, dict):
            for key, child in value.items():
                if key in {'bucket', 'bucket_name', 'source_archive_bucket'} and type(child) is str:
                    references.add(child)
                collect(child)
        elif isinstance(value, list):
            for child in value:
                collect(child)
        elif isinstance(value, str):
            references.update(re.findall(r'gs://([^/\s]+)', value))

    for item in plan.get('resource_changes', []):
        change = item['change']
        after = change.get('after') or {}
        if item.get('type') == 'google_storage_bucket':
            if 'create' in change['actions']:
                need(after.get('project') == PROJECTS['prod'] and type(after.get('name')) is str)
                created.add(after['name'])
            before = change.get('before') or {}
            if before.get('name'):
                existing.add(before['name'])
        for values in (change.get('before'), change.get('after')):
            collect(values)
            if item.get('type') == 'google_storage_bucket' and isinstance(values, dict) and values.get('name'):
                references.add(values['name'])
    for bucket in sorted(references - (created - existing)):
        need(re.fullmatch(r'[a-z0-9][a-z0-9._-]{1,221}[a-z0-9]', bucket) is not None)
        observed = common.google(env, 'storage.googleapis.com', 'storage/v1/b/' + bucket)
        need(observed.get('name') == bucket and str(observed.get('projectNumber')) == number)


def validate_observation_drift(plan, state, provider_schema):
    """Bind only computed BigQuery observations to native State and refresh.

    Configurable fields, identities, masks, unknowns and other resource types
    never qualify. The complete drift remains inside the private Plan HMAC.
    """
    drift = plan.get('resource_drift', [])
    need(type(drift) is list)
    if not drift:
        return
    need(type(state) is dict and type(state.get('resources')) is list)
    native = {}
    for resource in state['resources']:
        if resource.get('mode') != 'managed':
            continue
        address = ((resource['module'] + '.') if resource.get('module') else '')
        address += resource['type'] + '.' + resource['name']
        for instance in resource['instances']:
            key = instance.get('index_key')
            need('index_key' not in instance or type(key) is str or type(key) is int and key >= 0)
            name = address + ('[' + common.canonical(key).decode() + ']' if 'index_key' in instance else '')
            need(name not in native)
            native[name] = instance
    changes = {item['address']: item for item in plan['resource_changes']}
    seen = set()
    for item in drift:
        need(type(item) is dict and set(item) <= common.contract.RESOURCE_KEYS | {'change', 'action_reason'})
        address = item['address']
        need(address not in seen and address in native and address in changes)
        seen.add(address)
        current = changes[address]
        plan_need(item.get('type') == 'google_bigquery_table' and item.get('mode') == 'managed'
                  and item.get('provider_name') == 'registry.terraform.io/hashicorp/google', 'refresh-kind')
        for key in ('address', 'module_address', 'mode', 'type', 'name', 'index', 'provider_name'):
            need(item.get(key) == current.get(key))
        change, planned = item['change'], current['change']
        need(type(change) is dict and set(change) <= common.contract.CHANGE_KEYS)
        plan_need(change.get('actions') == ['update'] and not change.get('importing')
                  and not change.get('replace_paths'), 'refresh-actions')
        before, after = change['before'], change['after']
        need(type(before) is dict and type(after) is dict and set(before) == set(after))
        differing = {key for key in before if common.canonical(before[key]) != common.canonical(after[key])}
        plan_need(bool(differing) and differing <= common.contract.observation_keys(
            provider_schema, item['type'], NORMAL_OBSERVATIONS), 'refresh-fields')
        plan_need(common.canonical(before) == common.canonical(native[address]['attributes']), 'refresh-state-before')
        plan_need(common.canonical(after) == common.canonical(planned['before']), 'refresh-plan-before')
        # The shared mask validator rejects every unknown leaf, including nested ones.
        try:
            common.contract._mask(change.get('after_unknown', {}), unknown=True)
        except Exception:
            raise PlanRejected('refresh-unknowns') from None
        for key in ('before_sensitive', 'after_sensitive'):
            plan_need(common.canonical(change.get(key, {})) == common.canonical(planned.get('before_sensitive', {})), 'refresh-masks')
        identity = native[address].get('identity')
        plan_need(planned.get('before_identity') == identity, 'refresh-identity')
        # Core omits unchanged identity metadata from resource_drift. Its
        # continuity is still proven by the native State and Plan before.
        if 'before_identity' in change or 'after_identity' in change:
            plan_need({'before_identity', 'after_identity'} <= set(change), 'refresh-identity')
            plan_need(change['before_identity'] == change['after_identity'] == identity, 'refresh-identity')


def validate(plan, approved, *, post=False, project=PROJECT, project_number=None,
             state=None, provider_schema=None):
    plan_need(type(plan) is dict and plan.get('format_version') == '1.2'
              and plan.get('terraform_version') == common.VERSION, 'format')
    plan_need(plan.get('errored') is False and plan.get('complete') is True, 'incomplete')
    plan_need(not plan.get('deferred_changes'), 'deferred')
    for check in plan.get('checks', []):
        plan_need(check.get('status') == 'pass', 'checks')
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
                need(values['project'] == project)
            if project == PROJECTS['prod']:
                production_references(values, project, project_number)
        if post:
            need(action == ('no-op',))
        if 'delete' in action:
            destructive[address] = list(action)
        counts['replace' if len(action) == 2 else action[0]] += 1
    if not post:
        need(destructive == approved)
    if post:
        need(all(tuple(c['actions']) == ('no-op',) for c in plan.get('output_changes', {}).values()))
    try:
        validate_observation_drift(plan, state, provider_schema)
    except PlanRejected:
        raise
    except Exception:
        raise PlanRejected('refresh-drift') from None
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


def replan_projection(plan):
    """Normalize only documented order-insensitive JSON-plan collections.

    Retain every value, mask, action, configuration, dependency and check.
    Terraform Core emits relevant_attributes using heuristic collection order.
    Resource/module/check/instance collections are address-keyed, not ordered.
    Never sort values inside provider-managed resource attributes.
    """
    projection = deepcopy(plan)
    projection.pop('timestamp', None)
    if 'relevant_attributes' in projection:
        refs = projection['relevant_attributes']
        need(type(refs) is list and all(type(x) is dict for x in refs))
        projection['relevant_attributes'] = sorted(refs, key=common.canonical)
    if 'checks' in projection:
        checks = projection['checks']
        need(type(checks) is list and all(type(x) is dict for x in checks))
        for check in checks:
            if 'instances' in check and check['instances'] is not None:
                instances = check['instances']
                need(type(instances) is list and all(type(x) is dict for x in instances))
                check['instances'] = sorted(instances, key=common.canonical)
        projection['checks'] = sorted(checks, key=common.canonical)
    if 'resource_changes' in projection:
        changes = projection['resource_changes']
        need(type(changes) is list and all(type(x) is dict and type(x.get('address')) is str for x in changes))
        projection['resource_changes'] = sorted(changes, key=lambda x: x['address'])

    def sort_module(module):
        need(type(module) is dict)
        if 'resources' in module:
            resources = module['resources']
            need(type(resources) is list and all(type(x) is dict and type(x.get('address')) is str for x in resources))
            module['resources'] = sorted(resources, key=lambda x: x['address'])
        if 'child_modules' in module:
            children = module['child_modules']
            need(type(children) is list and all(type(x) is dict and type(x.get('address')) is str for x in children))
            for child in children:
                sort_module(child)
            module['child_modules'] = sorted(children, key=lambda x: x['address'])

    for section in ('planned_values', 'prior_state'):
        value = projection.get(section)
        if value is None:
            continue
        need(type(value) is dict)
        shown = value if section == 'planned_values' else value.get('values')
        need(type(shown) is dict)
        if 'root_module' in shown:
            sort_module(shown['root_module'])
    return projection


def commitment_payload(env, tfvars, state, plan):
    return {'sha': env['GITHUB_SHA'], 'run': env['GITHUB_RUN_ID'],
            'mode': env['NORMAL_MODE'], 'target': env['NORMAL_TARGET'],
            'tfvars': tfvars, 'review': review(env['NORMAL_TARGET']),
            'state': state, 'plan': replan_projection(plan)}


def commitment(env, tfvars, state, plan, key):
    return hmac.new(key, common.canonical(commitment_payload(env, tfvars, state, plan)),
                    hashlib.sha256).hexdigest()


def commitment_components(env, tfvars, state, plan, key):
    """Per-section keyed diagnostics. Values and HMACs are never printed."""
    payload = commitment_payload(env, tfvars, state, plan)
    planned = payload['plan']
    sections = {
        'source_run_mode': {k: payload[k] for k in ('sha', 'run', 'mode', 'target')},
        'inputs': payload['tfvars'],
        'review': payload['review'],
        'state': payload['state'],
        'resource_changes': planned.get('resource_changes'),
        'planned_values': planned.get('planned_values'),
        'prior_state': planned.get('prior_state'),
        'checks': planned.get('checks'),
        'relevant_attributes': planned.get('relevant_attributes'),
        'other_plan_fields': {k: v for k, v in planned.items() if k not in {
            'resource_changes', 'planned_values', 'prior_state', 'checks', 'relevant_attributes'}},
    }
    return {name: hmac.new(key, b'gcp-normal-diagnostic-v1\0' + name.encode() +
                          b'\0' + common.canonical(value), hashlib.sha256).hexdigest()
            for name, value in sections.items()}


def report_replan_mismatch(env):
    """GitHub expression comparisons provide booleans, never private HMACs."""
    flags = []
    for section in DIAGNOSTIC_SECTIONS:
        flag = env.get('NORMAL_MISMATCH_' + section.upper())
        need(flag in {'true', 'false'})
        if flag == 'true':
            flags.append(section)
    print('GCP_NORMAL_REPLAN: STOP; sections=' +
          (','.join(flags) if flags else 'commitment_only'))


def check_plan_time(env, now):
    issued = int(env.get('NORMAL_PLAN_TIME', '0'))
    need(0 <= now - issued <= MAX_AGE)


def match(env, digest, now):
    # Credentialless unit-test helper only. No expected digest is passed to
    # authenticated Apply as a step environment variable.
    need(re.fullmatch('[0-9a-f]{64}', env.get('NORMAL_EXPECTED_DIGEST', '')) is not None)
    need(hmac.compare_digest(digest, env['NORMAL_EXPECTED_DIGEST']))
    check_plan_time(env, now)


def snapshot(env, root, name):
    # Reuse bounded native-S3 state/lock/workspace checks, not migration contracts.
    return common.snapshot(env | {'MIGRATION_TARGET': env['NORMAL_TARGET']}, root, name)


def phase(name):
    # Only fixed labels reach public logs; never an exception, path or value.
    need(name in PHASES)
    print('GCP_NORMAL: phase=' + name, flush=True)


def identity(env):
    role = env['NORMAL_ROLE']
    account, arn = env['STATE_ACCOUNT_ID'], env['BACKEND_ROLE_ARN']
    need(re.fullmatch('[0-9]{12}', account) and arn.startswith('arn:aws:iam::' + account + ':role/')
         and arn.endswith('-' + role))
    result = common.aws(env, 'sts', 'get-caller-identity')
    need(result.get('Account') == account
         and result.get('UserId', '').startswith(env['BACKEND_ROLE_ID'] + ':')
         and result.get('Arn', '').startswith('arn:aws:sts::' + account + ':assumed-role/' + arn.rsplit('/', 1)[1] + '/'))
    target, project = env['NORMAL_TARGET'], target_project(env)
    need(env['GCP_SERVICE_ACCOUNT'] == f'terraform-{target}-normal-{role}@{project}.iam.gserviceaccount.com')
    current = common.google(env, 'cloudresourcemanager.googleapis.com', 'v3/projects/' + project)
    need(current.get('projectId') == project and current.get('state') == 'ACTIVE')
    if target == 'prod':
        name = current.get('name', '')
        need(re.fullmatch(r'projects/[0-9]+', name) is not None)
        env['NORMAL_PROJECT_NUMBER'] = name.split('/')[1]
    permissions = ['resourcemanager.projects.get', 'pubsub.topics.create', 'pubsub.topics.update', 'pubsub.topics.delete']
    own = common.google(env, 'cloudresourcemanager.googleapis.com', 'v3/projects/' + project + ':testIamPermissions', {'permissions': permissions})
    need(set(own.get('permissions', [])) == set(permissions if role == 'apply' else permissions[:1]))
    # Fail closed if the normal identity has control-plane rights in the other project.
    other_project = PROJECTS['prod' if target == 'dev' else 'dev']
    try:
        other = common.google(env, 'cloudresourcemanager.googleapis.com', 'v3/projects/' + other_project + ':testIamPermissions', {'permissions': permissions})
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
    target = env['NORMAL_TARGET']
    ref = development_ref(env['GITHUB_REF']) if target == 'dev' else 'refs/heads/main'
    expected = {
        'repository_id': '340900071', 'repository_owner_id': '54093651',
        'environment': 'terraform-' + target + '-' + env['NORMAL_ROLE'], 'ref': ref,
        'workflow_ref': CALLER + ref,
        'job_workflow_ref': REUSABLE + ref,
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
    return root / 'checkout/infra/gcp/environments' / env['NORMAL_TARGET']


def setup(env, root, tfvars):
    phase('source-copy')
    tfroot = source(env, root)
    common.write(root / 'terraform.rc', b'disable_checkpoint = true\n')
    common.write(root / 'inputs.tfvars.json', common.canonical(tfvars))
    target = env['NORMAL_TARGET']
    backend = dict(bucket=env['STATE_BUCKET'], key=f'youtube-study-space/{target}/terraform.tfstate',
                   region='ap-northeast-1', encrypt=True, use_lockfile=True,
                   workspace_key_prefix=f'youtube-study-space/{target}/workspaces',
                   allowed_account_ids=[env['STATE_ACCOUNT_ID']])
    common.write(root / 'backend.hcl', ''.join(f'{k} = {json.dumps(v)}\n' for k, v in backend.items()).encode())
    phase('terraform-init')
    common.terraform(env, root, tfroot, 'init', ['init', '-input=false', '-lockfile=readonly', '-backend-config=' + str(root / 'backend.hcl')])
    return tfroot


def plan(env, root, tfroot, name, approved, *, post=False, state=None):
    binary = root / (name + '.tfplan')
    phase('native-plan')
    result = common.terraform(env, root, tfroot, name, ['plan', '-input=false', '-detailed-exitcode', '-lock-timeout=60s',
        '-var-file=' + str(root / 'inputs.tfvars.json'), '-out=' + str(binary)], codes=(0, 2))
    phase('plan-json')
    value = common.decode(common.terraform(env, root, tfroot, name + '-show', ['show', '-json', str(binary)]).stdout)
    schema = None
    if value.get('resource_drift'):
        phase('plan-schema')
        schema = common.decode(common.terraform(env, root, tfroot, name + '-schema',
                                               ['providers', 'schema', '-json']).stdout)
        # This uses the same schema-pinned, value-free projection as desired
        # changes. Malformed/untrusted drift never produces dynamic output.
        try:
            changes = summarize_changes({'resource_changes': value['resource_drift']}, schema)
        except Exception:
            changes = None
        if changes is not None:
            print('GCP_NORMAL_DRIFT: ' + json.dumps({'changes': changes}, sort_keys=True))
    phase('plan-validation')
    try:
        counts = validate(value, approved, post=post, project=target_project(env),
                          project_number=env.get('NORMAL_PROJECT_NUMBER'), state=state, provider_schema=schema)
    except PlanRejected:
        raise
    except Exception:
        raise PlanRejected('resource-contract') from None
    phase('storage-ownership')
    verify_storage_ownership(env, value)
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
    plan(env, root, tfroot, name + '-post', {}, post=True, state=after['state'])
    need(snapshot(env, root, name + '-verified') == after)
    return after


def e2e(env, root, tfroot, before):
    need(env['NORMAL_TARGET'] == 'dev')
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
            binary, value, counts = plan(env, root, tfroot, phase, approved, state=before['state'])
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
            current = snapshot(env, root, 'recovery-current')
            binary, value, _ = plan(env, root, tfroot, 'recovery', {address: ['delete']}, state=current['state'])
            changed = [c for c in value.get('resource_changes', []) if c['change']['actions'] != ['no-op']]
            need(len(changed) == 1 and changed[0]['address'] == address
                 and changed[0]['change']['actions'] == ['delete'])
            apply_saved(env, root, tfroot, binary, current, 'recovery')
            print('GCP_NORMAL_E2E: recovery_probe_deleted=true')
        except Exception as error:
            common.close_http_error(error)
            print('GCP_NORMAL_E2E: STOP; probe cleanup requires documented operator recovery')
        raise
    # Compare identities and all managed attributes after removing the probe.
    need(before['state']['resources'] == baseline)
    print('GCP_NORMAL_E2E: PASS; baseline_restored=true')


def output_commitments(env, tfvars, before, plan_json, key, *, include_time=False):
    # The runner only logs output names. HMACs must never be placed into step
    # env or interpolated shell commands. GitHub expressions compare them.
    digest = commitment(env, tfvars, before, plan_json, key)
    values = commitment_components(env, tfvars, before, plan_json, key)
    need(set(values) == set(DIAGNOSTIC_SECTIONS))
    with open(env['GITHUB_OUTPUT'], 'a', encoding='utf-8') as output:
        output.write('digest=' + digest + '\ncomponents=' +
                     json.dumps(values, sort_keys=True, separators=(',', ':')) + '\n')
        if include_time:
            output.write('plan_time=' + str(int(time.time())) + '\n')
    return digest


def initial_production_noop(env, before, value, counts):
    if env['NORMAL_TARGET'] != 'prod' or env['NORMAL_MODE'] != 'normal-plan':
        return False
    # Native State groups for_each/count instances under one resource block.
    managed = sum(len(resource.get('instances', []))
                  for resource in before['state']['resources']
                  if resource.get('mode') == 'managed')
    return (managed == 30
            and sum(item.get('mode') == 'managed' for item in value.get('resource_changes', [])) == 30
            and counts['no-op'] == 30 and sum(counts.values()) == 30
            and all(c.get('actions') == ['no-op'] for c in value.get('output_changes', {}).values()))


def execute(env, *, prepare_only=False):
    role = env['NORMAL_ROLE']
    context(env, role)
    need((role == 'plan' and not prepare_only) or
         (role == 'apply' and prepare_only))
    root = private(env)
    tfvars, key = inputs(env)
    phase('identity')
    identity(env)
    phase('initial-state')
    before = snapshot(env, root, 'before')
    need(before['state'] is not None)
    tfroot = setup(env, root, tfvars)
    binary, value, counts = plan(env, root, tfroot, 'normal', review(env['NORMAL_TARGET']), state=before['state'])
    phase('state-readback')
    need(snapshot(env, root, 'after-plan') == before)
    phase('plan-summary')
    report_plan(env, root, tfroot, value)
    if initial_production_noop(env, before, value, counts):
        # Emitted only after native full-root validation and State readback.
        print(f"GCP_PROD_INITIAL_NOOP: PASS; source={env['GITHUB_SHA']}; run={env['GITHUB_RUN_ID']}; managed=30; state_unchanged=true")
    if env['NORMAL_MODE'] == 'normal-e2e':
        need(all(k in {'no-op', 'read'} or v == 0 for k, v in counts.items()))
    phase('plan-binding')
    digest = output_commitments(env, tfvars, before, value, key,
                                include_time=(role == 'plan'))
    if role == 'apply':
        # Save all private material solely inside this job's 0700 runner dir.
        # The next step can apply the *same sealed* native plan only after
        # Actions checks both jobs' HMACs without logging either HMAC.
        prepared = {'source_sha': env['GITHUB_SHA'], 'run_id': env['GITHUB_RUN_ID'],
                    'mode': env['NORMAL_MODE'], 'digest': digest,
                    'binary_sha256': binary.digest.hex(), 'before': before,
                    'counts': counts}
        common.write(root / 'normal-prepared.json', common.canonical(prepared))
        print('GCP_NORMAL: PASS; phase=apply-prepared')
    else:
        print('GCP_NORMAL: PASS; role=plan; counts=' + json.dumps(counts, sort_keys=True))


def apply_prepared(env):
    context(env, 'apply')
    root = private(env)
    path = root / 'normal-prepared.json'
    need(path.is_file() and not path.is_symlink())
    prepared = common.decode(path.read_bytes())
    need(type(prepared) is dict and
         set(prepared) == {'source_sha', 'run_id', 'mode', 'digest',
                           'binary_sha256', 'before', 'counts'})
    need(prepared['source_sha'] == env['GITHUB_SHA'] and
         prepared['run_id'] == env['GITHUB_RUN_ID'] and
         prepared['mode'] == env['NORMAL_MODE'])
    need(re.fullmatch('[0-9a-f]{64}', prepared['binary_sha256']) is not None)
    need(re.fullmatch('[0-9a-f]{64}', prepared['digest']) is not None)
    check_plan_time(env, int(time.time()))
    tfvars, key = inputs(env)
    identity(env)
    before = prepared['before']
    root_plan = root / 'normal.tfplan'
    shown = common.decode((root / 'normal-show.stdout').read_bytes())
    need(commitment(env, tfvars, before, shown, key) == prepared['digest'])
    binary = SavedPlan(root_plan, bytes.fromhex(prepared['binary_sha256']))
    need(binary.path.is_file() and not binary.path.is_symlink() and
         hashlib.sha256(binary.path.read_bytes()).digest() == binary.digest)
    tfroot = root / 'checkout/infra/gcp/environments' / env['NORMAL_TARGET']
    need(tfroot.is_dir() and not tfroot.is_symlink())
    print('GCP_NORMAL: phase=apply-and-verify')
    after = apply_saved(env, root, tfroot, binary, before, 'normal')
    if env['NORMAL_MODE'] == 'normal-e2e':
        counts = prepared['counts']
        need(type(counts) is dict and
             all(k in {'no-op', 'read'} or v == 0 for k, v in counts.items()))
        e2e(env, root, tfroot, after)
    print('GCP_NORMAL: PASS; role=apply; counts=' +
          json.dumps(prepared['counts'], sort_keys=True))


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
            review(env['NORMAL_TARGET'])
            root = private(env)
            need(not root.exists())
            root.mkdir(mode=0o700)
        elif operation == 'oidc':
            oidc(env)
        elif operation == 'execute':
            execute(env)
        elif operation == 'prepare':
            execute(env, prepare_only=True)
        elif operation == 'apply':
            apply_prepared(env)
        elif operation == 'mismatch':
            context(env, 'apply')
            report_replan_mismatch(env)
            need(False)
        else:
            need(False)
        return 0
    except Exception as error:
        common.close_http_error(error)
        if isinstance(error, PlanRejected):
            print('GCP_NORMAL_VALIDATION: STOP; reason=' + error.reason)
        # Constant diagnostics: provider/API stderr and exception values are private.
        print('GCP_NORMAL: STOP; operation failed; provider output suppressed; use the documented recovery procedure')
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
