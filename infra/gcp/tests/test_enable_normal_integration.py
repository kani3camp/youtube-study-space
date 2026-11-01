#!/usr/bin/env python3
"""DEVの本人限定・任意ブランチ実行とPR Plan、Environmentのbranch制限解除の範囲を検証する。"""
from copy import deepcopy
from contextlib import redirect_stdout
import io
from pathlib import Path
import sys
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))
import enable_normal_integration as op
import bootstrap_normal_ops as bootstrap

ACCOUNT = "123456789012"


def role(kind):
    provider = f"arn:aws:iam::{ACCOUNT}:oidc-provider/token.actions.githubusercontent.com"
    condition = {
        "StringEquals": {"token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
                         "token.actions.githubusercontent.com:actor_id": "54093651"},
        "StringLike": {
            "token.actions.githubusercontent.com:sub":
                op.custom_subject(kind, op.LEGACY_AWS_REF)
        },
    }
    return {"RoleName": "youtube-study-space-terraform-dev-" + kind,
            "RoleId": "AROA" + kind,
            "Arn": f"arn:aws:iam::{ACCOUNT}:role/youtube-study-space-terraform-dev-{kind}",
            "AssumeRolePolicyDocument": {
                "Version": "2012-10-17",
                "Statement": [{"Effect": "Allow",
                               "Action": "sts:AssumeRoleWithWebIdentity",
                               "Principal": {"Federated": provider},
                               "Condition": condition}],
            }}


def environment(role, restricted=True):
    return {'id': 100 if role == 'plan' else 200, 'name': 'terraform-dev-' + role,
            'can_admins_bypass': True,
            'deployment_branch_policy': {'custom_branch_policies': True, 'protected_branches': False} if restricted else None,
            'protection_rules': [{'type': 'branch_policy'}] if restricted else []}


