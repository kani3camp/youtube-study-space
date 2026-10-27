#!/usr/bin/env python3
"""Run an environment-bound foundation plan; raw cloud evidence stays private."""
from __future__ import annotations

import base64
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


def need(value):
    contract.need(value)


def run(args, *, cwd=None, codes=(0,), check="native-command"):
    result = subprocess.run(args, cwd=cwd, capture_output=True, text=True, timeout=600)
    if result.returncode not in codes and check in {"metrics-refresh-show", "metrics-refresh-apply", "metrics-final-state-show", "state-version-list", "state-version-head", "saved-runtime-plan-show"}:
        # These fixed classifications identify the failing native step without
        # printing raw Plan/State, arguments, paths, credentials or API errors.
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
        # Publish only source-defined labels. Never print native diagnostics,
        # command arguments or a permission name extracted from rejected data.
        diagnostics = result.stdout + result.stderr
        for permission in (
            "artifactregistry.repositories.get", "iam.roles.get", "iam.serviceAccounts.get", "iam.serviceAccounts.getIamPolicy",
            "iam.workloadIdentityPoolProviders.get", "iam.workloadIdentityPools.get",
            "resourcemanager.projects.get", "resourcemanager.projects.getIamPolicy",
            "run.jobs.get", "run.services.get", "run.services.getIamPolicy", "run.services.list",
            "secretmanager.secrets.get", "secretmanager.secrets.getIamPolicy",
            "serviceusage.services.get", "serviceusage.services.use",
        ):
            if re.search(r"Permission ['\"]" + re.escape(permission) + r"['\"] denied", diagnostics):
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
    contract.need(result.returncode in codes, check)
    return result.stdout


def request(url, token):
    req = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
    with urllib.request.urlopen(req, timeout=30) as response:
        return json.load(response)


def directory(env):
    parent = Path(env["RUNNER_TEMP"]).resolve()
    path = parent / "mypage-foundation-private"
    need(not path.is_symlink() and not path.resolve().is_relative_to(REPO))
    return path


def stage(env):
    kind = env["MYPAGE_ROLE"]
    contract.context(env, kind)
    need(re.fullmatch(r"[0-9]{12}", env.get("MYPAGE_STATE_ACCOUNT", "")))
    bucket = env.get("MYPAGE_STATE_BUCKET", "")
    contract.backend_policy(bucket, kind)
    need(env.get("MYPAGE_BACKEND_ROLE_ARN") == "arn:aws:iam::" + env["MYPAGE_STATE_ACCOUNT"] + ":role/youtube-study-space-" + contract.GITHUB_ENVIRONMENT + "-" + kind)
    need(re.fullmatch(r"AROA[A-Z0-9]{17}", env.get("MYPAGE_BACKEND_ROLE_ID", "")))
    expected_sa = "mypage-foundation-" + kind + "@" + contract.PROJECT + ".iam.gserviceaccount.com"
    need(env.get("MYPAGE_GCP_SERVICE_ACCOUNT") == expected_sa)
    need(re.fullmatch(r"projects/[0-9]+/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions", env.get("MYPAGE_GCP_WIF_PROVIDER", "")))
    need(len(env.get("MYPAGE_PLAN_HMAC_KEY", "")) >= 32)
    root = directory(env)
    need(not root.exists())
    root.mkdir(mode=0o700)
    shutil.copytree(REPO / "infra/mypage-platform", root / "source", ignore=shutil.ignore_patterns(".terraform", "*.tfstate*", "backend.hcl", "terraform.tfvars", "*.tfplan"))
    backend = {"bucket": bucket, "key": contract.STATE_KEY, "region": "ap-northeast-1",
               "encrypt": True, "use_lockfile": True, "workspace_key_prefix": contract.WORKSPACE_PREFIX,
               "allowed_account_ids": [env["MYPAGE_STATE_ACCOUNT"]]}
    (root / "backend.json").write_bytes(contract.canonical(backend))
    stage_name = contract.stage_for_mode(env["MYPAGE_MODE"])
    if stage_name in set(runtime_contract.MODES.values()):
        value = runtime_settings(env, stage_name)
        (root / "runtime-settings.json").write_bytes(contract.canonical({"runtime_settings": value}))
        if stage_name == "runtime_public":
            (root / ("source/environments/" + contract.CONFIG["state"] + "/owner-public-invoker-import.tf")).write_text(runtime_contract.public_invoker_import_config())
    print("Environment-bound foundation preflight passed; no cloud credentials requested yet.")


