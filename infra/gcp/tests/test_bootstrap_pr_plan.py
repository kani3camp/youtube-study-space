#!/usr/bin/env python3
"""Core PR Plan identity: owner PR only, read-only GCP role, State-read-only AWS."""
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
import bootstrap_pr_plan as pr
import terraform_normal_ops as ops


class PrPlanBootstrapTest(unittest.TestCase):
    def test_condition_admits_only_the_owners_pr_through_the_reviewed_workflows(self):
        for target in ('dev', 'prod'):
            condition = pr.condition(target)
            with self.subTest(target=target):
                self.assertLessEqual(len(condition), pr.CONDITION_LIMIT)
                for clause in ("assertion.event_name == 'pull_request'", "assertion.actor_id == '54093651'",
                               "assertion.run_attempt == '1'",
                               "assertion.environment == 'terraform-" + target + "-pr-plan'",
                               "assertion.workflow_ref == '" + ops.CALLER + "' + assertion.ref",
                               "assertion.job_workflow_ref == '" + ops.PR_PLAN_REUSABLE + "' + assertion.ref"):
                    self.assertIn(clause, condition)
                for absent in ('workflow_dispatch', 'gcp-terraform-authenticated.yml', '||'):
                    self.assertNotIn(absent, condition)
        self.assertNotIn('terraform-prod-pr-plan', pr.condition('dev'))

    def test_identity_names_match_the_controller_checks(self):
        for target in ('dev', 'prod'):
            self.assertEqual(pr.account(target).split('@')[0], 'terraform-' + target + '-normal-pr-plan')
            self.assertLessEqual(len(pr.account(target).split('@')[0]), 30)

    def test_aws_role_reads_only_the_state_and_fixes_every_claim_but_the_pr_number(self):
        policy = pr.aws_state_read_policy('dev', 'example-bucket')
        actions = set()
        for statement in policy['Statement']:
            value = statement['Action']
            actions.update([value] if isinstance(value, str) else value)
        self.assertEqual(actions, {'s3:GetObject', 's3:ListBucket'})
        self.assertEqual(policy['Statement'][0]['Resource'],
                         'arn:aws:s3:::example-bucket/youtube-study-space/dev/terraform.tfstate')
        trust = pr.aws_trust_policy('prod', '123456789012')['Statement'][0]['Condition']
        subject = trust['StringLike']['token.actions.githubusercontent.com:sub']
        self.assertEqual(subject.count('*'), 3)
        self.assertIn('environment:terraform-prod-pr-plan', subject)
        self.assertIn('event_name:pull_request', subject)
        self.assertEqual(trust['StringEquals']['token.actions.githubusercontent.com:actor_id'], '54093651')
        with self.assertRaises(ValueError):
            pr.aws_trust_policy('staging', '123456789012')


if __name__ == '__main__':
    unittest.main()
