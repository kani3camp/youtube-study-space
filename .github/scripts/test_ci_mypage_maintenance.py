from copy import deepcopy
from datetime import datetime, timezone
from pathlib import Path
import unittest

import mypage_maintenance as subject


ROOT = Path(__file__).resolve().parents[2]


def values():
    sha = "a" * 40
    return {
        "INPUT_ENVIRONMENT": "development",
        "INPUT_STATE": "ACTIVE",
        "INPUT_EXPECTED_REVISION": "7",
        "INPUT_ESTIMATED_END_AT": "2026-10-27T03:00:00Z",
        "INPUT_REASON_CATEGORY": "PLANNED_MAINTENANCE",
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


class MaintenanceContextTest(unittest.TestCase):
    def test_accepts_exact_development_transition(self):
        env = values()
        result = subject.context(env.get, lambda: datetime(2026, 10, 27, 2, tzinfo=timezone.utc))
        self.assertEqual(result["project"], "test-youtube-study-space")
        self.assertEqual(result["project_number"], "48101442817")
        self.assertEqual(result["provider"], "projects/48101442817/locations/global/workloadIdentityPools/mypage-maintenance/providers/github")
        self.assertEqual(result["public_origin"], "https://test--study-space.web.app")
        self.assertRegex(result["action_ref"], r"^[0-9a-f]{64}$")

    def test_accepts_explicit_initial_normal(self):
        env = values()
        env.update(INPUT_STATE="NORMAL", INPUT_EXPECTED_REVISION="0", INPUT_ESTIMATED_END_AT="", INPUT_REASON_CATEGORY="INITIALIZATION")
        self.assertEqual(subject.context(env.get)["state"], "NORMAL")

    def test_accepts_exact_production_confirmation(self):
        env = values()
        env.update(INPUT_ENVIRONMENT="production", INPUT_PRODUCTION_CONFIRMATION="PRODUCTION ACTIVE", GITHUB_REF="refs/heads/main", GITHUB_WORKFLOW_REF=f"{subject.REPOSITORY}/{subject.WORKFLOW}@refs/heads/main")
        result = subject.context(env.get, lambda: datetime(2026, 10, 27, 2, tzinfo=timezone.utc))
        self.assertEqual(result["project"], "youtube-study-space")
        self.assertEqual(result["project_number"], "906336399194")
        self.assertEqual(result["provider"], "projects/906336399194/locations/global/workloadIdentityPools/mypage-maintenance/providers/github")
        self.assertEqual(result["public_origin"], "https://moku.work")

    def test_rejects_every_trust_and_transition_mismatch(self):
        for key, bad in {
            "INPUT_ENVIRONMENT": "prod",
            "INPUT_STATE": "NOTICE",
            "INPUT_EXPECTED_REVISION": "-1",
            "INPUT_REASON_CATEGORY": "FREE_TEXT",
            "INPUT_EXPECTED_SHA": "b" * 40,
            "GITHUB_REPOSITORY": "fork/youtube-study-space",
            "GITHUB_REPOSITORY_ID": "1",
            "GITHUB_REPOSITORY_OWNER_ID": "1",
            "GITHUB_ACTOR_ID": "1",
            "MYPAGE_TRIGGERING_ACTOR_ID": "1",
            "GITHUB_EVENT_NAME": "pull_request",
            "GITHUB_RUN_ATTEMPT": "2",
            "GITHUB_REF": "refs/heads/feature",
            "GITHUB_WORKFLOW_REF": "untrusted",
        }.items():
            env = deepcopy(values())
            env[key] = bad
            with self.subTest(key=key), self.assertRaises(ValueError):
                subject.context(env.get, lambda: datetime(2026, 10, 27, 2, tzinfo=timezone.utc))

    def test_rejects_normal_with_estimate_and_bad_initialization(self):
        for changes in (
            {"INPUT_STATE": "NORMAL"},
            {"INPUT_EXPECTED_REVISION": "0"},
            {"INPUT_REASON_CATEGORY": "INITIALIZATION"},
            {"INPUT_ESTIMATED_END_AT": "2026-11-20T00:00:00Z"},
        ):
            env = values()
            env.update(changes)
            with self.subTest(changes=changes), self.assertRaises(ValueError):
                subject.context(env.get, lambda: datetime(2026, 10, 27, 2, tzinfo=timezone.utc))

    def test_public_status_contract_requires_exact_timestamps_and_state_shape(self):
        active = {
            "state": "ACTIVE",
            "updatedAt": "2026-10-27T02:00:00.123Z",
            "estimatedEndAt": "2026-10-27T03:00:00+09:00",
            "messageTemplate": "MYPAGE_MAINTENANCE_V1",
        }
        self.assertTrue(subject.valid_public_status(active, "ACTIVE"))
        normal = {key: value for key, value in active.items() if key != "estimatedEndAt"} | {"state": "NORMAL"}
        self.assertTrue(subject.valid_public_status(normal, "NORMAL"))
        for candidate, expected in (
            ({key: value for key, value in active.items() if key != "updatedAt"}, "ACTIVE"),
            (active | {"privateNote": "not public"}, "ACTIVE"),
            (active | {"updatedAt": "2026-99-27T02:00:00Z"}, "ACTIVE"),
            (active | {"estimatedEndAt": None}, "ACTIVE"),
            (normal | {"estimatedEndAt": "2026-10-27T03:00:00Z"}, "NORMAL"),
            (active | {"state": "NOTICE"}, "ACTIVE"),
            (active | {"messageTemplate": "free text"}, "ACTIVE"),
        ):
            with self.subTest(candidate=candidate, expected=expected):
                self.assertFalse(subject.valid_public_status(candidate, expected))


class MaintenanceWorkflowSourceContractTest(unittest.TestCase):
    def test_manual_only_preflight_and_signed_sender_wiring(self):
        workflow = (ROOT / ".github/workflows/mypage-maintenance.yml").read_text()
        self.assertIn("on:\n  workflow_dispatch:", workflow)
        for trigger in ("pull_request:", "push:", "schedule:", "workflow_call:"):
            self.assertNotIn(trigger, workflow)
        self.assertEqual(workflow.count("MYPAGE_TRIGGERING_ACTOR_ID: ${{ github.event.sender.id }}"), 2)
        self.assertNotIn("GITHUB_TRIGGERING_ACTOR_ID", workflow)
        self.assertLess(workflow.index("Reject untrusted context and invalid transition before authentication"), workflow.index("google-github-actions/auth@"))
        self.assertNotIn("set -x", workflow)

    def test_permissions_actions_identity_and_job_are_fixed(self):
        workflow = (ROOT / ".github/workflows/mypage-maintenance.yml").read_text()
        self.assertIn("permissions: {}", workflow)
        self.assertIn("id-token: write", workflow)
        self.assertIn("actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1", workflow)
        self.assertIn("google-github-actions/auth@7c6bc770dae815cd3e89ee6cdf493a5fab2cc093", workflow)
        self.assertIn("secrets.MYPAGE_MAINTENANCE_WIF_PROVIDER", workflow)
        self.assertIn("secrets.MYPAGE_MAINTENANCE_SERVICE_ACCOUNT", workflow)
        self.assertIn("EXPECTED_PROVIDER: ${{ steps.context.outputs.provider }}", workflow)
        self.assertIn('[[ "${PROVIDER}" == "${EXPECTED_PROVIDER}" ]]', workflow)
        self.assertNotIn("MYPAGE_GCP_WIF_PROVIDER", workflow)
        self.assertIn("gcloud run jobs execute mypage-maintenance-operator", workflow)
        self.assertIn("--args=\"${args}\"", workflow)
        self.assertNotIn("--authority=", workflow)
        self.assertIn("run_operator preview", workflow)
        self.assertIn("run_operator apply", workflow)
        self.assertIn("run_operator verify", workflow)
        self.assertLess(workflow.index("run_operator preview"), workflow.index("run_operator apply"))
        self.assertLess(workflow.index("run_operator apply"), workflow.index("run_operator verify"))
        self.assertIn("authoritativeReadbackVerified", workflow)
        self.assertIn("PUBLIC_ORIGIN: ${{ steps.context.outputs.public_origin }}", workflow)
        self.assertNotIn("vars.MYPAGE_PUBLIC_ORIGIN", workflow)
        self.assertIn("valid_public_status(candidate, expected)", workflow)
        self.assertIn("time.monotonic() + 30", workflow)
        self.assertIn("time.sleep(1)", workflow)
        self.assertIn("actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9", workflow)

    def test_terraform_trust_override_role_and_bootstrap_are_cross_checked(self):
        terraform = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertIn("assertion.workflow_sha == assertion.sha", terraform)
        self.assertNotIn("assertion.job_workflow_sha", terraform)
        self.assertIn('role     = "roles/run.jobsExecutorWithOverrides"', terraform)
        self.assertIn('image   = var.runtime_settings.maintenance_operator_image_digest', terraform)
        self.assertIn('contains(["maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"], var.stage)', terraform)
        self.assertIn('command = ["/app/mypage-maintenance-breakglass"]', terraform)
        self.assertIn('command = ["/app/mypage-maintenance-bootstrap-check"]', terraform)

    def test_repository_ci_discovers_and_lints_the_operator_workflow(self):
        repository_ci = (ROOT / ".github/workflows/ci-repository.yml").read_text()
        self.assertIn("-p 'test_ci_*.py'", repository_ci)
        self.assertIn(".github/workflows/mypage-maintenance.yml", repository_ci)


if __name__ == "__main__":
    unittest.main()
