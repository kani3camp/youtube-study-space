#!/usr/bin/env python3
"""Dummy pure request admission: no credentials, network or State access."""
from __future__ import annotations

from datetime import datetime, timedelta, timezone
import json
import unittest

import terraform_history_observation_sync_profile as gate

SHA = 'a' * 40
NOW = datetime(2026, 10, 16, 1, tzinfo=timezone.utc)


def dummy_reference():
    return dict(run_id=101, attempt=1, job_id=201, source_sha=SHA,
                proof_sha256='b' * 64, transport=gate.TRANSPORT,
                artifact_id=301, artifact_sha256='c' * 64)


def dummy_profile(operation='preview'):
    value = dict(schema_version=1, kind=gate.TYPE, operation=operation, target='dev',
                 root_contract='gcp-dev-default', git_sha=SHA,
                 issued_utc='2026-10-16T00:00:00Z', expires_utc='2026-10-17T00:00:00Z',
                 max_state_bytes=4194304, state_writers_quiescent=True,
                 request_id='12345678-1234-4234-8234-123456789abc')
    if operation == 'synchronize':
        value['preview_reference'] = dummy_reference()
    return value


def dummy_env(operation='preview'):
    value = dict(GITHUB_EVENT_NAME='workflow_dispatch', GITHUB_REPOSITORY=gate.REPOSITORY,
        GITHUB_REPOSITORY_ID=str(gate.REPOSITORY_ID), GITHUB_REPOSITORY_OWNER_ID=str(gate.OWNER_ID),
        GITHUB_REF='refs/heads/' + gate.BRANCH, GITHUB_SHA=SHA, GITHUB_RUN_ATTEMPT='1',
        GITHUB_RUN_ID='102', GITHUB_WORKFLOW_REF=f'{gate.REPOSITORY}/{gate.CALLER}@refs/heads/{gate.BRANCH}',
        MODE=gate.MODE, TARGET='dev', HISTORY_OBSERVATION_SYNC_OPERATION=operation,
        HISTORY_OBSERVATION_SYNC_PROFILE=json.dumps(dummy_profile(operation)),
        DEV_AUTHENTICATED_TERRAFORM_ENABLED='true', DEV_HISTORY_OBSERVATION_SYNC_ENABLED='true',
        DEV_HISTORY_OBSERVATION_SYNC_APPLY_ENABLED='true' if operation == 'synchronize' else 'false',
        DEV_HISTORY_OBSERVATION_SYNC_ISSUED_UTC='2026-10-16T00:00:00Z',
        DEV_HISTORY_OBSERVATION_SYNC_EXPIRES_UTC='2026-10-17T00:00:00Z')
    value.update({key: 'true' for key in gate.MANAGED_FLAGS})
    value.update({key: 'false' for key in gate.CLOSED_GATES})
    return value


