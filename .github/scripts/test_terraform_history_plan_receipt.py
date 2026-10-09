#!/usr/bin/env python3
"""Synthetic receipts only: no credential or real metadata/state access."""
import contextlib
import copy
import ast
from datetime import datetime, timezone
from decimal import Decimal, ROUND_CEILING
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import terraform_history_plan_receipt as receipt
import terraform_identity_smoke as smoke
from terraform_identity_diagnostics import RECEIPT_REASONS, ReceiptDependencyError, ReceiptInvariantError
from terraform_plan_summary import build_summary
import test_gcp_user_activity_schema_audit_workflow as fixtures

NOW = datetime(2026, 10, 8, 12, tzinfo=timezone.utc)
SHA = "a" * 40
ACTUAL_COST_POLICY = receipt.cost_policy


def profile(**changes):
    return {"model": receipt.MODEL, "git_sha": SHA, "issued_utc": "2026-10-08T11:00:00Z",
            "expires_utc": "2026-10-08T13:00:00Z", "max_state_bytes": 16384,
            "budget_month": "2026-10", "cloud_side_cost_usd": "0.003",
            "cloud_side_evidence_reviewed": True, "rates_verified": True,
            "state_writers_quiescent": True} | changes


def state_fixture():
    grouped = {}
    for address in sorted(receipt.BASELINE):
        match = re.fullmatch(r"(?:(module\..+)\.)?(google_[^.]+)\.([^\[]+)(?:\[(.+)\])?", address)
        module, kind, name, index = match.groups()
        instance = {"schema_version": 0, "identity_schema_version": 0,
                    "attributes": {"project": "test-youtube-study-space", "dummy": "DUMMY_STATE_PAYLOAD"}}
        if index is not None:
            instance["index_key"] = json.loads(index)
        resource = {"mode": "managed", "type": kind, "name": name,
                    "provider": 'provider["registry.terraform.io/hashicorp/google"]', "instances": [instance]}
        if module:
            resource["module"] = module
        key = (module, kind, name)
        if key in grouped:
            grouped[key]["instances"].append(instance)
        else:
            grouped[key] = resource
    checks = [{"object_kind": "var", "config_addr": "var.project_id", "status": "pass",
               "objects": [{"object_addr": "var.project_id", "status": "pass"}]}]
    for address in ("module.runtime_wif.var.own_provider", "module.runtime_wif.var.grant_keys",
                    "module.runtime_wif.var.inventory", "module.owned_apis.var.classification"):
        checks.append({"object_kind": "var", "config_addr": address, "status": "pass",
                       "objects": [{"object_addr": address, "status": "pass"}]})
    for address, objects in (("module.user_activity_history.var.field_order", None),
                             ("module.user_activity_history.var.field_descriptions", [])):
        checks.append({"object_kind": "var", "config_addr": address, "status": "pass", "objects": objects})
    return {"version": 4, "terraform_version": "1.16.4", "serial": 27,
            "lineage": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "resources": list(grouped.values()),
            "check_results": checks,
            # Terraform 1.16.4 uses json:"sensitive,omitempty" for false.
            "outputs": {key: {"type": "string", "value": value}
                        for key, value in [("environment", "development"), ("project_id", "test-youtube-study-space")]}}


class FakeS3:
    def __init__(self):
        self.body = json.dumps(state_fixture()).encode()
        self.version = "DUMMY_VERSION_ID"
        self.calls = []
        self.overrides = {}
        self.denied = None
        self.lock = False
        self.workspaces = False
        self.network = False

    def __call__(self, *args):
        self.calls.append(args)
        service, operation = args[:2]
        if self.network:
            raise RuntimeError("DUMMY_NETWORK_PRIVATE_ERROR")
        if operation == self.denied:
            return subprocess.CompletedProcess([], 1, "", "(403) DUMMY_PRIVATE_ACCOUNT_ERROR")
        if service == "sts":
            value = {"Account": "111111111111"}
        elif operation == "head-object":
            value = {"ContentLength": len(self.body), "VersionId": self.version, "ETag": '"dummy-etag"',
                     "ServerSideEncryption": "AES256", "Metadata": {}}
        elif operation == "get-object":
            Path(args[-1]).write_bytes(self.body)
            value = {"ContentLength": len(self.body), "VersionId": self.version, "ETag": '"dummy-etag"'}
        elif operation == "list-objects-v2":
            prefix = args[args.index("--prefix") + 1]
            present = self.lock if prefix.endswith(".tflock") else self.workspaces
            # AWS CLI 2.37.9 botocore auto-requests URL encoding and retains
            # EncodingType in the parsed response after decoding Prefix/Key.
            value = {"Name": "dummy-state-bucket", "Prefix": prefix, "MaxKeys": 1, "KeyCount": int(present),
                     "IsTruncated": False, "EncodingType": "url"}
            if present:
                value["Contents"] = [{"Key": prefix, "Size": 123}]
        else:
            # Even if mutation APIs would succeed, the implementation must not
            # invoke them. Unexpected calls remain visible to allowlist asserts.
            value = {"DUMMY_MUTATION_SUCCESS": True}
        value.update(self.overrides.get(operation, {}))
        return subprocess.CompletedProcess([], 0, json.dumps(value), "")


class ReceiptTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        self.env = {"RUNNER_TEMP": str(self.root), "GITHUB_SHA": SHA, "MODE": "plan", "TF_WORKSPACE": "default",
                    "TF_VAR_manage_user_activity_history": "true", "TF_VAR_project_id": "test-youtube-study-space",
                    "STATE_KEY": receipt.STATE_KEY, "STATE_AWS_REGION": "ap-northeast-1", "STATE_ACCOUNT_ID": "111111111111",
                    "STATE_BUCKET": "dummy-state-bucket", "AWS_MAX_ATTEMPTS": "1", "AWS_RETRY_MODE": "standard",
                    "GCP_SMOKE_ACCESS_TOKEN": "DUMMY_TOKEN", "GITHUB_STEP_SUMMARY": str(self.root / "summary"),
                    "BACKEND_INIT_OUTCOME": "success", "TERRAFORM_PLAN_OUTCOME": "success", "PLAN_VALIDATION_OUTCOME": "success", "PLAN_EXIT_CODE": "2",
                    "PLAN_COST_EVIDENCE": json.dumps(profile())}
        self.aws = FakeS3()
        source = fixtures.UserActivityHistoryPlanTest()
        source.setUp()
        self.plan, self.metadata = source.fixture()
        self.plan["terraform_version"] = "1.16.4"
        self.metadata.update(etag="DUMMY_ETAG", numRows="42", numBytes="84", lastModifiedTime="123456789")
        self.google_calls = []
        self.policy_patch = patch.object(receipt, "cost_policy", side_effect=lambda raw, sha, now=None: self.actual_policy(raw, sha, now=now or NOW))
        self.actual_policy = ACTUAL_COST_POLICY
        self.policy_patch.start()
        self.addCleanup(self.policy_patch.stop)

    def google(self, path, token, **kwargs):
        self.google_calls.append((path, token, kwargs))
        return 200, copy.deepcopy(self.metadata)

    def baseline(self):
        receipt.policy(self.env)
        # This is the actual history branch of the existing identity smoke.
        with patch.object(smoke, "aws", side_effect=self.aws):
            labels = smoke.verify_aws(self.env, plan_read_only=True)
        self.assertNotIn("DUMMY", str(labels))
        receipt.write_private(self.root / "user-history-before.json", self.metadata)
        receipt.before(self.env)
        receipt.write_private(self.root / "sanitized-plan.json", build_summary(self.plan, environment="dev", git_sha=SHA, policy="import-only"))

    def test_complete_snapshot_smoke_and_final_receipt_use_only_existing_reads(self):
        self.baseline()
        receipt.after(self.env, request=self.aws, metadata_request=self.google)
        result = receipt.private_json(str(self.root / "history-plan-after.json"))
        self.assertEqual(result["import"], 1)
        self.assertEqual(result["existing_no_op"], 11)
        self.assertTrue(result["stable_table_metadata_unchanged"] and result["native_lock_absent"])
        self.assertFalse(result["volatile_table_observations_changed"])
        self.assertEqual([a[:2] for a in self.aws.calls], [
            ("sts", "get-caller-identity"), ("s3api", "head-object"), ("s3api", "get-object"), ("s3api", "head-object"),
            ("s3api", "list-objects-v2"), ("s3api", "list-objects-v2"),
            ("s3api", "head-object"), ("s3api", "get-object"), ("s3api", "head-object"), ("s3api", "list-objects-v2")])
        for args in self.aws.calls[1:]:
            self.assertEqual(args[args.index("--expected-bucket-owner") + 1], "111111111111")
            if "--key" in args:
                self.assertEqual(args[args.index("--key") + 1], receipt.STATE_KEY)
        self.assertEqual(self.google_calls, [(receipt.TABLE_PATH, "DUMMY_TOKEN", {"host": "bigquery.googleapis.com"})])
        for path in self.root.glob("*.json"):
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)

    def test_cost_missing_or_malformed_stops_without_aws_or_metadata_reads(self):
        bad = ["", "{}", '{"model":1,"model":2}', "NaN", "[]", json.dumps(profile(unapproved=True)),
               *[json.dumps(profile(**{key: value})) for key, value in [
                   ("git_sha", "b" * 40), ("model", "unknown"), ("max_state_bytes", True), ("max_state_bytes", receipt.MAX_BYTES + 1),
                   ("max_state_bytes", 0), ("budget_month", "2026-09"), ("cloud_side_cost_usd", "NaN"),
                   ("cloud_side_cost_usd", "0"), ("cloud_side_cost_usd", "0.25"), ("cloud_side_cost_usd", "0.249"), ("cloud_side_cost_usd", "-0.01"),
                   ("cloud_side_cost_usd", 0.001), ("expires_utc", "2026-10-08T11:59:59Z"),
                   ("issued_utc", "2026-10-08T12:00:01Z"), ("expires_utc", "2026-10-10T12:00:00Z"),
                   ("rates_verified", False), ("cloud_side_evidence_reviewed", "true"), ("state_writers_quiescent", False)]]]
        for raw in bad:
            with self.subTest(raw=raw), self.assertRaises((ValueError, TypeError)):
                receipt.policy(dict(self.env, PLAN_COST_EVIDENCE=raw))
            self.assertFalse((self.root / "history-plan-cost.json").exists())
        self.assertEqual((self.aws.calls, self.google_calls), ([], []))

    def test_cost_bound_accounts_for_size_full_month_storage_and_all_ceiling_components(self):
        value = self.actual_policy(json.dumps(profile()), SHA, now=NOW)
        once = (Decimal(256) * Decimal("0.00001") + Decimal(128) * Decimal("0.00001")
                + Decimal(64 * 16384 + 256 * 16384) / Decimal(1024 ** 3) * Decimal("0.25") + Decimal("0.003"))
        storage = Decimal(2 * 32768) / Decimal(1024 ** 3) * Decimal("0.10")
        quantum = Decimal("0.000000001")
        expected = once.quantize(quantum, rounding=ROUND_CEILING) + storage.quantize(quantum, rounding=ROUND_CEILING)
        self.assertEqual(Decimal(value["upper_bound_usd"]), expected)
        for changes in ({"max_state_bytes": receipt.MAX_BYTES, "cloud_side_cost_usd": "0.20"}, {"cloud_side_cost_usd": "0.249"}):
            with self.assertRaises(ValueError):
                self.actual_policy(json.dumps(profile(**changes)), SHA, now=NOW)
        with self.assertRaises(ValueError):
            self.actual_policy(json.dumps(profile(issued_utc="2026-10-15T00:00:00Z", expires_utc="2026-10-15T01:00:00Z")), SHA,
                               now=datetime(2026, 10, 15, tzinfo=timezone.utc))

    def test_initial_head_rejects_unknown_storage_version_size_before_download(self):
        receipt.policy(self.env)
        for changes in ({"StorageClass": "STANDARD_IA"}, {"VersionId": "null"}, {"VersionId": None},
                        {"ContentLength": True}, {"ContentLength": 16385}, {"ContentLength": 0},
                        {"DeleteMarker": True}, {"ServerSideEncryption": "unknown"}, {"ContentEncoding": "gzip"}):
            self.aws.overrides = {"head-object": changes}; self.aws.calls.clear()
            with self.subTest(changes=changes), self.assertRaises(smoke.StageFailure) as caught:
                receipt.snapshot_before(self.env, request=self.aws)
            self.assertEqual((caught.exception.stage, caught.exception.category), ("history-head", "invalid-evidence"))
            self.assertEqual([a[1] for a in self.aws.calls], ["head-object"])

    def test_monthly_profile_accepts_no_deletion_deadline_and_rejects_old_lifetime_contract(self):
        value = self.actual_policy(json.dumps(profile()), SHA, now=NOW)
        ledger = value["monthly_ledger_entry"]
        self.assertEqual(ledger["budget_month"], "2026-10")
        self.assertEqual(ledger["basis"], "conservative-upper-bound-estimate")
        self.assertEqual(ledger["status"], "reserved-upper-bound")
        self.assertIsNone(ledger["retention_end_utc"])
        self.assertEqual(ledger["retention_assumption"], "no-assumed-deletion")
        self.assertEqual(ledger["future_rates"], "revalidate-each-month")
        self.assertIn("not-assumed-zero", ledger["future_side_costs"])
        self.assertGreater(Decimal(ledger["future_full_month_lock_storage_estimate_at_model_rates_usd"]), 0)
        for raw in (json.dumps(profile(lock_retention_days=31)),
                    json.dumps(profile(model="dev-history-plan-monthly-2026-10-08-v1")),
                    json.dumps(profile(model="dev-history-plan-2026-10-08-v1"))):
            with self.assertRaises(ValueError): self.actual_policy(raw, SHA, now=NOW)

    def test_monthly_scope_and_month_end_margin_reject_wrong_month_or_crossing(self):
        for value in (None, True, 202610, "2026-9", "2026-09", "2026-11"):
            with self.assertRaises(ValueError): self.actual_policy(json.dumps(profile(budget_month=value)), SHA, now=NOW)
        with patch.object(receipt, "MODEL_EXPIRES", datetime(2029, 1, 1, tzinfo=timezone.utc)):
            for month, last, following in (("2026-10", "2026-10-31", "2026-11-01"),
                                          ("2026-12", "2026-12-31", "2027-01-01"),
                                          ("2028-02", "2028-02-29", "2028-03-01")):
                issued = last + "T12:00:00Z"
                now = receipt.timestamp(issued)
                good = profile(budget_month=month, issued_utc=issued, expires_utc=last + "T23:30:00Z")
                self.actual_policy(json.dumps(good), SHA, now=now)
                for expires in (last + "T23:30:01Z", following + "T00:00:00Z"):
                    with self.assertRaises(ValueError): self.actual_policy(json.dumps(good | {"expires_utc": expires}), SHA, now=now)

    def test_monthly_decimal_cap_accepts_approved_quarter_dollar_and_rejects_one_quantum_over(self):
        self.assertEqual(receipt.LIMIT, Decimal("0.25"))
        above_old_cap = self.actual_policy(json.dumps(profile(cloud_side_cost_usd="0.10")), SHA, now=NOW)
        self.assertGreater(Decimal(above_old_cap["upper_bound_usd"]), Decimal("0.01"))
        self.assertLess(Decimal(above_old_cap["upper_bound_usd"]), Decimal("0.25"))
        base = self.actual_policy(json.dumps(profile(cloud_side_cost_usd="0.001")), SHA, now=NOW)
        ledger = base["monthly_ledger_entry"]
        remaining = receipt.LIMIT - Decimal(ledger["current_month_added_cost_upper_bound_usd"]) + Decimal("0.001")
        at_limit = self.actual_policy(json.dumps(profile(cloud_side_cost_usd=format(remaining, ".9f"))), SHA, now=NOW)
        self.assertEqual(Decimal(at_limit["upper_bound_usd"]), receipt.LIMIT)
        with self.assertRaises(ValueError):
            self.actual_policy(json.dumps(profile(cloud_side_cost_usd=format(remaining + Decimal("0.000000001"), ".9f"))), SHA, now=NOW)

    def test_monthly_reservation_is_stable_and_ledger_tampering_stops_before_reads(self):
        early = self.actual_policy(json.dumps(profile()), SHA, now=NOW)
        late = self.actual_policy(json.dumps(profile()), SHA, now=datetime(2026, 10, 8, 12, 30, tzinfo=timezone.utc))
        self.assertEqual(early, late)
        receipt.policy(self.env)
        path = self.root / "history-plan-cost.json"
        for changes in ({"retention_end_utc": "2026-10-31T00:00:00Z"},
                        {"added_retained_bytes_upper_bound": 0}, {"unapproved": True}):
            changed = copy.deepcopy(early); changed["monthly_ledger_entry"].update(changes)
            path.write_text(json.dumps(changed))
            with self.assertRaises(smoke.StageFailure) as caught: receipt.snapshot_before(self.env, request=self.aws)
            self.assertEqual((caught.exception.stage, caught.exception.category), ("history-policy", "invalid-evidence"))
        self.assertEqual(self.aws.calls, [])

    def test_normal_unlock_or_failed_receipt_never_cancels_recurring_storage_reservation(self):
        for failed in (False, True):
            child = ReceiptTests("runTest"); child.setUp(); self.addCleanup(child.doCleanups); child.baseline()
            before = receipt.private_json(str(child.root / "history-plan-cost.json"))["monthly_ledger_entry"]
            if failed:
                child.env["TERRAFORM_PLAN_OUTCOME"] = "failure"
                with self.assertRaises(ValueError): child.after_check()
            else:
                child.after_check()
                self.assertEqual(receipt.private_json(str(child.root / "history-plan-after.json"))["monthly_ledger_entry"], before)
            reserved = receipt.private_json(str(child.root / "history-plan-cost.json"))["monthly_ledger_entry"]
            self.assertEqual(reserved, before)
            self.assertEqual(reserved["added_retained_bytes_upper_bound"], 2 * receipt.LOCK_BYTES)
            self.assertIsNone(reserved["retention_end_utc"])
            self.assertEqual(child.aws.calls[-1][1], "list-objects-v2")

    def test_exact_state_eleven_rejects_history_missing_extra_duplicate_tainted_and_unknown(self):
        base = state_fixture()
        changes = [lambda s: s["resources"].pop(), lambda s: s["resources"].append(copy.deepcopy(s["resources"][0])),
                   lambda s: s["resources"][0].update(name="wrong"), lambda s: s["resources"][0].update(mode="data"),
                   lambda s: s["resources"][0]["instances"][0].update(status="tainted"),
                   lambda s: s["resources"][0]["instances"][0].update(deposed="dummy"),
                   lambda s: s.update(serial=True), lambda s: s.update(lineage="bad"), lambda s: s.update(unapproved=True),
                   lambda s: s["check_results"][0].update(status="unknown"),
                   lambda s: s["check_results"][0].update(config_addr="var.foreign"),
                   lambda s: s["resources"][0]["instances"][0].update(identity_schema_version=True),
                   lambda s: s["resources"][0]["instances"][0].update(create_before_destroy=True),
                   lambda s: s["outputs"]["project_id"].update(value="foreign"),
                   lambda s: s["resources"][0]["instances"][0]["attributes"].update(project="foreign")]
        for mutate in changes:
            value = copy.deepcopy(base); mutate(value)
            with self.assertRaises(ValueError):
                receipt.state_shape(json.dumps(value).encode())
        history = {"module": "module.user_activity_history[0]", "mode": "managed", "type": "google_bigquery_table", "name": "retained",
                   "provider": 'provider["registry.terraform.io/hashicorp/google"]', "instances": [{"schema_version": 0, "attributes": {"project": "test-youtube-study-space"}}]}
        base["resources"][0] = history
        with self.assertRaises(ValueError):
            receipt.state_shape(json.dumps(base).encode())
        for raw in (b'{"version":4,"version":4}', b'NaN'):
            with self.assertRaises(ValueError): receipt.state_shape(raw)

    def test_terraform_1164_normal_output_and_known_check_metadata(self):
        state = state_fixture()
        self.assertEqual(sum(len(resource["instances"]) for resource in state["resources"]), 11)
        self.assertNotIn("sensitive", state["outputs"]["project_id"])
        self.assertIsNone(state["check_results"][-2]["objects"])
        self.assertEqual(state["check_results"][-1]["objects"], [])
        receipt.state_shape(json.dumps(state).encode())
        explicit_false = copy.deepcopy(state)
        explicit_false["outputs"]["project_id"]["sensitive"] = False
        receipt.state_shape(json.dumps(explicit_false).encode())

        changes = [lambda s, value=value: s["outputs"]["project_id"].update(sensitive=value)
                   for value in (True, 0, None, "false")]
        changes += [lambda s: s["outputs"]["project_id"].update(unknown="PRIVATE_SENTINEL"),
                    lambda s: s["outputs"]["project_id"].pop("type"),
                    lambda s: s["outputs"]["project_id"].update(type="number"),
                    lambda s: s["outputs"]["project_id"].update(value="PRIVATE_SENTINEL")]
        for mutate in changes:
            altered = copy.deepcopy(state); mutate(altered)
            with self.subTest(mutate=mutate), self.assertRaises(ReceiptInvariantError) as caught:
                receipt.state_shape(json.dumps(altered).encode())
            self.assertEqual(caught.exception.reason, "state-output-value")
            self.assertNotIn("PRIVATE_SENTINEL", str(caught.exception))

    def test_known_zero_instance_checks_only_and_unknown_or_nonpass_stops(self):
        state = state_fixture()
        known_active = "module.runtime_wif.var.own_provider"
        known_zero = "module.user_activity_history.var.field_order"
        cases = [
            (lambda s: s["check_results"][-2].update(status="unknown"), "checks-result"),
            (lambda s: s["check_results"][-2].update(status="fail"), "checks-result"),
            (lambda s: s["check_results"][-2].pop("objects"), "checks-envelope"),
            (lambda s: s["check_results"][1].update(objects=None), "checks-object-list"),
            (lambda s: s["check_results"][1].update(objects=[]), "checks-object-list"),
            (lambda s: s["check_results"].append({"object_kind": "var", "config_addr": known_zero,
                "status": "pass", "objects": None}), "checks-result"),
            (lambda s: s["check_results"].append({"object_kind": "var", "config_addr": "module.foreign.var.private",
                "status": "pass", "objects": None}), "checks-address"),
            (lambda s: s["check_results"].append({"object_kind": "resource",
                "config_addr": "google_bigquery_table.foreign", "status": "pass", "objects": None}), "checks-address"),
            (lambda s: s["check_results"][1]["objects"][0].update(status="fail", failure_messages=["PRIVATE_SENTINEL"]),
                "checks-object-result"),
        ]
        for mutate, reason in cases:
            altered = copy.deepcopy(state); mutate(altered)
            with self.subTest(reason=reason, mutate=mutate), self.assertRaises(ReceiptInvariantError) as caught:
                receipt.state_shape(json.dumps(altered).encode())
            self.assertEqual(caught.exception.reason, reason)
            self.assertNotIn("PRIVATE_SENTINEL", str(caught.exception))
        self.assertIn(known_active, {check["config_addr"] for check in state["check_results"]})

    def test_every_receipt_require_has_a_literal_allowlisted_reason(self):
        source = Path(receipt.__file__).read_text()
        guards = [node for node in ast.walk(ast.parse(source)) if isinstance(node, ast.Call)
                  and isinstance(node.func, ast.Name) and node.func.id == "require"]
        self.assertGreaterEqual(len(guards), 90)
        for guard in guards:
            with self.subTest(line=guard.lineno):
                self.assertEqual(len(guard.args), 2)
                self.assertFalse(guard.keywords)
                self.assertIsInstance(guard.args[1], ast.Constant)
                self.assertIn(guard.args[1].value, RECEIPT_REASONS)

    def test_current_version_body_serial_lineage_and_metadata_changes_cannot_pass(self):
        cases = ["version", "bytes", "serial", "lineage", "newMetadata", "description", "creationTime", "streamingBuffer", "numRows"]
        # Each case needs a fresh immutable baseline, not overwritten evidence.
        for case in cases:
            with self.subTest(case=case), tempfile.TemporaryDirectory() as directory:
                child = ReceiptTests("runTest"); child.setUp(); self.addCleanup(child.doCleanups)
                child.baseline()
                if case == "version": child.aws.version = "DUMMY_NEW_VERSION"
                elif case in {"bytes", "serial", "lineage"}:
                    state = json.loads(child.aws.body)
                    if case == "bytes": child.aws.body += b"\n"
                    else:
                        state[case] = 28 if case == "serial" else "bbbbbbbb-cccc-dddd-eeee-ffffffffffff"
                        child.aws.body = json.dumps(state).encode()
                elif case == "description": child.metadata["schema"]["fields"][0]["description"] = "Public dummy changed description"
                elif case == "streamingBuffer": child.metadata[case] = {"unknown": "1"}
                elif case == "numRows": child.metadata[case] = {"invalid": True}
                else: child.metadata[case] = "DUMMY_CHANGED_VALUE"
                with self.assertRaises(ValueError): child.after_check()
                self.assertFalse((child.root / "history-plan-after.json").exists())

    def test_independent_writer_metrics_may_change_but_stable_unknown_fields_remain_strict(self):
        self.baseline()
        self.metadata.update(etag="DUMMY_ETAG_2", lastModifiedTime="123456790", numRows="43",
                             numBytes="90", numActiveLogicalBytes="90",
                             streamingBuffer={"estimatedRows": "1", "oldestEntryTime": "123456790"})
        receipt.after(self.env, request=self.aws, metadata_request=self.google)
        record = receipt.private_json(str(self.root / "history-plan-after.json"))
        self.assertTrue(record["stable_table_metadata_unchanged"])
        self.assertTrue(record["volatile_table_observations_changed"])
        self.assertEqual(receipt.private_json(str(self.root / "history-plan-table-after.json")), self.metadata)
        summary = (self.root / "summary").read_text()
        self.assertIn("Stable table metadata invariant: PASS", summary)
        self.assertIn("Volatile table observations: changed", summary)
        self.assertNotIn("DUMMY_", summary)

    def after_check(self):
        receipt.after(self.env, request=self.aws, metadata_request=self.google)

    def test_lock_presence_403_network_truncation_missing_fields_are_never_absence(self):
        receipt.policy(self.env)
        for override in ({"KeyCount": 1, "Contents": [{"Key": receipt.STATE_KEY + ".tflock"}]}, {"IsTruncated": True},
                         {"KeyCount": None}, {"MaxKeys": True}, {"Prefix": "other"}, {"Name": "foreign"},
                         {"NextContinuationToken": "more"}, {"NextContinuationToken": ""},
                         {"CommonPrefixes": None}, {"CommonPrefixes": [{"Prefix": "private"}]},
                         {"error": {"code": 403}}, {"unapproved": False},
                         {"EncodingType": "xml"}, {"EncodingType": None}, {"EncodingType": True}):
            self.aws.overrides = {"list-objects-v2": override}
            with self.assertRaises(ValueError): receipt.absent(self.env, receipt.STATE_KEY + ".tflock", request=self.aws)
        self.aws.overrides = {}; self.aws.denied = "list-objects-v2"
        with self.assertRaises(ReceiptDependencyError) as caught:
            receipt.absent(self.env, receipt.STATE_KEY + ".tflock", request=self.aws)
        self.assertEqual(caught.exception.reason, "s3-cli-failure")
        self.aws.denied = None; self.aws.network = True
        with self.assertRaises(ReceiptDependencyError) as caught:
            receipt.absent(self.env, receipt.STATE_KEY + ".tflock", request=self.aws)
        self.assertEqual(caught.exception.reason, "s3-request")

    def test_sdk_url_encoding_proves_empty_lock_and_workspace_lists_only(self):
        receipt.policy(self.env)
        for prefix in (receipt.STATE_KEY + ".tflock", receipt.STATE_KEY.rsplit("/", 1)[0] + "/workspaces/"):
            with self.subTest(prefix=prefix):
                receipt.absent(self.env, prefix, request=self.aws)
                self.assertEqual(self.aws.calls[-1][1], "list-objects-v2")
                self.assertEqual(self.aws.calls[-1][self.aws.calls[-1].index("--prefix") + 1], prefix)

    def test_released_lock_is_required_and_failed_plan_or_import_zero_cannot_pass(self):
        self.baseline(); self.aws.lock = True
        with self.assertRaises(ValueError): self.after_check()
        self.assertFalse((self.root / "history-plan-after.json").exists())

    def test_import_zero_and_missing_summary_fail_after_post_reads(self):
        self.baseline()
        path = self.root / "sanitized-plan.json"
        value = receipt.private_json(str(path)); value["counts"]["import"] = 0
        path.write_text(json.dumps(value))
        with self.assertRaises(ValueError): self.after_check()
        self.assertTrue((self.root / "history-plan-state-after.json").exists())
        self.assertTrue((self.root / "history-plan-table-after.json").exists())
        self.assertEqual(self.aws.calls[-1][1], "list-objects-v2")

    def test_missing_summary_still_checks_remote_invariants_before_stopping(self):
        self.baseline(); (self.root / "sanitized-plan.json").unlink()
        with self.assertRaises(ReceiptDependencyError) as caught: self.after_check()
        self.assertEqual(caught.exception.reason, "file-open-read")
        self.assertTrue((self.root / "history-plan-table-after.json").exists())
        self.assertEqual(self.aws.calls[-1][1], "list-objects-v2")

    def test_failed_init_plan_or_validation_still_check_post_state_metadata_lock_without_pass(self):
        for key in ("BACKEND_INIT_OUTCOME", "TERRAFORM_PLAN_OUTCOME", "PLAN_VALIDATION_OUTCOME", "PLAN_EXIT_CODE"):
            with self.subTest(key=key):
                child = ReceiptTests("runTest"); child.setUp(); self.addCleanup(child.doCleanups)
                child.baseline(); child.env[key] = "failure"
                with self.assertRaises(ValueError): child.after_check()
                self.assertTrue((child.root / "history-plan-state-after.json").exists())
                self.assertTrue((child.root / "history-plan-table-after.json").exists())
                self.assertEqual(child.aws.calls[-1][1], "list-objects-v2")
                self.assertFalse((child.root / "history-plan-after.json").exists())

    def test_expired_cost_evidence_still_gets_bounded_post_safety_reads_but_cannot_pass(self):
        self.baseline()
        expired = datetime(2026, 10, 8, 14, tzinfo=timezone.utc)
        with patch.object(receipt, "cost_policy", side_effect=lambda raw, sha, now=None: ACTUAL_COST_POLICY(raw, sha, now=now or expired)):
            with self.assertRaises(ValueError): self.after_check()
        self.assertEqual(self.aws.calls[-1][1], "list-objects-v2")
        self.assertEqual(len(self.google_calls), 1)
        self.assertTrue((self.root / "history-plan-table-after.json").exists())
        self.assertFalse((self.root / "history-plan-after.json").exists())

    def test_cli_dependency_error_is_suppressed_and_does_not_retry_or_emit_pass(self):
        self.baseline(); self.aws.denied = "head-object"
        stdout, stderr = io.StringIO(), io.StringIO(); count = len(self.aws.calls)
        with patch.dict(os.environ, self.env, clear=True), patch.object(receipt, "aws", side_effect=self.aws), \
             patch.object(receipt, "google", side_effect=self.google), contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            self.assertEqual(receipt.main(["--phase", "after"]), 3)
        self.assertEqual([a[1] for a in self.aws.calls[count:]], ["head-object", "list-objects-v2"])
        self.assertEqual(len(self.google_calls), 1)
        exposed = stdout.getvalue() + stderr.getvalue()
        self.assertIn("STOP", exposed); self.assertNotIn("PASS", exposed); self.assertNotIn("DUMMY", exposed)
        safety = (self.root / "summary").read_text()
        self.assertIn("Persistent state invariant: STOP", safety)
        self.assertIn("Exact native lock absence: PASS", safety)
        self.assertNotIn("receipt: PASS", safety)
        self.assertNotIn("DUMMY", safety)

    def test_403_metadata_and_boolean_import_summary_cannot_pass(self):
        for case in ("metadata403", "importbool"):
            child = ReceiptTests("runTest"); child.setUp(); self.addCleanup(child.doCleanups); child.baseline()
            if case == "metadata403":
                with self.assertRaises(ValueError):
                    receipt.after(child.env, request=child.aws, metadata_request=lambda *a, **k: (403, {"error": "DUMMY_ERROR"}))
            else:
                path = child.root / "sanitized-plan.json"; value = receipt.private_json(str(path)); value["counts"]["import"] = True
                path.write_text(json.dumps(value))
                with self.assertRaises(ValueError): child.after_check()
            self.assertFalse((child.root / "history-plan-after.json").exists())

    def test_cli_private_success_and_dependency_failure_never_expose_payloads(self):
        self.baseline()
        stdout, stderr = io.StringIO(), io.StringIO()
        with patch.dict(os.environ, self.env, clear=True), patch.object(receipt, "aws", side_effect=self.aws), \
             patch.object(receipt, "google", side_effect=self.google), contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            self.assertEqual(receipt.main(["--phase", "after"]), 0)
        exposed = stdout.getvalue() + stderr.getvalue() + (self.root / "summary").read_text()
        self.assertIn("receipt: PASS", exposed)
        self.assertIn("History sanitized summary SHA-256: `" +
                      receipt.hashlib.sha256(receipt.private_bytes(self.root / "sanitized-plan.json")).hexdigest() + "`", exposed)
        self.assertIn("added current UTC month cost bound", exposed)
        self.assertIn("cost bound <= USD0.25.", exposed)
        self.assertNotIn("cost bound <= USD0.01.", exposed)
        self.assertIn("carry into later monthly cumulative costs", exposed)
        self.assertIn("no deletion deadline assumed", exposed)
        self.assertNotIn("monthly_ledger_entry", exposed)
        for secret in ("DUMMY", self.env["STATE_BUCKET"], str(self.root), "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"):
            self.assertNotIn(secret, exposed)

    def test_private_symlink_public_permissions_collision_and_bad_context_stop(self):
        path = self.root / "history-plan-cost.json"
        target = self.root / "unrelated"; target.write_text("DUMMY_KEEP")
        path.symlink_to(target)
        with self.assertRaises(ReceiptDependencyError) as caught: receipt.policy(self.env)
        self.assertEqual(caught.exception.reason, "file-open-write")
        self.assertEqual(target.read_text(), "DUMMY_KEEP")
        path.unlink(); receipt.policy(self.env); path.chmod(0o644)
        with self.assertRaises(smoke.StageFailure) as caught: receipt.snapshot_before(self.env, request=self.aws)
        self.assertEqual((caught.exception.stage, caught.exception.category), ("history-policy", "invalid-evidence"))
        self.assertEqual(self.aws.calls, [])
        for changes in ({"MODE": "apply"}, {"TF_WORKSPACE": "foreign"}, {"TF_VAR_project_id": "youtube-study-space"}):
            with self.assertRaises(ValueError): receipt.policy(dict(self.env, **changes))


if __name__ == "__main__":
    unittest.main()
