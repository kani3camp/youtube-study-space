"""Development-only MyPage foundation boundary; no credentials or cloud I/O."""
from __future__ import annotations

import hashlib
import hmac
import json
import re

PROJECT = "test-youtube-study-space"
REF = "refs/heads/integration/mypage-canon-20261006"
REPOSITORY = "kani3camp/youtube-study-space"
CALLER = REPOSITORY + "/.github/workflows/ci.yml@" + REF
REUSABLE = REPOSITORY + "/.github/workflows/mypage-foundation-authenticated.yml@" + REF
STATE_KEY = "youtube-study-space/mypage-platform/dev/terraform.tfstate"
WORKSPACE_PREFIX = "youtube-study-space/mypage-platform/dev/workspaces"
EMAIL = "mypage-api@" + PROJECT + ".iam.gserviceaccount.com"
SECRETS = ("oauth_client_id", "oauth_client_secret", "youtube_api_key")
SECRET_IDS = dict(zip(SECRETS, ("mypage-oauth-client-id", "mypage-oauth-client-secret", "mypage-youtube-api-key")))
CLAIM_KEYS = ("repository_id", "repository_owner_id", "environment", "ref", "workflow_ref", "job_workflow_ref", "event_name")


class ContractError(ValueError):
    """Only fixed source-defined check labels; never include rejected values."""


def need(value, check="contract"):
    if not value:
        raise ContractError(check)


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False).encode()


def subject(kind):
    need(kind in {"plan", "apply"})
    values = ("340900071", "54093651", "mypage-dev-" + kind, REF, CALLER, REUSABLE, "workflow_dispatch")
    return ":".join(key + ":" + value.replace(":", "%3A") for key, value in zip(CLAIM_KEYS, values))


def trust_policy(account, kind):
    need(re.fullmatch(r"[0-9]{12}", account))
    return {"Version": "2012-10-17", "Statement": [{
        "Effect": "Allow", "Action": "sts:AssumeRoleWithWebIdentity",
        "Principal": {"Federated": f"arn:aws:iam::{account}:oidc-provider/token.actions.githubusercontent.com"},
        "Condition": {"StringEquals": {"token.actions.githubusercontent.com:aud": "sts.amazonaws.com",
                                        "token.actions.githubusercontent.com:sub": subject(kind),
                                        # Enforce the immutable actor at STS too; a
                                        # workflow-local check alone is mutable.
                                        "token.actions.githubusercontent.com:actor_id": "54093651"}},
    }]}


def backend_policy(bucket, kind):
    need(kind in {"plan", "apply"} and re.fullmatch(r"[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]", bucket))
    arn = "arn:aws:s3:::" + bucket
    return {"Version": "2012-10-17", "Statement": [
        {"Sid": "MyPageDefaultState", "Effect": "Allow", "Action": ["s3:GetObject"] + (["s3:PutObject"] if kind == "apply" else []), "Resource": arn + "/" + STATE_KEY},
        {"Sid": "MyPageNativeLock", "Effect": "Allow", "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"], "Resource": arn + "/" + STATE_KEY + ".tflock"},
        {"Sid": "MyPageDiscovery", "Effect": "Allow", "Action": "s3:ListBucket", "Resource": arn,
         "Condition": {"StringEquals": {"s3:prefix": [STATE_KEY, STATE_KEY + ".tflock", WORKSPACE_PREFIX + "/"]}}},
    ]}


def stage_for_mode(mode):
    need(mode in {"foundation-plan", "foundation-apply", "foundation-metadata-plan", "foundation-metadata-apply", "registry-plan", "registry-apply"})
    if mode.startswith("registry-"):
        return "registry"
    return "foundation_metadata" if mode.startswith("foundation-metadata-") else "foundation"


def context(env, kind):
    need(kind in {"plan", "apply"})
    expected = {"GITHUB_REPOSITORY": REPOSITORY, "GITHUB_REPOSITORY_ID": "340900071",
                "GITHUB_REPOSITORY_OWNER_ID": "54093651", "GITHUB_EVENT_NAME": "workflow_dispatch",
                "GITHUB_ACTOR": "kani3camp", "GITHUB_ACTOR_ID": "54093651",
                "GITHUB_TRIGGERING_ACTOR": "kani3camp", "GITHUB_RUN_ATTEMPT": "1",
                "GITHUB_REF": REF, "GITHUB_WORKFLOW_REF": CALLER, "TF_WORKSPACE": "default"}
    need(all(env.get(k) == v for k, v in expected.items()))
    sha = env.get("GITHUB_SHA", "")
    need(re.fullmatch(r"[a-f0-9]{40}", sha) and env.get("MYPAGE_EXPECTED_SHA") == sha)
    stage_for_mode(env.get("MYPAGE_MODE"))
    need(kind != "apply" or env["MYPAGE_MODE"].endswith("-apply"))


