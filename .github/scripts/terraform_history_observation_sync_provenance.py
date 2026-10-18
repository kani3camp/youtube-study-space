#!/usr/bin/env python3
"""Authenticate only the separate sanitized observation-preview type, GET only.

This is neither an issuer nor permission to execute an apply. The consumer must
obtain its own owner-approved apply Environment, fresh observations and seal.
No raw State, private digest, identity, nonce or drift value belongs in a proof.
"""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
import hashlib
import hmac
from io import BytesIO
import math
import re
import stat
import struct
import zipfile
import zlib

import terraform_history_observation_sync_profile as profile_module
import terraform_ownership_receipt as ownership
from terraform_plan_summary import COUNT_KEYS

TYPE = 'dev-history-observation-sync-preview-v1'
TRANSPORT = profile_module.TRANSPORT
PUBLIC_FILE = 'history-observation-sync-preview.json'
PUBLIC_DIRECTORY = 'history-observation-sync-public'
MAX_PROOF_BYTES = 8192
MAX_ARCHIVE_BYTES = 32768
MAX_PRIVATE_INTENT_BYTES = 16 * 1024 * 1024
MAX_PRIVATE_INTENT_DEPTH = 64
MAX_PRIVATE_INTENT_NODES = 300000
WORKFLOW_ID = 325439161
ENVIRONMENT_ID = 23410074023
JOB_NAME = 'GCP Terraform Authenticated / History observation sync preview (dev)'
APPLY_JOB_NAME = 'GCP Terraform Authenticated / History observation sync apply: ETAG + LAST_MODIFIED_TIME; check contents unchanged (dev)'
REQUIRED_STEPS = (
    'Validate dedicated sync profile and private binding',
    'Verify dedicated sync OIDC identity',
    'Configure sync AWS backend credential',
    'Verify exact sync AWS identity',
    'Configure sync GCP provider credential',
    'Build and verify private observation preview',
    'Remove all private sync material',
    'Validate sanitized sync preview freshness before publish',
    'Publish dedicated sanitized sync preview',
    'Post Configure sync GCP provider credential',
)
COUNTS = {key: (12 if key == 'no-op' else 1 if key == 'drift' else 0) for key in COUNT_KEYS}
CHECKS = {'source_inputs_verified', 'state_bytes_head_unchanged', 'metadata_unchanged',
          'lock_absent', 'workspace_absent', 'exact_observation_drift',
          'native_candidate_verified', 'fresh_execution'}
COMMITMENT_PURPOSES = {'scope', 'state_before', 'state_expected', 'metadata', 'intent'}
KEYS = {'schema_version', 'kind', 'repository_id', 'source_sha', 'run_id', 'run_attempt',
        'job_role', 'environment', 'target', 'root_contract', 'request_id', 'issued_utc',
        'expires_utc', 'observed_utc', 'counts', 'drift_alias', 'field_codes', 'commitments',
        'raw_candidate_state_verified', 'execution_permitted', 'migration_ready', 'checks', 'accounting'}


def need(value):
    if not value:
        raise ValueError('history observation sync provenance STOP')


canonical = profile_module.canonical
decode = profile_module.decode
reference = profile_module.reference
timestamp = profile_module.timestamp


def digest(value):
    return hashlib.sha256(canonical(value)).hexdigest()


