"""環境に固定したMyPage基盤の検査。認証情報やクラウド入出力は使わない。"""
from __future__ import annotations

import hashlib
import hmac
import json
import os
import re
from datetime import datetime
from pathlib import Path

TARGET = os.environ.get("MYPAGE_TARGET", "development")
TARGETS = {
    "development": {
        "project": "test-youtube-study-space", "project_number": "48101442817",
        "region": "asia-southeast2", "ref": "refs/heads/integration/mypage-canon-20261006",
        "state": "dev", "environment": "development", "github_environment": "mypage-dev",
        "origin": "https://test--study-space.web.app",
    },
    "production": {
        "project": "youtube-study-space", "project_number": "906336399194",
        "region": "asia-northeast2", "ref": "refs/heads/main",
        "state": "prod", "environment": "production", "github_environment": "mypage-prod",
        "origin": "https://moku.work",
    },
}
if TARGET not in TARGETS:
    raise ValueError("foundation-target")
CONFIG = TARGETS[TARGET]
PROJECT = CONFIG["project"]
PROJECT_NUMBER = CONFIG["project_number"]
REGION = CONFIG["region"]
REF = CONFIG["ref"]
ENVIRONMENT = CONFIG["environment"]
GITHUB_ENVIRONMENT = CONFIG["github_environment"]
ORIGIN = CONFIG["origin"]
REPOSITORY = "kani3camp/youtube-study-space"
WORKFLOWS = REPOSITORY + "/.github/workflows/"


def workflow_ref(name):
    return WORKFLOWS + name + "@" + REF


# Terraform Plan / Apply caller (the manual Terraform workflow).
CALLER = workflow_ref("terraform.yml")
REUSABLE = workflow_ref("mypage-foundation-authenticated.yml")
# 各imageは、それを配備するユースケースworkflowからだけbuildする。
# packageを分けることで、API配備がJob用imageを取り違えない。
IMAGE_COMPONENTS = {
    "api": {"package": "mypage-api", "dockerfile": "system/Dockerfile.mypage-api",
            "check": "system/scripts/check-mypage-image.sh", "caller": workflow_ref("deploy-mypage-api.yml")},
    "maintenance-operator": {"package": "mypage-maintenance-operator",
                             "dockerfile": "system/Dockerfile.mypage-maintenance-operator",
                             "check": "system/scripts/check-mypage-operator-image.sh",
                             "caller": workflow_ref("deploy-mypage-maintenance.yml")},
}
STATE_KEY = "youtube-study-space/mypage-platform/" + CONFIG["state"] + "/terraform.tfstate"
WORKSPACE_PREFIX = "youtube-study-space/mypage-platform/" + CONFIG["state"] + "/workspaces"
EMAIL = "mypage-api@" + PROJECT + ".iam.gserviceaccount.com"
SECRETS = ("oauth_client_id", "oauth_client_secret", "youtube_api_key")
SECRET_IDS = dict(zip(SECRETS, ("mypage-oauth-client-id", "mypage-oauth-client-secret", "mypage-youtube-api-key")))
CLAIM_KEYS = ("repository_id", "repository_owner_id", "environment", "ref", "workflow_ref", "job_workflow_ref", "event_name")


class ContractError(ValueError):
    """ソースで定義した固定の検査ラベルだけを使い、拒否した値は含めない。"""


def need(value, check="contract"):
    if not value:
        raise ContractError(check)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()


def image_component(env):
    component = env.get("MYPAGE_IMAGE_COMPONENT")
    need(component in IMAGE_COMPONENTS, "image-component")
    return IMAGE_COMPONENTS[component]


def caller(env, *, image=False):
    return image_component(env)["caller"] if image else CALLER


def subject(kind, event_name="workflow_dispatch", workflow=CALLER):
    # Every credentialed MyPage operation is an owner's manual dispatch.
    need(kind in {"plan", "apply"} and event_name == "workflow_dispatch")
    need(workflow in {CALLER} | {item["caller"] for item in IMAGE_COMPONENTS.values()}, "subject-workflow")
    values = ("340900071", "54093651", GITHUB_ENVIRONMENT + "-" + kind, REF, workflow, REUSABLE, event_name)
    return ":".join(key + ":" + value.replace(":", "%3A") for key, value in zip(CLAIM_KEYS, values))


