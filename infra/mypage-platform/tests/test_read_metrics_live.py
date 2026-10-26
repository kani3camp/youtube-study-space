"""P1 regressions: stale State, competing writes, conditional mutation, no cloud I/O."""
from copy import deepcopy
import hashlib
import io
import json
from pathlib import Path
import sys
import tempfile
import time
import unittest
from unittest.mock import Mock, patch
import urllib.error

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "infra/mypage-platform/scripts"))
import foundation_contract as c
import foundation_ops as ops
import runtime_contract as r
import read_metrics_transition as m
import read_metrics_live as live
import read_metrics_v1 as v1
from test_read_metrics_transition import fixture as base_fixture, applied_state, actual, service

TOKEN = "synthetic-access-token"
VERSION = {"VersionId": "synthetic-version", "ETag": "synthetic-state-etag", "ContentLength": 42}


def fixture(enabled=True, stage="runtime_private"):
    plan = base_fixture(enabled, stage)
    service(plan)["before"].update(uid="synthetic-service-uid", etag="source-etag")
    service(plan)["after"]["uid"] = "synthetic-service-uid"
    state = applied_state(plan)
    actual(state)["etag"] = "result-etag"
    return plan, state


def api_service(state):
    """Independent REST-shaped fixture, including protobuf-omitted false/zero."""
    container = state["template"][0]["containers"][0]
    env = []
    for item in container["env"]:
        if item.get("value_source"):
            selector = item["value_source"][0]["secret_key_ref"][0]
            env.append({"name": item["name"], "valueSource": {"secretKeyRef": dict(selector)}})
        else:
            env.append({"name": item["name"], "value": item["value"]})
    revision = state["latest_ready_revision"]
    return {
        "name": live.SERVICE_NAME, "uid": state["uid"], "etag": state["etag"],
        "generation": state["generation"], "observedGeneration": state["observed_generation"],
        "updateTime": state["update_time"], "ingress": "INGRESS_TRAFFIC_ALL", "launchStage": "GA",
        "terminalCondition": {"state": "CONDITION_SUCCEEDED"},
        "latestReadyRevision": live.SERVICE_NAME + "/revisions/" + revision,
        "latestCreatedRevision": live.SERVICE_NAME + "/revisions/" + revision,
        "trafficStatuses": [{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 100,
                             "revision": revision}],
        "traffic": [{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 100}],
        "scaling": {"maxInstanceCount": 1, "scalingMode": "AUTOMATIC"},
        "template": {"revision": revision, "serviceAccount": c.EMAIL, "timeout": "30s",
                     "executionEnvironment": "EXECUTION_ENVIRONMENT_GEN2", "maxInstanceRequestConcurrency": 40,
                     "scaling": {"maxInstanceCount": 1},
                     "containers": [{"image": container["image"], "env": env,
                                     "ports": [{"name": "http1", "containerPort": 8080}],
                                     "resources": {"limits": {"cpu": "1", "memory": "512Mi"}, "cpuIdle": True},
                                     "startupProbe": {"timeoutSeconds": 1, "periodSeconds": 10,
                                                      "failureThreshold": 3, "tcpSocket": {"port": 8080}}}]},
    }


def v1_service(state):
    env=[]
    for item in state["template"][0]["containers"][0]["env"]:
        if item.get("value_source"):
            ref=item["value_source"][0]["secret_key_ref"][0]
            env.append({"name":item["name"],"valueFrom":{"secretKeyRef":{"name":ref["secret"],"key":ref["version"]}}})
        else: env.append({"name":item["name"],"value":item["value"]})
    return {"apiVersion":"serving.knative.dev/v1","kind":"Service",
            "metadata":{"name":"mypage-api","namespace":c.PROJECT,"uid":state["uid"],"generation":int(state["generation"]),
                        "resourceVersion":"opaque-"+state["generation"],"labels":{"keep":"label"},"annotations":{"keep":"annotation"}},
            "spec":{"template":{"spec":{"containers":[{"image":state["template"][0]["containers"][0]["image"],"env":env,"futureField":"keep"}]}},
                    "futureField":"keep"},
            "status":{"latestReadyRevisionName":state["latest_ready_revision"],"latestCreatedRevisionName":state["latest_created_revision"],
                      "conditions":[{"type":"Ready","status":"True"}]}}