def bounded_private_intent(value):
    """Bound private JSON while retaining finite fractional Terraform inputs.

Public request/proof decoders still reject all floats. Private normal-plan
variables include fractional quota thresholds, so the HMAC domain must retain
their exact JSON values. This is a structural limit, not semantic validation;
the controller must validate field types and duplicate JSON keys first.
"""
    pending, remaining = [(value, 0)], MAX_PRIVATE_INTENT_NODES
    while pending:
        item, level = pending.pop()
        remaining -= 1
        need(remaining >= 0 and level <= MAX_PRIVATE_INTENT_DEPTH)
        if type(item) is dict:
            need(all(type(key) is str and len(key) <= MAX_PRIVATE_INTENT_BYTES for key in item))
            need(len(item) + len(pending) <= remaining)
            pending.extend((child, level + 1) for child in item.values())
        elif type(item) is list:
            need(len(item) + len(pending) <= remaining)
            pending.extend((child, level + 1) for child in item)
        elif type(item) is str:
            need(len(item) <= MAX_PRIVATE_INTENT_BYTES)
        elif type(item) is int:
            need(abs(item) <= 2 ** 63 - 1)
        elif type(item) is float:
            need(math.isfinite(item))
        else:
            # Exact types keep bool distinct from int/float and reject objects,
            # Decimal, bytes, numeric subclasses and non-JSON containers.
            need(item is None or type(item) is bool)
    need(len(canonical(value)) <= MAX_PRIVATE_INTENT_BYTES)
    return value


def commitment(binding, purpose, value):
    """Purpose-separated HMAC over private intent, never a bare private hash.

The controller supplies the complete validated State/HEAD/candidate/metadata/
input intent. This primitive authenticates that input; it cannot validate its
semantic completeness or grant execution permission.
"""
    ownership.binding(binding)
    need(type(purpose) is str and purpose in COMMITMENT_PURPOSES)
    bounded_private_intent(value)
    return hmac.new(bytes.fromhex(binding['nonce']),
                    b'dev-history-observation-sync-preview-v1\0' + purpose.encode() + b'\0'
                    + canonical(value), hashlib.sha256).hexdigest()


def validate_proof(value, profile=None, *, now=None):
    value = decode(value, maximum=MAX_PROOF_BYTES) if type(value) in {str, bytes} else value
    profile_module.bounded(value, maximum=MAX_PROOF_BYTES)
    need(type(value) is dict and set(value) == KEYS
         and type(value['schema_version']) is int and value['schema_version'] == 1
         and value['kind'] == TYPE and type(value['repository_id']) is int
         and value['repository_id'] == ownership.REPOSITORY_ID
         and profile_module.hex_value(value['source_sha'], 40)
         and profile_module.positive(value['run_id'])
         and type(value['run_attempt']) is int and value['run_attempt'] == 1)
    need(value['job_role'] == 'plan' and value['environment'] == 'terraform-dev-plan'
         and value['target'] == 'dev' and value['root_contract'] == 'gcp-dev-default'
         and value['drift_alias'] == 'USER_ACTIVITY_HISTORY'
         and value['field_codes'] == ['ETAG', 'LAST_MODIFIED_TIME']
         and value['raw_candidate_state_verified'] is True
         and value['execution_permitted'] is False and value['migration_ready'] is False)
    need(type(value['counts']) is dict and value['counts'] == COUNTS
         and all(type(item) is int for item in value['counts'].values())
         and type(value['checks']) is dict and set(value['checks']) == CHECKS
         and all(item is True for item in value['checks'].values())
         and type(value['commitments']) is dict and set(value['commitments']) == COMMITMENT_PURPOSES
         and all(profile_module.hex_value(item, 64) for item in value['commitments'].values())
         and value['accounting'] == {'provider_requests': 'unknown', 'actual_cost': 'unknown'})
    # Reuse the request's strict UUID, source and immutable <=24h window contract.
    candidate = dict(schema_version=1, kind=profile_module.TYPE, operation='preview',
                     target=value['target'], root_contract=value['root_contract'],
                     git_sha=value['source_sha'], issued_utc=value['issued_utc'],
                     expires_utc=value['expires_utc'], request_id=value['request_id'],
                     max_state_bytes=profile_module.MAX_STATE_BYTES, state_writers_quiescent=True)
    validated = profile_module.validate_profile(candidate, value['source_sha'], now=now)
    observed = timestamp(value['observed_utc'])
    current = now or datetime.now(timezone.utc)
    need(timestamp(value['issued_utc']) <= observed <= current < timestamp(value['expires_utc']))
    if profile is not None:
        expected = profile_module.validate_profile(profile, value['source_sha'], now=now)
        need(profile_module.shared_scope(validated) == profile_module.shared_scope(expected))
    need(len(canonical(value)) + 1 <= MAX_PROOF_BYTES)
    return value


