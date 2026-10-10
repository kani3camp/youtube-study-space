#!/usr/bin/env python3
"""Disabled protected runtime wave receipts, using existing private/read contracts.

No apply or authentication client is provided here. Workflow execution supplies
outcomes; bounded independent safety reads run even after plan/apply failure.
"""
from __future__ import annotations

import copy
import hashlib
import os
from pathlib import Path
import re
import stat
import sys
from urllib.parse import quote

import terraform_history_plan_receipt as private
import terraform_ownership_receipt as receipt
from terraform_identity_smoke import google
from terraform_history_table_metadata import stable_table_metadata

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / 'infra/gcp/scripts'))
import runtime_ownership_provenance as provenance
from runtime_ownership_packet import validate_packet
from prepare_runtime_ownership import prepare
from prepare_user_activity_history_workflow import PrivateArgumentParser
import validate_runtime_ownership_plan as graph
from validate_user_activity_history_plan import TABLE

BEFORE = 'runtime-execution-before.json'
SEALED = 'runtime-execution-sealed.json'
RECORD = 'runtime-ownership-receipt.json'
FILES = [BEFORE, SEALED, RECORD, 'runtime-state-before.json', 'runtime-state-preapply.json',
         'runtime-state-after.json', 'runtime-metadata-before.json', 'runtime-metadata-after.json',
         'runtime-execution-after.json', 'runtime-approved-plan.json', 'runtime-approved-post.json']
INPUT_ENV = ('TF_VAR_project_id', 'TF_VAR_manage_user_activity_history', 'TF_VAR_manage_primary_email',
             'TF_VAR_manage_youtube_quota_alerts', 'TF_VAR_manage_export_topic', 'TF_VAR_manage_export_scheduler',
             'TF_VAR_manage_export_function', 'TF_VAR_primary_email_address', 'TF_VAR_primary_email_channel_name',
             'TF_VAR_export_function_execution_service_account_email', 'TF_WORKSPACE')


def context(env, *, safety=False):
    root, packet, catalog = provenance.load_inputs(env)
    receipt.need(env.get('DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED') == 'true'
                 and env.get('DEV_HISTORY_POST_NOOP12_READY') == 'true'
                 and env.get('TF_VAR_manage_user_activity_history') == 'true'
                 and env.get('HISTORY_TARGET') == 'dev'
                 and env.get('AWS_MAX_ATTEMPTS') == '1' and env.get('AWS_RETRY_MODE') == 'standard')
    if env['MODE'] == 'apply':
        receipt.need(env.get('DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED') == 'true'
                     and env.get('DEV_HISTORY_RECEIPT_EMITTER_ENABLED') == 'true')
    metadata = private.private_json(root / 'user-history-before.json')
    provenance.require_verified_inputs(env, packet, metadata, now=provenance.receipt.timestamp(packet['reviewed_utc']) if safety else None)
    receipt.need(all(ref['run_id'] != int(env['GITHUB_RUN_ID']) for ref in
                     [packet['history_receipt']] + [i['reference'] for i in packet['adoption_receipts']]))
    return root, packet, metadata


def bundle(env, root, packet):
    candidate = validate_packet(packet, wave=packet['wave'], git_sha=env['GITHUB_SHA'])
    receipt.need(private.private_json(root / 'runtime-ownership.tfvars.json') == candidate)
    history = private.private_json(root / 'user-history-before.tfvars.json')
    from prepare_user_activity_history_adoption import prepare as history_inputs
    receipt.need(history == {k: v for k, v in history_inputs(private.private_json(root / 'user-history-before.json')).items()
                             if k != 'manage_user_activity_history'})
    receipt.need(all(env.get('TF_VAR_manage_export_' + k) == 'true' for k in ('function', 'scheduler', 'topic')))
    return dict(contract='runtime-private-input-v1', source_sha=env['GITHUB_SHA'], packet=packet,
                runtime=candidate, history=history, inputs={k: env.get(k) for k in INPUT_ENV})


def output(env, key, value):
    receipt.need(receipt.hex_value(value, 64))
    fd = os.open(env['GITHUB_OUTPUT'], os.O_WRONLY | os.O_APPEND | os.O_NOFOLLOW)
    with os.fdopen(fd, 'w') as handle:
        info = os.fstat(handle.fileno())
        receipt.need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
        handle.write(key + '=' + value + '\n')


