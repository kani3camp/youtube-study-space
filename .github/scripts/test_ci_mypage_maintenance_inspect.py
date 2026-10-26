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
        "INPUT_SERVICE_URL": "https://mypage-maintenance-inspect-48101442817.asia-southeast2.run.app",
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


def receipt(state="ACTIVE"):
    value = {
        "environment": "development",
        "state": state,
        "revision": 42,
        "updatedAt": "2026-10-27T02:00:00.123456Z",
        "messageTemplate": "MYPAGE_MAINTENANCE_V1",
        "reasonCategory": "PLANNED_MAINTENANCE" if state == "ACTIVE" else "RECOVERY",
        "authoritativeReadbackVerified": True,
        "readOnly": True,
    }
    if state == "ACTIVE":
        value["estimatedEndAt"] = "2026-10-27T03:00:00Z"
    return value


class MaintenanceInspectTest(unittest.TestCase):
    def test_accepts_only_exact_environment_bound_context(self):
        result = subject.context(values().get)
        self.assertEqual(result["project"], "test-youtube-study-space")
        self.assertEqual(result["service_account"], "mypage-maint-inspect-invoker@test-youtube-study-space.iam.gserviceaccount.com")
        self.assertEqual(result["provider"], "projects/48101442817/locations/global/workloadIdentityPools/mypage-maintenance/providers/github")
        self.assertEqual(result["service_url"], "https://mypage-maintenance-inspect-48101442817.asia-southeast2.run.app")
        prod = values()
        prod.update(
            INPUT_ENVIRONMENT="production",
            INPUT_PRODUCTION_CONFIRMATION="PRODUCTION INSPECT",
            INPUT_SERVICE_URL="https://mypage-maintenance-inspect-906336399194.asia-northeast2.run.app",
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
            "INPUT_SERVICE_URL": "https://attacker.example/mypage-maintenance-inspect",
        }.items():
            candidate = values()
            candidate[key] = bad
            with self.subTest(key=key), self.assertRaises(ValueError):
                subject.context(candidate.get)

    def test_parses_only_exact_sanitized_authoritative_receipt(self):
        parsed = subject.parse_receipt(receipt(), "development")
        self.assertEqual(parsed["revision"], 42)
        self.assertEqual(parsed["state"], "ACTIVE")
        self.assertTrue(parsed["readOnly"])
        self.assertEqual(parsed["estimatedEndAt"], "2026-10-27T03:00:00Z")
        normal = subject.parse_receipt(receipt("NORMAL"), "development")
        self.assertNotIn("estimatedEndAt", normal)
        mutations = (
            lambda value: value.update(privateField="private"),
            lambda value: value.update(revision=0),
            lambda value: value.update(revision="42"),
            lambda value: value.update(state="NOTICE"),
            lambda value: value.update(environment="production"),
            lambda value: value.update(messageTemplate="free text"),
            lambda value: value.update(estimatedEndAt="invalid"),
            lambda value: value.pop("reasonCategory"),
            lambda value: value.update(authoritativeReadbackVerified=False),
            lambda value: value.update(readOnly=False),
        )
        for mutate in mutations:
            candidate = deepcopy(receipt())
            mutate(candidate)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                subject.parse_receipt(candidate, "development")

    def test_workflow_is_manual_read_only_and_short_lived(self):
        workflow = (ROOT / ".github/workflows/mypage-maintenance-inspect.yml").read_text()
        self.assertIn("on:\n  workflow_dispatch:", workflow)
        for trigger in ("pull_request:", "push:", "schedule:", "workflow_call:"):
            self.assertNotIn(trigger, workflow)
        self.assertLess(workflow.index("Reject untrusted context before authentication"), workflow.index("google-github-actions/auth@"))
        self.assertIn("mypage-maintenance-inspect-${{ inputs.environment }}", workflow)
        self.assertIn("MYPAGE_MAINTENANCE_INSPECT_SERVICE_ACCOUNT", workflow)
        self.assertIn("MYPAGE_MAINTENANCE_INSPECT_SERVICE_URL", workflow)
        self.assertIn("token_format: id_token", workflow)
        self.assertNotIn("steps.auth.outputs.access_token", workflow)
        self.assertNotIn("firestore.googleapis.com", workflow)
        terraform = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertIn('command = ["/app/mypage-maintenance-inspect"]', terraform)
        self.assertIn('account_id   = "mypage-maint-inspect-invoker"', terraform)
        self.assertIn('member   = "serviceAccount:${local.maintenance_inspect_invoker_email}"', terraform)
        self.assertIn("retention-days: 7", workflow)
        self.assertNotIn("gcloud run jobs execute", workflow)
        self.assertNotIn("mypage-maintenance-invoker@", workflow)
        self.assertNotIn("set -x", workflow)


if __name__ == "__main__":
    unittest.main()
