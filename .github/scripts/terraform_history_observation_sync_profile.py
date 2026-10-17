#!/usr/bin/env python3
"""Pure admission shape for a separate, default-off development sync request.

A valid profile is not execution permission or Environment approval. Its public
UUID is an intent identifier; it never replaces the existing private nonce.
"""
from __future__ import annotations

import copy
from datetime import datetime, timedelta, timezone
import json
import re
import uuid

MODE = 'dev-history-observation-sync'
TYPE = MODE + '-request-v1'
TRANSPORT = 'history-observation-sync-preview-artifact-v1'
MAX_STATE_BYTES = 4 * 1024 * 1024
MAX_PROFILE_BYTES = 8192
REPOSITORY = 'kani3camp/youtube-study-space'
REPOSITORY_ID, OWNER_ID = 340900071, 54093651
BRANCH = 'feature/gcp-terraform-iac'
CALLER = '.github/workflows/ci.yml'
PROJECT = 'test-youtube-study-space'
SHARED_KEYS = {'schema_version', 'kind', 'target', 'root_contract', 'git_sha',
               'issued_utc', 'expires_utc', 'max_state_bytes',
               'state_writers_quiescent', 'request_id'}
REFERENCE_KEYS = {'run_id', 'attempt', 'job_id', 'source_sha', 'proof_sha256',
                  'transport', 'artifact_id', 'artifact_sha256'}
MANAGED_FLAGS = (
    'DEV_PRIMARY_EMAIL_IMPORT_ENABLED', 'DEV_QUOTA_MANAGED_ENABLED',
    'DEV_EXPORT_TOPIC_MANAGED_ENABLED', 'DEV_EXPORT_SCHEDULER_MANAGED_ENABLED',
    'DEV_EXPORT_FUNCTION_MANAGED_ENABLED', 'DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED',
)
CLOSED_GATES = (
    'DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED', 'DEV_CURRENT_STATE_BOOTSTRAP_ROOT_READY',
    'DEV_RUNTIME_OWNERSHIP_PLAN_ENABLED', 'DEV_RUNTIME_OWNERSHIP_SPLIT_INPUT_ENABLED',
    'DEV_HISTORY_POST_NOOP12_READY', 'DEV_QUOTA_CREATE_ENABLED',
    'DEV_QUOTA_STATE_REFRESH_ENABLED', 'DEV_TERRAFORM_SECURITY_PROBE_ENABLED',
    'DEV_CURRENT_STATE_READ_ENABLED', 'DEV_RUNTIME_METADATA_READ_ENABLED',
    'DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED',
    'PROD_AUTHENTICATED_TERRAFORM_ENABLED', 'PROD_EXECUTION_CONTRACT_READY',
    'PROD_DEVELOPMENT_EXIT_READY', 'PROD_AUTHENTICATED_TERRAFORM_APPLY_ENABLED',
)
LEGACY_INPUTS = (
    'PLAN_SAFETY_EVIDENCE', 'PLAN_COST_EVIDENCE', 'HISTORY_IMPORT_APPROVAL',
    'RUNTIME_REVIEWED_INPUTS_JSON', 'GCP_RUNTIME_REVIEWED_INPUTS_JSON',
    'RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON', 'GCP_RUNTIME_OWNERSHIP_REVIEWED_INPUTS_JSON',
    'HISTORY_PLAN_SAFETY_EVIDENCE',
    'HISTORY_OBSERVATION_SYNC_PREVIEW_REFERENCE',
)
INPUT_FLAGS = ('primary_email', 'youtube_quota_alerts', 'export_topic',
               'export_scheduler', 'export_function', 'user_activity_history')


def need(value):
    if not value:
        raise ValueError('history observation sync profile STOP')


def positive(value):
    return type(value) is int and 0 < value <= 2 ** 63 - 1


def hex_value(value, length):
    return type(value) is str and re.fullmatch('[0-9a-f]{' + str(length) + '}', value) is not None