# GCP rejects a provider attribute condition longer than this.
CONDITION_LIMIT = 4096


def provider_condition():
    """Exact condition for the owner-managed `mypage-foundation` WIF provider.

    The custom `sub` already pins Environment, ref, caller, reusable and event,
    so each allowed (Environment, caller) pair is one exact `sub`. Plan admits
    only the Terraform caller; Apply admits the Terraform caller and each
    image's own Deploy workflow. Push is never admitted.
    """
    pairs = [("plan", CALLER), ("apply", CALLER)]
    pairs += [("apply", item["caller"]) for _, item in sorted(IMAGE_COMPONENTS.items())]
    subjects = [subject(kind, "workflow_dispatch", workflow) for kind, workflow in pairs]
    common = {"repository_id": "340900071", "repository_owner_id": "54093651",
              "actor_id": "54093651", "actor": "kani3camp", "repository": REPOSITORY,
              "ref": REF, "job_workflow_ref": REUSABLE, "event_name": "workflow_dispatch",
              "run_attempt": "1"}
    condition = (" && ".join("assertion." + key + " == " + json.dumps(value) for key, value in common.items()) +
                 " && assertion.workflow_sha == assertion.job_workflow_sha" +
                 " && assertion.sub in " + json.dumps(subjects, separators=(", ", ": ")))
    need(len(condition) <= CONDITION_LIMIT, "provider-condition-length")
    return condition


def trust_policy(account, kind):
    need(re.fullmatch(r"[0-9]{12}", account))
    # Terraform State credentials are dispatch-only.
    events = ["workflow_dispatch"]
    return {"Version": "2012-10-17", "Statement": [{
        "Effect": "Allow", "Action": "sts:AssumeRoleWithWebIdentity",
        "Principal": {"Federated": f"arn:aws:iam::{account}:oidc-provider/token.actions.githubusercontent.com"},
        "Condition": {"StringEquals": {"token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
                                        "token.actions.githubusercontent.com:sub": subject(kind, event),
                                        # 変更されないactorの照合はSTSでも行う。ワークフロー内の検査だけでは変更できてしまう。
                                        "token.actions.githubusercontent.com:actor_id": "54093651"}},
    } for event in events]}


def backend_policy(bucket, kind):
    need(kind in {"plan", "apply"} and re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", bucket))
    arn = "arn:aws:s3:::" + bucket
    return {"Version": "2012-10-17", "Statement": [
        {"Sid": "MyPageDefaultState", "Effect": "Allow", "Action": ["s3:GetObject"] + (["s3:PutObject"] if kind == "apply" else []), "Resource": arn + "/" + STATE_KEY},
        {"Sid": "MyPageNativeLock", "Effect": "Allow", "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"], "Resource": arn + "/" + STATE_KEY + ".tflock"},
        {"Sid": "MyPageDiscovery", "Effect": "Allow", "Action": "s3:ListBucket", "Resource": arn,
         "Condition": {"StringEquals": {"s3:prefix": [STATE_KEY, STATE_KEY + ".tflock", WORKSPACE_PREFIX + "/"]}}},
    ]}


def desired_stage():
    path = Path(__file__).resolve().parents[1] / "environments" / CONFIG["state"] / "desired-stage.txt"
    need(path.is_file() and not path.is_symlink(), "desired-stage-file")
    stage = path.read_text(encoding="utf-8").strip()
    need(stage in {"disabled", "foundation_metadata", "foundation", "registry",
                   "maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"},
         "desired-stage-value")
    return stage


def stage_for_mode(mode):
    from runtime_contract import MODES
    if mode in {"desired-plan", "desired-apply"}:
        return desired_stage()
    if mode in MODES:
        if TARGET == "production":
            need(mode in {"maintenance-operator-plan", "maintenance-operator-apply",
                          "maintenance-upgrade-plan", "maintenance-upgrade-apply"}, "production-maintenance-mode")
        return MODES[mode]
    if mode in {"registry-plan", "registry-apply"}:
        return "registry"
    need(TARGET == "development", "production-maintenance-mode")
    need(mode in {"foundation-plan", "foundation-apply", "foundation-metadata-plan", "foundation-metadata-apply"})
    return "foundation_metadata" if mode.startswith("foundation-metadata-") else "foundation"


