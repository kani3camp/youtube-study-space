"""deletion_approval.py と、削除を伴うPlanの承認条件をテストする。

目的: 保護されていないEnvironment、管理者による迂回、別の実行での承認、承認なしの削除が通ってしまう不具合を検出する。
使われる場面: infra/mypage-platform/scripts/check-local.sh の `unittest discover`。CI GCP Terraform の terraform-tests(validate.sh 経由)で実行される。
失敗したとき: CI GCP Terraform が失敗し、PRをマージできない。
なくした場合: 削除承認の確認が壊れても気づけなくなる。
"""
from copy import deepcopy
import hashlib
import json
from pathlib import Path
import tempfile
import time
import unittest
from unittest.mock import patch

from test_foundation import c, ops, plan_fixture
from test_runtime import fixture as runtime_fixture
import deletion_approval as approval


ADDRESS = "module.platform[0].google_service_account_iam_member.retired[0]"
PROVIDER = "registry.terraform.io/hashicorp/google"
PRIVATE = "secret_value_not_for_logs"
NAME = "mypage-dev-delete-approval"


def deletion_plan(stage="foundation"):
    plan = plan_fixture() if stage == "foundation" else runtime_fixture(stage)
    values = {"id": PRIVATE}
    item = {"address": ADDRESS, "mode": "managed", "provider_name": PROVIDER}
    plan["resource_changes"].append(item | {"change": {"actions": ["delete"], "before": values, "after": None}})
    plan["prior_state"] = {"values": {"root_module": {"child_modules": [
        {"address": "module.platform[0]", "resources": [item | {"values": deepcopy(values)}]},
    ]}}}
    return plan


class DeletionPlanTest(unittest.TestCase):
    def test_a_retired_state_owned_address_can_be_planned_without_exposing_values(self):
        for stage in ("foundation", "runtime_public"):
            plan = deletion_plan(stage)
            with self.subTest(stage=stage), tempfile.TemporaryDirectory() as directory:
                _, deletions = ops.validate_plan(plan, stage)
                self.assertEqual(deletions, (ADDRESS,))
                self.assertEqual(plan["resource_changes"][-1]["change"]["actions"], ["delete"])
                summary = Path(directory) / "summary"
                ops.publish_plan_summary({"GITHUB_STEP_SUMMARY": str(summary)}, plan, stage, deletions)
                output = summary.read_text()
                self.assertIn("Standalone deletes: **1**", output)
                self.assertIn(ADDRESS, output)
                self.assertIn(c.ENVIRONMENT, output)
                self.assertNotIn(PRIVATE, output)
                changed = deepcopy(plan)
                changed["resource_changes"][-1]["change"]["before"]["id"] = "different"
                self.assertNotEqual(c.commitment(plan, "v1", "a" * 40, "k" * 32),
                                    c.commitment(changed, "v1", "a" * 40, "k" * 32))

    def test_invalid_or_protected_deletions_cannot_enter_the_approval_path(self):
        for mutation in (
            lambda p: p["resource_changes"][-1]["change"].update(actions=["delete", "create"]),
            lambda p: p["resource_changes"][-1]["change"].update(actions=["create", "delete"]),
            lambda p: p["resource_changes"][-1]["change"].update(importing={"id": "old"}),
            lambda p: p["resource_changes"][-1].update(deposed="stub"),
            lambda p: p["resource_changes"][-1].update(previous_address="stub"),
            lambda p: p["resource_changes"][-1].update(address=p["resource_changes"][0]["address"]),
            lambda p: p["resource_changes"][-1].update(address="module.core.google_project_iam_member.target"),
            lambda p: p["resource_changes"][-1].update(provider_name="registry.terraform.io/hashicorp/aws"),
            lambda p: p["resource_changes"][-1]["change"].update(after={"id": "stub"}),
            lambda p: p["resource_changes"].append(deepcopy(p["resource_changes"][-1])),
            lambda p: p.pop("prior_state"),
        ):
            plan = deletion_plan()
            mutation(plan)
            with self.subTest(plan=plan["resource_changes"][-1]), self.assertRaises(c.ContractError):
                ops.validate_plan(plan, "foundation")
        for policy in ({"deletion_protection": True}, {"deletion_policy": "PREVENT"},
                       {"deletion_policy": "ABANDON"}):
            plan = deletion_plan()
            before = plan["resource_changes"][-1]["change"]["before"]
            before.update(policy)
            plan["prior_state"]["values"]["root_module"]["child_modules"][0]["resources"][0]["values"] = before
            with self.subTest(policy=policy), self.assertRaisesRegex(c.ContractError, "deletion-protected-resource"):
                ops.validate_plan(plan, "foundation")

    def test_opt_in_is_required_only_when_a_delete_would_be_applied(self):
        ops.require_deletion_opt_in({"MYPAGE_ROLE": "plan", "MYPAGE_OPERATION": "plan"}, (ADDRESS,))
        for path in ({"MYPAGE_ROLE": "plan", "MYPAGE_OPERATION": "apply"}, {"MYPAGE_ROLE": "apply"}):
            with self.subTest(path=path):
                ops.require_deletion_opt_in(path, ())
                ops.require_deletion_opt_in(path | {"MYPAGE_ALLOW_DELETIONS": "true"}, (ADDRESS,))
                with self.assertRaisesRegex(c.ContractError, "enable-allow-mypage-deletions-to-apply"):
                    ops.require_deletion_opt_in(path | {"MYPAGE_ALLOW_DELETIONS": "false"}, (ADDRESS,))

    def test_saved_plan_requires_both_opt_in_and_approval_before_native_apply(self):
        plan = deletion_plan()
        for opt_in, approved in (("false", "false"), ("false", "true"), ("true", "false"), ("true", "true")):
            with self.subTest(opt_in=opt_in, approved=approved), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                saved = b"saved-plan"
                sha = "a" * 40
                (root / "foundation.tfplan").write_bytes(saved)
                (root / "prepared.json").write_text(json.dumps({"sha256": hashlib.sha256(saved).hexdigest(), "source": sha}))
                env = {"MYPAGE_ROLE": "apply", "MYPAGE_MODE": "foundation-apply", "GITHUB_SHA": sha,
                       "MYPAGE_PLAN_TIME": str(time.time()), "MYPAGE_ALLOW_DELETIONS": opt_in,
                       "MYPAGE_DELETION_APPROVED": approved}
                def run(args, **kwargs):
                    if args[:3] == ["terraform", "show", "-json"] and len(args) == 4:
                        return json.dumps(plan)
                    if args[:2] == ["terraform", "apply"]:
                        return ""
                    return json.dumps({"values": {"root_module": {"resources": [
                        {"address": address} for address in c.resource_contract("foundation")
                    ]}}})
                with patch.object(c, "context"), patch.object(ops, "directory", return_value=root), \
                     patch.object(ops, "state_version", return_value="new"), patch.object(ops, "run", side_effect=run) as native:
                    if opt_in == approved == "true":
                        ops.apply_saved(env)
                    else:
                        with self.assertRaises(c.ContractError):
                            ops.apply_saved(env)
                applied = [call for call in native.call_args_list if call.args[0][:2] == ["terraform", "apply"]]
                self.assertEqual(len(applied), int(opt_in == approved == "true"))


