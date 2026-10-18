#!/usr/bin/env python3
"""Native-operation contracts, trust negatives and fail-closed controller faults."""
from contextlib import redirect_stdout
from copy import deepcopy
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
import terraform_normal_ops as ops
import bootstrap_normal_ops as bootstrap

CANARY = 'PRIVATE_SECRET_CANARY_MUST_NOT_APPEAR'


def env(role='plan'):
    return {'GITHUB_EVENT_NAME': 'workflow_dispatch',
        'GITHUB_REPOSITORY': 'kani3camp/youtube-study-space', 'GITHUB_REPOSITORY_ID': '340900071',
        'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_ACTOR': 'kani3camp',
        'GITHUB_ACTOR_ID': '54093651', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
        'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_REF': 'refs/heads/dev',
        'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/dev',
        'GITHUB_SHA': 'a' * 40, 'NORMAL_EXPECTED_SHA': 'a' * 40,
        'NORMAL_TARGET': 'dev', 'NORMAL_MODE': 'normal-apply', 'NORMAL_ROLE': role,
        'GITHUB_JOB': 'normal_' + role, 'GITHUB_RUN_ID': '1234', 'TF_WORKSPACE': 'default',
        'STATE_AWS_REGION': 'ap-northeast-1'}


def plan(actions=('create',), resource='google_pubsub_topic.example'):
    return {'format_version': '1.2', 'terraform_version': ops.common.VERSION,
        'errored': False, 'complete': True, 'timestamp': '2026-10-19T00:00:00Z',
        'resource_changes': [{'address': resource, 'mode': 'managed',
            'provider_name': 'registry.terraform.io/hashicorp/google',
            'change': {'actions': list(actions), 'before': None,
                       'after': {'project': ops.PROJECT}, 'after_unknown': {'id': True}}}]}


