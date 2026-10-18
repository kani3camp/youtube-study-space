#!/usr/bin/env python3
"""Dummy-only independent authority observations; no real credentials or APIs."""
import contextlib
import copy
import io
import os
import unittest
from unittest.mock import patch

import terraform_runtime_iam_authority_read as gate
import test_terraform_runtime_metadata_read as fixtures


class AuthorityReadTest(unittest.TestCase):
    def setUp(self):
        self.fixture = fixtures.RuntimeReadTest()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.env = self.fixture.env | {"DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED": "true",
                                       "AUTHORITY_AUTH_OUTCOME": "success", "AUTHORITY_JOB_STATUS": "failure"}
        self.root = self.fixture.root
        self.calls = []
        self.responses = {alias: {"permissions": [permission for _, permission in group]} for alias, group in gate.GROUPS.items()}
        self.statuses = {}

    def request(self, path, token, body=None, *, host, method):
        self.assertEqual(token, "DUMMY_PRIVATE_TOKEN")
        self.calls.append((host, path, body, method))
        pool = self.fixture.pool
        expected = {
            "authority-project": ("cloudresourcemanager.googleapis.com", "v1/projects/" + gate.metadata.PROJECT, None, "GET"),
            "authority-project-roles": ("cloudresourcemanager.googleapis.com", "v3/projects/" + gate.metadata.PROJECT + ":testIamPermissions", {"permissions": ["iam.roles.create", "iam.roles.get", "iam.roles.update"]}, "POST"),
            "authority-pool-policy": ("iam.googleapis.com", "v1/" + pool + ":testIamPermissions", {"permissions": ["iam.workloadIdentityPools.getIamPolicy", "iam.workloadIdentityPools.setIamPolicy"]}, "POST"),
            "authority-sa-policy": ("iam.googleapis.com", f"v1/projects/{gate.metadata.PROJECT}/serviceAccounts/{gate.metadata.PROJECT}@appspot.gserviceaccount.com:testIamPermissions", {"permissions": ["iam.serviceAccounts.getIamPolicy", "iam.serviceAccounts.setIamPolicy"]}, "POST"),
        }
        matches = [alias for alias, value in expected.items() if value == (host, path, body, method)]
        self.assertEqual(len(matches), 1)
        alias = matches[0]
        data = self.fixture.data["runtime-project"] if alias == "authority-project" else self.responses[alias]
        return self.statuses.get(alias, 200), copy.deepcopy(data)

    def run_read(self, request=None, **overrides):
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            gate.read(self.env | overrides, request=request or self.request)
        self.assertEqual(output.getvalue(), (self.root / "summary").read_text())
        return output.getvalue()

    def failure(self, request=None, **overrides):
        output = io.StringIO()
        with contextlib.redirect_stdout(output), self.assertRaises(Exception):
            gate.read(self.env | overrides, request=request or self.request)
        return output.getvalue()

    def test_project_first_four_fixed_calls_and_all_authority_flags_remain_false(self):
        output = self.run_read()
        self.assertEqual(len(self.calls), 4)
        self.assertIn("runtime-iam-authority-read: PASS", output)
        for alias in gate.PERMISSION_ALIASES:
            self.assertIn(alias + ": HELD", output)
        for alias in gate.FLAGS:
            self.assertIn(alias + ": false", output)
        self.assertEqual({path.name for path in self.root.iterdir()}, {"summary"})
        for private in ("DUMMY_", self.fixture.number, gate.metadata.PROJECT, "https://", "googleapis", "@", "iam.roles."):
            self.assertNotIn(private, output)

    def test_both_exact_environment_identities_and_failed_metadata_job_can_observe(self):
        for identity in ("plan", "apply"):
            (self.root / "summary").unlink(missing_ok=True)
            output = self.run_read(RUNTIME_READ_IDENTITY=identity, GITHUB_JOB="runtime_metadata_read_" + identity,
                GCP_SMOKE_SERVICE_ACCOUNT=f"terraform-dev-{identity}@{gate.metadata.PROJECT}.iam.gserviceaccount.com")
            self.assertIn("runtime-iam-authority-read: PASS", output)

    def test_closed_gates_failed_auth_cancel_and_invalid_context_stop_before_requests(self):
        for key, value in {
            "DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED": "false", "DEV_RUNTIME_METADATA_READ_ENABLED": "false",
            "AUTHORITY_AUTH_OUTCOME": "failure", "AUTHORITY_JOB_STATUS": "cancelled", "MODE": "apply",
            "RUNTIME_TARGET": "prod", "GITHUB_JOB": "apply", "GITHUB_RUN_ATTEMPT": "2", "GITHUB_SHA": "a" * 39,
            "GITHUB_REPOSITORY_ID": "0", "GITHUB_REPOSITORY_OWNER_ID": "0", "GITHUB_REF": "refs/heads/dev",
            "GITHUB_WORKFLOW_REF": "untrusted", "GCP_SMOKE_ACCESS_TOKEN": "", "GCP_SMOKE_SERVICE_ACCOUNT": "bad",
            "DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": "2000-01-01T00:00:00Z"}.items():
            with self.subTest(key=key):
                self.assertEqual(self.failure(**{key: value}), "")
        self.assertEqual(self.calls, [])

    def test_bad_project_scope_shape_or_denial_never_reaches_any_scope_post(self):
        for change in ({"projectNumber": "999999999999"}, {"projectId": "youtube-study-space"},
                       {"lifecycleState": "DELETE_REQUESTED"}, {"unknown": "DUMMY_PRIVATE_SENTINEL"}):
            self.fixture.data["runtime-project"] = {"projectId": gate.metadata.PROJECT, "projectNumber": self.fixture.number, "lifecycleState": "ACTIVE"} | change
            self.calls.clear()
            output = self.failure()
            self.assertEqual(len(self.calls), 1)
            self.assertIn("authority-project: STOP", output)
            self.assertIn("authority-pool-set-policy: 未実行", output)
            (self.root / "summary").unlink(missing_ok=True)
        self.statuses["authority-project"] = 403
        self.calls.clear()
        output = self.failure()
        self.assertEqual(len(self.calls), 1)
        self.assertIn("authority-project-reason: HTTP403", output)

    def test_successful_empty_response_only_means_not_returned(self):
        for empty in ({}, {"permissions": []}):
            self.responses = dict.fromkeys(gate.GROUPS, empty)
            (self.root / "summary").unlink(missing_ok=True)
            output = self.run_read()
            for alias in gate.PERMISSION_ALIASES:
                self.assertIn(alias + ": NOT_RETURNED", output)
            self.assertIn("authority-writer-ready: false", output)

    def test_poisoned_scope_response_is_all_unknown_without_repair_from_other_scopes(self):
        variants = [None, [], {"permissions": None}, {"permissions": "iam.roles.get"},
                    {"permissions": [None]}, {"permissions": ["DUMMY_PRIVATE_UNREQUESTED"]},
                    {"permissions": ["iam.roles.create", "iam.roles.create"]},
                    {"permissions": ["iam.roles.create"], "unknown": "DUMMY_PRIVATE_SECRET"},
                    {"error": {"message": "DUMMY_PRIVATE_ERROR"}}]
        for value in variants:
            self.responses["authority-project-roles"] = value
            (self.root / "summary").unlink(missing_ok=True)
            output = self.failure()
            for alias, _ in gate.GROUPS["authority-project-roles"]:
                self.assertIn(alias + ": UNKNOWN", output)
            self.assertIn("authority-project-roles-reason: RESPONSE_REJECTED", output)
            self.assertIn("authority-pool-set-policy: HELD", output)
            self.assertNotIn("DUMMY_", output)

    def test_scope_http_failure_is_independent_and_never_inferred_from_transport(self):
        for status, expected in ((403, "HTTP403"), (400, "HTTP_OTHER")):
            (self.root / "summary").unlink(missing_ok=True)
            self.statuses["authority-pool-policy"] = status
            output = self.failure()
            self.assertIn("authority-pool-policy-reason: " + expected, output)
            self.assertIn("authority-pool-get-policy: UNKNOWN", output)
            self.assertIn("authority-sa-set-policy: HELD", output)
        self.statuses.clear()
        def request(path, token, body=None, *, host, method):
            if "workloadIdentityPools/" in path:
                raise RuntimeError("DUMMY_PRIVATE_TRANSPORT_BODY_URL_IDENTITY")
            return self.request(path, token, body, host=host, method=method)
        (self.root / "summary").unlink(missing_ok=True)
        output = self.failure(request)
        self.assertIn("authority-pool-policy-reason: TRANSPORT_FAILED", output)
        self.assertNotIn("HTTP403", output)
        self.assertNotIn("DUMMY_", output)

    def test_context_or_window_change_before_scope_posts_stops_all_further_calls(self):
        for key, value in (("DEV_RUNTIME_METADATA_READ_EXPIRES_UTC", "2000-01-01T00:00:00Z"),
                           ("AUTHORITY_JOB_STATUS", "cancelled"), ("DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED", "false")):
            env = self.env.copy()
            self.calls.clear()
            (self.root / "summary").unlink(missing_ok=True)
            def request(path, token, body=None, *, host, method):
                result = self.request(path, token, body, host=host, method=method)
                env[key] = value
                return result
            output = io.StringIO()
            with contextlib.redirect_stdout(output), self.assertRaises(Exception):
                gate.read(env, request=request)
            self.assertEqual(len(self.calls), 1)
            self.assertIn("authority-window: STOP", output.getvalue())
            self.assertIn("authority-pool-get-policy: 未実行", output.getvalue())

    def test_final_context_revalidation_prevents_success_after_last_response(self):
        env = self.env.copy()
        def request(path, token, body=None, *, host, method):
            result = self.request(path, token, body, host=host, method=method)
            if "/serviceAccounts/" in path:
                env["DEV_RUNTIME_METADATA_READ_EXPIRES_UTC"] = "2000-01-01T00:00:00Z"
            return result
        output = io.StringIO()
        with contextlib.redirect_stdout(output), self.assertRaises(Exception):
            gate.read(env, request=request)
        self.assertEqual(len(self.calls), 4)
        self.assertIn("authority-sa-policy: STOP", output.getvalue())
        self.assertIn("authority-sa-policy-reason: ADMISSION_STOP", output.getvalue())
        self.assertIn("authority-sa-get-policy: UNKNOWN", output.getvalue())
        self.assertIn("authority-sa-set-policy: UNKNOWN", output.getvalue())
        self.assertIn("authority-window: STOP", output.getvalue())
        self.assertIn("runtime-iam-authority-read: STOP", output.getvalue())

    def test_expiry_only_at_publication_keeps_timepoint_observations_but_never_readiness(self):
        statuses = dict.fromkeys(gate.REQUESTS, "PASS")
        reasons = dict.fromkeys(gate.REQUESTS, "PASS")
        observations = dict.fromkeys(gate.PERMISSION_ALIASES, "HELD")
        env = self.env | {"DEV_RUNTIME_METADATA_READ_EXPIRES_UTC": "2000-01-01T00:00:00Z"}
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            self.assertFalse(gate.publish(env, self.fixture.number, statuses, reasons, observations, "PASS"))
        self.assertIn("runtime-iam-authority-read: STOP", output.getvalue())
        self.assertIn("authority-window: STOP", output.getvalue())
        self.assertIn("authority-role-create: HELD", output.getvalue())
        for alias in gate.FLAGS:
            self.assertIn(alias + ": false", output.getvalue())

    def test_summary_sink_failure_preserves_only_fixed_observations_and_overall_stop(self):
        output, error = io.StringIO(), io.StringIO()
        with patch.dict(os.environ, self.env, clear=True), patch.object(gate.metadata, "google", side_effect=self.request), patch.object(gate.os, "open", side_effect=OSError("DUMMY_PRIVATE_PATH")), contextlib.redirect_stdout(output), contextlib.redirect_stderr(error):
            self.assertEqual(gate.main([]), 3)
        self.assertIn("runtime-iam-authority-read: STOP", output.getvalue())
        self.assertIn("authority-role-create: HELD", output.getvalue())
        self.assertNotIn("DUMMY_", output.getvalue() + error.getvalue())

    def test_public_sink_rejects_unknown_alias_or_private_status_reason_or_permission(self):
        statuses = dict.fromkeys(gate.REQUESTS, "PASS")
        reasons = dict.fromkeys(gate.REQUESTS, "PASS")
        observations = dict.fromkeys(gate.PERMISSION_ALIASES, "HELD")
        for args in ((statuses | {"DUMMY_PRIVATE_ALIAS": "PASS"}, reasons, observations),
                     (statuses | {gate.REQUESTS[0]: "DUMMY_PRIVATE_STATUS"}, reasons, observations),
                     (statuses, reasons | {gate.REQUESTS[0]: "DUMMY_PRIVATE_REASON"}, observations),
                     (statuses, reasons, observations | {gate.PERMISSION_ALIASES[0]: "DUMMY_PRIVATE_VALUE"})):
            output = io.StringIO()
            with contextlib.redirect_stdout(output), self.assertRaises(Exception):
                gate.publish(self.env, self.fixture.number, *args, "PASS")
            self.assertEqual(output.getvalue(), "")
            self.assertFalse((self.root / "summary").exists())

    def test_argparse_exception_and_summary_symlink_do_not_print_private_values(self):
        output, error = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(output), contextlib.redirect_stderr(error):
            self.assertEqual(gate.main(["DUMMY_PRIVATE_ARGV"]), 3)
        self.assertEqual(output.getvalue(), "")
        self.assertEqual(error.getvalue(), "runtime-iam-authority-read: STOP\n")
        target = self.root / "target"
        target.write_text("DUMMY_PRIVATE_TARGET")
        (self.root / "summary").symlink_to(target)
        self.assertEqual(self.failure(), "")
        self.assertEqual(target.read_text(), "DUMMY_PRIVATE_TARGET")
        self.assertEqual(self.calls, [])


if __name__ == "__main__":
    unittest.main()