def containers(state):
    """Validate every native container before baseline/runtime projection."""
    receipt.need(type(state.get('resources')) is list)
    result = {}
    for resource in state['resources']:
        receipt.need(type(resource) is dict and {'mode', 'type', 'name', 'provider', 'instances'} <= set(resource)
                     <= {'module', 'mode', 'type', 'name', 'provider', 'instances'}
                     and resource['mode'] == 'managed'
                     and resource['provider'] == 'provider["registry.terraform.io/hashicorp/google"]')
        receipt.need(all(type(resource[k]) is str and re.fullmatch(r'[A-Za-z_][A-Za-z0-9_-]*', resource[k])
                         for k in ('type', 'name'))
                     and ('module' not in resource or type(resource['module']) is str
                          and re.fullmatch(r'module\.[A-Za-z_][A-Za-z0-9_-]*(?:\[(?:\d+|"[^"]*")\])?', resource['module'])))
        key = (resource.get('module', ''), resource['type'], resource['name'])
        receipt.need(key not in result and type(resource['instances']) is list and bool(resource['instances']))
        result[key] = resource
    return result


def instances(state):
    result = {}
    for resource in containers(state).values():
        base = (resource.get('module', '') + '.' if 'module' in resource else '') + resource['type'] + '.' + resource['name']
        header = {k: v for k, v in resource.items() if k != 'instances'}
        for instance in resource['instances']:
            receipt.need(type(instance) is dict and {'schema_version', 'attributes'} <= set(instance) <= {
                             'schema_version', 'attributes', 'sensitive_attributes', 'private', 'dependencies', 'index_key',
                             'identity_schema_version', 'identity', 'create_before_destroy'}
                         and type(instance['schema_version']) is int and instance['schema_version'] >= 0
                         and type(instance['attributes']) is dict
                         and ('index_key' not in instance or type(instance['index_key']) in (int, str)))
            address = base + ('[' + receipt.canonical(instance['index_key']).decode() + ']' if 'index_key' in instance else '')
            receipt.need(address not in result)
            result[address] = (header, instance)
    return result


def state_shape(raw, packet, metadata, env, *, selection):
    """Independent cumulative full-state graph; baseline11 guard is unchanged."""
    state = private.decode(raw)
    receipt.need(type(state) is dict and state.get('terraform_version') == '1.16.4')
    # Empty/duplicate containers must not disappear when projecting baseline12
    # or flattening the cumulative runtime graph into instance addresses.
    flattened = instances(state)
    candidate = validate_packet(packet, wave=packet['wave'], git_sha=env['GITHUB_SHA'])
    chosen = packet[selection]
    candidate.update(own_runtime_wif_pool=chosen['pool'], own_runtime_wif_provider=chosen['provider'],
                     runtime_wif_grant_keys=chosen['grants'], owned_api_keys=chosen['apis'])
    runtime = graph.identities(candidate)
    baseline = copy.deepcopy(state)
    baseline['resources'] = [r for r in state['resources'] if r.get('module') not in {'module.runtime_wif', 'module.owned_apis'}]
    checks = state.get('check_results')
    if checks is not None:
        receipt.need(type(checks) is list)
        extra = [c for c in checks if c.get('object_kind') == 'resource' and c.get('config_addr', '').startswith(('module.runtime_wif.', 'module.owned_apis.'))]
        baseline['check_results'] = [c for c in checks if c not in extra]
        seen = set()
        for check in extra:
            receipt.need(type(check) is dict and set(check) == {'object_kind', 'config_addr', 'status', 'objects'}
                         and check['status'] == 'pass' and check['config_addr'] not in seen)
            seen.add(check['config_addr'])
            receipt.need(type(check['objects']) is list and bool(check['objects']))
            objects = set()
            for obj in check['objects']:
                receipt.need(type(obj) is dict and {'object_addr', 'status'} <= set(obj) <= {'object_addr', 'status', 'failure_messages'}
                             and obj['status'] == 'pass' and obj.get('failure_messages', []) == []
                             and obj['object_addr'] in runtime and obj['object_addr'] not in objects
                             and re.sub(r'\[(?:\d+|"[^"]*")\]', '', obj['object_addr']) == check['config_addr'])
                objects.add(obj['object_addr'])
    private.state_shape(receipt.canonical(baseline), imported=True)
    receipt.need(set(flattened) == private.BASELINE | {TABLE} | set(runtime))
    changes, planned = [], []
    for address, (header, instance) in flattened.items():
        if address in runtime:
            receipt.need(set(header) == {'module', 'mode', 'type', 'name', 'provider'} and header['mode'] == 'managed'
                         and header['provider'] == 'provider["registry.terraform.io/hashicorp/google"]'
                         and {'schema_version', 'attributes'} <= set(instance) <= {
                             'schema_version', 'attributes', 'sensitive_attributes', 'private', 'dependencies', 'index_key',
                             'identity_schema_version', 'identity', 'create_before_destroy'}
                         and type(instance['schema_version']) is int and instance['schema_version'] == 0
                         and instance.get('create_before_destroy', False) is False
                         and type(instance.get('sensitive_attributes', [])) is list
                         and type(instance.get('private', '')) is str
                         and type(instance.get('identity_schema_version', 0)) is int and instance.get('identity_schema_version', 0) == 0
                         and (instance.get('identity') is None or type(instance['identity']) is dict)
                         and type(instance.get('dependencies', [])) is list
                         and all(type(d) is str for d in instance.get('dependencies', [])))
        values = instance['attributes']
        changes.append(dict(address=address, mode=header['mode'], type=header['type'], provider_name=graph.GOOGLE,
                            change=dict(actions=['no-op'], before=values, after=values, after_unknown={})))
        planned.append(dict(address=address, mode=header['mode'], type=header['type'], provider_name=graph.GOOGLE,
                            schema_version=instance['schema_version'], values=values))
    plan = dict(format_version='1.2', terraform_version=state['terraform_version'], complete=True, errored=False,
                resource_changes=changes, planned_values={'root_module': {'resources': planned}},
                output_changes={k: dict(actions=['no-op'], before=v['value'], after=v['value'], after_unknown=False,
                    before_sensitive=v.get('sensitive', False), after_sensitive=v.get('sensitive', False)) for k, v in state['outputs'].items()})
    graph.validate(plan, packet=packet, wave=packet['wave'], git_sha=env['GITHUB_SHA'], metadata=metadata,
                   phase='post', execution_email=env['TF_VAR_export_function_execution_service_account_email'], state_phase=selection)
    return state


