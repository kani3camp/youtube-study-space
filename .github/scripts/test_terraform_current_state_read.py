#!/usr/bin/env python3
"""Public dummy metadata-only read contracts; no cloud credentials or writes."""
from __future__ import annotations

import contextlib
import copy
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import terraform_current_state_read as gate
import test_terraform_history_plan_receipt as state_fixtures
import test_gcp_user_activity_schema_audit_workflow as history_fixtures


class CurrentStateReadTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.env = {
            "RUNNER_TEMP": str(self.root), "GITHUB_STEP_SUMMARY": str(self.root / "summary"),
            "GITHUB_SHA": "a" * 40, "MODE": "current-state-read", "HISTORY_TARGET": "dev",
            "DEV_CURRENT_STATE_READ_ENABLED": "true", "TF_VAR_project_id": gate.PROJECT,
            "TF_VAR_manage_user_activity_history": "true", "TF_WORKSPACE": "default",
            "STATE_KEY": gate.private.STATE_KEY, "STATE_AWS_REGION": "ap-northeast-1",
            "STATE_ACCOUNT_ID": "111111111111", "STATE_BUCKET": "dummy-state-bucket",
            "AWS_MAX_ATTEMPTS": "1", "AWS_RETRY_MODE": "standard", "GCP_SMOKE_ACCESS_TOKEN": "DUMMY_TOKEN",
            "GCP_SMOKE_SERVICE_ACCOUNT": f"terraform-dev-plan@{gate.PROJECT}.iam.gserviceaccount.com",
            "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_REPOSITORY": "kani3camp/youtube-study-space",
            "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
            "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac", "GITHUB_RUN_ATTEMPT": "1",
            "GITHUB_JOB": "current_state_read",
            "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
        }
        source = history_fixtures.UserActivityHistoryPlanTest()
        source.setUp()
        plan, self.table = source.fixture(False)
        self.table.update(etag="DUMMY_ETAG", numRows="42", numBytes="84", lastModifiedTime="123456789")
        self.project = {"projectId": gate.PROJECT, "projectNumber": "222222222222",
                        "lifecycleState": "ACTIVE", "name": "DUMMY_PROJECT_NAME"}
        self.state = state_fixtures.state_fixture()
        self.state["resources"].append({
            "module": "module.user_activity_history[0]", "mode": "managed",
            "type": "google_bigquery_table", "name": "retained",
            "provider": 'provider["registry.terraform.io/hashicorp/google"]',
            "instances": [{"schema_version": 0,
                           "attributes": copy.deepcopy(plan["resource_changes"][-1]["change"]["after"])}],
        })
        self.aws = state_fixtures.FakeS3()
        self.aws.body = json.dumps(self.state).encode()
        self.google_calls = []

    def google(self, path, token, *, host, method):
        self.google_calls.append((path, token, host, method))
        self.assertEqual((token, method), ("DUMMY_TOKEN", "GET"))
        if host == "bigquery.googleapis.com":
            self.assertEqual(path, gate.adopted.TABLE_PATH)
            return 200, copy.deepcopy(self.table)
        self.assertEqual((host, path), ("cloudresourcemanager.googleapis.com", gate.PROJECT_PATH))
        return 200, copy.deepcopy(self.project)

    def run_read(self, **kwargs):
        return gate.read(self.env, request=kwargs.get("request", self.aws),
                         metadata_request=kwargs.get("metadata_request", self.google))

    def assert_private_output_absent(self):
        summary = (self.root / "summary").read_text()
        for private in ("DUMMY_", "111111111111", "222222222222", "dummy-state-bucket",
                        gate.PROJECT, gate.private.STATE_KEY, self.state["lineage"], "ownership-state",
                        "no-op", "quiescent", "attested"):
            self.assertNotIn(private, summary)
        self.assertFalse(any((self.root / name).exists() for name in gate.FILES))
        return summary

    def test_exact_bounded_state12_and_canonical_metadata_pass_using_only_reads(self):
        with patch.object(gate.private, "load_policy", side_effect=AssertionError("admission forbidden")), \
             patch.object(gate.adopted, "context", side_effect=AssertionError("no-op context forbidden")):
            self.run_read()
        summary = self.assert_private_output_absent()
        self.assertIn("Current-state metadata read: PASS", summary)
        self.assertIn("Managed resources: 12", summary)
        self.assertIn("Canonical columns: 8", summary)
        self.assertEqual([a[:2] for a in self.aws.calls], [
            ("s3api", "head-object"), ("s3api", "get-object"), ("s3api", "head-object"),
            ("s3api", "list-objects-v2"), ("s3api", "list-objects-v2"),
            ("s3api", "head-object"), ("s3api", "get-object"), ("s3api", "head-object"),
            ("s3api", "list-objects-v2"), ("s3api", "list-objects-v2"),
        ])
        for args in self.aws.calls:
            self.assertEqual(args[args.index("--expected-bucket-owner") + 1], "111111111111")
            if "--key" in args:
                self.assertEqual(args[args.index("--key") + 1], gate.private.STATE_KEY)
            if args[1] == "get-object":
                self.assertEqual(args[args.index("--range") + 1], "bytes=0-4194304")
        self.assertEqual(len(self.google_calls), 4)
        self.assertEqual(list(self.root.iterdir()), [self.root / "summary"])

    def test_closed_gate_wrong_context_or_combined_admission_stops_before_requests(self):
        deviations = {
            "DEV_CURRENT_STATE_READ_ENABLED": "false", "MODE": "plan", "HISTORY_TARGET": "prod",
            "TF_VAR_project_id": "youtube-study-space", "TF_VAR_manage_user_activity_history": "false",
            "TF_WORKSPACE": "other", "STATE_KEY": "youtube-study-space/prod/terraform.tfstate",
            "STATE_ACCOUNT_ID": "invalid", "AWS_MAX_ATTEMPTS": "2", "AWS_RETRY_MODE": "adaptive",
            "GITHUB_EVENT_NAME": "pull_request", "GITHUB_REPOSITORY_ID": "1",
            "GITHUB_REPOSITORY_OWNER_ID": "1", "GITHUB_REF": "refs/heads/dev",
            "GITHUB_RUN_ATTEMPT": "2", "GITHUB_JOB": "plan", "GITHUB_SHA": "b" * 39,
            "GITHUB_WORKFLOW_REF": "untrusted", "PLAN_SAFETY_EVIDENCE": '{"state_writers_quiescent":true}',
            "PLAN_COST_EVIDENCE": "{}", "HISTORY_IMPORT_APPROVAL": "{}", "OWNERSHIP_WAVE": "pool",
            "HISTORY_POST_NOOP": "true", "CURRENT_STATE_ATTESTATION": "true",
            "OWNERSHIP_ROOT_BASIS": "current-state-bootstrap", "GCP_SMOKE_ACCESS_TOKEN": "",
        }
        for key, value in deviations.items():
            with self.subTest(key=key), self.assertRaises(Exception):
                gate.read(self.env | {key: value}, request=self.aws, metadata_request=self.google)
        self.assertEqual((self.aws.calls, self.google_calls), ([], []))

    def test_missing_apply_or_unrelated_gcp_identity_stops_before_authentication(self):
        for service_account in (None, "", f"terraform-dev-apply@{gate.PROJECT}.iam.gserviceaccount.com",
                                f"dummy-unrelated@{gate.PROJECT}.iam.gserviceaccount.com",
                                "terraform-dev-plan@dummy-other-project.iam.gserviceaccount.com"):
            with self.subTest(service_account=service_account), self.assertRaises(Exception):
                gate.context(self.env | {"GCP_SMOKE_SERVICE_ACCOUNT": service_account})
        self.assertEqual((self.aws.calls, self.google_calls), ([], []))

    def test_oversized_or_unsafe_head_never_downloads_state(self):
        for changes in ({"ContentLength": gate.MAX_BYTES + 1}, {"ContentLength": True}, {"VersionId": "null"},
                        {"ContentLength": 0}, {"ServerSideEncryption": "none"}, {"DeleteMarker": True}):
            with self.subTest(changes=changes):
                self.aws.calls.clear()
                self.aws.overrides["head-object"] = changes
                with self.assertRaises(Exception):
                    self.run_read()
                self.assertNotIn("get-object", [c[1] for c in self.aws.calls])
                self.assertEqual([c[1] for c in self.aws.calls].count("list-objects-v2"), 4)
                self.assertIn("Current-state metadata read: STOP", self.assert_private_output_absent())

    def test_state_shape_schema_and_provider_deviations_stop(self):
        variants = []
        for mutate in (
            lambda s: s["resources"].pop(),
            lambda s: s["resources"].append(copy.deepcopy(s["resources"][-1])),
            lambda s: s.update(terraform_version="1.0.0"),
            lambda s: s["resources"][0].update(provider='provider["registry.terraform.io/hashicorp/aws"]'),
            lambda s: s["resources"][-1]["instances"][0]["attributes"].update(deletion_protection=False),
            lambda s: s["resources"][-1]["instances"][0]["attributes"].update(schema="[]"),
        ):
            variant = copy.deepcopy(self.state)
            mutate(variant)
            variants.append(json.dumps(variant).encode())
        for raw in variants + [b"{", b'{"version":4,"version":4}', b"NaN"]:
            with self.subTest(raw=raw[:10]), self.assertRaises(Exception):
                self.aws.body = raw
                self.run_read()
            self.assertIn("Current-state metadata read: STOP", self.assert_private_output_absent())

    def test_state_version_rawbytes_and_head_metadata_races_stop(self):
        for failure in ("version", "bytes", "head-metadata", "get-version", "within-snapshot"):
            with self.subTest(failure=failure):
                child = CurrentStateReadTests()
                child.setUp()
                try:
                    heads = 0
                    def aws(*args):
                        nonlocal heads
                        if args[1] == "head-object":
                            heads += 1
                            threshold = 2 if failure == "within-snapshot" else 3
                            if heads == threshold:
                                if failure in {"version", "within-snapshot"}:
                                    child.aws.version = "DUMMY_OTHER_VERSION"
                                elif failure == "bytes":
                                    child.aws.body += b"\n"
                                elif failure == "head-metadata":
                                    child.aws.overrides["head-object"] = {"Metadata": {"dummy": "PRIVATE_METADATA"}}
                        if failure == "get-version":
                            child.aws.overrides["get-object"] = {"VersionId": "DUMMY_WRONG_GET_VERSION"}
                        return child.aws(*args)
                    with self.assertRaises(Exception):
                        child.run_read(request=aws)
                    self.assertIn("Current-state metadata read: STOP", child.assert_private_output_absent())
                    self.assertEqual([c[1] for c in child.aws.calls].count("list-objects-v2"), 4)
                finally:
                    child.doCleanups()

    def test_table_schema_and_project_failures_never_suppress_other_metadata_reads(self):
        for failure in ("table-denied", "table-legacy", "table-unknown-field", "table-oversized",
                        "project-denied", "project-other", "project-deleted"):
            with self.subTest(failure=failure):
                calls = []
                def google(path, token, *, host, method):
                    calls.append(host)
                    status, value = self.google(path, token, host=host, method=method)
                    if host == "bigquery.googleapis.com":
                        if failure == "table-denied":
                            return 403, {"error": "DUMMY_PRIVATE_ERROR"}
                        if failure == "table-legacy":
                            value["schema"]["fields"].append({"name": "timestamp", "type": "TIMESTAMP"})
                        elif failure == "table-unknown-field":
                            value["schema"]["fields"][0]["dummyUnknown"] = "DUMMY_PRIVATE_SCHEMA"
                        elif failure == "table-oversized":
                            value["dummy"] = "x" * gate.MAX_BYTES
                    else:
                        if failure == "project-denied":
                            return 403, {"error": "DUMMY_PRIVATE_ERROR"}
                        if failure == "project-other":
                            value["projectId"] = "dummy-other-project"
                        elif failure == "project-deleted":
                            value["lifecycleState"] = "DELETE_REQUESTED"
                    return status, value
                with self.assertRaises(Exception):
                    self.run_read(metadata_request=google)
                self.assertEqual(calls, ["bigquery.googleapis.com", "cloudresourcemanager.googleapis.com"] * 2)
                self.assertIn("Current-state metadata read: STOP", self.assert_private_output_absent())

    def test_configuration_changes_stop_but_documented_volatile_table_observations_may_change(self):
        for failure in ("stable-table", "unknown-table", "project"):
            with self.subTest(failure=failure):
                calls = 0
                def google(path, token, *, host, method):
                    nonlocal calls
                    calls += 1
                    status, value = self.google(path, token, host=host, method=method)
                    if calls >= 3:
                        if host == "bigquery.googleapis.com":
                            value["creationTime" if failure == "stable-table" else "dummyUnknown" if failure == "unknown-table" else "etag"] = "DUMMY_NEW_VALUE"
                        elif failure == "project":
                            value["name"] = "DUMMY_NEW_PROJECT_NAME"
                    return status, value
                with self.assertRaises(Exception):
                    self.run_read(metadata_request=google)
                self.assertIn("Current-state metadata read: STOP", self.assert_private_output_absent())
        calls = 0
        def volatile(path, token, *, host, method):
            nonlocal calls
            calls += 1
            status, value = self.google(path, token, host=host, method=method)
            if calls == 3:
                value.update(etag="DUMMY_NEW_ETAG", numRows="43", numBytes="90", lastModifiedTime="987654321")
            return status, value
        self.run_read(metadata_request=volatile)

    def test_lock_workspace_and_dependency_failures_stop_with_independent_absence_reads(self):
        for failure in ("lock", "workspace", "get-denied", "list-denied", "network"):
            with self.subTest(failure=failure):
                self.aws.calls.clear()
                self.aws.lock, self.aws.workspaces = failure == "lock", failure == "workspace"
                self.aws.denied = "get-object" if failure == "get-denied" else "list-objects-v2" if failure == "list-denied" else None
                self.aws.network = failure == "network"
                with self.assertRaises(Exception):
                    self.run_read()
                self.assertEqual([c[1] for c in self.aws.calls].count("list-objects-v2"), 4)
                self.assertIn("Current-state metadata read: STOP", self.assert_private_output_absent())

    def test_private_staging_is_exclusive_owned0600_and_cleanup_is_mandatory(self):
        actual = gate.private.write_private
        def write(path, value):
            actual(path, value)
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
        with patch.object(gate.private, "write_private", side_effect=write):
            self.run_read()
        prior = self.root / gate.FILES[0]
        prior.write_text("DUMMY_EXISTING_PRIVATE")
        with self.assertRaises(Exception):
            self.run_read()
        self.assertEqual(prior.read_text(), "DUMMY_EXISTING_PRIVATE")
        prior.unlink()
        outside = self.root / "private-original"
        outside.write_text("DUMMY_SYMLINK_TARGET")
        prior.symlink_to(outside)
        with self.assertRaises(Exception):
            self.run_read()
        self.assertEqual(outside.read_text(), "DUMMY_SYMLINK_TARGET")
        prior.unlink()
        with patch.object(gate, "cleanup", side_effect=OSError("DUMMY_PRIVATE_CLEANUP_ERROR")), self.assertRaises(Exception):
            self.run_read()
        self.assertIn("Private temporary files removed: STOP", (self.root / "summary").read_text())
        gate.cleanup(self.root)

    def test_repository_or_symlink_temp_is_rejected_without_requests(self):
        alias = self.root / "alias"
        alias.symlink_to(self.root, target_is_directory=True)
        for target in (str(gate.ROOT), str(alias), "relative"):
            with self.subTest(target=target), self.assertRaises(Exception):
                gate.read(self.env | {"RUNNER_TEMP": target}, request=self.aws, metadata_request=self.google)
        self.assertEqual((self.aws.calls, self.google_calls), ([], []))

    def test_cli_suppresses_private_errors_and_accepts_no_private_admission_payload(self):
        stdout, stderr = io.StringIO(), io.StringIO()
        with patch.dict(os.environ, self.env, clear=True), \
             patch.object(gate, "google", side_effect=ValueError("DUMMY_PRIVATE_API_BODY")), \
             patch.object(gate.private, "aws", side_effect=self.aws), \
             contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            self.assertEqual(gate.main(["--phase", "read"]), 3)
            self.assertEqual(gate.main(["--private", "DUMMY_PRIVATE_INPUT"]), 3)
        self.assertNotIn("DUMMY_", stdout.getvalue() + stderr.getvalue())
        self.assertNotIn("Traceback", stderr.getvalue())
        self.assertIn("diagnostic suppressed", stderr.getvalue())

    def test_identity_phase_uses_exact_existing_read_only_claims_and_immutable_role_id(self):
        with patch.object(gate, "verify_oidc") as oidc, patch.object(gate, "verify_aws_identity") as aws:
            gate.identity(self.env)
        oidc.assert_called_once_with(self.env, plan_read_only=True)
        aws.assert_called_once_with(self.env, require_role_id=True)
        with patch.object(gate, "verify_oidc", side_effect=ValueError("DUMMY_PRIVATE_CLAIM")), \
             patch.object(gate, "verify_aws_identity") as aws, self.assertRaises(Exception):
            gate.identity(self.env)
        aws.assert_not_called()


if __name__ == "__main__":
    unittest.main()
