"""Metrics-only update and readback contracts; synthetic values, no cloud I/O."""
from copy import deepcopy
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "infra/mypage-platform/scripts"))
import foundation_contract as c
import runtime_contract as r
import read_metrics_transition as m


def fixture(enabled=True, stage="runtime_private"):
    value = r.settings({
        "image_digest": r.REPOSITORY + "@sha256:" + "a" * 64,
        "project_number": c.PROJECT_NUMBER, "web_app_id": "1:" + c.PROJECT_NUMBER + ":web:synthetic",
        "public_origin": r.ORIGIN, "privacy_version": "synthetic-p", "terms_version": "synthetic-t",
        "oauth_client_id_version": "1", "oauth_client_secret_version": "2", "youtube_api_key_version": "3",
        "maintenance_operator_enabled": True,
        "maintenance_operator_image_digest": r.REPOSITORY + "@sha256:" + "b" * 64,
        "maintenance_operator_image_source_sha": "c" * 40,
        "read_metrics_enabled": enabled, "read_metrics_from_revision": "mypage-api-before",
    })
    before_value = value | {"read_metrics_enabled": not enabled}
    env = [{"name": k, "value": v} for k, v in r.plain_environment(before_value).items()]
    for name, secret, field in (("MYPAGE_OAUTH_CLIENT_ID", "mypage-oauth-client-id", "oauth_client_id_version"),
                                ("MYPAGE_OAUTH_CLIENT_SECRET", "mypage-oauth-client-secret", "oauth_client_secret_version"),
                                ("MYPAGE_YOUTUBE_API_KEY", "mypage-youtube-api-key", "youtube_api_key_version")):
        env.append({"name": name, "value_source": [{"secret_key_ref": [{"secret": secret, "version": value[field]}]}]})
    resources = r.resource_contract(stage, value)
    for address in (r.MAINTENANCE_ROLE, r.MAINTENANCE_BOOTSTRAP_ROLE):
        resources[address]["deleted"] = False
    resources[r.SERVICE].update(
        scaling=[{"min_instance_count": 0, "max_instance_count": 1, "scaling_mode": "AUTOMATIC"}],
        traffic=[{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 100}],
        template=[{"revision": "", "service_account": c.EMAIL, "timeout": "30s", "max_instance_request_concurrency": 40,
                   "execution_environment": "EXECUTION_ENVIRONMENT_GEN2", "scaling": [{"min_instance_count": 0, "max_instance_count": 1}],
                   "containers": [{"image": value["image_digest"], "env": env,
                                   "ports": [{"name": "http1", "container_port": 8080}],
                                   "resources": [{"limits": {"cpu": "1", "memory": "512Mi"}, "cpu_idle": True, "startup_cpu_boost": False}],
                                   "startup_probe": [{"initial_delay_seconds": 0, "timeout_seconds": 1, "period_seconds": 10,
                                                      "failure_threshold": 3, "tcp_socket": [{"port": 8080}]}]}]}],
        generation="7", observed_generation="7", update_time="2026-10-23T00:00:00Z",
        latest_ready_revision="mypage-api-before", latest_created_revision="mypage-api-before",
        terminal_condition=[{"state": "CONDITION_SUCCEEDED"}], reconciling=False,
        traffic_statuses=[{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 100, "revision": "mypage-api-before"}],
    )
    changes = [{"address": address, "mode": "managed", "provider_name": "registry.terraform.io/hashicorp/google",
                "change": {"actions": ["no-op"], "before": deepcopy(data), "after": deepcopy(data), "after_unknown": {}}}
               for address, data in resources.items()]
    plan = {"format_version": "1.2", "complete": True, "errored": False,
            "variables": {"stage": {"value": stage}, "runtime_settings": {"value": value}}, "resource_changes": changes}
    change = service(plan)
    change["actions"] = ["update"]
    new_env = m.container(change["after"])["env"]
    new_env[:] = [x for x in new_env if x["name"] != m.METRICS_ENV]
    if enabled:
        new_env.insert(1, {"name": m.METRICS_ENV, "value": "true"})
    # Match Terraform's incomplete planned values for computed attributes.
    for key in m.COMPUTED:
        change["after"].pop(key, None)
        change["after_unknown"][key] = True
    return plan


def service(plan):
    return next(x["change"] for x in plan["resource_changes"] if x["address"] == r.SERVICE)


def value(plan):
    return plan["variables"]["runtime_settings"]["value"]


def applied_state(plan):
    items = [{"address": x["address"], "mode": x["mode"], "provider_name": x["provider_name"],
              "values": deepcopy(x["change"]["after"])} for x in plan["resource_changes"]]
    current = next(x["values"] for x in items if x["address"] == r.SERVICE)
    current.update(generation="8", observed_generation="8", update_time="2026-10-23T01:00:00Z",
                   latest_ready_revision="mypage-api-after", latest_created_revision="mypage-api-after",
                   terminal_condition=[{"state": "CONDITION_SUCCEEDED"}], reconciling=False,
                   traffic_statuses=[{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 100, "revision": "mypage-api-after"}])
    return {"values": {"root_module": {"child_modules": [{"resources": items}]}}}