def refresh_plan(plan, state):
    return {"format_version": plan["format_version"], "complete": True, "errored": False,
            "planned_values": {"root_module": {}},
            "prior_state": {"values": deepcopy(state["values"])},
            "resource_drift": [{"address": r.SERVICE, "mode": "managed",
                                "provider_name": "registry.terraform.io/hashicorp/google",
                                "change": {"actions": ["update"], "before": deepcopy(service(plan)["before"]),
                                           "after": deepcopy(actual(state)), "after_unknown": {}}}]}


def recovery_plan(plan, state):
    recovered = deepcopy(plan)
    observed = {x["address"]: x["values"] for x in state["values"]["root_module"]["child_modules"][0]["resources"]}
    for item in recovered["resource_changes"]:
        item["change"] = {"actions":["no-op"], "before":deepcopy(observed[item["address"]]),
                          "after":deepcopy(observed[item["address"]]), "after_unknown":{}}
    recovered["resource_drift"] = refresh_plan(plan, state)["resource_drift"]
    recovered["prior_state"] = {"values":deepcopy(state["values"])}
    return recovered


def operation(after):
    return {"name": "projects/" + c.PROJECT + "/locations/asia-southeast2/operations/synthetic-operation",
            "done": True, "response": after | {"@type": "type.googleapis.com/google.cloud.run.v2.Service"}}


