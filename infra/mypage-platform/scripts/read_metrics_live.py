"""ETag-conditional DEV metrics mutation and authoritative Cloud Run readback.

The provider's ordinary update Plan is reviewed but NEVER applied in this mode:
Google provider 8.5.0 does not send an etag. Only a bounded refresh-only Plan may
be applied after the conditional API mutation. No retry/rebase or auto rollback.
"""
from copy import deepcopy
import hashlib
import json
import re
import time
import urllib.error
import urllib.request

import foundation_contract as c
import runtime_contract as r
import read_metrics_transition as m

SERVICE_NAME = "projects/" + c.PROJECT + "/locations/asia-southeast2/services/mypage-api"
SERVICE_URL = "https://run.googleapis.com/v2/" + SERVICE_NAME
PATCH_URL = SERVICE_URL + "?updateMask=template.containers,template.revision&allowMissing=false"
OPERATION = re.compile(r"projects/" + re.escape(c.PROJECT) + r"/locations/asia-southeast2/operations/[A-Za-z0-9_-]+")
# Only output metadata is removed. Unknown API configuration stays in the
# before/after comparison rather than being silently ignored.
API_OUTPUTS = frozenset({
    "@type", "uid", "generation", "observedGeneration", "createTime", "updateTime",
    "deleteTime", "expireTime", "creator", "lastModifier", "terminalCondition",
    "conditions", "latestReadyRevision", "latestCreatedRevision", "trafficStatuses",
    "uri", "urls", "satisfiesPzs", "threatDetectionEnabled", "reconciling", "etag",
})
SINGLETONS = frozenset({
    "template", "scaling", "resources", "startup_probe", "liveness_probe", "readiness_probe",
    "tcp_socket", "http_get", "grpc", "value_source", "secret_key_ref", "binary_authorization",
    "vpc_access", "node_selector", "build_config", "multi_region_settings",
})
CONFIG_FIELDS = (
    "description", "client", "client_version", "ingress", "launch_stage", "binary_authorization",
    "custom_audiences", "scaling", "default_uri_disabled", "template", "traffic",
    "invoker_iam_disabled", "build_config", "multi_region_settings", "iap_enabled", "ssh_enabled",
)


def camel(name):
    first, *rest = name.split("_")
    return first + "".join(part.title() for part in rest)


def compact(value):
    """Proto JSON omits defaults; keep list order and non-default map values."""
    if type(value) is dict:
        result = {key: compact(item) for key, item in value.items()}
        return {key: item for key, item in result.items() if item not in (None, "", False, 0, [], {})}
    if type(value) is list:
        return [compact(item) for item in value]
    return value


def wire(value, field=""):
    """Translate the pinned provider's singleton blocks, never arbitrary keys."""
    if field in {"limits", "labels", "annotations", "effective_labels", "effective_annotations"}:
        return value or {}
    if field in SINGLETONS:
        if not value:
            return {}
        value = r.one(value, "metrics-live-block")
    if type(value) is dict:
        return {camel(key): wire(item, key) for key, item in value.items()}
    if type(value) is list:
        return [wire(item) for item in value]
    return value


def api_configuration(service):
    result = deepcopy({key: item for key, item in service.items() if key not in API_OUTPUTS})
    # Empty desired revision means auto-generated. A returned name must be the
    # current revision, not a user-chosen foreign or old revision.
    template = result.get("template", {})
    if template.get("revision"):
        c.need(m.revision_name(template["revision"]) == m.revision_name(service.get("latestCreatedRevision")),
               "metrics-live-template-revision")
    template.pop("revision", None)
    return compact(result)


def ready(service):
    c.need(type(service) is dict and service.get("name") == SERVICE_NAME, "metrics-live-target")
    c.need(service.get("@type") in (None, "type.googleapis.com/google.cloud.run.v2.Service"), "metrics-live-type")
    c.need(not service.get("deleteTime") and not service.get("expireTime"), "metrics-live-deleted")
    c.need(service.get("reconciling", False) is False and
           service.get("terminalCondition", {}).get("state") == "CONDITION_SUCCEEDED", "metrics-live-ready")
    generation = service.get("generation")
    c.need(type(generation) is str and re.fullmatch(r"[1-9][0-9]*", generation) is not None and
           service.get("observedGeneration") == generation, "metrics-live-generation")
    revision = m.revision_name(service.get("latestReadyRevision"))
    c.need(m.revision_name(service.get("latestCreatedRevision")) == revision, "metrics-live-ready")
    target = r.one(service.get("trafficStatuses"), "metrics-live-traffic")
    c.need(target.get("type") == "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST" and
           type(target.get("percent")) is int and target["percent"] == 100 and not target.get("tag") and
           m.revision_name(target.get("revision")) == revision, "metrics-live-traffic")
    etag = service.get("etag")
    c.need(type(etag) is str and 0 < len(etag) <= 1024 and "\n" not in etag and "\r" not in etag,
           "metrics-live-etag")
    c.need(type(service.get("uid")) is str and bool(service["uid"]), "metrics-live-identity")
    return revision


