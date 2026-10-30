"""ランタイムの拒否と、ソース完全一致のイメージ受入を検証する。クラウド入出力は行わない。"""
from copy import deepcopy
import json
from pathlib import Path
import sys
import unittest

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "infra/mypage-platform/scripts"))
import foundation_contract as c
import image_ops
import runtime_contract as r


def fixture(stage="runtime_private"):
    value = r.settings(json.loads((ROOT / "infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json").read_text()))
    after = r.resource_contract(stage, value)
    container = {"image": value["image_digest"], "ports": [{"name": "http1", "container_port": 8080}],
                 "resources": [{"limits": {"cpu": "1", "memory": "512Mi"}, "cpu_idle": True, "startup_cpu_boost": False}],
                 "startup_probe": [{"initial_delay_seconds": 0, "timeout_seconds": 1, "period_seconds": 10,
                                    "failure_threshold": 3, "tcp_socket": [{"port": 8080}]}],
                 "env": [{"name": k, "value": v} for k, v in r.plain_environment(value).items()]}
    for name, secret, field in (("MYPAGE_OAUTH_CLIENT_ID", "mypage-oauth-client-id", "oauth_client_id_version"),
                                ("MYPAGE_OAUTH_CLIENT_SECRET", "mypage-oauth-client-secret", "oauth_client_secret_version"),
                                ("MYPAGE_YOUTUBE_API_KEY", "mypage-youtube-api-key", "youtube_api_key_version")):
        container["env"].append({"name": name, "value_source": [{"secret_key_ref": [{"secret": secret, "version": value[field]}]}]})
    after[r.SERVICE].update(scaling=[{"min_instance_count": 0, "max_instance_count": 1, "scaling_mode": "AUTOMATIC"}],
                            traffic=[{"type": "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST", "percent": 100}],
                            template=[{"service_account": c.EMAIL, "timeout": "30s", "max_instance_request_concurrency": 40,
                                       "execution_environment": "EXECUTION_ENVIRONMENT_GEN2", "scaling": [{"min_instance_count": 0, "max_instance_count": 1}],
                                       "containers": [container]}])
    result = {"format_version": "1.2", "complete": True, "errored": False,
            "variables": {"stage": {"value": stage}, "runtime_settings": {"value": value}},
            "resource_changes": [{"address": address, "mode": "managed", "provider_name": "registry.terraform.io/hashicorp/google",
                                  "change": {"actions": (["create"] if address in r.maintenance_resource_contract() else ["no-op"]) if stage == "maintenance_upgrade" else (["no-op"] if address in r.maintenance_resource_contract() else ["create"]), "after": values, "after_unknown": {}}} for address, values in after.items()]}
    for item in result["resource_changes"]:
        if item["address"] in {r.MAINTENANCE_ROLE} and item["change"]["actions"] == ["no-op"]:
            item["change"]["after"]["deleted"] = False
    if stage == "runtime_public":
        change = next(x['change'] for x in result['resource_changes'] if x['address'] == r.INVOKER)
        change.update(actions=['no-op'], importing={'id': r.PUBLIC_INVOKER_IMPORT_ID})
    return result


def service(plan):
    return next(item["change"] for item in plan["resource_changes"] if item["address"] == r.SERVICE)