def oidc(env, *, image=False):
    kind = env["MYPAGE_ROLE"]
    contract.context(env, kind, image=image)
    url = env["ACTIONS_ID_TOKEN_REQUEST_URL"] + "&audience=sts.amazonaws.com"
    token = request(url, env["ACTIONS_ID_TOKEN_REQUEST_TOKEN"])["value"]
    encoded = token.split(".")[1]
    claims = json.loads(base64.urlsafe_b64decode(encoded + "=" * (-len(encoded) % 4)))
    exact = {"aud": "sts.amazonaws.com", "iss": "https://token.actions.githubusercontent.com",
             "sub": contract.subject(kind), "repository_id": "340900071", "repository_owner_id": "54093651",
             "ref": contract.REF, "workflow_ref": contract.CALLER, "job_workflow_ref": contract.REUSABLE,
             "workflow_sha": env["GITHUB_SHA"], "job_workflow_sha": env["GITHUB_SHA"],
             "actor_id": "54093651", "actor": "kani3camp", "run_attempt": "1",
             "environment": contract.GITHUB_ENVIRONMENT + "-" + kind, "event_name": "workflow_dispatch"}
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
    # A matching name outside this State must never be adopted by a new create.
    urls = {"module.platform[0].google_service_account.runtime":
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT + "/serviceAccounts/" + contract.EMAIL}
    urls.update({"module.platform[0].google_secret_manager_secret." + name:
                 "https://secretmanager.googleapis.com/v1/projects/" + contract.PROJECT + "/secrets/" + secret_id
                 for name, secret_id in contract.SECRET_IDS.items()})
    urls["module.platform[0].google_artifact_registry_repository.mypage[0]"] = (
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
        runtime_contract.MAINTENANCE_POOL:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT_NUMBER + "/locations/global/workloadIdentityPools/mypage-maintenance",
        runtime_contract.MAINTENANCE_PROVIDER:
            "https://iam.googleapis.com/v1/projects/" + contract.PROJECT_NUMBER + "/locations/global/workloadIdentityPools/mypage-maintenance/providers/github",
        runtime_contract.MAINTENANCE_JOB:
            "https://run.googleapis.com/v2/projects/" + contract.PROJECT + "/locations/" + contract.REGION + "/jobs/mypage-maintenance-operator",
    })
    for item in plan["resource_changes"]:
        if item["address"] not in urls or item["change"]["actions"] != ["create"]:
            continue
        try:
            request(urls[item["address"]], env["MYPAGE_GCP_ACCESS_TOKEN"])
        except urllib.error.HTTPError as error:
            need(error.code == 404)
        else:
            need(False)


def runtime_settings(env, stage=None):
    raw = env.get("MYPAGE_RUNTIME_SETTINGS_JSON", "")
    need(0 < len(raw.encode()) <= 65536)
    def pairs(items):
        result = {}
        for key, value in items:
            need(key not in result)
            result[key] = value
        return result
    value = runtime_contract.settings(json.loads(raw, object_pairs_hook=pairs), stage)
    release_file = REPO / "infra/mypage-platform/environments/dev/runtime-release.json"
    if stage != "maintenance_operator" and contract.TARGET == "development" and release_file.exists():
        contract.need(release_file.is_file() and not release_file.is_symlink(), "runtime-release-file")
        release = json.loads(release_file.read_text(), object_pairs_hook=pairs)
        contract.need(type(release) is dict and set(release) == {"image_digest", "image_source_sha", "hosting_backend_host", "previous_image_digest"}, "runtime-release-fields")
        value = runtime_contract.settings(value | release, stage)
        contract.need(all(release.values()) and release["previous_image_digest"] != release["image_digest"], "runtime-release-values")
    provider = re.fullmatch(r"projects/([0-9]+)/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions",
                            env.get("MYPAGE_GCP_WIF_PROVIDER", ""))
    need(provider and provider[1] == contract.PROJECT_NUMBER)
    if stage != "maintenance_operator":
        need(value["project_number"] == provider[1])
    return value


