#!/usr/bin/env python3
import contextlib
import base64
import io
import json
import os
import re
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path

import terraform_identity_smoke as smoke
import terraform_history_plan_receipt as receipt
import test_terraform_history_plan_receipt as receipt_fixtures


class IdentitySmokeTest(unittest.TestCase):
    def test_runtime_apply_composed_workflow_env_retains_exact_backend_role(self):
        workflow = (Path(__file__).parents[1] / 'workflows/gcp-terraform-authenticated.yml').read_text()
        job = workflow.split('  apply:\n', 1)[1].split('  security-probe:\n', 1)[0]
        job_env = job.split('    env:\n', 1)[1].split('    steps:\n', 1)[0]
        step = job.split('      - name: Verify Function identity exact read permissions\n', 1)[1].split('      - name:', 1)[0]
        self.assertIn('terraform_identity_smoke.py apply-read-only', step)
        values = dict(re.findall(r'^\s+(\w+): (.+)$', job_env + '\n' + step.split('        env:\n', 1)[1].split('        run:', 1)[0], re.M))
        substitutions = {'${{ inputs.mode }}': 'apply', '${{ needs.preflight.outputs.ownership_wave }}': 'pool',
            '${{ needs.preflight.outputs.manage_user_activity_history }}': 'true',
            '${{ secrets.AWS_TERRAFORM_STATE_ACCOUNT_ID }}': '111111111111',
            '${{ secrets.AWS_TERRAFORM_STATE_BUCKET }}': 'dummy-state-bucket',
            '${{ needs.preflight.outputs.state_key }}': receipt.STATE_KEY,
            '${{ secrets.AWS_TERRAFORM_BACKEND_ROLE_ID }}': 'DUMMY_EXPECTED'}
        keys = ('MODE', 'OWNERSHIP_WAVE', 'TF_VAR_manage_user_activity_history', 'STATE_ACCOUNT_ID',
                'STATE_BUCKET', 'STATE_KEY', 'BACKEND_ROLE_ID')
        env = {key: substitutions[values[key]] for key in keys}
        answer = subprocess.CompletedProcess([], 0, json.dumps(dict(Account='111111111111', UserId='DUMMY_EXPECTED:s')), '')
        with patch.object(smoke, 'aws', return_value=answer) as aws, \
             patch('runtime_ownership_provenance.load_inputs', return_value=(None, None, None)) as inputs:
            self.assertIn('Runtime state verification follows fresh private metadata', smoke.verify_aws(env, apply_read_only=True))
            aws.assert_called_once_with('sts', 'get-caller-identity')
            inputs.assert_called_once_with(env)
        for bad in ('DUMMY_WRONG', ''):
            with self.subTest(role=bad), patch.object(smoke, 'aws', return_value=answer), \
                 patch('runtime_ownership_provenance.load_inputs') as inputs, self.assertRaises(smoke.StageFailure):
                smoke.verify_aws(env | {'BACKEND_ROLE_ID': bad}, apply_read_only=True)
            inputs.assert_not_called()

    def test_early_backend_role_requires_exact_identity_in_plan_and_apply_without_gcp_or_s3(self):
        for mode in ("plan", "apply"):
            env = {"MODE": mode, "HISTORY_TARGET": "dev", "TF_VAR_project_id": "test-youtube-study-space",
                   "TF_VAR_manage_user_activity_history": "true", "STATE_ACCOUNT_ID": "111111111111",
                   "BACKEND_ROLE_ID": "DUMMY_EXPECTED"}
            cases = [("111111111111", "DUMMY_EXPECTED:session", 0, 0),
                     ("111111111111", "DUMMY_WRONG:session", 0, 1),
                     ("222222222222", "DUMMY_EXPECTED:session", 0, 1),
                     ("111111111111", None, 0, 1), ("111111111111", "DUMMY_EXPECTED", 0, 1),
                     ("111111111111", "DUMMY_EXPECTED:session", 1, 1)]
            for account, user_id, code, expected in cases:
                result = subprocess.CompletedProcess([], code, json.dumps({"Account": account, "UserId": user_id}), "")
                out, err = io.StringIO(), io.StringIO()
                with self.subTest(mode=mode, user_id=user_id, code=code), patch.dict(os.environ, env, clear=True), \
                     patch.object(smoke, "aws", return_value=result) as aws, \
                     patch.object(smoke.urllib.request, "urlopen") as http, \
                     contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                    self.assertEqual(smoke.main(["backend-role-read-only"]), expected)
                    aws.assert_called_once_with("sts", "get-caller-identity")
                    http.assert_not_called()
                    self.assertNotIn("DUMMY", out.getvalue() + err.getvalue())

    def test_early_backend_role_wrong_route_stops_without_credentials_or_requests(self):
        with patch.dict(os.environ, {"MODE": "security-probe"}, clear=True), \
             patch.object(smoke, "aws") as aws, contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(smoke.main(["backend-role-read-only"]), 1)
            aws.assert_not_called()

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
            self.assertEqual(smoke.main(["plan-read-only"]), 1)
        self.assertNotIn("PRIVATE", output.getvalue())
        self.assertIn("stage=oidc; category=dependency-error", output.getvalue())

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

    def test_topic_exact_get_only_and_later_wave_production_grants_stop(self):
        def request_for(grants, prod=()):
            def request(path, token, body=None, **kwargs):
                return 200, {"permissions": list(prod if "youtube-study-space:" in path else grants)}
            return request
        for kind in ("plan", "apply"):
            account = f"terraform-dev-{kind}@fixture.iam.gserviceaccount.com"
            result = smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(["pubsub.topics.get"]), project="fixture")
            self.assertEqual(len(result), 4)
            self.assertNotIn("PRIVATE", str(result))
            for extra in smoke.EXPORT_TOPIC_PERMISSIONS[1:]:
                with self.subTest(kind=kind, extra=extra), self.assertRaises(smoke.SmokeFailure):
                    smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(["pubsub.topics.get", extra]), project="fixture")
            for grants, prod in [([], []), (["pubsub.topics.get"], ["pubsub.topics.get"])]:
                with self.assertRaises(smoke.SmokeFailure):
                    smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(grants, prod), project="fixture")

    def test_scheduler_get_only_rejects_execution_mutation_list_publish_and_function(self):
        def request_for(grants, prod=()):
            def request(path, token, body=None, **kwargs):
                self.assertIn("cloudscheduler.jobs.enable", body["permissions"])
                self.assertIn("cloudscheduler.jobs.run", body["permissions"])
                return 200, {"permissions": list(prod if "youtube-study-space:" in path else grants)}
            return request
        allowed = {"pubsub.topics.get", "cloudscheduler.jobs.get"}
        for kind in ("plan", "apply"):
            account = f"terraform-dev-{kind}@fixture.iam.gserviceaccount.com"
            result = smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(allowed), project="fixture", scheduler=True)
            self.assertEqual(len(result), 4)
            self.assertNotIn("PRIVATE", str(result))
            for extra in ("cloudscheduler.jobs.list", "cloudscheduler.jobs.create", "cloudscheduler.jobs.update",
                    "cloudscheduler.jobs.delete", "cloudscheduler.jobs.run", "cloudscheduler.jobs.pause", "cloudscheduler.jobs.enable",
                    "cloudscheduler.jobs.fullView", "pubsub.topics.publish", "cloudfunctions.functions.get"):
                with self.subTest(kind=kind, extra=extra), self.assertRaises(smoke.SmokeFailure):
                    smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(allowed | {extra}), project="fixture", scheduler=True)
            for grants, prod in [(allowed - {"cloudscheduler.jobs.get"}, set()), (allowed, {"cloudscheduler.jobs.get"})]:
                with self.assertRaises(smoke.SmokeFailure):
                    smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(grants, prod), project="fixture", scheduler=True)


    def test_function_get_only_rejects_source_execution_mutation_and_production(self):
        allowed = {"pubsub.topics.get", "cloudscheduler.jobs.get", "cloudfunctions.functions.get"}
        def request_for(grants, prod=()):
            def request(path, token, body=None, **kwargs):
                self.assertTrue(set(smoke.FUNCTION_FORBIDDEN_PERMISSIONS) <= set(body["permissions"]))
                return 200, {"permissions": list(prod if "youtube-study-space:" in path else grants)}
            return request
        for kind in ("plan", "apply"):
            account = f"terraform-dev-{kind}@fixture.iam.gserviceaccount.com"
            result = smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(allowed), project="fixture", scheduler=True, function=True)
            self.assertNotIn("PRIVATE", str(result))
            for extra in (*smoke.FUNCTION_FORBIDDEN_PERMISSIONS, "pubsub.topics.publish", "cloudscheduler.jobs.run"):
                with self.subTest(kind=kind, extra=extra), self.assertRaises(smoke.SmokeFailure):
                    smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(allowed | {extra}), project="fixture", scheduler=True, function=True)
            for grants, prod in [(allowed - {"cloudfunctions.functions.get"}, set()), (allowed, {"cloudfunctions.functions.get"})]:
                with self.assertRaises(smoke.SmokeFailure):
                    smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(grants, prod), project="fixture", scheduler=True, function=True)
            with self.assertRaises(smoke.SmokeFailure):
                smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request_for(allowed), project="fixture", function=True)

        calls = []
        def dev_only_request(path, token, body=None, **kwargs):
            calls.append(path)
            return 200, {"permissions": sorted(allowed)}
        labels = smoke.verify_export_topic_google("PRIVATE_TOKEN", "terraform-dev-apply@fixture.iam.gserviceaccount.com",
            dev_only_request, project="fixture", scheduler=True, function=True, dev_only=True)
        self.assertEqual(calls, ["v3/projects/fixture:testIamPermissions"])
        self.assertFalse(any("production" in label for label in labels))

    def test_function_execution_input_comes_from_exact_metadata_get_after_mask(self):
        import contextlib
        import io
        import tempfile
        from pathlib import Path
        metadata = {"name": "projects/test-youtube-study-space/locations/asia-southeast2/functions/firestoreCollectionsExport",
            "runtime": "nodejs22", "status": "ACTIVE", "versionId": "8", "serviceAccountEmail": "test-youtube-study-space@appspot.gserviceaccount.com",
            "sourceUploadUrl": "PRIVATE_SOURCE_URL"}
        def request(path, token, body=None, **kwargs):
            self.assertEqual(path, "v1/" + metadata["name"])
            self.assertEqual(kwargs["host"], "cloudfunctions.googleapis.com")
            self.assertIsNone(body)
            return 200, metadata
        with tempfile.TemporaryDirectory() as directory, contextlib.redirect_stdout(io.StringIO()) as output:
            env = {"GITHUB_ENV": directory + "/env"}
            result = smoke.configure_function_execution_identity("PRIVATE_TOKEN", env, request)
            self.assertIn("::add-mask::" + metadata["serviceAccountEmail"], output.getvalue())
            self.assertEqual(Path(env["GITHUB_ENV"]).read_text(), "TF_VAR_export_function_execution_service_account_email=" + metadata["serviceAccountEmail"] + "\n")
            self.assertNotIn("PRIVATE", output.getvalue() + str(result))
            for field, value in [("serviceAccountEmail", "foreign"), ("versionId", "9"), ("runtime", "nodejs20")]:
                old = metadata[field]; metadata[field] = value
                with self.assertRaises(smoke.SmokeFailure): smoke.configure_function_execution_identity("PRIVATE_TOKEN", env, request)
                metadata[field] = old

    def test_apply_function_cli_uses_only_dev_permission_and_function_reads(self):
        project = "test-youtube-study-space"
        with tempfile.TemporaryDirectory() as directory:
            env = {"MODE": "apply", "GCP_SMOKE_ACCESS_TOKEN": "DUMMY_TOKEN",
                   "GCP_SMOKE_SERVICE_ACCOUNT": f"terraform-dev-apply@{project}.iam.gserviceaccount.com",
                   "GITHUB_STEP_SUMMARY": str(Path(directory) / "summary"), "GITHUB_ENV": str(Path(directory) / "env")}
            calls = []
            def verify(*args, **kwargs):
                calls.append(("dev-permissions", kwargs))
                self.assertTrue(kwargs["scheduler"] and kwargs["function"] and kwargs["dev_only"])
                return ["Dev function permissions checked"]
            def function(*args):
                calls.append(("function-get", {}))
                return ["Dev Function metadata checked"]
            with patch.dict(os.environ, env, clear=True), patch.object(smoke, "verify_export_topic_google", side_effect=verify), \
                    patch.object(smoke, "configure_function_execution_identity", side_effect=function), \
                    patch.object(smoke, "aws", side_effect=AssertionError("AWS must not be called")), \
                    contextlib.redirect_stdout(io.StringIO()):
                self.assertEqual(smoke.main(["export-function-apply-read-only"]), 0)
            self.assertEqual([call[0] for call in calls], ["dev-permissions", "function-get"])
            for change in ({"MODE": "security-probe"},
                           {"GCP_SMOKE_SERVICE_ACCOUNT": f"terraform-dev-plan@{project}.iam.gserviceaccount.com"}):
                calls.clear()
                with patch.dict(os.environ, env | change, clear=True), \
                        patch.object(smoke, "verify_export_topic_google", side_effect=verify), \
                        contextlib.redirect_stderr(io.StringIO()):
                    self.assertEqual(smoke.main(["export-function-apply-read-only"]), 1)
                self.assertEqual(calls, [])