def capture(env, root, name, *, request=None):
    first = private.head(env, private.MAX_BYTES, request=request)
    path = root / name
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    os.close(fd)
    got = private.s3(env, 'get-object', '--key', private.STATE_KEY, '--range', f'bytes=0-{private.MAX_BYTES}', str(path), request=request)
    raw = private.private_bytes(path)
    receipt.need(got.get('VersionId') == first['VersionId'] and got.get('ETag') == first['ETag']
                 and got.get('ContentLength') == first['ContentLength'] == len(raw)
                 and private.head(env, private.MAX_BYTES, request=request) == first)
    private.absent(env, private.STATE_KEY + '.tflock', request=request)
    private.absent(env, private.STATE_KEY.rsplit('/', 1)[0] + '/workspaces/', request=request)
    return first, raw


def metadata_snapshot(env, packet, *, request=None):
    """Exact existing metadata GETs; no IAM permission probe or mutation."""
    request = request or google
    errors = []
    def get(host, path):
        try:
            status, value = request(path, env['GCP_SMOKE_ACCESS_TOKEN'], host=host)
            receipt.need(status == 200 and type(value) is dict and len(receipt.canonical(value)) <= private.MAX_BYTES)
            return value
        except Exception:
            errors.append(True)
            return {}
    table = get('bigquery.googleapis.com', private.TABLE_PATH)
    expected = packet['inventory']
    project = get('cloudresourcemanager.googleapis.com', 'v1/projects/' + env['TF_VAR_project_id'])
    observed = copy.deepcopy(expected)
    observed['project'] = {key: project.get(key) for key in ('projectId', 'projectNumber')}
    pool = expected['pool']['name']
    observed['pool'] = get('iam.googleapis.com', 'v1/' + pool)
    observed['provider'] = get('iam.googleapis.com', 'v1/' + expected['provider']['name'])
    observed['pool_attestation_rules'] = get('iam.googleapis.com', 'v1/' + pool + ':listAttestationRules')
    observed['iam_policy'] = get('iam.googleapis.com', 'v1/' + expected['iam_policy_resource'] + ':getIamPolicy?options.requestedPolicyVersion=3')
    services, token, seen = [], '', set()
    for _ in range(16):
        page = get('serviceusage.googleapis.com', 'v1/projects/' + expected['project']['projectNumber'] + '/services?filter=state%3AENABLED&pageSize=200' +
                   ('&pageToken=' + quote(token, safe='') if token else ''))
        receipt.need(set(page) <= {'services', 'nextPageToken'} and type(page.get('services', [])) is list)
        services.extend(page.get('services', []))
        token = page.get('nextPageToken', '')
        receipt.need(type(token) is str and len(token) <= 2048)
        if not token:
            break
        receipt.need(token not in seen)
        seen.add(token)
    receipt.need(not token)
    observed['enabled_services'] = {'services': services}
    receipt.need(not errors)
    prepare(observed)
    # Complete IAM bindings/conditions, WIF trust and enabled API inventory are
    # retained, including unrelated entries. Pagination/order is normalized.
    def normalized(value):
        value = copy.deepcopy(value)
        value['project'] = {k: value['project'][k] for k in ('projectId', 'projectNumber')}
        value['pool_attestation_rules'] = {'attestationRules': value['pool_attestation_rules'].get('attestationRules', [])}
        value['enabled_services'] = {'services': sorted(value['enabled_services']['services'], key=lambda s: s['name'])}
        return value
    receipt.need(normalized(observed) == normalized(expected))
    receipt.check_current_binding(packet['receipt_binding'], env, table)
    return dict(inventory=normalized(observed), history=stable_table_metadata(table))


