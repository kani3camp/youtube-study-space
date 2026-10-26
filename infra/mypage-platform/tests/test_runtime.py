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
                                  "change": {"actions": ["no-op"] if address in r.maintenance_resource_contract() else ["create"], "after": values, "after_unknown": {}}} for address, values in after.items()]}
    if stage == "runtime_public":
        change = next(x['change'] for x in result['resource_changes'] if x['address'] == r.INVOKER)
        change.update(actions=['no-op'], importing={'id': r.PUBLIC_INVOKER_IMPORT_ID})
    return result


def service(plan):
    return next(item["change"] for item in plan["resource_changes"] if item["address"] == r.SERVICE)


class RuntimeTest(unittest.TestCase):
    def test_new_noop_plan_refreshes_only_same_generation_output_metadata(self):
        from test_read_metrics_transition import fixture as metrics_fixture
        plan = metrics_fixture()
        plan['variables']['runtime_settings']['value'].update(read_metrics_enabled=False, read_metrics_from_revision='')
        before = deepcopy(service(plan)['before'])
        before.update(uid='synthetic-service', etag='old-etag', last_modifier='old-modifier', annotations=None, labels=None, custom_audiences=None)
        before['template'][0].update(annotations=None, labels=None)
        before['template'][0]['containers'][0].update(args=None, command=None, depends_on=None)
        after = deepcopy(before)
        after.update(etag='new-etag', last_modifier='new-modifier', update_time='2026-10-23T00:01:00Z')
        for item in plan['resource_changes']:
            item['change'].update(actions=['no-op'])
        service(plan).update(before=deepcopy(after), after=after, after_unknown={})
        drift = deepcopy(next(x for x in plan['resource_changes'] if x['address'] == r.SERVICE))
        drift['change'].update(actions=['update'], before=before)
        plan['resource_drift'] = [drift]
        original = deepcopy(plan)
        self.assertEqual(r.validate_plan(plan, 'runtime_private'), {'resources':27,'creates':0})
        self.assertEqual(plan, original)
        mutations = (
            (lambda p: service(p).update(actions=['update']), 'runtime-refresh-planned-noop'),
            (lambda p: p['resource_drift'][0]['change']['after'].update(generation='8', observed_generation='8'), 'runtime-refresh-generation'),
            (lambda p: p['resource_drift'][0]['change']['after'].update(uid='other'), 'runtime-refresh-identity'),
            (lambda p: p['resource_drift'][0]['change']['after'].update(update_time='2026-10-22T00:00:00Z'), 'runtime-refresh-metadata'),
            (lambda p: p['resource_drift'][0]['change']['after']['template'][0].update(timeout='60s'), 'runtime-refresh-fields'),
            (lambda p: p['resource_drift'][0]['change']['after'].update(etag='old-etag'), 'runtime-refresh-metadata'),
        )
        for mutate, expected_check in mutations:
            changed=deepcopy(plan); mutate(changed)
            service(changed)['after'] = deepcopy(changed['resource_drift'][0]['change']['after'])
            with self.subTest(mutation=mutate), self.assertRaises(ValueError) as error:
                r.validate_plan(changed,'runtime_private')
            self.assertEqual(str(error.exception), expected_check)

    def test_owner_hosting_image_refresh_is_exact_ready_noop_and_committed(self):
        for stage, size in (("runtime_private", 27), ("runtime_public", 28)):
            plan = fixture(stage)
            for item in plan['resource_changes']:
                item['change']['actions'] = ['no-op']
                item['change'].pop('importing', None)
            after = service(plan)['after']
            before = deepcopy(after)
            before.update(generation='1', observed_generation='1', update_time='2026-10-21T00:00:00Z',
                          latest_ready_revision='mypage-api-old', latest_created_revision='mypage-api-old',
                          terminal_condition=[{'state': 'CONDITION_SUCCEEDED'}], reconciling=False,
                          traffic_statuses=[{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100, 'revision': 'mypage-api-old'}])
            before['template'][0]['revision'] = 'mypage-api-old'
            after.clear()
            after.update(deepcopy(before))
            value = plan['variables']['runtime_settings']['value']
            value.update(previous_image_digest=value['image_digest'], image_digest=r.REPOSITORY + '@sha256:' + 'b' * 64,
                         hosting_backend_host='mypage-api-observed-et.a.run.app')
            after['template'][0]['containers'][0]['image'] = value['image_digest']
            after['template'][0]['containers'][0]['env'].append({'name': 'MYPAGE_HOSTING_BACKEND_HOST', 'value': value['hosting_backend_host']})
            after['template'][0]['revision'] = 'mypage-api-new'
            after.update(generation='2', observed_generation='2', update_time='2026-10-21T01:00:00Z',
                         latest_ready_revision='mypage-api-new', latest_created_revision='mypage-api-new',
                         traffic_statuses=[{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100, 'revision': 'mypage-api-new'}])
            drift = deepcopy(next(x for x in plan['resource_changes'] if x['address'] == r.SERVICE))
            drift['change'].update(actions=['update'], before=before)
            plan['resource_drift'] = [drift]
            original = deepcopy(plan)
            self.assertEqual(r.validate_plan(plan, stage), {'resources': size, 'creates': 0})
            self.assertEqual(plan, original)
            automatic = deepcopy(plan)
            service(automatic)['after']['template'][0]['revision'] = ''
            automatic['resource_drift'][0]['change']['after']['template'][0]['revision'] = ''
            automatic['resource_drift'][0]['change']['before']['template'][0]['revision'] = ''
            self.assertEqual(r.validate_plan(automatic, stage), {'resources': size, 'creates': 0})
            for invalid_revision in ('mypage-api-old', 'other-service-new', None):
                rejected = deepcopy(automatic)
                service(rejected)['after']['template'][0]['revision'] = invalid_revision
                rejected['resource_drift'][0]['change']['after']['template'][0]['revision'] = invalid_revision
                with self.assertRaises(ValueError):
                    r.validate_plan(rejected, stage)
            for mutate in (
                lambda p: service(p).update(actions=['update']),
                lambda p: p['variables']['runtime_settings']['value'].update(previous_image_digest=r.REPOSITORY + '@sha256:' + 'c' * 64),
                lambda p: p['resource_drift'][0]['change'].update(after_unknown={'generation': True}),
                lambda p: p['resource_drift'][0]['change'].update(actions=['delete', 'create']),
                lambda p: p['resource_drift'][0]['change']['before']['template'][0].update(service_account='other@example.test'),
                lambda p: p['resource_drift'][0]['change']['before']['template'][0]['containers'][0]['env'].pop(),
                lambda p: p['resource_drift'][0]['change']['before'].update(generation='9'),
                lambda p: p['resource_drift'][0]['change']['before'].update(update_time='2026-10-22T00:00:00Z'),
                lambda p: p['resource_drift'][0]['change']['before'].update(invoker_iam_disabled=True),
                lambda p: p['resource_drift'][0]['change']['before'].update(ingress='INGRESS_TRAFFIC_INTERNAL_ONLY'),
                lambda p: p['resource_drift'][0]['change']['before']['template'][0].update(timeout='60s'),
            ):
                rejected = deepcopy(plan)
                mutate(rejected)
                with self.subTest(stage=stage, mutation=mutate), self.assertRaises(ValueError):
                    r.validate_plan(rejected, stage)
            # Keep planned/observed values equal to test the admission itself.
            for field, invalid in (('generation', '3'), ('observed_generation', '1'),
                                   ('latest_ready_revision', 'mypage-api-old'), ('reconciling', True),
                                   ('terminal_condition', [{'state': 'CONDITION_FAILED'}]),
                                   ('traffic_statuses', [{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 50, 'revision': 'mypage-api-new'}])):
                rejected = deepcopy(plan)
                service(rejected)['after'][field] = invalid
                rejected['resource_drift'][0]['change']['after'][field] = invalid
                with self.subTest(stage=stage, field=field), self.assertRaises(ValueError):
                    r.validate_plan(rejected, stage)
            key = 'private-key-for-synthetic-tests-only'
            self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                                c.commitment(plan | {'resource_drift': []}, 'state-version', 'a' * 40, key))

    def test_later_owner_image_release_preserves_every_environment_value(self):
        plan = fixture('runtime_public')
        for item in plan['resource_changes']:
            item['change']['actions'] = ['no-op']
            item['change'].pop('importing', None)
        value = plan['variables']['runtime_settings']['value']
        value.update(previous_image_digest=value['image_digest'],
                     image_digest=r.REPOSITORY + '@sha256:' + 'b' * 64,
                     hosting_backend_host='mypage-api-observed-et.a.run.app')
        before = deepcopy(service(plan)['after'])
        before['template'][0]['containers'][0]['env'].append(
            {'name': 'MYPAGE_HOSTING_BACKEND_HOST', 'value': value['hosting_backend_host']})
        before['template'][0]['revision'] = ''
        before.update(generation='2', observed_generation='2', update_time='2026-10-21T00:00:00Z',
                      latest_ready_revision='mypage-api-old', latest_created_revision='mypage-api-old',
                      terminal_condition=[{'state': 'CONDITION_SUCCEEDED'}], reconciling=False,
                      traffic_statuses=[{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100, 'revision': 'mypage-api-old'}])
        after = deepcopy(before)
        after['template'][0]['containers'][0]['image'] = value['image_digest']
        after.update(generation='3', observed_generation='3', update_time='2026-10-22T00:00:00Z',
                     latest_ready_revision='mypage-api-new', latest_created_revision='mypage-api-new',
                     traffic_statuses=[{'type': 'TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST', 'percent': 100, 'revision': 'mypage-api-new'}])
        service(plan)['after'] = after
        drift = deepcopy(next(x for x in plan['resource_changes'] if x['address'] == r.SERVICE))
        drift['change'].update(actions=['update'], before=before)
        plan['resource_drift'] = [drift]
        original = deepcopy(plan)
        self.assertEqual(r.validate_plan(plan, 'runtime_public'), {'resources': 28, 'creates': 0})
        self.assertEqual(plan, original)
        for mutate in (
            lambda p: p['resource_drift'][0]['change']['before']['template'][0]['containers'][0]['env'][-1].update(value='other-et.a.run.app'),
            lambda p: p['resource_drift'][0]['change']['before']['template'][0]['containers'][0]['env'].pop(0),
            lambda p: p['resource_drift'][0]['change']['before']['template'][0]['containers'][0]['env'][0].update(value='other'),
            lambda p: p['resource_drift'][0]['change']['before']['template'][0]['containers'][0].update(env=None),
            lambda p: service(p).update(actions=['update']),
            lambda p: p['resource_drift'][0]['change'].update(after_unknown={'template': True}),
        ):
            rejected = deepcopy(plan)
            mutate(rejected)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(rejected, 'runtime_public')

    def test_bootstrap_cleanup_refreshes_only_the_same_noop_iam_members(self):
        for stage, size in (("runtime_private", 27), ("runtime_public", 28)):
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
        for stage, size in (("runtime_private", 27), ("runtime_public", 28)):
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
        for stage, size in (("runtime_private", 27), ("runtime_public", 28)):
            plan = fixture(stage)
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
        self.assertEqual(r.validate_plan(qualified, 'runtime_public'), {'resources': 28, 'creates': 11})
        for mutate in (
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER)['after'].update(name='projects/other/locations/asia-southeast2/services/mypage-api'),
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
