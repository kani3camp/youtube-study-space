#!/usr/bin/env python3
"""Production normal-operation boundary: exact main, project and state isolation."""
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))
import terraform_normal_ops as ops

REPO = "kani3camp/youtube-study-space"
PROD = "youtube-study-space"
DEV = "test-youtube-study-space"


def context(target="prod", role="plan"):
    branch = "main" if target == "prod" else "dev"
    return {
        "GITHUB_EVENT_NAME": "workflow_dispatch",
        "GITHUB_REPOSITORY": REPO, "GITHUB_REPOSITORY_ID": "340900071",
        "GITHUB_REPOSITORY_OWNER_ID": "54093651", "GITHUB_ACTOR": "kani3camp",
        "GITHUB_ACTOR_ID": "54093651", "GITHUB_TRIGGERING_ACTOR": "kani3camp",
        "GITHUB_RUN_ATTEMPT": "1", "GITHUB_REF": "refs/heads/" + branch,
        "GITHUB_WORKFLOW_REF": REPO + "/.github/workflows/ci.yml@refs/heads/" + branch,
        "GITHUB_SHA": "a" * 40, "NORMAL_EXPECTED_SHA": "a" * 40,
        "NORMAL_TARGET": target, "NORMAL_MODE": "normal-apply",
        "NORMAL_ROLE": role, "GITHUB_JOB": "normal_" + role,
        "GITHUB_RUN_ID": "1234", "TF_WORKSPACE": "default",
        "STATE_AWS_REGION": "ap-northeast-1",
    }


def native_plan(project=PROD):
    return {"format_version": "1.2", "terraform_version": ops.common.VERSION,
            "errored": False, "complete": True,
            "resource_changes": [{"address": "google_pubsub_topic.example",
                                  "mode": "managed", "provider_name": "registry.terraform.io/hashicorp/google",
                                  "type": "google_pubsub_topic",
                                  "change": {"actions": ["create"], "before": None,
                                             "after": {"project": project}}}]}


