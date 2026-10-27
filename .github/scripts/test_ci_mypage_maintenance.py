from copy import deepcopy
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
import json
import threading
import unittest
import urllib.error

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

    def test_canonicalizes_rfc3339_estimate_before_authentication(self):
        env = values()
        env["INPUT_ESTIMATED_END_AT"] = "2026-10-27T12:00:00.123400000+09:00"
        result = subject.context(env.get, lambda: datetime(2026, 10, 27, 2, tzinfo=timezone.utc))
        self.assertEqual(result["estimated_end_at"], "2026-10-27T03:00:00.1234Z")

    def test_truncates_sub_microsecond_input_to_firestore_precision(self):
        env = values()
        env["INPUT_ESTIMATED_END_AT"] = "2026-10-27T03:00:00.123456789Z"
        result = subject.context(env.get, lambda: datetime(2026, 10, 27, 2, tzinfo=timezone.utc))
        self.assertEqual(result["estimated_end_at"], "2026-10-27T03:00:00.123456Z")

    def test_accepts_explicit_initial_normal(self):
        env = values()
        env.update(INPUT_STATE="NORMAL", INPUT_EXPECTED_REVISION="0", INPUT_ESTIMATED_END_AT="", INPUT_REASON_CATEGORY="INITIALIZATION")
        self.assertEqual(subject.context(env.get)["state"], "NORMAL")

    def test_accepts_only_recovery_for_normal_and_active_reasons_for_active(self):
        normal = values()
        normal.update(INPUT_STATE="NORMAL", INPUT_ESTIMATED_END_AT="", INPUT_REASON_CATEGORY="RECOVERY")
        self.assertEqual(subject.context(normal.get)["reason"], "RECOVERY")
        for reason in ("INITIALIZATION", "PLANNED_MAINTENANCE", "DATA_MIGRATION", "SECURITY_MAINTENANCE"):
            candidate = deepcopy(normal)
            candidate["INPUT_REASON_CATEGORY"] = reason
            with self.subTest(state="NORMAL", reason=reason), self.assertRaises(ValueError):
                subject.context(candidate.get)
        active = values()
        active["INPUT_REASON_CATEGORY"] = "RECOVERY"
        with self.assertRaises(ValueError):
            subject.context(active.get, lambda: datetime(2026, 10, 27, 2, tzinfo=timezone.utc))

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
        self.assertTrue(subject.valid_public_status(active, "ACTIVE", "2026-10-27T03:00:00+09:00"))
        normal = {key: value for key, value in active.items() if key != "estimatedEndAt"} | {"state": "NORMAL"}
        self.assertTrue(subject.valid_public_status(normal, "NORMAL", ""))
        active_without_estimate = {key: value for key, value in active.items() if key != "estimatedEndAt"}
        self.assertTrue(subject.valid_public_status(active_without_estimate, "ACTIVE", ""))
        for candidate, expected, estimate in (
            ({key: value for key, value in active.items() if key != "updatedAt"}, "ACTIVE", active["estimatedEndAt"]),
            (active | {"privateNote": "not public"}, "ACTIVE", active["estimatedEndAt"]),
            (active | {"updatedAt": "2026-99-27T02:00:00Z"}, "ACTIVE", active["estimatedEndAt"]),
            (active | {"estimatedEndAt": None}, "ACTIVE", active["estimatedEndAt"]),
            (active | {"estimatedEndAt": "2026-10-27T03:01:00+09:00"}, "ACTIVE", active["estimatedEndAt"]),
            (active_without_estimate, "ACTIVE", active["estimatedEndAt"]),
            (normal | {"estimatedEndAt": "2026-10-27T03:00:00Z"}, "NORMAL", ""),
            (active | {"state": "NOTICE"}, "ACTIVE", active["estimatedEndAt"]),
            (active | {"messageTemplate": "free text"}, "ACTIVE", active["estimatedEndAt"]),
        ):
            with self.subTest(candidate=candidate, expected=expected, estimate=estimate):
                self.assertFalse(subject.valid_public_status(candidate, expected, estimate))

    def test_deployed_capability_requires_strict_status_and_exact_frontend_marker(self):
        status = {
            "state": "ACTIVE",
            "updatedAt": "2026-10-27T02:00:00Z",
            "estimatedEndAt": "2026-10-27T03:00:00.123456789Z",
            "messageTemplate": "MYPAGE_MAINTENANCE_V1",
        }
        self.assertEqual(subject.public_control(status), {"state": "ACTIVE", "estimated_end_at": "2026-10-27T03:00:00.123456Z"})
        marker = subject.FRONTEND_CAPABILITY
        self.assertTrue(subject.valid_frontend_capability("<head>" + marker + "</head>"))
        self.assertFalse(subject.valid_frontend_capability("<head></head>"))
        self.assertFalse(subject.valid_frontend_capability(marker + marker))

    def test_public_postconditions_reject_same_and_cross_origin_redirects(self):
        status = {
            "state": "ACTIVE",
            "updatedAt": "2026-10-27T02:00:00Z",
            "messageTemplate": "MYPAGE_MAINTENANCE_V1",
        }

        class Handler(BaseHTTPRequestHandler):
            redirect_path = ""
            redirect_target = ""

            def do_GET(self):
                if self.path == self.redirect_path:
                    self.send_response(302)
                    self.send_header("Location", self.redirect_target)
                    self.end_headers()
                    return
                body = json.dumps(status).encode() if self.path == "/api/public/mypage-status" else b"ok"
                self.send_response(200)
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *_):
                pass

        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        origin = f"http://127.0.0.1:{server.server_port}"

        self.assertEqual(subject.verify_public_postconditions(origin, "ACTIVE", ""), status)
        for path, target in (
            ("/api/public/mypage-status", "/direct-status"),
            ("/mypage", "https://example.invalid/foreign"),
        ):
            Handler.redirect_path = path
            Handler.redirect_target = target
            with self.subTest(path=path, target=target), self.assertRaises(urllib.error.HTTPError):
                subject.verify_public_postconditions(origin, "ACTIVE", "")
        Handler.redirect_path = ""

    def test_public_status_body_is_bounded_before_json_decode(self):
        class OversizeResponse:
            status = 200

            def __enter__(self):
                return self

            def __exit__(self, *_):
                return False

            def geturl(self):
                return "https://example.invalid/status"

            def read(self, size):
                return b"x" * size

        class Opener:
            def open(self, *_args, **_kwargs):
                return OversizeResponse()

        with self.assertRaises(ValueError):
            subject.direct_get(Opener(), "https://example.invalid/status", 1, 16)


