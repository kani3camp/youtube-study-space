"""MyPage PR Plan: 所有者の同一repository PRだけを手動PlanのEnvironment/identityで受け付け、何も適用できないことを検証する。"""
from contextlib import redirect_stdout
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / 'infra/mypage-platform/scripts'))
import foundation_contract as c
import foundation_ops as ops


def pr_env():
    ref = 'refs/pull/42/merge'
    return {'GITHUB_REPOSITORY': c.REPOSITORY, 'GITHUB_REPOSITORY_ID': '340900071',
            'GITHUB_REPOSITORY_OWNER_ID': '54093651', 'GITHUB_EVENT_NAME': 'pull_request',
            'GITHUB_ACTOR': 'kani3camp', 'GITHUB_ACTOR_ID': '54093651', 'GITHUB_TRIGGERING_ACTOR': 'kani3camp',
            'GITHUB_RUN_ATTEMPT': '1', 'GITHUB_REF': ref, 'GITHUB_WORKFLOW_REF': c.WORKFLOWS + 'terraform.yml@' + ref,
            'GITHUB_SHA': 'a' * 40, 'MYPAGE_EXPECTED_SHA': 'a' * 40, 'MYPAGE_MODE': 'desired-plan',
            'MYPAGE_ROLE': 'plan', 'MYPAGE_PR_NUMBER': '42', 'MYPAGE_PR_HEAD_REPOSITORY': c.REPOSITORY,
            'TF_WORKSPACE': 'default'}


class PrPlanTest(unittest.TestCase):
    def test_context_admits_only_the_owners_same_repository_first_run(self):
        c.context(pr_env(), 'plan')
        for key, bad in (('GITHUB_ACTOR_ID', '0'), ('GITHUB_TRIGGERING_ACTOR', 'other'), ('GITHUB_RUN_ATTEMPT', '2'),
                         ('MYPAGE_PR_HEAD_REPOSITORY', 'fork/youtube-study-space'), ('MYPAGE_PR_NUMBER', '43'),
                         ('GITHUB_REF', 'refs/pull/42/head'), ('GITHUB_EVENT_NAME', 'pull_request_target'),
                         ('GITHUB_WORKFLOW_REF', c.CALLER), ('MYPAGE_MODE', 'desired-apply'),
                         ('MYPAGE_EXPECTED_SHA', 'b' * 40)):
            with self.subTest(key=key, bad=bad), self.assertRaises(ValueError):
                c.context(pr_env() | {key: bad}, 'plan')
        # A PR uses the manual Plan identity only: never Apply, never an image build.
        for kind in ('apply', 'pr-plan'):
            with self.assertRaises(ValueError):
                c.context(pr_env(), kind)
        with self.assertRaises(ValueError):
            c.context(pr_env(), 'plan', image=True)

    def test_only_the_plan_role_trusts_the_pr_and_every_claim_but_the_pr_number_is_fixed(self):
        statements = c.trust_policy('123456789012', 'plan')['Statement']
        self.assertEqual(len(statements), 2)
        self.assertEqual(statements[0]['Condition']['StringEquals']['token.actions.githubusercontent.com:sub'],
                         c.subject('plan'))
        trust = statements[1]['Condition']
        subject = trust['StringLike']['token.actions.githubusercontent.com:sub']
        self.assertEqual(subject, c.pr_subject('42').replace('refs/pull/42/merge', 'refs/pull/*/merge'))
        self.assertEqual(subject.count('*'), 3)
        self.assertIn('environment:mypage-dev-plan', subject)
        self.assertIn('job_workflow_ref:' + c.WORKFLOWS + 'mypage-foundation-authenticated.yml@refs/pull/*/merge', subject)
        self.assertIn('event_name:pull_request', subject)
        self.assertEqual(trust['StringEquals']['token.actions.githubusercontent.com:actor_id'], '54093651')
        # Apply stays dispatch-only.
        apply = c.trust_policy('123456789012', 'apply')['Statement']
        self.assertEqual(len(apply), 1)
        self.assertNotIn('StringLike', apply[0]['Condition'])
        with self.assertRaises(ValueError):
            c.backend_policy('example-bucket', 'pr-plan')

    def test_provider_condition_binds_the_pr_to_the_plan_environment_and_its_own_ref(self):
        condition = c.provider_condition()
        self.assertLessEqual(len(condition), c.CONDITION_LIMIT)
        dispatch, pull_request = condition.split(') || (')
        self.assertIn('assertion.event_name == "workflow_dispatch"', dispatch)
        self.assertNotIn('pull_request', dispatch)
        for clause in ('assertion.event_name == "pull_request"', 'assertion.actor_id == "54093651"',
                       'assertion.run_attempt == "1"', 'assertion.environment == "mypage-dev-plan"',
                       'assertion.workflow_ref == ' + json.dumps(c.WORKFLOWS + 'terraform.yml@') + ' + assertion.ref',
                       'assertion.job_workflow_ref == ' + json.dumps(c.WORKFLOWS + 'mypage-foundation-authenticated.yml@') + ' + assertion.ref'):
            self.assertIn(clause, pull_request)
        self.assertNotIn('mypage-dev-apply', pull_request)
        self.assertNotIn('workflow_dispatch', pull_request)

    def test_plan_runs_without_the_lock_and_emits_nothing_an_apply_could_use(self):
        commands = []
        def run(args, *, cwd=None, codes=(0,), check='native-command'):
            commands.append(args)
            if args[:3] == ['aws', 'sts', 'get-caller-identity']:
                return json.dumps({'Account': '123456789012', 'UserId': 'AROA' + 'A' * 17 + ':session',
                                   'Arn': 'arn:aws:sts::123456789012:assumed-role/youtube-study-space-mypage-dev-plan/s'})
            if args[:2] == ['terraform', 'version']:
                return json.dumps({'terraform_version': (ROOT / 'infra/mypage-platform/.terraform-version').read_text().strip()})
            if args[:2] == ['terraform', 'show']:
                return json.dumps({})
            return ''
        with tempfile.TemporaryDirectory() as temporary:
            env = pr_env() | {'RUNNER_TEMP': temporary, 'GITHUB_OUTPUT': str(Path(temporary) / 'output'),
                              'MYPAGE_STATE_ACCOUNT': '123456789012', 'MYPAGE_BACKEND_ROLE_ID': 'AROA' + 'A' * 17}
            ops.directory(env).mkdir(mode=0o700)
            with patch.object(ops, 'run', side_effect=run), patch.object(ops, 'state_version', return_value='absent'), \
                 patch.object(c, 'stage_for_mode', return_value='foundation'), \
                 patch.object(c, 'validate_plan', return_value={'resources': 8, 'creates': 0}), \
                 patch.object(ops, 'reject_unowned_resources'), redirect_stdout(io.StringIO()) as output:
                ops.prepare(env)
            self.assertFalse(Path(env['GITHUB_OUTPUT']).exists())
            self.assertFalse((ops.directory(env) / 'prepared.json').exists())
        plans = [args for args in commands if args[:2] == ['terraform', 'plan']]
        self.assertEqual(len(plans), 1)
        self.assertIn('-lock=false', plans[0])
        self.assertFalse(any(arg.startswith('-lock-timeout') for arg in plans[0]))
        self.assertIn('Nothing from this run can be applied', output.getvalue())


if __name__ == '__main__':
    unittest.main()