def context(env, kind, *, image=False):
    need(kind in {"plan", "apply"})
    expected = {"GITHUB_REPOSITORY": REPOSITORY, "GITHUB_REPOSITORY_ID": "340900071",
                "GITHUB_REPOSITORY_OWNER_ID": "54093651", "GITHUB_EVENT_NAME": "workflow_dispatch",
                "GITHUB_ACTOR": "kani3camp", "GITHUB_ACTOR_ID": "54093651",
                "GITHUB_TRIGGERING_ACTOR": "kani3camp", "GITHUB_RUN_ATTEMPT": "1",
                "GITHUB_REF": REF, "GITHUB_WORKFLOW_REF": caller(env, image=image), "TF_WORKSPACE": "default"}
    need(env.get("GITHUB_EVENT_NAME") == "workflow_dispatch", "event-name")
    need(all(env.get(k) == v for k, v in expected.items()))
    sha = env.get("GITHUB_SHA", "")
    need(re.fullmatch(r"[a-f0-9]{40}", sha) and env.get("MYPAGE_EXPECTED_SHA") == sha)
    if image:
        need(kind == "apply" and env.get("MYPAGE_MODE") == "image-build")
    else:
        stage_for_mode(env.get("MYPAGE_MODE"))
        need(kind != "apply" or env["MYPAGE_MODE"].endswith("-apply"))


def resource_contract(stage="foundation"):
    need(stage in {"disabled", "foundation_metadata", "foundation", "registry"})
    if stage == "disabled":
        return {}
    resources = {"google_service_account.runtime": {"project": PROJECT, "account_id": "mypage-api"}}
    for name, secret_id in SECRET_IDS.items():
        resources["google_secret_manager_secret." + name] = {
            "project": PROJECT, "secret_id": secret_id, "deletion_protection": True,
            "replication": [{"auto": [{"customer_managed_encryption": []}], "user_managed": []}],
        }
        resources["google_secret_manager_secret_iam_member." + name + "_runtime[0]"] = {
            "project": PROJECT, "secret_id": secret_id, "role": "roles/secretmanager.secretAccessor",
            "member": "serviceAccount:" + EMAIL,
        }
    resources["google_service_account_iam_member.runtime_signer[0]"] = {
        "service_account_id": "projects/" + PROJECT + "/serviceAccounts/" + EMAIL,
        "role": "roles/iam.serviceAccountTokenCreator", "member": "serviceAccount:" + EMAIL,
    }
    if stage == "registry":
        resources["google_artifact_registry_repository.mypage[0]"] = {
            "project": PROJECT, "location": REGION, "repository_id": "mypage",
            "description": "YouTube Study Space MyPage " + ENVIRONMENT + " images",
            "format": "DOCKER", "mode": "STANDARD_REPOSITORY",
            "cleanup_policy_dry_run": True, "docker_config": [{"immutable_tags": True}],
            "remote_repository_config": [], "virtual_repository_config": [], "cleanup_policies": [],
        }
    return {"module.platform[0]." + key: value for key, value in resources.items()
            if stage != "foundation_metadata" or "_iam_member." not in key}


def has_unknown(value):
    if isinstance(value, dict):
        return any(has_unknown(child) for child in value.values())
    if isinstance(value, list):
        return any(has_unknown(child) for child in value)
    need(value is None or type(value) is bool)
    return value is True


def registry_timestamp(value):
    need(type(value) is str, "registry-refresh-time-format")
    match = re.fullmatch(r"([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2})(?:\.([0-9]{1,9}))?Z", value)
    need(match is not None, "registry-refresh-time-format")
    try:
        seconds = datetime.strptime(match[1], "%Y-%m-%dT%H:%M:%S")
    except ValueError:
        need(False, "registry-refresh-time-format")
    return seconds, int((match[2] or "").ljust(9, "0"))


