#!/usr/bin/env python3

from __future__ import annotations

import re
import os
import subprocess
import tempfile
import textwrap
import unittest
from pathlib import Path

GITHUB_DIR = Path(__file__).parents[1]
WORKFLOW = GITHUB_DIR / "workflows" / "gcp-terraform-authenticated.yml"
CALLER = GITHUB_DIR / "workflows" / "ci.yml"


class TerraformAuthenticatedWorkflowTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.text = WORKFLOW.read_text(encoding="utf-8")
        cls.caller = CALLER.read_text(encoding="utf-8")

    def test_private_notification_inputs_are_masked_and_do_not_enable_alerts(self) -> None:
        for job in (self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)[0], self.text.split("  apply:\n", 1)[1]):
            self.assertIn("TF_VAR_primary_email_address", job)
            self.assertIn("TF_VAR_primary_email_channel_name", job)
            self.assertIn('"${PRIVATE_EMAIL%%@*}"', job)
            self.assertIn('"${PRIVATE_CHANNEL##*/}"', job)
            self.assertLess(job.index("PRIVATE_EMAIL"), job.index("Configure GCP provider credential"))
            self.assertIn("TF_VAR_manage_youtube_quota_alerts: ${{ needs.preflight.outputs.manage_quota }}", job)
        for env in ("dev", "prod"):
            root = GITHUB_DIR.parent / "infra/gcp/environments" / env
            self.assertIn("sensitive = true", (root / "notification-channels.tf").read_text())

    def test_job_level_env_does_not_use_unavailable_env_context(self) -> None:
        for job in (self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)[0], self.text.split("  apply:\n", 1)[1]):
            env_block = job.split("    env:\n", 1)[1].split("    steps:\n", 1)[0]
            self.assertNotRegex(env_block, r"\$\{\{[^}]*\benv\.")
        self.assertIn('DEV_PRIMARY_EMAIL_IMPORT_ENABLED: "true"', self.text)

    def test_reusable_workflow_is_not_directly_triggerable(self) -> None:
        self.assertIn("workflow_call:", self.text)
        self.assertNotIn("workflow_dispatch:", self.text)
        self.assertNotIn("pull_request_target", self.text)
        self.assertNotRegex(self.text, r"(?m)^\s*pull_request\s*:")

    def test_default_branch_ci_is_the_manual_dispatch_entrypoint(self) -> None:
        self.assertIn("workflow_dispatch:", self.caller)
        self.assertIn("terraform_authenticated:", self.caller)
        self.assertIn("terraform_target:", self.caller)
        self.assertIn("terraform_mode:", self.caller)
        self.assertIn("uses: ./.github/workflows/gcp-terraform-authenticated.yml", self.caller)
        self.assertIn("github.event_name == 'workflow_dispatch'", self.caller)
        self.assertIn("inputs.terraform_authenticated == true", self.caller)
        self.assertNotIn("secrets: inherit", self.caller)

    def test_production_remains_fail_closed_and_trusted_surface_is_fixed(self) -> None:
        self.assertIn('PROD_AUTHENTICATED_TERRAFORM_ENABLED: "false"', self.text)
        self.assertIn("refs/heads/feature/gcp-terraform-iac", self.text)
        self.assertIn('GITHUB_REPOSITORY_ID}" == "340900071"', self.text)
        self.assertIn('GITHUB_REPOSITORY_OWNER_ID}" == "54093651"', self.text)
        self.assertIn(
            'GITHUB_WORKFLOW_REF}" == "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac"',
            self.text,
        )

    def test_environment_secret_interface_is_explicit_without_inheritance(self) -> None:
        interface = self.text.split("    secrets:\n", 1)[1].split("\npermissions:", 1)[0]
        declared = set(re.findall(r"(?m)^      ([A-Z_]+):$", interface))
        used = set(re.findall(r"secrets\.([A-Z_]+)", self.text))
        self.assertEqual(declared, used)
        self.assertEqual(len(declared), 8)
        self.assertEqual(interface.count("required: false"), 8)
        call = self.caller.split("  gcp-terraform-authenticated:\n", 1)[1].split("\n  ci-gate:\n", 1)[0]
        forwarded = re.findall(r"(?m)^      ([A-Z_]+): \$\{\{ secrets\.([A-Z_]+) \}\}$", call)
        self.assertEqual({name for name, value in forwarded}, declared)
        self.assertTrue(all(name == value for name, value in forwarded))
        self.assertNotIn("secrets: inherit", self.caller)

    def run_preflight(self, **overrides: str) -> subprocess.CompletedProcess[str]:
        script = self.text.split("      - name: Enforce trusted execution surface", 1)[1].split("        run: |\n", 1)[1].split("\n  plan:\n", 1)[0]
        env = {
            "PATH": os.environ["PATH"], "GITHUB_EVENT_NAME": "workflow_dispatch",
            "GITHUB_REPOSITORY": "kani3camp/youtube-study-space", "GITHUB_REPOSITORY_ID": "340900071",
            "GITHUB_REPOSITORY_OWNER_ID": "54093651", "GITHUB_REF": "refs/heads/feature/gcp-terraform-iac",
            "GITHUB_WORKFLOW_REF": "kani3camp/youtube-study-space/.github/workflows/ci.yml@refs/heads/feature/gcp-terraform-iac",
            "GITHUB_SHA": "a" * 40, "TARGET": "dev", "MODE": "plan",
            "DEV_AUTHENTICATED_TERRAFORM_ENABLED": "true", "DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED": "false",
            "PROD_AUTHENTICATED_TERRAFORM_ENABLED": "false", "DEV_PRIMARY_EMAIL_IMPORT_ENABLED": "true", "DEV_QUOTA_CREATE_ENABLED": "false", "DEV_QUOTA_MANAGED_ENABLED": "false", "DEV_QUOTA_STATE_REFRESH_ENABLED": "false", "DEV_EXPORT_TOPIC_MANAGED_ENABLED": "true", "DEV_EXPORT_SCHEDULER_MANAGED_ENABLED": "true",
        }
        env.update(overrides)
        with tempfile.TemporaryDirectory() as directory:
            env["GITHUB_OUTPUT"] = str(Path(directory) / "outputs")
            result = subprocess.run(["bash", "-c", textwrap.dedent(script)], env=env, capture_output=True, text=True)
            result.outputs = Path(env["GITHUB_OUTPUT"]).read_text() if Path(env["GITHUB_OUTPUT"]).exists() else ""
            return result

    def test_topic_ownership_is_dev_only_and_same_between_jobs(self):
        self.assertIn('DEV_EXPORT_TOPIC_MANAGED_ENABLED: "true"', self.text)
        result = self.run_preflight()
        self.assertEqual(result.returncode, 0)
        self.assertIn("manage_export_topic=true", result.outputs)
        self.assertEqual(self.text.count("TF_VAR_manage_export_topic: ${{ needs.preflight.outputs.manage_export_topic }}"), 2)
        self.assertIn('echo "manage_export_topic=false"', self.text)

    def test_scheduler_ownership_requires_dev_topic_and_is_same_between_jobs(self):
        self.assertIn('DEV_EXPORT_SCHEDULER_MANAGED_ENABLED: "true"', self.text)
        result = self.run_preflight()
        self.assertEqual(result.returncode, 0)
        self.assertIn("manage_export_scheduler=true", result.outputs)
        self.assertEqual(self.text.count("TF_VAR_manage_export_scheduler: ${{ needs.preflight.outputs.manage_export_scheduler }}"), 2)
        self.assertIn('echo "manage_export_scheduler=false"', self.text)
        self.assertNotEqual(self.run_preflight(DEV_EXPORT_TOPIC_MANAGED_ENABLED="false").returncode, 0)
        self.assertIn("EXPORT_SCHEDULER_IDENTITY_REQUIRED", self.text)
        self.assertIn("terraform_identity_smoke.py export-scheduler", self.text)

    def test_enabled_development_plan_does_not_enable_apply(self) -> None:
        self.assertEqual(self.run_preflight().returncode, 0)
        rejected = self.run_preflight(MODE="apply")
        self.assertNotEqual(rejected.returncode, 0)
        self.assertIn("Development apply is disabled", rejected.stdout)

    def test_email_adoption_uses_same_trust_and_independent_apply_gate(self) -> None:
        self.assertNotEqual(self.run_preflight(MODE="email-adoption").returncode, 0)
        self.assertEqual(self.run_preflight(MODE="email-adoption", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="email-adoption", TARGET="prod").returncode, 0)
        self.assertIn('--phase post', self.text)
        self.assertIn('inputs.mode == \'email-adoption\'', self.text)

    def test_quota_create_requires_all_independent_gates_and_quota_plan_never_applies(self) -> None:
        self.assertEqual(self.run_preflight(MODE="quota-plan").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="quota-create", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="quota-plan", DEV_PRIMARY_EMAIL_IMPORT_ENABLED="false").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="quota-plan", TARGET="prod").returncode, 0)
        self.assertIn('DEV_QUOTA_CREATE_ENABLED: "false"', self.text)
        self.assertEqual(self.run_preflight(MODE="quota-create", DEV_QUOTA_CREATE_ENABLED="true", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="quota-create", DEV_QUOTA_CREATE_ENABLED="true").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="quota-create", DEV_QUOTA_CREATE_ENABLED="true", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true", TARGET="prod").returncode, 0)
        apply_job = self.text.split("  apply:\n", 1)[1]
        self.assertLess(apply_job.index("Verify quota apply identity least privilege"), apply_job.index("Re-plan at the exact approved commit"))
        self.assertIn("terraform_identity_smoke.py quota-apply", apply_job)
        self.assertIn("QUOTA_IDENTITY_REQUIRED", self.text)
        apply_condition = self.text.split("  apply:\n", 1)[1].split("    needs:", 1)[0]
        self.assertNotIn("quota-plan", apply_condition)

    def test_completed_state_refresh_is_closed_and_ownership_is_kept(self) -> None:
        self.assertIn('DEV_QUOTA_MANAGED_ENABLED: "true"', self.text)
        self.assertIn('DEV_QUOTA_STATE_REFRESH_ENABLED: "false"', self.text)
        self.assertNotEqual(self.run_preflight(MODE="quota-refresh", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true").returncode, 0)
        self.assertEqual(self.run_preflight(MODE="quota-refresh", DEV_QUOTA_STATE_REFRESH_ENABLED="true", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="quota-refresh", DEV_QUOTA_STATE_REFRESH_ENABLED="true").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="quota-refresh", DEV_QUOTA_STATE_REFRESH_ENABLED="true", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true", TARGET="prod").returncode, 0)
        owned = self.run_preflight(DEV_QUOTA_MANAGED_ENABLED="true")
        self.assertEqual(owned.returncode, 0)
        self.assertIn('manage_quota=true', owned.outputs)
        self.assertEqual(self.text.count('plan_args=(-refresh-only)'), 2)
        post = self.text.split('      - name: Require post-apply no-op', 1)[1]
        self.assertNotIn('-refresh-only', post)
        self.assertIn("inputs.mode == 'quota-refresh'", self.text.split('  apply:', 1)[1])

    def test_enabled_development_apply_still_requires_trusted_surface(self) -> None:
        enabled = re.search(r'DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED: "(true|false)"', self.text).group(1)
        self.assertEqual(self.run_preflight(MODE="apply", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED=enabled).returncode, 0)
        for key, value in {
            "GITHUB_EVENT_NAME": "pull_request", "GITHUB_REPOSITORY_ID": "0",
            "GITHUB_REF": "refs/heads/dev", "GITHUB_WORKFLOW_REF": "wrong/workflow",
            "TARGET": "prod", "DEV_AUTHENTICATED_TERRAFORM_ENABLED": "false",
        }.items():
            with self.subTest(key=key):
                self.assertNotEqual(self.run_preflight(MODE="apply", DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED=enabled, **{key: value}).returncode, 0)

    def test_untrusted_execution_is_rejected_before_any_authentication(self) -> None:
        for key, value in {
            "GITHUB_EVENT_NAME": "pull_request", "GITHUB_REPOSITORY": "attacker/repo",
            "GITHUB_REPOSITORY_ID": "0", "GITHUB_REPOSITORY_OWNER_ID": "0",
            "GITHUB_REF": "refs/heads/dev", "GITHUB_WORKFLOW_REF": "wrong/workflow",
            "GITHUB_SHA": "not-a-sha", "TARGET": "prod", "MODE": "destroy",
            "DEV_AUTHENTICATED_TERRAFORM_ENABLED": "false",
        }.items():
            with self.subTest(key=key):
                self.assertNotEqual(self.run_preflight(**{key: value}).returncode, 0)

    def test_empty_graph_still_forces_wif_exchange_and_sa_smoke(self) -> None:
        self.assertIn("token_format: access_token", self.text)
        self.assertIn("steps.plan_gcp_auth.outputs.access_token", self.text)
        self.assertIn("python3 .github/scripts/terraform_identity_smoke.py", self.text)

    def test_workspace_discovery_stays_within_target_prefix(self) -> None:
        self.assertEqual(self.text.count('workspace_key_prefix = "${STATE_KEY%/terraform.tfstate}/workspaces"'), 2)
        self.assertEqual(self.text.count("TF_WORKSPACE: default"), 2)

    def test_private_identity_fragments_are_masked_before_authentication(self) -> None:
        for job in (self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)[0], self.text.split("  apply:\n", 1)[1]):
            self.assertLess(job.index("Mask private identity fragments"), job.index("Configure AWS backend credential"))
            self.assertIn("secrets.AWS_TERRAFORM_BACKEND_ROLE_ID", job)

    def test_deployed_identity_identifiers_are_not_literal_source_values(self) -> None:
        sources = [WORKFLOW, Path(__file__), Path(__file__).with_name("terraform_identity_smoke.py"), Path(__file__).with_name("test_terraform_identity_smoke.py")]
        forbidden = (
            r"\b[a-z0-9-]+@(?:test-)?youtube-study-space\.iam\.gserviceaccount\.com\b",
            r"arn:aws:iam::[0-9]{12}:role/",
            r"projects/[0-9]{10,}/locations/global/workloadIdentityPools/",
        )
        for source in sources:
            for pattern in forbidden:
                with self.subTest(source=source.name, pattern=pattern):
                    self.assertNotRegex(source.read_text(), pattern)

    def test_oidc_permission_is_limited_to_authenticated_call_and_jobs(self) -> None:
        self.assertEqual(self.text.count("id-token: write"), 2)
        self.assertEqual(self.caller.count("id-token: write"), 1)
        preflight = self.text.split("  plan:\n", 1)[0]
        self.assertNotIn("id-token: write", preflight)

    def test_authenticated_jobs_use_environment_boundaries(self) -> None:
        self.assertIn("name: terraform-${{ inputs.target }}-plan", self.text)
        self.assertIn("name: terraform-${{ inputs.target }}-apply", self.text)

    def test_no_saved_plan_artifact_or_cache(self) -> None:
        for value in ("actions/upload-artifact", "actions/download-artifact", "actions/cache", "-auto-approve"):
            self.assertNotIn(value, self.text)
        self.assertNotRegex(self.text, r"(?m)^\s*uses:\s*.*upload-artifact")

    def test_raw_terraform_output_is_redirected(self) -> None:
        self.assertIn('>"${RUNNER_TEMP}/terraform-init.log" 2>&1', self.text)
        self.assertIn('>"${RUNNER_TEMP}/terraform-plan.log" 2>&1', self.text)
        self.assertIn('>"${RUNNER_TEMP}/terraform-apply.log" 2>&1', self.text)
        self.assertNotIn('cat "${RUNNER_TEMP}/terraform-plan.log"', self.text)
        self.assertNotIn('cat "${RUNNER_TEMP}/terraform-apply.log"', self.text)
        self.assertIn("terraform_protected_plan.py", self.text)

    def test_external_actions_are_full_sha_pinned(self) -> None:
        action_lines = re.findall(r"(?m)^\s*uses:\s*([^\s#]+)", self.text)
        self.assertGreaterEqual(len(action_lines), 8)
        for action in action_lines:
            if action.startswith("./"):
                continue
            self.assertRegex(action, r"^[^@]+@[0-9a-f]{40}$", action)

    def test_checkout_does_not_persist_github_token(self) -> None:
        self.assertEqual(self.text.count("persist-credentials: false"), 2)

    def test_import_only_policy_is_enforced_before_apply_and_post_apply(self) -> None:
        self.assertGreaterEqual(self.text.count("--policy import-only"), 3)
        self.assertIn("Re-plan does not match the approved sanitized projection", self.text)
        self.assertIn("Post-apply Terraform plan is not no-op", self.text)

    def test_private_identifiers_are_environment_secret_inputs_not_literals(self) -> None:
        required_secret_names = (
            "AWS_TERRAFORM_BACKEND_ROLE_ARN",
            "AWS_TERRAFORM_STATE_BUCKET",
            "AWS_TERRAFORM_STATE_ACCOUNT_ID",
            "GCP_TERRAFORM_WIF_PROVIDER",
            "GCP_TERRAFORM_SERVICE_ACCOUNT",
        )
        for name in required_secret_names:
            self.assertIn(f"secrets.{name}", self.text)


if __name__ == "__main__":
    unittest.main()
