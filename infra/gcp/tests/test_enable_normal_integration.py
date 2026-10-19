#!/usr/bin/env python3
"""Credentialless tests for DEV-only integration ref bootstrap.

All identities, roles and policy documents here are invented fixtures.
"""
from copy import deepcopy
from pathlib import Path
from unittest.mock import patch
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


    def test_existing_dev_backend_legacy_wildcard_is_preserved(self):
        for kind in ("plan", "apply"):
            initial = role(kind)
            conditions = initial["AssumeRolePolicyDocument"]["Statement"][0]["Condition"]
            conditions["StringEquals"].pop("token.actions.githubusercontent.com:sub")
            conditions["StringLike"] = {
                "token.actions.githubusercontent.com:sub":
                    op.custom_subject(kind, op.LEGACY_AWS_REF)}
            original = deepcopy(initial["AssumeRolePolicyDocument"])
            # Live DEV roles use exactly this historical seven-claim wildcard
            # and serve existing migration workflows. Preserve it unchanged.
            self.assertEqual(op.extended_trust(original, kind), original)
            self.assertEqual(op.eligible_roles([initial, role("apply" if kind == "plan" else "plan")])[kind]["RoleName"],
                             initial["RoleName"])

    def test_only_exact_legacy_wildcard_is_accepted(self):
        for kind in ("plan", "apply"):
            initial = role(kind)["AssumeRolePolicyDocument"]
            rule = initial["Statement"][0]["Condition"]
            rule["StringEquals"].pop("token.actions.githubusercontent.com:sub")
            legacy = op.custom_subject(kind, op.LEGACY_AWS_REF)
            rule["StringLike"] = {"token.actions.githubusercontent.com:sub": legacy}
            self.assertEqual(op.extended_trust(initial, kind), initial)
            for invalid in (
                legacy.replace("repository_id:340900071", "repository_id:999999999"),
                legacy.replace("refs/heads/*", "*"),
                legacy.replace("/ci.yml@refs/heads/*", "/evil.yml@refs/heads/*"),
            ):
                candidate = deepcopy(initial)
                candidate["Statement"][0]["Condition"]["StringLike"][
                    "token.actions.githubusercontent.com:sub"] = invalid
                with self.subTest(kind=kind, invalid=invalid[:20]), self.assertRaises(op.Stop):
                    op.extended_trust(candidate, kind)
            candidate = deepcopy(initial)
            candidate["Statement"][0]["Condition"]["StringEquals"][
                "token.actions.githubusercontent.com:sub"] = legacy
            candidate["Statement"][0]["Condition"]["StringLike"].pop(
                "token.actions.githubusercontent.com:sub")
            with self.assertRaises(op.Stop):
                op.extended_trust(candidate, kind)

    def test_aws_profile_is_optional_and_setup_never_calls_aws(self):
        # Existing shared AWS backend trust was verified independently.
        # Local gcloud and gh credentials are sufficient for setup.
        provider = {"state": "ACTIVE",
                    "oidc": {"issuerUri": "https://token.actions.githubusercontent.com"},
                    "attributeMapping": bootstrap.MAPPING,
                    "attributeCondition": bootstrap.CONDITION}
        policies = {"total_count": 2, "branch_policies": [
            {"name": "dev", "type": "branch"},
            {"name": op.BRANCH, "type": "branch"}]}
        def read_github(*args, **kwargs):
            address = args[0]
            if address.endswith("/actions/oidc/customization/sub"):
                return {"use_default": False, "include_claim_keys": list(op.CLAIM_KEYS)}
            if address.endswith("/deployment-branch-policies?per_page=100"):
                return policies
            return {"deployment_branch_policy": {"custom_branch_policies": True,
                                                  "protected_branches": False}}

        with patch.object(op, "aws", side_effect=AssertionError("AWS must not run")), patch.object(
                op, "gcloud", return_value=provider), patch.object(
                op, "gh", side_effect=read_github), patch.object(
                sys, "argv", ["bootstrap", "--apply"]):
            op.main()

    def test_aws_audit_explicitly_requires_local_identity(self):
        with patch.object(op, "aws", side_effect=op.Stop("session expired")), patch.object(
                sys, "argv", ["bootstrap", "--verify-aws"]):
            with self.assertRaises(op.Stop):
                op.main()
            self.assertEqual(op.PHASE, "aws_identity")

    def test_stop_phase_is_fixed_and_never_includes_private_cli_data(self):
        with patch.object(op, "aws", side_effect=op.Stop("PRIVATE_CANARY")), patch.object(
                sys, "argv", ["bootstrap", "--verify-aws"]):
            with self.assertRaises(op.Stop):
                op.main()
            self.assertEqual(op.PHASE, "aws_identity")

    def test_default_oidc_subject_is_rejected(self):
        original = role("plan")["AssumeRolePolicyDocument"]
        original["Statement"][0]["Condition"]["StringEquals"][
            "token.actions.githubusercontent.com:sub"
        ] = "repo:" + op.REPO + ":environment:terraform-dev-plan"
        with self.assertRaises(op.Stop):
            op.extended_trust(original, "plan")

    def test_oidc_template_must_match_ordered_custom_claims(self):
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

    def test_role_readback_requires_exact_raw_policy(self):
        fixture = role("plan")
        wanted = op.extended_trust(fixture["AssumeRolePolicyDocument"], "plan")
        stale = {"Role": fixture}
        current = {"Role": {**fixture, "AssumeRolePolicyDocument": wanted}}
        with patch.object(op, "aws", side_effect=[stale, current]) as aws, patch.object(
                op.time, "sleep") as sleeping:
            op.verify_role_readback(fixture["RoleName"], fixture["RoleId"], wanted)
            self.assertEqual(aws.call_count, 2)
            sleeping.assert_called_once()
        with patch.object(op, "aws", return_value=stale), patch.object(op.time, "sleep"), self.assertRaises(op.Stop):
            op.verify_role_readback(fixture["RoleName"], fixture["RoleId"], wanted)

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
