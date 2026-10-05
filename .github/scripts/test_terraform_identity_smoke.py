#!/usr/bin/env python3
import contextlib
import importlib.util
import io
import subprocess
import unittest
from unittest.mock import patch
from pathlib import Path

spec = importlib.util.spec_from_file_location("smoke", Path(__file__).with_name("terraform_identity_smoke.py"))
smoke = importlib.util.module_from_spec(spec)
spec.loader.exec_module(smoke)


class IdentitySmokeTest(unittest.TestCase):
    def test_state_integrity_summary_exposes_only_counts(self):
        state = {"version": 4, "serial": 9, "lineage": "PRIVATE_LINEAGE", "outputs": {"private": "PRIVATE_VALUE"},
                 "resources": [{"mode": "managed", "instances": [{"labels": "PRIVATE_EMAIL"}]},
                               {"mode": "data", "instances": [{}]}]}
        result = smoke.state_counts(state)
        self.assertEqual(result, "AWS current state resource count 1, serial 9, output count 1")
        self.assertNotIn("PRIVATE", result)
        for malformed in [{}, {"version": 4, "serial": "PRIVATE_VALUE"}]:
            with self.assertRaises(smoke.SmokeFailure): smoke.state_counts(malformed)

    def test_dependency_failures_are_not_treated_as_permission_denial(self):
        for error in ("(NoSuchKey)", "(404)", "(PreconditionFailed)", "timeout PRIVATE_SENTINEL"):
            with self.subTest(error=error), self.assertRaises(smoke.SmokeFailure):
                smoke.denied_aws(subprocess.CompletedProcess([], 1, "", error), "denied-operation")

    def test_only_explicit_aws_denials_pass(self):
        for error in ("(AccessDenied) PRIVATE_SENTINEL", "(403) Forbidden PRIVATE_SENTINEL"):
            smoke.denied_aws(subprocess.CompletedProcess([], 1, "", error), "denied-operation")
        with self.assertRaises(smoke.SmokeFailure):
            smoke.denied_aws(subprocess.CompletedProcess([], 0, "", "(AccessDenied)"), "denied-operation")

    def run_gcp(self, *, permissions=None, prod_permissions=None, impersonation_status=403):
        calls = []
        def request(path, token, body=None, **kwargs):
            calls.append(path)
            if "generateAccessToken" in path:
                return impersonation_status, {"error": {"status": "PERMISSION_DENIED", "message": "PRIVATE_SENTINEL"}}
            if path == "v3/projects/youtube-study-space:testIamPermissions":
                return 200, {"permissions": prod_permissions or []}
            if path.endswith(":testIamPermissions"):
                return 200, {"permissions": permissions or []}
            return 200, {"projectId": "development-fixture", "state": "ACTIVE"}
        result = smoke.verify_google("PRIVATE_TOKEN", "terraform-dev-plan@development-fixture.iam.gserviceaccount.com", request, project="development-fixture")
        return result, calls

    def test_empty_graph_smoke_performs_read_and_denied_impersonation(self):
        result, calls = self.run_gcp()
        self.assertEqual(len(result), 4)
        self.assertEqual(calls[0], "v3/projects/development-fixture")
        self.assertIn("terraform-dev-apply@", calls[-1])
        self.assertNotIn("PRIVATE", " ".join(result))

    def test_unexpected_mutation_or_production_grant_stops_smoke(self):
        for overrides in ({"permissions": ["iam.serviceAccounts.create"]}, {"prod_permissions": ["resourcemanager.projects.get"]}, {"impersonation_status": 200}, {"impersonation_status": 404}):
            with self.subTest(overrides=overrides), self.assertRaises(smoke.SmokeFailure) as caught:
                self.run_gcp(**overrides)
            self.assertNotIn("PRIVATE", str(caught.exception))

    def test_unexpected_dependency_error_does_not_leak_publicly(self):
        output = io.StringIO()
        with patch.object(smoke, "verify_oidc", side_effect=RuntimeError("PRIVATE_TOKEN_AND_IDENTIFIER")), contextlib.redirect_stderr(output):
            self.assertEqual(smoke.main([]), 1)
        self.assertNotIn("PRIVATE", output.getvalue())
        self.assertIn("raw error suppressed", output.getvalue())

    def quota_request(self, identity, *, extra=(), missing=(), prod=()):
        def request(path, token, body=None, **kwargs):
            self.assertIn("monitoring.alertPolicies.list", body["permissions"])
            self.assertIn("serviceusage.services.enable", body["permissions"])
            if "youtube-study-space:" in path:
                self.assertIn("bigquery.datasets.get", body["permissions"])
                return 200, {"permissions": list(prod)}
            allowed = {"monitoring.alertPolicies.get"}
            if identity == "apply": allowed.add("monitoring.alertPolicies.create")
            return 200, {"permissions": sorted((allowed - set(missing)) | set(extra))}
        return request

    def test_quota_plan_get_only_and_apply_get_create_are_required(self):
        for identity in ("plan", "apply"):
            result = smoke.verify_quota_google("PRIVATE_TOKEN", f"terraform-dev-{identity}@fixture.iam.gserviceaccount.com",
                self.quota_request(identity), identity=identity, project="fixture")
            self.assertEqual(len(result), 3)
            self.assertNotIn("PRIVATE", " ".join(result))
            for missing in (("monitoring.alertPolicies.get",), ("monitoring.alertPolicies.create",)) if identity == "apply" else (("monitoring.alertPolicies.get",),):
                with self.assertRaises(smoke.SmokeFailure):
                    smoke.verify_quota_google("PRIVATE_TOKEN", f"terraform-dev-{identity}@fixture.iam.gserviceaccount.com",
                        self.quota_request(identity, missing=missing), identity=identity, project="fixture")

    def test_quota_extra_mutation_list_data_iam_api_and_production_grants_stop(self):
        for identity in ("plan", "apply"):
            extras = ["monitoring.alertPolicies.update", "monitoring.alertPolicies.delete", "monitoring.alertPolicies.list",
                      "storage.objects.get", "bigquery.jobs.create", "resourcemanager.projects.setIamPolicy", "serviceusage.services.enable"]
            if identity == "plan": extras.append("monitoring.alertPolicies.create")
            for permission in extras:
                with self.subTest(identity=identity, permission=permission), self.assertRaises(smoke.SmokeFailure):
                    smoke.verify_quota_google("PRIVATE_TOKEN", f"terraform-dev-{identity}@fixture.iam.gserviceaccount.com",
                        self.quota_request(identity, extra=(permission,)), identity=identity, project="fixture")
            with self.assertRaises(smoke.SmokeFailure):
                smoke.verify_quota_google("PRIVATE_TOKEN", f"terraform-dev-{identity}@fixture.iam.gserviceaccount.com",
                    self.quota_request(identity, prod=("monitoring.alertPolicies.get",)), identity=identity, project="fixture")

    def test_quota_apply_uses_only_the_existing_development_apply_identity(self):
        for account in ("terraform-dev-plan@fixture.iam.gserviceaccount.com", "other@fixture.iam.gserviceaccount.com"):
            with self.assertRaises(smoke.SmokeFailure):
                smoke.verify_quota_google("PRIVATE_TOKEN", account, self.quota_request("apply"), identity="apply", project="fixture")

    def test_bigquery_metadata_import_rejects_data_query_mutation_and_prod_read(self):
        def request(path, token, body=None, **kwargs):
            if path.endswith(":testIamPermissions"):
                requested = body["permissions"]
                if "youtube-study-space:" in path:
                    self.assertIn("bigquery.datasets.get", requested)
                    self.assertIn("bigquery.tables.get", requested)
                    return 200, {"permissions": []}
                self.assertIn("bigquery.tables.getData", requested)
                self.assertIn("bigquery.jobs.create", requested)
                self.assertIn("bigquery.tables.update", requested)
                self.assertIn("bigquery.datasets.delete", requested)
                return 200, {"permissions": []}
            if "generateAccessToken" in path:
                return 403, {"error": {"status": "PERMISSION_DENIED"}}
            return 200, {"projectId": "development-fixture", "state": "ACTIVE"}
        smoke.verify_google("PRIVATE_TOKEN", "terraform-dev-plan@development-fixture.iam.gserviceaccount.com", request, project="development-fixture")
        for permission in ("bigquery.tables.getData", "bigquery.jobs.create", "bigquery.tables.update"):
            with self.subTest(permission=permission), self.assertRaises(smoke.SmokeFailure):
                self.run_gcp(permissions=[permission])
        with self.assertRaises(smoke.SmokeFailure):
            self.run_gcp(prod_permissions=["bigquery.datasets.get"])


if __name__ == "__main__":
    unittest.main()