def observed_transports(request, metadata_request):
    reads = {'state': 0, 'metadata': 0}
    def state_read(*args):
        reads['state'] += 1
        return (request or private.aws)(*args)
    def metadata_read(*args, **kwargs):
        reads['metadata'] += 1
        return (metadata_request or google)(*args, **kwargs)
    return reads, state_read, metadata_read


def before(env, *, request=None, metadata_request=None):
    root, packet, metadata = context(env)
    value = bundle(env, root, packet)
    commit = receipt.commitment(packet['receipt_binding'], value)
    if env['GITHUB_JOB'] == 'apply':
        receipt.need(env.get('EXPECTED_RUNTIME_INPUT_COMMITMENT') == commit)
    reads, state_read, metadata_read = observed_transports(request, metadata_request)
    remote_metadata = metadata_snapshot(env, packet, request=metadata_read)
    private.write_private(root / 'runtime-metadata-before.json', remote_metadata)
    head, raw = capture(env, root, 'runtime-state-before.json', request=state_read)
    state = state_shape(raw, packet, metadata, env, selection='adopted')
    verified = private.private_json(root / provenance.VERIFIED)
    receipt.need(receipt.state_commitment(packet['receipt_binding'], state) == verified['last_state_after_commitment'])
    saved = dict(inputs=value, input_commitment=commit, state_sha256=hashlib.sha256(raw).hexdigest(), head=head, provenance=verified, observed_reads=reads)
    private.write_private(root / BEFORE, saved)
    output(env, 'runtime_input_commitment', commit)


def require_fresh_inputs(env, packet):
    root, current, metadata = context(env)
    receipt.need(current == packet)
    saved = private.private_json(root / BEFORE)
    receipt.need(saved['inputs'] == bundle(env, root, packet)
                 and saved['provenance'] == private.private_json(root / provenance.VERIFIED)
                 and saved['input_commitment'] == receipt.commitment(packet['receipt_binding'], saved['inputs']))
    raw = private.private_bytes(root / 'runtime-state-before.json')
    state = state_shape(raw, packet, metadata, env, selection='adopted')
    receipt.need(hashlib.sha256(raw).hexdigest() == saved['state_sha256']
                 and receipt.state_commitment(packet['receipt_binding'], state) == saved['provenance']['last_state_after_commitment'])
    return root, saved


def approve_plan(env, plan, summary, *, phase):
    root, packet, metadata = context(env)
    require_fresh_inputs(env, packet)
    expected = graph.validate(plan, packet=packet, wave=packet['wave'], git_sha=env['GITHUB_SHA'], metadata=metadata,
                              phase=phase, execution_email=env['TF_VAR_export_function_execution_service_account_email'])
    expected = dict(expected, resources=[])
    receipt.need(summary == expected)
    private.write_private(root / ('runtime-approved-plan.json' if phase == 'before' else 'runtime-approved-post.json'),
                          dict(summary=expected, inputs=receipt.commitment(packet['receipt_binding'], bundle(env, root, packet))))


