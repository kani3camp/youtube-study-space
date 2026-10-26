"""Static least-privilege contract for the optional maintenance operator."""

from pathlib import Path
import json
import os
import re
import subprocess
import sys
import unittest


ROOT = Path(__file__).resolve().parents[3]


class MaintenanceOperatorInfrastructureTest(unittest.TestCase):
    def test_production_operator_contract_uses_main_and_separate_state_boundary(self):
        program = r'''
import json
import foundation_contract as c
import runtime_contract as r

value = r.settings({
    "image_digest": r.REPOSITORY + "@sha256:" + "a" * 64,
    "project_number": c.PROJECT_NUMBER,
    "web_app_id": "1:" + c.PROJECT_NUMBER + ":web:abcdef",
    "public_origin": c.ORIGIN,
    "privacy_version": "p1",
    "terms_version": "t1",
    "oauth_client_id_version": "1",
    "oauth_client_secret_version": "2",
    "youtube_api_key_version": "3",
    "maintenance_operator_enabled": True,
    "maintenance_operator_image_digest": r.REPOSITORY + "@sha256:" + "b" * 64,
    "maintenance_operator_image_source_sha": "c" * 40,
})
resources = r.resource_contract("maintenance_operator", value)
provider = resources[r.MAINTENANCE_PROVIDER]
print(json.dumps({
    "project": c.PROJECT,
    "region": c.REGION,
    "ref": c.REF,
    "state": c.STATE_KEY,
    "subject": c.subject("plan"),
    "condition": provider["attribute_condition"],
    "addresses": sorted(resources),
}))
'''
        env = os.environ | {
            "MYPAGE_TARGET": "production",
            "PYTHONPATH": str(ROOT / "infra/mypage-platform/scripts"),
        }
        result = subprocess.run(
            [sys.executable, "-c", program],
            cwd=ROOT,
            env=env,
            capture_output=True,
            text=True,
            check=True,
        )
        contract = json.loads(result.stdout)
        self.assertEqual(contract["project"], "youtube-study-space")
        self.assertEqual(contract["region"], "asia-northeast2")
        self.assertEqual(contract["ref"], "refs/heads/main")
        self.assertEqual(contract["state"], "youtube-study-space/mypage-platform/prod/terraform.tfstate")
        self.assertIn("environment:mypage-prod-plan", contract["subject"])
        self.assertIn("mypage-maintenance-production", contract["condition"])
        self.assertIn("@refs/heads/main", contract["condition"])
        self.assertFalse(any("artifact_registry_repository" in address for address in contract["addresses"]))

    def test_github_invoker_and_firestore_operator_are_distinct(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertIn('account_id   = "mypage-maintenance-invoker"', source)
        self.assertIn('account_id   = "mypage-maintenance-job"', source)
        self.assertIn('member  = "serviceAccount:${local.maintenance_operator_email}"', source)
        self.assertIn('member   = "serviceAccount:${local.maintenance_invoker_email}"', source)
        self.assertNotIn('roles/datastore.user"\n  member  = "serviceAccount:${local.maintenance_invoker_email}', source)
        self.assertIn('command = ["/app/mypage-maintenance-operator"]', source)

    def test_operator_graph_directly_orders_identity_consumers(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertIn('google_service_account.maintenance_invoker,', source)
        self.assertIn('google_service_account.maintenance_bootstrap,', source)
        self.assertIn('google_service_account.maintenance_operator,', source)
        self.assertIn('google_project_iam_custom_role.maintenance_firestore,', source)
        self.assertIn('google_project_iam_custom_role.maintenance_bootstrap_reader,', source)
        self.assertIn('google_cloud_run_v2_job.maintenance_operator,', source)
        self.assertEqual(source.count('google_iam_workload_identity_pool_provider.maintenance_github,'), 2)
        self.assertEqual(source.count('service_account = local.maintenance_operator_email'), 2)
        self.assertIn('service_account = local.maintenance_bootstrap_email', source)
        self.assertIn('depends_on = [google_project_iam_member.maintenance_firestore]', source)
        self.assertIn('depends_on = [google_project_iam_member.maintenance_bootstrap_reader]', source)
        self.assertIn('member  = "serviceAccount:${local.maintenance_bootstrap_email}"', source)

    def test_wif_is_exact_environment_scoped_and_manual_only(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        for claim in ("repository_id", "repository_owner_id", "actor_id", "actor", "run_attempt", "event_name", "ref", "workflow_ref", "environment", "workflow_sha", "sha"):
            self.assertIn(f"assertion.{claim}", source)
            self.assertIn(f'"attribute.{claim}"', source)
        self.assertIn("assertion.event_name == 'workflow_dispatch'", source)
        self.assertIn("assertion.run_attempt == '1'", source)
        self.assertIn("assertion.workflow_sha == assertion.sha", source)
        self.assertNotIn("assertion.job_workflow_sha", source)
        self.assertIn("local.maintenance_github_environment", source)
        self.assertIn("local.maintenance_inspect_github_environment", source)
        self.assertIn("mypage-maintenance-inspect.yml", source)
        self.assertNotIn("pull_request", source)
        self.assertIn('var.environment == "production" ? "906336399194" : "48101442817"', source)
        self.assertIn("projects/${local.maintenance_project_number}/locations/global/workloadIdentityPools/mypage-maintenance/attribute.workflow_ref", source)
        self.assertNotIn("projects/${var.runtime_settings.project_number}/locations/global/workloadIdentityPools/mypage-maintenance", source)

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
