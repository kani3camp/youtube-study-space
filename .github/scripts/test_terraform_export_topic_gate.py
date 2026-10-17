import copy
import unittest
import io
import json
import tempfile
from unittest.mock import patch
from pathlib import Path

from test_terraform_quota_create_gate import fixture as quota_fixture
from terraform_export_topic_gate import TOPIC, TOPIC_ID, validate
from terraform_plan_summary import build_summary


def fixture(importing=True):
    plan = quota_fixture("post")
    plan.update(complete=True, errored=False, output_changes={k: {"actions": ["no-op"], "before": k, "after": k, "after_unknown": False} for k in ("environment", "project_id")})
    value = {"id": TOPIC_ID, "project": "test-youtube-study-space", "name": "initiateFirestoreCollectionsExport", "labels": {}}
    change = {"actions": ["no-op"], "before": value, "after": copy.deepcopy(value), "after_unknown": {}}
    if importing:
        change["importing"] = {"id": TOPIC_ID}
    plan["resource_changes"].append({"address": TOPIC, "mode": "managed", "type": "google_pubsub_topic", "change": change})
    return plan


class ExportTopicGateTest(unittest.TestCase):
    def test_protected_entrypoint_enforces_dev_and_post_contract_without_leak(self):
        from terraform_protected_plan import main
        for phase, plan, environment, rc in [("before", fixture(), "dev", 0),
                                            ("post", fixture(False), "dev", 0),
                                            ("post", fixture(), "dev", 3),
                                            ("before", fixture(), "prod", 3)]:
            with tempfile.TemporaryDirectory() as directory:
                args = ["gate", "--operation", "apply", "--phase", phase, "--environment", environment,
                        "--git-sha", "a" * 40, "--policy", "import-only", "--json-output", directory + "/summary.json",
                        "--markdown-output", directory + "/summary.md"]
                with patch("sys.argv", args), patch("sys.stdin", io.StringIO(json.dumps(plan))), patch("sys.stderr", io.StringIO()), patch.dict("os.environ", {"TF_VAR_manage_export_topic": "true"}):
                    self.assertEqual(main(), rc)
                if rc == 0:
                    summary = Path(directory + "/summary.json").read_text()
                    self.assertNotIn(TOPIC_ID, summary)
                    self.assertEqual(json.loads(summary)["counts"]["no-op"], 9)

    def test_exact_import_and_persistent_post_noop(self):
        validate(fixture(), phase="before")
        validate(fixture(False), phase="before")
        validate(fixture(False), phase="post")
        summary = build_summary(fixture(), environment="dev", git_sha="a" * 40, policy="import-only")
        self.assertTrue(summary["policy_passed"])
        self.assertEqual(summary["counts"]["import"], 1)
        self.assertEqual(summary["counts"]["no-op"], 9)
        self.assertNotIn(TOPIC_ID, str(summary))
        with self.assertRaises(ValueError): validate(fixture(), phase="post")

    def test_any_other_action_drift_unknown_missing_or_generated_ownership_stops(self):
        mutations = [
            lambda p: p["resource_changes"][0]["change"].update(actions=["update"]),
            lambda p: p["resource_changes"][-1]["change"].update(after_unknown={"labels": True}),
            lambda p: p.update(resource_drift=[{}]),
            lambda p: p.update(deferred_changes=[{}]),
            lambda p: p["resource_changes"].pop(0),
            lambda p: p["resource_changes"][-1].update(address="google_pubsub_subscription.generated"),
            lambda p: p["resource_changes"][0]["change"].update(importing={"id": "foreign"}),
            lambda p: p["resource_changes"][-1]["change"]["importing"].update(id="foreign"),
            lambda p: p["resource_changes"][-1]["change"]["after"].update(labels={"unexpected": "value"}),
            lambda p: p["output_changes"]["project_id"].update(actions=["update"]),
        ]
        for mutate in mutations:
            plan = fixture(); mutate(plan)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError): validate(plan, phase="before")


if __name__ == "__main__": unittest.main()
