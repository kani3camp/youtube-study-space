#!/usr/bin/env python3
"""Offline backend proposal boundaries, with synthetic identifiers only."""
from __future__ import annotations

import contextlib
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest

sys.dont_write_bytecode = True
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "scripts"))
import production_backend_packet as packet


def inventory():
    return {
        "schema_version": "production-backend-private-inventory-v1",
        "source_git_sha": "a" * 40, "observed_at": "2026-01-01T00:00:00Z",
        "state_account_id": "111111111111", "state_bucket": "fixture-private-shared-state",
        "state_region": "ap-northeast-1", "state_status": "absent", "bucket_encryption": "AES256",
        "production_plan_role_name": "fixture-production-plan", "production_apply_role_name": "fixture-production-apply",
        "development_plan_role_name": "fixture-development-plan", "development_apply_role_name": "fixture-development-apply",
        "oidc_include_claim_keys": list(packet.CLAIM_KEYS),
    }


def permits(policy, action, resource, **context):
    """Evaluate this proposal's exact Allow/StringEquals subset, no AWS claims."""
    for statement in policy["Statement"]:
        actions = statement["Action"]
        if isinstance(actions, str):
            actions = [actions]
        if statement["Effect"] != "Allow" or action not in actions or resource != statement["Resource"]:
            continue
        for key, expected in statement.get("Condition", {}).get("StringEquals", {}).items():
            if context.get(key) not in (expected if isinstance(expected, list) else [expected]):
                break
        else:
            return True
    return False