class Contracts(unittest.TestCase):
    def test_generic_create_and_update_accept_multiple_resource_types(self):
        for resource in ('google_pubsub_topic.example', 'google_storage_bucket.example',
                         'google_cloud_run_v2_service.example', 'module.future.google_future_type.example["a"]'):
            for action in ('create', 'update'):
                self.assertEqual(ops.validate(plan((action,), resource), {})[action], 1)

    def test_delete_and_both_replace_orders_require_exact_committed_review(self):
        for action in (['delete'], ['delete', 'create'], ['create', 'delete']):
            p = plan(action)
            with self.assertRaises(ValueError):
                ops.validate(p, {})
            count = ops.validate(p, {'google_pubsub_topic.example': action})
            self.assertEqual(count['replace' if len(action) == 2 else 'delete'], 1)
            with self.assertRaises(ValueError):
                ops.validate(p, {'google_pubsub_topic.other': action})

    def test_unused_destructive_approval_is_rejected(self):
        with self.assertRaises(ValueError):
            ops.validate(plan(), {'google_pubsub_topic.example': ['delete']})

    def test_partial_errored_deferred_drift_and_failed_checks_rejected(self):
        for mutation in ({'complete': False}, {'errored': True}, {'resource_drift': [{}]},
                         {'deferred_changes': [{}]}, {'checks': [{'status': 'unknown'}]},
                         {'checks': [{'status': 'fail'}]}):
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                ops.validate(plan() | mutation, {})

    def test_duplicate_unknown_action_import_wrong_provider_and_production_rejected(self):
        mutations = []
        p = plan(); p['resource_changes'] *= 2; mutations.append(p)
        p = plan(('forget',)); mutations.append(p)
        p = plan(); p['resource_changes'][0]['change']['importing'] = {'id': CANARY}; mutations.append(p)
        p = plan(); p['resource_changes'][0]['provider_name'] = 'attacker/provider'; mutations.append(p)
        for side in ('before', 'after'):
            p = plan(); p['resource_changes'][0]['change'][side] = {'project': 'youtube-study-space'}; mutations.append(p)
        for p in mutations:
            with self.assertRaises(ValueError):
                ops.validate(p, {})

    def test_post_plan_requires_resource_and_output_noop(self):
        ops.validate(plan(('no-op',)), {}, post=True)
        for action in ('create', 'update', 'delete', 'read'):
            with self.assertRaises(ValueError):
                ops.validate(plan((action,)), {}, post=True)
        with self.assertRaises(ValueError):
            ops.validate(plan(('no-op',)) | {'output_changes': {'x': {'actions': ['update']}}}, {}, post=True)

    def test_trust_context_rejects_forks_prs_actor_attempt_ref_sha_workspace_and_prod(self):
        ops.context(env(), 'plan')
        changes = {'GITHUB_EVENT_NAME': 'pull_request', 'GITHUB_REPOSITORY': 'fork/repo',
            'GITHUB_REPOSITORY_ID': '0', 'GITHUB_REPOSITORY_OWNER_ID': '0',
            'GITHUB_ACTOR': 'other', 'GITHUB_ACTOR_ID': '0', 'GITHUB_TRIGGERING_ACTOR': 'other',
            'GITHUB_RUN_ATTEMPT': '2', 'GITHUB_REF': 'refs/heads/integration/mypage-canon-20261006',
            'GITHUB_WORKFLOW_REF': 'other', 'GITHUB_SHA': 'b' * 40, 'NORMAL_EXPECTED_SHA': 'b' * 40,
            'NORMAL_TARGET': 'prod', 'NORMAL_MODE': 'migration-apply', 'GITHUB_JOB': 'migration_plan',
            'TF_WORKSPACE': 'prod', 'STATE_AWS_REGION': 'us-east-1'}
        for k, v in changes.items():
            with self.subTest(key=k), self.assertRaises(ValueError):
                ops.context(env() | {k: v}, 'plan')

    def test_apply_is_not_authorized_by_plan_mode(self):
        with self.assertRaises(ValueError):
            ops.context(env('apply') | {'NORMAL_MODE': 'normal-plan'}, 'apply')

    def test_plan_binding_covers_state_inputs_source_run_mode_review_and_full_plan(self):
        key = b'b' * 32
        e, tfvars, state, p = env(), {'project_id': ops.PROJECT}, {'head': {'VersionId': 'one'}}, plan()
        with patch.object(ops, 'review', return_value={}):
            original = ops.commitment(e, tfvars, state, p, key)
            for e2, v2, s2, p2 in [(e | {'GITHUB_SHA': 'b' * 40}, tfvars, state, p),
                (e | {'GITHUB_RUN_ID': '999'}, tfvars, state, p),
                (e | {'NORMAL_MODE': 'normal-plan'}, tfvars, state, p),
                (e, tfvars | {'input': CANARY}, state, p),
                (e, tfvars, {'head': {'VersionId': 'two'}}, p),
                (e, tfvars, state, p | {'configuration': {'changed': True}})]:
                self.assertNotEqual(original, ops.commitment(e2, v2, s2, p2, key))
            self.assertEqual(original, ops.commitment(e, tfvars, state, p | {'timestamp': 'new'}, key))
        with patch.object(ops, 'review', return_value={'changed': ['delete']}):
            self.assertNotEqual(original, ops.commitment(e, tfvars, state, p, key))

    def test_stale_future_missing_and_different_plan_rejected(self):
        e = env() | {'NORMAL_EXPECTED_DIGEST': 'a' * 64, 'NORMAL_PLAN_TIME': '100'}
        ops.match(e, 'a' * 64, 100 + ops.MAX_AGE)
        for changes, digest, now in [({}, 'b' * 64, 100), ({}, 'a' * 64, 99),
            ({}, 'a' * 64, 100 + ops.MAX_AGE + 1), ({'NORMAL_EXPECTED_DIGEST': ''}, 'a' * 64, 100)]:
            with self.assertRaises(ValueError):
                ops.match(e | changes, digest, now)

    def test_cleanup_removes_private_files_even_after_execution_failure(self):
        with tempfile.TemporaryDirectory() as temporary:
            e = env() | {'RUNNER_TEMP': temporary}
            root = ops.private(e); root.mkdir(mode=0o700)
            (root / 'secret.tfstate').write_text(CANARY)
            with patch.dict(os.environ, e, clear=True), patch.object(sys, 'argv', ['ops', 'cleanup']):
                self.assertEqual(ops.main(), 0)
            self.assertFalse(root.exists())

    def test_permission_provider_state_and_mismatch_failures_do_not_leak_or_apply(self):
        for failure in ('identity', 'snapshot', 'setup', 'plan', 'match'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                e = env('apply') | {'RUNNER_TEMP': temporary}
                root = ops.private(e); root.mkdir(mode=0o700)
                mocks = {k: patch.object(ops, k) for k in ('inputs', 'identity', 'snapshot', 'setup', 'plan',
                         'review', 'commitment', 'match', 'apply_saved')}
                from contextlib import ExitStack
                with ExitStack() as stack:
                    m = {k: stack.enter_context(v) for k, v in mocks.items()}
                    m['inputs'].return_value = ({'project_id': ops.PROJECT}, b'b' * 32)
                    m['snapshot'].return_value = {'state': {'resources': []}}
                    m['plan'].return_value = (root / 'plan', plan(), {})
                    m[failure].side_effect = RuntimeError(CANARY)
                    output = io.StringIO()
                    with patch.dict(os.environ, e, clear=True), patch.object(sys, 'argv', ['ops', 'execute']), redirect_stdout(output):
                        self.assertEqual(ops.main(), 1)
                    self.assertNotIn(CANARY, output.getvalue())
                    m['apply_saved'].assert_not_called()

    def test_stale_state_stops_saved_apply(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); binary = root / 'plan'; binary.write_bytes(b'fixture')
            with patch.object(ops, 'snapshot', return_value={'changed': True}), patch.object(ops.common, 'terraform') as tf:
                with self.assertRaises(ValueError):
                    ops.apply_saved(env('apply'), root, root, binary, {'before': True}, 'fixture')
                tf.assert_not_called()

    def test_normal_modes_reach_preflight_only_for_exact_dev_sha(self):
        sys.path.insert(0, str(ops.REPO / '.github/scripts'))
        from test_terraform_authenticated_workflow import TerraformAuthenticatedWorkflowTest
        runner = TerraformAuthenticatedWorkflowTest()
        runner.text = (ops.REPO / '.github/workflows/gcp-terraform-authenticated.yml').read_text()
        for mode in ('normal-plan', 'normal-apply', 'normal-e2e'):
            good = runner.run_preflight(MODE=mode, NORMAL_EXPECTED_SHA='a' * 40)
            self.assertEqual(good.returncode, 0, good.stdout + good.stderr)
            for change in ({'NORMAL_EXPECTED_SHA': 'b' * 40}, {'NORMAL_EXPECTED_SHA': ''},
                {'GITHUB_REF': 'refs/heads/integration/mypage-canon-20261006',
                 'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/integration/mypage-canon-20261006'},
                {'TARGET': 'prod', 'GITHUB_REF': 'refs/heads/main',
                 'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/main'}):
                bad = runner.run_preflight(MODE=mode, **({'NORMAL_EXPECTED_SHA': 'a' * 40} | change))
                self.assertNotEqual(bad.returncode, 0)

    def test_bootstrap_grants_metadata_control_without_data_or_auth_admin(self):
        catalog = {'pubsub.topics.create', 'pubsub.topics.get', 'storage.buckets.delete',
            'storage.objects.get', 'storage.objects.delete', 'bigquery.tables.getData',
            'bigquery.tables.get', 'bigquery.jobs.create', 'secretmanager.versions.access',
            'secretmanager.versions.add', 'secretmanager.versions.destroy', 'secretmanager.secrets.create',
            'iam.serviceAccounts.create', 'iam.serviceAccounts.setIamPolicy',
            'iam.serviceAccounts.getAccessToken', 'iam.serviceAccountKeys.create',
            'resourcemanager.projects.setIamPolicy'}
        roles = bootstrap.selected_permissions(catalog, ['resourcemanager.projects.get'])
        self.assertEqual(set(roles['plan']), {'resourcemanager.projects.get', 'pubsub.topics.get', 'bigquery.tables.get'})
        self.assertEqual(set(roles['apply']) - set(roles['plan']),
            {'pubsub.topics.create', 'storage.buckets.delete', 'secretmanager.secrets.create', 'iam.serviceAccounts.create'})

    def test_workflow_uses_protected_independent_jobs_and_no_artifacts(self):
        text = (ops.REPO / '.github/workflows/gcp-terraform-authenticated.yml').read_text()
        for role in ('plan', 'apply'):
            block = text.split('\n  normal_' + role + ':', 1)[1].split('\n  normal_', 1)[0]
            self.assertIn('needs: ' + ('[preflight, normal_plan]' if role == 'apply' else 'preflight'), block)
            self.assertIn('environment: terraform-${{ inputs.target }}-' + role, block)
            self.assertIn('GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT', block)
            self.assertIn('GCP_TERRAFORM_NORMAL_WIF_PROVIDER', block)
            self.assertIn('if: ${{ always() }}', block)
            self.assertNotIn('continue-on-error', block)
        for value in ('actions/upload-artifact', 'actions/download-artifact', '-lock=false', 'force-unlock'):
            self.assertNotIn(value, text)


if __name__ == '__main__':
    unittest.main()