class BranchAuthorizationTest(unittest.TestCase):
    def test_gcp_condition_is_branch_name_agnostic(self):
        condition = bootstrap.CONDITION
        self.assertLessEqual(len(bootstrap.condition(op.CALLERS)), 4096)
        self.assertIn("assertion.ref.startsWith('refs/heads/')", condition)
        self.assertIn("assertion.ref != 'refs/heads/main'", condition)
        self.assertIn("assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + assertion.ref", condition)
        self.assertIn("' + assertion.ref", condition)
        self.assertIn("assertion.repository_id == '340900071'", condition)
        self.assertIn("assertion.job_workflow_sha == assertion.sha", condition)
        self.assertNotIn("refs/heads/dev", condition)
        self.assertNotIn("integration/gcp-terraform-normal-ops", condition)

    def test_gcp_pr_clause_admits_only_the_plan_environment_from_terraform_yml(self):
        condition = bootstrap.CONDITION
        dispatch, pull_request = condition.split(' || ')
        self.assertIn("assertion.event_name == 'workflow_dispatch'", dispatch)
        self.assertIn("assertion.event_name == 'pull_request'", pull_request)
        self.assertIn("assertion.environment == 'terraform-dev-plan'", pull_request)
        self.assertNotIn('terraform-dev-apply', pull_request)
        self.assertIn("assertion.ref.matches('^refs/pull/[1-9][0-9]{0,8}/merge$')", pull_request)
        self.assertIn("gcp-terraform-authenticated.yml@' + assertion.ref", pull_request)
        # The transitional ci.yml caller is never admitted for a PR.
        self.assertNotIn('ci.yml', bootstrap.condition(op.CALLERS).split(' || ')[1])
        # Owner, repository and retry limits apply to both clauses.
        for clause in ("assertion.actor_id == '54093651'", "assertion.run_attempt == '1'",
                       "assertion.repository_id == '340900071'"):
            self.assertEqual(condition.count(clause), 1)
            self.assertNotIn(clause, condition[condition.index('(('):condition.index('))')])

    def test_pr_subject_is_plan_only_and_terraform_yml_only(self):
        subject = op.custom_subject('plan', op.PR_AWS_REF, event_name='pull_request')
        self.assertIn('environment:terraform-dev-plan', subject)
        self.assertIn('event_name:pull_request', subject)
        self.assertIn('gcp-terraform-authenticated.yml@refs/pull/*/merge', subject)
        for args in (('apply', op.PR_AWS_REF, 'terraform.yml', 'pull_request'),
                     ('plan', op.PR_AWS_REF, 'ci.yml', 'pull_request'),
                     ('plan', op.PR_AWS_REF, 'terraform.yml', 'push')):
            with self.subTest(args=args), self.assertRaises(op.Stop):
                op.custom_subject(*args)

    def test_existing_aws_branch_agnostic_trust_is_read_only_valid(self):
        for kind in ("plan", "apply"):
            fixture = role(kind)
            original = deepcopy(fixture)
            op.audit_aws_role(fixture["AssumeRolePolicyDocument"], kind)
            self.assertEqual(fixture, original)
            invalid = deepcopy(fixture["AssumeRolePolicyDocument"])
            invalid["Statement"][0]["Condition"]["StringLike"][
                "token.actions.githubusercontent.com:sub"] = op.custom_subject(
                    kind, "refs/heads/dev")
            with self.assertRaises(op.Stop):
                op.audit_aws_role(invalid, kind)

    def test_plan_role_may_add_the_pr_subject_but_apply_role_may_not(self):
        pr_subject = op.custom_subject('plan', op.PR_AWS_REF, event_name='pull_request')
        for kind in ('plan', 'apply'):
            document = role(kind)['AssumeRolePolicyDocument']
            conditions = document['Statement'][0]['Condition']
            subject = pr_subject.replace('terraform-dev-plan', 'terraform-dev-' + kind)
            conditions['StringLike'][op.ISSUER + ':sub'] = [op.custom_subject(kind, op.LEGACY_AWS_REF), subject]
            if kind == 'plan':
                op.audit_aws_role(document, kind)
            else:
                with self.assertRaises(op.Stop):
                    op.audit_aws_role(document, kind)

    def test_aws_trust_admits_only_terraform_yml_and_rejects_the_retired_ci_yml_caller(self):
        self.assertEqual(op.CALLERS, ('terraform.yml',))
        for kind in ('plan', 'apply'):
            document = role(kind)['AssumeRolePolicyDocument']
            conditions = document['Statement'][0]['Condition']
            subjects = [op.custom_subject(kind, op.LEGACY_AWS_REF, caller) for caller in op.CALLERS]
            conditions['StringLike'][op.ISSUER + ':sub'] = subjects
            op.audit_aws_role(document, kind)
            retired = subjects[0].replace('terraform.yml@', 'ci.yml@')
            for value in (subjects + ['*'], subjects + [subjects[0]], subjects + [retired], [retired],
                          [subjects[0].replace('terraform.yml@', 'attacker.yml@')],
                          [subjects[0].replace('environment:terraform-dev-', 'environment:terraform-prod-')]):
                invalid = deepcopy(document)
                invalid['Statement'][0]['Condition']['StringLike'][op.ISSUER + ':sub'] = value
                with self.assertRaises(op.Stop):
                    op.audit_aws_role(invalid, kind)

    def test_aws_trust_rejects_wrong_repo_audience_principal_or_action(self):
        base = role("apply")["AssumeRolePolicyDocument"]
        mutations = [
            lambda p: p["Statement"][0]["Condition"]["StringLike"].update({
                "token.actions.githubusercontent.com:sub":
                    op.custom_subject("apply", op.LEGACY_AWS_REF).replace(
                        "repository_id:340900071", "repository_id:999999999")}),
            lambda p: p["Statement"][0]["Condition"]["StringEquals"].update({
                "token.actions.githubusercontent.com:aud": "wrong"}),
            lambda p: p["Statement"][0]["Principal"].update(Federated="wrong"),
            lambda p: p["Statement"][0].update(Action="sts:AssumeRole"),
        ]
        for mutate in mutations:
            candidate = deepcopy(base)
            mutate(candidate)
            with self.subTest(index=mutations.index(mutate)), self.assertRaises(op.Stop):
                op.audit_aws_role(candidate, "apply")

    def test_aws_federated_principal_array_is_audited(self):
        document = role("apply")["AssumeRolePolicyDocument"]
        trusted = document["Statement"][0]
        trusted["Principal"]["Federated"] = [trusted["Principal"]["Federated"]]
        op.audit_aws_role(document, "apply")
        # A second relaxed array-form grant must not be ignored.
        loose = deepcopy(trusted)
        loose["Condition"] = {"StringLike": {op.ISSUER + ":sub": "*"}}
        document["Statement"].append(loose)
        with self.assertRaises(op.Stop):
            op.audit_aws_role(document, "apply")

    def test_aws_rejects_all_unreviewed_webidentity_allow_shapes(self):
        base = role("apply")["AssumeRolePolicyDocument"]
        for alteration in (
            {"Principal": "*"},
            {"Principal": {"Federated": ["wrong", "other"]}},
            {"Action": "sts:*"},
            {"Action": "sts:assumerolewithwebidentity"},
            {"NotAction": "iam:*"},
            {"NotPrincipal": {"AWS": "*"}},
        ):
            candidate = deepcopy(base)
            candidate["Statement"].append(deepcopy(base["Statement"][0]) | alteration)
            with self.subTest(alteration=alteration), self.assertRaises(op.Stop):
                op.audit_aws_role(candidate, "apply")

    def test_aws_requires_native_immutable_owner_claim(self):
        for invalid in (None, "attacker", ["54093651", "attacker"]):
            candidate = role("apply")["AssumeRolePolicyDocument"]
            equals = candidate["Statement"][0]["Condition"]["StringEquals"]
            if invalid is None:
                del equals[op.ISSUER + ":actor_id"]
            else:
                equals[op.ISSUER + ":actor_id"] = invalid
            with self.subTest(invalid=invalid), self.assertRaises(op.Stop):
                op.audit_aws_role(candidate, "apply")

    def test_environment_read_accepts_existing_restriction_or_no_restriction(self):
        for role in ('plan', 'apply'):
            for restricted in (True, False):
                value = environment(role, restricted)
                with patch.object(op, 'gh', return_value=value):
                    endpoint, result = op.dev_environment(role)
                self.assertEqual(endpoint, f'repos/{op.REPO}/environments/terraform-dev-{role}')
                self.assertEqual(result, value)
        with patch.object(op, 'gh') as gh, self.assertRaises(op.Stop):
            op.dev_environment('prod-plan')
        gh.assert_not_called()

    def test_existing_environment_and_other_protections_cannot_be_silently_replaced(self):
        for change in ({'name': 'terraform-prod-plan'}, {'id': 'bad'},
                       {'deployment_branch_policy': {}},
                       {'protection_rules': [{'type': 'required_reviewers'}]},
                       {'protection_rules': [{'type': 'wait_timer', 'wait_timer': 5}]}):
            with patch.object(op, 'gh', return_value=environment('plan') | change), self.assertRaises(op.Stop):
                op.dev_environment('plan')
        value = environment('plan'); del value['deployment_branch_policy']
        with patch.object(op, 'gh', return_value=value), self.assertRaises(op.Stop):
            op.dev_environment('plan')

    def github_for_apply(self, history, restricted=True, operator=None):
        settings = {role: environment(role, restricted) for role in ('plan', 'apply')}
        def gh(*values, data=None):
            if values == ('user',):
                return operator if operator is not None else {'login': 'kani3camp', 'id': 54093651}
            if values == (f'repos/{op.REPO}/actions/oidc/customization/sub',):
                return {'use_default': False, 'include_claim_keys': list(op.CLAIM_KEYS)}
            method, endpoint = (values[1], values[2]) if values[0] == '-X' else ('GET', values[0])
            role = next((role for role in ('plan', 'apply')
                         if endpoint == f'repos/{op.REPO}/environments/terraform-dev-{role}'), None)
            self.assertIsNotNone(role)
            if method == 'PUT':
                self.assertEqual(data, {'deployment_branch_policy': None})
                settings[role] = environment(role, False)
                history.append((method, endpoint))
                return {}
            self.assertEqual(method, 'GET')
            return deepcopy(settings[role])
        return gh

    def test_apply_verifies_owner_and_opens_only_two_dev_environments(self):
        history = []
        def audited():
            self.assertEqual(history, [])
        output = io.StringIO()
        with patch.object(op, 'gh', side_effect=self.github_for_apply(history)), \
                patch.object(op, 'verify_aws', side_effect=audited) as aws, \
                patch.object(op, 'verify_gcp', side_effect=audited) as gcp, \
                patch.object(sys, 'argv', ['tool', '--apply']), redirect_stdout(output):
            op.main()
        aws.assert_not_called(); gcp.assert_not_called()
        self.assertEqual(history, [('PUT', f'repos/{op.REPO}/environments/terraform-dev-plan'),
                                   ('PUT', f'repos/{op.REPO}/environments/terraform-dev-apply')])
        self.assertIn('no branch restriction', output.getvalue())

    def test_apply_is_idempotent_and_preview_never_writes(self):
        for args, restricted in ((['tool'], True), (['tool', '--apply'], False)):
            history = []
            with patch.object(op, 'gh', side_effect=self.github_for_apply(history, restricted)), \
                    patch.object(op, 'verify_aws') as aws, patch.object(op, 'verify_gcp') as gcp, \
                    patch.object(sys, 'argv', args), redirect_stdout(io.StringIO()):
                op.main()
            self.assertEqual(history, [])
            aws.assert_not_called(); gcp.assert_not_called()

    def test_wrong_operator_or_failed_cloud_trust_prevents_all_environment_writes(self):
        for operator in ({'login': 'other', 'id': 54093651}, {'login': 'kani3camp', 'id': 0}):
            history = []
            with patch.object(op, 'gh', side_effect=self.github_for_apply(history, operator=operator)), \
                    patch.object(op, 'verify_aws') as aws, patch.object(op, 'verify_gcp'), \
                    patch.object(sys, 'argv', ['tool', '--apply']), self.assertRaises(op.Stop):
                op.main()
            self.assertEqual(history, []); aws.assert_not_called()
        for name in ('verify_aws', 'verify_gcp'):
            history = []
            with patch.object(op, 'gh', side_effect=self.github_for_apply(history)), \
                    patch.object(op, 'verify_aws'), patch.object(op, 'verify_gcp'), \
                    patch.object(op, name, side_effect=op.Stop('PRIVATE_CANARY')), \
                    patch.object(sys, 'argv', ['tool', '--apply', '--' + name.replace('_', '-')]), self.assertRaises(op.Stop):
                op.main()
            self.assertEqual(history, [])

    def test_failed_readback_is_not_reported_as_success(self):
        for change in ({'deployment_branch_policy': {'custom_branch_policies': True, 'protected_branches': False}},
                       {'id': 999}, {'can_admins_bypass': False}):
            history = []
            def read(role):
                value = environment(role, True)
                return (f'repos/{op.REPO}/environments/terraform-dev-{role}',
                        environment(role, False) | change if history else value)
            def update(endpoint, current):
                history.append(endpoint)
            output = io.StringIO()
            with patch.object(op, 'verify_operator'), patch.object(op, 'verify_oidc_template'), \
                    patch.object(op, 'verify_aws'), patch.object(op, 'verify_gcp'), \
                    patch.object(op, 'dev_environment', side_effect=read), \
                    patch.object(op, 'open_environment', side_effect=update), \
                    patch.object(sys, 'argv', ['tool', '--apply']), redirect_stdout(output), self.assertRaises(op.Stop):
                op.main()
            self.assertNotIn('no branch restriction', output.getvalue())

    def test_update_cannot_target_production_or_another_environment(self):
        for name in ('terraform-prod-plan', 'terraform-prod-apply', 'codeql-analysis'):
            with patch.object(op, 'gh') as gh, self.assertRaises(op.Stop):
                op.open_environment(f'repos/{op.REPO}/environments/{name}', environment('plan'))
            gh.assert_not_called()

    def test_optional_cloud_audits_are_read_only(self):
        aws_role_plan, aws_role_apply = role("plan"), role("apply")
        with patch.object(op, "aws", side_effect=[
                {"Account": ACCOUNT},
                {"Role": aws_role_plan},
                {"Role": aws_role_apply}]):
            op.verify_aws()

        provider = {"state": "ACTIVE",
                    "oidc": {"issuerUri": "https://token.actions.githubusercontent.com"},
                    "attributeMapping": bootstrap.MAPPING,
                    "attributeCondition": bootstrap.CONDITION}
        with patch.object(op, "provider_details", return_value=provider):
            op.verify_gcp()

    def test_gcp_readback_rejects_named_branch_conditions(self):
        provider = {"state": "ACTIVE",
                    "oidc": {"issuerUri": "https://token.actions.githubusercontent.com"},
                    "attributeMapping": bootstrap.MAPPING,
                    "attributeCondition": bootstrap.CONDITION}
        op.check_provider(provider)
        with self.assertRaises(op.Stop):
            op.check_provider(provider | {
                "attributeCondition": bootstrap.CONDITION + " && assertion.ref == 'refs/heads/dev'"})

    def test_gcp_readback_admits_only_terraform_yml_and_rejects_the_retired_dual_caller(self):
        # The retired ci.yml caller was removed from the DEV trust; re-adding it must fail the audit.
        condition = bootstrap.CONDITION
        self.assertEqual(condition, bootstrap.condition(op.CALLERS))
        self.assertNotIn('ci.yml', condition)
        provider = {'state': 'ACTIVE', 'oidc': {'issuerUri': 'https://token.actions.githubusercontent.com'},
                    'attributeMapping': bootstrap.MAPPING, 'attributeCondition': condition}
        op.check_provider(provider)
        for invalid in (bootstrap.condition(('ci.yml', 'terraform.yml')),
                        condition.replace("assertion.actor_id == '54093651'", 'true'),
                        condition.replace('terraform.yml@', 'attacker.yml@'),
                        condition.replace('refs/heads/', 'refs/'),
                        condition.replace("assertion.environment == 'terraform-dev-plan'",
                                          "assertion.environment == 'terraform-dev-apply'"),
                        bootstrap.CONDITION.replace('terraform.yml@', 'ci.yml@')):
            with self.assertRaises(op.Stop):
                op.check_provider(provider | {'attributeCondition': invalid})

    def test_oidc_template_requires_ordered_custom_claims(self):
        exact = {"use_default": False, "include_claim_keys": list(op.CLAIM_KEYS)}
        with patch.object(op, "gh", return_value=exact):
            op.verify_oidc_template()
        for invalid in (
            {"use_default": True, "include_claim_keys": list(op.CLAIM_KEYS)},
            {"use_default": False, "include_claim_keys": list(reversed(op.CLAIM_KEYS))},
            {"use_default": False, "include_claim_keys": list(op.CLAIM_KEYS[:-1])},
        ):
            with patch.object(op, "gh", return_value=invalid), self.assertRaises(op.Stop):
                op.verify_oidc_template()

    def test_stop_phase_does_not_echo_provider_values(self):
        with patch.object(op, 'verify_operator'), patch.object(op, "verify_oidc_template",
                          side_effect=op.Stop("PRIVATE_CANARY")),              patch.object(sys, "argv", ["tool"]),              redirect_stdout(io.StringIO()) as output:
            with self.assertRaises(op.Stop):
                op.main()
        self.assertEqual(op.PHASE, "github_oidc_template")
        self.assertNotIn("PRIVATE_CANARY", output.getvalue())


if __name__ == "__main__":
    unittest.main()
