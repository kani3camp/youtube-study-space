import copy
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from test_terraform_export_topic_gate import fixture as topic_fixture
from terraform_export_scheduler_gate import SCHEDULER, SCHEDULER_ID, validate
from terraform_export_topic_gate import TOPIC_ID
from terraform_plan_summary import build_summary


def fixture(importing=True):
    plan = topic_fixture(False)
    value = {"id": SCHEDULER_ID, "project": "test-youtube-study-space", "region": "asia-southeast2",
        "name": "scheduledFirestoreCollectionsExport", "schedule": "0 0 * * *", "time_zone": "Asia/Tokyo",
        "paused": False, "description": "", "attempt_deadline": "", "http_target": [], "app_engine_http_target": [],
        "retry_config": [{"retry_count": 3, "max_retry_duration": "0s", "min_backoff_duration": "5s",
            "max_backoff_duration": "3600s", "max_doublings": 5}],
        "pubsub_target": [{"topic_name": TOPIC_ID, "data": "c3RhcnQgZXhwb3J0"}]}
    change = {"actions": ["no-op"], "before": value, "after": copy.deepcopy(value), "after_unknown": {}}
    if importing:
        change["importing"] = {"id": SCHEDULER_ID}
    plan["resource_changes"].append({"address": SCHEDULER, "mode": "managed", "type": "google_cloud_scheduler_job", "change": change})
    return plan


class ExportSchedulerGateTest(unittest.TestCase):
    def test_exact_import_then_complete_persistent_noop(self):
        validate(fixture(), phase="before")
        validate(fixture(False), phase="before")
        validate(fixture(False), phase="post")
        with self.assertRaises(ValueError): validate(fixture(), phase="post")
        summary = build_summary(fixture(), environment="dev", git_sha="a" * 40, policy="import-only")
        self.assertTrue(summary["policy_passed"])
        self.assertEqual(summary["counts"]["import"], 1)
        self.assertEqual(summary["counts"]["no-op"], 10)
        self.assertNotIn(SCHEDULER_ID, str(summary))

    def test_existing_ownership_actions_unknowns_drift_and_import_scope_are_fixed(self):
        mutations = [
            lambda p: p["resource_changes"][0]["change"].update(actions=["update"]),
            lambda p: p["resource_changes"][-1]["change"].update(actions=["create"]),
            lambda p: p["resource_changes"][-1]["change"].update(after_unknown={"schedule": True}),
            lambda p: p.update(resource_drift=[{}]),
            lambda p: p.update(deferred_changes=[{}]),
            lambda p: p.update(complete=False),
            lambda p: p.update(errored=True),
            lambda p: p.update(checks=[{"status": "fail"}]),
            lambda p: p["resource_changes"].pop(0),
            lambda p: p["resource_changes"][-1].update(address="google_pubsub_subscription.generated"),
            lambda p: p["resource_changes"][-1]["change"]["importing"].update(id="foreign"),
            lambda p: p["resource_changes"][-2]["change"].update(importing={"id": TOPIC_ID}),
            lambda p: p["output_changes"]["project_id"].update(actions=["update"]),
        ]
        for mutate in mutations:
            plan = fixture(); mutate(plan)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError): validate(plan, phase="before")

    def test_remote_delivery_semantics_cannot_be_silently_redefined(self):
        mutations = [
            {"schedule": "* * * * *"}, {"time_zone": "UTC"}, {"paused": True}, {"region": "asia-northeast2"},
            {"description": "new"}, {"attempt_deadline": "180s"}, {"http_target": [{"uri": "unexpected"}]},
            {"pubsub_target": [{"topic_name": "foreign", "data": "c3RhcnQgZXhwb3J0"}]},
            {"pubsub_target": [{"topic_name": TOPIC_ID, "data": ""}]}, {"retry_config": []},
        ]
        for values in mutations:
            plan = fixture()
            for side in ("before", "after"): plan["resource_changes"][-1]["change"][side].update(values)
            with self.subTest(values=values), self.assertRaises(ValueError): validate(plan, phase="before")

    def test_protected_entrypoint_is_dev_only_dependency_bound_and_value_free(self):
        from terraform_protected_plan import main
        for phase, importing, env, topic, rc in [("before", True, "dev", "true", 0), ("post", False, "dev", "true", 0),
                ("post", True, "dev", "true", 3), ("before", True, "prod", "true", 3), ("before", True, "dev", "false", 3)]:
            with tempfile.TemporaryDirectory() as directory:
                args = ["gate", "--operation", "apply", "--phase", phase, "--environment", env, "--git-sha", "a" * 40,
                    "--policy", "import-only", "--json-output", directory + "/summary.json", "--markdown-output", directory + "/summary.md"]
                with patch("sys.argv", args), patch("sys.stdin", io.StringIO(json.dumps(fixture(importing)))), patch("sys.stderr", io.StringIO()), \
                        patch.dict("os.environ", {"TF_VAR_manage_export_scheduler": "true", "TF_VAR_manage_export_topic": topic}):
                    self.assertEqual(main(), rc)
                if rc == 0:
                    text = Path(directory + "/summary.json").read_text()
                    self.assertNotIn(SCHEDULER_ID, text)
                    self.assertEqual(json.loads(text)["counts"]["no-op"], 10)


if __name__ == "__main__": unittest.main()
