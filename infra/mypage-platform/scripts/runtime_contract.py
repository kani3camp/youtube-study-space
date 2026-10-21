"""DEV runtime admission; no credentials, cloud I/O or secret payloads."""
from __future__ import annotations

from copy import deepcopy
import json
import re

import foundation_contract as c

ORIGIN = "https://test--study-space.web.app"
REPOSITORY = "asia-southeast2-docker.pkg.dev/" + c.PROJECT + "/mypage/mypage-api"
SERVICE = "module.platform[0].google_cloud_run_v2_service.api[0]"
FIRESTORE = "module.platform[0].google_project_iam_member.firestore_user[0]"
INVOKER = "module.platform[0].google_cloud_run_v2_service_iam_member.public_invoker[0]"
MODES = {"runtime-private-plan": "runtime_private", "runtime-private-apply": "runtime_private",
         "runtime-public-plan": "runtime_public", "runtime-public-apply": "runtime_public"}
PUBLIC_INVOKER_NAME = "projects/" + c.PROJECT + "/locations/asia-southeast2/services/mypage-api"
PUBLIC_INVOKER_IMPORT_ID = PUBLIC_INVOKER_NAME + " roles/run.invoker allUsers"
FIELDS = {"image_digest", "project_number", "web_app_id", "public_origin", "privacy_version", "terms_version",
          "oauth_client_id_version", "oauth_client_secret_version", "youtube_api_key_version", "history_coverage_json", "image_source_sha", "hosting_backend_host", "previous_image_digest"}


def settings(value):
    c.need(type(value) is dict and set(value) <= FIELDS and set(value) >= FIELDS - {"history_coverage_json", "image_source_sha", "hosting_backend_host", "previous_image_digest"}, "runtime-settings-fields")
    value = value | {"history_coverage_json": value.get("history_coverage_json", ""), "image_source_sha": value.get("image_source_sha", ""), "hosting_backend_host": value.get("hosting_backend_host", ""), "previous_image_digest": value.get("previous_image_digest", "")}
    c.need(all(type(v) is str for v in value.values()), "runtime-settings-types")
    c.need(re.fullmatch(re.escape(REPOSITORY) + r"@sha256:[a-f0-9]{64}", value["image_digest"]), "runtime-image")
    c.need(re.fullmatch(r"[0-9]{6,20}", value["project_number"]), "runtime-project-number")
    c.need(re.fullmatch("1:" + value["project_number"] + r":web:[A-Za-z0-9]+", value["web_app_id"]), "runtime-app-project")
    c.need(value["public_origin"] == ORIGIN, "runtime-origin")
    for field in ("privacy_version", "terms_version"):
        c.need(re.fullmatch(r"[A-Za-z0-9._-]{1,64}", value[field]), "runtime-policy-version")
    for field in ("oauth_client_id_version", "oauth_client_secret_version", "youtube_api_key_version"):
        c.need(re.fullmatch(r"[1-9][0-9]*", value[field]), "runtime-secret-version")
    c.need(value["image_source_sha"] == "" or re.fullmatch(r"[a-f0-9]{40}", value["image_source_sha"]), "runtime-image-source-sha")
    c.need(value["hosting_backend_host"] == "" or re.fullmatch(r"mypage-api-[a-z0-9-]+\.a\.run\.app", value["hosting_backend_host"]), "runtime-hosting-backend")
    c.need(value["previous_image_digest"] == "" or re.fullmatch(re.escape(REPOSITORY) + r"@sha256:[a-f0-9]{64}", value["previous_image_digest"]), "runtime-previous-image")
    coverage = value["history_coverage_json"]
    c.need(len(coverage.encode()) <= 16384, "runtime-coverage-size")
    if coverage:
        c.need(type(json.loads(coverage)) is list, "runtime-coverage-array")
    return value


def resource_contract(stage):
    c.need(stage in {"runtime_private", "runtime_public"}, "runtime-stage")
    resources = c.resource_contract("registry") | {
        FIRESTORE: {"project": c.PROJECT, "role": "roles/datastore.user", "member": "serviceAccount:" + c.EMAIL},
        SERVICE: {"project": c.PROJECT, "name": "mypage-api", "location": "asia-southeast2",
                  "ingress": "INGRESS_TRAFFIC_ALL", "deletion_protection": True, "invoker_iam_disabled": False},
    }
    if stage == "runtime_public":
        resources[INVOKER] = {"project": c.PROJECT, "location": "asia-southeast2", "name": "mypage-api",
                              "role": "roles/run.invoker", "member": "allUsers"}
    return resources


