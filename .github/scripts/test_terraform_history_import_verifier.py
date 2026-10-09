#!/usr/bin/env python3
"""Synthetic one-shot import receipts; no cloud credentials or real state."""
import copy
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import terraform_history_import_verifier as gate
import terraform_history_plan_receipt as receipt
from terraform_plan_summary import build_summary
import test_terraform_history_plan_receipt as state_fixtures
import test_gcp_user_activity_schema_audit_workflow as plan_fixtures

SHA = "a" * 40


class ImportVerifierTest(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        fixture = plan_fixtures.UserActivityHistoryPlanTest()
        fixture.setUp()
        self.plan, self.metadata = fixture.fixture()
        self.plan["terraform_version"] = "1.16.4"
        self.metadata.update(etag="DUMMY_ETAG", numRows="42", numBytes="84", lastModifiedTime="123456789")
        self.aws = state_fixtures.FakeS3()
        now = datetime.now(timezone.utc)
        self.approved = {"target": "dev", "git_sha": SHA, "plan_run_id": 123456,
                         "cost_evidence_sha256": "b" * 64, "max_added_current_month_usd": "0.25",
                         "issued_utc": (now - timedelta(minutes=2)).strftime("%Y-%m-%dT%H:%M:%SZ"),
                         "expires_utc": (now + timedelta(hours=1)).strftime("%Y-%m-%dT%H:%M:%SZ")}
        summary = build_summary(self.plan, environment="dev", git_sha=SHA, policy="import-only")
        summary_raw = (json.dumps(summary, sort_keys=True, separators=(",", ":")) + "\n").encode()
        self.approved["summary_sha256"] = hashlib.sha256(summary_raw).hexdigest()
        self.summary_raw = summary_raw
        self.env = {"MODE": "apply", "HISTORY_TARGET": "dev", "GITHUB_SHA": SHA,
                    "TF_VAR_project_id": "test-youtube-study-space", "TF_VAR_manage_user_activity_history": "true",
                    "TF_WORKSPACE": "default", "STATE_KEY": receipt.STATE_KEY, "STATE_AWS_REGION": "ap-northeast-1",
                    "STATE_ACCOUNT_ID": "111111111111", "STATE_BUCKET": "dummy-state-bucket",
                    "AWS_MAX_ATTEMPTS": "1", "AWS_RETRY_MODE": "standard", "RUNNER_TEMP": str(self.root),
                    "GITHUB_OUTPUT": str(self.root / "output"), "GITHUB_STEP_SUMMARY": str(self.root / "summary"),
                    "GCP_SMOKE_ACCESS_TOKEN": "DUMMY_TOKEN", "APPLY_OUTCOME": "success", "POST_PLAN_OUTCOME": "success",
                    "APPROVED_SUMMARY_DIGEST": self.approved["summary_sha256"]}
        self.env["HISTORY_IMPORT_APPROVAL"] = json.dumps(self.approved)
        (self.root / "output").write_text("")
        receipt.write_private(self.root / "user-history-before.json", self.metadata)

    def before(self):
        gate.before(self.env, request=self.aws)

    def preapply(self):
        receipt.write_private(self.root / "sanitized-replan.json", json.loads(self.summary_raw))
        (self.root / "tfplan").write_bytes(b"DUMMY_SAVED_PLAN")
        (self.root / "tfplan").chmod(0o600)
        gate.preapply(self.env, request=self.aws)

    def imported_state(self):
        state = state_fixtures.state_fixture()
        state["serial"] += 1
        state["resources"].append({"module": "module.user_activity_history[0]", "mode": "managed",
                                   "type": "google_bigquery_table", "name": "retained",
                                   "provider": 'provider["registry.terraform.io/hashicorp/google"]',
                                   "instances": [{"schema_version": 0, "attributes": copy.deepcopy(self.plan["resource_changes"][-1]["change"]["after"])}]})
        state["check_results"].append({"object_kind": "resource",
                                       "config_addr": "module.user_activity_history.google_bigquery_table.retained",
                                       "status": "pass", "objects": [{"object_addr": gate.TABLE, "status": "pass"}]})
        return state

    def google(self, path, token, **kwargs):
        self.assertEqual((path, token, kwargs), (receipt.TABLE_PATH, "DUMMY_TOKEN", {"host": "bigquery.googleapis.com"}))
        return 200, copy.deepcopy(self.metadata)

    def test_exact_import_success_and_read_only_calls(self):
        self.before()
        self.preapply()
        self.aws.body = json.dumps(self.imported_state()).encode()
        gate.post(self.env, request=self.aws, metadata_request=self.google)
        self.assertIn("state=imported; metadata=PASS; volatile=unchanged; lock=absent", (self.root / "summary").read_text())
        self.assertEqual((self.root / "output").read_text(),
                         "history_plan_sha256=" + hashlib.sha256(b"DUMMY_SAVED_PLAN").hexdigest() + "\n")
        self.assertTrue(all(call[:2] in {("s3api", "head-object"), ("s3api", "get-object"),
                                             ("s3api", "list-objects-v2")} for call in self.aws.calls))
        self.assertNotIn("DUMMY_STATE_PAYLOAD", (self.root / "summary").read_text())

    def test_approval_fails_closed_on_target_sha_digest_time_and_mode(self):
        for change in ({"target": "prod"}, {"git_sha": "b" * 40}, {"summary_sha256": "x" * 64},
                       {"expires_utc": "2020-01-01T00:00:00Z"}, {"issued_utc": "2090-01-01T00:00:00Z"},
                       {"plan_run_id": 0}):
            with self.subTest(change=change), self.assertRaises(Exception):
                gate.approval(dict(self.env, HISTORY_IMPORT_APPROVAL=json.dumps(self.approved | change)))
        with self.assertRaises(Exception):
            gate.approval(dict(self.env, MODE="plan"))
        with self.assertRaises(Exception):
            gate.approval(dict(self.env, HISTORY_IMPORT_APPROVAL=self.env["HISTORY_IMPORT_APPROVAL"][:-1] + ',"target":"dev"}'))

    def test_actual_cli_with_dummy_approval_is_value_free(self):
        script = Path(gate.__file__)
        for value, expected in ((self.env["HISTORY_IMPORT_APPROVAL"], 0),
                                (self.env["HISTORY_IMPORT_APPROVAL"][:-1] + ',"target":"dev"}', 3)):
            result = subprocess.run(["python3", str(script), "--phase", "authorization"],
                                    env=os.environ | self.env | {"HISTORY_IMPORT_APPROVAL": value},
                                    capture_output=True, text=True)
            self.assertEqual(result.returncode, expected)
            self.assertNotIn("DUMMY_TOKEN", result.stdout + result.stderr)
            self.assertNotIn(str(self.root), result.stdout + result.stderr)

    def test_preapply_requires_unimported_state_exact_one_and_same_plan_digest(self):
        self.aws.body = json.dumps(self.imported_state()).encode()
        with self.assertRaises(Exception):
            self.before()
        (self.root / "history-import-state-before.json").unlink()
        self.aws.body = json.dumps(state_fixtures.state_fixture()).encode()
        self.before()
        self.plan["resource_changes"][-1]["change"].pop("importing")
        bad = build_summary(self.plan, environment="dev", git_sha=SHA, policy="import-only")
        receipt.write_private(self.root / "sanitized-replan.json", bad)
        (self.root / "tfplan").write_bytes(b"DUMMY_SAVED_PLAN")
        (self.root / "tfplan").chmod(0o600)
        with self.assertRaises(Exception):
            gate.preapply(self.env, request=self.aws)

    def test_normal_writer_metrics_can_change_without_authorizing_stable_metadata_changes(self):
        self.before()
        self.preapply()
        self.aws.body = json.dumps(self.imported_state()).encode()
        self.metadata.update(etag="DUMMY_ETAG_2", numRows="43", numBytes="90", numActiveLogicalBytes="90",
                             lastModifiedTime="123456790", streamingBuffer={"estimatedRows": "1"})
        gate.post(self.env, request=self.aws, metadata_request=self.google)
        self.assertIn("metadata=PASS; volatile=changed; lock=absent", (self.root / "summary").read_text())
        self.assertEqual(receipt.private_json(str(self.root / "history-import-table-after.json")), self.metadata)

    def test_post_distinguishes_no_import_partial_state_stable_metadata_and_lock(self):
        for failure in ("unchanged", "other-state", "schema", "description", "creation-time", "unknown-field",
                        "bad-metric", "unknown-streaming-field", "lock", "apply-failed"):
            with self.subTest(failure=failure):
                child = ImportVerifierTest()
                child.setUp()
                try:
                    child.before()
                    child.preapply()
                    if failure != "unchanged":
                        state = child.imported_state()
                        if failure == "other-state":
                            state["resources"][0]["instances"][0]["attributes"]["dummy"] = "CHANGED"
                        child.aws.body = json.dumps(state).encode()
                    if failure == "schema":
                        child.metadata["schema"]["fields"].reverse()
                    if failure == "description":
                        child.metadata["schema"]["fields"][0]["description"] = "DUMMY_CHANGED_DESCRIPTION"
                    if failure == "creation-time":
                        child.metadata["creationTime"] = "123456790"
                    if failure == "unknown-field":
                        child.metadata["unrecognizedStableSetting"] = "DUMMY_CHANGED"
                    if failure == "bad-metric":
                        child.metadata["numRows"] = {"unexpected": True}
                    if failure == "unknown-streaming-field":
                        child.metadata["streamingBuffer"] = {"unexpected": "1"}
                    if failure == "lock":
                        child.aws.lock = True
                    if failure == "apply-failed":
                        child.env["APPLY_OUTCOME"] = "failure"
                    with self.assertRaises(Exception):
                        gate.post(child.env, request=child.aws, metadata_request=child.google)
                    self.assertNotIn("DUMMY_STATE_PAYLOAD", (child.root / "summary").read_text())
                finally:
                    child.doCleanups()


if __name__ == "__main__":
    unittest.main()
