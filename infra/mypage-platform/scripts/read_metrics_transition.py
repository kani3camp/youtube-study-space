"""Exact DEV metrics-only serving transition; no cloud I/O or private output."""
from copy import deepcopy
import re

import foundation_contract as c
import runtime_contract as r

METRICS_ENV = "MYPAGE_READ_METRICS"
# Output-only attributes of the pinned Cloud Run provider. Never hide unknowns
# in configuration (including template, env, secrets, identity and traffic).
COMPUTED = frozenset({
    "etag", "generation", "observed_generation", "update_time", "last_modifier",
    "latest_created_revision", "latest_ready_revision", "terminal_condition",
    "conditions", "traffic_statuses", "reconciling",
})


def revision_name(value):
    pattern = (r"(?:projects/" + re.escape(c.PROJECT) +
               r"/locations/asia-southeast2/services/mypage-api/revisions/)?"
               r"(mypage-api-[a-z0-9-]{1,50})")
    match = re.fullmatch(pattern, value) if type(value) is str else None
    c.need(match is not None, "metrics-transition-revision")
    return match[1]


def ready_revision(service):
    ready = revision_name(service.get("latest_ready_revision"))
    c.need(revision_name(service.get("latest_created_revision")) == ready,
           "metrics-transition-ready")
    terminal = r.one(service.get("terminal_condition"), "metrics-transition-ready")
    c.need(terminal.get("state") == "CONDITION_SUCCEEDED" and service.get("reconciling") is False,
           "metrics-transition-ready")
    generation = str(service.get("generation"))
    c.need(re.fullmatch(r"[1-9][0-9]*", generation) is not None and
           str(service.get("observed_generation")) == generation, "metrics-transition-generation")
    traffic = r.one(service.get("traffic_statuses"), "metrics-transition-traffic")
    c.need(traffic.get("type") == "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST" and
           traffic.get("percent") == 100 and not traffic.get("tag") and
           revision_name(traffic.get("revision")) == ready, "metrics-transition-traffic")
    return ready


def configuration(service):
    """All present attributes except an explicit output-only allowlist."""
    return {key: value for key, value in service.items() if key not in COMPUTED}


def container(service):
    template = r.one(service.get("template"), "metrics-transition-template")
    # A fixed revision cannot be reused for a new environment configuration.
    c.need(template.get("revision") in (None, ""), "metrics-transition-fixed-revision")
    return r.one(template.get("containers"), "metrics-transition-container")


def recovery_transition(plan, stage, value):
    """Prove one already-applied toggle before a fresh State-only recovery.

    Never mutate the native Plan: its complete drift remains in both HMACs.
    The copy is only an equivalent transition proof for existing validators.
    """
    if not plan.get("resource_drift"):
        return None
    drift = r.one(plan.get("resource_drift"), "metrics-recovery-drift")
    change = drift.get("change", {})
    c.need(drift.get("address") == r.SERVICE and drift.get("mode") == "managed"
           and drift.get("provider_name") == "registry.terraform.io/hashicorp/google"
           and change.get("actions") == ["update"] and not change.get("importing")
           and not change.get("replace_paths") and not c.has_unknown(change.get("after_unknown")), "metrics-recovery-drift")
    for item in plan.get("resource_changes", []):
        current = item.get("change", {})
        c.need(current.get("actions") == ["no-op"] and current.get("before") == current.get("after")
               and not current.get("importing") and not current.get("replace_paths")
               and not c.has_unknown(current.get("after_unknown")), "metrics-recovery-no-remote-write")
    projected = deepcopy(plan)
    projected["resource_drift"] = []
    service = next((x["change"] for x in projected.get("resource_changes", []) if x.get("address") == r.SERVICE), None)
    c.need(service is not None and service.get("after") == change.get("after"), "metrics-recovery-result")
    service.update(actions=["update"], before=deepcopy(change.get("before")))
    validate_plan(projected, stage, value)
    prior = plan.get("prior_state", {}).get("values")
    c.need(type(prior) is dict, "metrics-recovery-prior-state")
    validate_readback({"values": prior}, projected, stage)
    return projected