def preapply(env, *, request=None, metadata_request=None):
    root, packet, _ = context(env)
    receipt.need(env['MODE'] == 'apply' and env['GITHUB_JOB'] == 'apply')
    _, saved = require_fresh_inputs(env, packet)
    reads, state_read, metadata_read = observed_transports(request, metadata_request)
    head, raw = capture(env, root, 'runtime-state-preapply.json', request=state_read)
    receipt.need(head == saved['head'] and raw == private.private_bytes(root / 'runtime-state-before.json'))
    receipt.need(metadata_snapshot(env, packet, request=metadata_read) == private.private_json(root / 'runtime-metadata-before.json'))
    summary = private.private_bytes(root / 'sanitized-replan.json')
    receipt.need(hashlib.sha256(summary).hexdigest() == env.get('APPROVED_SUMMARY_DIGEST'))
    approved = private.private_json(root / 'runtime-approved-plan.json')
    receipt.need(private.decode(summary) == approved['summary'] and approved['inputs'] == saved['input_commitment'])
    seal = dict(input_commitment=saved['input_commitment'], tfplan_sha256=hashlib.sha256(private.private_bytes(root / 'tfplan')).hexdigest(),
                summary_sha256=hashlib.sha256(summary).hexdigest(), observed_reads=reads)
    private.write_private(root / SEALED, seal)


def check_seal(env):
    root, packet, _ = context(env)
    receipt.need(env['MODE'] == 'apply' and env['GITHUB_JOB'] == 'apply')
    _, saved = require_fresh_inputs(env, packet)
    seal = private.private_json(root / SEALED)
    receipt.need(set(seal) == {'input_commitment', 'tfplan_sha256', 'summary_sha256', 'observed_reads'}
                 and seal['input_commitment'] == saved['input_commitment']
                 and seal['tfplan_sha256'] == hashlib.sha256(private.private_bytes(root / 'tfplan')).hexdigest()
                 and seal['summary_sha256'] == env.get('APPROVED_SUMMARY_DIGEST')
                 == hashlib.sha256(private.private_bytes(root / 'sanitized-replan.json')).hexdigest())


def state_delta(old, new, packet):
    prior_containers, current_containers = containers(old), containers(new)
    # A second grant/API instance extends its existing unique container; it
    # never splits or replaces any prior container/header/membership.
    receipt.need(set(prior_containers) <= set(current_containers)
                 and all({k: v for k, v in current_containers[key].items() if k != 'instances'} ==
                         {k: v for k, v in value.items() if k != 'instances'}
                         for key, value in prior_containers.items()))
    receipt.need(all(new[k] == old[k] for k in ('lineage', 'version', 'terraform_version', 'outputs')))
    if packet['wave'] == 'noop':
        receipt.need(new == old)
        return
    receipt.need(new['serial'] > old['serial'])  # Existing state-only delta contract, no guessed serial+1.
    previous, current = instances(old), instances(new)
    receipt.need(set(previous) < set(current) and len(current) == len(previous) + 1
                 and all(current[k] == v for k, v in previous.items()))
    added = set(current) - set(previous)
    def preserved_checks(state):
        results = copy.deepcopy(state.get('check_results'))
        if results is None:
            return None
        for check in results:
            if check['object_kind'] == 'resource':
                check['objects'] = [o for o in check['objects'] if o['object_addr'] not in added]
        return [c for c in results if c['object_kind'] != 'resource' or c['objects']]
    receipt.need(preserved_checks(new) == preserved_checks(old))