def public_invoker_import_config():
    # Only this already approved member can be adopted. No input interpolation.
    return 'import {\n  to = ' + INVOKER + '\n  id = "' + PUBLIC_INVOKER_IMPORT_ID + '"\n}\n'


def plain_environment(value):
    env = {"GOOGLE_CLOUD_PROJECT": c.PROJECT, "MYPAGE_ENVIRONMENT": "development", "MYPAGE_REGION": "asia-southeast2",
           "MYPAGE_SIGNER_EMAIL": c.EMAIL, "MYPAGE_INFRASTRUCTURE_READY": "true"}
    for name, field in (("MYPAGE_PROJECT_NUMBER", "project_number"), ("MYPAGE_WEB_APP_ID", "web_app_id"),
                        ("MYPAGE_PUBLIC_ORIGIN", "public_origin"), ("MYPAGE_PRIVACY_VERSION", "privacy_version"),
                        ("MYPAGE_TERMS_VERSION", "terms_version")):
        env[name] = value[field]
    if value["history_coverage_json"]:
        env["MYPAGE_HISTORY_COVERAGE"] = value["history_coverage_json"]
    if value["hosting_backend_host"]:
        env["MYPAGE_HOSTING_BACKEND_HOST"] = value["hosting_backend_host"]
    return env


def one(value, check):
    c.need(type(value) is list and len(value) == 1 and type(value[0]) is dict, check)
    return value[0]


def empty_options(value, names):
    for name in names:
        c.need(value.get(name) in (None, "", False, 0, [], {}), "runtime-unsupported-option")


def known_path(mask, *path):
    for part in path:
        c.need(mask is not True, "runtime-config-unknown")
        if isinstance(part, int):
            mask = mask[part] if isinstance(mask, list) and len(mask) > part else None
        else:
            mask = mask.get(part) if isinstance(mask, dict) else None
    c.need(not c.has_unknown(mask), "runtime-config-unknown")


