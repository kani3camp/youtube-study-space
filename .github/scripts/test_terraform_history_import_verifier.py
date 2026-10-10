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
        self.approved = {"schema_version": 2, "target": "dev", "git_sha": SHA, "plan_run_id": 123456,
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
        from test_terraform_ownership_receipt import dummy_context
        self.env.update(dummy_context())
        self.env["PLAN_SAFETY_EVIDENCE"] = json.dumps({"schema_version": 1, "git_sha": SHA,
            "issued_utc": self.approved["issued_utc"], "expires_utc": self.approved["expires_utc"],
            "max_state_bytes": receipt.MAX_BYTES, "state_writers_quiescent": True})
        binding = {"nonce": "ab" * 32, "backend": {"account_id": "111111111111", "bucket": "dummy-state-bucket",
            "key": receipt.STATE_KEY, "region": "ap-northeast-1", "workspace": "default"}, "history_metadata": self.metadata}
        self.env["RUNTIME_OWNERSHIP_PACKET_JSON"] = json.dumps({"schema_version": 1, "receipt_binding": binding})
        self.env["EXPECTED_HISTORY_RECEIPT_SCOPE"] = gate.ownership.commitment(binding, gate.ownership.history_scope(binding))
        self.env["HISTORY_IMPORT_APPROVAL"] = json.dumps(self.approved)
        (self.root / "output").write_text("")
        receipt.write_private(self.root / "user-history-before.json", self.metadata)
        post_summary = json.loads(self.summary_raw)
        post_summary["counts"]["import"] = 0
        for resource in post_summary["resources"]:
            resource["import"] = False
        receipt.write_private(self.root / "post-apply-summary.json", post_summary)

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
                       {"plan_run_id": 0}, {"plan_run_id": True}, {"schema_version": 1},
                       {"schema_version": True}, {"cost_evidence_sha256": "c" * 64},
                       {"max_added_current_month_usd": "0.25"}):
            with self.subTest(change=change), self.assertRaises(Exception):
                gate.approval(dict(self.env, HISTORY_IMPORT_APPROVAL=json.dumps(self.approved | change)))
        with self.assertRaises(Exception):
            gate.approval(dict(self.env, MODE="plan"))
        with self.assertRaises(Exception):
            gate.approval(dict(self.env, HISTORY_IMPORT_APPROVAL=self.env["HISTORY_IMPORT_APPROVAL"][:-1] + ',"target":"dev"}'))

    def test_approval_requires_first_attempt_emitter_and_fresh_safety_before_cloud(self):
        safety = json.loads(self.env["PLAN_SAFETY_EVIDENCE"])
        changes = [{"DEV_HISTORY_RECEIPT_EMITTER_ENABLED": "false"}, {"GITHUB_RUN_ATTEMPT": "2"},
                   {"GITHUB_JOB": "other"}, {"PLAN_COST_EVIDENCE": "legacy"},
                   {"PLAN_SAFETY_EVIDENCE": json.dumps(safety | {"state_writers_quiescent": False})},
                   {"PLAN_SAFETY_EVIDENCE": json.dumps(safety | {"expires_utc": "2020-01-01T00:00:00Z"})}]
        for change in changes:
            with self.subTest(change=change), self.assertRaises(Exception):
                gate.before(self.env | change, request=lambda *_: self.fail("no cloud before admission"))
        for job in ("plan", "apply"):
            self.assertEqual(gate.approval(self.env | {"GITHUB_JOB": job}), self.approved)

    def test_apply_state_snapshot_honors_owner_size_bound_before_download(self):
        safety = json.loads(self.env["PLAN_SAFETY_EVIDENCE"])
        safety["max_state_bytes"] = len(self.aws.body) - 1
        with self.assertRaises(Exception):
            gate.before(self.env | {"PLAN_SAFETY_EVIDENCE": json.dumps(safety)}, request=self.aws)
        self.assertEqual([call[:2] for call in self.aws.calls], [("s3api", "head-object")])

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

    def test_oversized_dummy_plan_stops_before_approval_and_apply_without_public_values(self):
        binary = self.root / "tfplan"
        binary.write_bytes(b"PRIVATE_DUMMY_PLAN" + b"x" * receipt.MAX_BYTES)
        binary.chmod(0o600)
        script = Path(gate.__file__)
        result = subprocess.run(["python3", str(script), "--phase", "size"],
            env=os.environ | self.env | {"MODE": "plan"}, capture_output=True, text=True)
        self.assertEqual(result.returncode, 3)
        self.assertIn("STOP", result.stderr)
        self.assertNotIn("PRIVATE_DUMMY_PLAN", result.stdout + result.stderr)
        self.assertNotIn(str(self.root), result.stdout + result.stderr)
        self.before()
        receipt.write_private(self.root / "sanitized-replan.json", json.loads(self.summary_raw))
        with self.assertRaises(Exception):
            gate.preapply(self.env, request=self.aws)
        self.assertFalse((self.root / "history-import-preapply.json").exists())
        self.assertEqual((self.root / "output").read_text(), "")

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


