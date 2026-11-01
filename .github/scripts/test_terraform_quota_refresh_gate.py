import copy
import unittest
import json
import io
import tempfile
from pathlib import Path
from unittest.mock import patch

from test_terraform_email_adoption_gate import EMAIL, NAME
from test_terraform_quota_create_gate import fixture as quota_fixture
from terraform_quota_create_gate import ADDRESSES
from terraform_quota_refresh_gate import validate
from terraform_plan_summary import build_summary


def fixture():
    post = quota_fixture("post")
    graph = [{"address": r["address"], "mode": r["mode"], "type": r.get("type", r["address"].split(".")[-2]), "values": r["change"].get("after", {"id": "existing-fixture"})}
             for r in post["resource_changes"]]
    drift = []
    for r in post["resource_changes"]:
        if r["address"] not in ADDRESSES:
            continue
        r = copy.deepcopy(r)
        r["change"].update(actions=["update"], before_sensitive={"user_labels": None}, after_sensitive={"user_labels": {}})
        r["change"]["before"]["user_labels"] = None
        drift.append(r)
    return {"complete": True, "applyable": True, "errored": False, "planned_values": {"root_module": {}, "outputs": {}},
            "prior_state": {"values": {"root_module": {"resources": graph}}}, "resource_drift": drift,
            "output_changes": {k: {"actions": ["no-op"], "before": k, "after": k, "after_unknown": False}
                               for k in ("environment", "project_id")}, "checks": [{"status": "pass"}]}


class QuotaRefreshTest(unittest.TestCase):
    def check(self, plan):
        validate(plan, channel_name=NAME, email=EMAIL)

    def test_exact_state_representation_only_preserves_global_stop_and_public_counts(self):
        plan = fixture()
        self.check(plan)
        summary = build_summary(plan, environment="dev", git_sha="a" * 40, policy="import-only")
        self.assertFalse(summary["policy_passed"])
        self.assertEqual(summary["counts"]["drift"], 3)
        self.assertTrue(all(v == 0 for k, v in summary["counts"].items() if k != "drift"))
        self.assertNotIn(NAME, str(summary)); self.assertNotIn(EMAIL, str(summary))

    def test_protected_entrypoint_is_dev_only_and_post_requires_noop_eight(self):
        from terraform_protected_plan import main
        for phase, plan, environment, expected_rc in [
            ("before", fixture(), "dev", 0), ("post", quota_fixture("post"), "dev", 0),
            ("before", fixture(), "prod", 3), ("post", fixture(), "dev", 3),
        ]:
            with tempfile.TemporaryDirectory() as directory:
                args = ["gate", "--operation", "quota-refresh", "--phase", phase, "--environment", environment,
                        "--git-sha", "a" * 40, "--policy", "import-only", "--json-output", directory + "/summary.json",
                        "--markdown-output", directory + "/summary.md"]
                with patch("sys.argv", args), patch("sys.stdin", io.StringIO(json.dumps(plan))), patch("sys.stderr", io.StringIO()), patch.dict("os.environ", {"TF_VAR_primary_email_channel_name": NAME, "TF_VAR_primary_email_address": EMAIL}):
                    self.assertEqual(main(), expected_rc)
                if expected_rc == 0:
                    summary = json.loads(Path(directory + "/summary.json").read_text())
                    self.assertEqual(summary["counts"]["drift"], 3 if phase == "before" else 0)
                    self.assertEqual(summary["counts"]["no-op"], 0 if phase == "before" else 8)
                    self.assertNotIn(NAME, str(summary)); self.assertNotIn(EMAIL, str(summary))

    def test_any_action_unknown_output_graph_or_other_drift_stops(self):
        mutations = [
            lambda p: p.update(resource_changes=quota_fixture("post")["resource_changes"]),
            lambda p: p.update(deferred_changes=[{}]),
            lambda p: p.update(complete=False),
            lambda p: p.update(errored=True),
            lambda p: p.update(planned_values={"root_module": {"resources": [{}]}}),
            lambda p: p["checks"][0].update(status="unknown"),
            lambda p: p["prior_state"]["values"]["root_module"]["resources"].pop(0),
            lambda p: p["resource_drift"].pop(),
            lambda p: p["resource_drift"].append(copy.deepcopy(p["resource_drift"][0])),
            lambda p: p["resource_drift"][0].update(address="unexpected"),
            lambda p: p["resource_drift"][0]["change"].update(after_unknown={"name": True}),
            lambda p: p["resource_drift"][0]["change"].update(importing={"id": "unexpected"}),
            lambda p: p["resource_drift"][0]["change"].update(actions=["delete", "create"]),
            lambda p: p["output_changes"]["project_id"].update(after="unexpected"),
        ]
        for mutate in mutations:
            plan = fixture(); mutate(plan)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError): self.check(plan)

    def test_real_label_or_any_config_metadata_or_sensitivity_change_stops(self):
        mutations = [
            lambda c: c["after"].update(user_labels={"unexpected": "value"}),
            lambda c: c["before"].update(user_labels={}),
            lambda c: c["before"].update(enabled=False),
            lambda c: c["before"].update(name="projects/foreign/alertPolicies/foreign"),
            lambda c: c["before_sensitive"].update(notification_channels=True),
            lambda c: c["after_sensitive"].update(user_labels=True),
        ]
        for mutate in mutations:
            plan = fixture(); mutate(plan["resource_drift"][0]["change"])
            with self.subTest(mutate=mutate), self.assertRaises(ValueError): self.check(plan)

    def test_approved_policy_semantics_and_adopted_email_are_required(self):
        for field, value in [("enabled", False), ("notification_channels", ["foreign"]), ("project", "youtube-study-space")]:
            plan = fixture()
            drift = plan["resource_drift"][0]
            drift["change"]["before"][field] = value
            drift["change"]["after"][field] = value
            graph = plan["prior_state"]["values"]["root_module"]["resources"]
            next(r for r in graph if r["address"] == drift["address"])["values"][field] = value
            with self.subTest(field=field), self.assertRaises(ValueError): self.check(plan)
        with self.assertRaises(ValueError): validate(fixture(), channel_name=NAME, email="foreign")


if __name__ == "__main__": unittest.main()