def after(env, *, request=None, metadata_request=None):
    # Bind context/nonce/backend before safety reads. Later freshness or outcome
    # failures must not suppress the other independent read attempts.
    root, packet, metadata = context(env, safety=True)
    saved = private.private_json(root / BEFORE)
    old_raw = private.private_bytes(root / 'runtime-state-before.json')
    receipt.need(saved['state_sha256'] == hashlib.sha256(old_raw).hexdigest())
    reads, state_read, metadata_read = observed_transports(request, metadata_request)
    checks = {'state': False, 'metadata': False, 'lock': False, 'workspace': False}
    applied = env['GITHUB_JOB'] == 'apply'
    try:
        head, raw = capture(env, root, 'runtime-state-after.json', request=state_read)
        new = state_shape(raw, packet, metadata, env, selection='selected' if applied else 'adopted')
        if applied:
            state_delta(private.decode(old_raw), new, packet)
        else:
            receipt.need(head == saved['head'] and raw == old_raw)
        checks['state'] = checks['lock'] = checks['workspace'] = True
    except Exception:
        pass
    # Independent absence checks survive a corrupt/read-failed state body.
    for name, prefix in [('lock', private.STATE_KEY + '.tflock'), ('workspace', private.STATE_KEY.rsplit('/', 1)[0] + '/workspaces/')]:
        if not checks[name]:
            try:
                private.absent(env, prefix, request=state_read)
                checks[name] = True
            except Exception:
                pass
    try:
        current = metadata_snapshot(env, packet, request=metadata_read)
        private.write_private(root / 'runtime-metadata-after.json', current)
        checks['metadata'] = current == private.private_json(root / 'runtime-metadata-before.json')
    except Exception:
        pass
    private.write_private(root / 'runtime-execution-after.json', dict(checks=checks, observed_reads=reads,
        outcomes={k: env.get(k, 'unknown') for k in ('BACKEND_INIT_OUTCOME', 'TERRAFORM_PLAN_OUTCOME', 'PLAN_VALIDATION_OUTCOME', 'APPLY_OUTCOME', 'POST_PLAN_OUTCOME')},
        provider_requests='unknown', actual_cost='unknown'))
    with open(env['GITHUB_STEP_SUMMARY'], 'a') as handle:
        handle.write('Runtime wave safety: ' + '; '.join(k + '=' + ('PASS' if v else 'STOP') for k, v in checks.items()) +
                     f"; state_reads={reads['state']}; metadata_reads={reads['metadata']}; provider_requests=unknown; actual_cost=unknown\n")
    require_fresh_inputs(env, packet)
    receipt.need(all(checks.values()))
    if not applied:
        receipt.need(all(env.get(k) == 'success' for k in ('BACKEND_INIT_OUTCOME', 'TERRAFORM_PLAN_OUTCOME', 'PLAN_VALIDATION_OUTCOME')))
        approved = private.private_json(root / 'runtime-approved-plan.json')
        receipt.need(approved['inputs'] == saved['input_commitment'])
        return
    check_seal(env)
    receipt.need(env.get('APPLY_OUTCOME') == env.get('POST_PLAN_OUTCOME') == 'success'
                 and env.get('POST_PLAN_EXIT_CODE') == '0')
    post = private.private_json(root / 'runtime-approved-post.json')
    receipt.need(post['inputs'] == saved['input_commitment'] and post['summary'] == private.private_json(root / 'post-apply-summary.json'))
    prior = saved['provenance']
    candidate = prepare(packet['inventory'])
    machine = dict(schema_version=1, repository_id=receipt.REPOSITORY_ID, source_sha=env['GITHUB_SHA'],
        run_id=int(env['GITHUB_RUN_ID']), run_attempt=1, job_role='apply', environment='terraform-dev-apply',
        target='dev', root_contract='gcp-dev-default', operation='apply', wave=packet['wave'], phase='post',
        prior_receipt_sha256=prior['last_receipt_sha256'], prior_scope_commitment=prior['adopted_scope_commitment'],
        prior_state_commitment=prior['last_state_after_commitment'], state_after_commitment=receipt.state_commitment(packet['receipt_binding'], new),
        scope_commitment=receipt.commitment(packet['receipt_binding'], provenance.scope(packet['receipt_binding'], candidate, packet['selected'])),
        resource_count=len(instances(new)), import_before_count=int(packet['wave'] != 'noop'), import_post_count=0,
        checks={k: True for k in receipt.CHECKS}, accounting=dict(terraform_outcomes={'apply': env['APPLY_OUTCOME'], 'post_plan': env['POST_PLAN_OUTCOME']},
        post_state_reads=reads['state'], post_metadata_reads=reads['metadata'], provider_requests='unknown', actual_cost='unknown'))
    private.write_private(root / RECORD, receipt.validate_receipt(machine))


def main():
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument('--phase', required=True, choices=('before', 'after', 'preapply', 'check-seal'))
        args = parser.parse_args()
        {'before': before, 'after': after, 'preapply': preapply, 'check-seal': check_seal}[args.phase](dict(os.environ))
    except Exception:
        print('STOP: runtime execution receipt rejected; private diagnostic suppressed.', file=sys.stderr)
        return 3
    print('Runtime private execution checks passed; values suppressed.')
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