def timestamp(value):
    need(type(value) is str and re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ', value) is not None)
    try:
        return datetime.fromisoformat(value.replace('Z', '+00:00'))
    except ValueError:
        raise ValueError('history observation sync profile STOP') from None


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True,
                      allow_nan=False).encode()


def bounded(value, *, maximum=MAX_PROFILE_BYTES, depth=16, nodes=2048):
    """Bound strict JSON primitives before schema-specific bool/integer checks."""
    pending, remaining = [(value, 0)], nodes
    while pending:
        item, level = pending.pop()
        remaining -= 1
        need(remaining >= 0 and level <= depth)
        if type(item) is dict:
            need(all(type(key) is str and len(key) <= maximum for key in item))
            pending.extend((child, level + 1) for child in item.values())
        elif type(item) is list:
            pending.extend((child, level + 1) for child in item)
        elif type(item) is str:
            need(len(item) <= maximum)
        elif type(item) is int:
            need(abs(item) <= 2 ** 63 - 1)
        else:
            need(item is None or type(item) is bool)
    need(len(canonical(value)) <= maximum)
    return value


def decode(raw, *, maximum=MAX_PROFILE_BYTES):
    need(type(raw) in {str, bytes} and 0 < len(raw) <= maximum)
    def unique(pairs):
        result = {}
        for key, value in pairs:
            need(key not in result)
            result[key] = value
        return result
    def integer(value):
        need(len(value.lstrip('-')) <= 19)
        result = int(value)
        need(abs(result) <= 2 ** 63 - 1)
        return result
    def invalid(_):
        raise ValueError('history observation sync profile STOP')
    try:
        value = json.loads(raw, object_pairs_hook=unique, parse_int=integer,
                           parse_float=invalid, parse_constant=invalid)
        return bounded(value, maximum=maximum)
    except (ValueError, TypeError, UnicodeError, RecursionError):
        raise ValueError('history observation sync profile STOP') from None


def reference(value):
    bounded(value)
    need(type(value) is dict and set(value) == REFERENCE_KEYS
         and all(positive(value[key]) for key in ('run_id', 'attempt', 'job_id', 'artifact_id'))
         and value['attempt'] == 1 and value['transport'] == TRANSPORT
         and hex_value(value['source_sha'], 40)
         and all(hex_value(value[key], 64) for key in ('proof_sha256', 'artifact_sha256')))
    return copy.deepcopy(value)


def validate_profile(raw, source_sha, *, now=None):
    value = decode(raw) if type(raw) in {str, bytes} else bounded(raw)
    need(type(value) is dict and type(value.get('operation')) is str
         and value['operation'] in {'preview', 'synchronize'})
    expected = SHARED_KEYS | {'operation'}
    if value['operation'] == 'synchronize':
        expected |= {'preview_reference'}
    need(set(value) == expected and type(value['schema_version']) is int
         and value['schema_version'] == 1 and value['kind'] == TYPE
         and value['target'] == 'dev' and value['root_contract'] == 'gcp-dev-default'
         and hex_value(source_sha, 40) and value['git_sha'] == source_sha
         and type(value['max_state_bytes']) is int and value['max_state_bytes'] == MAX_STATE_BYTES
         and value['state_writers_quiescent'] is True)
    identifier = value['request_id']
    try:
        parsed = uuid.UUID(identifier) if type(identifier) is str else None
    except (ValueError, AttributeError):
        parsed = None
    need(parsed is not None and parsed.version == 4 and str(parsed) == identifier)
    issued, expires = (timestamp(value[key]) for key in ('issued_utc', 'expires_utc'))
    current = now or datetime.now(timezone.utc)
    need(issued <= current < expires <= issued + timedelta(hours=24))
    if value['operation'] == 'synchronize':
        ref = reference(value['preview_reference'])
        need(ref['source_sha'] == source_sha)
    return copy.deepcopy(value)


def shared_scope(value):
    """Immutable cross-run intent; operation and authenticated reference are separate."""
    need(type(value) is dict and SHARED_KEYS <= set(value))
    return copy.deepcopy({key: value[key] for key in sorted(SHARED_KEYS)})


