#!/usr/bin/env python3
"""MyPage基盤のTerraform Planを作り、検査し、承認後に適用するための実行役。元のクラウド出力は非公開のまま保つ。

目的: 検査を通ったPlanだけを同じ内容のまま適用し、公開する要約には値や名前を出さない。未知のリソースや削除が紛れ込む事故を防ぐ。
使われる場面: MyPage Foundation Authenticated ワークフロー(.github/workflows/mypage-foundation-authenticated.yml)の preflight・oidc・execute・apply・cleanup の各ステップ。image_ops.py や read_metrics_live.py からも部品として読み込まれる。
失敗したとき: 基盤のPlanまたはApplyが中断される。自動の再試行はしない。
なくした場合: MyPage基盤のPlan・Applyワークフローが動かなくなる。
"""
from __future__ import annotations

import base64
from collections import Counter
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import time
import urllib.error
import urllib.request

import foundation_contract as contract
import runtime_contract

REPO = Path(__file__).resolve().parents[3]
PHASE = "context"
# オーナーの公開invokerを取り込むときだけ、prepareが書くJSON var-file。
PUBLIC_INVOKER_FILE = "public-invoker.json"


def need(value):
    contract.need(value)


def run(args, *, cwd=None, codes=(0,), check="native-command"):
    result = subprocess.run(args, cwd=cwd, capture_output=True, text=True, timeout=600)
    if result.returncode not in codes and check in {"metrics-refresh-show", "metrics-refresh-apply", "metrics-final-state-show", "state-version-list", "state-version-head", "saved-plan-show"}:
        # 固定の分類で失敗したネイティブ処理を特定する。
        # 元のPlan/State・引数・パス・認証情報・APIエラーは出力しない。
        diagnostics = result.stdout + result.stderr
        for marker, label in (
            ("Saved plan is stale", "saved-plan-stale"),
            ("Saved plan does not match", "saved-plan-state-mismatch"),
            ("Cannot apply non-applyable plan", "non-applyable-plan"),
            ("Cannot apply incomplete plan", "incomplete-plan"),
            ("Cannot apply failed plan", "failed-plan"),
            ("Error acquiring the state lock", "state-lock"),
            ("Failed to save state", "state-save"),
            ("Error saving state", "state-save"),
            ("Inconsistent dependency lock file", "provider-lock"),
            ("Failed to load plugin schemas", "provider-schema"),
            ("Invalid provider configuration", "provider-configuration"),
            ("No state file was found", "state-missing"),
            ("invalid_grant", "identity-expired"),
            ("ExpiredToken", "identity-expired"),
            ("AccessDenied", "access-denied"),
        ):
            if marker in diagnostics:
                check += "-" + label
                break
    if result.returncode not in codes and check == "terraform-plan":
        # ソースで定義したラベルだけを公開する。
        # ネイティブ診断、コマンド引数、拒否データから取り出した権限名は出力しない。
        diagnostics = result.stdout + result.stderr
        for permission in (
            "artifactregistry.repositories.get", "artifactregistry.repositories.getIamPolicy", "iam.roles.get", "iam.serviceAccounts.get", "iam.serviceAccounts.getIamPolicy",
            "iam.workloadIdentityPoolProviders.get", "iam.workloadIdentityPools.get",
            "iam.workloadIdentityPools.getAttestationRules",
            "resourcemanager.projects.get", "resourcemanager.projects.getIamPolicy",
            "run.jobs.get", "run.jobs.getIamPolicy", "run.services.get", "run.services.getIamPolicy", "run.services.list",
            "secretmanager.secrets.get", "secretmanager.secrets.getIamPolicy",
            "serviceusage.services.get", "serviceusage.services.use",
        ):
            # IAM APIは権限名を「iam.googleapis.com/workloadIdentityPools.get」の形でも返す。
            service, rest = permission.split(".", 1)
            if re.search(r"Permission ['\"](?:" + re.escape(permission) + "|" + re.escape(service + ".googleapis.com/" + rest) +
                         r")['\"] denied", diagnostics):
                check = "terraform-plan-permission-" + permission
                break
        else:
            for marker, label in (
                ("SERVICE_DISABLED", "terraform-plan-api-disabled"),
                ("Error acquiring the state lock", "terraform-plan-state-lock"),
                ("Error: Invalid value for variable", "terraform-plan-variable-validation"),
                ("Error: Invalid function argument", "terraform-plan-function-argument"),
                ("Error: Invalid provider configuration", "terraform-plan-provider-configuration"),
                ("Error: Unsupported argument", "terraform-plan-unsupported-argument"),
            ):
                if marker in diagnostics:
                    check = label
                    break
        describe_plan_errors(diagnostics)
    contract.need(result.returncode in codes, check)
    return result.stdout


