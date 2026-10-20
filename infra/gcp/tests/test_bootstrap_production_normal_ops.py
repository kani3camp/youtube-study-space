#!/usr/bin/env python3
"""Offline production normal bootstrap trust and least-privilege contracts."""
from copy import deepcopy
import json
from pathlib import Path
import sys
import subprocess
import tempfile
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
            {"disabled": True},
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
        permissions = prod.production_permissions(catalog, {"resourcemanager.projects.get"})
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

    def test_existing_production_trust_and_project_grants_are_exact(self):
        expected = ("roles/iam.workloadIdentityUser", "principalSet://valid")
        good = {"bindings": [{"role": expected[0], "members": [expected[1]]}]}
        self.assertEqual(prod.check_policy(good, {expected}), {expected})
        self.assertEqual(prod.check_policy({"bindings": []}, {expected}), set())
        for invalid in (
            {"bindings": [{"role": expected[0], "members": ["attacker"]}]},
            {"bindings": [{"role": expected[0], "members": [expected[1], "attacker"]}]},
            {"bindings": [{"role": expected[0], "members": [expected[1]],
                            "condition": {"expression": "true"}}]},
            {"bindings": [{"role": expected[0], "members": [expected[1]]},
                           {"role": "roles/iam.serviceAccountTokenCreator",
                            "members": ["serviceAccount:attacker@example.com"]}]},
        ):
            with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                prod.check_policy(invalid, {expected})
        member = "serviceAccount:terraform-prod-normal-plan@youtube-study-space.iam.gserviceaccount.com"
        scoped = "projects/youtube-study-space/roles/terraformNormalPlan"
        self.assertFalse(prod.check_project_grants({"bindings": []}, member, scoped))
        self.assertTrue(prod.check_project_grants({
            "bindings": [{"role": scoped, "members": [member]}]
        }, member, scoped))
        for forbidden in (
            {"role": "roles/editor", "members": [member]},
            {"role": scoped, "members": [member], "condition": {"expression": "true"}},
        ):
            with self.subTest(forbidden=forbidden), self.assertRaises(ValueError):
                prod.check_project_grants({"bindings": [forbidden]}, member, scoped)

    def test_existing_unexpected_production_provider_stops_before_any_write(self):
        calls = []
        def gcloud(*args, missing=False):
            calls.append(args)
            if args[:3] == ("iam", "roles", "describe"):
                return None
            if args[:3] == ("iam", "service-accounts", "describe"):
                return None
            if args[:3] == ("iam", "workload-identity-pools", "describe"):
                return {"state": "ACTIVE"}
            if args[:4] == ("iam", "workload-identity-pools", "providers", "describe"):
                return {"state": "ACTIVE", "oidc": {"issuerUri": "https://" + prod.ISSUER},
                        "attributeMapping": prod.MAPPING, "attributeCondition": "unreviewed"}
            raise AssertionError("unexpected write or read " + repr(args))
        with patch.object(prod, "gcloud", side_effect=gcloud), self.assertRaises(ValueError):
            prod.bootstrap("12345", {"plan": [], "apply": []})
        self.assertFalse(any("create" in args or "update" in args for args in calls))

    def test_existing_role_permission_mismatch_stops_before_mutation(self):
        calls = []
        def gcloud(*args, missing=False):
            calls.append(args)
            if args[:3] == ("iam", "roles", "describe"):
                return {"stage": "GA", "includedPermissions": ["pubsub.topics.delete"]}
            raise AssertionError("unexpected read/write " + repr(args))
        with patch.object(prod, "gcloud", side_effect=gcloud), self.assertRaises(ValueError):
            prod.bootstrap("12345", {"plan": ["resourcemanager.projects.get"], "apply": []})
        self.assertEqual(len(calls), 1)

    def test_existing_identity_drift_stops_before_any_write(self):
        runtime = "terraform-prod-workload@" + prod.PROJECT + ".iam.gserviceaccount.com"
        normal = "terraform-prod-normal-plan@" + prod.PROJECT + ".iam.gserviceaccount.com"
        provider = {"name": "projects/12345/locations/global/workloadIdentityPools/terraform-normal/providers/github-actions",
                    "state": "ACTIVE", "oidc": {"issuerUri": "https://" + prod.ISSUER},
                    "attributeMapping": prod.MAPPING, "attributeCondition": prod.CONDITION}
        for defect in ("runtime-key", "runtime-disabled", "plan-disabled", "apply-disabled",
                       "extra-provider", "deleted-provider", "pool-disabled"):
            calls = []
            def gcloud(*args, missing=False):
                calls.append(args)
                if args[:3] == ("iam", "roles", "describe"):
                    return None
                if args[:3] == ("iam", "service-accounts", "describe"):
                    return {"email": args[3], "disabled": (
                        args[3] == runtime and defect == "runtime-disabled"
                        or args[3] == normal and defect == "plan-disabled"
                        or "normal-apply@" in args[3] and defect == "apply-disabled")}
                if args[:3] == ("iam", "service-accounts", "keys"):
                    return [{"keyType": "USER_MANAGED"}] if args[4] == "--iam-account=" + runtime and defect == "runtime-key" else []
                if args[:3] == ("iam", "service-accounts", "get-iam-policy"):
                    return {"bindings": []}
                if args[:3] == ("iam", "workload-identity-pools", "describe"):
                    return {"state": "ACTIVE", "disabled": defect == "pool-disabled"}
                if args[:4] == ("iam", "workload-identity-pools", "providers", "describe"):
                    return provider
                if args[:4] == ("iam", "workload-identity-pools", "providers", "list"):
                    self.assertIn("--show-deleted", args)
                    return [provider, provider | {"name": "other", "state": "DELETED" if defect == "deleted-provider" else "ACTIVE"}] if defect.endswith("provider") else [provider]
                if args[:2] == ("projects", "get-iam-policy"):
                    return {"bindings": []}
                raise AssertionError("unexpected mutation " + repr(args))
            with self.subTest(defect=defect), patch.object(prod, "gcloud", side_effect=gcloud), self.assertRaises(ValueError):
                prod.bootstrap("12345", {"plan": [], "apply": []})
            self.assertFalse(any("create" in a or "add-iam-policy-binding" in a for a in calls))

    def test_account_readback_rejects_disabled_and_wrong_identity(self):
        email = "terraform-prod-normal-plan@" + prod.PROJECT + ".iam.gserviceaccount.com"
        prod.verify_account({"email": email}, email)
        prod.verify_account({"email": email, "disabled": False}, email)
        for account in ({"email": email, "disabled": True}, {"email": "other"},
                        {"email": email, "disabled": "false"}):
            with self.subTest(account=account), self.assertRaises(ValueError):
                prod.verify_account(account, email)

    def test_effective_grants_reject_resource_inherited_group_and_partial_results(self):
        email = "terraform-prod-normal-plan@" + prod.PROJECT + ".iam.gserviceaccount.com"
        member = "serviceAccount:" + email
        role = "projects/" + prod.PROJECT + "/roles/terraformNormalPlan"
        resource = "//cloudresourcemanager.googleapis.com/projects/" + prod.PROJECT
        result = {"fullyExplored": True, "attachedResourceFullName": resource,
                  "iamBinding": {"role": role, "members": [member]}}
        response = {"fullyExplored": True, "mainAnalysis": {"fullyExplored": True,
                    "analysisQuery": {"scope": "projects/" + prod.PROJECT,
                                      "identitySelector": {"identity": member},
                                      "options": {"expandGroups": True}},
                    "analysisResults": [result]}}
        approved = prod.allowed_account_grants("12345", "plan")
        prod.verify_effective_grants(response, member, "projects/" + prod.PROJECT, approved, {role})
        for change in (
            {"iamBinding": {"role": "roles/secretmanager.secretAccessor", "members": [member]},
             "attachedResourceFullName": "//secretmanager.googleapis.com/projects/12345/secrets/private"},
            {"iamBinding": {"role": "roles/owner", "members": [member]},
             "attachedResourceFullName": "//cloudresourcemanager.googleapis.com/folders/456"},
            {"iamBinding": {"role": role, "members": ["group:operators@example.com"]}},
            {"iamBinding": {"role": role, "members": [member], "condition": {"expression": "false"}}},
            {"fullyExplored": False},
        ):
            changed = deepcopy(response)
            changed["mainAnalysis"]["analysisResults"] = [result | change]
            with self.subTest(change=change), self.assertRaises(ValueError):
                prod.verify_effective_grants(changed, member, "projects/" + prod.PROJECT, approved, {role})
        for change in ({"fullyExplored": False}, {"analysisResults": []},
                       {"nonCriticalErrors": [{"code": "PERMISSION_DENIED"}]}):
            changed = deepcopy(response)
            changed["mainAnalysis"].update(change)
            with self.subTest(change=change), self.assertRaises(ValueError):
                prod.verify_effective_grants(changed, member, "projects/" + prod.PROJECT, approved, {role})

    def test_ancestor_inventory_uses_highest_scope_before_any_write(self):
        calls = []
        def gcloud(*args, missing=False):
            calls.append(args)
            if args[:2] == ("projects", "get-ancestors"):
                return [{"type": "project", "id": prod.PROJECT},
                        {"type": "folder", "id": "456"}, {"type": "organization", "id": "789"}]
            if args[:2] == ("projects", "get-iam-policy"):
                return {"bindings": []}
            if args[:3] == ("iam", "service-accounts", "describe"):
                if "normal-plan@" in args[3]:
                    return {"email": args[3]}
                return None
            if args[:3] == ("iam", "service-accounts", "keys"):
                return []
            if args[:3] == ("iam", "service-accounts", "get-iam-policy"):
                return {"bindings": []}
            if "describe" in args:
                return None
            raise AssertionError("unexpected mutation " + repr(args))
        def run(args, **kwargs):
            self.assertIn("--organization=789", args)
            self.assertNotIn("--project=" + prod.PROJECT, args)
            member = next(x.split("=", 1)[1] for x in args if x.startswith("--identity="))
            return {"fullyExplored": True, "mainAnalysis": {"fullyExplored": True,
                    "analysisQuery": {"scope": "organizations/789", "identitySelector": {"identity": member},
                                      "options": {"expandGroups": True}},
                    "analysisResults": [{"fullyExplored": True,
                        "attachedResourceFullName": "//secretmanager.googleapis.com/projects/12345/secrets/private",
                        "iamBinding": {"role": "roles/secretmanager.secretAccessor", "members": [member]}}]}}
        with patch.object(prod, "gcloud", side_effect=gcloud), \
             patch.object(prod.dev, "run", side_effect=run), self.assertRaises(ValueError):
            prod.bootstrap("12345", {"plan": [], "apply": []})
        self.assertFalse(any("create" in a or "add-iam-policy-binding" in a for a in calls))

    def test_new_account_readback_retries_only_absence_and_is_bounded(self):
        email = "terraform-prod-normal-plan@" + prod.PROJECT + ".iam.gserviceaccount.com"
        with patch.object(prod, "gcloud", side_effect=[None, None, {"email": email}]) as read, \
             patch.object(prod.time, "sleep") as sleep:
            prod.wait_for_account(email)
            self.assertEqual(read.call_count, 3)
            self.assertEqual(sleep.call_count, 2)
        for result in (RuntimeError("permission denied"), {"email": email, "disabled": True}):
            with self.subTest(result=result), patch.object(prod, "gcloud", side_effect=[result]) as read, \
                 patch.object(prod.time, "sleep") as sleep, self.assertRaises((RuntimeError, ValueError)):
                prod.wait_for_account(email)
            self.assertEqual(read.call_count, 1)
            sleep.assert_not_called()
        with patch.object(prod, "gcloud", return_value=None) as read, \
             patch.object(prod.time, "sleep") as sleep, self.assertRaises(ValueError):
            prod.wait_for_account(email)
        self.assertEqual(read.call_count, 12)
        self.assertEqual(sleep.call_count, 11)

    def test_backend_policies_include_inline_and_attached_and_exact_prod_scope(self):
        for kind in ("plan", "apply"):
            expected = prod.backend.permissions_policy("fixture-state-bucket", kind)
            prod.verify_backend_permissions([expected], [], "fixture-state-bucket", kind)
            prod.verify_backend_permissions([], [expected], "fixture-state-bucket", kind)
            for invalid in (
                prod.backend.permissions_policy("another-bucket", kind),
                prod.backend.permissions_policy("fixture-state-bucket", "apply" if kind == "plan" else "plan"),
                {"Version": "2012-10-17", "Statement": [{"Effect": "Allow", "Action": "s3:*", "Resource": "*"}]},
            ):
                with self.subTest(kind=kind, invalid=invalid), self.assertRaises(ValueError):
                    prod.verify_backend_permissions([invalid], [], "fixture-state-bucket", kind)
            for field in ("Resource", "Condition"):
                altered = deepcopy(expected)
                if field == "Resource":
                    altered["Statement"][0][field] = "arn:aws:s3:::fixture-state-bucket/youtube-study-space/dev/terraform.tfstate"
                else:
                    altered["Statement"][2].pop(field)
                with self.assertRaises(ValueError):
                    prod.verify_backend_permissions([altered], [], "fixture-state-bucket", kind)
            with self.assertRaises(ValueError):
                prod.verify_backend_permissions([expected], [expected], "fixture-state-bucket", kind)

    def test_string_equals_subject_preserves_native_actor_and_main(self):
        document = aws_policy("plan")
        condition = document["Statement"][0]["Condition"]
        condition["StringEquals"].update(condition.pop("StringLike"))
        prod.verify_role(document, "123456789012", "plan")
        condition["StringEquals"].pop("token.actions.githubusercontent.com:actor_id")
        with self.assertRaises(ValueError):
            prod.verify_role(document, "123456789012", "plan")

    def test_private_backend_configuration_does_not_invent_existing_role_names(self):
        value = {"state_account_id": "123456789012", "state_bucket": "fixture-state-bucket",
                 "production_plan_role_name": "existing-prod-plan", "production_apply_role_name": "existing-prod-apply"}
        with patch.object(prod.backend, "private_file", return_value=value):
            self.assertEqual(prod.load_backend(Path("/private/config.json")), value)
        for changed in ({"production_plan_role_name": "ProdRead"},
                        {"production_apply_role_name": "ProdWrite"},
                        {"production_plan_role_name": "existing-prod-apply",
                         "production_apply_role_name": "existing-prod-plan"}):
            with self.subTest(changed=changed), patch.object(prod.backend, "private_file", return_value=value | changed), self.assertRaises(ValueError):
                prod.load_backend(Path("/private/config.json"))
        calls = []
        def aws(*args):
            calls.append(args)
            if args[:2] == ("sts", "get-caller-identity"):
                return {"Account": value["state_account_id"]}
            kind = "apply" if "existing-prod-apply" in args else "plan"
            if args[1] == "get-role":
                return {"Role": {"Arn": "arn:aws:iam::123456789012:role/" + value[f"production_{kind}_role_name"],
                                 "AssumeRolePolicyDocument": aws_policy(kind)}}
            if args[1] == "list-role-policies":
                return {"PolicyNames": ["backend"]}
            if args[1] == "list-attached-role-policies":
                return {"AttachedPolicies": []}
            if args[1] == "get-role-policy":
                return {"PolicyDocument": prod.backend.permissions_policy(value["state_bucket"], kind)}
            raise AssertionError(args)
        with patch.object(prod, "aws", side_effect=aws):
            self.assertEqual(prod.preflight_cloud(value), "123456789012")
        self.assertTrue(any("existing-prod-plan" in args for args in calls))
        self.assertTrue(any("existing-prod-apply" in args for args in calls))

    def test_main_noop_evidence_rejects_stale_rerun_migration_and_failed_runs(self):
        run = {"id": 123, "head_sha": "a" * 40, "run_attempt": 1, "head_branch": "main",
               "event": "workflow_dispatch", "status": "completed", "conclusion": "success",
               "path": ".github/workflows/ci.yml", "repository": {"id": 340900071},
               "head_repository": {"id": 340900071}, "actor": {"id": 54093651},
               "triggering_actor": {"id": 54093651}}
        job = {"id": 456, "run_id": 123, "head_sha": "a" * 40,
               "name": "GCP Terraform / Terraform Plan (prod)", "conclusion": "success"}
        marker = "GCP_PROD_INITIAL_NOOP: PASS; source=" + "a" * 40 + "; run=123; managed=30; state_unchanged=true"
        current = "a" * 40
        def gh(endpoint):
            if endpoint.endswith("/branches/main"):
                return {"commit": {"sha": current}}
            if endpoint.endswith("/jobs?per_page=100"):
                return {"total_count": 1, "jobs": [job]}
            return run
        with patch.object(prod, "gh", side_effect=gh), patch.object(prod.subprocess, "run",
                return_value=subprocess.CompletedProcess([], 0, "timestamp " + marker + "\n", "")):
            self.assertEqual(prod.verify_initial_noop("123"), "a" * 40)
            for change in ({"run_attempt": 2}, {"head_branch": "dev"}, {"conclusion": "failure"},
                           {"actor": {"id": 42}}, {"head_repository": {"id": 42}}):
                old = dict(run)
                run.update(change)
                with self.subTest(change=change), self.assertRaises(ValueError):
                    prod.verify_initial_noop("123")
                run.clear(); run.update(old)
            current = "b" * 40
            with self.assertRaises(ValueError):
                prod.verify_initial_noop("123")
            current = "a" * 40
            job["name"] = "GCP Terraform / Terraform Existing Resources Plan (prod)"
            with self.assertRaises(ValueError):
                prod.verify_initial_noop("123")
        job["name"] = "GCP Terraform / Terraform Plan (prod)"
        with patch.object(prod, "gh", side_effect=gh), patch.object(prod.subprocess, "run",
                return_value=subprocess.CompletedProcess([], 0, "no proof\n", "")), self.assertRaises(ValueError):
            prod.verify_initial_noop("123")

    def test_check_mode_reads_existing_gcp_but_does_not_mutate(self):
        args = ["script", "--check", "--backend-config", "/private/backend.json"]
        permitted = {"plan": ["resourcemanager.projects.get"],
                     "apply": ["resourcemanager.projects.get", "pubsub.topics.create"]}
        with patch.object(sys, "argv", args), \
             patch.object(prod, "load_backend", return_value={}), \
             patch.object(prod, "preflight_cloud"), patch.object(prod, "preflight_github"), \
             patch.object(prod, "permissions", return_value=("12345", permitted)), \
             patch.object(prod, "bootstrap") as bootstrap:
            prod.main()
            bootstrap.assert_called_once_with("12345", permitted, enable_apply=False, check=True)
        with patch.object(sys, "argv", args + ["--enable-apply"]), self.assertRaises(ValueError):
            prod.main()

    def test_plan_bootstrap_never_creates_or_grants_apply_identity(self):
        calls, roles, accounts, policies = [], {}, {}, {}
        project_policy = {"bindings": []}
        provider = None
        def gcloud(*args, missing=False):
            nonlocal provider
            calls.append(args)
            if args[:2] == ("iam", "roles"):
                if args[2] == "create":
                    roles[args[3]] = {"stage": "GA", "includedPermissions": []}
                    return {}
                return roles.get(args[3])
            if args[:2] == ("projects", "get-iam-policy"):
                return project_policy
            if args[:2] == ("projects", "add-iam-policy-binding"):
                project_policy["bindings"].append({"role": args[4].split("=", 1)[1],
                                                   "members": [args[3].split("=", 1)[1]]})
                return {}
            if args[:2] == ("iam", "service-accounts"):
                action = args[2]
                if action == "describe":
                    return accounts.get(args[3])
                if action == "create":
                    email = args[3] + "@" + prod.PROJECT + ".iam.gserviceaccount.com"
                    accounts[email] = {"email": email}
                    return {}
                if action == "keys":
                    return []
                if action == "get-iam-policy":
                    return policies.get(args[3], {"bindings": []})
                if action == "add-iam-policy-binding":
                    policies[args[3]] = {"bindings": [{"role": args[5].split("=", 1)[1],
                                                       "members": [args[4].split("=", 1)[1]]}]}
                    return {}
            if args[:3] == ("iam", "workload-identity-pools", "providers"):
                if args[3] == "list":
                    return [provider] if provider else []
                if args[3] == "create-oidc":
                    provider = {"name": "projects/12345/locations/global/workloadIdentityPools/terraform-normal/providers/github-actions",
                                "state": "ACTIVE", "oidc": {"issuerUri": "https://" + prod.ISSUER},
                                "attributeMapping": prod.MAPPING, "attributeCondition": prod.CONDITION}
                return provider
            if args[:2] == ("iam", "workload-identity-pools"):
                return None
            raise AssertionError(args)
        def gh(endpoint):
            return {"secrets": [{"name": key} for key in ("GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT", "GCP_TERRAFORM_NORMAL_WIF_PROVIDER")]}
        with patch.object(prod, "gcloud", side_effect=gcloud), patch.object(prod, "gh", side_effect=gh), \
             patch.object(prod, "policy_analysis_scope", return_value=("project", prod.PROJECT)), \
             patch.object(prod, "audit_effective_grants"), \
             patch.object(prod.dev, "run") as run:
            prod.bootstrap("12345", {"plan": [], "apply": []})
            self.assertEqual(len(run.call_args_list), 2)
            self.assertTrue(all("terraform-prod-plan" in call.args[0] for call in run.call_args_list))
        self.assertEqual(set(accounts), {"terraform-prod-normal-plan@" + prod.PROJECT + ".iam.gserviceaccount.com"})
        self.assertEqual(set(roles), {"terraformNormalPlan"})
        self.assertFalse(any("terraform-prod-normal-apply" == a or "terraform-prod-workload" == a for call in calls for a in call))
        # A fresh identity that is not fully indexed must remain untrusted.
        calls.clear(); roles.clear(); accounts.clear(); policies.clear()
        project_policy["bindings"].clear()
        provider = None
        with patch.object(prod, "gcloud", side_effect=gcloud), \
             patch.object(prod, "policy_analysis_scope", return_value=("project", prod.PROJECT)), \
             patch.object(prod, "audit_effective_grants", side_effect=ValueError("incomplete inventory")), \
             patch.object(prod.dev, "run") as run, self.assertRaises(ValueError):
            prod.bootstrap("12345", {"plan": [], "apply": []})
        run.assert_not_called()
        self.assertEqual(project_policy["bindings"], [])
        self.assertEqual(policies, {})


if __name__ == "__main__":
    unittest.main()