def verify_runtime_inputs(env, value, stage):
    # Infrastructure-only changes may retain a reviewed image. Its immutable
    # build commit must have the identical full Docker context, including its
    # Dockerfile, dependencies, ignore rules and source. Never retag an image.
    prefix = "projects/" + contract.PROJECT + "/locations/" + contract.REGION + "/repositories/mypage/packages/mypage-api"
    if stage != "maintenance_operator":
        source = value.get("image_source_sha") or env["GITHUB_SHA"]
        if source != env["GITHUB_SHA"]:
            contract.need(re.fullmatch(r"[a-f0-9]{40}", source), "runtime-image-source-sha")
            run(["git", "fetch", "--no-tags", "--depth=1", "origin", source], cwd=REPO, check="runtime-image-source-fetch")
            current_tree = run(["git", "rev-parse", env["GITHUB_SHA"] + ":system"], cwd=REPO).strip()
            image_tree = run(["git", "rev-parse", source + ":system"], cwd=REPO).strip()
            contract.need(re.fullmatch(r"[a-f0-9]{40}", current_tree) and current_tree == image_tree,
                          "runtime-image-source-tree")
        tag = request("https://artifactregistry.googleapis.com/v1/" + prefix + "/tags/" + source, env["MYPAGE_GCP_ACCESS_TOKEN"])
        expected = prefix + "/versions/sha256:" + value["image_digest"].split("@sha256:")[1]
        contract.need(tag.get("version") == expected, "runtime-source-image-digest")
    operator_source = value["maintenance_operator_image_source_sha"]
    if operator_source != env["GITHUB_SHA"]:
        run(["git", "fetch", "--no-tags", "--depth=1", "origin", operator_source], cwd=REPO, check="maintenance-operator-source-fetch")
        run(["git", "merge-base", "--is-ancestor", operator_source, env["GITHUB_SHA"]], cwd=REPO, check="maintenance-operator-source-ancestor")
    operator_tag = request("https://artifactregistry.googleapis.com/v1/" + prefix + "/tags/" + operator_source, env["MYPAGE_GCP_ACCESS_TOKEN"])
    operator_expected = prefix + "/versions/sha256:" + value["maintenance_operator_image_digest"].split("@sha256:")[1]
    contract.need(operator_tag.get("version") == operator_expected, "maintenance-operator-source-image-digest")
    if stage != "maintenance_operator":
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