class PlanReadOnlySmokeTest(unittest.TestCase):
    """Exercise the CLI entrypoint with synthetic HTTP/CLI dependencies only."""

    def run_smoke(self, args=("plan-read-only",), *, legacy_denials=False, claims_override=None,
                  extra_permissions=(), missing_permission=None, account="PRIVATE_ACCOUNT",
                  state_key="youtube-study-space/dev/terraform.tfstate", service_account=None,
                  fail_read=None, put_error="(AccessDenied) PRIVATE_SENTINEL", change_after_put=None,
                  mode=None, probe_gate=None, post_noop_gate=None, history_enabled=False,
                  expected_role_id="PRIVATE_ROLE_ID",
                  returned_user_id="PRIVATE_ROLE_ID:GitHubActions", sts_returncode=0):
        project = "test-youtube-study-space"
        claims = {
            "repository_id": "340900071", "repository_owner_id": "54093651",
            "environment": "terraform-dev-plan", "ref": "refs/heads/feature/gcp-terraform-iac",
            "workflow_ref": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
            "job_workflow_ref": "kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/feature/gcp-terraform-iac",
            "event_name": "workflow_dispatch",
        }
        claims["sub"] = ":".join(f"{k}:{v.replace(':', '%3A')}" for k, v in claims.items())
        claims.update(workflow_sha="a" * 40, job_workflow_sha="a" * 40)
        claims.update(claims_override or {})
        token = "header." + base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip("=") + ".PRIVATE_SIGNATURE"
        http_calls, aws_calls = [], []
        permission_reads = 0

        def urlopen(request, **kwargs):
            nonlocal permission_reads
            url = request.full_url
            body = json.loads(request.data) if request.data else None
            http_calls.append((request.get_method(), url, body))
            if "fixture.invalid/oidc" in url:
                if fail_read == "oidc": raise RuntimeError("PRIVATE_OIDC_URL_TOKEN")
                data = {"value": token}
            elif url.endswith(f"/v3/projects/{project}"):
                if fail_read == "project": raise RuntimeError("PRIVATE_READ_ERROR")
                data = {"projectId": project, "state": "ACTIVE"}
            elif url.endswith(f"/v3/projects/{project}:testIamPermissions"):
                permission_reads += 1
                if fail_read == "permissions" or (fail_read == "quota" and permission_reads == 2) or \
                        (fail_read == "export" and permission_reads == 3):
                    raise RuntimeError("PRIVATE_GCP_BODY")
                requested = body["permissions"]
                allowed = {"monitoring.alertPolicies.get", "pubsub.topics.get", "cloudscheduler.jobs.get", "cloudfunctions.functions.get"}
                data = {"permissions": sorted((set(requested) & allowed) | set(extra_permissions))}
                if missing_permission in data["permissions"]: data["permissions"].remove(missing_permission)
            elif url.endswith("/functions/firestoreCollectionsExport"):
                if fail_read == "function": raise RuntimeError("PRIVATE_READ_ERROR")
                data = {"name": f"projects/{project}/locations/asia-southeast2/functions/firestoreCollectionsExport",
                        "runtime": "nodejs22", "status": "ACTIVE", "versionId": "8",
                        "serviceAccountEmail": f"{project}@appspot.gserviceaccount.com", "sourceUploadUrl": "PRIVATE_SOURCE_URL"}
            else:
                # These operations would be ALLOWED. A read-only CLI must never call them.
                if legacy_denials:
                    if "generateAccessToken" in url:
                        raise smoke.urllib.error.HTTPError(url, 403, "Forbidden", {}, io.BytesIO(b'{"error":{"status":"PERMISSION_DENIED"}}'))
                    data = {"permissions": []}
                else:
                    data = {"accessToken": "PRIVATE_APPLY_TOKEN", "permissions": ["resourcemanager.projects.get"]}
            response = io.BytesIO(json.dumps(data).encode())
            response.status = 200
            return response

        state_body = [json.dumps(receipt_fixtures.state_fixture()).encode()]
        current_version = ["DUMMY_VERSION_ID"]
        lock_present = [False]

        def aws(*args):
            aws_calls.append(args)
            if args[:2] == ("sts", "get-caller-identity"):
                if fail_read == "sts": raise RuntimeError("PRIVATE_STS_ID")
                caller = {"Account": account}
                if returned_user_id is not None:
                    caller["UserId"] = returned_user_id
                return subprocess.CompletedProcess([], sts_returncode, json.dumps(caller), "")
            if args[:2] == ("s3api", "get-object"):
                if fail_read == "state": return subprocess.CompletedProcess([], 1, "", "PRIVATE_READ_ERROR")
                Path(args[-1]).write_bytes(state_body[0])
                return subprocess.CompletedProcess([], 0, json.dumps({"ContentLength": len(state_body[0]), "VersionId": current_version[0], "ETag": '"dummy-etag"'}), "")
            if args[:2] == ("s3api", "head-object") and args[args.index("--key") + 1] == state_key:
                return subprocess.CompletedProcess([], 0, json.dumps({"ContentLength": len(state_body[0]), "VersionId": current_version[0],
                    "ETag": '"dummy-etag"', "ServerSideEncryption": "AES256", "Metadata": {}}), "")
            if args[:2] == ("s3api", "list-objects-v2") and args[args.index("--prefix") + 1] == state_key + ".tflock":
                return subprocess.CompletedProcess([], 0, json.dumps({"Name": "PRIVATE_BUCKET", "Prefix": state_key + ".tflock",
                    "MaxKeys": 1, "KeyCount": int(lock_present[0]), "IsTruncated": False,
                    **({"Contents": [{"Key": state_key + ".tflock", "Size": 1}]} if lock_present[0] else {})}), "")
            if args[:2] == ("s3api", "put-object"):
                if put_error == "raise": raise RuntimeError("PRIVATE_TIMEOUT_TOKEN")
                if change_after_put == "version": current_version[0] = "DUMMY_NEW_VERSION_ID"
                if change_after_put == "body": state_body[0] += b"\n"
                if change_after_put == "lock": lock_present[0] = True
                return subprocess.CompletedProcess([], 1 if put_error else 0, "", put_error)
            return subprocess.CompletedProcess([], 1 if legacy_denials else 0, "PRIVATE_APPLY_CREDENTIAL",
                                               "(AccessDenied) PRIVATE_SENTINEL" if legacy_denials else "")

        with tempfile.TemporaryDirectory() as directory:
            selected_mode = mode or ({"plan-read-only": "plan", "apply-read-only": "apply",
                                      "post-noop-read-only": "plan",
                                      "security-probe": "security-probe"}.get(args[0], "apply") if args else "apply")
            env = {"RUNNER_TEMP": directory, "GITHUB_STEP_SUMMARY": directory + "/summary", "GITHUB_ENV": directory + "/env",
                   "MODE": selected_mode,
                   "DEV_TERRAFORM_SECURITY_PROBE_ENABLED": probe_gate if probe_gate is not None else ("true" if args == ("security-probe",) else "false"),
                   "HISTORY_POST_NOOP": "true" if args == ("post-noop-read-only",) else "false",
                   "DEV_HISTORY_POST_NOOP_ENABLED": post_noop_gate if post_noop_gate is not None else "false",
                   "TF_VAR_manage_user_activity_history": "true" if history_enabled else "false",
                   "GITHUB_SHA": "a" * 40, "ACTIONS_ID_TOKEN_REQUEST_URL": "https://fixture.invalid/oidc?request=1",
                   "ACTIONS_ID_TOKEN_REQUEST_TOKEN": "PRIVATE_GITHUB_TOKEN", "BACKEND_ROLE_ARN": "arn:aws:iam::PRIVATE_ACCOUNT:role/fixture-plan",
                   "BACKEND_ROLE_ID": expected_role_id,
                   "STATE_ACCOUNT_ID": "PRIVATE_ACCOUNT", "STATE_BUCKET": "PRIVATE_BUCKET", "STATE_KEY": state_key,
                   "GCP_SMOKE_ACCESS_TOKEN": "PRIVATE_PLAN_TOKEN", "GCP_SMOKE_SERVICE_ACCOUNT": service_account or f"terraform-dev-plan@{project}.iam.gserviceaccount.com",
                   "QUOTA_IDENTITY_REQUIRED": "true", "EXPORT_TOPIC_IDENTITY_REQUIRED": "true",
                   "EXPORT_SCHEDULER_IDENTITY_REQUIRED": "true", "EXPORT_FUNCTION_IDENTITY_REQUIRED": "true"}
            stdout, stderr = io.StringIO(), io.StringIO()
            with patch.dict(os.environ, env, clear=True), patch.object(smoke.urllib.request, "urlopen", side_effect=urlopen), \
                    patch.object(smoke, "aws", side_effect=aws), contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
                rc = smoke.main(list(args))
            summary_file = Path(env["GITHUB_STEP_SUMMARY"])
            summary = summary_file.read_text() if summary_file.exists() else ""
            return rc, http_calls, aws_calls, summary, stdout.getvalue() + stderr.getvalue()

    def test_plan_cli_exact_dev_read_allowlist_even_if_omitted_probes_would_succeed(self):
        rc, http, aws, summary, output = self.run_smoke()
        self.assertEqual(rc, 0, output)
        self.assertEqual([call[:2] for call in aws], [("sts", "get-caller-identity"), ("s3api", "get-object")])
        self.assertEqual(aws[1][aws[1].index("--key") + 1], "youtube-study-space/dev/terraform.tfstate")
        self.assertEqual([(method, url.split("googleapis.com/")[-1]) for method, url, _ in http[1:]], [
            ("GET", "v3/projects/test-youtube-study-space"),
            ("POST", "v3/projects/test-youtube-study-space:testIamPermissions"),
            ("POST", "v3/projects/test-youtube-study-space:testIamPermissions"),
            ("POST", "v3/projects/test-youtube-study-space:testIamPermissions"),
            ("GET", "v1/projects/test-youtube-study-space/locations/asia-southeast2/functions/firestoreCollectionsExport"),
        ])
        self.assertEqual(http[0][0], "GET")
        self.assertNotIn("PRIVATE", summary + output)

    def test_omissions_are_skipped_and_never_reported_as_deny_pass(self):
        rc, _, _, summary, output = self.run_smoke()
        self.assertEqual(rc, 0)
        skipped = [line for line in summary.splitlines() if line.startswith("- SKIPPED")]
        self.assertEqual(len(skipped), 5)
        self.assertIn("Full security gate: NOT VERIFIED", summary)
        self.assertIn("full security gate: NOT VERIFIED", output)
        for line in summary.splitlines():
            if line.startswith("- PASS"):
                for omitted in ("AssumeRole", "generateAccessToken", "PutObject", "production", "other-product", "impersonation"):
                    self.assertNotIn(omitted, line)

    def test_read_only_plan_still_rejects_dev_mutation_grants_and_missing_reads(self):
        for permission in ("bigquery.jobs.create", "bigquery.tables.getData", "bigquery.tables.update", "iam.serviceAccounts.getAccessToken"):
            with self.subTest(extra=permission):
                rc, _, _, _, output = self.run_smoke(extra_permissions=(permission,))
                self.assertEqual(rc, 1)
                self.assertNotIn("PRIVATE", output)
        for permission in ("monitoring.alertPolicies.get", "pubsub.topics.get", "cloudscheduler.jobs.get", "cloudfunctions.functions.get"):
            with self.subTest(missing=permission):
                self.assertEqual(self.run_smoke(missing_permission=permission)[0], 1)

    def test_wrong_sha_identity_state_or_dependency_error_fails_closed(self):
        project = "test-youtube-study-space"
        cases = [{"claims_override": {"workflow_sha": "b" * 40}}, {"claims_override": {"environment": "terraform-dev-apply"}},
                 {"account": "WRONG_ACCOUNT"}, {"state_key": "youtube-study-space/prod/terraform.tfstate"},
                 {"service_account": f"terraform-dev-apply@{project}.iam.gserviceaccount.com"},
                 *[{"fail_read": name} for name in ("project", "function", "state")]]
        for overrides in cases:
            with self.subTest(overrides=overrides):
                rc, http, aws, _, output = self.run_smoke(**overrides)
                self.assertEqual(rc, 1)
                self.assertNotIn("PRIVATE", output)
                self.assertTrue(all(call[:2] in (("sts", "get-caller-identity"), ("s3api", "get-object")) for call in aws))
                self.assertFalse(any("generateAccessToken" in url or "/projects/youtube-study-space" in url for _, url, _ in http))

    def test_dependency_stages_are_fixed_and_hide_sentinels(self):
        expected = {"oidc": "oidc", "project": "gcp-project", "permissions": "gcp-project",
                    "quota": "gcp-quota", "export": "gcp-export",
                    "function": "gcp-function", "sts": "aws-sts", "state": "aws-state-read"}
        for failure, stage in expected.items():
            with self.subTest(failure=failure):
                rc, _, _, summary, output = self.run_smoke(fail_read=failure)
                self.assertEqual(rc, 1)
                self.assertEqual(summary, "")
                category = "check-failed" if failure == "state" else "dependency-error"
                self.assertIn(f"stage={stage}; category={category}", output)
                self.assertNotIn("PRIVATE", output)

    def test_invalid_response_and_guard_stages_are_fixed(self):
        for overrides, stage, category in [
            ({"claims_override": {"workflow_sha": "b" * 40}}, "oidc", "check-failed"),
            ({"account": "WRONG_ACCOUNT"}, "aws-sts", "check-failed"),
            ({"state_key": "youtube-study-space/prod/terraform.tfstate"}, "aws-state-read", "check-failed"),
        ]:
            with self.subTest(stage=stage):
                rc, _, _, summary, output = self.run_smoke(**overrides)
                self.assertEqual((rc, summary), (1, ""))
                self.assertIn(f"stage={stage}; category={category}", output)
                self.assertNotIn("PRIVATE", output)

    def test_explicit_security_probe_keeps_all_negative_checks_and_reports_actual_denials(self):
        rc, http, aws, summary, output = self.run_smoke(args=("security-probe",), legacy_denials=True)
        self.assertEqual(rc, 0, output)
        operations = [call[:2] for call in aws]
        self.assertEqual(operations.count(("sts", "get-caller-identity")), 2)
        self.assertEqual(operations.count(("sts", "assume-role-with-web-identity")), 1)
        self.assertEqual(operations.count(("s3api", "put-object")), 1)
        self.assertEqual(operations.count(("s3api", "list-objects-v2")), 4)
        self.assertEqual(operations.count(("s3api", "head-object")), 6)
        put = next(call for call in aws if call[:2] == ("s3api", "put-object"))
        self.assertIn("--if-match", put)
        self.assertNotIn("--if-none-match", put)
        self.assertRegex(put[put.index("--if-match") + 1], r'^"[0-9a-f]{32}"$')
        self.assertNotEqual(put[put.index("--if-match") + 1], '"dummy-etag"')
        self.assertEqual(put[put.index("--expected-bucket-owner") + 1], "PRIVATE_ACCOUNT")
        self.assertEqual(sum("generateAccessToken" in url for _, url, _ in http), 1)
        self.assertEqual(sum("/projects/youtube-study-space:" in url for _, url, _ in http), 3)
        self.assertIn("PASS: AWS plan state PutObject denied for mismatched If-Match request", summary)
        self.assertNotIn("SKIPPED", summary)
        self.assertNotIn("PRIVATE", summary + output)
        self.assertEqual(self.run_smoke(args=())[0], 1)

    def test_security_probe_rejects_wrong_runtime_identity_before_any_negative_request(self):
        rc, http, aws, summary, output = self.run_smoke(
            args=("security-probe",), service_account="terraform-dev-apply@" + "test-youtube-study-space.iam.gserviceaccount.com")
        self.assertEqual(rc, 1)
        self.assertEqual(http, [])
        self.assertEqual(aws, [])
        self.assertEqual(summary, "")
        self.assertIn("stage=gcp-plan-service-account-target; category=check-failed", output)
        self.assertNotIn("PRIVATE", output)

        for overrides, category in [
            ({"expected_role_id": ""}, "check-failed"),
            ({"expected_role_id": "WRONG_ROLE_ID"}, "check-failed"),
            ({"returned_user_id": ""}, "check-failed"),
            ({"returned_user_id": None}, "check-failed"),
            ({"account": "WRONG_ACCOUNT"}, "check-failed"),
            ({"sts_returncode": 1}, "check-failed"),
            ({"fail_read": "sts"}, "dependency-error"),
        ]:
            with self.subTest(overrides=overrides):
                rc, http, aws, summary, output = self.run_smoke(
                    args=("security-probe",), legacy_denials=True, **overrides)
                self.assertEqual(rc, 1)
                self.assertEqual(http, [])
                self.assertEqual([call[:2] for call in aws], [("sts", "get-caller-identity")])
                self.assertEqual(summary, "")
                self.assertIn(f"stage=aws-sts; category={category}", output)
                self.assertNotIn("PRIVATE", output)

    def test_apply_read_only_uses_exact_dev_state_and_lock_reads_without_any_negative_request(self):
        for probe_gate in ("false", "true"):
            with self.subTest(probe_gate=probe_gate):
                rc, http, calls, summary, output = self.run_smoke(args=("apply-read-only",),
                    probe_gate=probe_gate, history_enabled=True)
                self.assertEqual(rc, 0, output)
                self.assertEqual([c[:2] for c in calls], [
                    ("sts", "get-caller-identity"), ("s3api", "head-object"),
                    ("s3api", "get-object"), ("s3api", "head-object"),
                    ("s3api", "list-objects-v2")])
                self.assertFalse(any("generateAccessToken" in url or "/projects/youtube-study-space:" in url
                                     for _, url, _ in http))
                self.assertIn("Full security gate: NOT VERIFIED", summary)
                self.assertFalse(any("PutObject denied" in line for line in summary.splitlines() if line.startswith("- PASS")))
                self.assertNotIn("PRIVATE", summary + output)
        for mode, gate in (("plan", "true"), ("security-probe", "true")):
            rc, http, calls, summary, _ = self.run_smoke(args=("apply-read-only",), mode=mode, probe_gate=gate)
            self.assertEqual((rc, http, calls, summary), (1, [], [], ""))

    def test_post_noop_read_only_requires_separate_gate_and_uses_positive_reads(self):
        rc, http, calls, summary, output = self.run_smoke(
            args=("post-noop-read-only",), post_noop_gate="true", history_enabled=True)
        self.assertEqual(rc, 0, output)
        self.assertEqual([call[:2] for call in calls], [
            ("sts", "get-caller-identity"), ("s3api", "head-object"),
            ("s3api", "get-object"), ("s3api", "head-object"),
            ("s3api", "list-objects-v2")])
        self.assertFalse(any("generateAccessToken" in url for _, url, _ in http))
        self.assertIn("Full security gate: NOT VERIFIED", summary)
        for options in ({"post_noop_gate": "false", "history_enabled": True},
                        {"post_noop_gate": "true", "history_enabled": False},
                        {"post_noop_gate": "true", "history_enabled": True, "mode": "apply"}):
            with self.subTest(options=options):
                rc, http, calls, summary, _ = self.run_smoke(args=("post-noop-read-only",), **options)
                self.assertEqual((rc, http, calls, summary), (1, [], [], ""))

    def test_full_negative_probe_requires_explicit_cli_mode_and_independent_gate(self):
        for args, mode, gate in (((), "security-probe", "true"), (("security-probe",), "apply", "true"),
                                 (("security-probe",), "security-probe", "false")):
            rc, http, calls, summary, _ = self.run_smoke(args=args, mode=mode, probe_gate=gate, legacy_denials=True)
            self.assertEqual((rc, http, calls, summary), (1, [], [], ""))

    def test_conditional_probe_accepts_only_explicit_deny_with_unchanged_private_snapshots(self):
        for error in ("(412) PreconditionFailed", "(404) NoSuchKey", "(409) ConditionalRequestConflict",
                      "timeout PRIVATE_SENTINEL", "(403) and (412) PRIVATE_SENTINEL", ""):
            with self.subTest(error=error):
                rc, _, calls, summary, output = self.run_smoke(args=("security-probe",), legacy_denials=True, put_error=error)
                self.assertEqual(rc, 1)
                self.assertEqual(sum(c[:2] == ("s3api", "get-object") for c in calls), 2)
                self.assertEqual(summary, "")
                self.assertIn("stage=aws-state-probe-deny; category=check-failed", output)
                self.assertNotIn("PRIVATE", output)
        for mutation in ("version", "body", "lock"):
            with self.subTest(mutation=mutation):
                rc, _, _, summary, output = self.run_smoke(args=("security-probe",), legacy_denials=True,
                    change_after_put=mutation)
                self.assertEqual((rc, summary), (1, ""))
                stage = "aws-state-probe-post" if mutation == "lock" else "aws-state-probe-invariant"
                self.assertIn(f"stage={stage};", output)
        rc, _, calls, summary, _ = self.run_smoke(args=("security-probe",), legacy_denials=True)
        self.assertEqual(rc, 0)
        self.assertIn("AWS plan state PutObject denied for mismatched If-Match request", summary)
        self.assertIn("request/header only", summary)
        self.assertEqual(sum(c[:2] == ("s3api", "get-object") for c in calls), 2)
        with patch.object(smoke.secrets, "token_hex", return_value="dummy-etag"):
            rc, _, calls, summary, _ = self.run_smoke(args=("security-probe",), legacy_denials=True)
        self.assertEqual((rc, summary), (1, ""))
        self.assertFalse(any(c[:2] == ("s3api", "put-object") for c in calls))
        rc, _, _, summary, output = self.run_smoke(args=("security-probe",), legacy_denials=True,
            put_error="raise")
        self.assertEqual((rc, summary), (1, ""))
        self.assertIn("stage=aws-state-probe-request; category=dependency-error", output)
        self.assertNotIn("PRIVATE", output)

    def test_invalid_read_only_cli_does_not_fall_back_to_legacy(self):
        rc, http, aws, summary, _ = self.run_smoke(args=("plan-read-only", "apply"))
        self.assertEqual((rc, http, aws, summary), (1, [], [], ""))

    def test_read_only_helpers_reject_apply_identity_before_any_request(self):
        from unittest.mock import Mock
        account = "terraform-dev-apply@fixture.iam.gserviceaccount.com"
        request = Mock()
        for operation in (
            lambda: smoke.verify_google("PRIVATE_TOKEN", account, request, project="fixture", plan_read_only=True),
            lambda: smoke.verify_quota_google("PRIVATE_TOKEN", account, request, project="fixture", identity="apply", plan_read_only=True),
            lambda: smoke.verify_export_topic_google("PRIVATE_TOKEN", account, request, project="fixture", plan_read_only=True),
        ):
            with self.assertRaises(smoke.SmokeFailure): operation()
        request.assert_not_called()