def validate_service(after, unknown, value):
    for key in ("project", "name", "location", "ingress", "deletion_protection", "invoker_iam_disabled"):
        c.need(not c.has_unknown(unknown.get(key)), "runtime-config-unknown")
    for key in ("min_instance_count", "max_instance_count", "scaling_mode"):
        known_path(unknown, "scaling", 0, key)
    for key in ("type", "percent"):
        known_path(unknown, "traffic", 0, key)
    for key in ("service_account", "timeout", "max_instance_request_concurrency", "execution_environment", "scaling"):
        known_path(unknown, "template", 0, key)
    for key in ("image", "ports", "resources", "env", "startup_probe"):
        known_path(unknown, "template", 0, "containers", 0, key)
    empty_options(after, ("annotations", "labels", "binary_authorization", "build_config", "custom_audiences",
                          "multi_region_settings", "ssh_enabled", "tags", "iap_enabled"))
    scaling = one(after.get("scaling"), "runtime-service-scaling")
    c.need(scaling.get("min_instance_count") == 0 and scaling.get("max_instance_count") == 1
           and scaling.get("scaling_mode") == "AUTOMATIC", "runtime-service-capacity")
    empty_options(scaling, ("manual_instance_count",))
    traffic = one(after.get("traffic"), "runtime-traffic")
    c.need(traffic.get("type") == "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST" and traffic.get("percent") == 100, "runtime-latest-traffic")
    empty_options(traffic, ("tag",))
    c.need(not traffic.get("revision") or re.fullmatch(r"mypage-api-[a-z0-9-]{1,50}", traffic["revision"]), "runtime-traffic-revision")
    template = one(after.get("template"), "runtime-template")
    c.need(template.get("service_account") == c.EMAIL and template.get("timeout") == "30s"
           and template.get("max_instance_request_concurrency") == 40
           and template.get("execution_environment") == "EXECUTION_ENVIRONMENT_GEN2", "runtime-template-boundary")
    empty_options(template, ("annotations", "labels", "encryption_key", "node_selector", "sandboxes", "volumes",
                             "vpc_access", "workload_identity_config", "health_check_disabled", "session_affinity"))
    c.need(not template.get("revision") or re.fullmatch(r"mypage-api-[a-z0-9-]{1,50}", template["revision"]), "runtime-owned-revision")
    scaling = one(template.get("scaling"), "runtime-revision-scaling")
    c.need(scaling.get("min_instance_count") == 0 and scaling.get("max_instance_count") == 1, "runtime-revision-capacity")
    container = one(template.get("containers"), "runtime-container")
    c.need(container.get("image") == value["image_digest"], "runtime-image-value")
    empty_options(container, ("args", "command", "depends_on", "base_image_uri", "sandbox_launcher", "volume_mounts", "working_dir"))
    empty_options(container, ("liveness_probe", "readiness_probe"))
    probe = one(container.get("startup_probe"), "runtime-startup-probe")
    c.need(probe.get("initial_delay_seconds") == 0 and probe.get("timeout_seconds") == 1
           and probe.get("period_seconds") == 10 and probe.get("failure_threshold") == 3
           and probe.get("tcp_socket") == [{"port": 8080}], "runtime-startup-probe-value")
    empty_options(probe, ("grpc", "http_get"))
    port = one(container.get("ports"), "runtime-port")
    c.need(port == {"name": "http1", "container_port": 8080}, "runtime-port-value")
    resources = one(container.get("resources"), "runtime-resources")
    c.need(resources.get("limits") == {"cpu": "1", "memory": "512Mi"}
           and resources.get("cpu_idle") is True and resources.get("startup_cpu_boost") is False, "runtime-billing-resources")
    env = container.get("env")
    c.need(type(env) is list and all(type(e) is dict for e in env), "runtime-environment-shape")
    by_name = {e.get("name"): e for e in env}
    plain = plain_environment(value)
    secret = {"MYPAGE_OAUTH_CLIENT_ID": ("mypage-oauth-client-id", "oauth_client_id_version"),
              "MYPAGE_OAUTH_CLIENT_SECRET": ("mypage-oauth-client-secret", "oauth_client_secret_version"),
              "MYPAGE_YOUTUBE_API_KEY": ("mypage-youtube-api-key", "youtube_api_key_version")}
    c.need(len(by_name) == len(env) and set(by_name) == set(plain) | set(secret), "runtime-environment-set")
    for name, expected in plain.items():
        c.need(by_name[name].get("value") == expected and not by_name[name].get("value_source"), "runtime-environment-value")
    for name, (secret_id, field) in secret.items():
        c.need(not by_name[name].get("value"), "runtime-no-secret-payload")
        source = one(by_name[name].get("value_source"), "runtime-secret-source")
        c.need(one(source.get("secret_key_ref"), "runtime-secret-ref") == {"secret": secret_id, "version": value[field]}, "runtime-secret-pinned")


def validate_service_empty_refresh(item, planned, value):
    """Only the eight observed null-to-empty defaults, with no serving write."""
    c.need(item.get("mode") == "managed" and item.get("provider_name") == "registry.terraform.io/hashicorp/google", "runtime-refresh-provider")
    change = item.get("change", {})
    c.need(change.get("actions") == ["update"] and not change.get("importing") and not change.get("replace_paths"), "runtime-refresh-action")
    c.need(not c.has_unknown(change.get("after_unknown")), "runtime-refresh-unknown")
    before, after = change.get("before"), change.get("after")
    c.need(type(before) is dict and type(after) is dict, "runtime-refresh-shape")
    plan = planned.get(SERVICE, {}).get("change", {})
    c.need(plan.get("actions") == ["no-op"] and plan.get("after") == after
           and not c.has_unknown(plan.get("after_unknown")), "runtime-refresh-planned-noop")
    normalized = deepcopy(before)
    paths = [("annotations",), ("labels",), ("custom_audiences",),
             ("template", 0, "annotations"), ("template", 0, "labels"),
             ("template", 0, "containers", 0, "args"),
             ("template", 0, "containers", 0, "command"),
             ("template", 0, "containers", 0, "depends_on")]
    for path in paths:
        old_parent, new_parent = normalized, after
        for key in path[:-1]:
            if isinstance(key, int):
                c.need(type(old_parent) is list and type(new_parent) is list
                       and len(old_parent) == len(new_parent) == 1, "runtime-refresh-shape")
            else:
                c.need(type(old_parent) is dict and type(new_parent) is dict
                       and key in old_parent and key in new_parent, "runtime-refresh-shape")
            old_parent, new_parent = old_parent[key], new_parent[key]
        key = path[-1]
        c.need(type(old_parent) is dict and type(new_parent) is dict
               and key in old_parent and key in new_parent, "runtime-refresh-shape")
        old, new = old_parent[key], new_parent[key]
        if old != new:
            empty_type = dict if key in {"annotations", "labels"} else list
            c.need(old is None and type(new) is empty_type and not new, "runtime-refresh-empty-only")
            old_parent[key] = deepcopy(new)
    c.need(normalized != before and normalized == after, "runtime-refresh-fields")
    for candidate in (before, after):
        for key, expected in resource_contract("runtime_private")[SERVICE].items():
            c.need(candidate.get(key) == expected, "runtime-refresh-boundary")
        validate_service(candidate, {}, value)