def validate_refresh(plan, resources):
    """RefreshはStateの観測。provider出力の形は再定義せず、適用する操作と最終設定を検証する。"""
    planned = {item.get("address"): item for item in plan.get("resource_changes", [])}
    seen = set()
    for item in plan.get("resource_drift", []) or []:
        address = item.get("address")
        need(address in resources and address not in seen, "refresh-resource")
        seen.add(address)
        need(item.get("mode") == "managed" and item.get("provider_name") == "registry.terraform.io/hashicorp/google", "refresh-provider")
        change = item.get("change", {})
        need(change.get("actions") == ["update"] and not change.get("importing") and not change.get("replace_paths"), "refresh-action")
        # 変更を取り込むだけでありremote writeは行わない。最終desired値はvalidate_planで検査する。
        need(planned.get(address, {}).get("change", {}).get("actions") == ["no-op"], "refresh-planned-noop")
        need(planned[address]["change"].get("after") == change.get("after"), "refresh-planned-state")


def validate_plan(plan, stage="foundation"):
    need(plan.get("format_version") == "1.2", "plan-format")
    need(plan.get("complete") is True, "plan-complete")
    need(plan.get("errored") is False, "plan-errored")
    validate_refresh(plan, resource_contract(stage))
    need(not plan.get("deferred_changes"), "deferred-changes")
    need(not plan.get("action_invocations") and not plan.get("deferred_action_invocations"), "provider-actions")
    need(all(item.get("status") == "pass" for item in plan.get("checks", []) or []), "plan-checks")
    need(plan.get("variables", {}).get("stage", {}).get("value") == stage, "plan-stage")
    contract, seen = resource_contract(stage), set()
    for item in plan.get("resource_changes", []):
        address = item.get("address")
        need(address in contract and address not in seen and item.get("mode") == "managed", "resource-address-mode")
        seen.add(address)
        need(item.get("provider_name") == "registry.terraform.io/hashicorp/google", "resource-provider")
        change = item["change"]
        need(change.get("actions") in [["create"], ["no-op"]] and not change.get("importing") and not change.get("replace_paths"), "resource-action")
        after = change.get("after")
        need(type(after) is dict and not after.get("condition"), "resource-after-condition")
        for key, value in contract[address].items():
            if key == "replication":
                # 自動レプリケーションだけを許可する。プロバイダーバージョンによっては空リストを省略する。
                replication = after.get(key)
                need(type(replication) is list and len(replication) == 1, "secret-replication")
                need(not replication[0].get("user_managed") and len(replication[0].get("auto", [])) == 1, "secret-auto-replication")
                need(not replication[0]["auto"][0].get("customer_managed_encryption"), "secret-encryption")
            elif key == "secret_id" and address in {
                "module.platform[0].google_secret_manager_secret_iam_member." + name + "_runtime[0]"
                for name in SECRET_IDS
            }:
                # Google 8.5.0では、作成・再取得後のIAM資源のSecret識別子を完全リソース名で保存する。
                # 同じプロジェクトとSecretの完全一致だけを受け入れ、任意の接尾辞で照合しない。
                need(after.get(key) in {value, "projects/" + PROJECT + "/secrets/" + value}, "resource-value-secret_id")
            else:
                # keyは固定の資源仕様から取り、Planからは取らない。
                need(after.get(key) == value, "resource-value-" + key)
            need(not has_unknown(change.get("after_unknown", {}).get(key)), "resource-unknown-" + key)
        if address.endswith("google_service_account.runtime"):
            need(after.get("disabled") in {None, False}, "runtime-disabled")
    need(seen == set(contract), "resource-set")
    return {"resources": len(seen), "creates": sum(x["change"]["actions"] == ["create"] for x in plan["resource_changes"])}


def commitment(plan, state_version, sha, key):
    need(type(key) is str and len(key) >= 32)
    # Terraformは依存元を順序なしのrelevant_attributesとして列挙し、ネイティブPlanごとに順序が変わり得る。
    # 各エントリーの内容と重複数を保持する。他の配列、特にaction列と属性パスの順序は維持する。
    stable = {k: v for k, v in plan.items() if k != "timestamp"}
    if "relevant_attributes" in stable:
        need(type(stable["relevant_attributes"]) is list, "plan-relevant-attributes")
        stable["relevant_attributes"] = sorted(stable["relevant_attributes"], key=canonical)
    return hmac.new(key.encode(), canonical({"plan": stable, "state_version": state_version, "source": sha}), hashlib.sha256).hexdigest()


if __name__ == "__main__":
    import sys
    # Print-only helper for the owner; it performs no cloud call.
    need(sys.argv[1:] == ["provider-condition"], "foundation-contract-command")
    print(provider_condition())
