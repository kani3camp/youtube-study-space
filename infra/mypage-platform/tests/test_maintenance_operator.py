"""Static least-privilege contract for the optional maintenance operator."""

from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[3]


class MaintenanceOperatorInfrastructureTest(unittest.TestCase):
    def test_github_invoker_and_firestore_operator_are_distinct(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertIn('account_id   = "mypage-maintenance-invoker"', source)
        self.assertIn('account_id   = "mypage-maintenance-job"', source)
        self.assertIn('member  = "serviceAccount:${local.maintenance_operator_email}"', source)
        self.assertIn('member   = "serviceAccount:${local.maintenance_invoker_email}"', source)
        self.assertNotIn('roles/datastore.user"\n  member  = "serviceAccount:${local.maintenance_invoker_email}', source)
        self.assertIn('command = ["/app/mypage-maintenance-operator"]', source)

    def test_wif_is_exact_environment_scoped_and_manual_only(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        for claim in ("repository_id", "repository_owner_id", "actor_id", "actor", "run_attempt", "event_name", "ref", "workflow_ref", "environment", "workflow_sha", "sha"):
            self.assertIn(f"assertion.{claim}", source)
        self.assertIn("assertion.event_name == 'workflow_dispatch'", source)
        self.assertIn("assertion.run_attempt == '1'", source)
        self.assertIn("assertion.workflow_sha == assertion.sha", source)
        self.assertNotIn("assertion.job_workflow_sha", source)
        self.assertIn("local.maintenance_github_environment", source)
        self.assertNotIn("pull_request", source)

    def test_operator_is_opt_in_and_has_no_key_resource(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertIn('contains(["maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"], var.stage)', source)
        self.assertNotIn("google_service_account_key", source)
        self.assertIn('"datastore.entities.get"', source)
        self.assertIn('"datastore.entities.create"', source)
        self.assertIn('"datastore.entities.update"', source)
        self.assertIn('role_id     = "mypageMaintenanceBootstrapReader"', source)
        self.assertIn('command = ["/app/mypage-maintenance-bootstrap-check"]', source)
        self.assertIn('command = ["/app/mypage-maintenance-breakglass"]', source)
        self.assertIn('member   = "serviceAccount:${local.backend_deployer_email}"', source)
        self.assertEqual(source.count('role     = "roles/run.invoker"'), 4)  # public API plus three fixed bootstrap callers

    def test_bootstrap_stage_precedes_api_and_override_role_is_exact(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertIn('image   = var.runtime_settings.maintenance_operator_image_digest', source)
        self.assertIn('role     = "roles/run.jobsExecutorWithOverrides"', source)
        self.assertNotIn('role     = "roles/run.invoker"\n  member   = "serviceAccount:${local.maintenance_invoker_email}"', source)
        fixture = (ROOT / "infra/mypage-platform/environments/dev/tests/foundation-contract.tftest.hcl").read_text()
        self.assertIn('run "maintenance_operator_contract"', fixture)
        self.assertIn('!module.platform[0].runtime_enabled', fixture)

    def test_authenticated_entrypoint_stages_operator_runtime_settings(self):
        source = (ROOT / "infra/mypage-platform/scripts/foundation_ops.py").read_text()
        self.assertIn(
            'contract.stage_for_mode(env["MYPAGE_MODE"]) in set(runtime_contract.MODES.values())',
            source,
        )
        self.assertNotIn('.startswith("runtime_")', source)

    def test_runtime_plan_and_apply_checkout_full_history_for_operator_ancestry(self):
        source = (ROOT / ".github/workflows/mypage-foundation-authenticated.yml").read_text()
        before_image_build = source.split("  image_build:", 1)[0]
        self.assertEqual(before_image_build.count("fetch-depth: 0"), 2)

    def test_runtime_stages_require_completed_bootstrap_before_any_mutation(self):
        contract = (ROOT / "infra/mypage-platform/scripts/runtime_contract.py").read_text()
        self.assertIn('"runtime-maintenance-bootstrap-complete"', contract)
        foundation = (ROOT / ".github/workflows/mypage-foundation-authenticated.yml").read_text()
        backend = (ROOT / ".github/workflows/mypage-backend-authenticated.yml").read_text()
        for source in (foundation, backend):
            self.assertIn("gcloud run jobs execute mypage-maintenance-bootstrap-check", source)
            bootstrap = source.index("gcloud run jobs execute mypage-maintenance-bootstrap-check")
            next_step = source.index("\n      - name:", bootstrap)
            self.assertNotIn("--args", source[bootstrap:next_step])
            self.assertNotIn("--update-env-vars", source[bootstrap:next_step])
        self.assertLess(foundation.index("Require initialized maintenance control before planning"), foundation.index("Plan bounded private foundation resources"))
        self.assertLess(backend.index("Require initialized maintenance control before mutating"), backend.index("Conditionally update only the source-pinned image"))


if __name__ == "__main__":
    unittest.main()