def context(env, *, phase='preview', now=None):
    """Pre-auth profile/source admission only; caller checks actual job/Environment."""
    need(type(env) is dict and all(type(key) is str for key in env)
         and type(phase) is str and phase in {'preview', 'synchronize'})
    expected = {
        'GITHUB_EVENT_NAME': 'workflow_dispatch', 'GITHUB_REPOSITORY': REPOSITORY,
        'GITHUB_REPOSITORY_ID': str(REPOSITORY_ID), 'GITHUB_REPOSITORY_OWNER_ID': str(OWNER_ID),
        'GITHUB_REF': 'refs/heads/' + BRANCH, 'GITHUB_RUN_ATTEMPT': '1',
        'GITHUB_WORKFLOW_REF': f'{REPOSITORY}/{CALLER}@refs/heads/{BRANCH}',
        'MODE': MODE, 'HISTORY_OBSERVATION_SYNC_OPERATION': phase,
        'DEV_AUTHENTICATED_TERRAFORM_ENABLED': 'true',
        'DEV_HISTORY_OBSERVATION_SYNC_ENABLED': 'true',
    }
    need(all(env.get(key) == value for key, value in expected.items())
         and hex_value(env.get('GITHUB_SHA'), 40)
         and type(env.get('GITHUB_RUN_ID')) is str
         and re.fullmatch('[1-9][0-9]{0,18}', env['GITHUB_RUN_ID']) is not None
         and positive(int(env['GITHUB_RUN_ID'])))
    need(env.get('TARGET', env.get('HISTORY_TARGET')) == 'dev'
         and all(env.get(key, 'dev') == 'dev' for key in ('TARGET', 'HISTORY_TARGET'))
         and all(env.get(key) == 'true' for key in MANAGED_FLAGS)
         and all(env.get(key) == 'false' for key in CLOSED_GATES))
    # Both runs use the same immutable source R. Enabling the separate apply
    # gate therefore cannot invalidate its preview. Operation/job/owner approval
    # and the controller's compiled apply block remain independent requirements.
    apply_gate = env.get('DEV_HISTORY_OBSERVATION_SYNC_APPLY_ENABLED')
    need(type(apply_gate) is str and apply_gate in {'true', 'false'}
         and (phase == 'preview' or apply_gate == 'true'))
    need(all(not env.get(key, '') for key in LEGACY_INPUTS)
         and env.get('HISTORY_POST_NOOP', 'false') == 'false'
         and env.get('CURRENT_STATE_ATTESTATION', 'false') == 'false'
         and env.get('OWNERSHIP_WAVE', 'none') == 'none'
         and env.get('OWNERSHIP_ROOT_BASIS', 'original-history-import') == 'original-history-import')
    need(env.get('TF_VAR_project_id', PROJECT) == PROJECT
         and env.get('TF_WORKSPACE', 'default') == 'default')
    allowed_flags = {'TF_VAR_manage_' + key for key in INPUT_FLAGS}
    need(all(key in allowed_flags and value == 'true' for key, value in env.items()
             if key.startswith('TF_VAR_manage_')))
    value = validate_profile(env.get('HISTORY_OBSERVATION_SYNC_PROFILE', ''), env['GITHUB_SHA'], now=now)
    need(value['operation'] == phase)
    issued = timestamp(env.get('DEV_HISTORY_OBSERVATION_SYNC_ISSUED_UTC'))
    expires = timestamp(env.get('DEV_HISTORY_OBSERVATION_SYNC_EXPIRES_UTC'))
    current = now or datetime.now(timezone.utc)
    need(issued <= timestamp(value['issued_utc']) <= current < timestamp(value['expires_utc'])
         <= expires <= issued + timedelta(hours=24))
    if phase == 'synchronize' and 'GITHUB_RUN_ID' in env:
        need(type(env['GITHUB_RUN_ID']) is str and re.fullmatch('[1-9][0-9]*', env['GITHUB_RUN_ID']) is not None
             and int(env['GITHUB_RUN_ID']) != value['preview_reference']['run_id'])
    return value
