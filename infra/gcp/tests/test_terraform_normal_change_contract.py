#!/usr/bin/env python3
"""Negative-first, credentialless normal-change proposal tests (synthetic only)."""
from __future__ import annotations

from copy import deepcopy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "infra/gcp/scripts"))
import terraform_normal_change_contract as policy

ADDRESS = "module.mypage.google_cloud_run_v2_service.api"
SECRET = "PRIVATE_NORMAL_CHANGE_CANARY_NOT_FOR_PUBLIC_OUTPUT"


def review(action="create"):
    return {"schema_version": policy.SCHEMA, "environment": "dev",
            "changes": [{"address": ADDRESS, "type": "google_cloud_run_v2_service",
                         "action": action}]}


def resource(action="create"):
    return {
        "address": ADDRESS, "mode": "managed", "type": "google_cloud_run_v2_service",
        "provider_name": policy.PROVIDER,
        "change": {"actions": [action],
                   "before": None if action == "create" else {"labels": {"version": "old"}},
                   "after": {"labels": {"version": SECRET}},
                   "after_unknown": {"id": True} if action == "create" else {}},
    }


def plan(action="create"):
    return {"format_version": "1.2", "terraform_version": policy.TERRAFORM_VERSION,
            "complete": True, "errored": False, "resource_changes": [
                {"address": "module.existing.google_storage_bucket.backup",
                 "mode": "managed", "type": "google_storage_bucket",
                 "provider_name": policy.PROVIDER,
                 "change": {"actions": ["no-op"], "before": {"private": SECRET},
                            "after": {"private": SECRET}}},
                resource(action)]}