def artifact_name(run_id, source_sha):
    need(profile_module.positive(run_id) and profile_module.hex_value(source_sha, 40))
    return f'history-observation-sync-preview-v1-{run_id}-1-{source_sha}'


def _zip_layout(raw, archive, entry):
    """One non-ZIP64 local entry and exact central directory, including descriptors."""
    need(len(raw) >= 22 and entry.header_offset == 0)
    end = struct.unpack('<4s4H2IH', raw[-22:])
    need(end[0] == b'PK\x05\x06' and end[1:5] == (0, 0, 1, 1) and end[7] == 0
         and end[6] == archive.start_dir and end[5] + end[6] == len(raw) - 22)
    local = struct.unpack('<4s5H3I2H', raw[:30])
    need(local[0] == b'PK\x03\x04' and local[2] == entry.flag_bits
         and entry.flag_bits in {0, 8, 0x800, 0x808} and local[3] == entry.compress_type
         and local[9] == len(PUBLIC_FILE) and local[10] == 0
         and raw[30:30 + local[9]] == PUBLIC_FILE.encode())
    data_end = 30 + local[9] + entry.compress_size
    expected = (entry.CRC, entry.compress_size, entry.file_size)
    if entry.flag_bits & 8:
        need(tuple(local[6:9]) in {(0, 0, 0), expected})
        descriptor = raw[data_end:archive.start_dir]
        if descriptor.startswith(b'PK\x07\x08'):
            descriptor = descriptor[4:]
        need(len(descriptor) == 12 and struct.unpack('<3I', descriptor) == expected)
    else:
        need(tuple(local[6:9]) == expected and data_end == archive.start_dir)


def archive_proof(raw, archive_sha256, proof_sha256, *, profile=None, now=None):
    need(type(raw) is bytes and 0 < len(raw) <= MAX_ARCHIVE_BYTES
         and profile_module.hex_value(archive_sha256, 64)
         and profile_module.hex_value(proof_sha256, 64)
         and hashlib.sha256(raw).hexdigest() == archive_sha256)
    try:
        with zipfile.ZipFile(BytesIO(raw)) as archive:
            entries = archive.infolist()
            need(len(entries) == 1 and archive.comment == b'')
            entry = entries[0]
            need(entry.filename == PUBLIC_FILE and entry.orig_filename == PUBLIC_FILE
                 and not entry.is_dir() and not (entry.flag_bits & 1)
                 and not (entry.external_attr & 0x10) and entry.extra == b'' and entry.comment == b''
                 and stat.S_IFMT(entry.external_attr >> 16) in {0, stat.S_IFREG}
                 and 0 < entry.file_size <= MAX_PROOF_BYTES
                 and 0 < entry.compress_size <= MAX_ARCHIVE_BYTES
                 and entry.compress_type in {zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED})
            _zip_layout(raw, archive, entry)
            # Reading through EOF validates the CRC without extracting a file.
            with archive.open(entry) as stream:
                body = stream.read(MAX_PROOF_BYTES + 1)
            need(len(body) == entry.file_size and len(body) <= MAX_PROOF_BYTES)
        value = validate_proof(decode(body, maximum=MAX_PROOF_BYTES), profile=profile, now=now)
        need(body == canonical(value) + b'\n' and digest(value) == proof_sha256)
        return value
    except (zipfile.BadZipFile, NotImplementedError, RuntimeError, ValueError, EOFError, struct.error, zlib.error):
        raise ValueError('history observation sync provenance STOP') from None


def _json(request, path):
    value = request(path)
    profile_module.bounded(value, maximum=1024 * 1024, depth=24, nodes=50000)
    return value