PUBLIC_HTTP_STATUS = {"400", "401", "403", "404", "409", "412", "429", "500", "503"}
DIAGNOSTIC_HEADER = re.compile(r"^(?:[\u2577\u2502\u2575] ?)?(Error|Warning): ", re.MULTILINE)
# Terraformの位置情報は「with <address>,」の直後に「on <file> line <n>」が続く。
DIAGNOSTIC_ADDRESS = re.compile(r"^(?:\u2502)?[ ]*with (\S+),\n(?:\u2502)?[ ]*on \S+ line [0-9]+", re.MULTILINE)


def defined_addresses():
    """Repository-defined MyPage module addresses; nothing taken from native output is trusted.

    MyPage resources use count 0/1 at most, so only the bare address and index [0] are public.
    """
    names = set()
    for path in sorted((REPO / "infra/mypage-platform/modules").rglob("*.tf")):
        for kind, name in re.findall(r'^resource "([a-z0-9_]+)" "([a-z0-9_]+)"', path.read_text(), re.MULTILINE):
            base = contract.MODULE + "." + kind + "." + name
            names |= {base, base + "[0]"}
    return names


def describe_plan_errors(diagnostics):
    # ネイティブ診断の本文は出さない。エラーごとに、リポジトリで定義した資源のaddressと
    # 既知のHTTP statusだけを出す（どの資源の読取りで止まったかの切り分け用）。
    # -no-colorでは枠なし、TTYでは枠付きになるため、どちらも診断の見出しで区切る。
    known = defined_addresses()
    headers = list(DIAGNOSTIC_HEADER.finditer(diagnostics))
    entries = []
    for index, header in enumerate(headers):
        if header.group(1) != "Error":
            continue
        block = diagnostics[header.start():headers[index + 1].start() if index + 1 < len(headers) else len(diagnostics)]
        summary = block.split("\n", 1)[0]
        status = re.search(r"\bError (\d{3})\b", summary)
        located = [match.group(1) for match in DIAGNOSTIC_ADDRESS.finditer(block)]
        # 出せるのはリポジトリ定義の文字列だけ。位置情報が一意に既知の資源を指すときに限る。
        address = located[-1] if located and located[-1] in known else None
        entries.append({
            "address": address or "<withheld>",
            "status": status.group(1) if status and status.group(1) in PUBLIC_HTTP_STATUS else "<withheld>",
        })
    print("Terraform plan errors: " + json.dumps(entries[:20], sort_keys=True) + ".")


def request(url, token):
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
    with urllib.request.urlopen(req, timeout=30) as response:
        return json.load(response)


def directory(env):
    parent = Path(env["RUNNER_TEMP"]).resolve()
    path = parent / "mypage-foundation-private"
    need(not path.is_symlink() and not path.resolve().is_relative_to(REPO))
    return path


def preflight(env):
    kind = env["MYPAGE_ROLE"]
    contract.context(env, kind)
    need(re.fullmatch(r"[0-9]{12}", env.get("MYPAGE_STATE_ACCOUNT", "")))
    bucket = env.get("MYPAGE_STATE_BUCKET", "")
    contract.backend_policy(bucket, kind)
    need(env.get("MYPAGE_BACKEND_ROLE_ARN") == "arn:aws:iam::" + env["MYPAGE_STATE_ACCOUNT"] + ":role/youtube-study-space-" + contract.GITHUB_ENVIRONMENT + "-" + kind)
    need(re.fullmatch(r"AROA[A-Z0-9]{17}", env.get("MYPAGE_BACKEND_ROLE_ID", "")))
    expected_sa = "mypage-foundation-" + kind + "@" + contract.PROJECT + ".iam.gserviceaccount.com"
    need(env.get("MYPAGE_GCP_SERVICE_ACCOUNT") == expected_sa)
    need(re.fullmatch(r"projects/[0-9]+/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions",
                      env.get("MYPAGE_GCP_WIF_PROVIDER", "")))
    need(len(env.get("MYPAGE_PLAN_HMAC_KEY", "")) >= 32)
    root = directory(env)
    need(not root.exists())
    root.mkdir(mode=0o700)
    shutil.copytree(REPO / "infra/mypage-platform", root / "source", ignore=shutil.ignore_patterns(".terraform", "*.tfstate*", "backend.hcl", "terraform.tfvars", "*.tfplan"))
    backend = {"bucket": bucket, "key": contract.STATE_KEY, "region": "ap-northeast-1",
               "encrypt": True, "use_lockfile": True, "workspace_key_prefix": contract.WORKSPACE_PREFIX,
               "allowed_account_ids": [env["MYPAGE_STATE_ACCOUNT"]]}
    (root / "backend.json").write_bytes(contract.canonical(backend))
    value = runtime_settings(env)
    if value is not None:
        (root / "runtime-settings.json").write_bytes(contract.canonical({"runtime_settings": value}))
    print("Environment-bound foundation preflight passed; no cloud credentials requested yet.")


