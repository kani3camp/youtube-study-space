"""Strict DEV metrics opt-in; never authorize a serving update or cloud I/O."""
from copy import deepcopy
import json
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "infra/mypage-platform/scripts"))
import foundation_contract as c
import runtime_contract as r

METRICS = "MYPAGE_READ_METRICS"


def settings_fixture():
    return json.loads((ROOT / "infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json").read_text())


def service_change(plan):
    return next(item["change"] for item in plan["resource_changes"] if item["address"] == r.SERVICE)


def environment(plan):
    return service_change(plan)["after"]["template"][0]["containers"][0]["env"]


def plan_fixture(enabled, stage="runtime_private", action="no-op"):
    value = r.settings(settings_fixture() | {"read_metrics_enabled": enabled})
    resources = r.resource_contract(stage)
    env = [{"name": key, "value": val} for key, val in r.plain_environment(value).items()]
    for name, secret, field in (
        ("MYPAGE_OAUTH_CLIENT_ID", "mypage-oauth-client-id", "oauth_client_id_version"),
        ("MYPAGE_OAUTH_CLIENT_SECRET", "mypage-oauth-client-secret", "oauth_client_secret_version"),
        ("MYPAGE_YOUTUBE_API_KEY", "mypage-youtube-api-key", "youtube_api_key_version"),
    ):
        env.append({"name": name, "value_source": [{"secret_key_ref": [{"secret": secret, "version": value[field]}]}]})
    resources[r.SERVICE].update(
        scaling=[{"min_instance_count": 0, "max_instance_count": 1, "scaling_mode": "AUTOMATIC"}],
        traffic=[{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 100}],
        template=[{
            "service_account": c.EMAIL, "timeout": "30s", "max_instance_request_concurrency": 40,
            "execution_environment": "EXECUTION_ENVIRONMENT_GEN2",
            "scaling": [{"min_instance_count": 0, "max_instance_count": 1}],
            "containers": [{
                "image": value["image_digest"], "ports": [{"name": "http1", "container_port": 8080}],
                "resources": [{"limits": {"cpu": "1", "memory": "512Mi"}, "cpu_idle": True, "startup_cpu_boost": False}],
                "startup_probe": [{"initial_delay_seconds": 0, "timeout_seconds": 1, "period_seconds": 10,
                                   "failure_threshold": 3, "tcp_socket": [{"port": 8080}]}],
                "env": env,
            }],
        }],
    )
    return {
        "format_version": "1.2", "complete": True, "errored": False,
        "variables": {"stage": {"value": stage}, "runtime_settings": {"value": value}},
        "resource_changes": [
            {"address": address, "mode": "managed", "provider_name": "registry.terraform.io/hashicorp/google",
             "change": {"actions": ["no-op" if address == r.INVOKER else action],
                        "after": after, "after_unknown": {}}}
            for address, after in resources.items()
        ],
    }