def _listing(request, path, field):
    items, total = [], None
    for page in range(1, 11):
        response = _json(request, path + f'?per_page=100&page={page}')
        need(type(response) is dict and set(response) == {'total_count', field}
             and type(response['total_count']) is int and 0 < response['total_count'] <= 1000
             and type(response[field]) is list and 0 < len(response[field]) <= 100)
        total = response['total_count'] if total is None else total
        need(response['total_count'] == total)
        items.extend(response[field])
        need(len(items) <= total)
        if len(items) == total:
            break
        need(len(response[field]) == 100)
    need(len(items) == total)
    return items


def _approvals(request, path):
    values, seen = [], set()
    for page in range(1, 11):
        response = _json(request, path + f'/approvals?per_page=100&page={page}')
        need(type(response) is list and len(response) <= 100)
        for item in response:
            key = canonical(item)
            need(key not in seen)
            seen.add(key)
            values.append(item)
        if len(response) < 100:
            break
    else:
        need(False)
    need(values)
    matches = 0
    for item in values:
        need(type(item) is dict and type(item.get('environments')) is list
             and len(item['environments']) == 1)
        env = item['environments'][0]
        need(type(env) is dict and type(env.get('id')) is int and env['id'] == ENVIRONMENT_ID
             and env.get('name') == 'terraform-dev-plan'
             and item.get('state') == 'approved' and type(item.get('user')) is dict
             and type(item['user'].get('id')) is int and item['user']['id'] == ownership.OWNER_ID)
        matches += 1
    need(matches == 1)
    return values


def _run(value, ref):
    ownership.check_run(value, ref, {'workflow_id': WORKFLOW_ID})
    for key in ('actor', 'triggering_actor'):
        need(type(value.get(key)) is dict and type(value[key].get('id')) is int
             and value[key]['id'] == ownership.OWNER_ID)


def _job(jobs, ref):
    need(all(type(job) is dict and profile_module.positive(job.get('id'))
             and type(job.get('run_id')) is int and job['run_id'] == ref['run_id']
             and job.get('head_sha') == ref['source_sha'] for job in jobs)
         and len({job['id'] for job in jobs}) == len(jobs))
    matches = [job for job in jobs if job.get('name') == JOB_NAME]
    need(len(matches) == 1)
    job = matches[0]
    need(job['id'] == ref['job_id'] and job.get('head_branch') == ownership.BRANCH
         and job.get('status') == 'completed' and job.get('conclusion') == 'success'
         and timestamp(job['started_at']) <= timestamp(job['completed_at']))
    steps = job.get('steps')
    need(type(steps) is list and all(type(step) is dict and profile_module.positive(step.get('number')) for step in steps)
         and len({step['number'] for step in steps}) == len(steps))
    positions = []
    for name in REQUIRED_STEPS:
        selected = [step for step in steps if step.get('name') == name]
        need(len(selected) == 1 and selected[0].get('status') == 'completed'
             and selected[0].get('conclusion') == 'success')
        positions.append(selected[0]['number'])
    need(positions == sorted(positions))
    protected = {APPLY_JOB_NAME, 'GCP Terraform Authenticated / Authenticated protected plan',
                 'GCP Terraform Authenticated / Protected validated apply'}
    for other in jobs:
        other_steps = other.get('steps', [])
        need(type(other_steps) is list and all(type(step) is dict for step in other_steps))
        if other['id'] == job['id']:
            continue
        if other.get('name') in protected:
            need(other.get('status') == 'completed' and other.get('conclusion') == 'skipped' and other_steps == [])
        need(not any(step.get('name') in REQUIRED_STEPS
                     or step.get('name') == 'Apply the locally re-created saved plan'
                     for step in other_steps if step.get('conclusion') != 'skipped'))
    return job


