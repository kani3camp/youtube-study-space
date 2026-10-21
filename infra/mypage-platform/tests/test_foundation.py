"""Observable admission, native Plan and isolated backend permission boundaries."""
from copy import deepcopy
from contextlib import redirect_stderr
import importlib.util
import io
import json
from pathlib import Path
import re
import sys
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'infra/mypage-platform/scripts'))
import foundation_contract as c
import foundation_ops as ops


def plan_fixture():
    changes = []
    for address, values in c.resource_contract().items():
        changes.append({'address': address, 'mode': 'managed', 'provider_name': 'registry.terraform.io/hashicorp/google',
                        'change': {'actions': ['create'], 'after': deepcopy(values), 'after_unknown': {}}})
    return {'format_version': '1.2', 'complete': True, 'errored': False, 'variables': {'stage': {'value': 'foundation'}},
            'resource_changes': changes}


def refreshed_metadata_fixture():
    plan = plan_fixture()
    plan['resource_drift'] = []
    for item in plan['resource_changes']:
        if item['address'].endswith('.google_service_account.runtime'):
            item['change']['actions'] = ['no-op']
        if '.google_secret_manager_secret.' not in item['address']:
            continue
        item['change']['actions'] = ['no-op']
        item['change']['after'].update(annotations={}, labels={}, version_aliases={})
        drift = deepcopy(item)
        drift['change']['actions'] = ['update']
        drift['change']['before'] = deepcopy(drift['change']['after'])
        drift['change']['before'].update(annotations=None, labels=None, version_aliases=None)
        plan['resource_drift'].append(drift)
    return plan