class HistoryEmitterTest(unittest.TestCase):
    def setUp(self):
        from test_terraform_ownership_receipt import dummy_binding, dummy_context
        self.case = ImportVerifierTest()
        self.case.setUp()
        self.addCleanup(self.case.doCleanups)
        self.case.env.update(dummy_context())
        self.binding = dummy_binding()
        self.case.env['EXPECTED_HISTORY_RECEIPT_SCOPE'] = gate.ownership.commitment(self.binding, gate.ownership.history_scope(self.binding))
        self.case.env['RUNTIME_OWNERSHIP_PACKET_JSON'] = json.dumps(dict(schema_version=1, receipt_binding=self.binding))
        self.case.before()
        self.case.preapply()
        self.case.aws.body = json.dumps(self.case.imported_state()).encode()
        post_summary = json.loads(self.case.summary_raw)
        post_summary['counts']['import'] = 0
        for resource in post_summary['resources']:
            resource['import'] = False
        (self.case.root / 'post-apply-summary.json').unlink()
        receipt.write_private(self.case.root / 'post-apply-summary.json', post_summary)

    def run_post(self):
        gate.post(self.case.env, request=self.case.aws, metadata_request=self.case.google)

    def test_success_creates_private_sanitized_receipt_only_after_all_checks(self):
        from io import StringIO
        import terraform_ownership_receipt as ownership
        self.run_post()
        path = self.case.root / ownership.RECORD
        self.assertEqual(path.stat().st_mode & 0o777, 0o600)
        value = receipt.private_json(str(path))
        self.assertEqual(value['wave'], 'history12')
        self.assertEqual(value['state_after_commitment'], ownership.state_commitment(self.binding, self.case.imported_state()))
        self.assertEqual(value['accounting']['post_state_reads'], 4)
        self.assertEqual(value['accounting']['post_metadata_reads'], 1)
        with patch('sys.stdout', StringIO()) as output:
            ownership.emit(self.case.env)
        self.assertEqual(output.getvalue().count(ownership.MARKER), 1)
        for sentinel in ['DUMMY_TOKEN', 'DUMMY_ETAG', '111111111111', 'dummy-state-bucket', self.binding['nonce']]:
            self.assertNotIn(sentinel, output.getvalue())

    def test_disabled_emitter_rejects_history_import_without_any_post_reads(self):
        self.case.env['DEV_HISTORY_RECEIPT_EMITTER_ENABLED'] = 'false'
        self.case.env.pop('RUNTIME_OWNERSHIP_PACKET_JSON')
        (self.case.root / 'post-apply-summary.json').unlink()
        calls = len(self.case.aws.calls)
        with self.assertRaises(Exception):
            self.run_post()
        self.assertEqual(len(self.case.aws.calls), calls)
        self.assertFalse((self.case.root / gate.ownership.RECORD).exists())

    def test_failed_apply_post_state_metadata_lock_never_issue_receipt(self):
        mutations = [lambda c: c.env.update(APPLY_OUTCOME='failure'), lambda c: c.env.update(POST_PLAN_OUTCOME='failure'),
            lambda c: setattr(c.aws, 'body', json.dumps(state_fixtures.state_fixture()).encode()),
            lambda c: c.metadata['schema']['fields'].reverse(), lambda c: setattr(c.aws, 'lock', True)]
        for mutate in mutations:
            child = HistoryEmitterTest()
            child.setUp()
            try:
                mutate(child.case)
                with self.subTest(case=mutations.index(mutate)), self.assertRaises(Exception):
                    child.run_post()
                self.assertFalse((child.case.root / gate.ownership.RECORD).exists())
            finally:
                child.doCleanups()

    def test_post_import_marker_wrong_graph_or_missing_summary_rejects(self):
        mutations = [lambda s: s['counts'].update(**{'import': 1}), lambda s: s['resources'][0].update(**{'import': True}),
                     lambda s: s['resources'].pop(), lambda s: s.update(git_sha='b'*40), lambda s: s.update(drift=['dummy'])]
        for mutate in mutations:
            child = HistoryEmitterTest()
            child.setUp()
            try:
                path = child.case.root / 'post-apply-summary.json'
                summary = json.loads(path.read_text())
                mutate(summary)
                path.write_text(json.dumps(summary))
                with self.subTest(case=mutations.index(mutate)), self.assertRaises(Exception):
                    child.run_post()
                self.assertFalse((child.case.root / gate.ownership.RECORD).exists())
            finally:
                child.doCleanups()
        (self.case.root / 'post-apply-summary.json').unlink()
        with self.assertRaises(Exception):
            self.run_post()
        self.assertFalse((self.case.root / gate.ownership.RECORD).exists())

    def test_private_binding_scope_context_nonce_and_envelope_are_exact(self):
        changes = [lambda b: b['backend'].update(bucket='dummy-other-bucket'),
                   lambda b: b['history_metadata']['schema']['fields'].reverse(), lambda b: b.update(nonce='x'*64),
                   lambda b: b.update(private_extra='DUMMY_PRIVATE_SENTINEL')]
        for mutate in changes:
            child = HistoryEmitterTest()
            child.setUp()
            try:
                mutate(child.binding)
                child.case.env['RUNTIME_OWNERSHIP_PACKET_JSON'] = json.dumps(dict(schema_version=1, receipt_binding=child.binding))
                with self.subTest(case=changes.index(mutate)), self.assertRaises(Exception):
                    child.run_post()
                self.assertFalse((child.case.root / gate.ownership.RECORD).exists())
            finally:
                child.doCleanups()
        self.case.env['GITHUB_RUN_ATTEMPT'] = '2'
        with self.assertRaises(Exception):
            self.run_post()
        self.assertFalse((self.case.root / gate.ownership.RECORD).exists())

    def test_issuer_file_overwrite_and_symlink_are_rejected(self):
        target = self.case.root / gate.ownership.RECORD
        target.symlink_to(self.case.root / 'DUMMY_PRIVATE_TARGET')
        with self.assertRaises(Exception):
            self.run_post()
        self.assertFalse((self.case.root / 'DUMMY_PRIVATE_TARGET').exists())

    def test_failure_cli_suppresses_private_values_and_does_not_emit_marker(self):
        from io import StringIO
        self.case.env['RUNTIME_OWNERSHIP_PACKET_JSON'] = '{"DUMMY_PRIVATE_SENTINEL":1}'
        original = gate.post
        with patch.dict(os.environ, self.case.env, clear=True), patch('sys.argv', ['history', '--phase', 'post']), \
             patch.object(gate, 'post', side_effect=lambda env: original(env, request=self.case.aws, metadata_request=self.case.google)), \
             patch('sys.stdout', StringIO()) as out, patch('sys.stderr', StringIO()) as err:
            self.assertEqual(gate.main(), 3)
        public = out.getvalue()+err.getvalue()
        self.assertNotIn('DUMMY_PRIVATE_SENTINEL', public)
        self.assertNotIn(self.binding['nonce'], public)
        self.assertNotIn(gate.ownership.MARKER, public)


if __name__ == "__main__":
    unittest.main()