def validate_owner_hosting_image_refresh(item, planned, value):
    """Read back one declared owner image/Hosting-host fix; CI never updates Run."""
    change = item.get("change", {})
    before, after = change.get("before"), change.get("after")
    c.need(item.get("mode") == "managed" and item.get("provider_name") == "registry.terraform.io/hashicorp/google", "runtime-owner-refresh-provider")
    c.need(change.get("actions") == ["update"] and not change.get("importing") and not change.get("replace_paths")
           and not c.has_unknown(change.get("after_unknown")), "runtime-owner-refresh-action")
    c.need(type(before) is dict and type(after) is dict and set(before) == set(after), "runtime-owner-refresh-shape")
    plan = planned.get(SERVICE, {}).get("change", {})
    c.need(plan.get("actions") == ["no-op"] and plan.get("after") == after and not c.has_unknown(plan.get("after_unknown")), "runtime-owner-refresh-noop")
    previous = value["previous_image_digest"]
    c.need(previous and previous != value["image_digest"] and value["hosting_backend_host"], "runtime-owner-refresh-declaration")
    old_value = value | {"image_digest": previous, "hosting_backend_host": ""}
    for candidate, config in ((before, old_value), (after, value)):
        for key, expected in resource_contract("runtime_private")[SERVICE].items():
            c.need(candidate.get(key) == expected, "runtime-owner-refresh-boundary")
        validate_service(candidate, {}, config)
    normalized = deepcopy(before)
    old_template, new_template = one(normalized.get("template"), "runtime-owner-refresh-template"), one(after.get("template"), "runtime-owner-refresh-template")
    old_container, new_container = one(old_template.get("containers"), "runtime-owner-refresh-container"), one(new_template.get("containers"), "runtime-owner-refresh-container")
    c.need(old_container.get("image") == previous and new_container.get("image") == value["image_digest"], "runtime-owner-refresh-image")
    old_container["image"] = value["image_digest"]
    old_env, new_env = old_container.get("env"), new_container.get("env")
    c.need(type(old_env) is list and type(new_env) is list and len(new_env) == len(old_env) + 1, "runtime-owner-refresh-env")
    added = [x for x in new_env if x.get("name") == "MYPAGE_HOSTING_BACKEND_HOST"]
    c.need(len(added) == 1 and set(added[0]) <= {"name", "value", "value_source"}
           and added[0].get("value") == value["hosting_backend_host"] and not added[0].get("value_source"), "runtime-owner-refresh-host")
    c.need([x for x in new_env if x.get("name") != "MYPAGE_HOSTING_BACKEND_HOST"] == old_env, "runtime-owner-refresh-env")
    old_container["env"] = deepcopy(new_env)
    # Auto-generated revision names are observed on latest_ready_revision;
    # Run leaves the optional desired template.revision empty before and after.
    ready = after.get("latest_ready_revision")
    ready_match = re.fullmatch(r"(?:projects/" + re.escape(c.PROJECT) + r"/locations/asia-southeast2/services/mypage-api/revisions/)?(mypage-api-[a-z0-9-]{1,50})", ready) if type(ready) is str else None
    c.need(ready_match is not None and after.get("latest_created_revision") == ready, "runtime-owner-refresh-ready")
    revision = ready_match[1]
    desired_revision = new_template.get("revision")
    c.need(desired_revision == revision or (desired_revision == "" and old_template.get("revision") == ""), "runtime-owner-refresh-ready")
    old_template["revision"] = desired_revision
    terminal = one(after.get("terminal_condition"), "runtime-owner-refresh-ready")
    c.need(terminal.get("state") == "CONDITION_SUCCEEDED" and not after.get("reconciling"), "runtime-owner-refresh-ready")
    traffic = one(after.get("traffic_statuses"), "runtime-owner-refresh-traffic")
    c.need(traffic.get("percent") == 100 and traffic.get("type") == "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
           and traffic.get("revision", "").split("/")[-1] == revision and not traffic.get("tag"), "runtime-owner-refresh-traffic")
    c.need(re.fullmatch(r"[1-9][0-9]*", str(before.get("generation")))
           and int(after.get("generation", 0)) == int(before["generation"]) + 1
           and str(after.get("observed_generation")) == str(after.get("generation")), "runtime-owner-refresh-generation")
    c.need(c.registry_timestamp(after.get("update_time")) > c.registry_timestamp(before.get("update_time")), "runtime-owner-refresh-time")
    computed = {"etag", "generation", "observed_generation", "update_time", "last_modifier", "latest_created_revision", "latest_ready_revision", "terminal_condition", "conditions", "traffic_statuses", "reconciling"}
    for key in computed:
        if key in after:
            normalized[key] = deepcopy(after[key])
    c.need(normalized == after, "runtime-owner-refresh-fields")