def artifact_metadata(value, ref, job, *, now=None):
    current = now or datetime.now(timezone.utc)
    need(type(value) is dict and type(value.get('id')) is int and value['id'] == ref['artifact_id']
         and value.get('name') == artifact_name(ref['run_id'], ref['source_sha'])
         and value.get('expired') is False and profile_module.positive(value.get('size_in_bytes'))
         and value['size_in_bytes'] <= MAX_ARCHIVE_BYTES
         and value.get('digest') == 'sha256:' + ref['artifact_sha256'])
    run = value.get('workflow_run')
    need(type(run) is dict and all(type(run.get(key)) is int for key in ('id', 'repository_id', 'head_repository_id'))
         and run['id'] == ref['run_id'] and run['repository_id'] == ownership.REPOSITORY_ID
         and run['head_repository_id'] == ownership.REPOSITORY_ID
         and run.get('head_sha') == ref['source_sha'] and run.get('head_branch') == ownership.BRANCH)
    created, updated, expires = (timestamp(value.get(key)) for key in ('created_at', 'updated_at', 'expires_at'))
    need(timestamp(job['started_at']) <= created <= updated <= timestamp(job['completed_at'])
         and current < expires <= created + timedelta(days=1))
    return value


def authenticate(ref, profile, *, request, now=None):
    """Complete bounded GET provenance; no logs, missing-page fallback or apply grant."""
    ref = reference(ref)
    wanted = profile_module.validate_profile(profile, ref['source_sha'], now=now)
    need(wanted['operation'] == 'synchronize' and wanted['preview_reference'] == ref)
    base = f'/repos/{ownership.REPOSITORY}/actions/runs/{ref["run_id"]}'
    initial = _json(request, base)
    _run(initial, ref)
    _run(_json(request, base + '/attempts/1'), ref)
    jobs = _listing(request, base + '/attempts/1/jobs', 'jobs')
    job = _job(jobs, ref)
    listing = _listing(request, base + '/artifacts', 'artifacts')
    need(len(listing) == 1)
    path = f'/repos/{ownership.REPOSITORY}/actions/artifacts/{ref["artifact_id"]}'
    artifact = artifact_metadata(_json(request, path), ref, job, now=now)
    need(listing[0] == artifact)
    raw = request(path + '/zip', archive=True)
    need(type(raw) is bytes and len(raw) == artifact['size_in_bytes'])
    value = archive_proof(raw, ref['artifact_sha256'], ref['proof_sha256'], profile=wanted, now=now)
    need(value['run_id'] == ref['run_id'] and value['source_sha'] == ref['source_sha']
         and timestamp(value['issued_utc']) <= timestamp(job['started_at'])
         <= timestamp(value['observed_utc']) <= timestamp(job['completed_at'])
         < timestamp(value['expires_utc']))
    approvals = _approvals(request, base)
    need(_approvals(request, base) == approvals
         and artifact_metadata(_json(request, path), ref, job, now=now) == artifact
         and _listing(request, base + '/artifacts', 'artifacts') == listing
         and _listing(request, base + '/attempts/1/jobs', 'jobs') == jobs)
    final = _json(request, base)
    _run(final, ref)
    need(final == initial)
    # Recheck a real clock at completion: slow pagination cannot extend expiry.
    validate_proof(value, profile=wanted, now=now)
    return value, dict(started_at=job['started_at'], completed_at=job['completed_at'])


class GitHubRead:
    """Fixed GET routes over the existing no-retry, credential-stripping transport."""
    def __init__(self, token):
        # The existing implementation uses NoRedirect for both requests. Only a
        # validated HTTPS artifact-host redirect is followed, using Request(url)
        # without the API Authorization header; another redirect is not followed.
        self._transport = ownership.GitHubRead(token)

    def __call__(self, path, *, archive=False):
        need(type(archive) is bool)
        base = '/repos/' + ownership.REPOSITORY + '/actions/'
        page = r'\?per_page=100&page=(?:[1-9]|10)'
        run = r'runs/[1-9][0-9]*(?:/attempts/1(?:/jobs' + page + r')?|/(?:artifacts|approvals)' + page + r')?'
        artifact = r'artifacts/[1-9][0-9]*' + (r'/zip' if archive else '')
        need(type(path) is str and re.fullmatch(re.escape(base) +
             ('(?:' + artifact + ')' if archive else '(?:' + run + '|' + artifact + ')'), path) is not None)
        result = self._transport(path, archive=archive)
        if archive:
            need(type(result) is bytes and 0 < len(result) <= MAX_ARCHIVE_BYTES)
        return result
