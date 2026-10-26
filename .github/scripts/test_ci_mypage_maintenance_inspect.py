from copy import deepcopy
from pathlib import Path
import unittest

import mypage_maintenance_inspect as subject


ROOT = Path(__file__).resolve().parents[2]


def values():
    sha = "a" * 40
    return {
        "INPUT_ENVIRONMENT": "development",
        "INPUT_EXPECTED_SHA": sha,
        "INPUT_PRODUCTION_CONFIRMATION": "",
        "GITHUB_SHA": sha,
        "GITHUB_REPOSITORY": subject.REPOSITORY,
        "GITHUB_REPOSITORY_ID": subject.REPOSITORY_ID,
        "GITHUB_REPOSITORY_OWNER_ID": subject.OWNER_ID,
        "GITHUB_ACTOR_ID": subject.OWNER_ID,
        "MYPAGE_TRIGGERING_ACTOR_ID": subject.OWNER_ID,
        "GITHUB_EVENT_NAME": "workflow_dispatch",
        "GITHUB_RUN_ATTEMPT": "1",
        "GITHUB_RUN_ID": "123456789",
        "GITHUB_REF": "refs/heads/integration/mypage-canon-20261006",
        "GITHUB_WORKFLOW_REF": f"{subject.REPOSITORY}/{subject.WORKFLOW}@refs/heads/integration/mypage-canon-20261006",
    }


def document(state="ACTIVE"):
    project = "test-youtube-study-space"
    fields = {
        "environment": {"stringValue": "development"},
        "state": {"stringValue": state},
        "revision": {"integerValue": "42"},
        "updatedAt": {"timestampValue": "2026-10-27T02:00:00.123456Z"},
        "messageTemplate": {"stringValue": "MYPAGE_MAINTENANCE_V1"},
        "reasonCategory": {"stringValue": "PLANNED_MAINTENANCE" if state == "ACTIVE" else "RECOVERY"},
    }
    if state == "ACTIVE":
        fields["estimatedEndAt"] = {"timestampValue": "2026-10-27T03:00:00Z"}
    return {
        "name": f"projects/{project}/databases/(default)/documents/mypage-runtime-control/current",
        "fields": fields,
        "createTime": "2026-10-27T01:00:00Z",
        "updateTime": "2026-10-27T02:00:00.123456Z",
    }


class MaintenanceInspectTest(unittest.TestCase):
    def test_accepts_only_exact_environment_bound_context(self):
        result = subject.context(values().get)
        self.assertEqual(result["project"], "test-youtube-study-space")
        self.assertEqual(result["service_account"], "mypage-maintenance-bootstrap@test-youtube-study-space.iam.gserviceaccount.com")
        self.assertEqual(result["provider"], "projects/48101442817/locations/global/workloadIdentityPools/mypage-maintenance/providers/github")
        prod = values()
        prod.update(
            INPUT_ENVIRONMENT="production",
            INPUT_PRODUCTION_CONFIRMATION="PRODUCTION INSPECT",
            GITHUB_REF="refs/heads/main",
            GITHUB_WORKFLOW_REF=f"{subject.REPOSITORY}/{subject.WORKFLOW}@refs/heads/main",
        )
        self.assertEqual(subject.context(prod.get)["project"], "youtube-study-space")
        for key, bad in {
            "INPUT_ENVIRONMENT": "prod",
            "INPUT_EXPECTED_SHA": "b" * 40,
            "GITHUB_REPOSITORY_ID": "1",
            "GITHUB_REPOSITORY_OWNER_ID": "1",
            "GITHUB_ACTOR_ID": "1",
            "MYPAGE_TRIGGERING_ACTOR_ID": "1",
            "GITHUB_EVENT_NAME": "pull_request",
            "GITHUB_RUN_ATTEMPT": "2",
            "GITHUB_REF": "refs/heads/main",
            "GITHUB_WORKFLOW_REF": "untrusted",
        }.items():
            candidate = values()
            candidate[key] = bad
            with self.subTest(key=key), self.assertRaises(ValueError):
                subject.context(candidate.get)

    def test_parses_only_exact_authoritative_firestore_document(self):
        receipt = subject.parse_document(document(), "development", "test-youtube-study-space")
        self.assertEqual(receipt["revision"], 42)
        self.assertEqual(receipt["state"], "ACTIVE")
        self.assertTrue(receipt["readOnly"])
        self.assertEqual(receipt["estimatedEndAt"], "2026-10-27T03:00:00Z")
        normal = subject.parse_document(document("NORMAL"), "development", "test-youtube-study-space")
        self.assertNotIn("estimatedEndAt", normal)
        mutations = (
            lambda value: value["fields"].update(privateField={"stringValue": "private"}),
            lambda value: value["fields"]["revision"].update(integerValue="0"),
            lambda value: value["fields"]["revision"].update(integerValue="not-integer"),
            lambda value: value["fields"]["state"].update(stringValue="NOTICE"),
            lambda value: value["fields"]["environment"].update(stringValue="production"),
            lambda value: value["fields"]["messageTemplate"].update(stringValue="free text"),
            lambda value: value["fields"]["estimatedEndAt"].update(timestampValue="invalid"),
            lambda value: value["fields"].pop("reasonCategory"),
            lambda value: value.update(name="projects/other/databases/(default)/documents/mypage-runtime-control/current"),
        )
        for mutate in mutations:
            candidate = deepcopy(document())
            mutate(candidate)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                subject.parse_document(candidate, "development", "test-youtube-study-space")

    def test_workflow_is_manual_read_only_and_short_lived(self):
        workflow = (ROOT / ".github/workflows/mypage-maintenance-inspect.yml").read_text()
        self.assertIn("on:\n  workflow_dispatch:", workflow)
        for trigger in ("pull_request:", "push:", "schedule:", "workflow_call:"):
            self.assertNotIn(trigger, workflow)
        self.assertLess(workflow.index("Reject untrusted context before authentication"), workflow.index("google-github-actions/auth@"))
        self.assertIn("mypage-maintenance-inspect-${{ inputs.environment }}", workflow)
        self.assertIn("MYPAGE_MAINTENANCE_INSPECT_SERVICE_ACCOUNT", workflow)
        self.assertIn("mypage-maintenance-bootstrap@", (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text())
        self.assertIn("retention-days: 7", workflow)
        self.assertNotIn("gcloud run jobs execute", workflow)
        self.assertNotIn("mypage-maintenance-invoker@", workflow)
        self.assertNotIn("set -x", workflow)


if __name__ == "__main__":
    unittest.main()
