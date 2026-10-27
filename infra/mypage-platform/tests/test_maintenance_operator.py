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
}, "maintenance_operator")
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
        self.assertTrue(any("artifact_registry_repository" in address for address in contract["addresses"]))

    def test_github_invoker_and_firestore_operator_are_distinct(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertRegex(source, r'account_id\s+= "mypage-maintenance-invoker"')
        self.assertRegex(source, r'account_id\s+= "mypage-maintenance-job"')
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
        self.assertEqual(source.count('google_iam_workload_identity_pool_provider.maintenance_github,'), 1)
        self.assertEqual(len(re.findall(r'service_account\s+= local\.maintenance_operator_email', source)), 2)
        self.assertRegex(source, r'service_account\s+= local\.maintenance_bootstrap_email')
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
        self.assertRegex(source, r'role_id\s+= "mypageMaintenanceBootstrapReader"')
        self.assertIn('command = ["/app/mypage-maintenance-bootstrap-check"]', source)
        self.assertIn('command = ["/app/mypage-maintenance-breakglass"]', source)
        self.assertIn('member   = "serviceAccount:${local.backend_deployer_email}"', source)
        self.assertEqual(source.count('role     = "roles/run.invoker"'), 4)  # public API and three bootstrap callers

    def test_bootstrap_stage_precedes_api_and_override_role_is_exact(self):
        source = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertIn('image   = var.runtime_settings.maintenance_operator_image_digest', source)
        self.assertIn('role     = "roles/run.jobsExecutorWithOverrides"', source)
        self.assertNotIn('role     = "roles/run.invoker"\n  member   = "serviceAccount:${local.maintenance_invoker_email}"', source)
        fixture = (ROOT / "infra/mypage-platform/environments/dev/tests/foundation-contract.tftest.hcl").read_text()
        self.assertIn('run "maintenance_operator_contract"', fixture)
        self.assertIn('!module.platform[0].runtime_enabled', fixture)
        image_check = (ROOT / "system/scripts/check-mypage-image.sh").read_text()
        for binary in (
            "mypage-maintenance-operator",
            "mypage-maintenance-breakglass",
            "mypage-maintenance-bootstrap-check",
        ):
            self.assertIn(binary, image_check)

    def test_authenticated_entrypoint_stages_operator_runtime_settings(self):
        source = (ROOT / "infra/mypage-platform/scripts/foundation_ops.py").read_text()
        self.assertIn(
            'if stage_name in set(runtime_contract.MODES.values()):',
            source,
        )
        self.assertNotIn('.startswith("runtime_")', source)

    def test_production_operator_bootstrap_needs_only_operator_image(self):
        program = r'''
import json
import foundation_contract as c
import foundation_ops as ops
import runtime_contract as r

sha = "c" * 40
raw = {
    "maintenance_operator_enabled": True,
    "maintenance_operator_image_digest": r.REPOSITORY + "@sha256:" + "b" * 64,
    "maintenance_operator_image_source_sha": sha,
}
env = {
    "GITHUB_SHA": sha,
    "MYPAGE_GCP_ACCESS_TOKEN": "private-token",
    "MYPAGE_GCP_WIF_PROVIDER": "projects/" + c.PROJECT_NUMBER + "/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions",
    "MYPAGE_RUNTIME_SETTINGS_JSON": json.dumps(raw),
}
value = ops.runtime_settings(env, "maintenance_operator")
prefix = "projects/" + c.PROJECT + "/locations/" + c.REGION + "/repositories/mypage/packages/mypage-api"
calls = []
ops.request = lambda url, token: calls.append(url) or {
    "version": prefix + "/versions/sha256:" + "b" * 64,
}
ops.verify_runtime_inputs(env, value, "maintenance_operator")
print(json.dumps({"value": value, "calls": calls}))
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
        actual = json.loads(result.stdout)
        self.assertEqual(actual["value"]["image_digest"], "")
        self.assertEqual(actual["value"]["oauth_client_id_version"], "")
        self.assertEqual(len(actual["calls"]), 1)
        self.assertIn("/tags/" + "c" * 40, actual["calls"][0])
        self.assertNotIn("secretmanager", actual["calls"][0])

    def test_production_registry_and_image_build_bootstrap_are_source_complete(self):
        program = r'''
import json
import foundation_contract as c
import image_ops

sha = "a" * 40
env = {
    "GITHUB_REPOSITORY": c.REPOSITORY,
    "GITHUB_REPOSITORY_ID": "340900071",
    "GITHUB_REPOSITORY_OWNER_ID": "54093651",
    "GITHUB_EVENT_NAME": "workflow_dispatch",
    "GITHUB_ACTOR": "kani3camp",
    "GITHUB_ACTOR_ID": "54093651",
    "GITHUB_TRIGGERING_ACTOR": "kani3camp",
    "GITHUB_RUN_ATTEMPT": "1",
    "GITHUB_REF": c.REF,
    "GITHUB_WORKFLOW_REF": c.CALLER,
    "TF_WORKSPACE": "default",
    "GITHUB_SHA": sha,
    "MYPAGE_EXPECTED_SHA": sha,
    "MYPAGE_MODE": "image-build",
    "MYPAGE_ROLE": "apply",
    "MYPAGE_GCP_SERVICE_ACCOUNT": "mypage-foundation-apply@" + c.PROJECT + ".iam.gserviceaccount.com",
    "MYPAGE_GCP_WIF_PROVIDER": "projects/" + c.PROJECT_NUMBER + "/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions",
}
image_ops.preflight(env)
registry = c.resource_contract(c.stage_for_mode("registry-plan"))
allowed = {mode: c.stage_for_mode(mode) for mode in (
    "registry-plan", "registry-apply", "maintenance-operator-plan",
    "maintenance-operator-apply", "maintenance-upgrade-plan", "maintenance-upgrade-apply",
)}
rejected = []
for mode in ("foundation-plan", "runtime-private-plan", "runtime-public-apply"):
    try:
        c.stage_for_mode(mode)
    except ValueError:
        rejected.append(mode)
print(json.dumps({"prefix": image_ops.PREFIX, "registry": registry, "allowed": allowed, "rejected": rejected}))
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
        actual = json.loads(result.stdout)
        self.assertEqual(
            actual["prefix"],
            "projects/youtube-study-space/locations/asia-northeast2/repositories/mypage",
        )
        repository = actual["registry"]["module.platform[0].google_artifact_registry_repository.mypage[0]"]
        self.assertEqual(repository["project"], "youtube-study-space")
        self.assertEqual(repository["location"], "asia-northeast2")
        self.assertTrue(repository["docker_config"][0]["immutable_tags"])
        self.assertEqual(set(actual["allowed"]), {
            "registry-plan", "registry-apply", "maintenance-operator-plan",
            "maintenance-operator-apply", "maintenance-upgrade-plan", "maintenance-upgrade-apply",
        })
        self.assertEqual(actual["rejected"], [
            "foundation-plan", "runtime-private-plan", "runtime-public-apply",
        ])

    def test_runtime_plan_and_apply_checkout_full_history_for_operator_ancestry(self):
        source = (ROOT / ".github/workflows/mypage-foundation-authenticated.yml").read_text()
        before_image_build = source.split("  image_build:", 1)[0]
        self.assertEqual(before_image_build.count("fetch-depth: 0"), 2)
        image_build = source.split("  image_build:", 1)[1]
        self.assertNotIn("configure-aws-credentials", image_build)
        self.assertIn("inputs.target == 'production' && 'mypage-prod-apply'", image_build)
        self.assertIn("inputs.target == 'production' && 'youtube-study-space'", image_build)
        target_contract = source.split("  foundation_plan:", 1)[0]
        self.assertIn("foundation_contract.context(os.environ, 'plan')", target_contract)

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
        self.assertLess(foundation.index("Plan bounded private foundation resources"), foundation.index("Require initialized maintenance control after planning"))
        self.assertLess(foundation.index("Independently reproduce saved Plan"), foundation.index("Require initialized maintenance control before applying"))
        self.assertLess(foundation.index("Require initialized maintenance control before applying"), foundation.index("Apply matching saved Plan"))
        self.assertIn("steps.prepared.outputs.commitment == needs.foundation_plan.outputs.commitment", foundation)
        self.assertIn("inputs.mode == 'runtime-private-plan' || inputs.mode == 'runtime-public-plan'", foundation)
        self.assertLess(backend.index("Require initialized maintenance control before mutating"), backend.index("Conditionally update only the source-pinned image"))


if __name__ == "__main__":
    unittest.main()