class ProfileTest(unittest.TestCase):
    def test_preview_and_separate_apply_reference_share_immutable_intent(self):
        preview, apply = dummy_profile(), dummy_profile('synchronize')
        for raw in (preview, json.dumps(preview), gate.canonical(preview)):
            self.assertEqual(gate.validate_profile(raw, SHA, now=NOW), preview)
        self.assertEqual(gate.shared_scope(preview), gate.shared_scope(apply))
        self.assertEqual(gate.context(dummy_env(), now=NOW), preview)
        self.assertEqual(gate.context(dummy_env('synchronize'), phase='synchronize', now=NOW), apply)
        copied = gate.validate_profile(apply, SHA, now=NOW)
        copied['preview_reference']['job_id'] += 1
        self.assertNotEqual(copied, apply)

    def test_same_source_preview_accepts_both_apply_gate_values_without_authorizing_apply(self):
        for apply_gate in ('true', 'false'):
            env = dummy_env(); env['DEV_HISTORY_OBSERVATION_SYNC_APPLY_ENABLED'] = apply_gate
            self.assertEqual(gate.context(env, now=NOW)['operation'], 'preview')
        env = dummy_env('synchronize'); env['DEV_HISTORY_OBSERVATION_SYNC_APPLY_ENABLED'] = 'false'
        with self.assertRaises(ValueError):gate.context(env, phase='synchronize', now=NOW)
        for enabled in ('false', '', True):
            env = dummy_env(); env['DEV_HISTORY_OBSERVATION_SYNC_ENABLED'] = enabled
            env['DEV_HISTORY_OBSERVATION_SYNC_APPLY_ENABLED'] = 'true'
            with self.assertRaises(ValueError):gate.context(env, now=NOW)

    def test_wrong_type_unknown_key_scope_and_reference_rejected(self):
        mutations = [
            ('schema_version', True), ('kind', 'dev-history12-current-state-v1'),
            ('operation', 'apply'), ('target', 'prod'), ('root_contract', 'other'),
            ('git_sha', 'd' * 40), ('max_state_bytes', True), ('max_state_bytes', 4194303),
            ('state_writers_quiescent', 1), ('request_id', 'd' * 64),
            ('request_id', '12345678-1234-1234-8234-123456789abc'),
            ('request_id', '12345678-1234-4234-8234-123456789ABC'),
            ('preview_reference', dummy_reference()), ('nonce', 'd' * 64),
        ]
        for key, value in mutations:
            with self.subTest(key=key, value=value), self.assertRaises(ValueError):
                changed = dummy_profile(); changed[key] = value
                gate.validate_profile(changed, SHA, now=NOW)
        for key, value in [('attempt', True), ('attempt', 2), ('transport', 'artifact-v1'),
                           ('source_sha', 'd' * 40), ('job_id', False), ('run_id', 0)]:
            with self.subTest(reference=key), self.assertRaises(ValueError):
                changed = dummy_profile('synchronize'); changed['preview_reference'][key] = value
                gate.validate_profile(changed, SHA, now=NOW)
        apply = dummy_profile('synchronize'); apply.pop('preview_reference')
        with self.assertRaises(ValueError):gate.validate_profile(apply, SHA, now=NOW)

    def test_strict_json_and_resource_budgets(self):
        samples = [b'{"operation":"preview","operation":"synchronize"}',
                   b'{"x":NaN}', b'{"x":Infinity}', b'{"x":1.0}',
                   b'{"x":9223372036854775808}', b'[' * 40 + b']' * 40,
                   b' ' * (gate.MAX_PROFILE_BYTES + 1), b'\xff']
        for raw in samples:
            with self.subTest(raw=raw[:50]), self.assertRaises(ValueError):
                gate.validate_profile(raw, SHA, now=NOW)
        for obj in ({'x': float('nan')}, {'x': (1,)}, {1: 'x'}, {'x': [0] * 3000}):
            with self.subTest(kind=type(obj)), self.assertRaises(ValueError):gate.bounded(obj)
        cycle = []; cycle.append(cycle)
        with self.assertRaises(ValueError):gate.bounded(cycle)

    def test_expired_future_oversized_or_retimed_windows_fail(self):
        for now in (NOW - timedelta(hours=2), NOW + timedelta(days=1)):
            with self.assertRaises(ValueError):gate.validate_profile(dummy_profile(), SHA, now=now)
        for key, value in [('expires_utc', '2026-10-17T00:00:01Z'),
                           ('issued_utc', '2026-10-16T00:00:00+00:00'),
                           ('expires_utc', '2026-13-16T00:00:00Z')]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                changed = dummy_profile(); changed[key] = value
                gate.validate_profile(changed, SHA, now=NOW)
        env = dummy_env(); env['DEV_HISTORY_OBSERVATION_SYNC_EXPIRES_UTC'] = '2026-10-16T23:59:59Z'
        with self.assertRaises(ValueError):gate.context(env, now=NOW)

    def test_default_off_wrong_source_flags_legacy_or_prod_fail_closed(self):
        changed_values = dict(GITHUB_EVENT_NAME='push', GITHUB_REPOSITORY_ID='1',
            GITHUB_REPOSITORY_OWNER_ID='1', GITHUB_REF='refs/heads/dev', GITHUB_RUN_ATTEMPT='2',
            GITHUB_WORKFLOW_REF='wrong', GITHUB_SHA='d' * 40, TARGET='prod', HISTORY_TARGET='prod',
            MODE='plan', HISTORY_OBSERVATION_SYNC_OPERATION='synchronize',
            DEV_HISTORY_OBSERVATION_SYNC_ENABLED='false',
            DEV_HISTORY_OBSERVATION_SYNC_APPLY_ENABLED=True, HISTORY_POST_NOOP='true',
            CURRENT_STATE_ATTESTATION='true', OWNERSHIP_WAVE='noop',
            OWNERSHIP_ROOT_BASIS='current-state-bootstrap', TF_WORKSPACE='other',
            TF_VAR_project_id='other', TF_VAR_manage_user_activity_history='false',
            TF_VAR_manage_unknown='false')
        changed_values.update({key: 'false' for key in gate.MANAGED_FLAGS})
        changed_values.update({key: 'true' for key in gate.CLOSED_GATES})
        changed_values.update({key: 'MIXED_OLD_INPUT' for key in gate.LEGACY_INPUTS})
        for key, value in changed_values.items():
            with self.subTest(key=key), self.assertRaises(ValueError):
                env = dummy_env(); env[key] = value; gate.context(env, now=NOW)
        env = dummy_env(); env.pop('DEV_HISTORY_OBSERVATION_SYNC_ENABLED')
        with self.assertRaises(ValueError):gate.context(env, now=NOW)
        env = dummy_env('synchronize'); env['GITHUB_RUN_ID'] = '101'
        with self.assertRaises(ValueError):gate.context(env, phase='synchronize', now=NOW)
        # No approval or private nonce input is needed for pure shape admission.
        self.assertNotIn('RUNTIME_OWNERSHIP_PACKET_JSON', dummy_env())
        self.assertNotIn('GITHUB_ENVIRONMENT', dummy_env())


if __name__ == '__main__':
    unittest.main()
