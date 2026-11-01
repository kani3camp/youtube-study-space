import copy
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from test_terraform_export_scheduler_gate import fixture as scheduler_fixture
from terraform_export_function_gate import FUNCTION, FUNCTION_ID, validate
from terraform_export_topic_gate import TOPIC_ID
from terraform_plan_summary import build_summary

EMAIL = "existing-execution@example.invalid"


def fixture(importing=True):
    plan = scheduler_fixture(False)
    value = {"id": FUNCTION_ID, "project": "test-youtube-study-space", "region": "asia-southeast2",
        "name": "firestoreCollectionsExport", "runtime": "nodejs22", "entry_point": "scheduledFirestoreExport",
        "available_memory_mb": 256, "timeout": 60, "min_instances": 0, "max_instances": 1,
        "service_account_email": EMAIL, "ingress_settings": "ALLOW_ALL", "status": "ACTIVE", "version_id": "8",
        "environment_variables": {"YSS_EXPORT_ENVIRONMENT": "development", "YSS_EXPORT_PROJECT_ID": "test-youtube-study-space"},
        "effective_labels": {"deployment-tool": "cli-gcloud"},
        "event_trigger": [{"event_type": "google.pubsub.topic.publish", "resource": TOPIC_ID, "failure_policy": [{"retry": False}]}]}
    change = {"actions": ["no-op"], "before": value, "after": copy.deepcopy(value), "after_unknown": {}}
    if importing: change["importing"] = {"id": FUNCTION_ID}
    plan["resource_changes"].append({"address": FUNCTION, "mode": "managed", "type": "google_cloudfunctions_function", "change": change})
    return plan


class ExportFunctionGateTest(unittest.TestCase):
    def test_import_and_persistent_complete_noop(self):
        for phase, importing in [("before", True), ("before", False), ("post", False)]:
            validate(fixture(importing), phase=phase, execution_email=EMAIL)
        with self.assertRaises(ValueError): validate(fixture(), phase="post", execution_email=EMAIL)
        summary = build_summary(fixture(), environment="dev", git_sha="a" * 40, policy="import-only")
        self.assertTrue(summary["policy_passed"])
        self.assertEqual((summary["counts"]["import"], summary["counts"]["no-op"]), (1, 11))
        self.assertNotIn(EMAIL, str(summary))
        self.assertNotIn(FUNCTION_ID, str(summary))

    def test_exact_graph_and_existing_ten_cannot_change(self):
        mutations = [lambda p: p["resource_changes"].pop(0),
            lambda p: p["resource_changes"].append(copy.deepcopy(p["resource_changes"][-1])),
            lambda p: p["resource_changes"][-1].update(address="google_pubsub_subscription.generated"),
            lambda p: p["resource_changes"][-1]["change"].update(actions=["create"]),
            lambda p: p["resource_changes"][-1]["change"].update(actions=["update"]),
            lambda p: p["resource_changes"][0]["change"].update(actions=["delete"]),
            lambda p: p["resource_changes"][-2]["change"].update(importing={"id": "second"}),
            lambda p: p["resource_changes"][-1]["change"]["importing"].update(id="foreign"),
            lambda p: p["resource_changes"][-1]["change"].update(after_unknown={"source_archive_object": True}),
            lambda p: p.update(resource_drift=[{}]), lambda p: p.update(deferred_changes=[{}]),
            lambda p: p.update(complete=False), lambda p: p.update(errored=True),
            lambda p: p.update(checks=[{"status": "unknown"}]),
            lambda p: p["output_changes"]["project_id"].update(actions=["update"])]
        for mutation in mutations:
            plan = fixture(); mutation(plan)
            with self.subTest(mutation=mutation), self.assertRaises(ValueError): validate(plan, phase="before", execution_email=EMAIL)

    def test_runtime_source_and_generated_ownership_boundaries(self):
        mutations = [{"runtime": "nodejs20"}, {"version_id": "9"}, {"status": "DEPLOY_IN_PROGRESS"},
            {"service_account_email": "other"}, {"max_instances": 2}, {"min_instances": 1}, {"timeout": 61},
            {"available_memory_mb": 512}, {"environment_variables": {}}, {"description": "new"},
            {"event_trigger": [{"resource": "foreign"}]}, {"effective_labels": {}},
            *[{key: "new"} for key in ("source_archive_bucket", "source_archive_object", "source_repository", "labels",
                "terraform_labels", "trigger_http", "vpc_connector", "kms_key_name", "docker_repository", "build_worker_pool")]]
        for values in mutations:
            plan = fixture()
            for side in ("before", "after"): plan["resource_changes"][-1]["change"][side].update(values)
            with self.subTest(values=values), self.assertRaises(ValueError): validate(plan, phase="before", execution_email=EMAIL)
        with self.assertRaises(ValueError): validate(fixture(), phase="before", execution_email="")

    def test_entrypoint_requires_dev_dependencies_and_private_identity(self):
        from terraform_protected_plan import main
        for phase, importing, env, topic, scheduler, email, rc in [
            ("before", True, "dev", "true", "true", EMAIL, 0), ("post", False, "dev", "true", "true", EMAIL, 0),
            ("post", True, "dev", "true", "true", EMAIL, 3), ("before", True, "prod", "true", "true", EMAIL, 3),
            ("before", True, "dev", "false", "true", EMAIL, 3), ("before", True, "dev", "true", "false", EMAIL, 3),
            ("before", True, "dev", "true", "true", "", 3)]:
            with tempfile.TemporaryDirectory() as directory:
                args = ["gate", "--operation", "apply", "--phase", phase, "--environment", env, "--git-sha", "a" * 40,
                    "--policy", "import-only", "--json-output", directory + "/summary.json", "--markdown-output", directory + "/summary.md"]
                with patch("sys.argv", args), patch("sys.stdin", io.StringIO(json.dumps(fixture(importing)))), patch("sys.stderr", io.StringIO()), \
                        patch.dict("os.environ", {"TF_VAR_manage_export_function": "true", "TF_VAR_manage_export_topic": topic,
                            "TF_VAR_manage_export_scheduler": scheduler, "TF_VAR_export_function_execution_service_account_email": email}):
                    self.assertEqual(main(), rc)
                if rc == 0:
                    text = Path(directory + "/summary.json").read_text()
                    self.assertNotIn(EMAIL, text)
                    self.assertEqual(json.loads(text)["counts"]["no-op"], 11)


if __name__ == "__main__": unittest.main()
