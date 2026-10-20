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
from unittest.mock import Mock, patch

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
            'provider_name': 'registry.terraform.io/hashicorp/google', 'type': 'google_pubsub_topic',
            'change': {'actions': list(actions), 'before': None,
                       'after': {'project': ops.PROJECT}, 'after_unknown': {'id': True}}}]}


class Contracts(unittest.TestCase):
    def observation_fixture(self):
        address = 'module.table[0].google_bigquery_table.retained'
        old = {'project': ops.PROJECT, 'id': 'fixture', 'etag': 'old',
               'creation_time': 0, 'last_modified_time': 1, 'num_bytes': 0, 'num_long_term_bytes': 0,
               'num_rows': 0, 'schema': CANARY}
        refreshed = old | {'etag': 'new', 'last_modified_time': 2,
                           'num_bytes': 128, 'num_long_term_bytes': 64, 'num_rows': 1}
        p = plan(('no-op',), address)
        item = p['resource_changes'][0]
        item.update(type='google_bigquery_table', name='retained', module_address='module.table[0]')
        item['change'] = {'actions': ['no-op'], 'before': refreshed, 'after': refreshed,
                          'before_sensitive': {}, 'after_sensitive': {}, 'after_unknown': {}}
        drift = deepcopy(item)
        drift['change'] = drift['change'] | {'actions': ['update'], 'before': old}
        p['resource_drift'] = [drift]
        state = {'resources': [{'module': 'module.table[0]', 'type': 'google_bigquery_table',
            'name': 'retained', 'mode': 'managed', 'instances': [{'attributes': deepcopy(old)}]}]}
        schema = {'format_version': '1.0', 'provider_schemas': {'registry.terraform.io/hashicorp/google': {
            'resource_schemas': {'google_bigquery_table': {'version': 0, 'block': {
                'attributes': {k: {'computed': True, 'type': 'string'}
                               for k in (ops.NORMAL_OBSERVATIONS | {'creation_time'})}}}}}}}
        return p, state, schema

    def test_computed_observations_require_exact_native_state_and_refreshed_plan(self):
        p, state, schema = self.observation_fixture()
        self.assertEqual(ops.validate(p, {}, state=state, provider_schema=schema)['no-op'], 1)
        self.assertEqual(ops.validate(p, {}, state=state, provider_schema=schema, post=True)['no-op'], 1)
        for mutate in (
            lambda p, s, sc: p['resource_drift'][0]['change']['after'].update(schema='changed'),
            lambda p, s, sc: p['resource_drift'][0]['change']['after'].update(creation_time=999),
            lambda p, s, sc: p['resource_drift'][0]['change'].update(actions=['delete']),
            lambda p, s, sc: p['resource_drift'][0]['change'].update(importing={'id': CANARY}),
            lambda p, s, sc: p['resource_drift'][0]['change'].update(after_unknown={'etag': True}),
            lambda p, s, sc: p['resource_drift'][0]['change'].update(after_unknown={'nested': [False, True]}),
            lambda p, s, sc: p['resource_drift'][0]['change'].update(after_sensitive={'schema': True}),
            lambda p, s, sc: p['resource_drift'][0]['change'].update(after_identity={'id': CANARY}),
            lambda p, s, sc: p['resource_drift'][0].update(provider_name='attacker/provider'),
            lambda p, s, sc: p['resource_drift'][0].update(address=CANARY),
            lambda p, s, sc: p['resource_drift'].append(deepcopy(p['resource_drift'][0])),
            lambda p, s, sc: s['resources'][0]['instances'][0]['attributes'].update(etag='unbound'),
            lambda p, s, sc: s.update(resources=[]),
            lambda p, s, sc: s['resources'][0]['instances'][0].update(index_key=None),
            lambda p, s, sc: sc['provider_schemas']['registry.terraform.io/hashicorp/google'][
                'resource_schemas']['google_bigquery_table']['block']['attributes']['etag'].update(optional=True),
        ):
            q, s, sc = deepcopy((p, state, schema))
            mutate(q, s, sc)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                ops.validate(q, {}, state=s, provider_schema=sc)
        for missing in ({'state': state}, {'provider_schema': schema}, {}):
            with self.subTest(missing=missing), self.assertRaises(ValueError):
                ops.validate(p, {}, **missing)
        for key in (0, 'fixture-key'):
            q, s, sc = deepcopy((p, state, schema))
            s['resources'][0]['instances'][0]['index_key'] = key
            for item in (q['resource_changes'][0], q['resource_drift'][0]):
                item['address'] += '[' + ops.common.canonical(key).decode() + ']'
                item['index'] = key
            self.assertEqual(ops.validate(q, {}, state=s, provider_schema=sc)['no-op'], 1)
        q, s, sc = deepcopy((p, state, schema))
        identity = {'project': ops.PROJECT, 'table_id': 'fixture'}
        s['resources'][0]['instances'][0]['identity'] = identity
        q['resource_changes'][0]['change'].update(before_identity=identity, after_identity=identity)
        self.assertEqual(ops.validate(q, {}, state=s, provider_schema=sc)['no-op'], 1)
        for key in ('before_identity', 'after_identity'):
            bad = deepcopy(q)
            bad['resource_drift'][0]['change'][key] = None
            with self.subTest(identity_key=key), self.assertRaises(ValueError):
                ops.validate(bad, {}, state=s, provider_schema=sc)

    def test_observation_drift_remains_in_plan_commitment(self):
        p, state, _ = self.observation_fixture()
        with patch.object(ops, 'review', return_value={}):
            a = ops.commitment(env(), {}, state, p, b'x' * 32)
            p['resource_drift'][0]['change']['before']['etag'] = 'other'
            self.assertNotEqual(a, ops.commitment(env(), {}, state, p, b'x' * 32))

    def test_native_plan_drift_diagnostic_omits_values_and_field_names(self):
        p, state, schema = self.observation_fixture()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'fixture.tfplan').write_text(CANARY)
            replies = [Mock(stdout=b'', returncode=0), Mock(stdout=json.dumps(p).encode()),
                       Mock(stdout=json.dumps(schema).encode())]
            with patch.object(ops.common, 'terraform', side_effect=replies), patch.object(
                    ops, 'verify_storage_ownership'), redirect_stdout(io.StringIO()) as output:
                ops.plan(env(), root, root, 'fixture', {}, state=state)
            diagnostic = next(line for line in output.getvalue().splitlines() if line.startswith('GCP_NORMAL_DRIFT:'))
            self.assertEqual(json.loads(diagnostic.removeprefix('GCP_NORMAL_DRIFT: ')), {'changes': [
                {'resource_type': 'google_bigquery_table', 'actions': ['update'], 'count': 1}]})
            self.assertNotIn(CANARY, output.getvalue())
            self.assertNotIn('etag', output.getvalue())
            self.assertNotIn('num_rows', output.getvalue())

    def test_plan_rejection_diagnostics_are_fixed_codes_only(self):
        with self.assertRaises(ValueError):
            ops.PlanRejected(CANARY)
        with patch.object(sys, 'argv', ['ops', 'execute']), patch.object(
                ops, 'execute', side_effect=ops.PlanRejected('refresh-drift')), redirect_stdout(io.StringIO()) as output:
            self.assertEqual(ops.main(), 1)
        self.assertIn('GCP_NORMAL_VALIDATION: STOP; reason=refresh-drift', output.getvalue())
        self.assertNotIn(CANARY, output.getvalue())

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

    def test_any_non_main_branch_context_is_allowed_only_with_matching_caller(self):
        for branch in ("dev", "integration/gcp-terraform-normal-ops",
                       "feature/arbitrary-normal-change"):
            ref = "refs/heads/" + branch
            for role in ("plan", "apply"):
                v = env(role)
                v["GITHUB_REF"] = ref
                v["GITHUB_WORKFLOW_REF"] = ops.CALLER + ref
                self.assertEqual(ops.context(v, role), "dev")
                with self.assertRaises(ValueError):
                    ops.context(v | {"GITHUB_WORKFLOW_REF":
                                      ops.CALLER + "refs/heads/other"}, role)
        for bad in ("refs/heads/main", "refs/tags/v1", "refs/pull/1/merge",
                    "refs/heads/-bad"):
            v = env("plan") | {"GITHUB_REF": bad,
                               "GITHUB_WORKFLOW_REF": ops.CALLER + bad}
            with self.assertRaises(ValueError):
                ops.context(v, "plan")


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

    def test_replan_projection_normalizes_only_order_insensitive_collections(self):
        with patch.object(ops, 'review', return_value={}):
            key = b'x' * 32
            e = env()
            baseline = plan()
            baseline['resource_changes'].extend([{
                'address': 'google_pubsub_topic.z', 'mode': 'managed',
                'provider_name': 'registry.terraform.io/hashicorp/google',
                'change': {'actions': ['no-op'], 'before': {'enabled': True},
                           'after': {'enabled': True}}}])
            baseline['relevant_attributes'] = [
                {'resource': 'google_pubsub_topic.z', 'attribute': ['labels', 'a']},
                {'resource': 'google_pubsub_topic.example', 'attribute': ['name']}]
            baseline['checks'] = [
                {'status': 'pass', 'address': {'kind': 'var', 'to_display': 'var.z'},
                 'instances': [{'status': 'pass', 'address': {'to_display': 'var.z[1]'}},
                               {'status': 'pass', 'address': {'to_display': 'var.z[0]'}}]},
                {'status': 'pass', 'address': {'kind': 'var', 'to_display': 'var.a'}}]
            baseline['planned_values'] = {'root_module': {
                'resources': [{'address': 'google_pubsub_topic.z', 'values': {'labels': ['a', 'b']}},
                              {'address': 'google_pubsub_topic.example', 'values': {}}],
                'child_modules': [
                    {'address': 'module.z', 'resources': [{'address': 'module.z.google_pubsub_topic.a', 'values': {}}]},
                    {'address': 'module.a', 'resources': []}]}}
            baseline['prior_state'] = {'format_version': '1.0', 'values': deepcopy(baseline['planned_values'])}
            changed_order = deepcopy(baseline)
            for keyname in ('resource_changes', 'relevant_attributes', 'checks'):
                changed_order[keyname].reverse()
            changed_order['checks'][1]['instances'].reverse()
            for phase in ('planned_values',):
                changed_order[phase]['root_module']['resources'].reverse()
                changed_order[phase]['root_module']['child_modules'].reverse()
            changed_order['prior_state']['values']['root_module']['resources'].reverse()
            changed_order['prior_state']['values']['root_module']['child_modules'].reverse()
            changed_order['timestamp'] = 'different-creation-time'
            tfvars = {'project_id': ops.PROJECT}
            state = {'head': {'VersionId': 'fixed-s3-version'}}
            self.assertEqual(ops.commitment(e, tfvars, state, baseline, key),
                             ops.commitment(e, tfvars, state, changed_order, key))
            self.assertEqual(ops.commitment_components(e, tfvars, state, baseline, key),
                             ops.commitment_components(e, tfvars, state, changed_order, key))
            # Values within provider attributes are ordered when expressed as JSON lists.
            changed_values = deepcopy(changed_order)
            for resource in changed_values['planned_values']['root_module']['resources']:
                if resource['address'] == 'google_pubsub_topic.z':
                    resource['values']['labels'] = ['b', 'a']
            self.assertNotEqual(ops.commitment(e, tfvars, state, baseline, key),
                                ops.commitment(e, tfvars, state, changed_values, key))

    def test_replan_diagnostics_print_boolean_section_labels_not_hmacs(self):
        flags = {f'NORMAL_MISMATCH_{name.upper()}': 'false' for name in ops.DIAGNOSTIC_SECTIONS}
        flags['NORMAL_MISMATCH_STATE'] = 'true'
        with redirect_stdout(io.StringIO()) as output:
            ops.report_replan_mismatch(flags)
        self.assertIn('sections=state', output.getvalue())
        self.assertNotIn(CANARY, output.getvalue())
        with self.assertRaises(ValueError):
            ops.report_replan_mismatch(flags | {'NORMAL_MISMATCH_INPUTS': 'secret'})
        with self.assertRaises(ValueError):
            ops.report_replan_mismatch({})


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

    def test_phase_diagnostics_reject_private_or_injected_names_before_output(self):
        for value in (CANARY, 'native-plan\n::warning::' + CANARY, '', None):
            with self.subTest(value=value), redirect_stdout(io.StringIO()) as output:
                with self.assertRaises(ValueError):
                    ops.phase(value)
                self.assertEqual(output.getvalue(), '')

    def test_plan_faults_identify_the_boundary_without_private_output_or_success(self):
        from contextlib import ExitStack
        failures = {
            'identity': 'identity', 'snapshot': 'initial-state',
            'source': 'source-copy', 'init': 'terraform-init',
            'native': 'native-plan', 'show': 'plan-json',
            'validate': 'plan-validation', 'verify_storage_ownership': 'storage-ownership',
            'readback': 'state-readback', 'report_plan': 'plan-summary',
            'output_commitments': 'plan-binding',
        }
        for failure, expected in failures.items():
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                e = env('plan') | {'RUNNER_TEMP': temporary, 'NORMAL_TARGET': 'prod',
                    'NORMAL_MODE': 'normal-plan', 'GITHUB_REF': 'refs/heads/main',
                    'GITHUB_WORKFLOW_REF': ops.CALLER + 'refs/heads/main',
                    'STATE_BUCKET': 'fixture-state', 'STATE_ACCOUNT_ID': '123456789012'}
                root = ops.private(e); root.mkdir(mode=0o700)
                (root / 'normal.tfplan').write_text(CANARY)
                state = {'state': {'resources': []}, 'raw': CANARY}
                p = plan(('no-op',)); p['variables'] = {'private': CANARY}
                output = io.StringIO()
                with ExitStack() as stack:
                    m = {name: stack.enter_context(patch.object(ops, name)) for name in (
                        'inputs', 'identity', 'snapshot', 'source', 'review', 'validate',
                        'verify_storage_ownership', 'report_plan', 'output_commitments', 'apply_saved')}
                    m['inputs'].return_value = ({'project_id': ops.PROJECTS['prod']}, b'b' * 32)
                    m['source'].return_value = root
                    m['snapshot'].return_value = state
                    m['review'].return_value = {}
                    m['validate'].return_value = {'no-op': 1}
                    results = [Mock(stdout=b'', returncode=0), Mock(stdout=b'', returncode=0),
                               Mock(stdout=json.dumps(p).encode(), returncode=0)]
                    if failure in ('init', 'native', 'show'):
                        results[('init', 'native', 'show').index(failure)] = RuntimeError(CANARY)
                    elif failure == 'readback':
                        m['snapshot'].side_effect = [state, {'private': CANARY}]
                    else:
                        m[failure].side_effect = RuntimeError(CANARY)
                    stack.enter_context(patch.object(ops.common, 'terraform', side_effect=results))
                    with patch.dict(os.environ, e, clear=True), patch.object(sys, 'argv', ['ops', 'execute']), redirect_stdout(output):
                        self.assertEqual(ops.main(), 1)
                    text = output.getvalue()
                    phases = [line for line in text.splitlines() if line.startswith('GCP_NORMAL: phase=')]
                    self.assertEqual(phases[-1], 'GCP_NORMAL: phase=' + expected)
                    self.assertIn('GCP_NORMAL: STOP;', text)
                    self.assertNotIn(CANARY, text)
                    self.assertNotIn('PASS;', text)
                    self.assertNotIn('GCP_PROD_INITIAL_NOOP:', text)
                    m['apply_saved'].assert_not_called()

    def test_permission_provider_state_and_mismatch_failures_do_not_leak_or_apply(self):
        for failure in ('identity', 'snapshot', 'setup', 'plan', 'commitment'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temporary:
                e = env('apply') | {'RUNNER_TEMP': temporary}
                root = ops.private(e); root.mkdir(mode=0o700)
                mocks = {k: patch.object(ops, k) for k in ('inputs', 'identity', 'snapshot', 'setup', 'plan',
                         'review', 'report_plan', 'commitment', 'apply_saved')}
                from contextlib import ExitStack
                with ExitStack() as stack:
                    m = {k: stack.enter_context(v) for k, v in mocks.items()}
                    m['inputs'].return_value = ({'project_id': ops.PROJECT}, b'b' * 32)
                    m['snapshot'].return_value = {'state': {'resources': []}}
                    m['plan'].return_value = (root / 'plan', plan(), {})
                    m[failure].side_effect = RuntimeError(CANARY)
                    output = io.StringIO()
                    with patch.dict(os.environ, e, clear=True), patch.object(sys, 'argv', ['ops', 'prepare']), redirect_stdout(output):
                        self.assertEqual(ops.main(), 1)
                    self.assertNotIn(CANARY, output.getvalue())
                    m[failure].assert_called_once()
                    m['apply_saved'].assert_not_called()

    def test_apply_requires_sealed_private_plan_and_fresh_time(self):
        with tempfile.TemporaryDirectory() as directory:
            e = env('apply') | {'RUNNER_TEMP': directory, 'NORMAL_PLAN_TIME': '10'}
            private = ops.private(e)
            private.mkdir(mode=0o700)
            prepared = {'source_sha': e['GITHUB_SHA'], 'run_id': e['GITHUB_RUN_ID'],
                        'mode': e['NORMAL_MODE'], 'digest': 'a' * 64,
                        'binary_sha256': 'b' * 64,
                        'before': {'state': {'resources': []}},
                        'counts': {'create': 0, 'no-op': 1}}
            ops.common.write(private / 'normal-prepared.json', ops.common.canonical(prepared))
            with patch.object(ops, 'identity'), patch.object(
                    ops, 'inputs', return_value=({'project_id': ops.PROJECT}, b'x' * 32)), patch.object(
                    ops, 'apply_saved') as applied:
                with self.assertRaises(ValueError):
                    ops.apply_prepared(e)
                applied.assert_not_called()

    def test_workflow_never_interpolates_hmac_as_step_environment(self):
        text = (ops.REPO / '.github/workflows/gcp-terraform-authenticated.yml').read_text()
        self.assertNotIn('NORMAL_EXPECTED_DIGEST: ${{ needs.normal_plan.outputs.digest }}', text)
        self.assertNotIn('NORMAL_EXPECTED_COMPONENTS: ${{ needs.normal_plan.outputs.components }}', text)
        self.assertIn('steps.prepared.outputs.digest == needs.normal_plan.outputs.digest', text)
        for cmd in ('prepare', 'apply', 'mismatch'):
            self.assertIn('terraform_normal_ops.py ' + cmd, text)
        self.assertIn("steps.normal.outcome != 'success'", text)

    def test_stale_state_stops_saved_apply(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); binary = root / 'plan'; binary.write_bytes(b'fixture')
            with patch.object(ops, 'snapshot', return_value={'changed': True}), patch.object(ops.common, 'terraform') as tf:
                with self.assertRaises(ValueError):
                    ops.apply_saved(env('apply'), root, root, ops.SavedPlan(binary, ops.hashlib.sha256(binary.read_bytes()).digest()), {'before': True}, 'fixture')
                tf.assert_not_called()

    def test_saved_binary_changed_after_show_cannot_be_applied(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary); binary = root / 'plan'; binary.write_bytes(b'original')
            saved = ops.SavedPlan(binary, ops.hashlib.sha256(binary.read_bytes()).digest())
            binary.write_bytes(b'changed')
            with patch.object(ops, 'snapshot') as snapshot, patch.object(ops.common, 'terraform') as tf:
                with self.assertRaises(ValueError):
                    ops.apply_saved(env('apply'), root, root, saved, {}, 'fixture')
                snapshot.assert_not_called()
                tf.assert_not_called()

    def test_normal_modes_reach_preflight_only_for_exact_dev_sha(self):
        sys.path.insert(0, str(ops.REPO / '.github/scripts'))
        from test_terraform_authenticated_workflow import TerraformAuthenticatedWorkflowTest
        runner = TerraformAuthenticatedWorkflowTest()
        runner.text = (ops.REPO / '.github/workflows/gcp-terraform-authenticated.yml').read_text()
        for mode in ('normal-plan', 'normal-apply', 'normal-e2e'):
            good = runner.run_preflight(MODE=mode, NORMAL_EXPECTED_SHA='a' * 40)
            self.assertEqual(good.returncode, 0, good.stdout + good.stderr)
            for branch in ('integration/gcp-terraform-normal-ops', 'feature/arbitrary'):
                ref = 'refs/heads/' + branch
                integrated = runner.run_preflight(MODE=mode, NORMAL_EXPECTED_SHA='a' * 40,
                    GITHUB_REF=ref, GITHUB_WORKFLOW_REF=ops.CALLER + ref)
                self.assertEqual(integrated.returncode, 0, integrated.stdout + integrated.stderr)
            for change in ({'NORMAL_EXPECTED_SHA': 'b' * 40}, {'NORMAL_EXPECTED_SHA': ''},
                {'GITHUB_REF': 'refs/tags/v1',
                 'GITHUB_WORKFLOW_REF': 'kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/tags/v1'}):
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

    def summary_schema(self):
        return {'provider_schemas': {'registry.terraform.io/hashicorp/google': {
            'resource_schemas': {'google_pubsub_topic': {'block': CANARY}, 'google_storage_bucket': {}},
            'data_source_schemas': {'google_pubsub_topic': {}}}}}

    def test_public_summary_groups_types_actions_counts_and_omits_noop(self):
        p = plan(('update',))
        first = p['resource_changes'][0]
        first['address'] = 'module.' + CANARY + '.google_pubsub_topic.name["' + CANARY + '"]'
        first['change']['before'] = {'project': ops.PROJECT, 'secret': CANARY}
        first['change']['after']['secret'] = CANARY
        first['name'], first['index'], first['module_address'] = CANARY, CANARY, CANARY
        p['variables'], p['output_changes'], p['prior_state'] = {'secret': CANARY}, {'secret': CANARY}, {'secret': CANARY}
        second = deepcopy(first); second['address'] += '.second'
        replace = deepcopy(first); replace['type'] = 'google_storage_bucket'
        replace['change']['actions'] = ['create', 'delete']
        noop = deepcopy(first); noop['change']['actions'] = ['no-op']
        p['resource_changes'] += [replace, noop, second]
        expected = [
            {'resource_type': 'google_pubsub_topic', 'actions': ['update'], 'count': 2},
            {'resource_type': 'google_storage_bucket', 'actions': ['create', 'delete'], 'count': 1}]
        self.assertEqual(ops.summarize_changes(p, self.summary_schema()), expected)
        p['resource_changes'].reverse()
        self.assertEqual(ops.summarize_changes(p, self.summary_schema()), expected)
        with tempfile.TemporaryDirectory() as directory:
            summary = Path(directory) / 'summary.md'
            summary.write_text('Existing summary\n')
            output = io.StringIO()
            with patch.object(ops.common, 'terraform', return_value=Mock(
                    stdout=json.dumps(self.summary_schema()).encode())) as tf, redirect_stdout(output):
                ops.report_plan({'GITHUB_STEP_SUMMARY': str(summary)}, Path(directory), Path(directory), p)
            tf.assert_called_once_with({'GITHUB_STEP_SUMMARY': str(summary)}, Path(directory), Path(directory),
                'summary-schema', ['providers', 'schema', '-json'])
            self.assertEqual(json.loads(output.getvalue().removeprefix('GCP_NORMAL_PLAN: ')), {'changes': expected})
            markdown = summary.read_text()
            self.assertTrue(markdown.startswith('Existing summary\n'))
            self.assertIn('| `google_pubsub_topic` | update | 2 |', markdown)
            self.assertIn('| `google_storage_bucket` | create → delete | 1 |', markdown)
            self.assertNotIn(CANARY, output.getvalue() + markdown)
            self.assertNotIn('no-op', output.getvalue() + markdown)

    def test_public_summary_preserves_both_replace_orders_and_data_reads(self):
        for actions in (['delete', 'create'], ['create', 'delete'], ['create'], ['delete'], ['read']):
            p = plan(actions)
            if actions == ['read']:
                p['resource_changes'][0]['mode'] = 'data'
            self.assertEqual(ops.summarize_changes(p, self.summary_schema()), [
                {'resource_type': 'google_pubsub_topic', 'actions': actions, 'count': 1}])

    def test_public_summary_rejects_untrusted_type_or_log_injection_before_publishing(self):
        for provider, kind, mode in [('registry.terraform.io/hashicorp/google', CANARY, 'managed'),
            ('registry.terraform.io/hashicorp/google', 'google_pubsub_topic\n::warning::' + CANARY, 'managed'),
            ('registry.terraform.io/hashicorp/google', 'google_' + 'x' * 201, 'managed'),
            ('attacker/' + CANARY, 'google_pubsub_topic', 'managed'),
            ('registry.terraform.io/hashicorp/google', 'google_pubsub_topic', CANARY)]:
            p = plan(); p['resource_changes'][0].update(provider_name=provider, type=kind, mode=mode)
            output = io.StringIO()
            with tempfile.TemporaryDirectory() as directory:
                summary = Path(directory) / 'summary.md'
                with patch.object(ops.common, 'terraform', return_value=Mock(
                        stdout=json.dumps(self.summary_schema()).encode())), redirect_stdout(output):
                    with self.assertRaises(ValueError):
                        ops.report_plan({'GITHUB_STEP_SUMMARY': str(summary)}, Path(directory), Path(directory), p)
                self.assertEqual(output.getvalue(), '')
                self.assertFalse(summary.exists())

    def test_public_summary_noop_has_no_changed_resource_rows(self):
        self.assertEqual(ops.summarize_changes(plan(('no-op',)), self.summary_schema()), [])
        with tempfile.TemporaryDirectory() as directory:
            summary = Path(directory) / 'summary.md'
            output = io.StringIO()
            with redirect_stdout(output):
                ops.publish_plan_summary({'GITHUB_STEP_SUMMARY': str(summary)}, [])
            self.assertEqual(json.loads(output.getvalue().removeprefix('GCP_NORMAL_PLAN: ')), {'changes': []})
            self.assertEqual(summary.read_text(), '### Terraform resource changes\n\nNo resource changes.\n')

    def test_public_summary_without_job_summary_still_writes_log(self):
        output = io.StringIO()
        with redirect_stdout(output):
            ops.publish_plan_summary({}, [])
        self.assertEqual(json.loads(output.getvalue().removeprefix('GCP_NORMAL_PLAN: ')), {'changes': []})

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
