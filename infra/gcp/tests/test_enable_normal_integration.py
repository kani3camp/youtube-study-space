#!/usr/bin/env python3
"""DEVの本人限定・任意ブランチ実行と、Environment変更範囲を検証する。"""
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
        self.assertIn("assertion.ref.startsWith('refs/heads/')", condition)
        self.assertIn("assertion.ref != 'refs/heads/main'", condition)
        self.assertIn("assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + assertion.ref", condition)
        self.assertIn("' + assertion.ref", condition)
        self.assertIn("assertion.repository_id == '340900071'", condition)
        self.assertIn("assertion.job_workflow_sha == assertion.sha", condition)
        self.assertNotIn("refs/heads/dev", condition)
        self.assertNotIn("integration/gcp-terraform-normal-ops", condition)

    def test_control_branch_accepts_any_non_main_branch(self):
        for value in ("dev", "feature/x", "integration/gcp-terraform-normal-ops",
                      "fix/foo.bar-1"):
            self.assertEqual(op.branch_name(value), value)
        for value in ("main", "-bad", "bad branch", "refs/heads/dev", "", "../x"):
            with self.subTest(value=value), self.assertRaises(op.Stop):
                op.branch_name(value)

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

    def test_existing_dual_caller_trust_retains_owner_repository_and_workflow_limits(self):
        for kind in ('plan', 'apply'):
            document = role(kind)['AssumeRolePolicyDocument']
            conditions = document['Statement'][0]['Condition']
            subjects = [op.custom_subject(kind, op.LEGACY_AWS_REF, caller) for caller in op.CALLERS]
            conditions['StringLike'][op.ISSUER + ':sub'] = subjects
            op.audit_aws_role(document, kind)
            for value in (subjects + ['*'], subjects + [subjects[0]],
                          [subjects[0].replace('ci.yml@', 'attacker.yml@')],
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
        rules = {role: ([{'id': 1, 'name': 'dev', 'type': 'branch'}] if restricted else [])
                 for role in ('plan', 'apply')}
        def gh(*values, data=None):
            if values == ('user',):
                return operator if operator is not None else {'login': 'kani3camp', 'id': 54093651}
            if values == (f'repos/{op.REPO}/actions/oidc/customization/sub',):
                return {'use_default': False, 'include_claim_keys': list(op.CLAIM_KEYS)}
            method, endpoint = (values[1], values[2]) if values[0] == '-X' else ('GET', values[0])
            role = next((role for role in ('plan', 'apply')
                         if endpoint.startswith(f'repos/{op.REPO}/environments/terraform-dev-{role}')), None)
            self.assertIsNotNone(role)
            if method == 'PUT':
                self.assertEqual(data, {'deployment_branch_policy': {'custom_branch_policies': True, 'protected_branches': False}})
                settings[role] = environment(role, True)
            elif method == 'POST':
                self.assertEqual(data, {'name': 'dev', 'type': 'branch'})
                rules[role].append({'id': 1, **data})
            elif method != 'GET':
                self.fail('予期しないAPI変更')
            if method != 'GET':
                history.append((method, endpoint))
                return {}
            if '/deployment-branch-policies' in endpoint:
                self.assertIsNotNone(settings[role]['deployment_branch_policy'])
                return {'total_count': len(rules[role]), 'branch_policies': deepcopy(rules[role])}
            return deepcopy(settings[role])
        return gh

    def test_apply_verifies_owner_and_restricts_only_two_dev_environments(self):
        history = []
        def audited():
            self.assertEqual(history, [])
        with patch.object(op, 'gh', side_effect=self.github_for_apply(history, restricted=False)), \
                patch.object(op, 'verify_aws', side_effect=audited) as aws, \
                patch.object(op, 'verify_gcp', side_effect=audited) as gcp, \
                patch.object(sys, 'argv', ['tool', '--apply']), redirect_stdout(io.StringIO()):
            op.main()
        aws.assert_not_called(); gcp.assert_not_called()
        self.assertEqual(len(history), 4)
        self.assertTrue(all('/environments/terraform-dev-' in endpoint for _, endpoint in history))
        self.assertEqual([method for method, _ in history], ['PUT', 'POST', 'PUT', 'POST'])

    def test_apply_is_idempotent_and_preview_never_writes(self):
        for args in (['tool'], ['tool', '--apply']):
            history = []
            with patch.object(op, 'gh', side_effect=self.github_for_apply(history)), \
                    patch.object(op, 'verify_aws') as aws, patch.object(op, 'verify_gcp') as gcp, \
                    patch.object(sys, 'argv', args), redirect_stdout(io.StringIO()):
                op.main()
            self.assertEqual(history, [])
            if '--apply' not in args:
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
        for change in ({'deployment_branch_policy': None},
                       {'id': 999}, {'can_admins_bypass': False}):
            history = []
            def read(role):
                value = environment(role, True)
                return (f'repos/{op.REPO}/environments/terraform-dev-{role}',
                        value | change if history else value)
            def update(endpoint, current, target):
                history.append(endpoint)
            output = io.StringIO()
            with patch.object(op, 'verify_operator'), patch.object(op, 'verify_oidc_template'), \
                    patch.object(op, 'verify_aws'), patch.object(op, 'verify_gcp'), \
                    patch.object(op, 'dev_environment', side_effect=read), \
                    patch.object(op, 'authorize_control_branch', side_effect=update), \
                    patch.object(sys, 'argv', ['tool', '--apply']), redirect_stdout(output), self.assertRaises(op.Stop):
                op.main()
            self.assertNotIn('trusted DEV control branch authorized', output.getvalue())

    def test_update_cannot_target_production_or_another_environment(self):
        for name in ('terraform-prod-plan', 'terraform-prod-apply', 'codeql-analysis'):
            with patch.object(op, 'gh') as gh, self.assertRaises(op.Stop):
                op.authorize_control_branch(f'repos/{op.REPO}/environments/{name}', {}, 'dev')
            gh.assert_not_called()

    def test_new_control_branch_preserves_existing_exact_authorizations(self):
        endpoint = f'repos/{op.REPO}/environments/terraform-dev-plan'
        rules = {'total_count': 1, 'branch_policies': [{'id': 1, 'name': 'dev', 'type': 'branch'}]}
        target = 'integration/mypage-canon-20261006'
        final = {'total_count': 2, 'branch_policies': rules['branch_policies'] + [{'id': 2, 'name': target, 'type': 'branch'}]}
        with patch.object(op, 'gh', side_effect=[rules, {}, final]) as gh:
            op.authorize_control_branch(endpoint, environment('plan'), target)
        writes = [call for call in gh.call_args_list if call.args[0] == '-X']
        self.assertEqual(len(writes), 1)
        self.assertEqual(writes[0].args[:2], ('-X', 'POST'))
        self.assertEqual(writes[0].kwargs['data'], {'name': target, 'type': 'branch'})
        with patch.object(op, 'gh', side_effect=[final, final]) as gh:
            op.authorize_control_branch(endpoint, environment('plan'), 'dev')
        self.assertTrue(all(call.args[0] != '-X' for call in gh.call_args_list))

    def test_wildcards_and_tags_cannot_be_silently_trusted(self):
        endpoint = f'repos/{op.REPO}/environments/terraform-dev-plan'
        for name, kind in (('*', 'branch'), ('feature/*', 'branch'), ('dev', 'tag')):
            rules = {'total_count': 1, 'branch_policies': [{'id': 1, 'name': name, 'type': kind}]}
            with patch.object(op, 'gh', return_value=rules) as gh, self.assertRaises(op.Stop):
                op.authorize_control_branch(endpoint, environment('plan'), 'dev')
            self.assertTrue(all(call.args[0] != '-X' for call in gh.call_args_list))


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

    def test_gcp_readback_accepts_existing_dual_caller_without_relaxing_other_claims(self):
        # DEV trust still lists the retired ci.yml caller next to terraform.yml during the migration.
        caller = "assertion.workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + assertion.ref"
        callers = ("assertion.workflow_ref in ['kani3camp/youtube-study-space/.github/workflows/ci.yml@' + assertion.ref, "
                   "'kani3camp/youtube-study-space/.github/workflows/terraform.yml@' + assertion.ref]")
        self.assertIn(caller, bootstrap.CONDITION)
        condition = bootstrap.CONDITION.replace(caller, callers)
        self.assertNotEqual(condition, bootstrap.CONDITION)
        provider = {'state': 'ACTIVE', 'oidc': {'issuerUri': 'https://token.actions.githubusercontent.com'},
                    'attributeMapping': bootstrap.MAPPING, 'attributeCondition': condition}
        op.check_provider(provider)
        for invalid in (condition.replace("assertion.actor_id == '54093651'", 'true'),
                        condition.replace('terraform.yml@', 'attacker.yml@'),
                        condition.replace('refs/heads/', 'refs/'),
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