def validate_plan(plan, stage):
    value = settings(plan.get("variables", {}).get("runtime_settings", {}).get("value"))
    c.need(plan.get("variables", {}).get("stage", {}).get("value") == stage, "runtime-plan-stage")
    expected = resource_contract(stage)
    changes = plan.get("resource_changes", [])
    c.need(type(changes) is list, "runtime-plan-resources")
    base = set(c.resource_contract("registry"))
    foundation = deepcopy(plan)
    foundation["variables"]["stage"]["value"] = "registry"
    foundation["resource_changes"] = [item for item in changes if item.get("address") in base]
    foundation["resource_drift"] = []
    drift_seen = set()
    planned = {item.get("address"): item for item in changes}
    for item in plan.get("resource_drift", []) or []:
        address = item.get("address")
        c.need(address not in drift_seen, "runtime-duplicate-drift")
        drift_seen.add(address)
        if address in base:
            foundation["resource_drift"].append(item)
        elif address == SERVICE:
            before = item.get("change", {}).get("before", {})
            template = one(before.get("template"), "runtime-refresh-template") if type(before) is dict else {}
            container = one(template.get("containers"), "runtime-refresh-container")
            if value["previous_image_digest"] and container.get("image") == value["previous_image_digest"]:
                validate_owner_hosting_image_refresh(item, planned, value)
            else:
                validate_service_empty_refresh(item, planned, value)
        else:
            # Removing temporary bootstrap access changes the same shared
            # policy etag, while these fixed managed members stay unchanged.
            c.need(address in {FIRESTORE, INVOKER} and address in expected, "runtime-drift-address")
            member_values = expected[address]
            if address == INVOKER and item.get("change", {}).get("after", {}).get("name") == PUBLIC_INVOKER_NAME:
                member_values = member_values | {"name": PUBLIC_INVOKER_NAME}
            c.validate_iam_etag_refresh(item, planned, member_values)
    c.validate_plan(foundation, "registry")
    seen = set()
    for item in changes:
        address = item.get("address")
        c.need(address in expected and address not in seen, "runtime-resource-address")
        seen.add(address)
        if address in base:
            continue
        c.need(item.get("mode") == "managed" and item.get("provider_name") == "registry.terraform.io/hashicorp/google", "runtime-resource-provider")
        change = item.get("change", {})
        # Initial deployment and no-op only. Updating a serving revision is a
        # separate reviewed transition, not an implicit expansion of this gate.
        c.need(change.get("actions") in [["create"], ["no-op"]] and not change.get("replace_paths"), "runtime-resource-action")
        importing = change.get("importing")
        if address == INVOKER:
            c.need(stage == "runtime_public" and change.get("actions") == ["no-op"], "runtime-invoker-owner-managed")
            if importing:
                c.need(importing == {"id": PUBLIC_INVOKER_IMPORT_ID}, "runtime-invoker-import")
        else:
            c.need(not importing, "runtime-resource-action")
        after, unknown = change.get("after"), change.get("after_unknown", {})
        c.need(type(after) is dict and not after.get("condition"), "runtime-resource-after")
        for key, expected_value in expected[address].items():
            allowed = {expected_value}
            if address == INVOKER and key == "name":
                allowed.add(PUBLIC_INVOKER_NAME)  # The provider importer stores the full fixed service name.
            c.need(after.get(key) in allowed and not c.has_unknown(unknown.get(key)), "runtime-resource-value")
        if address == SERVICE:
            validate_service(after, unknown, value)
    c.need(seen == set(expected), "runtime-resource-set")
    return {"resources": len(seen), "creates": sum(item["change"]["actions"] == ["create"] for item in changes)}
