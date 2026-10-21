"""Runtime boundary failures and exact-source image admission, without cloud I/O."""
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
    value = json.loads((ROOT / "infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json").read_text())
    after = r.resource_contract(stage)
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
                                  "change": {"actions": ["create"], "after": values, "after_unknown": {}}} for address, values in after.items()]}
    if stage == "runtime_public":
        change = next(x['change'] for x in result['resource_changes'] if x['address'] == r.INVOKER)
        change.update(actions=['no-op'], importing={'id': r.PUBLIC_INVOKER_IMPORT_ID})
    return result


def service(plan):
    return next(item["change"] for item in plan["resource_changes"] if item["address"] == r.SERVICE)


class RuntimeTest(unittest.TestCase):
    def test_bootstrap_cleanup_refreshes_only_the_same_noop_iam_members(self):
        for stage, size in (("runtime_private", 11), ("runtime_public", 12)):
            plan = fixture(stage)
            plan['resource_drift'] = []
            for item in plan['resource_changes']:
                item['change']['actions'] = ['no-op']
                item['change'].pop('importing', None)  # Already managed members in this State-refresh scenario.
                if item['address'] not in {r.FIRESTORE, r.INVOKER}:
                    continue
                item['change']['after']['etag'] = 'synthetic-new-etag'
                drift = deepcopy(item)
                drift['change'].update(actions=['update'], before=deepcopy(drift['change']['after']) | {'etag': 'synthetic-old-etag'})
                plan['resource_drift'].append(drift)
            original = deepcopy(plan)
            self.assertEqual(r.validate_plan(plan, stage), {'resources': size, 'creates': 0})
            self.assertEqual(plan, original)
            for mutate in (
                lambda p: p['resource_drift'][0]['change']['after'].update(member='allUsers'),
                lambda p: (p['resource_drift'][0]['change']['before'].update(member='allUsers'),
                           p['resource_drift'][0]['change']['after'].update(member='allUsers')),
                lambda p: p['resource_drift'][0]['change']['after'].update(condition=[{'expression': 'true'}]),
                lambda p: p['resource_drift'][0]['change']['after_unknown'].update(etag=True),
                lambda p: next(x for x in p['resource_changes'] if x['address'] == r.FIRESTORE)['change'].update(actions=['update']),
                lambda p: p['resource_drift'].append(deepcopy(p['resource_drift'][0])),
                lambda p: p['resource_drift'][0].update(address=r.SERVICE),
            ):
                rejected = deepcopy(plan)
                mutate(rejected)
                with self.subTest(stage=stage, mutation=mutate), self.assertRaises(ValueError):
                    r.validate_plan(rejected, stage)
            key = 'private-key-for-synthetic-tests-only'
            self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                                c.commitment(plan | {'resource_drift': []}, 'state-version', 'a' * 40, key))

    def test_initial_service_refresh_accepts_only_observed_empty_defaults_on_noop(self):
        for stage, size in (("runtime_private", 11), ("runtime_public", 12)):
            plan = fixture(stage)
            for item in plan['resource_changes']:
                item['change']['actions'] = ['no-op']
            after = service(plan)['after']
            after.update(annotations={}, labels={}, custom_audiences=[])
            after['template'][0].update(annotations={}, labels={})
            after['template'][0]['containers'][0].update(args=[], command=[], depends_on=[])
            drift = deepcopy(next(x for x in plan['resource_changes'] if x['address'] == r.SERVICE))
            before = deepcopy(after)
            before.update(annotations=None, labels=None, custom_audiences=None)
            before['template'][0].update(annotations=None, labels=None)
            before['template'][0]['containers'][0].update(args=None, command=None, depends_on=None)
            drift['change'].update(actions=['update'], before=before)
            plan['resource_drift'] = [drift]
            original = deepcopy(plan)
            self.assertEqual(r.validate_plan(plan, stage), {'resources': size, 'creates': 0})
            self.assertEqual(plan, original)
            for mutate in (
                lambda p: p['resource_drift'][0]['change']['after'].update(labels={'unexpected': 'private-value'}),
                lambda p: p['resource_drift'][0]['change']['before'].update(labels={'retained': 'private-value'}),
                lambda p: p['resource_drift'][0]['change']['after']['template'][0]['containers'][0].update(command=['/other']),
                lambda p: p['resource_drift'][0]['change']['after']['template'][0]['containers'][0].update(image=r.REPOSITORY + '@sha256:' + 'b' * 64),
                lambda p: p['resource_drift'][0]['change']['after'].update(invoker_iam_disabled=True),
                lambda p: p['resource_drift'][0]['change']['after'].update(new_field=[]),
                lambda p: p['resource_drift'][0]['change']['after_unknown'].update(labels=True),
                lambda p: service(p).update(actions=['update']),
                lambda p: service(p)['after'].update(labels=None),
                lambda p: p['resource_drift'][0]['change'].update(actions=['delete', 'create']),
                lambda p: p['resource_drift'][0].update(provider_name='registry.terraform.io/other/provider'),
                lambda p: p['resource_drift'].append(deepcopy(p['resource_drift'][0])),
            ):
                rejected = deepcopy(plan)
                mutate(rejected)
                with self.subTest(stage=stage, mutation=mutate), self.assertRaises(ValueError):
                    r.validate_plan(rejected, stage)
            key = 'private-key-for-synthetic-tests-only'
            self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                                c.commitment(plan | {'resource_drift': []}, 'state-version', 'a' * 40, key))

    def test_private_and_public_have_exact_resources_and_pinned_secrets(self):
        for stage, size in (("runtime_private", 11), ("runtime_public", 12)):
            plan = fixture(stage)
            self.assertEqual(r.validate_plan(plan, stage), {"resources": size, "creates": size - (stage == "runtime_public")})
            for item in plan["resource_changes"]:
                item["change"]["actions"] = ["no-op"]
            self.assertEqual(r.validate_plan(plan, stage), {"resources": size, "creates": 0})

    def test_only_exact_owner_public_member_can_be_imported_without_cloud_writes(self):
        plan = fixture('runtime_public')
        self.assertEqual(r.public_invoker_import_config(), 'import {\n  to = ' + r.INVOKER + '\n  id = "' + r.PUBLIC_INVOKER_IMPORT_ID + '"\n}\n')
        for mutate in (
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER).update(actions=['create']),
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER).update(importing={'id': 'private-other-resource'}),
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER).update(importing={'id': r.PUBLIC_INVOKER_IMPORT_ID, 'unknown': True}),
            lambda p: service(p).update(importing={'id': r.PUBLIC_INVOKER_IMPORT_ID}),
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

    def test_security_capacity_image_and_environment_changes_are_rejected(self):
        changes = [
            lambda a: a.update(invoker_iam_disabled=True),
            lambda a: a["scaling"][0].update(max_instance_count=10),
            lambda a: a["template"][0].update(service_account="other@" + c.PROJECT + ".iam.gserviceaccount.com"),
            lambda a: a["template"][0].update(vpc_access=[{"egress": "ALL_TRAFFIC"}]),
            lambda a: a["template"][0].update(health_check_disabled=True),
            lambda a: a["template"][0]["containers"][0].update(image=r.REPOSITORY + ":latest"),
            lambda a: a["template"][0]["containers"][0].update(command=["/other-binary"]),
            lambda a: a["template"][0]["containers"][0]["resources"][0].update(cpu_idle=False),
            lambda a: a["template"][0]["containers"][0]["env"].append({"name": "FIRESTORE_EMULATOR_HOST", "value": "other"}),
            lambda a: a["template"][0]["containers"][0]["env"][-1].update(value="private-payload"),
            lambda a: a["template"][0]["containers"][0]["env"][-1]["value_source"][0]["secret_key_ref"][0].update(version="latest"),
            lambda a: a.update(build_config=[{"enable_automatic_updates": True}]),
            lambda a: a.update(ssh_enabled=True),
            lambda a: a["traffic"][0].update(percent=50),
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
