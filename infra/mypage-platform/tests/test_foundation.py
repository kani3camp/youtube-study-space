"""Observable admission, native Plan and isolated backend permission boundaries."""
from copy import deepcopy
import importlib.util
import json
from pathlib import Path
import sys
import unittest

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


class FoundationTest(unittest.TestCase):
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
            self.assertIn(c.REF, conditions['token.actions.githubusercontent.com:sub'])
            self.assertNotIn('*', json.dumps(conditions))

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