class GitHubApprovalTest(unittest.TestCase):
    def test_environment_requires_real_reviewers_and_no_admin_bypass(self):
        valid = {"id": 42, "name": NAME, "can_admins_bypass": False, "protection_rules": [
            {"type": "required_reviewers", "reviewers": [{"type": "User", "reviewer": {"id": 1}}]},
        ]}
        with patch.object(approval, "fetch", return_value=valid):
            self.assertEqual(approval.protected_environment(NAME), 42)
        for change in ({"id": "42"}, {"name": "other"}, {"can_admins_bypass": True}, {"protection_rules": []},
                       {"protection_rules": [{"type": "required_reviewers", "reviewers": []}]}):
            with self.subTest(change=change), patch.object(approval, "fetch", return_value=valid | change), \
                 self.assertRaises(c.ContractError):
                approval.protected_environment(NAME)

    def test_run_review_must_approve_the_same_environment_id_and_first_attempt(self):
        env = {"GITHUB_RUN_ID": "12", "GITHUB_RUN_ATTEMPT": "1", "MYPAGE_APPROVAL_ENVIRONMENT_ID": "42"}
        accepted = {"state": "approved", "environments": [{"id": 42, "name": NAME}]}
        with patch.object(approval, "fetch", return_value=[accepted]) as fetch:
            approval.verify(env, NAME, 42)
            fetch.assert_called_once_with("/actions/runs/12/approvals")
        for invalid in ([], [accepted | {"state": "rejected"}], [accepted | {"state": "bypassed"}],
                        [accepted | {"environments": [{"id": 43, "name": NAME}]}],
                        [accepted, accepted | {"state": "rejected"}]):
            with self.subTest(invalid=invalid), patch.object(approval, "fetch", return_value=invalid), \
                 self.assertRaisesRegex(c.ContractError, "deletion-run-not-approved"):
                approval.verify(env, NAME, 42)
        for changed in ({"GITHUB_RUN_ATTEMPT": "2"}, {"MYPAGE_APPROVAL_ENVIRONMENT_ID": "43"}):
            with self.subTest(changed=changed), self.assertRaises(c.ContractError):
                approval.verify(env | changed, NAME, 42)