class ProductionBackendPacketTest(unittest.TestCase):
    def test_exact_default_state_lock_and_discovery_permissions_exclude_dev_and_data_delete(self):
        proposal = packet.build_packet(inventory())
        bucket = "arn:aws:s3:::fixture-private-shared-state"
        state = bucket + "/youtube-study-space/prod/terraform.tfstate"
        for kind, role in proposal["roles"].items():
            policy = role["permissions_policy"]
            self.assertTrue(permits(policy, "s3:GetObject", state))
            self.assertEqual(permits(policy, "s3:PutObject", state), kind == "apply")
            self.assertFalse(permits(policy, "s3:DeleteObject", state))
            self.assertFalse(permits(policy, "s3:GetObjectVersion", state))
            for action in ("s3:GetObject", "s3:PutObject", "s3:DeleteObject"):
                self.assertTrue(permits(policy, action, state + ".tflock"))
            for prefix in (packet.STATE_KEY, packet.STATE_KEY + ".tflock", packet.WORKSPACE_PREFIX + "/"):
                self.assertTrue(permits(policy, "s3:ListBucket", bucket, **{"s3:prefix": prefix}))
            for prefix in (None, "", "youtube-study-space/", "youtube-study-space/dev/", "unrelated-product/",
                           "youtube-study-space/prod/", packet.WORKSPACE_PREFIX + "/other/"):
                self.assertFalse(permits(policy, "s3:ListBucket", bucket, **{"s3:prefix": prefix}))
            for other in (bucket + "/youtube-study-space/dev/terraform.tfstate", bucket + "/unrelated-product/terraform.tfstate",
                          bucket + "/" + packet.WORKSPACE_PREFIX + "/unexpected/terraform.tfstate"):
                for action in ("s3:GetObject", "s3:PutObject", "s3:DeleteObject"):
                    self.assertFalse(permits(policy, action, other))
            self.assertNotIn("*", json.dumps(policy))
        self.assertEqual(proposal["cloud_mutations"], 0)
        self.assertFalse(proposal["live_readiness_attested"])

    def test_trust_exactly_binds_environment_repository_ref_caller_reusable_workflow_and_event(self):
        proposal = packet.build_packet(inventory())
        subjects = []
        for kind, role in proposal["roles"].items():
            statement, = role["trust_policy"]["Statement"]
            self.assertEqual(statement["Action"], "sts:AssumeRoleWithWebIdentity")
            self.assertEqual(statement["Principal"], {
                "Federated": "arn:aws:iam::111111111111:oidc-provider/token.actions.githubusercontent.com"})
            equals = statement["Condition"]["StringEquals"]
            self.assertEqual(equals["token.actions.githubusercontent.com:aud"], "sts.amazonaws.com")
            subject = equals["token.actions.githubusercontent.com:sub"]
            subjects.append(subject)
            replacements = {
                "repository_id:340900071": "repository_id:999999999",
                "repository_owner_id:54093651": "repository_owner_id:999999999",
                f"environment:terraform-prod-{kind}": f"environment:terraform-dev-{kind}",
                "ref:refs/heads/main": "ref:refs/heads/dev",
                "workflow_ref:kani3camp/youtube-study-space/.github/workflows/ci.yml@": "workflow_ref:kani3camp/youtube-study-space/.github/workflows/other.yml@",
                "job_workflow_ref:kani3camp/youtube-study-space/.github/workflows/gcp-terraform-authenticated.yml@": "job_workflow_ref:kani3camp/youtube-study-space/.github/workflows/other.yml@",
                "event_name:workflow_dispatch": "event_name:pull_request",
            }
            for before, after in replacements.items():
                with self.subTest(kind=kind, claim=before):
                    self.assertIn(before, subject)
                    self.assertNotEqual(subject.replace(before, after), equals["token.actions.githubusercontent.com:sub"])
            self.assertNotIn("*", json.dumps(role["trust_policy"]))
        self.assertNotEqual(*subjects)

    def test_missing_collision_unexpected_subject_and_credential_inputs_stop(self):
        cases = [{}, inventory() | {"unexpected": "private-sentinel"},
                 inventory() | {"AWS_SECRET_ACCESS_KEY": "private-sentinel"},
                 inventory() | {"production_plan_role_name": "fixture-development-plan"},
                 inventory() | {"production_apply_role_name": "fixture-production-plan"},
                 inventory() | {"state_status": "AccessDenied"}, inventory() | {"state_status": "unknown"},
                 inventory() | {"oidc_include_claim_keys": ["sub"]},
                 inventory() | {"oidc_include_claim_keys": list(reversed(packet.CLAIM_KEYS))},
                 inventory() | {"state_bucket": "invalid/*"}, inventory() | {"bucket_encryption": "aws:kms"},
                 inventory() | {"observed_at": "2030-01-01T00:00:00Z"},
                 inventory() | {"observed_at": "2026-01-01T00:00:00"}]
        for value in cases:
            with self.subTest(value=list(value)), self.assertRaises((ValueError, TypeError)):
                packet.build_packet(value)

    def test_cli_owner_only_output_no_private_stdout_and_no_overwrite(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "inventory.json"
            destination = Path(directory) / "packet.json"
            source.write_text(json.dumps(inventory()))
            source.chmod(0o600)
            out, err = io.StringIO(), io.StringIO()
            with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                self.assertEqual(packet.main(["--inventory", str(source), "--output", str(destination)]), 0)
                before = destination.read_bytes()
                self.assertEqual(packet.main(["--inventory", str(source), "--output", str(destination)]), 1)
            self.assertEqual(destination.stat().st_mode & 0o777, 0o600)
            self.assertEqual(destination.read_bytes(), before)
            for value in (inventory()["state_bucket"], inventory()["state_account_id"], inventory()["production_plan_role_name"]):
                self.assertNotIn(value, out.getvalue() + err.getvalue())

    def test_cli_rejects_public_input_or_output_duplicate_fields_and_symlinks(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "inventory.json"
            source.write_text(json.dumps(inventory()))
            source.chmod(0o600)
            output = Path(directory) / "packet.json"
            for bad_input, bad_output in ((source, packet.ROOT / "unexpected-private-packet.json"),):
                with contextlib.redirect_stderr(io.StringIO()):
                    self.assertEqual(packet.main(["--inventory", str(bad_input), "--output", str(bad_output)]), 1)
                self.assertFalse(bad_output.exists())
            source.chmod(0o644)
            with contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(packet.main(["--inventory", str(source), "--output", str(output)]), 1)
            self.assertFalse(output.exists())
            source.chmod(0o600)
            source.write_text('{"schema_version":"first","schema_version":"private-sentinel"}')
            link = Path(directory) / "link.json"
            link.symlink_to(source)
            for bad_input in (source, link):
                with contextlib.redirect_stderr(io.StringIO()):
                    self.assertEqual(packet.main(["--inventory", str(bad_input), "--output", str(output)]), 1)
            self.assertFalse(output.exists())

    def test_cli_rejects_output_in_another_git_checkout(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "inventory.json"
            source.write_text(json.dumps(inventory()))
            source.chmod(0o600)
            checkout = Path(directory) / "other-checkout"
            checkout.mkdir()
            (checkout / ".git").mkdir()
            output = checkout / "private-packet.json"
            with contextlib.redirect_stderr(io.StringIO()):
                self.assertEqual(packet.main(["--inventory", str(source), "--output", str(output)]), 1)
            self.assertFalse(output.exists())

    def test_cli_argument_errors_never_echo_private_values(self):
        for arguments in (["--inventory", "/private/private-sentinel.json", "--unknown", "private-sentinel"],
                          ["--unknown-private-sentinel"], ["--inventory"],
                          ["--inventory", "/private/private-sentinel.json"]):
            out, err = io.StringIO(), io.StringIO()
            with self.subTest(arguments=arguments), contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
                self.assertEqual(packet.main(arguments), 1)
            self.assertEqual(out.getvalue(), "")
            self.assertEqual(err.getvalue(), "Production backend proposal STOP; private diagnostic suppressed.\n")


if __name__ == "__main__":
    unittest.main()