def actual(state):
    return next(x["values"] for x in state["values"]["root_module"]["child_modules"][0]["resources"] if x["address"] == r.SERVICE)


class MetricsTransitionTest(unittest.TestCase):
    def test_enable_disable_private_public_and_no_plan_mutation(self):
        for enabled in (True, False):
            for stage, count in (("runtime_private", 28), ("runtime_public", 29)):
                with self.subTest(enabled=enabled, stage=stage):
                    plan = fixture(enabled, stage)
                    original = deepcopy(plan)
                    self.assertEqual(r.validate_plan(plan, stage), {"resources": count, "creates": 0, "updates": 1})
                    self.assertEqual(plan, original)
                    state = applied_state(plan)
                    m.validate_readback(state, plan, stage)
                    self.assertEqual(plan, original)

    def test_undeclared_update_and_noop_transition_are_rejected(self):
        plan = fixture()
        value(plan)["read_metrics_from_revision"] = ""
        with self.assertRaises(c.ContractError):
            r.validate_plan(plan, "runtime_private")
        plan = fixture()
        service(plan)["actions"] = ["no-op"]
        with self.assertRaises(c.ContractError):
            r.validate_plan(plan, "runtime_private")

    def test_revision_declaration_requires_exact_string(self):
        for revision in (None, True, 1, [], {}, "", "other-api-ready", "../mypage-api-ready", "mypage-api-old\n"):
            raw = value(fixture()) | {"read_metrics_from_revision": revision}
            if revision == "":
                self.assertEqual(r.settings(raw)["read_metrics_from_revision"], "")
            else:
                with self.subTest(revision=revision), self.assertRaises(c.ContractError):
                    r.settings(raw)

    def test_stale_or_unready_source_is_rejected(self):
        for key, invalid in (("latest_ready_revision", "mypage-api-stale"),
                             ("latest_created_revision", "mypage-api-stale"),
                             ("reconciling", True), ("reconciling", None),
                             ("observed_generation", "6"), ("generation", "invalid"),
                             ("terminal_condition", [{"state": "CONDITION_FAILED"}]),
                             ("traffic_statuses", [{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 99, "revision": "mypage-api-before"}])):
            plan = fixture()
            service(plan)["before"][key] = invalid
            with self.subTest(key=key), self.assertRaises(c.ContractError):
                r.validate_plan(plan, "runtime_private")

    def test_even_same_suffix_foreign_revision_is_rejected(self):
        plan = fixture()
        service(plan)["before"]["latest_ready_revision"] = "projects/other/locations/asia-southeast2/services/mypage-api/revisions/mypage-api-before"
        with self.assertRaises(c.ContractError):
            r.validate_plan(plan, "runtime_private")
        plan = fixture()
        prefix = "projects/" + c.PROJECT + "/locations/asia-southeast2/services/mypage-api/revisions/"
        for key in ("latest_ready_revision", "latest_created_revision"):
            service(plan)["before"][key] = prefix + "mypage-api-before"
        self.assertEqual(r.validate_plan(plan, "runtime_private")["updates"], 1)

    def test_configuration_changes_cannot_hide_with_the_toggle(self):
        mutations = (
            lambda x: x.update(project="youtube-study-space"),
            lambda x: x.update(ingress="INGRESS_TRAFFIC_INTERNAL_ONLY"),
            lambda x: x.update(invoker_iam_disabled=True),
            lambda x: x.update(custom_audiences=["extra"]),
            lambda x: x.update(description="unreviewed"),
            lambda x: x.update(default_uri_disabled=True),
            lambda x: x["traffic"][0].update(percent=50),
            lambda x: x["scaling"][0].update(max_instance_count=2),
            lambda x: x["template"][0].update(service_account="other@example.test"),
            lambda x: x["template"][0].update(revision="mypage-api-pinned"),
            lambda x: x["template"][0].update(timeout="60s"),
            lambda x: m.container(x).update(image=r.REPOSITORY + "@sha256:" + "b" * 64),
            lambda x: m.container(x)["env"][0].update(value="other-project"),
            lambda x: m.container(x)["env"][-1]["value_source"][0]["secret_key_ref"][0].update(version="4"),
        )
        for mutation in mutations:
            plan = fixture()
            mutation(service(plan)["after"])
            with self.subTest(mutation=mutation), self.assertRaises(c.ContractError):
                r.validate_plan(plan, "runtime_private")

    def test_configuration_unknowns_remain_fail_closed(self):
        for mask in ({"template": True}, {"template": [{"containers": [{"env": True}]}]},
                     {"ingress": True}, {"new_attribute": True}, {"uid": True}):
            plan = fixture()
            service(plan)["after_unknown"].update(mask)
            with self.subTest(mask=mask), self.assertRaises(c.ContractError):
                r.validate_plan(plan, "runtime_private")

    def test_toggle_must_change_only_the_expected_boolean(self):
        for enabled in (True, False):
            plan = fixture(enabled)
            value(plan)["read_metrics_enabled"] = not enabled
            with self.assertRaises(c.ContractError):
                r.validate_plan(plan, "runtime_private")
        for mutation in (lambda env: env.append({"name": m.METRICS_ENV, "value": "true"}),
                         lambda env: env[1].update(value="false"),
                         lambda env: env[1].update(value_source=[{"secret_key_ref": [{"secret": "unreviewed", "version": "1"}]}])):
            plan = fixture()
            mutation(m.container(service(plan)["after"])["env"])
            with self.assertRaises(c.ContractError):
                r.validate_plan(plan, "runtime_private")

    def test_other_resource_mutation_import_or_inventory_change_is_rejected(self):
        for actions in (["create"], ["update"], ["delete"], ["delete", "create"], ["read"]):
            plan = fixture()
            plan["resource_changes"][0]["change"]["actions"] = actions
            with self.subTest(actions=actions), self.assertRaises(c.ContractError):
                r.validate_plan(plan, "runtime_private")
        for mutation in (
            lambda p: p["resource_changes"].pop(0),
            lambda p: p["resource_changes"].append(deepcopy(p["resource_changes"][0])),
            lambda p: p["resource_changes"][0]["change"]["after"].update(display_name="unreviewed"),
            lambda p: service(p).update(importing={"id": "arbitrary"}),
            lambda p: service(p).update(replace_paths=[["template"]]),
            lambda p: service(p).update(actions=["delete", "create"]),
        ):
            plan = fixture()
            mutation(plan)
            with self.assertRaises(c.ContractError):
                r.validate_plan(plan, "runtime_private")

    def test_drift_and_incomplete_plans_are_not_accepted(self):
        for values in ({"resource_drift": [{"address": r.SERVICE}]}, {"complete": False},
                       {"errored": True}, {"deferred_changes": [1]}, {"action_invocations": [1]},
                       {"checks": [{"status": "unknown"}]}):
            with self.subTest(values=values), self.assertRaises(c.ContractError):
                r.validate_plan(fixture() | values, "runtime_private")

    def test_full_plan_commitment_binds_revision_and_operation(self):
        plan = fixture()
        key = "synthetic-test-key-which-is-at-least-32-bytes"
        first = c.commitment(plan, "state-7", "a" * 40, key)
        changed = deepcopy(plan)
        value(changed)["read_metrics_from_revision"] = "mypage-api-other"
        self.assertNotEqual(first, c.commitment(changed, "state-7", "a" * 40, key))
        self.assertNotEqual(first, c.commitment(plan, "state-8", "a" * 40, key))
        self.assertNotEqual(first, c.commitment(fixture(False), "state-7", "a" * 40, key))

    def test_readback_rejects_stale_partial_or_unexpected_results(self):
        for mutation in (
            lambda s: actual(s).update(latest_ready_revision="mypage-api-before", latest_created_revision="mypage-api-before"),
            lambda s: actual(s).update(generation="9", observed_generation="9"),
            lambda s: actual(s).update(reconciling=True),
            lambda s: actual(s).update(update_time="2026-10-22T00:00:00Z"),
            lambda s: m.container(actual(s))["env"].pop(1),
            lambda s: actual(s)["traffic_statuses"][0].update(percent=50),
            lambda s: s["values"]["root_module"]["child_modules"][0]["resources"].pop(0),
            lambda s: s["values"]["root_module"]["child_modules"][0]["resources"][0]["values"].update(display_name="unexpected"),
        ):
            plan = fixture()
            state = applied_state(plan)
            mutation(state)
            with self.subTest(mutation=mutation), self.assertRaises(c.ContractError):
                m.validate_readback(state, plan, "runtime_private")

    def test_rollback_binds_newly_observed_revision_not_the_old_plan(self):
        plan = fixture(False)
        value(plan)["read_metrics_from_revision"] = "mypage-api-enabled"
        before = service(plan)["before"]
        before.update(latest_ready_revision="mypage-api-enabled", latest_created_revision="mypage-api-enabled")
        before["traffic_statuses"][0]["revision"] = "mypage-api-enabled"
        self.assertEqual(r.validate_plan(plan, "runtime_private")["updates"], 1)
        value(plan)["read_metrics_from_revision"] = "mypage-api-before"
        with self.assertRaises(c.ContractError):
            r.validate_plan(plan, "runtime_private")

    def test_errors_do_not_echo_rejected_configuration(self):
        plan = fixture()
        m.container(service(plan)["after"])["env"][0]["value"] = "synthetic-sensitive-value"
        with self.assertRaises(c.ContractError) as caught:
            r.validate_plan(plan, "runtime_private")
        self.assertNotIn("synthetic-sensitive-value", str(caught.exception))


if __name__ == "__main__":
    unittest.main()
