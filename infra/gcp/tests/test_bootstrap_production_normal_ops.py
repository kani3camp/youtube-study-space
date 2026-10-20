#!/usr/bin/env python3
"""Offline production normal bootstrap trust and least-privilege contracts."""
from copy import deepcopy
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))
import bootstrap_production_normal_ops as prod
import bootstrap_normal_ops as dev


def aws_policy(role):
    return {
        "Version": "2012-10-17",
        "Statement": [{
            "Effect": "Allow",
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Principal": {"Federated":
                          "arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"},
            "Condition": {
                "StringEquals": {"token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
                                 "token.actions.githubusercontent.com:actor_id": "54093651"},
                "StringLike": {"token.actions.githubusercontent.com:sub": prod.subject(role)},
            }
        }]
    }


def github_env(*names):
    return {"deployment_branch_policy": {"custom_branch_policies": True,
                                          "protected_branches": False}}


class ProductionBootstrap(unittest.TestCase):
    def test_prod_condition_is_main_only_and_no_dev_environment(self):
        condition = prod.CONDITION
        self.assertIn("assertion.ref == 'refs/heads/main'", condition)
        self.assertIn("assertion.workflow_sha == assertion.sha", condition)
        self.assertIn("assertion.actor_id == '54093651'", condition)
        self.assertIn("terraform-prod-plan", condition)
        self.assertIn("terraform-prod-apply", condition)
        self.assertNotIn("terraform-dev-plan", condition)
        self.assertNotIn("feature/gcp-terraform-iac", condition)
        self.assertEqual(prod.MAPPING, dev.MAPPING)
        self.assertEqual(prod.PROJECT, "youtube-study-space")

    def test_exact_production_aws_trust_and_native_actor_id(self):
        for role in ("plan", "apply"):
            original = aws_policy(role)
            prod.verify_role(original, "123456789012", role)
            for alter in ("wrong-branch", "wrong-actor", "extra-allow", "wildcard",
                          "not-action", "wrong-audience", "wrong-role"):
                item = deepcopy(original)
                statement = item["Statement"][0]
                if alter == "wrong-branch":
                    statement["Condition"]["StringLike"]["token.actions.githubusercontent.com:sub"] = (
                        prod.subject(role).replace("refs/heads/main", "refs/heads/dev"))
                elif alter == "wrong-actor":
                    statement["Condition"]["StringEquals"]["token.actions.githubusercontent.com:actor_id"] = "123"
                elif alter == "extra-allow":
                    item["Statement"].append({"Effect": "Allow", "NotAction": "iam:*", "Principal": "*"})
                elif alter == "wildcard":
                    statement["Action"] = "sts:*"
                elif alter == "not-action":
                    statement["NotAction"] = "iam:*"
                elif alter == "wrong-audience":
                    statement["Condition"]["StringEquals"]["token.actions.githubusercontent.com:aud"] = "other"
                else:
                    statement["Condition"]["StringLike"]["token.actions.githubusercontent.com:sub"] = prod.subject("apply" if role == "plan" else "plan")
                with self.subTest(role=role, alter=alter), self.assertRaises(ValueError):
                    prod.verify_role(item, "123456789012", role)

    def test_production_wif_provider_strict_readback(self):
        provider = {
            "state": "ACTIVE",
            "oidc": {"issuerUri": "https://token.actions.githubusercontent.com"},
            "attributeMapping": prod.MAPPING,
            "attributeCondition": prod.CONDITION,
        }
        prod.verify_provider(provider)
        for bad in (
            {"state": "DISABLED"},
            {"attributeCondition": prod.CONDITION.replace("refs/heads/main", "refs/heads/dev")},
            {"attributeMapping": {}},
        ):
            with self.assertRaises(ValueError):
                prod.verify_provider(provider | bad)

    def test_production_permissions_exclude_content_secrets_and_global_iam(self):
        catalog = {
            "run.services.create", "run.services.get", "run.services.setIamPolicy", "pubsub.topics.create",
            "pubsub.topics.get", "storage.objects.get",
            "bigquery.tables.getData", "bigquery.tables.updateData",
            "secretmanager.versions.access", "secretmanager.secrets.setIamPolicy",
            "storage.buckets.setIamPolicy", "iam.workloadIdentityPools.setIamPolicy",
            "resourcemanager.projects.setIamPolicy", "iam.serviceAccounts.create",
        }
        permissions = prod.production_permissions(catalog)
        self.assertIn("pubsub.topics.create", permissions["apply"])
        self.assertIn("run.services.create", permissions["apply"])
        self.assertNotIn("run.services.create", permissions["plan"])
        for secret in ("run.services.setIamPolicy", "secretmanager.secrets.setIamPolicy",
                       "storage.buckets.setIamPolicy", "storage.objects.get", "bigquery.tables.getData",
                       "bigquery.tables.updateData", "secretmanager.versions.access",
                       "iam.workloadIdentityPools.setIamPolicy",
                       "resourcemanager.projects.setIamPolicy"):
            self.assertNotIn(secret, permissions["apply"], secret)

    def test_production_github_environment_requires_main_branch(self):
        names = ["main"]
        required = {
            "GCP_TERRAFORM_MANAGED_INPUTS_JSON",
            "AWS_TERRAFORM_BACKEND_ROLE_ARN",
            "AWS_TERRAFORM_BACKEND_ROLE_ID",
            "AWS_TERRAFORM_STATE_BUCKET",
            "AWS_TERRAFORM_STATE_ACCOUNT_ID",
        }
        def gh(*args):
            endpoint = args[0]
            if endpoint.endswith("customization/sub"):
                return {"use_default": False, "include_claim_keys": list(prod.CLAIMS)}
            if endpoint == "user":
                return {"login": "kani3camp", "id": 54093651}
            if endpoint.endswith("/deployment-branch-policies?per_page=100"):
                return {"total_count": len(names),
                        "branch_policies": [{"name": name, "type": "branch"} for name in names]}
            if endpoint.endswith("/secrets"):
                return {"secrets": [{"name": name} for name in required]}
            return github_env()
        with patch.object(prod, "gh", side_effect=gh):
            prod.preflight_github()
        names[:] = ["dev"]
        with patch.object(prod, "gh", side_effect=gh), self.assertRaises(ValueError):
            prod.preflight_github()
        names[:] = ["main", "feature/*"]
        with patch.object(prod, "gh", side_effect=gh), self.assertRaises(ValueError):
            prod.preflight_github()

    def test_check_mode_is_read_only_and_does_not_bootstrap(self):
        args = ["script", "--check"]
        permitted = {"plan": ["resourcemanager.projects.get"],
                     "apply": ["resourcemanager.projects.get", "pubsub.topics.create"]}
        with patch.object(sys, "argv", args), \
             patch.object(prod, "preflight_cloud", return_value="123456789012"), \
             patch.object(prod, "preflight_github"), \
             patch.object(prod, "permissions", return_value=("12345", permitted)), \
             patch.object(prod, "bootstrap", side_effect=AssertionError("must not mutate")):
            prod.main()
        with patch.object(sys, "argv", ["script", "--random"]), self.assertRaises(ValueError):
            prod.main()


if __name__ == "__main__":
    unittest.main()
