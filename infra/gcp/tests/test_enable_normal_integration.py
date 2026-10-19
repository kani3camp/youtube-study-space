#!/usr/bin/env python3
"""Credentialless tests for DEV-only integration ref bootstrap.

All identities, roles and policy documents here are invented fixtures.
"""
from copy import deepcopy
from pathlib import Path
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))
import enable_normal_integration as op
import bootstrap_normal_ops as bootstrap

ACCOUNT = "123456789012"


def role(kind):
    provider = f"arn:aws:iam::{ACCOUNT}:oidc-provider/token.actions.githubusercontent.com"
    sub = op.custom_subject(kind, op.OLD_REF)
    conditions = {
        "StringEquals": {
            "token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
            "token.actions.githubusercontent.com:sub": sub,
        },
        "StringLike": {
            "token.actions.githubusercontent.com:ref": op.OLD_REF,
            "token.actions.githubusercontent.com:workflow_ref":
                op.REPO + "/.github/workflows/ci.yml@" + op.OLD_REF,
            "token.actions.githubusercontent.com:job_workflow_ref":
                op.REPO + "/.github/workflows/gcp-terraform-authenticated.yml@" + op.OLD_REF,
        },
    }
    return {"RoleName": "terraform-state-dev-" + kind, "RoleId": "AROA" + kind,
            "Arn": f"arn:aws:iam::{ACCOUNT}:role/terraform-state-dev-{kind}",
            "AssumeRolePolicyDocument": {
                "Version": "2012-10-17", "Statement": [
                    {"Effect": "Allow", "Action": "sts:AssumeRoleWithWebIdentity",
                     "Principal": {"Federated": provider}, "Condition": conditions},
                ],
            },
    }


class IntegrationTrustTest(unittest.TestCase):
    def test_exact_dual_branch_mapping_and_actor_bound(self):
        condition = bootstrap.CONDITION
        self.assertIn("refs/heads/integration/gcp-terraform-normal-ops", condition)
        self.assertIn("assertion.ref in ['refs/heads/dev',", condition)
        self.assertIn("assertion.workflow_ref ==", condition)
        self.assertIn("' + assertion.ref", condition)
        self.assertIn("assertion.repository_id == '340900071'", condition)
        self.assertIn("assertion.job_workflow_sha == assertion.sha", condition)
        self.assertNotIn("refs/heads/*", condition)

    def test_aws_trust_expands_only_allowed_branch_claims(self):
        plan, apply = role("plan"), role("apply")
        roles = op.eligible_roles([plan, apply])
        self.assertEqual(set(roles), {"plan", "apply"})
        for kind, item in roles.items():
            original = deepcopy(item["AssumeRolePolicyDocument"])
            updated = op.extended_trust(original, kind)
            self.assertEqual(item["AssumeRolePolicyDocument"], original)
            old = original["Statement"][0]["Condition"]
            new = updated["Statement"][0]["Condition"]
            self.assertEqual(old["StringEquals"]["token.actions.githubusercontent.com:aud"],
                             new["StringEquals"]["token.actions.githubusercontent.com:aud"])
            self.assertEqual(new["StringEquals"]["token.actions.githubusercontent.com:sub"],
                             [op.custom_subject(kind, op.OLD_REF),
                              op.custom_subject(kind, op.GIT_REF)])
            for name in ("ref", "workflow_ref", "job_workflow_ref"):
                key = "token.actions.githubusercontent.com:" + name
                self.assertEqual(new["StringLike"][key],
                                 [old["StringLike"][key],
                                  old["StringLike"][key].replace(op.OLD_REF, op.GIT_REF)])
            self.assertEqual(updated, op.extended_trust(updated, kind))

    def test_aws_rejects_broad_or_unrecognized_trust(self):
        valid = role("apply")["AssumeRolePolicyDocument"]
        variants = [
            lambda policy: policy["Statement"][0]["Condition"]["StringLike"].update({
                "token.actions.githubusercontent.com:ref": "refs/heads/*"}),
            lambda policy: policy["Statement"][0]["Condition"]["StringEquals"].update({
                "token.actions.githubusercontent.com:sub": "repository_id:999999999"}),
            lambda policy: policy["Statement"][0]["Principal"].update(Federated="other"),
            lambda policy: policy["Statement"][0].update(Action="sts:AssumeRole"),
        ]
        for mutate in variants:
            candidate = deepcopy(valid)
            mutate(candidate)
            with self.subTest(index=variants.index(mutate)), self.assertRaises(op.Stop):
                op.extended_trust(candidate, "apply")
        with self.assertRaises(op.Stop):
            op.eligible_roles([role("plan"), role("plan"), role("apply")])

    def test_selected_github_branch_policies_reject_patterns(self):
        base = {"branch_policies": [
            {"name": "dev", "type": "branch"},
            {"name": op.BRANCH, "type": "branch"}],
            "total_count": 2}
        self.assertTrue(op.integration_present(base))
        self.assertFalse(op.integration_present(
            {"branch_policies": [{"name": "dev", "type": "branch"}], "total_count": 1}))
        for name in ("*", "integration/*", "release?"):
            with self.subTest(name=name), self.assertRaises(op.Stop):
                op.integration_present({
                    "branch_policies": [{"name": "dev", "type": "branch"},
                                        {"name": name, "type": "branch"}]})

    def test_gcp_provider_rejects_unexpected_condition(self):
        base = {"state": "ACTIVE",
                "oidc": {"issuerUri": "https://token.actions.githubusercontent.com"},
                "attributeMapping": bootstrap.MAPPING,
                "attributeCondition": op.OLD_CONDITION}
        op.check_provider(base)
        op.check_provider(base | {"attributeCondition": bootstrap.CONDITION})
        for key, val in (("state", "DISABLED"), ("attributeMapping", {}),
                         ("attributeCondition", "true")):
            with self.subTest(key=key), self.assertRaises(op.Stop):
                op.check_provider(base | {key: val})


if __name__ == "__main__":
    unittest.main()