class FoundationTest(unittest.TestCase):
    def test_every_dispatch_apply_mode_reaches_the_independent_apply_job(self):
        spec = importlib.util.spec_from_file_location('workflow_yaml', ROOT / '.github/codeql/validate-source.py')
        loader = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(loader)
        dispatch = loader.load_yaml(ROOT / '.github/workflows/ci.yml')['on']['workflow_dispatch']['inputs']
        modes = dispatch['mypage_foundation_mode']['options']
        jobs = loader.load_yaml(ROOT / '.github/workflows/mypage-foundation-authenticated.yml')['jobs']
        expression = jobs['foundation_apply']['if'].removeprefix('${{').removesuffix('}}').strip()
        admitted = set()
        for term in expression.split(' || '):
            match = re.fullmatch(r"inputs.mode == '([^']+)'", term)
            self.assertIsNotNone(match)
            admitted.add(match[1])
        self.assertEqual(admitted, {mode for mode in modes if mode.endswith('-apply')})
        for mode in modes:
            c.stage_for_mode(mode)

    def test_registry_stage_adds_only_the_immutable_dev_repository(self):
        plan = plan_fixture()
        plan['variables']['stage']['value'] = 'registry'
        for item in plan['resource_changes']:
            item['change']['actions'] = ['no-op']
        address = 'module.platform[0].google_artifact_registry_repository.mypage[0]'
        plan['resource_changes'].append({
            'address': address, 'mode': 'managed', 'provider_name': 'registry.terraform.io/hashicorp/google',
            'change': {'actions': ['create'], 'after': deepcopy(c.resource_contract('registry')[address]), 'after_unknown': {}},
        })
        self.assertEqual(c.stage_for_mode('registry-plan'), 'registry')
        self.assertEqual(c.stage_for_mode('registry-apply'), 'registry')
        self.assertEqual(c.validate_plan(plan, 'registry'), {'resources': 9, 'creates': 1})
        for key, bad in (
            ('project', 'youtube-study-space'), ('location', 'asia-northeast2'), ('repository_id', 'gcf-artifacts'),
            ('format', 'NPM'), ('mode', 'REMOTE_REPOSITORY'), ('cleanup_policy_dry_run', False),
            ('docker_config', [{'immutable_tags': False}]), ('remote_repository_config', [{'description': 'unreviewed'}]),
            ('virtual_repository_config', [{'upstream_policies': []}]), ('cleanup_policies', [{'id': 'delete'}]),
        ):
            rejected = deepcopy(plan)
            rejected['resource_changes'][-1]['change']['after'][key] = bad
            with self.subTest(field=key), self.assertRaises(ValueError):
                c.validate_plan(rejected, 'registry')
        for actions in (['update'], ['delete'], ['delete', 'create']):
            rejected = deepcopy(plan)
            rejected['resource_changes'][-1]['change']['actions'] = actions
            with self.subTest(actions=actions), self.assertRaises(ValueError):
                c.validate_plan(rejected, 'registry')
        # An opt-in stage cannot leak the repository into the old Foundation mode.
        plan['variables']['stage']['value'] = 'foundation'
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_post_apply_noop_accepts_only_the_same_secret_resource_name(self):
        plan = plan_fixture()
        for item in plan['resource_changes']:
            item['change']['actions'] = ['no-op']
            if '.google_secret_manager_secret_iam_member.' in item['address']:
                short = item['change']['after']['secret_id']
                item['change']['after']['secret_id'] = 'projects/' + c.PROJECT + '/secrets/' + short
        original = deepcopy(plan)
        self.assertEqual(c.validate_plan(plan), {'resources': 8, 'creates': 0})
        self.assertEqual(plan, original)
        iam = next(i for i, item in enumerate(plan['resource_changes'])
                   if '.google_secret_manager_secret_iam_member.' in item['address'])
        for rejected in (
            'projects/youtube-study-space/secrets/mypage-oauth-client-id',
            'projects/' + c.PROJECT + '/secrets/other-secret',
            'projects/' + c.PROJECT + '/secrets/mypage-oauth-client-id/versions/1',
            'untrusted/prefix/mypage-oauth-client-id',
            'projects/123456789/secrets/mypage-oauth-client-id',
        ):
            changed = deepcopy(plan)
            changed['resource_changes'][iam]['change']['after']['secret_id'] = rejected
            with self.subTest(rejected=rejected), self.assertRaises(ValueError):
                c.validate_plan(changed)
        # Do not broaden the Secret metadata resource's short-name contract.
        metadata = next(item for item in plan['resource_changes'] if '.google_secret_manager_secret.' in item['address'])
        metadata['change']['after']['secret_id'] = 'projects/' + c.PROJECT + '/secrets/mypage-oauth-client-id'
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_dependency_source_order_does_not_change_independent_plan_commitment(self):
        plan = plan_fixture() | {'relevant_attributes': [
            {'resource': 'module.platform[0].google_service_account.runtime', 'attribute': ['account_id']},
            {'resource': 'module.platform[0].google_secret_manager_secret.oauth_client_id', 'attribute': ['secret_id']},
        ]}
        original = deepcopy(plan)
        reordered = deepcopy(plan)
        reordered['relevant_attributes'].reverse()
        key = 'private-key-for-synthetic-tests-only'
        self.assertEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                         c.commitment(reordered, 'state-version', 'a' * 40, key))
        self.assertEqual(plan, original)

    def test_dependency_source_contents_and_multiplicity_remain_bound(self):
        plan = plan_fixture() | {'relevant_attributes': [
            {'resource': 'synthetic-resource', 'attribute': ['items', 0, 'name']},
        ]}
        key = 'private-key-for-synthetic-tests-only'
        expected = c.commitment(plan, 'state-version', 'a' * 40, key)
        for change in (
            lambda p: p['relevant_attributes'][0].update(resource='other-resource'),
            lambda p: p['relevant_attributes'].append(deepcopy(p['relevant_attributes'][0])),
            lambda p: p['relevant_attributes'].clear(),
            lambda p: p['relevant_attributes'][0]['attribute'].reverse(),
        ):
            mutated = deepcopy(plan)
            change(mutated)
            self.assertNotEqual(c.commitment(mutated, 'state-version', 'a' * 40, key), expected)
        with self.assertRaises(ValueError):
            c.commitment(plan | {'relevant_attributes': {}}, 'state-version', 'a' * 40, key)

    def test_action_sequence_order_is_not_canonicalized(self):
        plan = plan_fixture()
        plan['resource_changes'][0]['change']['actions'] = ['delete', 'create']
        other = deepcopy(plan)
        other['resource_changes'][0]['change']['actions'].reverse()
        key = 'private-key-for-synthetic-tests-only'
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(other, 'state-version', 'a' * 40, key))

    def test_only_empty_map_refreshes_on_existing_noop_secrets_are_admitted(self):
        self.assertEqual(c.validate_plan(refreshed_metadata_fixture()), {'resources': 8, 'creates': 4})
        metadata = refreshed_metadata_fixture()
        metadata['variables']['stage']['value'] = 'foundation_metadata'
        metadata['resource_changes'] = [x for x in metadata['resource_changes'] if '_iam_member.' not in x['address']]
        self.assertEqual(c.validate_plan(metadata, 'foundation_metadata'), {'resources': 4, 'creates': 0})
        no_op = refreshed_metadata_fixture()
        for item in no_op['resource_changes']:
            item['change']['actions'] = ['no-op']
        self.assertEqual(c.validate_plan(no_op), {'resources': 8, 'creates': 0})

    def test_real_drift_unknowns_foreign_resources_and_planned_writes_still_stop(self):
        mutations = [
            lambda p: p['resource_drift'][0]['change']['before'].update(labels={'retained': 'value'}),
            lambda p: p['resource_drift'][0]['change']['after'].update(labels={'unexpected': 'value'}),
            lambda p: (p['resource_drift'][0]['change']['before'].update(labels={}),
                       p['resource_drift'][0]['change']['after'].update(labels=None)),
            lambda p: p['resource_drift'][0]['change']['after'].update(project='youtube-study-space'),
            lambda p: p['resource_drift'][0]['change']['after'].update(replication=[]),
            lambda p: p['resource_drift'][0]['change']['after'].update(secret_id='other-secret'),
            lambda p: p['resource_drift'][0]['change']['after'].update(new_field={}),
            lambda p: p['resource_drift'][0]['change']['after_unknown'].update(labels=True),
            lambda p: p['resource_drift'][0].update(address='module.platform[0].google_service_account.runtime'),
            lambda p: p['resource_drift'][0].update(address='unreviewed-secret'),
            lambda p: p['resource_drift'][0].update(provider_name='registry.terraform.io/unreviewed/provider'),
            lambda p: p['resource_drift'][0].update(mode='data'),
            lambda p: p['resource_drift'][0]['change'].update(actions=['delete', 'create']),
            lambda p: p['resource_changes'][1]['change'].update(actions=['update']),
            lambda p: p['resource_changes'][1]['change'].update(actions=['create']),
            lambda p: p['resource_drift'].append(deepcopy(p['resource_drift'][0])),
        ]
        for mutate in mutations:
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                plan = refreshed_metadata_fixture()
                mutate(plan)
                c.validate_plan(plan)

    def test_empty_metadata_refresh_remains_bound_to_independent_plan_commitment(self):
        plan = refreshed_metadata_fixture()
        key = 'private-key-for-synthetic-tests-only'
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(plan | {'resource_drift': []}, 'state-version', 'a' * 40, key))

    def test_rejected_plan_values_never_enter_public_diagnostics(self):
        plan = plan_fixture()
        private = 'private-project-and-secret-payload-marker'
        plan['resource_changes'][0]['change']['after']['project'] = private
        output = io.StringIO()
        with patch.object(sys, 'argv', ['foundation_ops.py', 'execute']), \
                patch.object(ops, 'prepare', side_effect=lambda env: c.validate_plan(plan)), \
                redirect_stderr(output):
            self.assertEqual(ops.main(), 1)
        self.assertIn('check=resource-value-project', output.getvalue())
        self.assertNotIn(private, output.getvalue())

    def test_only_eight_expected_private_resources_are_admitted(self):
        self.assertEqual(c.validate_plan(plan_fixture()), {'resources': 8, 'creates': 8})
        changes = plan_fixture()['resource_changes']
        changes[0]['change']['actions'] = ['no-op']
        self.assertEqual(c.validate_plan(plan_fixture() | {'resource_changes': changes})['creates'], 7)

    def test_rejects_deletion_public_access_foreign_principals_secret_versions_and_unknowns(self):
        mutations = [
            lambda p: p['resource_changes'][0]['change'].update(actions=['delete', 'create']),
            lambda p: p['resource_changes'][0]['change'].update(actions=['update']),
            lambda p: p['resource_changes'].append({'address': 'google_cloud_run_v2_service_iam_member.public_invoker'}),
            lambda p: p['resource_changes'].append({'address': 'google_secret_manager_secret_version.raw_value'}),
            lambda p: p['resource_changes'][0]['change']['after'].update(project='youtube-study-space'),
            lambda p: p['resource_changes'][-1]['change']['after'].update(member='allUsers'),
            lambda p: p['resource_changes'][-1]['change']['after'].update(condition=[{'expression': 'true'}]),
            lambda p: p['resource_changes'][0]['change']['after_unknown'].update(account_id=True),
            lambda p: p['resource_changes'].pop(),
            lambda p: p.update(complete=False),
            lambda p: p.update(resource_drift=[{'address': 'unreviewed'}]),
            lambda p: p['variables']['stage'].update(value='runtime_public'),
            lambda p: p['resource_changes'][1]['change']['after']['replication'][0].update(user_managed=[{}]),
        ]
        for mutate in mutations:
            plan = plan_fixture()
            mutate(plan)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                c.validate_plan(plan)

    def test_private_plan_commitment_binds_changes_source_and_state(self):
        plan = plan_fixture()
        key = 'private-key-for-synthetic-tests-only'
        digest = c.commitment(plan, 'absent', 'a' * 40, key)
        self.assertEqual(digest, c.commitment(plan | {'timestamp': 'later'}, 'absent', 'a' * 40, key))
        for changed, version, sha in ((plan | {'applyable': False}, 'absent', 'a' * 40), (plan, 'new-version', 'a' * 40), (plan, 'absent', 'b' * 40)):
            self.assertNotEqual(digest, c.commitment(changed, version, sha, key))

    def test_metadata_first_stage_cannot_grant_iam(self):
        plan = plan_fixture()
        plan['variables']['stage']['value'] = 'foundation_metadata'
        plan['resource_changes'] = [x for x in plan['resource_changes'] if '_iam_member.' not in x['address']]
        self.assertEqual(c.validate_plan(plan, 'foundation_metadata'), {'resources': 4, 'creates': 4})
        plan['resource_changes'].append(plan_fixture()['resource_changes'][-1])
        with self.assertRaises(ValueError):
            c.validate_plan(plan, 'foundation_metadata')

    def test_provider_actions_cannot_bypass_resource_boundary(self):
        plan = plan_fixture() | {'action_invocations': [{'address': 'action.google_arbitrary.call'}]}
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_nested_false_unknown_masks_are_known_but_nested_true_is_rejected(self):
        plan = plan_fixture()
        secret = next(x for x in plan['resource_changes'] if '.google_secret_manager_secret.' in x['address'])
        secret['change']['after_unknown']['replication'] = [{'auto': [{'customer_managed_encryption': []}]}]
        c.validate_plan(plan)
        secret['change']['after_unknown']['replication'][0]['auto'][0]['customer_managed_encryption'] = True
        with self.assertRaises(ValueError):
            c.validate_plan(plan)

    def test_exact_actor_ref_and_sha_before_credentials(self):
        env = {'GITHUB_REPOSITORY': c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
               'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_EVENT_NAME': 'workflow_dispatch',
               'GITHUB_ACTOR': 'kani3camp', 'GITHUB_ACTOR_ID': '54093651', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
               'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_REF': c.REF, 'GITHUB_WORKFLOW_REF': c.CALLER,
               'TF_WORKSPACE': 'default', 'GITHUB_SHA': 'a' * 40, 'MYPAGE_EXPECTED_SHA': 'a' * 40,
               'MYPAGE_MODE': 'foundation-plan'}
        c.context(env, 'plan')
        for key, bad in (('GITHUB_EVENT_NAME', 'pull_request'), ('GITHUB_REF', 'refs/heads/main'),
                         ('GITHUB_ACTOR_ID', '1234'), ('GITHUB_RUN_ATTEMPT', '2'), ('MYPAGE_EXPECTED_SHA', 'b' * 40)):
            with self.subTest(key=key), self.assertRaises(ValueError):
                c.context(env | {key: bad}, 'plan')
        with self.assertRaises(ValueError):
            c.context(env, 'apply')

    def test_backend_actions_cannot_cross_state_or_delete_state(self):
        for kind in ('plan', 'apply'):
            policy = c.backend_policy('synthetic-state-bucket', kind)
            state, lock, listing = policy['Statement']
            self.assertEqual(state['Resource'], 'arn:aws:s3:::synthetic-state-bucket/' + c.STATE_KEY)
            self.assertNotIn('s3:DeleteObject', state['Action'])
            self.assertEqual('s3:PutObject' in state['Action'], kind == 'apply')
            self.assertEqual(lock['Resource'], state['Resource'] + '.tflock')
            self.assertFalse(any('*' in value for value in listing['Condition']['StringEquals']['s3:prefix']))
            conditions = c.trust_policy('123456789012', kind)['Statement'][0]['Condition']['StringEquals']
            self.assertEqual(conditions['token.actions.githubusercontent.com:actor_id'], '54093651')
            self.assertIn(c.REF, conditions['token.actions.githubusercontent.com:sub'])
            self.assertNotIn('*', json.dumps(conditions))

    def test_state_absence_requires_a_successful_exact_prefix_listing(self):
        env = {'MYPAGE_STATE_BUCKET': 'synthetic-state-bucket', 'MYPAGE_STATE_ACCOUNT': '123456789012'}
        listing = {'IsTruncated': False, 'Prefix': c.STATE_KEY, 'Contents': []}
        with patch.object(ops, 'run', return_value=json.dumps(listing)) as command:
            self.assertEqual(ops.state_version(env), 'absent')
            self.assertEqual(command.call_count, 1)
            self.assertIn('list-objects-v2', command.call_args.args[0])
        with patch.object(ops, 'run', side_effect=ValueError('AccessDenied')):
            with self.assertRaises(ValueError):
                ops.state_version(env)

    def test_workflow_apply_requires_independent_plan_agreement_and_cleanup(self):
        # Reuse the repository's YAML-1.2 parser when its CI dependency is installed.
        spec = importlib.util.spec_from_file_location('workflow_source', ROOT / '.github/codeql/validate-source.py')
        module = importlib.util.module_from_spec(spec)
        try:
            spec.loader.exec_module(module)
        except ModuleNotFoundError:
            self.skipTest('PyYAML checked by source-contract CI')
        workflow = module.load_yaml(ROOT / '.github/workflows/mypage-foundation-authenticated.yml')
        self.assertEqual(set(workflow['on']), {'workflow_call'})
        self.assertFalse(workflow['concurrency']['cancel-in-progress'])
        for kind in ('plan', 'apply'):
            job = workflow['jobs']['foundation_' + kind]
            self.assertEqual(job['environment'], 'mypage-dev-' + kind)
            self.assertEqual(job['steps'][-1]['if'], '${{ always() }}')
            self.assertEqual(job['steps'][1]['run'].split()[-1], 'stage')
        apply = workflow['jobs']['foundation_apply']
        step = next(x for x in apply['steps'] if x.get('id') == 'applied')
        self.assertIn("steps.prepared.outputs.commitment == needs.foundation_plan.outputs.commitment", step['if'])
        self.assertIn("steps.prepared.outputs.commitment != ''", step['if'])
        self.assertNotIn('commitment', json.dumps(step.get('env', {})))


if __name__ == '__main__':
    unittest.main()