def resource_contract(stage="foundation"):
    need(stage in {"foundation_metadata", "foundation", "registry"})
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
            "project": PROJECT, "location": "asia-southeast2", "repository_id": "mypage",
            "description": "YouTube Study Space MyPage development images",
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


def validate_empty_metadata_refresh(plan):
    """Admit only observed null-to-empty Secret maps on planned no-op resources.

    Google 8.5.0 refreshes these unset maps from null to {} after initial
    creation. Keep the full native drift in the HMAC; do not erase it or admit
    an actual metadata change, another resource, or a Terraform write.
    """
    secrets = {"module.platform[0].google_secret_manager_secret." + name for name in SECRET_IDS}
    planned = {item.get("address"): item for item in plan.get("resource_changes", [])}
    seen = set()
    for item in plan.get("resource_drift", []) or []:
        address = item.get("address")
        need(address in secrets and address not in seen, "metadata-refresh-address")
        seen.add(address)
        need(item.get("mode") == "managed" and item.get("provider_name") == "registry.terraform.io/hashicorp/google", "metadata-refresh-provider")
        change = item["change"]
        need(change.get("actions") == ["update"] and not change.get("importing") and not change.get("replace_paths"), "metadata-refresh-action")
        need(not has_unknown(change.get("after_unknown")), "metadata-refresh-unknown")
        before, after = change.get("before"), change.get("after")
        need(type(before) is dict and type(after) is dict and set(before) == set(after), "metadata-refresh-shape")
        changed = {key for key in before if before[key] != after[key]}
        need(bool(changed) and changed <= {"annotations", "labels", "version_aliases"}, "metadata-refresh-fields")
        need(all(before[key] is None and type(after[key]) is dict and not after[key] for key in changed), "metadata-refresh-empty-only")
        need(planned.get(address, {}).get("change", {}).get("actions") == ["no-op"], "metadata-refresh-planned-noop")


def validate_plan(plan, stage="foundation"):
    need(plan.get("format_version") == "1.2", "plan-format")
    need(plan.get("complete") is True, "plan-complete")
    need(plan.get("errored") is False, "plan-errored")
    validate_empty_metadata_refresh(plan)
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
                # Auto replication only; provider versions may omit empty lists.
                replication = after.get(key)
                need(type(replication) is list and len(replication) == 1, "secret-replication")
                need(not replication[0].get("user_managed") and len(replication[0].get("auto", [])) == 1, "secret-auto-replication")
                need(not replication[0]["auto"][0].get("customer_managed_encryption"), "secret-encryption")
            elif key == "secret_id" and address in {
                "module.platform[0].google_secret_manager_secret_iam_member." + name + "_runtime[0]"
                for name in SECRET_IDS
            }:
                # Google 8.5.0 stores the IAM resource's Secret identifier as
                # a full resource name after create/refresh. Accept only the
                # exact same project and Secret, never an arbitrary suffix.
                need(after.get(key) in {value, "projects/" + PROJECT + "/secrets/" + value}, "resource-value-secret_id")
            else:
                # key is from the fixed resource contract, never the Plan.
                need(after.get(key) == value, "resource-value-" + key)
            need(not has_unknown(change.get("after_unknown", {}).get(key)), "resource-unknown-" + key)
        if address.endswith("google_service_account.runtime"):
            need(after.get("disabled") in {None, False}, "runtime-disabled")
    need(seen == set(contract), "resource-set")
    return {"resources": len(seen), "creates": sum(x["change"]["actions"] == ["create"] for x in plan["resource_changes"])}


def commitment(plan, state_version, sha, key):
    need(type(key) is str and len(key) >= 32)
    # Terraform lists dependency sources as an unordered relevant_attributes
    # collection. Repeated native plans can emit it in a different order.
    # Preserve every entry, its contents and multiplicity; other arrays retain
    # their ordering (in particular action sequences and attribute paths).
    stable = {k: v for k, v in plan.items() if k != "timestamp"}
    if "relevant_attributes" in stable:
        need(type(stable["relevant_attributes"]) is list, "plan-relevant-attributes")
        stable["relevant_attributes"] = sorted(stable["relevant_attributes"], key=canonical)
    return hmac.new(key.encode(), canonical({"plan": stable, "state_version": state_version, "source": sha}), hashlib.sha256).hexdigest()