class MaintenanceWorkflowSourceContractTest(unittest.TestCase):
    def test_manual_least_privilege_operator_sequence(self):
        workflow = (ROOT / ".github/workflows/mypage-maintenance.yml").read_text()
        self.assertIn("on:\n  workflow_dispatch:", workflow)
        for trigger in ("pull_request:", "push:", "schedule:", "workflow_call:"):
            self.assertNotIn(trigger, workflow)
        self.assertLess(workflow.index("Reject untrusted context and invalid transition before authentication"), workflow.index("google-github-actions/auth@"))
        self.assertLess(workflow.index("Verify deployed capability and bind the exact previous state"), workflow.index("google-github-actions/auth@"))
        self.assertIn("permissions: {}", workflow)
        self.assertIn("id-token: write", workflow)
        self.assertIn("secrets.MYPAGE_MAINTENANCE_WIF_PROVIDER", workflow)
        self.assertIn("secrets.MYPAGE_MAINTENANCE_SERVICE_ACCOUNT", workflow)
        self.assertIn("gcloud run jobs execute mypage-maintenance-operator", workflow)
        self.assertIn("run_operator preview", workflow)
        self.assertIn("run_operator apply", workflow)
        self.assertIn("run_operator verify", workflow)
        self.assertLess(workflow.index("run_operator preview"), workflow.index("run_operator apply"))
        self.assertLess(workflow.index("run_operator apply"), workflow.index("run_operator verify"))


if __name__ == "__main__":
    unittest.main()