def match_state(live, state):
    """Compare a live GET with a Plan source or final State, including ETag."""
    c.need(ready(live) == m.ready_revision(state), "metrics-live-revision-mismatch")
    for api_key, tf_key in (("generation", "generation"), ("observedGeneration", "observed_generation"),
                           ("etag", "etag"), ("uid", "uid")):
        c.need(live.get(api_key) == state.get(tf_key), "metrics-live-identity-mismatch")
    c.need(c.registry_timestamp(live.get("updateTime")) == c.registry_timestamp(state.get("update_time")),
           "metrics-live-time-mismatch")
    expected = {camel(key): wire(state.get(key), key) for key in CONFIG_FIELDS}
    expected["labels"] = state.get("effective_labels", state.get("labels")) or {}
    expected["annotations"] = state.get("effective_annotations", state.get("annotations")) or {}
    expected["template"].pop("revision", None)
    observed = api_configuration(live)
    # The provider canonicalizes named env blocks; REST preserves insertion
    # order. Compare this one named collection after rejecting duplicate names.
    # Do not reorder command/args/containers or change the saved Plan itself.
    def named_environment(config):
        containers = config.get("template", {}).get("containers")
        c.need(type(containers) is list, "metrics-live-env-shape")
        for container in containers:
            env = container.get("env", [])
            c.need(type(env) is list and all(type(x) is dict and type(x.get("name")) is str and x["name"] for x in env),
                   "metrics-live-env-shape")
            c.need(len({x["name"] for x in env}) == len(env), "metrics-live-env-duplicate")
            if "env" in container:
                container["env"] = sorted(env, key=lambda x: x["name"])
        return config
    c.need(compact(named_environment(expected)) == compact(named_environment({key: observed.get(key) for key in expected})),
           "metrics-live-config-mismatch")


def planned_service(plan):
    return next(item["change"] for item in plan["resource_changes"] if item["address"] == r.SERVICE)


def state_service(state):
    found = []
    def walk(module):
        found.extend(item["values"] for item in module.get("resources", []) if item["address"] == r.SERVICE)
        for child in module.get("child_modules", []):
            walk(child)
    walk(state.get("values", {}).get("root_module", {}))
    c.need(len(found) == 1, "metrics-live-state-service")
    return found[0]


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise c.ContractError("metrics-live-redirect")