def describe_runtime_drift(plan):
    """Publish only fixed schema labels, never State/Plan values or paths."""
    # Fixed schema labels from google provider 8.5.0; no rejected names.
    fields = set(["name","location","project","template","annotations","containers","image","args","base_image_uri","command","depends_on","env","liveness_probe","failure_threshold","grpc","port","service","http_get","http_headers","value","path","initial_delay_seconds","period_seconds","tcp_socket","timeout_seconds","ports","container_port","readiness_probe","success_threshold","resources","cpu_idle","limits","startup_cpu_boost","sandbox_launcher","startup_probe","volume_mounts","mount_path","sub_path","working_dir","build_info","function_target","source_location","encryption_key","execution_environment","gpu_zonal_redundancy_disabled","health_check_disabled","labels","max_instance_request_concurrency","node_selector","accelerator","revision","sandboxes","templates","scaling","concurrency_utilization","cpu_utilization","max_instance_count","min_instance_count","service_account","session_affinity","timeout","volumes","cloud_sql_instance","instances","empty_dir","medium","size_limit","gcs","bucket","mount_options","read_only","nfs","server","secret","default_mode","items","mode","version","vpc_access","connector","egress","network_interfaces","network","subnetwork","tags","workload_identity_config","identity","identity_certificate_enabled","identity_type","binary_authorization","breakglass_justification","policy","use_default","build_config","base_image","enable_automatic_updates","environment_variables","image_uri","worker_pool","client","client_version","custom_audiences","default_uri_disabled","description","iap_enabled","ingress","invoker_iam_disabled","launch_stage","multi_region_settings","regions","multi_region_id","manual_instance_count","scaling_mode","ssh_enabled","traffic","percent","tag","type","conditions","execution_reason","last_transition_time","message","reason","revision_reason","severity","state","create_time","creator","delete_time","effective_annotations","effective_labels","etag","expire_time","generation","last_modifier","latest_created_revision","latest_ready_revision","observed_generation","reconciling","terminal_condition","terraform_labels","traffic_statuses","uri","uid","update_time","urls","deletion_protection","deletion_policy","value_source","secret_key_ref"])
    planned = {x.get("address"): x for x in plan.get("resource_changes", [])}
    for item in plan.get("resource_drift", []) or []:
        if item.get("address") != runtime_contract.SERVICE:
            continue
        change = item.get("change", {})
        before, after = change.get("before"), change.get("after")
        if type(before) is not dict or type(after) is not dict:
            print("Runtime service drift diagnostic: unavailable.")
            continue
        changed = {k for k in set(before) | set(after) if before.get(k) != after.get(k) or (k in before) != (k in after)}
        labels = sorted(changed & fields)
        def shape(v):
            if v is None: return "null"
            if type(v) is dict: return "empty-object" if not v else "object"
            if type(v) is list: return "empty-array" if not v else "array"
            return {str: "string", bool: "boolean", int: "number", float: "number"}.get(type(v), "other")
        shapes = []
        def walk(a, b, path):
            if a == b or len(shapes) >= 64: return
            if type(a) is dict and type(b) is dict and not path.endswith(("annotations", "labels", "limits")):
                for k in sorted((set(a) | set(b)) & fields):
                    walk(a.get(k), b.get(k), (path + "." if path else "") + k)
            elif type(a) is list and type(b) is list and len(a) == len(b) and len(a) <= 32:
                for av, bv in zip(a, b): walk(av, bv, path + "[]")
            else: shapes.append({"field": path, "before": shape(a), "after": shape(b)})
        walk(before, after, "")
        noop = planned.get(runtime_contract.SERVICE, {}).get("change", {}).get("actions") == ["no-op"]
        print("Runtime service drift diagnostic: " + json.dumps({"fields": labels, "otherFields": len(changed - fields), "plannedNoop": noop, "shapes": shapes}, sort_keys=True) + ".")


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
    stage = contract.stage_for_mode(env["MYPAGE_MODE"])
    arguments = ["terraform", "plan", "-input=false", "-no-color", "-lock-timeout=60s", "-var=stage=" + stage, "-out=" + str(saved)]
    value = None
    if stage in set(runtime_contract.MODES.values()):
        PHASE = "runtime-inputs"
        value = runtime_settings(env, stage)
        contract.need(json.loads((root / "runtime-settings.json").read_text()) == {"runtime_settings": value}, "runtime-staged-settings")
        verify_runtime_inputs(env, value, stage)
        if stage in {"maintenance_upgrade", "runtime_public"}:
            verify_owner_public_invoker(env)
        arguments.append("-var-file=" + str(root / "runtime-settings.json"))
    PHASE = "terraform-plan"
    run(arguments, cwd=work, check="terraform-plan")
    plan = json.loads(run(["terraform", "show", "-json", str(saved)], cwd=work))
    PHASE = "plan-contract"
    if value is not None:
        need(plan.get("variables", {}).get("runtime_settings", {}).get("value") == value)
        describe_runtime_drift(plan)
        summary = runtime_contract.validate_plan(plan, stage)
    else:
        summary = contract.validate_plan(plan, stage)
    reject_unowned_resources(env, plan)
    need(state_version(env) == version)
    digest = contract.commitment(plan, version, env["GITHUB_SHA"], env["MYPAGE_PLAN_HMAC_KEY"])
    (root / "prepared.json").write_bytes(contract.canonical({"sha256": hashlib.sha256(saved.read_bytes()).hexdigest(), "source": env["GITHUB_SHA"]}))
    if value is not None and value["read_metrics_from_revision"]:
        (root / "metrics-state-version.json").write_bytes(contract.canonical(version))
    with open(env["GITHUB_OUTPUT"], "a") as output:
        output.write("commitment=" + digest + "\nplan_time=" + str(time.time()) + "\n")
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
    stage = contract.stage_for_mode(env["MYPAGE_MODE"])
    if stage in set(runtime_contract.MODES.values()):
        plan = json.loads(run(["terraform", "show", "-json", str(saved)], cwd=work, check="saved-runtime-plan-show"))
        value = runtime_contract.settings(plan.get("variables", {}).get("runtime_settings", {}).get("value"), stage)
        if value["read_metrics_from_revision"]:
            # Provider 8.5.0 omits etag on Update. NEVER apply its update Plan:
            # use a conditional API mutation and validated refresh-only State.
            import read_metrics_live
            result = read_metrics_live.apply_saved(env, root, plan, stage)
            print("DEV metrics State-only recovery and live/State verification passed." if result == "state-recovery" else
                  "DEV metrics conditional update and live/State verification passed.")
            return
        runtime_contract.validate_plan(plan, stage)
    run(["terraform", "apply", "-input=false", "-no-color", str(saved)], cwd=work)
    PHASE = "readback"
    state = json.loads(run(["terraform", "show", "-json"], cwd=work))
    addresses = set()
    def walk(module):
        addresses.update(x["address"] for x in module.get("resources", []))
        for child in module.get("child_modules", []):
            walk(child)
    walk(state["values"]["root_module"])
    stage = contract.stage_for_mode(env["MYPAGE_MODE"])
    resources = runtime_contract.resource_contract(stage) if stage in set(runtime_contract.MODES.values()) else contract.resource_contract(stage)
    need(addresses == set(resources))
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
        elif command == "stage":
            stage(os.environ)
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
        # ContractError contains only source-defined labels, not rejected data.
        print("MyPage foundation STOP at " + PHASE + "; check=" + str(error) + ".", file=sys.stderr)
        return 1
    except Exception:
        # Native Terraform/API diagnostics can contain private values.
        print("MyPage foundation STOP at " + PHASE + "; private diagnostics suppressed.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