def oidc(env, *, image=False):
    kind = env["MYPAGE_ROLE"]
    contract.context(env, kind, image=image)
    url = env["ACTIONS_ID_TOKEN_REQUEST_URL"] + "&audience=sts.amazonaws.com"
    token = request(url, env["ACTIONS_ID_TOKEN_REQUEST_TOKEN"])["value"]
    encoded = token.split(".")[1]
    claims = json.loads(base64.urlsafe_b64decode(encoded + "=" * (-len(encoded) % 4)))
    if contract.is_pr(env):
        ref = env["GITHUB_REF"]
        workflow = contract.WORKFLOWS + "terraform.yml@" + ref
        reusable = contract.WORKFLOWS + contract.REUSABLE_NAME + "@" + ref
        sub = contract.pr_subject(env["MYPAGE_PR_NUMBER"])
    else:
        ref, workflow, reusable = contract.REF, contract.caller(env, image=image), contract.REUSABLE
        sub = contract.subject(kind, env["GITHUB_EVENT_NAME"], workflow)
    exact = {"aud": "sts.amazonaws.com", "iss": "https://token.actions.githubusercontent.com",
             "sub": sub, "repository_id": "340900071", "repository_owner_id": "54093651",
             "ref": ref, "workflow_ref": workflow, "job_workflow_ref": reusable,
             "workflow_sha": env["GITHUB_SHA"], "job_workflow_sha": env["GITHUB_SHA"],
             "actor_id": "54093651", "actor": "kani3camp", "run_attempt": env["GITHUB_RUN_ATTEMPT"],
             "environment": contract.GITHUB_ENVIRONMENT + "-" + kind,
             "event_name": env["GITHUB_EVENT_NAME"]}
    need(all(claims.get(k) == v for k, v in exact.items()))
    print("Exact environment-bound OIDC claims verified; AWS/GCP still validate the signed token.")


def state_version(env):
    common = ["--bucket", env["MYPAGE_STATE_BUCKET"], "--expected-bucket-owner", env["MYPAGE_STATE_ACCOUNT"], "--output", "json"]
    listing = json.loads(run(["aws", "s3api", "list-objects-v2", *common, "--prefix", contract.STATE_KEY, "--max-keys", "2", "--no-paginate"], check="state-version-list"))
    need(listing.get("IsTruncated") is False and listing.get("Prefix") == contract.STATE_KEY)
    entries = [item for item in listing.get("Contents", []) if item["Key"] == contract.STATE_KEY]
    if not entries:
        return "absent"
    need(len(entries) == 1)
    metadata = json.loads(run(["aws", "s3api", "head-object", *common, "--key", contract.STATE_KEY], check="state-version-head"))
    need(metadata.get("VersionId") not in {None, "null", ""} and metadata.get("ServerSideEncryption") == "AES256")
    return {k: metadata[k] for k in ("VersionId", "ETag", "ContentLength")}


