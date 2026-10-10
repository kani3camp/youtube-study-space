#!/usr/bin/env python3
"""Synthetic independent post-import no-op checks; no live cloud credentials."""
import copy
import json
from pathlib import Path
import unittest

import terraform_history_plan_receipt as receipt
import terraform_history_post_noop_receipt as gate
from terraform_plan_summary import build_summary
import test_terraform_history_plan_receipt as state_fixtures
import test_gcp_user_activity_schema_audit_workflow as history_fixtures


class IndependentHistoryPostNoopTest(unittest.TestCase):
    def setUp(self):
        self.fixture = state_fixtures.ReceiptTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.root, self.aws = self.fixture.root, self.fixture.aws
        source = history_fixtures.UserActivityHistoryPlanTest()
        source.setUp()
        self.plan, self.metadata = source.fixture(False)
        self.plan["terraform_version"] = "1.16.4"
        self.metadata.update(etag="DUMMY_ETAG", numRows="42", numBytes="84",
                             lastModifiedTime="123456789")
        self.env = self.fixture.env | {
            "HISTORY_POST_NOOP": "true", "DEV_HISTORY_POST_NOOP_ENABLED": "true",
            "HISTORY_TARGET": "dev", "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac",
            "PLAN_EXIT_CODE": "0"}
        state = state_fixtures.state_fixture()
        state["serial"] += 1
        state["resources"].append({
            "module": "module.user_activity_history[0]", "mode": "managed",
            "type": "google_bigquery_table", "name": "retained",
            "provider": 'provider["registry.terraform.io/hashicorp/google"]',
            "instances": [{"schema_version": 0,
                           "attributes": copy.deepcopy(self.plan["resource_changes"][-1]["change"]["after"])}]})
        state["check_results"].append({
            "object_kind": "resource",
            "config_addr": "module.user_activity_history.google_bigquery_table.retained",
            "status": "pass", "objects": [{"object_addr": gate.TABLE, "status": "pass"}]})
        self.aws.body = json.dumps(state).encode()
        receipt.policy(self.env)
        receipt.write_private(self.root / "user-history-post.json", self.metadata)
        receipt.write_private(self.root / "sanitized-plan.json",
                              build_summary(self.plan, environment="dev", git_sha=self.env["GITHUB_SHA"],
                                            policy="import-only"))

    def table_get(self, path, token, *, host):
        self.assertEqual((path, token, host),
                         (gate.TABLE_PATH, "DUMMY_TOKEN", "bigquery.googleapis.com"))
        return 200, copy.deepcopy(self.metadata)

    def test_exact_imported_state_and_full_root_noop12_pass_without_mutation(self):
        gate.before(self.env, request=self.aws)
        self.metadata.update(etag="DUMMY_ETAG_2", numRows="43", lastModifiedTime="123456790")
        gate.after(self.env, request=self.aws, metadata_request=self.table_get)
        self.assertIn("import0/no-op12", (self.root / "summary").read_text())
        self.assertEqual([call[:2] for call in self.aws.calls], [
            ("s3api", "head-object"), ("s3api", "get-object"), ("s3api", "head-object"),
            ("s3api", "list-objects-v2"), ("s3api", "list-objects-v2"),
            ("s3api", "head-object"), ("s3api", "get-object"), ("s3api", "head-object"),
            ("s3api", "list-objects-v2"), ("s3api", "list-objects-v2")])
        self.assertNotIn("DUMMY_TOKEN", (self.root / "summary").read_text())
        self.assertNotIn("DUMMY_STATE_PAYLOAD", (self.root / "summary").read_text())
        self.assertEqual((self.root / "history-post-noop-table-after.json").stat().st_mode & 0o777, 0o600)

    def test_closed_gate_or_unimported_state_stops_before_plan(self):
        for change in ({"DEV_HISTORY_POST_NOOP_ENABLED": "false"},
                       {"HISTORY_POST_NOOP": "false"}, {"HISTORY_TARGET": "prod"},
                       {"GITHUB_REF": "refs/heads/dev"}):
            with self.subTest(change=change), self.assertRaises(Exception):
                gate.before(self.env | change, request=self.aws)
        self.assertEqual(self.aws.calls, [])
        self.aws.body = json.dumps(state_fixtures.state_fixture()).encode()
        with self.assertRaises(Exception):
            gate.before(self.env, request=self.aws)
        self.assertFalse((self.root / gate.BEFORE).exists())

    def test_state_lock_metadata_plan_and_summary_deviations_stop(self):
        for failure in ("version", "lock", "metadata", "plan", "import-summary"):
            with self.subTest(failure=failure):
                child = IndependentHistoryPostNoopTest()
                child.setUp()
                try:
                    gate.before(child.env, request=child.aws)
                    if failure == "version":
                        child.aws.version = "DUMMY_NEW_VERSION"
                    elif failure == "lock":
                        child.aws.lock = True
                    elif failure == "metadata":
                        child.metadata["creationTime"] = "123456790"
                    elif failure == "plan":
                        child.env["PLAN_EXIT_CODE"] = "2"
                    else:
                        source = history_fixtures.UserActivityHistoryPlanTest()
                        source.setUp()
                        import_plan, _ = source.fixture(True)
                        receipt.write_private(child.root / "bad-summary.json",
                                              build_summary(import_plan, environment="dev",
                                                            git_sha=child.env["GITHUB_SHA"], policy="import-only"))
                        (child.root / "sanitized-plan.json").unlink()
                        (child.root / "bad-summary.json").rename(child.root / "sanitized-plan.json")
                    with self.assertRaises(Exception):
                        gate.after(child.env, request=child.aws, metadata_request=child.table_get)
                    self.assertNotIn("import0/no-op12", (child.root / "summary").read_text()
                                     if (child.root / "summary").exists() else "")
                finally:
                    child.doCleanups()


if __name__ == "__main__":
    unittest.main()