class HistoryPlanCliTest(unittest.TestCase):
    """Run the real history branch with synthetic receipt files and fake AWS."""

    def setUp(self):
        self.fixture = receipt_fixtures.ReceiptTests("runTest")
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.aws = self.fixture.aws
        self.fixture.env["GCP_SMOKE_SERVICE_ACCOUNT"] = "terraform-dev-plan@" + "test-youtube-study-space.iam.gserviceaccount.com"

    def run_history(self, request=None, *, make_policy=True):
        if make_policy:
            receipt.policy(self.fixture.env)
        output, errors = io.StringIO(), io.StringIO()
        with patch.dict(os.environ, self.fixture.env, clear=True), \
                patch.object(smoke, "verify_oidc", return_value=["OIDC dummy checks"]), \
                patch.object(smoke, "verify_google", return_value=["GCP dummy checks"]), \
                patch.object(smoke, "aws", side_effect=request or self.aws), \
                contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
            result = smoke.main(["plan-read-only"])
        summary = Path(self.fixture.env["GITHUB_STEP_SUMMARY"])
        return result, output.getvalue() + errors.getvalue(), summary.read_text() if summary.exists() else ""

    def test_normal_history_cli_accepts_sdk_url_encoding_and_only_reads(self):
        result, output, summary = self.run_history()
        self.assertEqual(result, 0, output)
        self.assertIn("exact persistent eleven-resource state", summary)
        self.assertEqual([call[:2] for call in self.aws.calls], [
            ("sts", "get-caller-identity"), ("s3api", "head-object"),
            ("s3api", "get-object"), ("s3api", "head-object"),
            ("s3api", "list-objects-v2"), ("s3api", "list-objects-v2")])
        self.assertTrue((self.fixture.root / "history-plan-state-before-receipt.json").exists())
        self.assertNotIn("DUMMY", output + summary)
        self.assertNotIn("PRIVATE", output + summary)

    def test_history_policy_head_get_rehead_and_lists_report_fixed_stages(self):
        cases = {"history-policy": None, "history-head": "head-object",
                 "history-get": "get-object", "history-re-head": "head-object",
                 "history-lock-list": "list-objects-v2", "history-workspace-list": "list-objects-v2"}
        for stage, operation in cases.items():
            with self.subTest(stage=stage):
                child = receipt_fixtures.ReceiptTests("runTest")
                child.setUp(); self.addCleanup(child.doCleanups)
                child.env["GCP_SMOKE_SERVICE_ACCOUNT"] = "terraform-dev-plan@" + "test-youtube-study-space.iam.gserviceaccount.com"
                calls = 0
                def failing_request(*args):
                    nonlocal calls
                    result = child.aws(*args)
                    if args[1] == operation:
                        calls += 1
                        if stage not in {"history-re-head", "history-workspace-list"} or calls == 2:
                            raise RuntimeError("PRIVATE_URL_TOKEN_HTTP_BODY_TRACEBACK")
                    return result
                output, errors = io.StringIO(), io.StringIO()
                if stage != "history-policy":
                    receipt.policy(child.env)
                with patch.dict(os.environ, child.env, clear=True), \
                        patch.object(smoke, "verify_oidc", return_value=[]), \
                        patch.object(smoke, "verify_google", return_value=[]), \
                        patch.object(smoke, "aws", side_effect=failing_request), \
                        contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
                    result = smoke.main(["plan-read-only"])
                exposed = output.getvalue() + errors.getvalue()
                self.assertEqual(result, 1)
                self.assertIn(f"stage={stage}; category=dependency-error", exposed)
                self.assertNotIn("PRIVATE", exposed)
                self.assertFalse((child.root / "history-plan-state-before-receipt.json").exists())

    def test_invalid_history_evidence_is_classified_without_passing(self):
        cases = {
            "history-policy": (None, {"profile": "PRIVATE_BAD_PROFILE"}),
            "history-head": ("head-object", {"ContentLength": 0}),
            "history-get": ("get-object", {"VersionId": "PRIVATE_WRONG_VERSION"}),
            "history-re-head": ("head-object", {"VersionId": "PRIVATE_WRONG_VERSION"}),
            "history-lock-list": ("list-objects-v2", {"KeyCount": 1}),
            "history-workspace-list": ("list-objects-v2", {"KeyCount": 1}),
        }
        for stage, (operation, change) in cases.items():
            with self.subTest(stage=stage):
                child = receipt_fixtures.ReceiptTests("runTest")
                child.setUp(); self.addCleanup(child.doCleanups)
                child.env["GCP_SMOKE_SERVICE_ACCOUNT"] = "terraform-dev-plan@" + "test-youtube-study-space.iam.gserviceaccount.com"
                receipt.policy(child.env)
                if stage == "history-policy":
                    (child.root / "history-plan-admission.json").write_text(json.dumps(change))
                calls = 0
                def corrupted_request(*args):
                    nonlocal calls
                    result = child.aws(*args)
                    if args[1] == operation:
                        calls += 1
                        if stage not in {"history-re-head", "history-workspace-list"} or calls == 2:
                            value = json.loads(result.stdout)
                            value.update(change)
                            return subprocess.CompletedProcess([], 0, json.dumps(value), "")
                    return result
                output, errors = io.StringIO(), io.StringIO()
                with patch.dict(os.environ, child.env, clear=True), \
                        patch.object(smoke, "verify_oidc", return_value=[]), \
                        patch.object(smoke, "verify_google", return_value=[]), \
                        patch.object(smoke, "aws", side_effect=corrupted_request), \
                        contextlib.redirect_stdout(output), contextlib.redirect_stderr(errors):
                    result = smoke.main(["plan-read-only"])
                exposed = output.getvalue() + errors.getvalue()
                self.assertEqual(result, 1)
                self.assertIn(f"stage={stage}; category=invalid-evidence", exposed)
                self.assertNotIn("PRIVATE", exposed)
                self.assertFalse((child.root / "history-plan-state-before-receipt.json").exists())