class RuntimeTest(unittest.TestCase):
    def test_maintenance_upgrade_preserves_existing_public_runtime_and_adds_only_maintenance(self):
        plan = fixture("maintenance_upgrade")
        maintenance = set(r.maintenance_resource_contract())
        self.assertEqual(r.validate_plan(plan, "maintenance_upgrade"), {"resources": len(plan["resource_changes"]), "creates": len(maintenance)})
        for item in plan["resource_changes"]:
            expected = ["create"] if item["address"] in maintenance else ["no-op"]
            self.assertEqual(item["change"]["actions"], expected)
        for actions in (["create"], ["update"], ["delete", "create"]):
            rejected = deepcopy(plan)
            service(rejected)["actions"] = actions
            with self.subTest(actions=actions), self.assertRaisesRegex(c.ContractError, "maintenance-upgrade-runtime-noop|runtime-resource-action"):
                r.validate_plan(rejected, "maintenance_upgrade")
        rejected = deepcopy(plan)
        next(item for item in rejected["resource_changes"] if item["address"] == r.MAINTENANCE_JOB)["change"]["actions"] = ["update"]
        with self.assertRaisesRegex(c.ContractError, "runtime-resource-action"):
            r.validate_plan(rejected, "maintenance_upgrade")

    def test_maintenance_wif_and_iam_capabilities_are_exact(self):
        mutations = (
            (r.MAINTENANCE_POOL,
             lambda item: item["change"]["after"].update(mode="TRUST_DOMAIN"),
             "runtime-resource-value-" + r.MAINTENANCE_POOL + "-mode"),
            (r.MAINTENANCE_POOL,
             lambda item: item["change"].update(after_unknown={"mode": True}),
             "runtime-resource-value-" + r.MAINTENANCE_POOL + "-mode"),
            (r.MAINTENANCE_PROVIDER,
             lambda item: item["change"]["after"]["oidc"][0].update(allowed_audiences=["https://example.invalid"]),
             "maintenance-provider-issuer"),
            (r.MAINTENANCE_PROVIDER,
             lambda item: item["change"].update(after_unknown={"oidc": [{"allowed_audiences": True}]}),
             "maintenance-provider-unknown"),
            (r.MAINTENANCE_PROVIDER,
             lambda item: item["change"]["after"]["oidc"][0].update(jwks_json="{}"),
             "runtime-unsupported-option"),
            (r.MAINTENANCE_WIF,
             lambda item: item["change"]["after"].update(condition=[{"expression": "true"}]),
             "runtime-resource-after"),
            (r.MAINTENANCE_FIRESTORE,
             lambda item: item["change"]["after"].update(condition=[{"expression": "true"}]),
             "runtime-resource-after"),
            (r.MAINTENANCE_JOB_INVOKER,
             lambda item: item["change"].update(after_unknown={"condition": True}),
             "maintenance-iam-unknown"),
        )
        for address, mutate, check in mutations:
            rejected = fixture("maintenance_upgrade")
            item = next(item for item in rejected["resource_changes"] if item["address"] == address)
            mutate(item)
            with self.subTest(address=address, check=check), self.assertRaises(c.ContractError) as error:
                r.validate_plan(rejected, "maintenance_upgrade")
            self.assertEqual(str(error.exception), check)

    def test_noop_revision_must_equal_the_exact_observed_ready_revision(self):
        cases = (
            ("runtime_private", r.SERVICE, "mypage-api-ready", "runtime-owned-revision"),
        )
        for stage, address, ready, check in cases:
            plan = fixture(stage)
            item = next(item for item in plan["resource_changes"] if item["address"] == address)
            item["change"]["actions"] = ["no-op"]
            after = item["change"]["after"]
            after["latest_ready_revision"] = ready
            after["template"][0]["revision"] = ready
            after["traffic"][0]["revision"] = ready
            with self.subTest(address=address, revision="ready"):
                r.validate_plan(plan, stage)

            rejected = deepcopy(plan)
            changed = next(item for item in rejected["resource_changes"] if item["address"] == address)
            changed["change"]["after"]["template"][0]["revision"] = ready + "-pinned"
            with self.subTest(address=address, revision="pinned"), self.assertRaises(c.ContractError) as error:
                r.validate_plan(rejected, stage)
            self.assertEqual(str(error.exception), check)

    def test_private_and_public_have_exact_resources_and_pinned_secrets(self):
        for stage in ("runtime_private", "runtime_public"):
            plan = fixture(stage)
            size = len(plan["resource_changes"])
            self.assertEqual(r.validate_plan(plan, stage), {"resources": size, "creates": 11})
            for item in plan["resource_changes"]:
                item["change"]["actions"] = ["no-op"]
            self.assertEqual(r.validate_plan(plan, stage), {"resources": size, "creates": 0})

    def test_runtime_cannot_create_the_operator_or_bootstrap_boundary(self):
        for address in r.maintenance_resource_contract():
            plan = fixture()
            next(item for item in plan["resource_changes"] if item["address"] == address)["change"]["actions"] = ["create"]
            with self.subTest(address=address), self.assertRaisesRegex(c.ContractError, "^runtime-maintenance-bootstrap-complete$"):
                r.validate_plan(plan, "runtime_private")

    def test_only_exact_owner_public_member_can_be_imported_without_cloud_writes(self):
        plan = fixture('runtime_public')
        self.assertEqual(r.public_invoker_import_config(), 'import {\n  to = ' + r.INVOKER + '\n  id = "' + r.PUBLIC_INVOKER_IMPORT_ID + '"\n}\n')
        qualified = deepcopy(plan)
        next(x['change'] for x in qualified['resource_changes'] if x['address'] == r.INVOKER)['after']['name'] = r.PUBLIC_INVOKER_NAME
        self.assertEqual(r.validate_plan(qualified, 'runtime_public'), {'resources': len(qualified['resource_changes']), 'creates': 11})
        for mutate in (
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER).update(actions=['create']),
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER).update(importing={'id': 'private-other-resource'}),
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER)['after'].update(role='roles/run.admin'),
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER)['after'].update(member='allAuthenticatedUsers'),
        ):
            rejected = deepcopy(plan)
            mutate(rejected)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(rejected, 'runtime_public')
        key = 'private-key-for-synthetic-tests-only'
        other = deepcopy(plan)
        next(x['change'] for x in other['resource_changes'] if x['address'] == r.INVOKER).pop('importing')
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(other, 'state-version', 'a' * 40, key))

    def test_ci_owned_image_drift_allows_multiple_revisions_without_terraform_update(self):
        plan = fixture("runtime_public")
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["no-op"]
            item["change"].pop("importing", None)
        settings = plan["variables"]["runtime_settings"]["value"]
        settings["hosting_backend_host"] = "mypage-api-observed-et.a.run.app"
        after = service(plan)["after"]
        after["template"][0]["containers"][0]["env"].append({
            "name": "MYPAGE_HOSTING_BACKEND_HOST", "value": settings["hosting_backend_host"]
        })
        before = deepcopy(after)
        before.update(uid="fixed-service-uid", generation="1")
        after.update(uid="fixed-service-uid", generation="6")
        after["template"][0]["containers"][0]["image"] = r.REPOSITORY + "@sha256:" + "c" * 64
        drift = deepcopy(next(item for item in plan["resource_changes"] if item["address"] == r.SERVICE))
        drift["change"].update(actions=["update"], before=before)
        plan["resource_drift"] = [drift]
        saved = deepcopy(plan)
        self.assertEqual(r.validate_plan(plan, "runtime_public"),
                         {"resources": len(plan["resource_changes"]), "creates": 0})
        self.assertEqual(plan, saved)
        # Once Terraform has refreshed the state, it must not restore the old
        # bootstrap digest on a subsequent infrastructure Plan either.
        stable = deepcopy(plan)
        stable.pop("resource_drift")
        self.assertEqual(r.validate_plan(stable, "runtime_public"),
                         {"resources": len(plan["resource_changes"]), "creates": 0})
        def bad(mutator):
            candidate = deepcopy(plan)
            mutator(candidate)
            with self.assertRaises(ValueError):
                r.validate_plan(candidate, "runtime_public")
        bad(lambda p: p["resource_drift"][0]["change"]["before"]["template"][0]["containers"][0]["env"][0].update(value="injected"))
        bad(lambda p: p["resource_drift"][0]["change"]["before"].update(uid="other-service"))
        bad(lambda p: p["resource_drift"][0]["change"]["after"].update(generation="1"))
        bad(lambda p: p["resource_drift"][0]["change"]["after"]["template"][0]["containers"][0].update(image="other/registry@sha256:" + "f" * 64))
        bad(lambda p: service(p).update(actions=["update"]))
        bad(lambda p: p["resource_drift"][0]["change"]["after"]["template"][0]["containers"][0]["env"][0].update(value="unreviewed"))
        # Unknown/mismatched planned after cannot be used to bypass refresh.
        bad(lambda p: p["resource_drift"][0]["change"].update(after_unknown={"template": True}))


    def test_security_capacity_image_and_environment_changes_are_rejected(self):
        changes = [
            lambda a: a.update(invoker_iam_disabled=True),
            lambda a: a["scaling"][0].update(max_instance_count=10),
            lambda a: a["template"][0].update(service_account="other@" + c.PROJECT + ".iam.gserviceaccount.com"),
            lambda a: a["template"][0]["containers"][0].update(image=r.REPOSITORY + ":latest"),
            lambda a: a["template"][0]["containers"][0].update(command=["/other-binary"]),
            lambda a: a["template"][0]["containers"][0]["env"].append({"name": "FIRESTORE_EMULATOR_HOST", "value": "other"}),
            lambda a: a["template"][0]["containers"][0]["env"][-1]["value_source"][0]["secret_key_ref"][0].update(version="latest"),
        ]
        for mutate in changes:
            plan = fixture()
            mutate(service(plan)["after"])
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(plan, "runtime_private")

    def test_updates_unknowns_new_resources_and_public_access_in_private_mode_stop(self):
        for mutate in (
            lambda p: service(p).update(actions=["update"]),
            lambda p: service(p).update(after_unknown={"template": [{"containers": [{"env": True}]}]}),
            lambda p: p["resource_changes"].append(deepcopy(fixture("runtime_public")["resource_changes"][-1])),
            lambda p: p.update(resource_drift=[{"address": r.SERVICE}]),
        ):
            plan = fixture()
            mutate(plan)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(plan, "runtime_private")

    def test_image_job_cannot_obtain_state_credentials_or_run_as_other_source(self):
        env = {"GITHUB_REPOSITORY": c.REPOSITORY, "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
               "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_ACTOR": "kani3camp", "GITHUB_ACTOR_ID": "54093651",
               "GITHUB_TRIGGERING_ACTOR": "kani3camp", "GITHUB_RUN_ATTEMPT": "1", "GITHUB_REF": c.REF,
               "GITHUB_WORKFLOW_REF": c.CALLER, "TF_WORKSPACE": "default", "GITHUB_SHA": "a" * 40,
               "MYPAGE_EXPECTED_SHA": "a" * 40, "MYPAGE_MODE": "image-build", "MYPAGE_ROLE": "apply",
               "MYPAGE_GCP_SERVICE_ACCOUNT": "mypage-foundation-apply@" + c.PROJECT + ".iam.gserviceaccount.com",
               "MYPAGE_GCP_WIF_PROVIDER": "projects/123456789/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions"}
        image_ops.preflight(env)
        for key, bad in (("AWS_ACCESS_KEY_ID", "synthetic"), ("GITHUB_EVENT_NAME", "pull_request"),
                         ("GITHUB_RUN_ATTEMPT", "2"), ("MYPAGE_EXPECTED_SHA", "b" * 40), ("MYPAGE_ROLE", "plan")):
            with self.subTest(field=key), self.assertRaises(ValueError):
                image_ops.preflight(env | {key: bad})
        with self.assertRaises(ValueError):
            c.context(env, "apply")


if __name__ == "__main__":
    unittest.main()