class ReadMetricsTest(unittest.TestCase):
    def test_omission_and_explicit_false_preserve_existing_environment(self):
        raw = settings_fixture()
        original = deepcopy(raw)
        implicit = r.settings(raw)
        explicit = r.settings(raw | {"read_metrics_enabled": False})
        self.assertEqual(raw, original)
        self.assertIs(implicit["read_metrics_enabled"], False)
        self.assertEqual(implicit, explicit)
        self.assertNotIn(METRICS, r.plain_environment(implicit))
        enabled = r.settings(raw | {"read_metrics_enabled": True})
        self.assertEqual(r.plain_environment(enabled), r.plain_environment(implicit) | {METRICS: "true"})

    def test_json_opt_in_requires_an_actual_boolean_without_echoing_input(self):
        for invalid in ("true", "false", "private-input-sentinel", 0, 1, 1.0, None, [], {}):
            with self.subTest(value=invalid), self.assertRaisesRegex(c.ContractError, "^runtime-read-metrics-type$"):
                r.settings(settings_fixture() | {"read_metrics_enabled": invalid})
        with self.assertRaisesRegex(c.ContractError, "^runtime-settings-types$"):
            r.settings(settings_fixture() | {"read_metrics_enabled": True, "privacy_version": True})
        with self.assertRaisesRegex(c.ContractError, "^runtime-settings-fields$"):
            r.settings(settings_fixture() | {"read_metrics_enabled": True, "unreviewed": "private-input-sentinel"})

    def test_matching_create_and_noop_plans_preserve_resource_inventory(self):
        for enabled in (False, True):
            for stage in ("runtime_private", "runtime_public"):
                for action in ("create", "no-op"):
                    with self.subTest(enabled=enabled, stage=stage, action=action):
                        plan = plan_fixture(enabled, stage, action)
                        original = deepcopy(plan)
                        self.assertEqual(r.validate_plan(plan, stage), {
                            "resources": 12 if stage == "runtime_public" else 11,
                            "creates": 11 if action == "create" else 0,
                        })
                        self.assertEqual(plan, original)
                        self.assertEqual([e for e in environment(plan) if e["name"] == METRICS],
                                         [{"name": METRICS, "value": "true"}] if enabled else [])

    def test_unrequested_missing_duplicate_or_secret_backed_metrics_are_rejected(self):
        for stage in ("runtime_private", "runtime_public"):
            unrequested = plan_fixture(False, stage)
            environment(unrequested).append({"name": METRICS, "value": "true"})
            with self.assertRaises(c.ContractError):
                r.validate_plan(unrequested, stage)
            for replacement in ([], [{"name": METRICS, "value": "true"}] * 2,
                                [{"name": METRICS, "value": "false"}], [{"name": METRICS, "value": "TRUE"}],
                                [{"name": METRICS, "value": True}],
                                [{"name": METRICS, "value_source": [{"secret_key_ref": [{"secret": "private", "version": "1"}]}]}],
                                [{"name": METRICS, "value": "true", "value_source": [{"secret_key_ref": [{"secret": "private", "version": "1"}]}]}]):
                with self.subTest(stage=stage, replacement=replacement):
                    plan = plan_fixture(True, stage)
                    env = environment(plan)
                    env[:] = [entry for entry in env if entry["name"] != METRICS] + replacement
                    with self.assertRaises(c.ContractError):
                        r.validate_plan(plan, stage)

    def test_unknown_environment_or_production_target_is_rejected(self):
        for stage in ("runtime_private", "runtime_public"):
            unknown = plan_fixture(True, stage)
            service_change(unknown)["after_unknown"] = {"template": [{"containers": [{"env": True}]}]}
            with self.assertRaisesRegex(c.ContractError, "^runtime-config-unknown$"):
                r.validate_plan(unknown, stage)
            wrong_project = plan_fixture(True, stage)
            service_change(wrong_project)["after"]["project"] = "youtube-study-space"
            with self.assertRaises(c.ContractError):
                r.validate_plan(wrong_project, stage)
            wrong_env = plan_fixture(True, stage)
            next(e for e in environment(wrong_env) if e["name"] == "MYPAGE_ENVIRONMENT")["value"] = "production"
            with self.assertRaises(c.ContractError):
                r.validate_plan(wrong_env, stage)

    def test_metrics_opt_in_does_not_authorize_serving_updates_or_replacements(self):
        for stage in ("runtime_private", "runtime_public"):
            for enabled in (False, True):
                for actions in (["update"], ["delete", "create"], ["create", "delete"], ["delete"]):
                    with self.subTest(stage=stage, enabled=enabled, actions=actions):
                        plan = plan_fixture(enabled, stage)
                        service_change(plan)["actions"] = actions
                        with self.assertRaisesRegex(c.ContractError, "^runtime-resource-action$"):
                            r.validate_plan(plan, stage)

    def test_metrics_drift_cannot_be_disguised_as_empty_metadata_refresh(self):
        for stage in ("runtime_private", "runtime_public"):
            for enabled in (False, True):
                plan = plan_fixture(enabled, stage)
                before = deepcopy(service_change(plan)["after"])
                old_env = before["template"][0]["containers"][0]["env"]
                if enabled:
                    old_env[:] = [entry for entry in old_env if entry["name"] != METRICS]
                else:
                    old_env.append({"name": METRICS, "value": "true"})
                drift = deepcopy(next(item for item in plan["resource_changes"] if item["address"] == r.SERVICE))
                drift["change"].update(actions=["update"], before=before)
                plan["resource_drift"] = [drift]
                with self.subTest(stage=stage, enabled=enabled), self.assertRaises(c.ContractError):
                    r.validate_plan(plan, stage)

    def test_opt_in_changes_the_reviewed_plan_commitment(self):
        key = "synthetic-test-commitment-key-only-123456789"
        off = plan_fixture(False)
        on = plan_fixture(True)
        self.assertNotEqual(c.commitment(off, "state-version", "a" * 40, key),
                            c.commitment(on, "state-version", "a" * 40, key))


if __name__ == "__main__":
    unittest.main()