class OfflinePolicyTest(unittest.TestCase):
    def test_exact_reviewed_create_and_update_allow_noop_baseline(self):
        for action in ("create", "update"):
            with self.subTest(action=action):
                outcome = policy.validate(plan(action), review(action))
                self.assertEqual(outcome["create"], int(action == "create"))
                self.assertEqual(outcome["update"], int(action == "update"))
                self.assertEqual(outcome["no_op"], 1)
                self.assertEqual(outcome["policy"], "offline-only")
                self.assertNotIn(SECRET, str(outcome))
                self.assertNotIn(ADDRESS, str(outcome))

    def test_empty_review_accepts_only_full_noop(self):
        base = plan()
        base["resource_changes"].pop()
        outcome = policy.validate(base, review() | {"changes": []})
        self.assertEqual((outcome["create"], outcome["update"], outcome["no_op"]), (0, 0, 1))

    def test_reject_nonapproved_or_dangerous_resource_actions(self):
        cases = [
            lambda p: p["resource_changes"][-1]["change"].update(actions=["delete"]),
            lambda p: p["resource_changes"][-1]["change"].update(actions=["delete", "create"]),
            lambda p: p["resource_changes"][-1]["change"].update(actions=["create", "delete"]),
            lambda p: p["resource_changes"][-1]["change"].update(actions=["read"]),
            lambda p: p["resource_changes"][-1]["change"].update(importing={"id": SECRET}),
            lambda p: p["resource_changes"][-1]["change"].update(replace_paths=[["labels"]]),
            lambda p: p["resource_changes"][-1].update(previous_address="google_fake.legacy"),
            lambda p: p["resource_changes"][-1].update(type="google_project_iam_policy"),
            lambda p: p["resource_changes"][-1].update(provider_name="attacker/unknown"),
            lambda p: p["resource_changes"][-1].update(mode="data"),
            lambda p: p["resource_changes"][-1].update(address="module.hijack.google_cloud_run_v2_service.api"),
            lambda p: p["resource_changes"].append(deepcopy(p["resource_changes"][-1])),
            lambda p: p["resource_changes"].append({
                "address": "google_project_iam_policy.project", "mode": "managed",
                "change": {"actions": ["create"], "after": {}}}),
            lambda p: p["resource_changes"].pop(),
            lambda p: p["resource_changes"][-1]["change"].update(before={"exists": True}),
            lambda p: p["resource_changes"][-1]["change"].update(after=None),
            lambda p: p["resource_changes"].insert(0, {
                "address": "data.google_secret_manager_secret_version.key", "mode": "data",
                "change": {"actions": ["read"], "after": {"private": SECRET}}}),
        ]
        for mutate in cases:
            candidate = plan()
            mutate(candidate)
            with self.subTest(index=cases.index(mutate)), self.assertRaises(policy.Rejected) as raised:
                policy.validate(candidate, review())
            self.assertNotIn(SECRET, str(raised.exception))
            self.assertNotIn(ADDRESS, str(raised.exception))

    def test_reject_incomplete_plan_unknown_update_output_drift_and_checks(self):
        cases = [
            lambda p: p.update(format_version="0.9"),
            lambda p: p.update(terraform_version="1.16.3"),
            lambda p: p.update(complete=False),
            lambda p: p.update(errored=True),
            lambda p: p.update(resource_drift=[{"address": ADDRESS, "change": {"actions": ["update"]}}]),
            lambda p: p.update(deferred_changes=[{}]),
            lambda p: p.update(action_invocations=[{}]),
            lambda p: p.update(checks=[{"status": "fail"}]),
            lambda p: p.update(output_changes={"url": {"actions": ["create"], "after": SECRET}}),
        ]
        for mutate in cases:
            candidate = plan()
            mutate(candidate)
            with self.subTest(index=cases.index(mutate)), self.assertRaises(policy.Rejected):
                policy.validate(candidate, review())
        candidate = plan("update")
        candidate["resource_changes"][-1]["change"]["after_unknown"] = {"uri": True}
        with self.assertRaises(policy.Rejected):
            policy.validate(candidate, review("update"))

    def test_review_is_exact_bounded_and_dev_only(self):
        cases = [
            lambda r: r.update(environment="prod"),
            lambda r: r.update(schema_version="legacy"),
            lambda r: r.update(extra=SECRET),
            lambda r: r["changes"][0].update(type="google_project_iam_policy"),
            lambda r: r["changes"][0].update(action="delete"),
            lambda r: r["changes"][0].update(address="module.foo[0].google_cloud_run_v2_service.api"),
            lambda r: r["changes"].append(deepcopy(r["changes"][0])),
            lambda r: r["changes"].extend([{**r["changes"][0], "address": f"google_cloud_run_v2_service.api{i}"} for i in range(4)]),
        ]
        for mutate in cases:
            candidate = review()
            mutate(candidate)
            with self.subTest(index=cases.index(mutate)), self.assertRaises(policy.Rejected):
                policy.validate(plan(), candidate)

    def test_cli_never_prints_private_values_on_pass_or_stop(self):
        with tempfile.TemporaryDirectory() as directory:
            plan_file = Path(directory) / "plan.json"
            review_file = Path(directory) / "review.json"
            for legal in (True, False):
                candidate = plan()
                if not legal:
                    candidate["resource_changes"][-1]["change"]["actions"] = ["delete"]
                plan_file.write_text(json.dumps(candidate))
                review_file.write_text(json.dumps(review()))
                result = subprocess.run([
                    sys.executable, str(policy.__file__), "--plan", str(plan_file),
                    "--review", str(review_file)], capture_output=True, text=True, check=False)
                self.assertEqual(result.returncode, 0 if legal else 1)
                self.assertIn("PASS" if legal else "STOP", result.stdout)
                for value in (SECRET, ADDRESS, str(plan_file), str(review_file)):
                    self.assertNotIn(value, result.stdout + result.stderr)

    def test_duplicate_json_keys_and_oversize_rejected_before_plan(self):
        with tempfile.TemporaryDirectory() as directory:
            p = Path(directory) / "invalid.json"
            p.write_text('{"schema_version":"a","schema_version":"b"}')
            with self.assertRaises(policy.Rejected):
                policy.load_private(p)
            p.write_bytes(b"x" * (policy.MAX_BYTES + 1))
            with self.assertRaises(policy.Rejected):
                policy.load_private(p)


if __name__ == "__main__":
    unittest.main()
