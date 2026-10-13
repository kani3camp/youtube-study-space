#!/usr/bin/env python3
"""Separate, default-off current-state root; never a historical import receipt.

Only the protected post-noop verifier can create the private record. Public
bytes contain opaque bindings, never backend, nonce, state hashes or HEAD.
GitHub authenticity comes from a separately reviewed source catalog, not HMAC.
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
import hashlib
import hmac
from io import BytesIO
import os
from pathlib import Path
import stat
import sys
import zipfile

import terraform_ownership_receipt as ownership
import terraform_history_plan_receipt as private
import terraform_history_post_noop_receipt as noop
from terraform_history_table_metadata import stable_table_metadata

TYPE = 'dev-history12-current-state-v1'
TRANSPORT = 'current-state-artifact-v1'
PUBLIC_FILE = 'current-state-attestation.json'
PUBLIC_DIRECTORY = 'current-state-public'
RECORD = 'current-state-attestation-record.json'
EMIT_STEP = 'Emit verified current-state attestation'
PUBLISH_STEP = 'Publish sanitized current-state attestation'
REQUIRED_STEPS = [
    'Checkout trusted commit',
    'Check bounded history execution evidence before authentication or locks',
    'Check private current-state binding before credentials',
    'Verify exact AWS execution identity before GCP authentication',
    'Verify development identity boundaries without public identifiers',
    'Read canonical history metadata privately',
    'Check fresh current-state metadata binding',
    'Bind the independently adopted history state baseline',
    'Initialize remote backend without public output',
    'Create saved plan without public output',
    'Sanitize and enforce the selected protected contract',
    'Verify independent post-import no-op and unchanged state',
    EMIT_STEP, PUBLISH_STEP, 'Cleanup sensitive temporary files',
]
CHECKS = {'full_root_noop', 'state_bytes_head_unchanged', 'metadata_unchanged',
          'lock_absent', 'workspace_absent', 'exact_guards', 'fresh_execution'}
KEYS = {'schema_version', 'attestation_type', 'repository_id', 'source_sha',
        'run_id', 'run_attempt', 'job_role', 'environment', 'target', 'root_contract',
        'operation', 'phase', 'historical_delta_attested', 'issued_utc', 'expires_utc',
        'observed_utc', 'resource_count', 'terraform_version', 'counts', 'checks',
        'scope_commitment', 'state_after_commitment', 'state_observation_commitment', 'accounting'}


def scope(value):
    ownership.binding(value)
    return dict(contract=TYPE, backend=value['backend'],
                history=stable_table_metadata(value['history_metadata']))


def observation_commitment(value, head, raw):
    ownership.binding(value)
    ownership.need(type(head) is dict and type(raw) is bytes and 0 < len(raw) <= private.MAX_BYTES)
    observation = dict(head=head, raw_state_sha256=hashlib.sha256(raw).hexdigest())
    return hmac.new(bytes.fromhex(value['nonce']), b'ownership-state-observation-v1\0'
                    + ownership.canonical(observation), hashlib.sha256).hexdigest()


def validate_attestation(value):
    ownership.need(type(value) is dict and set(value) == KEYS
                   and type(value['schema_version']) is int and value['schema_version'] == 1
                   and value['attestation_type'] == TYPE
                   and type(value['repository_id']) is int and value['repository_id'] == ownership.REPOSITORY_ID
                   and ownership.hex_value(value['source_sha'], 40) and ownership.positive(value['run_id'])
                   and type(value['run_attempt']) is int and value['run_attempt'] == 1)
    ownership.need(value['job_role'] == 'plan' and value['environment'] == 'terraform-dev-plan'
                   and value['target'] == 'dev' and value['root_contract'] == 'gcp-dev-default'
                   and value['operation'] == 'protected-plan' and value['phase'] == 'current-state'
                   and value['historical_delta_attested'] is False
                   and type(value['resource_count']) is int and value['resource_count'] == 12
                   and value['terraform_version'] == '1.16.4')
    issued, observed, expires = (private.timestamp(value[k]) for k in ('issued_utc', 'observed_utc', 'expires_utc'))
    ownership.need(issued <= observed < expires <= issued + timedelta(hours=24))
    ownership.need(type(value['counts']) is dict and value['counts'] == noop.EXPECTED_COUNTS
                   and all(type(v) is int for v in value['counts'].values())
                   and type(value['checks']) is dict and set(value['checks']) == CHECKS
                   and all(v is True for v in value['checks'].values()))
    ownership.need(all(ownership.hex_value(value[k], 64) for k in
                       ('scope_commitment', 'state_after_commitment', 'state_observation_commitment')))
    ownership.need(type(value['accounting']) is dict and value['accounting'] == dict(
        terraform_outcomes=dict(init='success', plan='success', validation='success', plan_exit_code=0),
        explicit_state_reads=10, explicit_metadata_reads=2, provider_requests='unknown',
        workflow_requests='unknown', actual_cost='unknown'))
    accounting = value['accounting']
    ownership.need(type(accounting['explicit_state_reads']) is int
                   and type(accounting['explicit_metadata_reads']) is int
                   and type(accounting['terraform_outcomes']['plan_exit_code']) is int)
    ownership.need(len(ownership.canonical(value)) + 1 <= ownership.MAX_RECEIPT_BYTES)
    return value


def reference(value):
    ownership.need(type(value) is dict and set(value) == ownership.REFERENCE_KEYS
                   and all(ownership.positive(value[k]) for k in ('run_id', 'attempt', 'job_id', 'artifact_id'))
                   and value['attempt'] == 1 and value['transport'] == TRANSPORT
                   and ownership.hex_value(value['source_sha'], 40)
                   and all(ownership.hex_value(value[k], 64) for k in ('receipt_sha256', 'artifact_sha256')))
    return value


def load_catalog(value):
    ownership.need(type(value) is dict and set(value) == {'schema_version', 'issuers'}
                   and type(value['schema_version']) is int and value['schema_version'] == 1
                   and type(value['issuers']) is dict)
    for sha, item in value['issuers'].items():
        ownership.need(ownership.hex_value(sha, 40) and type(item) is dict and set(item) == {
            'workflow_id', 'job_names', 'environment', 'reviewer_ids', 'attestation_step',
            'publisher_step', 'required_steps', 'purpose'})
        ownership.need(ownership.positive(item['workflow_id']) and item['purpose'] == TYPE
                       and type(item['job_names']) is dict and set(item['job_names']) == {'plan', 'apply'}
                       and all(type(v) is str and v for v in item['job_names'].values())
                       and len(set(item['job_names'].values())) == 2)
        environment = item['environment']
        ownership.need(type(environment) is dict and set(environment) == {'id', 'name'}
                       and ownership.positive(environment['id']) and environment['name'] == 'terraform-dev-plan')
        reviewers, steps = item['reviewer_ids'], item['required_steps']
        ownership.need(type(reviewers) is list and reviewers and all(ownership.positive(i) for i in reviewers)
                       and len(reviewers) == len(set(reviewers))
                       and type(steps) is list and all(type(s) is str and s for s in steps)
                       and len(steps) == len(set(steps)) and set(REQUIRED_STEPS) <= set(steps)
                       and [s for s in steps if s in REQUIRED_STEPS] == REQUIRED_STEPS
                       and item['attestation_step'] == EMIT_STEP and item['publisher_step'] == PUBLISH_STEP)
    return value


def artifact_name(run_id, source_sha):
    ownership.need(ownership.positive(run_id) and ownership.hex_value(source_sha, 40))
    return f'current-state-attestation-v1-{run_id}-1-{source_sha}'


def archive_attestation(raw, archive_digest, attestation_digest):
    ownership.need(type(raw) is bytes and 0 < len(raw) <= ownership.MAX_ARCHIVE_BYTES
                   and ownership.hex_value(archive_digest, 64)
                   and hashlib.sha256(raw).hexdigest() == archive_digest)
    with zipfile.ZipFile(BytesIO(raw)) as archive:
        entries = archive.infolist()
        ownership.need(len(entries) == 1)
        entry = entries[0]
        ownership.need(entry.filename == PUBLIC_FILE and entry.orig_filename == PUBLIC_FILE
                       and not entry.is_dir() and not (entry.flag_bits & 1) and not (entry.external_attr & 0x10)
                       and stat.S_IFMT(entry.external_attr >> 16) in {0, stat.S_IFREG}
                       and 0 < entry.file_size <= ownership.MAX_RECEIPT_BYTES
                       and 0 < entry.compress_size <= ownership.MAX_ARCHIVE_BYTES
                       and entry.compress_type in {zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED})
        with archive.open(entry) as stream:
            body = stream.read(ownership.MAX_RECEIPT_BYTES + 1)
        ownership.need(len(body) == entry.file_size and len(body) <= ownership.MAX_RECEIPT_BYTES)
    value = validate_attestation(ownership.decode(body))
    ownership.need(body == ownership.canonical(value) + b'\n' and ownership.digest(value) == attestation_digest)
    return value


def artifact_metadata(value, ref, job, *, now=None):
    # Reuse all legacy metadata/timing limits with only the explicit artifact
    # name replaced; original transport/type/schema validators are untouched.
    ownership.need(type(value) is dict and value.get('name') == artifact_name(ref['run_id'], ref['source_sha']))
    ownership.artifact_metadata(dict(value, name=ownership.artifact_name(ref['run_id'], ref['source_sha'])),
                                ref, job, now=now)
    return value


def authenticate(ref, catalog, *, request, now=None):
    """Bounded exact GETs, no log fallback; protected plan and no executed apply."""
    reference(ref)
    issuer = load_catalog(catalog)['issuers'].get(ref['source_sha'])
    ownership.need(issuer is not None)
    base = f"/repos/{ownership.REPOSITORY}/actions/runs/{ref['run_id']}"
    initial = request(base)
    ownership.check_run(initial, ref, issuer)
    ownership.check_run(request(base + '/attempts/1'), ref, issuer)
    jobs, total = [], None
    for page in range(1, 11):
        response = request(base + f'/attempts/1/jobs?per_page=100&page={page}')
        ownership.need(type(response) is dict and set(response) == {'total_count', 'jobs'}
                       and type(response['total_count']) is int and 0 < response['total_count'] <= 1000
                       and type(response['jobs']) is list and 0 < len(response['jobs']) <= 100)
        total = response['total_count'] if total is None else total
        ownership.need(total == response['total_count'])
        jobs.extend(response['jobs'])
        ownership.need(len(jobs) <= total)
        if len(jobs) == total:
            break
        ownership.need(len(response['jobs']) == 100)
    ownership.need(len(jobs) == total and all(type(j) is dict and ownership.positive(j.get('id'))
        and type(j.get('run_id')) is int and j['run_id'] == ref['run_id']
        and j.get('head_sha') == ref['source_sha'] for j in jobs)
        and len({j['id'] for j in jobs}) == len(jobs))
    plans = [j for j in jobs if j.get('name') == issuer['job_names']['plan']]
    applies = [j for j in jobs if j.get('name') == issuer['job_names']['apply']]
    ownership.need(len(plans) == 1 and len(applies) <= 1
                   and all(j.get('status') == 'completed' and j.get('conclusion') == 'skipped'
                           and j.get('steps', []) == [] for j in applies))
    # Reject an apply step even if a job was renamed to evade the name pin.
    ownership.need(not any(s.get('name') == 'Apply the locally re-created saved plan'
                           and s.get('conclusion') != 'skipped' for j in jobs for s in j.get('steps', [])))
    job = plans[0]
    ownership.need(job['id'] == ref['job_id'] and job.get('head_branch') == ownership.BRANCH
                   and job.get('status') == 'completed' and job.get('conclusion') == 'success'
                   and ownership.timestamp(job['started_at']) <= ownership.timestamp(job['completed_at']))
    steps = job.get('steps')
    ownership.need(type(steps) is list and all(type(s) is dict and ownership.positive(s.get('number')) for s in steps)
                   and len({s['number'] for s in steps}) == len(steps))
    ownership.need(all(j['id'] == job['id'] for j in jobs for s in j.get('steps', [])
                       if s.get('name') in {EMIT_STEP, PUBLISH_STEP} and s.get('conclusion') != 'skipped'))
    positions = []
    for name in issuer['required_steps']:
        matches = [s for s in steps if s.get('name') == name]
        ownership.need(len(matches) == 1 and matches[0].get('status') == 'completed'
                       and matches[0].get('conclusion') == 'success')
        positions.append(matches[0]['number'])
    ownership.need(positions == sorted(positions))
    listing = request(base + '/artifacts?per_page=100&page=1')
    ownership.need(type(listing) is dict and set(listing) == {'total_count', 'artifacts'}
                   and type(listing['total_count']) is int and listing['total_count'] == 1
                   and type(listing['artifacts']) is list and len(listing['artifacts']) == 1)
    path = f"/repos/{ownership.REPOSITORY}/actions/artifacts/{ref['artifact_id']}"
    artifact = artifact_metadata(request(path), ref, job, now=now)
    ownership.need(listing['artifacts'][0] == artifact)
    raw = request(path + '/zip', archive=True)
    ownership.need(type(raw) is bytes and len(raw) == artifact['size_in_bytes'])
    value = archive_attestation(raw, ref['artifact_sha256'], ref['receipt_sha256'])
    ownership.need(value['run_id'] == ref['run_id'] and value['source_sha'] == ref['source_sha']
                   and private.timestamp(value['issued_utc']) <= ownership.timestamp(job['started_at'])
                   <= private.timestamp(value['observed_utc']) <= ownership.timestamp(job['completed_at'])
                   < private.timestamp(value['expires_utc']))
    reviews = request(base + '/approvals')
    ownership.need(type(reviews) is list and 0 < len(reviews) <= 100)
    matches, environment = [], issuer['environment']
    for review in reviews:
        ownership.need(type(review) is dict and type(review.get('environments')) is list)
        for actual in review['environments']:
            ownership.need(type(actual) is dict)
            if actual.get('id') == environment['id'] or actual.get('name') == environment['name']:
                ownership.need(type(actual.get('id')) is int and actual['id'] == environment['id']
                               and actual.get('name') == environment['name'])
                matches.append(review)
    ownership.need(matches and all(r.get('state') == 'approved' and type(r.get('user')) is dict
        and ownership.positive(r['user'].get('id')) and r['user']['id'] in issuer['reviewer_ids'] for r in matches))
    ownership.need(artifact_metadata(request(path), ref, job, now=now) == artifact
                   and request(base + '/artifacts?per_page=100&page=1') == listing)
    final = request(base)
    ownership.check_run(final, ref, issuer)
    ownership.need(final == initial)
    return value, dict(started_at=job['started_at'], completed_at=job['completed_at'])


def context(env):
    ownership.context(env)
    ownership.need(env.get('GITHUB_JOB') == 'plan' and env.get('MODE') == 'plan'
                   and env.get('HISTORY_TARGET') == 'dev' and env.get('HISTORY_POST_NOOP') == 'true'
                   and env.get('CURRENT_STATE_ATTESTATION') == 'true'
                   and env.get('DEV_CURRENT_STATE_ATTESTATION_EMITTER_ENABLED') == 'true'
                   and env.get('OWNERSHIP_WAVE', 'none') == 'none'
                   and env.get('OWNERSHIP_ROOT_BASIS', 'original-history-import') == 'original-history-import'
                   and not env.get('HISTORY_IMPORT_APPROVAL', '')
                   and env.get('TF_VAR_project_id') == 'test-youtube-study-space'
                   and env.get('TF_VAR_manage_user_activity_history') == 'true')


def check_binding(env, *, fresh=False):
    context(env)
    envelope = ownership.decode(env.get('RUNTIME_OWNERSHIP_PACKET_JSON', ''))
    ownership.need(type(envelope) is dict and set(envelope) == {'schema_version', 'attestation_type', 'receipt_binding'}
                   and type(envelope['schema_version']) is int and envelope['schema_version'] == 1
                   and envelope['attestation_type'] == TYPE)
    metadata = private.private_json(Path(env['RUNNER_TEMP']) / 'user-history-post.json') if fresh else None
    ownership.check_current_binding(envelope['receipt_binding'], env, metadata)
    return envelope['receipt_binding']


def exact_summary(summary, sha):
    ownership.need(type(summary) is dict and set(summary) == {'schema_version', 'git_sha', 'environment',
        'terraform_version', 'policy', 'policy_passed', 'counts', 'resources', 'drift', 'violations'}
        and type(summary['schema_version']) is int and summary['schema_version'] == 1
        and summary['terraform_version'] == '1.16.4' and type(summary['counts']) is dict
        and all(type(v) is int for v in summary['counts'].values()))
    noop.exact_noop_summary(summary, sha)
    ownership.need(all(set(item) == {'address', 'mode', 'action', 'import'} for item in summary['resources']))


def record_verified(env, initial, head, raw, metadata):
    """Called only after every independent post-noop check passed."""
    value = check_binding(env, fresh=True)
    root, admission = noop.context(env)
    exact_summary(private.private_json(root / 'sanitized-plan.json'), env['GITHUB_SHA'])
    state = private.state_shape(raw, imported=True)
    ownership.need(state['terraform_version'] == '1.16.4' and raw == private.private_bytes(root / noop.STATE_BEFORE)
                   and initial['head'] == head and stable_table_metadata(metadata) == initial['stable_metadata'])
    profile = admission['profile']
    attestation = dict(schema_version=1, attestation_type=TYPE, repository_id=ownership.REPOSITORY_ID,
        source_sha=env['GITHUB_SHA'], run_id=int(env['GITHUB_RUN_ID']), run_attempt=1,
        job_role='plan', environment='terraform-dev-plan', target='dev', root_contract='gcp-dev-default',
        operation='protected-plan', phase='current-state', historical_delta_attested=False,
        issued_utc=profile['issued_utc'], expires_utc=profile['expires_utc'],
        observed_utc=datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
        resource_count=12, terraform_version='1.16.4', counts=noop.EXPECTED_COUNTS,
        checks={k: True for k in CHECKS}, scope_commitment=ownership.commitment(value, scope(value)),
        state_after_commitment=ownership.state_commitment(value, state),
        state_observation_commitment=observation_commitment(value, head, raw),
        accounting=dict(terraform_outcomes=dict(init='success', plan='success', validation='success', plan_exit_code=0),
            explicit_state_reads=10, explicit_metadata_reads=2, provider_requests='unknown',
            workflow_requests='unknown', actual_cost='unknown'))
    private.write_private(root / RECORD, validate_attestation(attestation))


def emit(env):
    binding = check_binding(env, fresh=True)
    root, admission = noop.context(env)
    value = validate_attestation(private.private_json(root / RECORD))
    raw = private.private_bytes(root / noop.STATE_AFTER)
    initial = private.private_json(root / noop.BEFORE)
    exact_summary(private.private_json(root / 'sanitized-plan.json'), env['GITHUB_SHA'])
    state = private.state_shape(raw, imported=True)
    ownership.need(value['source_sha'] == env['GITHUB_SHA'] and value['run_id'] == int(env['GITHUB_RUN_ID'])
                   and value['issued_utc'] == admission['profile']['issued_utc']
                   and value['expires_utc'] == admission['profile']['expires_utc']
                   and value['scope_commitment'] == ownership.commitment(binding, scope(binding))
                   and value['state_after_commitment'] == ownership.state_commitment(binding, state)
                   and value['state_observation_commitment'] == observation_commitment(binding, initial['head'], raw)
                   and raw == private.private_bytes(root / noop.STATE_BEFORE)
                   and stable_table_metadata(private.private_json(root / 'history-post-noop-table-after.json'))
                       == initial['stable_metadata'])
    ownership.need(root.stat().st_uid == os.geteuid())
    directory = root / PUBLIC_DIRECTORY
    directory.mkdir(mode=0o700)
    fd = os.open(directory / PUBLIC_FILE, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(ownership.canonical(value) + b'\n')
    print('Current-state attestation staged; no historical delta claimed.')


def main():
    try:
        ownership.need(sys.argv[1:] in (['check-binding'], ['check-binding', '--fresh'], ['emit']))
        if sys.argv[1] == 'emit':
            emit(dict(os.environ))
        else:
            check_binding(dict(os.environ), fresh='--fresh' in sys.argv)
            print('Private current-state binding checked; values suppressed.')
    except Exception:
        print('STOP: current-state attestation rejected; private diagnostic suppressed.', file=sys.stderr)
        return 3
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
