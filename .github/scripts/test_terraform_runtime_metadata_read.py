#!/usr/bin/env python3
"""Exact runtime metadata observations with dummy identities and HTTP fixtures."""
from __future__ import annotations

import base64
import contextlib
import copy
from datetime import datetime, timedelta, timezone
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import urllib.error

import terraform_runtime_metadata_read as gate


class RuntimeReadTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.number = "111111111111"
        self.env = {
            "RUNNER_TEMP": str(self.root), "GITHUB_STEP_SUMMARY": str(self.root / "summary"),
            "MODE": "runtime-metadata-read", "RUNTIME_TARGET": "dev", "RUNTIME_READ_IDENTITY": "plan",
            "DEV_RUNTIME_METADATA_READ_ENABLED": "true", "GITHUB_SHA": "a" * 40,
            "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
            "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
            "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac", "GITHUB_RUN_ATTEMPT": "1",
            "GITHUB_JOB": "runtime_metadata_read_plan",
            "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
            "GCP_SMOKE_SERVICE_ACCOUNT": f"terraform-dev-plan@{gate.PROJECT}.iam.gserviceaccount.com",
            "GCP_SMOKE_ACCESS_TOKEN": "DUMMY_PRIVATE_TOKEN",
            "RUNTIME_READ_WIF_PROVIDER": f"projects/{self.number}/locations/global/workloadIdentityPools/dummy-github/providers/dummy-provider",
            "ACTIONS_ID_TOKEN_REQUEST_URL": "https://dummy.invalid/oidc?request=1",
            "ACTIONS_ID_TOKEN_REQUEST_TOKEN": "DUMMY_PRIVATE_OIDC_TOKEN",
        }
        now = datetime.now(timezone.utc).replace(microsecond=0)
        self.env.update(DEV_RUNTIME_METADATA_READ_ISSUED_UTC=(now - timedelta(seconds=1)).strftime("%Y-%m-%dT%H:%M:%SZ"),
                        DEV_RUNTIME_METADATA_READ_EXPIRES_UTC=(now + timedelta(hours=7)).strftime("%Y-%m-%dT%H:%M:%SZ"))
        self.pool = f"projects/{self.number}/locations/global/workloadIdentityPools/aws-runtime"
        self.data = {
            "runtime-pool": {"name": self.pool, "state": "ACTIVE", "displayName": "DUMMY_PRIVATE_POOL"},
            "runtime-attestation": {},
            "runtime-provider": {"name": self.pool + "/providers/aws-provider", "state": "ACTIVE",
                "aws": {"accountId": "222222222222"}, "attributeMapping": {"google.subject": "assertion.arn", "attribute.aws_role": "assertion.arn", "attribute.account": "assertion.account"},
                "attributeCondition": "attribute.account == 'DUMMY_PRIVATE_CEL'"},
            "runtime-sa-policy": {"version": 3, "etag": "DUMMY_PRIVATE_ETAG", "bindings": [
                {"role": "roles/iam.workloadIdentityUser", "members": ["DUMMY_PRIVATE_MEMBER"],
                 "condition": {"title": "DUMMY_PRIVATE_TITLE", "expression": "true"}},
                {"role": "roles/viewer", "members": ["user:dummy@example.invalid"]}]},
            "runtime-project": {"projectId": gate.PROJECT, "projectNumber": self.number, "lifecycleState": "ACTIVE"},
            "runtime-enabled-services": {"services": [self.service("iam.googleapis.com"), self.service("serviceusage.googleapis.com")]},
            "runtime-project-permissions": {},
        }
        self.calls = []

    def service(self, name):
        return {"name": f"projects/{self.number}/services/{name}", "parent": "projects/" + self.number,
                "state": "ENABLED", "config": {"name": name, "title": "DUMMY_PRIVATE_TITLE"}}

    def alias(self, host, path):
        if path.endswith(":testIamPermissions"):
            return "runtime-project-permissions"
        if host == "serviceusage.googleapis.com":
            return "runtime-enabled-services"
        if host == "cloudresourcemanager.googleapis.com":
            return "runtime-project"
        if ":listAttestationRules" in path:
            return "runtime-attestation"
        if ":getIamPolicy" in path:
            return "runtime-sa-policy"
        if "/providers/" in path:
            return "runtime-provider"
        return "runtime-pool"

    def request(self, path, token, body=None, *, host, method):
        alias = self.alias(host, path)
        self.calls.append((alias, host, path, method, body, token))
        self.assertEqual(token, "DUMMY_PRIVATE_TOKEN")
        if alias == "runtime-project-permissions":
            self.assertEqual((method, body), ("POST", {"permissions": list(gate.MUTATIONS)}))
        elif alias == "runtime-sa-policy":
            self.assertEqual((method, body), ("POST", None))
        else:
            self.assertEqual((method, body), ("GET", None))
        return 200, copy.deepcopy(self.data[alias])

    def run_read(self, request=None, **overrides):
        return gate.read(self.env | overrides, request=request or self.request)

    def safe_summary(self, status):
        result = (self.root / "summary").read_text()
        self.assertIn("runtime-metadata-read: " + status, result)
        self.assertEqual(set(line.rsplit(": ", 1)[-1] for line in result.splitlines()) <= {"PASS", "STOP", "未実行"}, True)
        self.assertEqual({p.name for p in self.root.iterdir()}, {"summary"})
        for secret in ("DUMMY_", self.number, gate.PROJECT, "appspot", "googleapis", "principalSet", "attribute.", "roles/"):
            self.assertNotIn(secret, result)
        return result

    def test_exact_six_metadata_types_twice_and_scoped_permission_read_only(self):
        self.run_read()
        self.safe_summary("PASS")
        self.assertEqual([c[0] for c in self.calls], ["runtime-project", "runtime-pool", "runtime-attestation", "runtime-provider", "runtime-sa-policy", "runtime-enabled-services"] * 2 + ["runtime-project-permissions"])
        pool_paths = [c[2] for c in self.calls if c[0] == "runtime-pool"]
        self.assertEqual(pool_paths, ["v1/" + self.pool] * 2)
        self.assertIn(":getIamPolicy?options.requestedPolicyVersion=3", self.calls[4][2])
        self.assertEqual(self.calls[0][2], "v1/projects/" + gate.PROJECT)
        self.assertEqual(self.calls[5][2], f"v1/projects/{self.number}/services?filter=state:ENABLED&pageSize=200")
        for call in self.calls:
            for forbidden in ("youtube-study-space/production", ":setIamPolicy", ":generateAccessToken", "/keys", "/buckets", "bigquery", "cloudfunctions"):
                self.assertNotIn(forbidden, call[2])

    def test_wrong_project_number_or_scope_stops_before_any_runtime_endpoint(self):
        for changes in ({"projectNumber": "999999999999"}, {"projectId": "youtube-study-space"}):
            self.calls.clear()
            original = copy.deepcopy(self.data["runtime-project"])
            self.data["runtime-project"].update(changes)
            with self.subTest(changes=changes), self.assertRaises(Exception):
                self.run_read()
            self.assertEqual([c[0] for c in self.calls], ["runtime-project"])
            self.safe_summary("STOP")
            self.data["runtime-project"] = original

    def test_apply_has_separate_exact_context_and_reads(self):
        self.run_read(RUNTIME_READ_IDENTITY="apply", GITHUB_JOB="runtime_metadata_read_apply",
            GCP_SMOKE_SERVICE_ACCOUNT=f"terraform-dev-apply@{gate.PROJECT}.iam.gserviceaccount.com")
        self.safe_summary("PASS")
        for override in ({"RUNTIME_READ_IDENTITY": "apply"}, {"GITHUB_JOB": "apply"},
                         {"GCP_SMOKE_SERVICE_ACCOUNT": f"terraform-dev-apply@{gate.PROJECT}.iam.gserviceaccount.com"}):
            self.calls.clear()
            with self.subTest(override=override), self.assertRaises(Exception):
                self.run_read(**override)
            self.assertEqual(self.calls, [])

    def test_independent_non_success_probe_preserves_other_observed_results_without_payload(self):
        for alias in gate.ALIASES[:6]:
            for status in (403, 404, 429, 500):
                self.calls.clear()
                (self.root / "summary").unlink(missing_ok=True)
                def request(path, token, body=None, *, host, method):
                    result = self.request(path, token, body, host=host, method=method)
                    return (status, {"error": {"message": "DUMMY_PRIVATE_ERROR"}}) if self.alias(host, path) == alias else result
                with self.subTest(alias=alias, status=status), self.assertRaises(Exception):
                    self.run_read(request)
                summary = self.safe_summary("STOP")
                self.assertIn(alias + ": STOP", summary)
                if alias == "runtime-project":
                    self.assertEqual([call[0] for call in self.calls], ["runtime-project"])
                    self.assertIn("runtime-pool: 未実行", summary)
                else:
                    self.assertEqual(self.calls[-1][0], "runtime-project-permissions")
                    for other in set(gate.ALIASES[:6]) - {alias}:
                        self.assertIn(other + ": PASS", summary)

    def test_one_failed_phase_is_never_filled_in_by_the_successful_phase(self):
        failed = False
        def request(path, token, body=None, *, host, method):
            nonlocal failed
            result = self.request(path, token, body, host=host, method=method)
            if self.alias(host, path) == "runtime-attestation" and not failed:
                failed = True
                return 403, {"error": {"message": "DUMMY_PRIVATE_DENIED"}}
            return result
        with self.assertRaises(Exception):
            self.run_read(request)
        summary = self.safe_summary("STOP")
        self.assertIn("runtime-attestation: STOP", summary)
        self.assertIn("runtime-provider: PASS", summary)
        self.assertIn("runtime-stable: 未実行", summary)
        self.assertEqual(sum(call[0] == "runtime-attestation" for call in self.calls), 2)

    def test_completed_pagination_is_escaped_bounded_and_compared(self):
        def request(path, token, body=None, *, host, method):
            result = self.request(path, token, body, host=host, method=method)
            if host == "serviceusage.googleapis.com":
                if "pageToken=" in path:
                    self.assertTrue(path.endswith("pageToken=DUMMY%2B%2F%3F%26%3D"))
                    return 200, {"services": [self.service("serviceusage.googleapis.com")]}
                return 200, {"services": [self.service("iam.googleapis.com")], "nextPageToken": "DUMMY+/?&="}
            return result
        self.run_read(request)
        self.safe_summary("PASS")
        self.assertEqual(sum(c[0] == "runtime-enabled-services" for c in self.calls), 4)

    def test_incomplete_repeating_oversized_and_unknown_pages_stop(self):
        variants = [None, {"error": {}}, {"unknown": []}, {"services": None}, {"nextPageToken": None},
                    {"nextPageToken": "a" * (gate.MAX_TOKEN + 1)}, {"nextPageToken": "repeats"},
                    {"services": [self.service("iam.googleapis.com")] * 201}]
        for value in variants:
            def request(path, token, body=None, *, host, method):
                result = self.request(path, token, body, host=host, method=method)
                return (200, copy.deepcopy(value)) if host == "serviceusage.googleapis.com" else result
            with self.subTest(value=str(value)[:25]), self.assertRaises(Exception):
                self.run_read(request)
            self.safe_summary("STOP")
        pages = 0
        def request(path, token, body=None, *, host, method):
            nonlocal pages
            result = self.request(path, token, body, host=host, method=method)
            if host == "serviceusage.googleapis.com":
                pages += 1
                return 200, {"nextPageToken": str(pages)}
            return result
        with self.assertRaises(Exception):
            self.run_read(request)
        self.assertEqual(pages, gate.MAX_PAGES * 2)

    def test_attestation_known_empty_shapes_are_supported_without_fabricating_response(self):
        for value in ({}, {"attestationRules": []}, {"nextPageToken": ""}, {"attestationRules": [], "nextPageToken": ""}):
            with self.subTest(value=value):
                self.data["runtime-attestation"] = value
                self.run_read()
        for value in (None, {"error": {}}, {"unknown": False}, {"attestationRules": None},
                      {"attestationRules": [{}]}, {"nextPageToken": "pending"}):
            with self.subTest(value=value), self.assertRaises(Exception):
                self.data["runtime-attestation"] = value
                self.run_read()
            self.safe_summary("STOP")

    def test_unknown_wrong_scope_malformed_disabled_or_incomplete_metadata_stops(self):
        mutations = [
            lambda d: d["runtime-pool"].update(unknown=True),
            lambda d: d["runtime-pool"].update(state="DELETED"),
            lambda d: d["runtime-pool"].update(mode="TRUST_DOMAIN"),
            lambda d: d["runtime-pool"].update(name=self.pool + "-other"),
            lambda d: d["runtime-pool"].update(disabled=True),
            lambda d: d["runtime-provider"].update(oidc={}),
            lambda d: d["runtime-provider"]["aws"].update(accountId="bad"),
            lambda d: d["runtime-provider"].update(attributeMapping={}),
            lambda d: d["runtime-provider"].update(attributeCondition=""),
            lambda d: d["runtime-project"].update(projectId="youtube-study-space"),
            lambda d: d["runtime-project"].update(projectNumber="999999999999"),
            lambda d: d["runtime-project"].update(lifecycleState="DELETE_REQUESTED"),
            lambda d: d["runtime-project"].update(unknown=True),
            lambda d: d["runtime-sa-policy"].update(unknown=True),
            lambda d: d["runtime-sa-policy"].update(version=1),
            lambda d: d["runtime-sa-policy"]["bindings"][0].update(condition=None),
            lambda d: d["runtime-sa-policy"]["bindings"][0].update(members=[None]),
            lambda d: d["runtime-enabled-services"]["services"][0].update(state="DISABLED"),
            lambda d: d["runtime-enabled-services"]["services"][0].update(parent="projects/999"),
            lambda d: d["runtime-enabled-services"]["services"][0]["config"].update(unknown=True),
            lambda d: d["runtime-enabled-services"]["services"].append(copy.deepcopy(d["runtime-enabled-services"]["services"][0])),
        ]
        original = copy.deepcopy(self.data)
        for mutate in mutations:
            self.data = copy.deepcopy(original)
            mutate(self.data)
            with self.subTest(mutate=mutate), self.assertRaises(Exception):
                self.run_read()
            self.safe_summary("STOP")

    def test_every_stable_field_including_unrelated_policy_and_byte_exact_cel_must_match(self):
        for alias, mutate in (
            ("runtime-pool", lambda d: d.update(description="DUMMY_PRIVATE_CHANGE")),
            ("runtime-provider", lambda d: d.update(attributeCondition=d["attributeCondition"] + " ")),
            ("runtime-sa-policy", lambda d: d["bindings"][1].update(role="roles/editor")),
            ("runtime-sa-policy", lambda d: d["bindings"][0]["condition"].update(expression="false")),
            ("runtime-project", lambda d: d.update(name="DUMMY_PRIVATE_CHANGE")),
            ("runtime-enabled-services", lambda d: d["services"][0]["config"].update(title="DUMMY_PRIVATE_CHANGE")),
        ):
            seen = 0
            def request(path, token, body=None, *, host, method):
                nonlocal seen
                status, value = self.request(path, token, body, host=host, method=method)
                if self.alias(host, path) == alias:
                    seen += 1
                    if seen == 2:
                        mutate(value)
                return status, value
            with self.subTest(alias=alias), self.assertRaises(Exception):
                self.run_read(request)
            self.assertIn("runtime-stable: STOP", self.safe_summary("STOP"))

    def test_project_scope_mutation_observation_rejects_grants_denials_and_unknowns(self):
        for status, value in ((403, {}), (200, None), (200, {"unknown": []}),
                              (200, {"permissions": None}), (200, {"permissions": [gate.MUTATIONS[0]]})):
            def request(path, token, body=None, *, host, method):
                result = self.request(path, token, body, host=host, method=method)
                return (status, value) if path.endswith(":testIamPermissions") else result
            with self.subTest(value=value), self.assertRaises(Exception):
                self.run_read(request)
            self.safe_summary("STOP")

    def test_second_project_scope_failure_prevents_all_later_runtime_reads(self):
        projects = 0
        def request(path, token, body=None, *, host, method):
            nonlocal projects
            status, value = self.request(path, token, body, host=host, method=method)
            if self.alias(host, path) == "runtime-project":
                projects += 1
                if projects == 2:
                    value["projectNumber"] = "999999999999"
            return status, value
        with self.assertRaises(Exception):
            self.run_read(request)
        self.assertEqual(len(self.calls), 7)
        summary = self.safe_summary("STOP")
        self.assertIn("runtime-project: STOP", summary)
        self.assertIn("runtime-pool: 未実行", summary)
        self.assertIn("runtime-project-permissions: 未実行", summary)

    def test_expiry_after_the_last_successful_http_response_cannot_report_success(self):
        def request(path, token, body=None, *, host, method):
            result = self.request(path, token, body, host=host, method=method)
            if self.alias(host, path) == "runtime-project-permissions":
                self.env["DEV_RUNTIME_METADATA_READ_EXPIRES_UTC"] = "2000-01-01T00:00:00Z"
            return result
        with self.assertRaises(Exception):
            gate.read(self.env, request=request)
        summary = self.safe_summary("STOP")
        self.assertIn("runtime-stable: PASS", summary)
        self.assertIn("runtime-project-permissions: PASS", summary)
        self.assertIn("runtime-window: STOP", summary)

    def test_closed_gate_untrusted_mixed_and_invalid_source_window_stop_before_requests(self):
        variants = {
            "DEV_RUNTIME_METADATA_READ_ENABLED": "false", "MODE": "apply", "RUNTIME_TARGET": "prod",
            "GITHUB_JOB": "apply", "GITHUB_RUN_ATTEMPT": "2", "GITHUB_REF": "refs/heads/dev",
            "GITHUB_EVENT_NAME": "pull_request", "GITHUB_REPOSITORY_ID": "0", "GITHUB_REPOSITORY_OWNER_ID": "0",
            "GITHUB_REPOSITORY": "dummy/repo", "GITHUB_WORKFLOW_REF": "other", "GITHUB_SHA": "a" * 39,
            "GCP_SMOKE_SERVICE_ACCOUNT": "", "GCP_SMOKE_ACCESS_TOKEN": "", "RUNTIME_READ_WIF_PROVIDER": "projects/invalid",
            "PLAN_SAFETY_EVIDENCE": "{}", "PLAN_COST_EVIDENCE": "{}", "HISTORY_IMPORT_APPROVAL": "{}",
            "OWNERSHIP_WAVE": "pool", "HISTORY_POST_NOOP": "true", "CURRENT_STATE_ATTESTATION": "true",
            "OWNERSHIP_ROOT_BASIS": "current-state-bootstrap", "DEV_RUNTIME_METADATA_READ_ISSUED_UTC": "",
            "DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": "2000-01-01T00:00:00Z",
        }
        for key, value in variants.items():
            with self.subTest(key=key), self.assertRaises(Exception):
                self.run_read(**{key: value})
        self.assertEqual(self.calls, [])
        now = datetime.now(timezone.utc)
        for override in ({"DEV_RUNTIME_METADATA_READ_ISSUED_UTC": (now + timedelta(hours=1)).strftime("%Y-%m-%dT%H:%M:%SZ")},
                         {"DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": (now + timedelta(hours=9)).strftime("%Y-%m-%dT%H:%M:%SZ")}):
            with self.subTest(override=override), self.assertRaises(Exception):
                self.run_read(**override)
        self.assertEqual(self.calls, [])

    def test_window_expiry_between_snapshots_stops_and_writes_no_private_files(self):
        def request(path, token, body=None, *, host, method):
            result = self.request(path, token, body, host=host, method=method)
            if len(self.calls) == 6:
                self.env["DEV_RUNTIME_METADATA_READ_EXPIRES_UTC"] = "2000-01-01T00:00:00Z"
            return result
        with self.assertRaises(Exception):
            gate.read(self.env, request=request)
        self.assertEqual(len(self.calls), 6)
        self.safe_summary("STOP")
        gate.context(self.env, gate=False)  # Cleanup remains available after expiry.

    def test_oversized_aggregate_snapshot_and_body_stop(self):
        self.data["runtime-pool"]["description"] = "x" * gate.MAX_BYTES
        with self.assertRaises(Exception):
            self.run_read()
        self.assertEqual([c[0] for c in self.calls], ["runtime-project", "runtime-pool"])
        self.safe_summary("STOP")
        with self.assertRaises(Exception):
            gate.load(b" " * (gate.MAX_BYTES + 1))
        for raw in (b'{"a":1,"a":2}', b'{"a":NaN}', b'[]', b'{'):
            with self.assertRaises(Exception):
                gate.load(raw)

    def test_temp_or_summary_symlink_does_not_write_or_follow_target(self):
        target = self.root / "private-target"
        target.write_text("DUMMY_PRIVATE_SENTINEL")
        summary = self.root / "summary"
        summary.symlink_to(target)
        with self.assertRaises(Exception):
            self.run_read()
        self.assertEqual(target.read_text(), "DUMMY_PRIVATE_SENTINEL")
        link = self.root / "temp-link"
        link.symlink_to(self.root, target_is_directory=True)
        with self.assertRaises(Exception):
            gate.context(self.env | {"RUNNER_TEMP": str(link)})

    def oidc(self, identity="plan", **overrides):
        claims = {
            "repository_id": "340900071", "repository_owner_id": "54093651",
            "environment": f"terraform-dev-{identity}", "ref": "refs/heads/feature/gcp-terraform-iac",
            "workflow_ref": self.env["GITHUB_WORKFLOW_REF"],
            "job_workflow_ref": "kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@refs/heads/feature/gcp-terraform-iac",
            "event_name": "workflow_dispatch",
        }
        claims["sub"] = ":".join(f"{k}:{v.replace(':', '%3A')}" for k, v in claims.items())
        claims.update(workflow_sha=self.env["GITHUB_SHA"], job_workflow_sha=self.env["GITHUB_SHA"], run_attempt="1")
        claims.update(overrides)
        encoded = base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip("=")
        return "header." + encoded + ".DUMMY_SIGNATURE"

    def response(self, raw, length=None):
        result = io.BytesIO(raw)
        result.status = 200
        result.headers = {} if length is None else {"Content-Length": str(length)}
        return result

    def test_exact_oidc_for_both_identities_and_cross_environment_same_sha_and_attempt_denials(self):
        for identity in ("plan", "apply"):
            env = self.env | {"RUNTIME_READ_IDENTITY": identity, "GITHUB_JOB": "runtime_metadata_read_" + identity,
                              "GCP_SMOKE_SERVICE_ACCOUNT": f"terraform-dev-{identity}@{gate.PROJECT}.iam.gserviceaccount.com"}
            for overrides in ({}, {"environment": "terraform-dev-other"}, {"workflow_sha": "b" * 40},
                              {"job_workflow_sha": "b" * 40}, {"repository_id": "0"}, {"repository_owner_id": "0"},
                              {"run_attempt": "2"}, {"sub": "untrusted"}):
                token = self.oidc(identity, **overrides)
                response = self.response(json.dumps({"value": token}).encode())
                with patch.object(gate.urllib.request, "build_opener") as opener:
                    opener.return_value.open.return_value = response
                    if overrides:
                        with self.assertRaises(Exception):
                            gate.identity(env)
                    else:
                        gate.identity(env)
                    self.assertIsInstance(opener.call_args.args[0], gate.NoRedirect)
        # Extracting the shared checker retained the existing safe plan default.
        encoded = self.oidc("apply").split(".")[1]
        claims = json.loads(base64.urlsafe_b64decode(encoded + "=" * (-len(encoded) % 4)))
        with self.assertRaises(Exception):
            gate.verify_oidc_claims(claims, self.env)

    def test_http_transport_is_bounded_redirect_free_and_never_reads_error_body(self):
        for raw, length in ((b'{"a":1}', None), (b'{}', gate.MAX_BYTES + 1), (b'{}', 3), (b'x' * (gate.MAX_BYTES + 1), None)):
            with patch.object(gate.urllib.request, "build_opener") as opener:
                opener.return_value.open.return_value = self.response(raw, length)
                if raw == b'{"a":1}':
                    self.assertEqual(gate.google("v1/dummy", "DUMMY_TOKEN", host="iam.googleapis.com", method="GET"), (200, {"a": 1}))
                else:
                    with self.assertRaises(Exception):
                        gate.google("v1/dummy", "DUMMY_TOKEN", host="iam.googleapis.com", method="GET")
                self.assertIsInstance(opener.call_args.args[0], gate.NoRedirect)
        with patch.object(gate.urllib.request, "build_opener") as opener:
            opener.return_value.open.return_value = self.response(b'{}')
            policy_path = "v1/projects/dummy/serviceAccounts/dummy@appspot.gserviceaccount.com:getIamPolicy?options.requestedPolicyVersion=3"
            gate.google(policy_path, "DUMMY_TOKEN", host="iam.googleapis.com", method="POST")
            sent = opener.return_value.open.call_args.args[0]
            self.assertEqual(sent.get_method(), "POST")
            self.assertIsNone(sent.data)  # REST requires an empty HTTP request body.
            self.assertEqual(sent.full_url, "https://iam.googleapis.com/" + policy_path)
        error_body = io.BytesIO(b"DUMMY_PRIVATE_ERROR")
        error = urllib.error.HTTPError("https://dummy.invalid", 403, "DUMMY_PRIVATE_ERROR", {}, error_body)
        with patch.object(gate.urllib.request, "build_opener") as opener, patch.object(error, "read", side_effect=AssertionError("error body must not be consumed")):
            opener.return_value.open.side_effect = error
            self.assertEqual(gate.google("v1/dummy", "DUMMY_TOKEN", host="iam.googleapis.com", method="GET"), (403, {}))

    def test_cli_errors_are_fixed_alias_status_only(self):
        out, err = io.StringIO(), io.StringIO()
        with patch.dict(os.environ, self.env, clear=True), patch.object(gate, "read", side_effect=RuntimeError("DUMMY_PRIVATE_BODY_URL_IDENTITY_CEL")), contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            self.assertEqual(gate.main(["--phase", "read"]), 3)
        self.assertEqual(out.getvalue(), "")
        self.assertEqual(err.getvalue(), "runtime-metadata-read: STOP\n")


if __name__ == "__main__":
    unittest.main()