def validate_plan(plan, stage, value):
    """Admit one env toggle, never an image rollout or generic runtime update."""
    c.need(value["read_metrics_from_revision"] != "", "metrics-transition-declaration")
    recovered = recovery_transition(plan, stage, value)
    if recovered is not None:
        return validate_plan(recovered, stage, value) | {"updates": 0, "stateRecovery": True}
    c.need(not plan.get("resource_drift"), "metrics-transition-drift")
    changes = plan.get("resource_changes")
    c.need(type(changes) is list, "metrics-transition-resources")
    expected = r.resource_contract(stage)
    seen = set()
    service_change = None
    for item in changes:
        address = item.get("address")
        c.need(address in expected and address not in seen and item.get("mode") == "managed" and
               item.get("provider_name") == "registry.terraform.io/hashicorp/google",
               "metrics-transition-resources")
        seen.add(address)
        change = item.get("change", {})
        c.need(not change.get("replace_paths") and not change.get("importing"),
               "metrics-transition-action")
        before, after = change.get("before"), change.get("after")
        c.need(type(before) is dict and type(after) is dict, "metrics-transition-shape")
        if address == r.SERVICE:
            c.need(change.get("actions") == ["update"], "metrics-transition-action")
            service_change = change
        else:
            c.need(change.get("actions") == ["no-op"] and before == after and
                   not c.has_unknown(change.get("after_unknown")), "metrics-transition-other-resource")
    c.need(seen == set(expected) and service_change is not None, "metrics-transition-resources")
    before, after = service_change["before"], service_change["after"]
    unknown = service_change.get("after_unknown", {})
    c.need(type(unknown) is dict and not c.has_unknown({k: v for k, v in unknown.items() if k not in COMPUTED}),
           "metrics-transition-config-unknown")
    c.need(ready_revision(before) == value["read_metrics_from_revision"], "metrics-transition-stale-revision")
    for candidate in (before, after):
        for key, fixed in expected[r.SERVICE].items():
            c.need(candidate.get(key) == fixed, "metrics-transition-boundary")
    r.validate_service(before, {}, value | {"read_metrics_enabled": not value["read_metrics_enabled"]}, ["no-op"])
    r.validate_service(after, unknown, value, ["no-op"])
    normalized = deepcopy(configuration(before))
    old_container, new_container = container(normalized), container(after)
    # Order may move when inserting/removing the sole dynamic env entry; all
    # other entries (including their secret references) must remain identical.
    old_env, new_env = old_container["env"], new_container["env"]
    c.need([x for x in old_env if x["name"] != METRICS_ENV] ==
           [x for x in new_env if x["name"] != METRICS_ENV], "metrics-transition-env-only")
    old_container["env"] = deepcopy(new_env)
    c.need(normalized == configuration(after), "metrics-transition-env-only")
    # Reuse the entire existing resource/configuration admission. Only this
    # independently verified service action is projected to no-op in a copy;
    # the ORIGINAL full Plan still feeds HMAC agreement and conditional API apply.
    checked = deepcopy(plan)
    next(x for x in checked["resource_changes"] if x["address"] == r.SERVICE)["change"]["actions"] = ["no-op"]
    result = r.validate_initial_plan(checked, stage)
    return result | {"updates": 1}


def validate_readback(state, plan, stage):
    """Verify saved-plan result without accepting a stale serving revision."""
    value = r.settings(plan.get("variables", {}).get("runtime_settings", {}).get("value"))
    plan = recovery_transition(plan, stage, value) or plan
    validate_plan(plan, stage, value)
    resources = {}

    def walk(module):
        for item in module.get("resources", []):
            address = item.get("address")
            c.need(address not in resources, "metrics-readback-resources")
            resources[address] = item
        for child in module.get("child_modules", []):
            walk(child)

    walk(state.get("values", {}).get("root_module", {}))
    c.need(set(resources) == set(r.resource_contract(stage)), "metrics-readback-resources")
    for planned in plan["resource_changes"]:
        observed = resources[planned["address"]]
        c.need(observed.get("mode") == "managed" and
               observed.get("provider_name") == "registry.terraform.io/hashicorp/google",
               "metrics-readback-provider")
        actual = observed.get("values")
        c.need(type(actual) is dict, "metrics-readback-shape")
        expected = planned["change"]["after"]
        if planned["address"] != r.SERVICE:
            c.need(actual == expected, "metrics-readback-other-resource")
            continue
        c.need(configuration(actual) == configuration(expected), "metrics-readback-config")
        r.validate_service(actual, {}, value, ["no-op"])
        before = planned["change"]["before"]
        c.need(ready_revision(actual) != value["read_metrics_from_revision"], "metrics-readback-old-revision")
        c.need(int(actual["generation"]) == int(before["generation"]) + 1, "metrics-readback-generation")
        c.need(c.registry_timestamp(actual.get("update_time")) > c.registry_timestamp(before.get("update_time")),
               "metrics-readback-time")


def readback_saved(env):
    """Run only in the authenticated apply job against its retained saved Plan."""
    import hashlib
    import json
    import foundation_ops as ops

    c.context(env, "apply")
    c.need(env.get("MYPAGE_ROLE") == "apply", "metrics-readback-role")
    stage = c.stage_for_mode(env["MYPAGE_MODE"])
    c.need(stage in {"runtime_private", "runtime_public"}, "metrics-readback-stage")
    root = ops.directory(env)
    c.need(root.is_dir() and not root.is_symlink(), "metrics-readback-evidence")
    saved = root / "foundation.tfplan"
    receipt = json.loads((root / "prepared.json").read_text())
    c.need(receipt == {"sha256": hashlib.sha256(saved.read_bytes()).hexdigest(), "source": env["GITHUB_SHA"]},
           "metrics-readback-plan-receipt")
    work = root / "source/environments/dev"
    plan = json.loads(ops.run(["terraform", "show", "-json", str(saved)], cwd=work))
    value = r.settings(plan.get("variables", {}).get("runtime_settings", {}).get("value"))
    if not value["read_metrics_from_revision"]:
        print("Metrics transition not requested; existing runtime release checks unchanged.")
        return
    state = json.loads(ops.run(["terraform", "show", "-json"], cwd=work))
    from read_metrics_live import verify_state_live
    verify_state_live(env, state, plan, stage)
    print("DEV metrics transition live API and State readback passed.")


if __name__ == "__main__":
    import os
    import sys

    try:
        c.need(sys.argv[1:] == ["readback"], "metrics-readback-command")
        readback_saved(os.environ)
    except c.ContractError as error:
        print("MyPage metrics transition STOP; check=" + str(error) + ".", file=sys.stderr)
        raise SystemExit(1)
    except Exception:
        print("MyPage metrics transition STOP; private diagnostics suppressed.", file=sys.stderr)
        raise SystemExit(1)