def reject_unowned_resources(env, plan):
    # このState外の同名資源を、新規作成で取り込まない。
    urls = {contract.MODULE + ".google_service_account.runtime":
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/serviceAccounts/" + contract.EMAIL}
    urls.update({contract.MODULE + ".google_secret_manager_secret." + name:
                 "https://secretmanager.googleapis.com/v1/projects/" + contract.PROJECT + "/secrets/" + secret_id
                 for name, secret_id in contract.SECRET_IDS.items()})
    urls[contract.MODULE + ".google_artifact_registry_repository.mypage"] = (
        "https://artifactregistry.googleapis.com/v1/projects/" + contract.PROJECT +
        "/locations/" + contract.REGION + "/repositories/mypage")
    urls[runtime_contract.SERVICE] = (
        "https://run.googleapis.com/v2/projects/" + contract.PROJECT + "/locations/" + contract.REGION + "/services/mypage-api")
    urls.update({
        runtime_contract.MAINTENANCE_OPERATOR_SA:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/serviceAccounts/mypage-maintenance-job@" + contract.PROJECT + ".iam.gserviceaccount.com",
        runtime_contract.MAINTENANCE_INVOKER_SA:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/serviceAccounts/mypage-maintenance-invoker@" + contract.PROJECT + ".iam.gserviceaccount.com",
        runtime_contract.MAINTENANCE_ROLE:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/roles/mypageMaintenanceOperator",
        runtime_contract.MAINTENANCE_IMAGE_ROLE:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/roles/mypageMaintenanceImageRelease",
        runtime_contract.MAINTENANCE_READ_ROLE:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/roles/mypageMaintenanceExecutionRead",
        runtime_contract.MAINTENANCE_POOL:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT_NUMBER + "/locations/global/workloadIdentityPools/mypage-maintenance",
        runtime_contract.MAINTENANCE_PROVIDER:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT_NUMBER + "/locations/global/workloadIdentityPools/mypage-maintenance/providers/github",
        runtime_contract.MAINTENANCE_JOB:
            "https://run.googleapis.com/v2/projects/" + contract.PROJECT + "/locations/" + contract.REGION + "/jobs/mypage-maintenance-operator",
        runtime_contract.BACKEND_RELEASE_ROLE:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/roles/mypageBackendRelease",
    })
    for name in runtime_contract.RELEASE_COMPONENTS:
        pool = ("https://iam.googleapis.com/v1/projects/" + contract.PROJECT_NUMBER +
                "/locations/global/workloadIdentityPools/" + contract.RELEASES[name]["pool"])
        urls[runtime_contract.RELEASE_SA[name]] = (
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/serviceAccounts/" + contract.release_account(name))
        urls[runtime_contract.RELEASE_POOL[name]] = pool
        urls[runtime_contract.RELEASE_PROVIDER[name]] = pool + "/providers/" + contract.RELEASE_PROVIDER
    for item in plan["resource_changes"]:
        if item["address"] not in urls or item["change"]["actions"] != ["create"]:
            continue
        try:
            request(urls[item["address"]], env["MYPAGE_GCP_ACCESS_TOKEN"])
        except urllib.error.HTTPError as error:
            contract.need(error.code == 404, "unowned-resource-lookup")
        else:
            contract.need(False, "unowned-resource-exists")


def runtime_settings(env):
    """Environmentの実行設定を返す。未設定（初回構築でSecret値・imageがまだ無い）ならNone。"""
    raw = env.get("MYPAGE_RUNTIME_SETTINGS_JSON", "")
    if raw == "":
        return None
    need(len(raw.encode()) <= 65536)
    def pairs(items):
        result = {}
        for key, value in items:
            need(key not in result)
            result[key] = value
        return result
    parsed = json.loads(raw, object_pairs_hook=pairs)
    # Environment secrets may retain these retired keys. Drop them before
    # validating or writing tfvars; all other unknown fields stay fail-closed.
    # The consent versions are source constants (system/core/mypage/policy.go).
    retired = {"maintenance_operator_enabled", "maintenance_operator_image_digest",
               "maintenance_operator_image_source_sha", "history_coverage_json",
               "privacy_version", "terms_version"}
    parsed = {key: item for key, item in parsed.items() if key not in retired}
    value = runtime_contract.settings(parsed)
    # After the first Terraform creation, CI/CD owns the Cloud Run image.
    # This fixed host remains Terraform-owned; no release JSON overlays the image.
    if contract.TARGET == "development":
        value = runtime_contract.settings(
            value | {"hosting_backend_host": "mypage-api-eu5mhkcewq-et.a.run.app",
                     "previous_image_digest": ""})
    provider = re.fullmatch(r"projects/([0-9]+)/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions",
                            env.get("MYPAGE_GCP_WIF_PROVIDER", ""))
    need(provider and provider[1] == contract.PROJECT_NUMBER)
    need(value["project_number"] == provider[1])
    return value


def service_exists(env):
    """固定名のCloud Run APIが実在するかを読み取りだけで確かめる（404以外の失敗は停止）。"""
    url = ("https://run.googleapis.com/v2/projects/" + contract.PROJECT + "/locations/" +
           contract.REGION + "/services/mypage-api")
    try:
        request(url, env["MYPAGE_GCP_ACCESS_TOKEN"])
    except urllib.error.HTTPError as error:
        contract.need(error.code == 404, "runtime-service-lookup")
        return False
    return True


def verify_runtime_inputs(env, value):
    # Terraform's initial image is an immutable bootstrap input, not the live
    # application release. Validate its provenance without requiring the current
    # HEAD's source tree to equal an older, already released image.
    prefix = "projects/" + contract.PROJECT + "/locations/" + contract.REGION + "/repositories/mypage/packages/mypage-api"
    source = value.get("image_source_sha") or env["GITHUB_SHA"]
    if source != env["GITHUB_SHA"]:
        contract.need(re.fullmatch(r"[a-f0-9]{40}", source), "runtime-image-source-sha")
        run(["git", "fetch", "--no-tags", "--depth=1", "origin", source], cwd=REPO, check="runtime-image-source-fetch")
        run(["git", "merge-base", "--is-ancestor", source, env["GITHUB_SHA"]],
            cwd=REPO, check="runtime-image-source-ancestor")
    tag = request("https://artifactregistry.googleapis.com/v1/" + prefix + "/tags/" + source, env["MYPAGE_GCP_ACCESS_TOKEN"])
    expected = prefix + "/versions/sha256:" + value["image_digest"].split("@sha256:")[1]
    contract.need(tag.get("version") == expected, "runtime-source-image-digest")
    for name, field in (("mypage-oauth-client-id", "oauth_client_id_version"),
                        ("mypage-oauth-client-secret", "oauth_client_secret_version"),
                        ("mypage-youtube-api-key", "youtube_api_key_version")):
        metadata = request("https://secretmanager.googleapis.com/v1/projects/" + contract.PROJECT + "/secrets/" + name + "/versions/" + value[field],
                           env["MYPAGE_GCP_ACCESS_TOKEN"])
        contract.need(metadata.get("state") == "ENABLED", "runtime-secret-version-enabled")


def verify_owner_public_invoker(env):
    url = "https://run.googleapis.com/v2/projects/" + contract.PROJECT + "/locations/" + contract.REGION + "/services/mypage-api:getIamPolicy?options.requestedPolicyVersion=3"
    policy = request(url, env["MYPAGE_GCP_ACCESS_TOKEN"])
    public = [b for b in policy.get("bindings", []) if "allUsers" in b.get("members", [])]
    contract.need(len(public) == 1 and public[0].get("role") == "roles/run.invoker"
                  and not public[0].get("condition"), "runtime-owner-public-invoker")
    contract.need(not any("allAuthenticatedUsers" in b.get("members", []) for b in policy.get("bindings", [])), "runtime-owner-public-invoker")


# Resource addresses are fixed by this repository's module (count indices only),
# so they are not private. Anything else is withheld.
PUBLIC_ADDRESS = re.compile(re.escape(contract.MODULE) + r"\.[a-z0-9_]+\.[a-z0-9_]+(?:\[[0-9]+\])?")
PUBLIC_ACTIONS = frozenset({"no-op", "create", "read", "update", "delete"})

# These two resources rely on lifecycle.prevent_destroy alone, which Terraform
# stops checking if their resource blocks are removed entirely from the source.
LIFECYCLE_PROTECTED = {
    contract.MODULE + ".google_service_account.runtime",
    contract.MODULE + ".google_artifact_registry_repository.mypage",
}


def public_entry(item):
    address = item.get("address")
    actions = item.get("change", {}).get("actions")
    return {"address": address if type(address) is str and PUBLIC_ADDRESS.fullmatch(address) else "<withheld>",
            "actions": actions if type(actions) is list and set(actions) <= PUBLIC_ACTIONS else ["<withheld>"]}


# Provider schema attribute names (never values, map keys or list contents).
PUBLIC_ATTRIBUTE = re.compile(r"[a-z][a-z0-9_]{0,63}")


# Enum-valued Terraform-side attributes whose values are fixed by the provider
# and safe to show; any other value is withheld.
PUBLIC_ENUM_ATTRIBUTES = {"deletion_policy": frozenset({"DELETE", "PREVENT", "ABANDON"})}


def public_enum_values(change):
    before, after = change.get("before"), change.get("after")
    if type(before) is not dict or type(after) is not dict:
        return {}
    shown = lambda key, value: value if value is None or value in PUBLIC_ENUM_ATTRIBUTES[key] else "<withheld>"
    return {key: {"before": shown(key, before.get(key)), "after": shown(key, after.get(key))}
            for key in sorted(PUBLIC_ENUM_ATTRIBUTES) if before.get(key) != after.get(key)}


def changed_attributes(change):
    """Top-level attribute names whose planned value differs; values are never read out."""
    before, after = change.get("before"), change.get("after")
    unknown = change.get("after_unknown") or {}
    if type(before) is not dict or type(after) is not dict or type(unknown) is not dict:
        return ["<withheld>"]
    names = {key for key in set(before) | set(after) if before.get(key) != after.get(key)}
    # Terraform serializes known nested structures as empty containers ([{}]);
    # only a nested True marks a value that is unknown until Apply.
    names |= {key for key, value in unknown.items() if contract.has_unknown(value)}
    return sorted(name if type(name) is str and PUBLIC_ATTRIBUTE.fullmatch(name) else "<withheld>" for name in names)


def describe_runtime_drift(plan):
    # 出力値・schema・field名を公開しない。詳細Planはprivateに保持する。
    # 契約違反の切り分けに必要な、固定の資源addressと操作名だけを出す。
    drift = plan.get("resource_drift", []) or []
    changes = plan.get("resource_changes", []) or []
    planned = {item.get("address"): item for item in changes}
    writes = [item for item in changes if item.get("change", {}).get("actions") != ["no-op"]]
    print("Terraform drift summary: " + json.dumps({"observedResources": len(drift),
          "plannedWrites": len(writes)}, sort_keys=True) + ".")
    print("Terraform drift detail: " + json.dumps([
        public_entry(item) | {"plannedActions": public_entry(planned.get(item.get("address"), {}))["actions"]}
        for item in drift], sort_keys=True) + ".")
    print("Terraform planned writes: " + json.dumps(sorted(
        (public_entry(item) for item in writes), key=lambda entry: entry["address"]), sort_keys=True) + ".")
    # In-place updates of existing resources: which top-level attributes change.
    print("Terraform planned update attributes: " + json.dumps(sorted(
        (public_entry(item) | {"attributes": changed_attributes(item.get("change", {})),
                               "enumValues": public_enum_values(item.get("change", {}))}
         for item in writes if item.get("change", {}).get("actions") == ["update"]),
        key=lambda entry: entry["address"]), sort_keys=True) + ".")


def plan_without_deletions(plan):
    """Keep normal validation intact; separately validate standalone State-owned deletions."""
    changes = plan.get("resource_changes", [])
    removed = [item for item in changes if "delete" in item.get("change", {}).get("actions", [])]
    if not removed:
        return plan, ()
    expected = runtime_contract.plan_resources(plan)
    prior = plan.get("prior_state", {}).get("values", {}).get("root_module", {})
    platform = [module for module in prior.get("child_modules", []) if module.get("address") == contract.MODULE]
    contract.need(len(platform) == 1, "deletion-state-module")
    resources = {item["address"]: item for item in platform[0].get("resources", [])}
    addresses = set()
    for item in removed:
        change = item["change"]
        contract.need(change.get("actions") == ["delete"] and not change.get("importing") and
                      not change.get("replace_paths") and not item.get("previous_address") and
                      not item.get("deposed"), "deletion-must-be-standalone")
        address = item.get("address")
        contract.need(type(address) is str and PUBLIC_ADDRESS.fullmatch(address) and
                      address not in expected and address not in addresses, "deletion-resource-address")
        contract.need(address not in LIFECYCLE_PROTECTED, "deletion-protected-resource")
        before = change.get("before")
        state = resources.get(address, {})
        provider = "registry.terraform.io/hashicorp/google"
        contract.need(item.get("mode") == state.get("mode") == "managed" and
                      item.get("provider_name") == state.get("provider_name") == provider and
                      type(before) is dict and state.get("values") == before and
                      change.get("after") is None and
                      not contract.has_unknown(change.get("after_unknown")), "deletion-state-resource")
        contract.need(before.get("deletion_protection") in (None, False) and
                      before.get("deletion_policy") in (None, "DELETE"), "deletion-protected-resource")
        addresses.add(address)
    remaining = [item for item in changes if item.get("address") not in addresses]
    contract.need(len(remaining) + len(addresses) == len(changes), "deletion-resource-address")
    return plan | {"resource_changes": remaining}, tuple(sorted(addresses))


def validate_plan(plan):
    checked, deletions = plan_without_deletions(plan)
    value, _ = runtime_contract.declared(plan)
    if deletions and value is not None:
        contract.need(not value["read_metrics_from_revision"], "deletion-metrics-transition")
    return runtime_contract.validate_plan(checked), deletions


def require_deletion_opt_in(env, deletions):
    if deletions and (env["MYPAGE_ROLE"] == "apply" or env.get("MYPAGE_OPERATION") == "apply"):
        contract.need(env.get("MYPAGE_ALLOW_DELETIONS") == "true", "enable-allow-mypage-deletions-to-apply")


def publish_plan_summary(env, plan, deletions=()):
    """固定資源種別・操作・件数と検証済みの削除アドレスをJob Summaryへ公開する。"""
    if not env.get("GITHUB_STEP_SUMMARY"):
        return
    # 種別は期待資源か検証済みの削除アドレスから求め、Planのtypeや設定値は使わない。
    types = {address: address.rsplit(".", 2)[-2] for address in (*runtime_contract.plan_resources(plan), *deletions)}
    actions = {("no-op",): "no-op", ("read",): "read", ("create",): "create", ("update",): "update",
               ("delete",): "delete", ("delete", "create"): "replace", ("create", "delete"): "replace"}
    grouped = Counter()
    counts = Counter()
    for item in plan.get("resource_changes", []):
        action = actions.get(tuple(item["change"]["actions"]))
        need(item["address"] in types and action is not None)
        grouped[types[item["address"]], action] += 1
        counts[action] += 1
        counts["import"] += int(isinstance(item["change"].get("importing"), dict))
    counts["drift"] = len(plan.get("resource_drift", []) or [])
    value, public = runtime_contract.declared(plan)
    lines = ["### MyPage Terraform Plan", "", f"Environment: `{contract.ENVIRONMENT}`",
             "Runtime: " + ("declared" if value is not None else "**not declared** (runtime settings not provided)"),
             "Public invoker: " + ("adopted from the owner binding" if public else "not declared"),
             "", "Plan contract: **PASS**", "",
             "| Action | Count |", "| --- | ---: |"]
    lines += [f"| {action} | {counts[action]} |" for action in
              ("create", "update", "delete", "replace", "read", "no-op", "import", "drift")]
    lines += ["", "### Terraform resource changes", ""]
    if grouped:
        lines += ["| Resource type | Actions | Count |", "| --- | --- | ---: |"]
        lines += [f"| `{kind}` | {action} | {count} |" for (kind, action), count in sorted(grouped.items())]
    else:
        lines += ["No resource changes."]
    if deletions:
        lines += ["", "### MyPage deletion targets", "", f"Standalone deletes: **{len(deletions)}**", ""]
        lines += [f"- `{address}`" for address in deletions]
    lines += ["", "Drift is an observation count; it does not imply a planned write."]
    with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as stream:
        stream.write("\n".join(lines) + "\n")


def prepare(env):
    global PHASE
    kind = env["MYPAGE_ROLE"]
    contract.context(env, kind)
    root = directory(env)
    need(root.is_dir() and not root.is_symlink())
    PHASE = "identity"
    identity = json.loads(run(["aws", "sts", "get-caller-identity", "--output", "json"]))
    need(identity["Account"] == env["MYPAGE_STATE_ACCOUNT"] and identity["UserId"].split(":")[0] == env["MYPAGE_BACKEND_ROLE_ID"])
    need(identity["Arn"].startswith("arn:aws:sts::" + env["MYPAGE_STATE_ACCOUNT"] + ":assumed-role/youtube-study-space-" + contract.GITHUB_ENVIRONMENT + "-" + kind + "/"))
    version = state_version(env)
    work = root / ("source/environments/" + contract.CONFIG["state"])
    PHASE = "init"
    expected_version = (REPO / "infra/mypage-platform/.terraform-version").read_text().strip()
    need(json.loads(run(["terraform", "version", "-json"]))["terraform_version"] == expected_version)
    run(["terraform", "init", "-input=false", "-lockfile=readonly", "-no-color", "-backend-config=" + str(root / "backend.json")], cwd=work)
    PHASE = "plan"
    saved = root / "foundation.tfplan"
    # PR Plan is advisory and never blocks a manual Plan or Apply; Apply always re-plans.
    lock = "-lock=false" if contract.is_pr(env) else "-lock-timeout=60s"
    arguments = ["terraform", "plan", "-input=false", "-no-color", lock, "-out=" + str(saved)]
    # 段階の選択値は無い。宣言する範囲は、Terraform外の前提が実在するかだけで決まる。
    # - runtime settings（番号固定Secret版・環境別image）があれば実行系を宣言する。
    # - 固定名のAPIが既に実在すれば、settingsとオーナーの公開invokerは必須（無ければ停止）。
    PHASE = "runtime-inputs"
    value = runtime_settings(env)
    settings_file = root / "runtime-settings.json"
    contract.need((json.loads(settings_file.read_text()) == {"runtime_settings": value}) if value is not None
                  else not settings_file.exists(), "runtime-staged-settings")
    service = service_exists(env)
    contract.need(value is not None or not service, "runtime-settings-required")
    if value is not None:
        verify_runtime_inputs(env, value)
        arguments.append("-var-file=" + str(settings_file))
    if service:
        verify_owner_public_invoker(env)
        (work / "owner-public-invoker-import.tf").write_text(runtime_contract.public_invoker_import_config())
        # CLIの-varはPlanに未変換の文字列として記録されるため、JSON var-fileで真偽値として渡す。
        public_file = root / PUBLIC_INVOKER_FILE
        public_file.write_bytes(contract.canonical({"public_invoker": True}))
        arguments.append("-var-file=" + str(public_file))
    PHASE = "terraform-plan"
    run(arguments, cwd=work, check="terraform-plan")
    plan = json.loads(run(["terraform", "show", "-json", str(saved)], cwd=work))
    PHASE = "plan-contract"
    # Planの宣言は、上で観測した事実（settingsの有無・Serviceの実在）と一致しなければならない。
    variables = plan.get("variables", {})
    declared = variables.get("runtime_settings", {}).get("value")
    if declared != value:
        # 値は出さず、レビュー済みフィールド名のうち一致しないものだけを示す。
        names = sorted(runtime_contract.FIELDS & (set(declared or {}) | set(value or {})))
        print("Declared runtime settings mismatch: " + json.dumps({
            "declared": declared is not None, "observed": value is not None,
            "fields": [name for name in names if (declared or {}).get(name) != (value or {}).get(name)]}) + ".")
    contract.need(declared == value, "plan-declared-runtime-settings")
    public = variables.get("public_invoker", {}).get("value")
    contract.need(public is service, "plan-declared-public-invoker-" + ("unset" if public is None else type(public).__name__))
    if value is not None:
        describe_runtime_drift(plan)
    summary, deletions = validate_plan(plan)
    reject_unowned_resources(env, plan)
    need(state_version(env) == version)
    if contract.is_pr(env):
        # No commitment, saved-plan record or timestamp leaves a PR Plan job;
        # only these value-free counts reach the log and the Job Summary.
        message = ("MyPage PR Plan (advisory) accepted by the plan contract: " + str(summary["resources"]) + " resources, " +
                   str(summary["creates"]) + " creates" + (f", {len(deletions)} deletes" if deletions else "") +
                   ". Nothing from this run can be applied.")
        print(message)
        if env.get("GITHUB_STEP_SUMMARY"):
            with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as stream:
                stream.write("### MyPage PR Plan\n\n" + message + "\n")
        return
    if kind == "plan":
        publish_plan_summary(env, plan, deletions)
    require_deletion_opt_in(env, deletions)
    digest = contract.commitment(plan, version, env["GITHUB_SHA"], env["MYPAGE_PLAN_HMAC_KEY"])
    (root / "prepared.json").write_bytes(contract.canonical({"sha256": hashlib.sha256(saved.read_bytes()).hexdigest(), "source": env["GITHUB_SHA"]}))
    if value is not None and value["read_metrics_from_revision"]:
        (root / "metrics-state-version.json").write_bytes(contract.canonical(version))
    with open(env["GITHUB_OUTPUT"], "a") as output:
        output.write("commitment=" + digest + "\nplan_time=" + str(time.time()) + "\n")
        output.write("has_deletions=" + ("true" if deletions else "false") + "\n")
    print("Environment-bound foundation plan accepted: " + str(summary["resources"]) + " resources, " + str(summary["creates"]) +
          " creates; accepted computed metadata refreshes: " + str(len(plan.get("resource_drift", []) or [])) + ". No apply.")


def apply_saved(env):
    global PHASE
    contract.context(env, "apply")
    need(env["MYPAGE_ROLE"] == "apply")
    age = time.time() - float(env["MYPAGE_PLAN_TIME"])
    need(0 <= age <= 1800)
    root = directory(env)
    saved = root / "foundation.tfplan"
    receipt = json.loads((root / "prepared.json").read_text())
    need(receipt == {"sha256": hashlib.sha256(saved.read_bytes()).hexdigest(), "source": env["GITHUB_SHA"]})
    work = root / ("source/environments/" + contract.CONFIG["state"])
    PHASE = "apply"
    plan = json.loads(run(["terraform", "show", "-json", str(saved)], cwd=work, check="saved-plan-show"))
    _, deletions = validate_plan(plan)
    require_deletion_opt_in(env, deletions)
    if deletions:
        contract.need(env.get("MYPAGE_DELETION_APPROVED") == "true", "deletion-approval-required")
    value, _ = runtime_contract.declared(plan)
    if value is not None:
        if value["read_metrics_from_revision"]:
            # プロバイダー8.5.0は更新時にetagを省略するため、更新Planは絶対に適用しない。
            # 条件付きAPI変更と、検証済みのrefresh-only Stateを使う。
            import read_metrics_live
            result = read_metrics_live.apply_saved(env, root, plan)
            print("DEV metrics State-only recovery and live/State verification passed." if result == "state-recovery" else
                  "DEV metrics conditional update and live/State verification passed.")
            return
    run(["terraform", "apply", "-input=false", "-no-color", str(saved)], cwd=work)
    PHASE = "readback"
    state = json.loads(run(["terraform", "show", "-json"], cwd=work))
    addresses = set()
    def walk(module):
        addresses.update(x["address"] for x in module.get("resources", []))
        for child in module.get("child_modules", []):
            walk(child)
    walk(state["values"]["root_module"])
    need(addresses == set(runtime_contract.plan_resources(plan)))
    need(state_version(env) != "absent")
    print("Environment-bound foundation saved Plan applied and expected State resources read back.")


def main():
    global PHASE
    os.umask(0o077)
    try:
        command = sys.argv[1]
        if command == "cleanup":
            path = directory(os.environ)
            if path.exists():
                shutil.rmtree(path)
        elif command == "preflight":
            preflight(os.environ)
        elif command == "oidc":
            oidc(os.environ)
        elif command == "execute":
            prepare(os.environ)
        elif command == "apply":
            apply_saved(os.environ)
        else:
            need(False)
        return 0
    except contract.ContractError as error:
        # ContractErrorにはソースで定義したラベルだけを含め、拒否したデータは含めない。
        print("MyPage foundation STOP at " + PHASE + "; check=" + str(error) + ".", file=sys.stderr)
        return 1
    except Exception:
        # TerraformやAPIの元の診断には非公開値が含まれ得る。
        print("MyPage foundation STOP at " + PHASE + "; private diagnostics suppressed.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
