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

    def test_single_history_plan_cost_and_baseline_precede_all_native_lock_operations(self):
        plan = self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)[0]
        ordered = ["Bound the single history plan cost", "Configure AWS backend credential", "Configure GCP provider credential",
                   "Verify development identity", "Read canonical history metadata privately", "Bind the private history plan baseline",
                   "Initialize remote backend", "Create saved plan", "Sanitize and enforce", "Verify private history plan invariants", "Cleanup sensitive"]
        positions = [plan.index(name) for name in ordered]
        self.assertEqual(positions, sorted(positions))
        self.assertIn('AWS_MAX_ATTEMPTS: "1"', plan)
        self.assertIn('AWS_RETRY_MODE: standard', plan)
        self.assertIn('retry-max-attempts: 1', plan)
        self.assertIn('max_retries         = 1', plan)
        self.assertIn('-lock-timeout=0s', plan)
        self.assertIn("PLAN_COST_EVIDENCE: ${{ inputs.plan_cost_evidence }}", plan)
        self.assertIn("plan_cost_evidence: ${{ inputs.terraform_plan_cost_evidence }}", self.caller)
        self.assertIn("!(github.event_name == 'workflow_dispatch' && inputs.terraform_authenticated == true)", self.caller)
        self.assertNotIn("continue-on-error", plan)
        self.assertNotIn("force-unlock", plan)

    def test_history_post_receipt_runs_on_failed_init_plan_or_validation_before_private_cleanup(self):
        plan = self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)[0]
        post = plan.split("      - name: Verify private history plan invariants", 1)[1].split("      - name: Cleanup sensitive", 1)[0]
        self.assertIn("if: ${{ always() && steps.history_plan_before.outcome == 'success' }}", post)
        self.assertIn("steps.plan_gcp_auth.outputs.access_token", post)
        self.assertIn("--phase after", post)
        for phase in ("plan", "sanitize", "backend_init"):
            self.assertNotIn(f"steps.{phase}.outcome == 'success'", post)
        cleanup = plan.split("      - name: Cleanup sensitive temporary files", 1)[1]
        self.assertIn("if: always()", cleanup)
        for name in ("cost", "state-before", "state-after", "state-before-receipt", "before", "table-after", "after"):
            self.assertIn(f'"${{RUNNER_TEMP}}/history-plan-{name}.json"', cleanup)
        self.assertIn("test_terraform_history_plan_receipt.py", self.caller)

    def test_one_shot_history_import_has_separate_approval_and_state_receipts(self):
        plan = self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)[0]
        apply = self.text.split("  apply:\n", 1)[1]
        self.assertIn('DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED: "false"', self.text)
        self.assertIn("history_import_approval: ${{ inputs.terraform_history_import_approval }}", self.caller)
        self.assertLess(plan.index("Check one-shot history import authorization"), plan.index("Configure AWS backend credential"))
        ordered = ["Re-read canonical history metadata", "Capture exact pre-import state",
                   "Re-plan at the exact", "Check one-shot re-plan size", "Verify re-plan matches", "Seal the one-shot history saved plan",
                   "Apply the locally", "Verify post-import canonical metadata", "Require post-apply no-op",
                   "Verify one-shot history state and table receipt", "Cleanup sensitive temporary files"]
        self.assertEqual([apply.index(step) for step in ordered], sorted(apply.index(step) for step in ordered))
        self.assertIn("history_plan_sha256", apply)
        self.assertIn("steps.history_import_before.outcome == 'success'", apply)
        self.assertIn('"${RUNNER_TEMP}/history-import-table-after.json"', apply)
        self.assertNotIn("force-unlock", apply)
        self.assertIn("test_terraform_history_import_verifier.py", self.caller)
        self.assertLess(plan.index("Create saved plan without public output"), plan.index("Check one-shot history saved plan size"))
        self.assertLess(plan.index("Check one-shot history saved plan size"), plan.index("Sanitize and enforce"))
        self.assertEqual(self.text.count("terraform_history_import_verifier.py --phase size"), 2)

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
        self.assertEqual(len(declared), 9)
        self.assertEqual(interface.count("required: false"), 9)
        call = re.split(r"\n  [a-zA-Z0-9_-]+:\n",
                        self.caller.split("  gcp-terraform-authenticated:\n", 1)[1], maxsplit=1)[0]
        forwarded = re.findall(r"(?m)^      ([A-Z_]+): \$\{\{ secrets\.([A-Z_]+) \}\}$", call)
        self.assertEqual(len(forwarded), len(declared))
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
            "DEV_TERRAFORM_SECURITY_PROBE_ENABLED": "false",
            "PROD_AUTHENTICATED_TERRAFORM_ENABLED": "false", "DEV_PRIMARY_EMAIL_IMPORT_ENABLED": "true", "DEV_QUOTA_CREATE_ENABLED": "false", "DEV_QUOTA_MANAGED_ENABLED": "false", "DEV_QUOTA_STATE_REFRESH_ENABLED": "false", "DEV_EXPORT_TOPIC_MANAGED_ENABLED": "true", "DEV_EXPORT_SCHEDULER_MANAGED_ENABLED": "true", "DEV_EXPORT_FUNCTION_MANAGED_ENABLED": "true",
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

    def test_function_requires_both_adopted_dependencies_and_same_private_input(self):
        self.assertIn('DEV_EXPORT_FUNCTION_MANAGED_ENABLED: "true"', self.text)
        result = self.run_preflight()
        self.assertEqual(result.returncode, 0)
        self.assertIn("manage_export_function=true", result.outputs)
        self.assertEqual(self.text.count("TF_VAR_manage_export_function: ${{ needs.preflight.outputs.manage_export_function }}"), 2)
        self.assertIn('echo "manage_export_function=false"', self.text)
        for flag in ("DEV_EXPORT_TOPIC_MANAGED_ENABLED", "DEV_EXPORT_SCHEDULER_MANAGED_ENABLED"):
            self.assertNotEqual(self.run_preflight(**{flag: "false"}).returncode, 0)
        self.assertIn("EXPORT_FUNCTION_IDENTITY_REQUIRED", self.text)
        apply = self.text.split("  apply:\n", 1)[1]
        self.assertLess(apply.index("terraform_identity_smoke.py export-function"), apply.index("Re-plan at the exact approved commit"))
        self.assertIn("needs.preflight.outputs.manage_export_function != 'true'", apply)
        self.assertNotIn("TF_VAR_export_function_execution_service_account_email:", self.text)

    def test_history_plan_only_activation_requires_complete_dev_baseline(self):
        self.assertIn('DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED: "true"', self.text)
        self.assertIn('DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED: "false"', self.text)
        gates = {"DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED": "true", "DEV_QUOTA_MANAGED_ENABLED": "true"}
        enabled = self.run_preflight(**gates)
        self.assertEqual(enabled.returncode, 0)
        self.assertIn("manage_user_activity_history=true", enabled.outputs)
        for flag in ("DEV_EXPORT_FUNCTION_MANAGED_ENABLED", "DEV_EXPORT_SCHEDULER_MANAGED_ENABLED",
                     "DEV_EXPORT_TOPIC_MANAGED_ENABLED", "DEV_PRIMARY_EMAIL_IMPORT_ENABLED", "DEV_QUOTA_MANAGED_ENABLED"):
            self.assertNotEqual(self.run_preflight(**dict(gates, **{flag: "false"})).returncode, 0)
        for mode in ("email-adoption", "quota-plan", "quota-create", "quota-refresh"):
            self.assertNotEqual(self.run_preflight(**gates, MODE=mode,
                DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED="true", DEV_QUOTA_CREATE_ENABLED="true",
                DEV_QUOTA_STATE_REFRESH_ENABLED="true").returncode, 0)
        self.assertNotEqual(self.run_preflight(DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED="invalid").returncode, 0)
        self.assertNotEqual(self.run_preflight(**gates, TARGET="prod").returncode, 0)

    def test_history_metadata_is_fresh_in_both_jobs_and_after_saved_apply(self):
        plan, apply = self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)
        for job in (plan, apply):
            self.assertIn("TF_VAR_manage_user_activity_history: ${{ needs.preflight.outputs.manage_user_activity_history }}", job)
            self.assertIn("needs.preflight.outputs.manage_user_activity_history == 'true'", job)
            self.assertIn('"${RUNNER_TEMP}/user-history-before.json"', job)
            self.assertIn('"${RUNNER_TEMP}/user-history-post.json"', job)
            self.assertIn('"${RUNNER_TEMP}/user-history-before.tfvars.json"', job)
            self.assertIn('"${RUNNER_TEMP}/user-history-post.tfvars.json"', job)
        self.assertLess(plan.index("Verify development identity"), plan.index("prepare_user_activity_history_workflow.py --phase before"))
        self.assertLess(plan.index("prepare_user_activity_history_workflow.py --phase before"), plan.index("Create saved plan"))
        self.assertLess(apply.index("prepare_user_activity_history_workflow.py --phase before"), apply.index("Re-plan at the exact"))
        self.assertLess(apply.index("Apply the locally"), apply.index("prepare_user_activity_history_workflow.py --phase post"))
        self.assertLess(apply.index("prepare_user_activity_history_workflow.py --phase post"), apply.index("Require post-apply no-op"))
        self.assertIn("history_metadata_digest: ${{ steps.history_metadata.outputs.history_metadata_digest }}", plan)
        self.assertEqual(apply.count("HISTORY_APPROVED_METADATA_DIGEST: ${{ needs.plan.outputs.history_metadata_digest }}"), 2)
        self.assertNotIn("terraform import", self.text)
        self.assertNotIn("TF_VAR_user_activity_history_field_order", self.text)
        self.assertNotIn("YSS_USER_ACTIVITY_HISTORY_METADATA_FILE", self.text)

    def test_history_private_varfile_is_passed_to_each_plan_only_when_enabled(self):
        cases = []
        for step in self.text.split("      - name: ")[1:]:
            name = step.split("\n", 1)[0]
            if name in {"Create saved plan without public output", "Re-plan at the exact approved commit", "Require post-apply no-op"}:
                cases.append((name, textwrap.dedent(step.split("        run: |\n", 1)[1])))
        self.assertEqual(len(cases), 3)
        for name, script in cases:
            if name == "Require post-apply no-op":
                script = script.split("\nplan_rc=$?", 1)[0] + "\nexit 0\n"
            for enabled in ("true", "false"):
                with self.subTest(step=name, enabled=enabled), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    fake = root / "terraform"
                    fake.write_text('#!/bin/bash\nprintf "%s\\n" "$@" > "$ARGUMENT_RECORD"\nexit 0\n')
                    fake.chmod(0o700)
                    env = {"PATH": directory + ":" + os.environ["PATH"], "RUNNER_TEMP": directory,
                           "TF_ROOT": "synthetic-root", "MODE": "apply", "TF_VAR_manage_user_activity_history": enabled,
                           "GITHUB_OUTPUT": str(root / "runner-output"), "ARGUMENT_RECORD": str(root / "arguments")}
                    result = subprocess.run(["bash", "-c", script], env=env, capture_output=True, text=True)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    arguments = (root / "arguments").read_text().splitlines()
                    phase = "post" if name == "Require post-apply no-op" else "before"
                    expected = f"-var-file={directory}/user-history-{phase}.tfvars.json"
                    self.assertEqual(expected in arguments, enabled == "true")
                    self.assertNotIn(directory, result.stdout + result.stderr)
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

    def test_source_history_activation_permits_only_protected_plan(self) -> None:
        gates = dict(re.findall(r'(?m)^  ([A-Z_]+): "(true|false)"$', self.text))
        self.assertEqual(gates["DEV_USER_ACTIVITY_HISTORY_MANAGED_ENABLED"], "true")
        self.assertEqual(gates["DEV_AUTHENTICATED_TERRAFORM_APPLY_ENABLED"], "false")
        result = self.run_preflight(**gates)
        self.assertEqual(result.returncode, 0)
        self.assertIn("manage_user_activity_history=true", result.outputs)
        for mode in ("apply", "email-adoption", "quota-create", "quota-refresh", "quota-plan", "security-probe"):
            with self.subTest(mode=mode):
                rejected = self.run_preflight(MODE=mode, **gates)
                self.assertNotEqual(rejected.returncode, 0)
                if mode not in ("quota-plan", "security-probe"):
                    self.assertIn("Development apply is disabled", rejected.stdout)
        for key, value in {
            "GITHUB_EVENT_NAME": "pull_request", "GITHUB_REPOSITORY_ID": "0",
            "GITHUB_REF": "refs/heads/dev", "GITHUB_WORKFLOW_REF": "wrong/workflow",
            "TARGET": "prod", "DEV_AUTHENTICATED_TERRAFORM_ENABLED": "false",
        }.items():
            with self.subTest(key=key):
                self.assertNotEqual(self.run_preflight(**(gates | {key: value})).returncode, 0)

    def test_all_plan_job_routes_use_explicit_read_only_smoke(self):
        plan = self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)[0]
        step = plan.split("      - name: Verify development identity boundaries without public identifiers\n", 1)[1].split("      - name: ", 1)[0]
        self.assertIn("MODE: ${{ inputs.mode }}", step)
        script = textwrap.dedent(step.split("        run: |\n", 1)[1])
        with tempfile.TemporaryDirectory() as directory:
            recorder = Path(directory) / "python3"
            recorder.write_text('#!/bin/bash\nprintf "%s\\n" "$@"\n')
            recorder.chmod(0o700)
            for mode in ("plan", "apply", "email-adoption", "quota-plan", "quota-create", "quota-refresh"):
                with self.subTest(mode=mode):
                    result = subprocess.run(["bash", "-c", script], env={"PATH": directory + os.pathsep + os.environ["PATH"], "MODE": mode},
                                            capture_output=True, text=True)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    expected = "plan-read-only" if mode in ("plan", "quota-plan") else "apply-read-only"
                    self.assertEqual(result.stdout.splitlines(), [".github/scripts/terraform_identity_smoke.py", expected])
        apply = self.text.split("  apply:\n", 1)[1]
        self.assertNotIn("plan-read-only", apply)
        self.assertNotIn("terraform_identity_smoke.py\n", plan)
        function = apply.split("      - name: Verify Function identity exact read permissions\n", 1)[1].split("      - name: ", 1)[0]
        script = textwrap.dedent(function.split("        run: |\n", 1)[1])
        with tempfile.TemporaryDirectory() as directory:
            recorder = Path(directory) / "python3"
            recorder.write_text('#!/bin/bash\nprintf "%s\\n" "$@"\n')
            recorder.chmod(0o700)
            for mode, expected in (("apply", "export-function-apply-read-only"),
                                   ("quota-create", "export-function")):
                result = subprocess.run(["bash", "-c", script],
                    env={"PATH": directory + os.pathsep + os.environ["PATH"], "MODE": mode},
                    capture_output=True, text=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(result.stdout.splitlines(), [".github/scripts/terraform_identity_smoke.py", expected])

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
        self.assertEqual(self.text.count("id-token: write"), 3)
        self.assertEqual(self.caller.count("id-token: write"), 2)
        terraform_call = self.caller.split("  gcp-terraform-authenticated:\n", 1)[1].split(
            "\n  gcp-user-activity-schema-audit:\n", 1
        )[0]
        audit_call = self.caller.split("  gcp-user-activity-schema-audit:\n", 1)[1].split(
            "\n  ci-gate:\n", 1
        )[0]
        self.assertEqual(terraform_call.count("id-token: write"), 1)
        self.assertEqual(audit_call.count("id-token: write"), 1)
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
        self.assertEqual(self.text.count("persist-credentials: false"), 3)

    def test_disabled_security_probe_uses_plan_oidc_without_terraform_or_state_write_route(self) -> None:
        self.assertIn('DEV_TERRAFORM_SECURITY_PROBE_ENABLED: "false"', self.text)
        self.assertIn("          - security-probe\n", self.caller)
        self.assertNotEqual(self.run_preflight(MODE="security-probe").returncode, 0)
        enabled = self.run_preflight(MODE="security-probe", DEV_TERRAFORM_SECURITY_PROBE_ENABLED="true")
        self.assertEqual(enabled.returncode, 0, enabled.stdout)
        self.assertIn("manage_user_activity_history=false", enabled.outputs)
        self.assertNotEqual(self.run_preflight(MODE="security-probe", TARGET="prod",
            DEV_TERRAFORM_SECURITY_PROBE_ENABLED="true").returncode, 0)
        self.assertNotEqual(self.run_preflight(MODE="apply", DEV_TERRAFORM_SECURITY_PROBE_ENABLED="true").returncode, 0)
        plan = self.text.split("  plan:\n", 1)[1].split("  apply:\n", 1)[0]
        self.assertIn("if: ${{ inputs.mode != 'security-probe' }}", plan)
        probe = self.text.split("  security-probe:\n", 1)[1]
        for required in ("needs: preflight", "inputs.mode == 'security-probe'", "inputs.target == 'dev'",
                         "name: terraform-dev-plan", "id-token: write", "ref: ${{ github.sha }}",
                         "persist-credentials: false", "git rev-parse HEAD", "${GITHUB_SHA}",
                         "AWS_TERRAFORM_BACKEND_ROLE_ARN", "GCP_TERRAFORM_WIF_PROVIDER",
                         "terraform_identity_smoke.py security-probe"):
            self.assertIn(required, probe)
        for forbidden in ("setup-terraform", "terraform -chdir", "terraform init", "terraform plan",
                          "terraform apply", "backend.hcl", "--if-none-match"):
            self.assertNotIn(forbidden, probe)

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
