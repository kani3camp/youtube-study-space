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
        "updateTime": state["update_time"], "ingress": "INGRESS_TRAFFIC_ALL",
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


def refresh_plan(plan, state):
    return {"format_version": plan["format_version"], "complete": True, "errored": False,
            "planned_values": deepcopy(state["values"]),
            "resource_drift": [{"address": r.SERVICE, "mode": "managed",
                                "provider_name": "registry.terraform.io/hashicorp/google",
                                "change": {"actions": ["update"], "before": deepcopy(service(plan)["before"]),
                                           "after": deepcopy(actual(state)), "after_unknown": {}}}]}


def operation(after):
    return {"name": "projects/" + c.PROJECT + "/locations/asia-southeast2/operations/synthetic-operation",
            "done": True, "response": after | {"@type": "type.googleapis.com/google.cloud.run.v2.Service"}}


class MetricsLiveTest(unittest.TestCase):
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

    def test_single_conditional_patch_preserves_every_other_container_field(self):
        for enabled in (True, False):
            plan, state = fixture(enabled)
            before, after = api_service(service(plan)["before"]), api_service(actual(state))
            original = deepcopy(before)
            with patch.object(live, "request", side_effect=[operation(after), after]) as request:
                self.assertEqual(live.conditional_update(TOKEN, before, service(plan)["after"]), after)
            token, url, body = request.call_args_list[0].args
            self.assertEqual((token, url), (TOKEN, live.PATCH_URL))
            self.assertEqual(set(body), {"name", "etag", "template"})
            self.assertEqual(body["etag"], before["etag"])
            self.assertEqual(body["template"]["revision"], "")
            old_container = deepcopy(before["template"]["containers"][0])
            new_container = deepcopy(body["template"]["containers"][0])
            old_env, new_env = old_container.pop("env"), new_container.pop("env")
            self.assertEqual(old_container, new_container)
            self.assertEqual([x for x in old_env if x["name"] != m.METRICS_ENV],
                             [x for x in new_env if x["name"] != m.METRICS_ENV])
            self.assertEqual(any(x["name"] == m.METRICS_ENV for x in new_env), enabled)
            self.assertEqual(before, original)
            self.assertEqual(request.call_count, 2)

    def test_external_write_after_get_conflicts_without_retry_or_rebase(self):
        plan, _ = fixture()
        before = api_service(service(plan)["before"])
        current_etag = "external-write-after-get"
        def server(token, url, body=None):
            self.assertEqual(url, live.PATCH_URL)
            self.assertNotEqual(body["etag"], current_etag)
            raise c.ContractError("metrics-live-conflict")
        with patch.object(live, "request", side_effect=server) as request, self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN, before, service(plan)["after"])
        self.assertEqual(request.call_count, 1)

    def test_http_conflict_and_auth_failures_never_echo_payload_or_retry(self):
        for code in (409, 412, 403, 500):
            opener = Mock()
            opener.open.side_effect = urllib.error.HTTPError(live.PATCH_URL, code, "private-error-body", {}, io.BytesIO(b"private"))
            with patch.object(live.urllib.request, "build_opener", return_value=opener), self.assertRaises(c.ContractError) as caught:
                live.request(TOKEN, live.PATCH_URL, {"etag": "source", "private": "never-print"})
            self.assertNotIn("private", str(caught.exception))
            self.assertEqual(opener.open.call_count, 1)
            expected = "metrics-live-conflict" if code in (409, 412) else "metrics-live-api-failed"
            self.assertEqual(str(caught.exception), expected)

    def test_redirect_or_foreign_operation_never_receives_credentials(self):
        with self.assertRaises(c.ContractError):
            live.NoRedirect().redirect_request(None, None, 307, "", {}, "https://example.test")
        for url in ("https://example.test", live.SERVICE_URL + ":delete", live.SERVICE_URL.replace(c.PROJECT, "youtube-study-space")):
            with self.subTest(url=url), self.assertRaises(c.ContractError):
                live.request(TOKEN, url)
        plan, _ = fixture()
        before = api_service(service(plan)["before"])
        with patch.object(live, "request", return_value={"name": "projects/foreign/locations/asia-southeast2/operations/abc"}) as request, self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN, before, service(plan)["after"])
        self.assertEqual(request.call_count, 1)

    def test_operation_error_and_timeout_do_not_retry_the_patch(self):
        plan, state = fixture()
        before, after = api_service(service(plan)["before"]), api_service(actual(state))
        failed = operation(after)
        failed.pop("response")
        failed["error"] = {"code": 10, "message": "private"}
        with patch.object(live, "request", return_value=failed) as request, self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN, before, service(plan)["after"])
        self.assertEqual(request.call_count, 1)
        pending = {"name": operation(after)["name"]}
        with patch.object(live, "request", return_value=pending) as request, patch.object(live.time, "monotonic", side_effect=[0, 301]), self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN, before, service(plan)["after"])
        self.assertEqual(request.call_count, 1)

    def test_completed_operation_still_requires_current_live_service(self):
        plan, state = fixture()
        before, after = api_service(service(plan)["before"]), api_service(actual(state))
        rolled_back = deepcopy(after)
        rolled_back["trafficStatuses"][0]["revision"] = "mypage-api-before"
        with patch.object(live, "request", side_effect=[operation(after), rolled_back]), self.assertRaises(c.ContractError):
            live.conditional_update(TOKEN, before, service(plan)["after"])

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
            lambda p: p["planned_values"]["root_module"]["child_modules"][0]["resources"][0]["values"].update(display_name="other-change"),
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
                with patch.object(c, "context"), patch.object(ops, "run", side_effect=run), patch.object(ops, "state_version", return_value=VERSION), patch.object(live, "request", side_effect=[before, operation(after), after, after, after]) as request:
                    live.apply_saved(env, root, plan, "runtime_private")
                self.assertEqual(request.call_count, 5)
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
