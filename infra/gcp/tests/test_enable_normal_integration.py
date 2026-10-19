#!/usr/bin/env python3
"""Credentialless contracts for GitHub-Environment-driven DEV Terraform access."""
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


def policies(*names):
    return {"total_count": len(names),
            "branch_policies": [{"name": name, "type": "branch"} for name in names]}


class BranchAuthorizationTest(unittest.TestCase):
    def test_gcp_condition_is_branch_name_agnostic(self):
        condition = bootstrap.CONDITION
        self.assertIn("assertion.ref.startsWith('refs/heads/')", condition)
        self.assertIn("assertion.ref != 'refs/heads/main'", condition)
        self.assertIn("assertion.workflow_ref ==", condition)
        self.assertIn("' + assertion.ref", condition)
        self.assertIn("assertion.repository_id == '340900071'", condition)
        self.assertIn("assertion.job_workflow_sha == assertion.sha", condition)
        self.assertNotIn("refs/heads/dev", condition)
        self.assertNotIn("integration/gcp-terraform-normal-ops", condition)

    def test_branch_names_are_selected_by_github_only(self):
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

    def test_github_environment_requires_exact_selected_branches(self):
        self.assertTrue(op.branch_present(
            policies("dev", "feature/a"), "feature/a"))
        self.assertFalse(op.branch_present(
            policies("dev"), "feature/a"))
        for name in ("*", "feature/*", "release?"):
            with self.subTest(name=name), self.assertRaises(op.Stop):
                op.exact_policy_names(policies("dev", name))
        with self.assertRaises(op.Stop):
            op.branch_present(policies("feature/a"), "feature/a")

    def test_default_apply_mutates_github_only(self):
        current = {
            "plan": ("plan-endpoint", policies("dev")),
            "apply": ("apply-endpoint", policies("dev")),
        }
        updated = {
            "plan": ("plan-endpoint", policies("dev", op.DEFAULT_BRANCH)),
            "apply": ("apply-endpoint", policies("dev", op.DEFAULT_BRANCH)),
        }
        calls = {"plan": 0, "apply": 0}

        def read(role):
            calls[role] += 1
            return current[role] if calls[role] == 1 else updated[role]

        with patch.object(op, "aws", side_effect=AssertionError("AWS must not run")),              patch.object(op, "gcloud", side_effect=AssertionError("GCP must not run")),              patch.object(op, "verify_oidc_template"),              patch.object(op, "branch_policies", side_effect=read),              patch.object(op, "add_branch") as add,              patch.object(sys, "argv", ["tool", "--apply"]):
            op.main()
        self.assertEqual(add.call_count, 2)

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
        with patch.object(op, "verify_oidc_template",
                          side_effect=op.Stop("PRIVATE_CANARY")),              patch.object(sys, "argv", ["tool"]),              redirect_stdout(io.StringIO()) as output:
            with self.assertRaises(op.Stop):
                op.main()
        self.assertEqual(op.PHASE, "github_oidc_template")
        self.assertNotIn("PRIVATE_CANARY", output.getvalue())


if __name__ == "__main__":
    unittest.main()