class MetricsLiveTest(unittest.TestCase):
    def test_refresh_native_diagnostics_publish_only_fixed_step_labels(self):
        private = "must-not-appear-in-public-diagnostics"
        for check, marker, label in (
            ("metrics-refresh-apply", "Saved plan is stale", "saved-plan-stale"),
            ("metrics-refresh-show", "Failed to load plugin schemas", "provider-schema"),
            ("metrics-refresh-apply", "invalid_grant", "identity-expired"),
            ("metrics-refresh-apply", "Cannot apply failed plan", "failed-plan"),
            ("state-version-list", "ExpiredToken", "identity-expired"),
            ("state-version-head", "AccessDenied", "access-denied"),
            ("saved-runtime-plan-show", "unknown native error", None),
            ("metrics-final-state-show", "unknown native error", None),
        ):
            result = Mock(returncode=1, stdout=private, stderr=marker + private)
            with self.subTest(check=check, marker=marker), patch.object(ops.subprocess, "run", return_value=result), self.assertRaises(c.ContractError) as error:
                ops.run(["terraform", "synthetic-command"], check=check)
            self.assertEqual(str(error.exception), check + ("-" + label if label else ""))
            self.assertNotIn(private, str(error.exception))

    def test_recovery_proves_exact_toggle_without_mutating_native_plan(self):
        for enabled in (True, False):
            for stage in ("runtime_private", "runtime_public"):
                plan, state = fixture(enabled, stage)
                recovered = recovery_plan(plan, state)
                original = deepcopy(recovered)
                result = m.validate_plan(recovered, stage, r.settings(recovered["variables"]["runtime_settings"]["value"]))
                self.assertTrue(result["stateRecovery"])
                self.assertEqual(result["updates"], 0)
                m.validate_readback(state, recovered, stage)
                self.assertEqual(recovered, original)
                self.assertNotEqual(c.commitment(recovered, VERSION, "a" * 40, "synthetic-key-for-recovery-contract-tests"),
                                    c.commitment(recovered | {"resource_drift":[]}, VERSION, "a" * 40, "synthetic-key-for-recovery-contract-tests"))
                for mutate in (
                    lambda p: p["resource_drift"].append(deepcopy(p["resource_drift"][0])),
                    lambda p: p["resource_drift"][0]["change"]["before"].update(generation="1"),
                    lambda p: p["resource_drift"][0]["change"]["before"]["template"][0]["containers"][0].update(image=r.REPOSITORY + "@sha256:" + "b" * 64),
                    lambda p: service(p).update(actions=["update"]),
                    lambda p: service(p).update(after_unknown={"template":True}),
                    lambda p: p["prior_state"]["values"]["root_module"]["child_modules"][0]["resources"].pop(),
                ):
                    changed = deepcopy(recovered); mutate(changed)
                    with self.subTest(enabled=enabled, stage=stage, mutation=mutate), self.assertRaises(c.ContractError):
                        m.validate_plan(changed, stage, r.settings(changed["variables"]["runtime_settings"]["value"]))

    def test_recovery_applies_only_fresh_refresh_without_patch(self):
        for enabled in (True, False):
            plan, state = fixture(enabled)
            recovered = recovery_plan(plan, state)
            after = api_service(actual(state))
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "metrics-state-version.json").write_text(json.dumps(VERSION))
                commands = []
                def run(cmd, **kwargs):
                    commands.append(cmd)
                    if cmd[1] == "plan":
                        (root / "metrics-refresh.tfplan").write_bytes(b"fresh-refresh")
                        return ""
                    if cmd[:3] == ["terraform", "show", "-json"]:
                        return json.dumps(refresh_plan(plan, state) if len(cmd) == 4 else state)
                    return ""
                env = {"MYPAGE_ROLE":"apply", "MYPAGE_PLAN_TIME":str(time.time()), "MYPAGE_GCP_ACCESS_TOKEN":TOKEN}
                with patch.object(c, "context"), patch.object(ops, "run", side_effect=run), patch.object(ops, "state_version", return_value=VERSION), \
                     patch.object(live, "request", return_value=after) as request, patch.object(live, "conditional_update") as update:
                    live.apply_saved(env, root, recovered, "runtime_private")
                update.assert_not_called()
                self.assertTrue(all(call.args == (TOKEN, live.SERVICE_URL) for call in request.call_args_list))
                self.assertTrue(all(cmd[-1] == str(root / "metrics-refresh.tfplan") for cmd in commands if cmd[1] == "apply"))

    def test_provider_defaults_encode_exactly_one_envvar_value_member(self):
        entries = [
            {"name":"PLAIN", "value":"public", "value_source":[]},
            {"name":"EMPTY", "value":"", "value_source":[]},
            {"name":"SECRET", "value":"", "value_source":[{"secret_key_ref":[{"secret":"synthetic-secret", "version":"1"}]}]},
        ]
        original = deepcopy(entries)
        self.assertEqual(live.environment_payload(entries), [
            {"name":"PLAIN", "value":"public"},
            {"name":"EMPTY", "value":""},
            {"name":"SECRET", "valueSource":{"secretKeyRef":{"secret":"synthetic-secret", "version":"1"}}},
        ])
        self.assertEqual(entries, original)
        for mutate in (
            lambda xs: xs[-1].update(value="conflicting"),
            lambda xs: xs[0].update(value_source={}),
            lambda xs: xs[0].update(unknown="rejected"),
            lambda xs: xs.append(deepcopy(xs[0])),
        ):
            changed = deepcopy(entries); mutate(changed)
            with self.subTest(mutation=mutate), self.assertRaises(c.ContractError):
                live.environment_payload(changed)

    def test_named_environment_order_preserves_exact_values_and_secret_references(self):
        plan, state = fixture()
        expected = actual(state)
        original = deepcopy(expected)
        api = api_service(expected)
        env = api["template"]["containers"][0]["env"]
        env.reverse()
        live.match_state(api, expected)
        self.assertEqual(expected, original)
        for mutate in (
            lambda xs: next(x for x in xs if "value" in x).update(value="changed"),
            lambda xs: next(x for x in xs if "valueSource" in x)["valueSource"]["secretKeyRef"].update(version="2"),
            lambda xs: xs.append(deepcopy(xs[0])),
            lambda xs: xs[0].update(name=""),
        ):
            changed = deepcopy(api)
            mutate(changed["template"]["containers"][0]["env"])
            with self.subTest(mutation=mutate), self.assertRaises(c.ContractError):
                live.match_state(changed, expected)
        ordered = deepcopy(expected)
        ordered["template"][0]["containers"][0]["args"] = ["first", "second"]
        changed = deepcopy(api)
        changed["template"]["containers"][0]["args"] = ["second", "first"]
        with self.assertRaises(c.ContractError):
            live.match_state(changed, ordered)

    def test_plan_and_live_identity_match_without_mutation(self):
        for enabled in (True, False):
            plan, state = fixture(enabled)
            original = deepcopy(plan)
            live.match_state(api_service(service(plan)["before"]), service(plan)["before"])
            live.match_state(api_service(actual(state)), actual(state))
            self.assertEqual(plan, original)

    def test_normal_state_cannot_hide_rolled_back_or_failed_live_service(self):
        plan, state = fixture()
        m.validate_readback(state, plan, "runtime_private")
        mutations = (
            lambda x: x.update(etag="different-etag"),
            lambda x: x.update(generation="9", observedGeneration="9"),
            lambda x: x.update(reconciling=True),
            lambda x: x.update(terminalCondition={"state": "CONDITION_FAILED"}),
            lambda x: x["trafficStatuses"][0].update(revision="mypage-api-before"),
            lambda x: x["trafficStatuses"][0].update(percent=50),
            lambda x: x["template"]["containers"][0]["env"].pop(1),
            lambda x: x["template"]["containers"][0].update(image="unexpected"),
            lambda x: x.update(name=live.SERVICE_NAME.replace(c.PROJECT, "youtube-study-space")),
            lambda x: x.update(uid="recreated-service"),
        )
        for mutate in mutations:
            response = api_service(actual(state))
            mutate(response)
            with self.subTest(mutation=mutate), patch.object(live, "request", return_value=response), self.assertRaises(c.ContractError):
                live.verify_state_live({"MYPAGE_GCP_ACCESS_TOKEN": TOKEN}, state, plan, "runtime_private")

    def test_plan_followed_by_external_change_is_not_accepted_as_new_baseline(self):
        plan, _ = fixture()
        response = api_service(service(plan)["before"])
        for key, value in (("etag", "external-change"), ("generation", "8"),
                           ("latestReadyRevision", "mypage-api-external"), ("uid", "new-service")):
            altered = response | {key: value}
            with self.subTest(key=key), self.assertRaises(c.ContractError):
                live.match_state(altered, service(plan)["before"])

    def setUp(self):
        # Unit tests can never contact a real Google endpoint.
        blocked = patch.object(live.urllib.request, "build_opener", side_effect=AssertionError("unexpected live network"))
        blocked.start(); self.addCleanup(blocked.stop)

    def test_versioned_replace_preserves_full_configuration_and_original_version(self):
        for enabled in (True, False):
            plan, state = fixture(enabled)
            before, after = api_service(service(plan)["before"]), api_service(actual(state))
            old = v1_service(service(plan)["before"])
            new = v1_service(actual(state))
            original = deepcopy(old)
            with patch.object(v1, "request", side_effect=[old, new, new]) as request, patch.object(live, "request", side_effect=[before, after, after]):
                self.assertEqual(live.conditional_update(TOKEN, before, service(plan)["after"]), after)
            payload = request.call_args_list[1].args[1]
            self.assertEqual(payload["metadata"]["resourceVersion"], old["metadata"]["resourceVersion"])
            self.assertNotIn("status", payload)
            left, right = v1.configuration(old), v1.configuration(payload)
            old_env = left["spec"]["template"]["spec"]["containers"][0].pop("env")
            new_env = right["spec"]["template"]["spec"]["containers"][0].pop("env")
            self.assertEqual(left, right)
            self.assertEqual({x["name"]:x for x in old_env if x["name"] != m.METRICS_ENV},
                             {x["name"]:x for x in new_env if x["name"] != m.METRICS_ENV})
            self.assertEqual(any(x["name"] == m.METRICS_ENV for x in new_env), enabled)
            self.assertEqual(old, original)

    def test_external_write_conflicts_without_put_retry_or_version_rebase(self):
        plan, _ = fixture(); before = api_service(service(plan)["before"])
        old = v1_service(service(plan)["before"])
        with patch.object(v1, "request", side_effect=[old, c.ContractError("metrics-v1-conflict")]) as request, \
             patch.object(live, "request", return_value=before), self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN, before, service(plan)["after"])
        self.assertEqual(request.call_count, 2)
        self.assertEqual(request.call_args_list[1].args[1]["metadata"]["resourceVersion"], old["metadata"]["resourceVersion"])

    def test_v2_race_after_v1_get_stops_before_any_put(self):
        plan, _ = fixture(); before = api_service(service(plan)["before"])
        for mutate in (lambda x:x.update(etag="raced"), lambda x:x.update(uid="recreated"),
                       lambda x:x["template"]["containers"][0].update(image="other-image")):
            changed = deepcopy(before); mutate(changed)
            with patch.object(v1, "request", return_value=v1_service(service(plan)["before"])) as request, \
                 patch.object(live, "request", return_value=changed), self.assertRaises(c.ContractError):
                live.conditional_update(TOKEN, before, service(plan)["after"])
            self.assertEqual(request.call_count, 1)

    def test_versioned_http_conflict_never_echoes_payload_or_retries(self):
        for code in (409, 412, 403, 500):
            opener = Mock(); opener.open.side_effect = urllib.error.HTTPError(v1.URL, code, "private", {}, io.BytesIO(b"private"))
            with patch.object(v1.urllib.request, "build_opener", return_value=opener), self.assertRaises(c.ContractError) as error:
                v1.request(TOKEN, {"metadata":{"resourceVersion":"opaque"}})
            self.assertNotIn("private", str(error.exception)); self.assertEqual(opener.open.call_count, 1)
            self.assertEqual(str(error.exception), "metrics-v1-conflict" if code in (409,412) else "metrics-v1-api-failed")

    def test_redirect_and_foreign_identity_never_receive_a_put(self):
        with self.assertRaises(c.ContractError):
            live.NoRedirect().redirect_request(None,None,307,"",{},"https://example.test")
        plan, _ = fixture(); before = api_service(service(plan)["before"])
        wrong = v1_service(service(plan)["before"]); wrong["metadata"]["uid"] = "other"
        with patch.object(v1,"request",return_value=wrong) as request, patch.object(live,"request"), self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN,before,service(plan)["after"])
        self.assertEqual(request.call_count,1)

    def test_returned_service_requires_reconciliation_and_unchanged_other_fields(self):
        plan,state = fixture(); before,after=api_service(service(plan)["before"]),api_service(actual(state))
        old,new=v1_service(service(plan)["before"]),v1_service(actual(state))
        broken=deepcopy(new);broken["spec"]["futureField"]="unexpected"
        with patch.object(v1,"request",side_effect=[old,new,broken]), patch.object(live,"request",side_effect=[before,after]), self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN,before,service(plan)["after"])
        pending=deepcopy(after);pending["reconciling"]=True
        with patch.object(v1,"request",side_effect=[old,new]) as request, patch.object(live,"request",side_effect=[before,pending]), \
             patch.object(v1.time,"monotonic",side_effect=[0,301]), self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN,before,service(plan)["after"])
        self.assertEqual(request.call_count,2)

    def test_refresh_only_rejects_any_unrelated_drift_or_remote_write(self):
        plan, state = fixture()
        refresh = refresh_plan(plan, state)
        self.assertEqual(live.validate_refresh(refresh, plan, "runtime_private"), state)
        mutations = (
            lambda p: p.update(complete=False),
            lambda p: p.update(action_invocations=[1]),
            lambda p: p.update(resource_changes=[{"address": r.SERVICE, "change": {"actions": ["update"]}}]),
            lambda p: p["resource_drift"].append(deepcopy(p["resource_drift"][0])),
            lambda p: p["resource_drift"][0]["change"]["before"].update(etag="different-state"),
            lambda p: p["resource_drift"][0]["change"].update(actions=["delete"]),
            lambda p: p["prior_state"]["values"]["root_module"]["child_modules"][0]["resources"][0]["values"].update(display_name="other-change"),
            lambda p: p.update(prior_state={}),
            lambda p: p["planned_values"].update(root_module={"resources":[{}]}),
        )
        for mutation in mutations:
            changed = deepcopy(refresh)
            mutation(changed)
            with self.subTest(mutation=mutation), self.assertRaises(c.ContractError):
                live.validate_refresh(changed, plan, "runtime_private")

    def test_apply_uses_only_saved_refresh_plan_and_rechecks_api_after_state_write(self):
        for enabled in (True, False):
            plan, state = fixture(enabled)
            before, after = api_service(service(plan)["before"]), api_service(actual(state))
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "metrics-state-version.json").write_text(json.dumps(VERSION))
                commands = []
                def run(args, **kwargs):
                    commands.append(args)
                    if args[:2] == ["terraform", "plan"]:
                        self.assertIn("-refresh-only", args)
                        (root / "metrics-refresh.tfplan").write_bytes(b"synthetic-refresh-only-plan")
                        return ""
                    if args[:3] == ["terraform", "show", "-json"]:
                        return json.dumps(refresh_plan(plan, state) if len(args) == 4 else state)
                    self.assertEqual(args, ["terraform", "apply", "-input=false", "-no-color", str(root / "metrics-refresh.tfplan")])
                    return ""
                env = {"MYPAGE_ROLE": "apply", "MYPAGE_GCP_ACCESS_TOKEN": TOKEN, "MYPAGE_PLAN_TIME": str(time.time())}
                with patch.object(c, "context"), patch.object(ops, "run", side_effect=run), patch.object(ops, "state_version", return_value=VERSION), patch.object(live, "conditional_update", return_value=after), patch.object(live, "request", side_effect=[before, after, after]) as request:
                    live.apply_saved(env, root, plan, "runtime_private")
                self.assertEqual(request.call_count, 3)
                self.assertFalse(any("foundation.tfplan" in x for cmd in commands if cmd[1] == "apply" for x in cmd))

    def test_state_version_change_stops_before_mutation(self):
        plan, _ = fixture()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "metrics-state-version.json").write_text(json.dumps(VERSION))
            with patch.object(c, "context"), patch.object(ops, "state_version", return_value=VERSION | {"VersionId": "changed"}), patch.object(live, "request") as request, self.assertRaises(c.ContractError):
                live.apply_saved({"MYPAGE_ROLE": "apply"}, root, plan, "runtime_private")
            request.assert_not_called()

    def test_foundation_entrypoint_never_applies_native_provider_metrics_update(self):
        plan, _ = fixture()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "foundation.tfplan").write_bytes(b"reviewed-plan")
            sha = "a" * 40
            receipt = {"sha256": hashlib.sha256(b"reviewed-plan").hexdigest(), "source": sha}
            (root / "prepared.json").write_text(json.dumps(receipt))
            env = {"MYPAGE_ROLE": "apply", "MYPAGE_MODE": "runtime-private-apply", "GITHUB_SHA": sha,
                   "MYPAGE_PLAN_TIME": str(time.time())}
            with patch.object(c, "context"), patch.object(ops, "directory", return_value=root), patch.object(ops, "run", return_value=json.dumps(plan)) as run, patch.object(live, "apply_saved") as apply:
                ops.apply_saved(env)
            apply.assert_called_once_with(env, root, plan, "runtime_private")
            self.assertFalse(any(call.args[0][:2] == ["terraform", "apply"] for call in run.call_args_list))

    def test_expired_or_tampered_saved_plan_stops_before_conditional_mutation(self):
        plan, _ = fixture()
        for expired in (False, True):
            with tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                (root / "foundation.tfplan").write_bytes(b"plan")
                (root / "prepared.json").write_text(json.dumps({"sha256": "wrong", "source": "a" * 40}))
                env = {"MYPAGE_ROLE": "apply", "MYPAGE_MODE": "runtime-private-apply", "GITHUB_SHA": "a" * 40,
                       "MYPAGE_PLAN_TIME": str(time.time() - (1801 if expired else 1))}
                with patch.object(c, "context"), patch.object(ops, "directory", return_value=root), patch.object(live, "apply_saved") as apply, self.assertRaises(c.ContractError):
                    ops.apply_saved(env)
                apply.assert_not_called()

    def test_readback_entrypoint_checks_live_even_when_saved_state_is_valid(self):
        plan, state = fixture()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "foundation.tfplan").write_bytes(b"plan")
            (root / "prepared.json").write_text(json.dumps({"sha256": hashlib.sha256(b"plan").hexdigest(), "source": "a" * 40}))
            env = {"MYPAGE_ROLE": "apply", "MYPAGE_MODE": "runtime-private-apply", "GITHUB_SHA": "a" * 40,
                   "MYPAGE_GCP_ACCESS_TOKEN": TOKEN}
            response = api_service(actual(state))
            response["trafficStatuses"][0]["revision"] = "mypage-api-before"
            with patch.object(c, "context"), patch.object(ops, "directory", return_value=root), patch.object(ops, "run", side_effect=[json.dumps(plan), json.dumps(state)]), patch.object(live, "request", return_value=response) as request, self.assertRaises(c.ContractError):
                m.readback_saved(env)
            request.assert_called_once_with(TOKEN, live.SERVICE_URL)


if __name__ == "__main__":
    unittest.main()