class ProductionNormalOps(unittest.TestCase):
    def test_only_main_and_owner_can_enter_prod(self):
        for role in ("plan", "apply"):
            self.assertEqual(ops.context(context(role=role), role), "prod")
            for bad in (
                {"GITHUB_REF": "refs/heads/dev"},
                {"GITHUB_REF": "refs/heads/feature/foo",
                 "GITHUB_WORKFLOW_REF": REPO + "/.github/workflows/ci.yml@refs/heads/feature/foo"},
                {"GITHUB_REF": "refs/tags/v1"},
                {"GITHUB_WORKFLOW_REF": REPO + "/.github/workflows/ci.yml@refs/heads/dev"},
                {"GITHUB_ACTOR_ID": "100"},
                {"GITHUB_TRIGGERING_ACTOR": "someone-else"},
                {"GITHUB_RUN_ATTEMPT": "2"},
                {"GITHUB_SHA": "f" * 40},
                {"NORMAL_EXPECTED_SHA": "b" * 40},
                {"GITHUB_JOB": "migration_" + role},
                {"TF_WORKSPACE": "production"},
            ):
                with self.subTest(role=role, bad=bad), self.assertRaises(ValueError):
                    ops.context(context(role=role) | bad, role)

    def test_prod_e2e_cannot_touch_real_cloud(self):
        env = context() | {"NORMAL_MODE": "normal-e2e"}
        with self.assertRaises(ValueError):
            ops.context(env)
        with self.assertRaises(ValueError):
            ops.e2e(env, None, None, None)

    def test_dev_normal_is_still_non_main(self):
        self.assertEqual(ops.context(context("dev")), "dev")
        branch = context("dev") | {
            "GITHUB_REF": "refs/heads/feature/safe",
            "GITHUB_WORKFLOW_REF": REPO + "/.github/workflows/ci.yml@refs/heads/feature/safe",
        }
        self.assertEqual(ops.context(branch), "dev")
        with self.assertRaises(ValueError):
            ops.context(context("dev") | {"GITHUB_REF": "refs/heads/main",
                                        "GITHUB_WORKFLOW_REF": REPO + "/.github/workflows/ci.yml@refs/heads/main"})

    def test_project_and_kind_are_pinned(self):
        self.assertEqual(ops.target_project(context()), PROD)
        self.assertEqual(ops.target_project(context("dev")), DEV)
        with self.assertRaises(ValueError):
            ops.target_project(context("staging"))
        for project in (PROD, DEV):
            p = native_plan(project)
            if project == PROD:
                self.assertEqual(ops.validate(p, {}, project=PROD)["create"], 1)
            else:
                with self.assertRaises(ValueError):
                    ops.validate(p, {}, project=PROD)
        with self.assertRaises(ValueError):
            ops.validate(native_plan(), {"google_pubsub_topic.example": ["delete"]}, project=PROD)

    def test_prod_private_inputs_cannot_select_dev_project(self):
        with patch.object(ops.common, "decode_input_transport",
                          return_value={"tfvars": {"project_id": PROD, "existing_resources": ["dataset"]},
                                        "plan_integrity_key": "0" * 64}):
            values, key = ops.inputs(context())
            self.assertEqual(values["project_id"], PROD)
            self.assertEqual(values["existing_resources"], ["dataset"])
            self.assertEqual(len(key), 32)
        with patch.object(ops.common, "decode_input_transport",
                          return_value={"tfvars": {"project_id": DEV},
                                        "plan_integrity_key": "0" * 64}):
            with self.assertRaises(ValueError):
                ops.inputs(context())

    def test_prod_review_is_explicit_and_default_empty(self):
        self.assertEqual(ops.review("prod"), {})
        self.assertEqual(ops.review("dev"), {})
        with self.assertRaises(ValueError):
            ops.review("staging")

    def test_state_reader_receives_exact_target(self):
        with patch.object(ops.common, "snapshot", return_value={"ok": True}) as snap:
            self.assertEqual(ops.snapshot(context(), Path("/tmp"), "before"), {"ok": True})
            self.assertEqual(snap.call_args.args[0]["MIGRATION_TARGET"], "prod")
        with patch.object(ops.common, "snapshot", return_value={"ok": True}) as snap:
            ops.snapshot(context("dev"), Path("/tmp"), "before")
            self.assertEqual(snap.call_args.args[0]["MIGRATION_TARGET"], "dev")

    def test_backend_key_and_root_never_cross_environments(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            for target in ("prod", "dev"):
                env = context(target)
                env.update({"STATE_BUCKET": "fixture-terraform-state", "STATE_ACCOUNT_ID": "123456789012"})
                tfroot = root / target
                tfroot.mkdir()
                with patch.object(ops, "source", return_value=tfroot), \
                     patch.object(ops.common, "terraform") as terraform:
                    out = ops.setup(env, root, {"project_id": ops.target_project(env)})
                self.assertEqual(out, tfroot)
                backend = (root / "backend.hcl").read_text()
                self.assertIn("youtube-study-space/" + target + "/terraform.tfstate", backend)
                self.assertIn("youtube-study-space/" + target + "/workspaces", backend)
                self.assertNotIn("youtube-study-space/" + ("dev" if target == "prod" else "prod") +
                                 "/terraform.tfstate", backend)
                terraform.assert_called_once()
                for temporary in ("terraform.rc", "inputs.tfvars.json", "backend.hcl"):
                    (root / temporary).unlink()

    def test_commitment_binds_target(self):
        value = native_plan()
        with patch.object(ops, "review", return_value={}):
            p1 = ops.commitment_payload(context(), {"project_id": PROD}, {"version": 1}, value)
            p2 = ops.commitment_payload(context("dev"), {"project_id": PROD}, {"version": 1}, value)
            self.assertNotEqual(p1, p2)
            self.assertEqual(p1["target"], "prod")
            digests = ops.commitment_components(context(), {"project_id": PROD}, {}, value, b"k" * 32)
            self.assertEqual(set(digests), set(ops.DIAGNOSTIC_SECTIONS))
            self.assertRegex(digests["source_run_mode"], r"^[0-9a-f]{64}$")

    def test_prod_identity_requires_prod_sa_and_denies_dev_scope(self):
        e = context()
        e.update({
            "STATE_ACCOUNT_ID": "123456789012",
            "BACKEND_ROLE_ARN": "arn:aws:iam::123456789012:role/youtube-study-space-terraform-prod-plan",
            "BACKEND_ROLE_ID": "AROAEXAMPLE",
            "GCP_SERVICE_ACCOUNT": "terraform-prod-normal-plan@" + PROD + ".iam.gserviceaccount.com",
        })
        perms = ["resourcemanager.projects.get", "pubsub.topics.create",
                 "pubsub.topics.update", "pubsub.topics.delete"]
        def google(_env, _service, name, data=None):
            if name == "v3/projects/" + PROD:
                return {"projectId": PROD, "state": "ACTIVE"}
            if name == "v3/projects/" + PROD + ":testIamPermissions":
                self.assertEqual(data, {"permissions": perms})
                return {"permissions": perms[:1]}
            if name == "v3/projects/" + DEV + ":testIamPermissions":
                return {"permissions": []}
            raise AssertionError(name)
        with patch.object(ops.common, "aws", return_value={
                    "Account": e["STATE_ACCOUNT_ID"], "UserId": "AROAEXAMPLE:session",
                    "Arn": "arn:aws:sts::123456789012:assumed-role/youtube-study-space-terraform-prod-plan/session"}), \
             patch.object(ops.common, "google", side_effect=google):
            ops.identity(e)
            e["GCP_SERVICE_ACCOUNT"] = "terraform-dev-normal-plan@" + DEV + ".iam.gserviceaccount.com"
            with self.assertRaises(ValueError):
                ops.identity(e)

    def test_prod_normal_inputs_are_repository_only_not_migration(self):
        self.assertEqual(json.loads((ops.REPO / "infra/gcp/environments/prod/normal-operations.json").read_text()),
                         {"destructive_changes": {}})
        self.assertEqual(json.loads((ops.REPO / "infra/gcp/environments/prod/normal.tfvars.json").read_text()), {})


if __name__ == "__main__":
    unittest.main()