class HistoryPlanSubprocessCliTest(unittest.TestCase):
    """Exercise the script as __main__, with every external call replaced in the child."""

    def test_script_entrypoint_keeps_history_stage_and_secret_boundary(self):
        claims = {
            "repository_id": "340900071", "repository_owner_id": "54093651",
            "environment": "terraform-dev-plan", "ref": "refs/heads/feature/gcp-terraform-iac",
            "workflow_ref": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
            "job_workflow_ref": "kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/feature/gcp-terraform-iac",
            "event_name": "workflow_dispatch",
        }
        claims["sub"] = ":".join(f"{key}:{value.replace(':', '%3A')}" for key, value in claims.items())
        claims.update(workflow_sha="a" * 40, job_workflow_sha="a" * 40)
        token = "header." + base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip("=") + ".DUMMY_SIGNATURE"
        sitecustomize = '''\
import io
import json
import os
import subprocess
import urllib.request
from datetime import datetime, timezone
from test_terraform_history_plan_receipt import FakeS3
import terraform_history_plan_receipt as receipt

class FixedDatetime(datetime):
    @classmethod
    def now(cls, tz=None):
        return datetime(2026, 10, 8, 12, tzinfo=timezone.utc)

receipt.datetime = FixedDatetime

backend = FakeS3()
mutation = os.environ.get("DUMMY_STATE_MUTATION")
if mutation:
    state = json.loads(backend.body)
    if mutation == "output-sensitive":
        state["outputs"]["project_id"]["sensitive"] = True
    elif mutation == "output-extra":
        state["outputs"]["project_id"]["PRIVATE_UNKNOWN_FIELD"] = "PRIVATE_SENTINEL"
    elif mutation == "check-unknown":
        state["check_results"][1]["config_addr"] = "module.PRIVATE_UNKNOWN.var.secret"
    elif mutation == "check-empty-active":
        state["check_results"][1]["objects"] = None
    elif mutation == "check-nonpass":
        state["check_results"][-2]["status"] = "unknown"
    elif mutation == "state-serial-bool":
        state["serial"] = True
    elif mutation == "resource-extra":
        state["resources"].append(state["resources"][0] | {"name": "PRIVATE_EXTRA"})
    if mutation == "json-duplicate":
        backend.body = b'{"version":4,"version":4}'
    else:
        backend.body = json.dumps(state).encode()
counts = {}
sentinel = "PRIVATE_URL_TOKEN_HTTP_BODY_TRACEBACK"

def fake_run(command, **kwargs):
    if command[0] != "aws" or command[-2:] != ["--output", "json"]:
        raise RuntimeError(sentinel)
    args = command[1:-2]
    pair = tuple(args[:2])
    if pair not in {("sts", "get-caller-identity"), ("s3api", "head-object"),
                    ("s3api", "get-object"), ("s3api", "list-objects-v2")}:
        raise RuntimeError(sentinel)
    operation = pair[1]
    counts[operation] = counts.get(operation, 0) + 1
    stage = os.environ.get("DUMMY_FAIL_STAGE")
    failure = {"history-head": ("head-object", 1), "history-get": ("get-object", 1),
               "history-re-head": ("head-object", 2), "history-lock-list": ("list-objects-v2", 1),
               "history-workspace-list": ("list-objects-v2", 2)}.get(stage)
    if failure == (operation, counts[operation]):
        raise RuntimeError(sentinel)
    return backend(*args)

def fake_urlopen(request, **kwargs):
    url = request.full_url
    if "fixture.invalid/oidc" in url:
        payload = {"value": os.environ["DUMMY_OIDC_TOKEN"]}
    elif url.endswith("/v3/projects/test-youtube-study-space"):
        payload = {"projectId": "test-youtube-study-space", "state": "ACTIVE"}
    elif url.endswith("/v3/projects/test-youtube-study-space:testIamPermissions"):
        payload = {"permissions": []}
    else:
        raise RuntimeError(sentinel)
    response = io.BytesIO(json.dumps(payload).encode())
    response.status = 200
    return response

subprocess.run = fake_run
urllib.request.urlopen = fake_urlopen
'''
        script = Path(__file__).with_name("terraform_identity_smoke.py")
        cases = [(None, None, None),
                 *[(stage, None, "file-open-read" if stage == "history-policy" else "s3-request")
                   for stage in ("history-policy", "history-head", "history-get", "history-re-head",
                                 "history-lock-list", "history-workspace-list")],
                 *[("history-get", mutation, reason) for mutation, reason in (
                     ("output-sensitive", "state-output-value"), ("output-extra", "state-output-value"),
                     ("check-unknown", "checks-address"), ("check-empty-active", "checks-object-list"),
                     ("check-nonpass", "checks-result"), ("state-serial-bool", "state-serial"),
                     ("resource-extra", "state-address-set"), ("json-duplicate", "json-duplicate"))]]
        for stage, mutation, reason in cases:
            with self.subTest(stage=stage, mutation=mutation):
                fixture = receipt_fixtures.ReceiptTests("runTest")
                fixture.setUp(); self.addCleanup(fixture.doCleanups)
                evidence = receipt_fixtures.safety_profile()
                if stage != "history-policy":
                    budget = receipt_fixtures.ACTUAL_EXECUTION_POLICY(json.dumps(evidence), fixture.env["GITHUB_SHA"], now=receipt_fixtures.NOW)
                    receipt.write_private(fixture.root / "history-plan-admission.json", budget)
                fixture.env.update({
                    "GCP_SMOKE_SERVICE_ACCOUNT": "terraform-dev-plan@" + "test-youtube-study-space.iam.gserviceaccount.com",
                    "ACTIONS_ID_TOKEN_REQUEST_URL": "https://fixture.invalid/oidc?request=1",
                    "ACTIONS_ID_TOKEN_REQUEST_TOKEN": "DUMMY_GITHUB_TOKEN",
                    "BACKEND_ROLE_ARN": "arn:aws:iam::" + "111111111111:role/fixture-plan",
                    "DUMMY_OIDC_TOKEN": token,
                    "DUMMY_FAIL_STAGE": (stage or "") if mutation is None else "",
                    "DUMMY_STATE_MUTATION": mutation or "",
                    "PYTHONPATH": os.pathsep.join((str(fixture.root), str(script.parent))),
                    "PYTHONDONTWRITEBYTECODE": "1",
                })
                (fixture.root / "sitecustomize.py").write_text(sitecustomize)
                result = subprocess.run([sys.executable, str(script), "plan-read-only"],
                                        env=fixture.env, capture_output=True, text=True, timeout=15)
                exposed = result.stdout + result.stderr
                self.assertNotIn("PRIVATE_", exposed)
                self.assertNotIn("Traceback", exposed)
                if stage is None:
                    self.assertEqual(result.returncode, 0, exposed)
                    self.assertIn("Development plan read-only checks: PASS", exposed)
                    self.assertTrue((fixture.root / "history-plan-state-before-receipt.json").exists())
                else:
                    self.assertEqual(result.returncode, 1, exposed)
                    category = "invalid-evidence" if mutation else "dependency-error"
                    self.assertIn(f"stage={stage}; category={category}; reason={reason}", exposed)
                    self.assertFalse((fixture.root / "history-plan-state-before-receipt.json").exists())


if __name__ == "__main__":
    unittest.main()