def request(token, url, body=None):
    """Fixed Google endpoint, bounded response, no redirects or write retries."""
    operation = url.removeprefix("https://run.googleapis.com/v2/")
    allowed = (url == PATCH_URL) if body is not None else (url == SERVICE_URL or OPERATION.fullmatch(operation))
    c.need(bool(allowed), "metrics-live-url")
    c.need(type(token) is str and bool(token), "metrics-live-credential")
    req = urllib.request.Request(url, data=c.canonical(body) if body is not None else None,
                                 method="PATCH" if body is not None else "GET",
                                 headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"})
    try:
        with urllib.request.build_opener(NoRedirect()).open(req, timeout=30) as response:
            raw = response.read(1024 * 1024 + 1)
    except urllib.error.HTTPError as error:
        # No response text/headers, rejected environment, token or URL in logs.
        label = "metrics-live-conflict" if error.code in (409, 412) else "metrics-live-api-failed"
        raise c.ContractError(label) from None
    c.need(len(raw) <= 1024 * 1024, "metrics-live-response-size")
    value = json.loads(raw)
    c.need(type(value) is dict, "metrics-live-response")
    return value


def expected_configuration(before, target):
    expected = deepcopy(before)
    r.one(expected["template"]["containers"], "metrics-live-container")["env"] = wire(m.container(target)["env"])
    return api_configuration(expected)


def validate_result(live, before, target):
    c.need(ready(live) != ready(before), "metrics-live-old-revision")
    c.need(live["uid"] == before["uid"] and int(live["generation"]) == int(before["generation"]) + 1 and
           live["etag"] != before["etag"], "metrics-live-result-generation")
    c.need(c.registry_timestamp(live.get("updateTime")) > c.registry_timestamp(before.get("updateTime")),
           "metrics-live-result-time")
    c.need(api_configuration(live) == expected_configuration(before, target), "metrics-live-result-config")


def conditional_update(token, before, target):
    """The original ETag, not a freshly accepted competing version, is sent."""
    containers = deepcopy(before["template"]["containers"])
    r.one(containers, "metrics-live-container")["env"] = wire(m.container(target)["env"])
    body = {"name": SERVICE_NAME, "etag": before["etag"],
            "template": {"revision": "", "containers": containers}}
    operation = request(token, PATCH_URL, body)
    name = operation.get("name")
    c.need(type(name) is str and OPERATION.fullmatch(name) is not None, "metrics-live-operation-target")
    deadline = time.monotonic() + 300
    while operation.get("done", False) is not True:
        c.need(time.monotonic() < deadline, "metrics-live-operation-timeout")
        time.sleep(2)
        operation = request(token, "https://run.googleapis.com/v2/" + name)
        c.need(operation.get("name") == name, "metrics-live-operation-target")
    c.need(not operation.get("error") and type(operation.get("response")) is dict,
           "metrics-live-operation-failed")
    result = operation["response"]
    validate_result(result, before, target)
    live = request(token, SERVICE_URL)
    validate_result(live, before, target)
    c.need(live["etag"] == result["etag"] and ready(live) == ready(result), "metrics-live-superseded")
    return live


def validate_refresh(refresh, plan, stage):
    """Only observed metrics transition + output metadata may enter State."""
    c.need(refresh.get("format_version") == plan["format_version"] and refresh.get("complete") is True and
           refresh.get("errored") is False, "metrics-refresh-envelope")
    c.need(not refresh.get("deferred_changes") and not refresh.get("action_invocations") and
           not refresh.get("deferred_action_invocations") and
           all(item.get("status") == "pass" for item in refresh.get("checks", []) or []), "metrics-refresh-incomplete")
    seen = set()
    for item in refresh.get("resource_changes", []):
        change = item.get("change", {})
        c.need(item.get("address") in r.resource_contract(stage) and item["address"] not in seen and
               change.get("actions") == ["no-op"] and not change.get("importing") and
               not change.get("replace_paths") and not c.has_unknown(change.get("after_unknown")),
               "metrics-refresh-no-remote-write")
        seen.add(item["address"])
    drift = r.one(refresh.get("resource_drift"), "metrics-refresh-drift")
    change = drift.get("change", {})
    c.need(drift.get("address") == r.SERVICE and drift.get("mode") == "managed" and
           drift.get("provider_name") == "registry.terraform.io/hashicorp/google" and
           change.get("actions") == ["update"] and not change.get("importing") and not change.get("replace_paths") and
           not c.has_unknown(change.get("after_unknown")) and change.get("before") == planned_service(plan)["before"],
           "metrics-refresh-source")
    state = {"values": refresh.get("planned_values", {})}
    m.validate_readback(state, plan, stage)
    c.need(change.get("after") == state_service(state), "metrics-refresh-result")
    return state


def verify_state_live(env, state, plan, stage):
    """State is necessary but not a substitute for the authoritative API GET."""
    m.validate_readback(state, plan, stage)
    match_state(request(env.get("MYPAGE_GCP_ACCESS_TOKEN"), SERVICE_URL), state_service(state))


def apply_saved(env, root, plan, stage):
    """Called only after foundation_ops validates the saved Plan receipt/age."""
    import foundation_ops as ops

    c.context(env, "apply")
    c.need(env.get("MYPAGE_ROLE") == "apply", "metrics-live-role")
    value = r.settings(plan.get("variables", {}).get("runtime_settings", {}).get("value"))
    m.validate_plan(plan, stage, value)
    baseline = json.loads((root / "metrics-state-version.json").read_text())
    c.need(type(baseline) is dict and baseline == ops.state_version(env), "metrics-live-state-version")
    before = request(env.get("MYPAGE_GCP_ACCESS_TOKEN"), SERVICE_URL)
    match_state(before, planned_service(plan)["before"])
    c.need(ops.state_version(env) == baseline, "metrics-live-state-version")
    c.need(0 <= time.time() - float(env["MYPAGE_PLAN_TIME"]) <= 1800, "metrics-live-plan-expired")
    live = conditional_update(env.get("MYPAGE_GCP_ACCESS_TOKEN"), before, planned_service(plan)["after"])
    # The only cloud write is the ETag PATCH above. Terraform now refreshes
    # State, never applies the original provider update or an arbitrary drift.
    c.need(ops.state_version(env) == baseline, "metrics-live-state-version")
    work = root / "source/environments/dev"
    refresh_file = root / "metrics-refresh.tfplan"
    ops.run(["terraform", "plan", "-refresh-only", "-input=false", "-no-color", "-lock-timeout=60s",
             "-var=stage=" + stage, "-var-file=" + str(root / "runtime-settings.json"),
             "-out=" + str(refresh_file)], cwd=work, check="terraform-plan")
    digest = hashlib.sha256(refresh_file.read_bytes()).hexdigest()
    refresh = json.loads(ops.run(["terraform", "show", "-json", str(refresh_file)], cwd=work))
    state = validate_refresh(refresh, plan, stage)
    match_state(live, state_service(state))
    verify_state_live(env, state, plan, stage)
    c.need(ops.state_version(env) == baseline, "metrics-live-state-version")
    # Keep a byte hash of the independently validated refresh-only file.
    c.need(hashlib.sha256(refresh_file.read_bytes()).hexdigest() == digest, "metrics-refresh-receipt")
    ops.run(["terraform", "apply", "-input=false", "-no-color", str(refresh_file)], cwd=work)
    state = json.loads(ops.run(["terraform", "show", "-json"], cwd=work))
    verify_state_live(env, state, plan, stage)
