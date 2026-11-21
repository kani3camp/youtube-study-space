"""runtime_contract.py(Cloud Runの許可設定)と image_ops.py のイメージ受け入れをテストする。クラウドには接続しない。

目的: メンテナンス用の権限が広がる、公開設定や環境変数や容量が許可なく変わる、別のコミットのイメージが受け入れられる、といった不具合を検出する。
使われる場面: infra/mypage-platform/scripts/check-local.sh の `unittest discover`。CI GCP Terraform の terraform-tests(validate.sh 経由)で実行される。他のテストから部品も読み込まれる。
失敗したとき: CI GCP Terraform が失敗し、PRをマージできない。
なくした場合: Cloud Runの許可ルールの変更が、自動では検査されなくなる。test_foundation.py と対象が一部重なる。
"""
from copy import deepcopy
import json
from pathlib import Path
import sys
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / "infra/mypage-platform/scripts"))
import foundation_contract as c
import image_ops
import runtime_contract as r


def fixture(public=False, *, missing_maintenance=False):
    """宣言済み実行系のPlan。既存のmaintenance境界はno-op、それ以外は作成として組み立てる。

    missing_maintenance=Trueでは逆に、既存の実行系を保ったままmaintenance境界だけを作成する。
    """
    value = r.settings(json.loads((ROOT / "infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json").read_text()))
    after = r.resource_contract(runtime=True, public=public, value=value)
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
            "variables": {"runtime_settings": {"value": value}, "public_invoker": {"value": public}},
            "resource_changes": [{"address": address, "mode": "managed", "provider_name": "registry.terraform.io/hashicorp/google",
                                  "change": {"actions": (["create"] if address in r.maintenance_resource_contract() else ["no-op"]) if missing_maintenance else (["no-op"] if address in r.maintenance_resource_contract() else ["create"]), "after": values, "after_unknown": {}}} for address, values in after.items()]}
    for item in result["resource_changes"]:
        if item["address"] in {r.MAINTENANCE_ROLE} and item["change"]["actions"] == ["no-op"]:
            item["change"]["after"]["deleted"] = False
    if public:
        change = next(x['change'] for x in result['resource_changes'] if x['address'] == r.INVOKER)
        change.update(actions=['no-op'], importing={'id': r.PUBLIC_INVOKER_IMPORT_ID})
    return result


def service(plan):
    return next(item["change"] for item in plan["resource_changes"] if item["address"] == r.SERVICE)


class RuntimeTest(unittest.TestCase):
    def test_maintenance_invoker_can_read_job_and_wait_without_deploy_or_firestore_access(self):
        plan = fixture(public=True)
        additions = {r.MAINTENANCE_READ_ROLE, r.MAINTENANCE_JOB_READER}
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["create"] if item["address"] in additions else ["no-op"]
        resources = {item["address"]: item["change"]["after"] for item in plan["resource_changes"]}
        self.assertEqual(set(resources[r.MAINTENANCE_READ_ROLE]["permissions"]),
                         {"run.jobs.get", "run.executions.get"})
        self.assertEqual(resources[r.MAINTENANCE_JOB_READER]["role"],
                         "projects/" + c.PROJECT + "/roles/mypageMaintenanceExecutionRead")
        self.assertEqual(resources[r.MAINTENANCE_JOB_READER]["member"],
                         "serviceAccount:mypage-maintenance-invoker@" + c.PROJECT + ".iam.gserviceaccount.com")
        self.assertEqual(resources[r.MAINTENANCE_JOB_INVOKER]["role"], "roles/run.jobsExecutorWithOverrides")
        self.assertEqual(r.validate_plan(plan),
                         {"resources": len(plan["resource_changes"]), "creates": 2})

        for address, field, wrong in (
            (r.MAINTENANCE_READ_ROLE, "permissions", ["run.jobs.get"]),
            (r.MAINTENANCE_READ_ROLE, "permissions", ["run.executions.get"]),
            (r.MAINTENANCE_READ_ROLE, "permissions", ["run.executions.get", "run.jobs.get", "run.jobs.update"]),
            (r.MAINTENANCE_READ_ROLE, "permissions", ["datastore.entities.get", "run.executions.get", "run.jobs.get"]),
            (r.MAINTENANCE_JOB_READER, "role", "roles/run.admin"),
            (r.MAINTENANCE_JOB_READER, "member", "allUsers"),
            (r.MAINTENANCE_JOB_READER, "member", "serviceAccount:other@" + c.PROJECT + ".iam.gserviceaccount.com"),
            (r.MAINTENANCE_JOB_READER, "project", "other-project"),
            (r.MAINTENANCE_JOB_READER, "location", "us-central1"),
        ):
            rejected = deepcopy(plan)
            item = next(item for item in rejected["resource_changes"] if item["address"] == address)
            item["change"]["after"][field] = wrong
            with self.subTest(address=address, field=field, wrong=wrong), self.assertRaises(c.ContractError):
                r.validate_plan(rejected)
        for address in additions:
            rejected = deepcopy(plan)
            rejected["resource_changes"] = [item for item in rejected["resource_changes"] if item["address"] != address]
            with self.subTest(missing=address), self.assertRaises(c.ContractError):
                r.validate_plan(rejected)

    def test_existing_public_runtime_can_add_only_the_exact_missing_maintenance_boundary(self):
        plan = fixture(public=True, missing_maintenance=True)
        maintenance = set(r.maintenance_resource_contract())
        self.assertEqual(r.validate_plan(plan), {"resources": len(plan["resource_changes"]), "creates": len(maintenance)})
        for item in plan["resource_changes"]:
            expected = ["create"] if item["address"] in maintenance else ["no-op"]
            self.assertEqual(item["change"]["actions"], expected)
        for actions in (["update"], ["delete", "create"]):
            rejected = deepcopy(plan)
            service(rejected)["actions"] = actions
            with self.subTest(actions=actions), self.assertRaisesRegex(c.ContractError, "runtime-resource-action"):
                r.validate_plan(rejected)
        rejected = deepcopy(plan)
        next(item for item in rejected["resource_changes"] if item["address"] == r.MAINTENANCE_JOB)["change"]["actions"] = ["update"]
        with self.assertRaisesRegex(c.ContractError, "runtime-resource-action"):
            r.validate_plan(rejected)

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
            (r.MAINTENANCE_JOB_READER,
             lambda item: item["change"].update(after_unknown={"condition": True}),
             "maintenance-iam-unknown"),
            (r.OPERATOR_IMAGE_BINDING,
             lambda item: item["change"]["after"].update(member="serviceAccount:other@" + c.PROJECT + ".iam.gserviceaccount.com"),
             "runtime-resource-value-" + r.OPERATOR_IMAGE_BINDING + "-member"),
            (r.OPERATOR_IMAGE_BINDING,
             lambda item: item["change"]["after"].update(member="serviceAccount:mypage-backend-deploy@" + c.PROJECT + ".iam.gserviceaccount.com"),
             "runtime-resource-value-" + r.OPERATOR_IMAGE_BINDING + "-member"),
            (r.OPERATOR_IMAGE_ACT_AS,
             lambda item: item["change"]["after"].update(role="roles/iam.serviceAccountTokenCreator"),
             "runtime-resource-value-" + r.OPERATOR_IMAGE_ACT_AS + "-role"),
        )
        for address, mutate, check in mutations:
            rejected = fixture(public=True, missing_maintenance=True)
            item = next(item for item in rejected["resource_changes"] if item["address"] == address)
            mutate(item)
            with self.subTest(address=address, check=check), self.assertRaises(c.ContractError) as error:
                r.validate_plan(rejected)
            self.assertEqual(str(error.exception), check)

    def test_noop_revision_must_equal_the_exact_observed_ready_revision(self):
        cases = (
            (r.SERVICE, "mypage-api-ready", "runtime-owned-revision"),
        )
        for address, ready, check in cases:
            plan = fixture()
            item = next(item for item in plan["resource_changes"] if item["address"] == address)
            item["change"]["actions"] = ["no-op"]
            after = item["change"]["after"]
            after["latest_ready_revision"] = ready
            after["template"][0]["revision"] = ready
            after["traffic"][0]["revision"] = ready
            with self.subTest(address=address, revision="ready"):
                r.validate_plan(plan)

            rejected = deepcopy(plan)
            changed = next(item for item in rejected["resource_changes"] if item["address"] == address)
            changed["change"]["after"]["template"][0]["revision"] = ready + "-pinned"
            with self.subTest(address=address, revision="pinned"), self.assertRaises(c.ContractError) as error:
                r.validate_plan(rejected)
            self.assertEqual(str(error.exception), check)

    def test_maintenance_job_image_is_ci_owned_after_bootstrap(self):
        plan = fixture(public=True)
        item = next(item for item in plan["resource_changes"] if item["address"] == r.MAINTENANCE_JOB)
        item["change"]["actions"] = ["no-op"]
        item["change"]["after"]["template"][0]["template"][0]["containers"][0]["image"] = (
            r.REPOSITORY + "@sha256:" + "d" * 64
        )
        self.assertEqual(r.validate_plan(plan),
                         {"resources": len(plan["resource_changes"]), "creates": 11})
        # The dedicated operator package is accepted beside the legacy API package
        # until the first operator release has been verified.
        container = item["change"]["after"]["template"][0]["template"][0]["containers"][0]
        container["image"] = r.OPERATOR_REPOSITORY + "@sha256:" + "d" * 64
        r.validate_plan(plan)
        for foreign in (r.IMAGE_REGISTRY + "other@sha256:" + "d" * 64,
                        r.OPERATOR_REPOSITORY + ":latest"):
            container["image"] = foreign
            with self.subTest(image=foreign), self.assertRaisesRegex(c.ContractError, "^maintenance-job-image$"):
                r.validate_plan(plan)

        # 初回作成だけは、レビュー済みruntime settingsのAPI bootstrap digestと完全一致させる。
        created = fixture(public=True, missing_maintenance=True)
        job = next(item for item in created["resource_changes"] if item["address"] == r.MAINTENANCE_JOB)
        self.assertEqual(job["change"]["after"]["template"][0]["template"][0]["containers"][0]["image"],
                         created["variables"]["runtime_settings"]["value"]["image_digest"])
        self.assertEqual(r.validate_plan(created),
                         {"resources": len(created["resource_changes"]),
                          "creates": len(r.maintenance_resource_contract())})
        for other in (r.REPOSITORY + "@sha256:" + "e" * 64, r.OPERATOR_REPOSITORY + "@sha256:" + "e" * 64):
            rejected = deepcopy(created)
            next(item for item in rejected["resource_changes"] if item["address"] == r.MAINTENANCE_JOB)[
                "change"]["after"]["template"][0]["template"][0]["containers"][0]["image"] = other
            with self.subTest(image=other), self.assertRaisesRegex(c.ContractError, "^maintenance-job-bootstrap-image$"):
                r.validate_plan(rejected)

    def test_job_refresh_treats_provider_null_and_empty_list_as_the_same_unset_list(self):
        plan = fixture(public=True)
        value = r.settings(plan["variables"]["runtime_settings"]["value"])
        job = next(item["change"]["after"] for item in plan["resource_changes"] if item["address"] == r.MAINTENANCE_JOB)
        container = lambda state: state["template"][0]["template"][0]["containers"][0]
        before, after = deepcopy(job), deepcopy(job)
        container(before)["args"] = None
        container(after)["args"] = []
        r.validate_cicd_job_image_refresh(before, after, value)
        for field, changed in (("args", ["--unreviewed"]), ("command", ["/bin/sh"]), ("command", None)):
            moved = deepcopy(after)
            container(moved)[field] = changed
            with self.subTest(field=field), self.assertRaises(c.ContractError):
                r.validate_cicd_job_image_refresh(before, moved, value)

    def test_job_iam_members_accept_the_provider_full_job_path_after_a_read(self):
        bindings = (r.MAINTENANCE_JOB_INVOKER, r.MAINTENANCE_JOB_READER, r.OPERATOR_IMAGE_BINDING)
        plan = fixture(public=True)
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["no-op"]
            if item["address"] in bindings:
                item["change"]["after"]["name"] = r.MAINTENANCE_JOB_FULL_NAME
        present = {item["address"] for item in plan["resource_changes"]} & set(bindings)
        self.assertEqual(present, set(bindings))
        r.validate_plan(plan)
        for address in sorted(present):
            for wrong in ("projects/" + c.PROJECT + "/locations/" + c.REGION + "/jobs/other-job",
                          "projects/other-project/locations/" + c.REGION + "/jobs/mypage-maintenance-operator",
                          "projects/" + c.PROJECT + "/locations/us-central1/jobs/mypage-maintenance-operator",
                          r.PUBLIC_INVOKER_NAME):
                rejected = deepcopy(plan)
                next(item for item in rejected["resource_changes"] if item["address"] == address)["change"]["after"]["name"] = wrong
                with self.subTest(address=address, wrong=wrong), self.assertRaises(c.ContractError):
                    r.validate_plan(rejected)

    def provider_fix_plan(self):
        plan = fixture(public=True)
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["no-op"]
        item = next(item for item in plan["resource_changes"] if item["address"] == r.MAINTENANCE_PROVIDER)
        before = deepcopy(item["change"]["after"])
        before.update(attribute_condition=r.maintenance_provider_condition(legacy=True),
                      attribute_mapping=r.maintenance_provider_mapping(legacy=True))
        item["change"].update(actions=["update"], before=before)
        return plan, item

    def test_maintenance_provider_subject_fits_sts_and_requires_the_reusable_job(self):
        mapping = r.maintenance_provider_mapping()
        self.assertEqual(mapping["google.subject"], r.MAINTENANCE_SUBJECT)
        longest = "340900071:mypage-maintenance-development:54093651"
        self.assertLessEqual(len(longest.encode()), 127)
        self.assertIn("assertion.job_workflow_ref == 'kani3camp/youtube-study-space/.github/workflows/"
                      "mypage-maintenance-authenticated.yml@" + c.REF + "'", r.maintenance_provider_condition())
        self.assertNotIn("job_workflow_ref", r.maintenance_provider_condition(legacy=True))

    def test_only_the_reviewed_maintenance_provider_fix_is_accepted_as_an_update(self):
        plan, item = self.provider_fix_plan()
        self.assertTrue(r.reviewed_provider_update(r.MAINTENANCE_PROVIDER, item["change"]))
        self.assertEqual(r.validate_plan(plan), {"resources": len(plan["resource_changes"]), "creates": 0})
        for name, mutate in (
            ("other-before-condition", lambda ch: ch["before"].update(attribute_condition="true")),
            ("other-before-mapping", lambda ch: ch["before"]["attribute_mapping"].update({"attribute.extra": "assertion.sub"})),
            ("only-mapping", lambda ch: ch["before"].update(attribute_condition=r.maintenance_provider_condition())),
            ("after-mapping-extra", lambda ch: ch["after"]["attribute_mapping"].update({"attribute.extra": "assertion.sub"})),
            ("after-condition-widened", lambda ch: ch["after"].update(attribute_condition="true")),
            ("other-attribute", lambda ch: ch["after"].update(disabled=True)),
            ("unknown", lambda ch: ch.update(after_unknown={"attribute_condition": True})),
            ("replace", lambda ch: ch.update(replace_paths=[["attribute_mapping"]])),
            ("delete-create", lambda ch: ch.update(actions=["delete", "create"])),
        ):
            rejected, changed = self.provider_fix_plan()
            mutate(changed["change"])
            with self.subTest(name=name):
                self.assertFalse(r.reviewed_provider_update(r.MAINTENANCE_PROVIDER, changed["change"]))
                with self.assertRaises(c.ContractError):
                    r.validate_plan(rejected)
        self.assertFalse(r.reviewed_provider_update(r.MAINTENANCE_POOL, item["change"]))

    def state_only_plan(self):
        plan = fixture(public=True)
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["no-op"]
        item = next(item for item in plan["resource_changes"] if item["address"] == r.SERVICE)
        after = item["change"]["after"]
        after["deletion_policy"] = "PREVENT"
        before = deepcopy(after)
        before["deletion_policy"] = None
        item["change"].update(actions=["update"], before=before)
        return plan, item

    def test_only_the_reviewed_service_state_adoption_is_accepted_as_an_update(self):
        plan, item = self.state_only_plan()
        self.assertEqual(r.state_only_service_update(r.SERVICE, item["change"]), {"deletion_policy"})
        self.assertEqual(r.validate_plan(plan), {"resources": len(plan["resource_changes"]), "creates": 0})
        # The provider default recorded in State is adopted the same way.
        default_plan, default_item = self.state_only_plan()
        default_item["change"]["before"]["deletion_policy"] = "DELETE"
        self.assertEqual(r.state_only_service_update(r.SERVICE, default_item["change"]), {"deletion_policy"})
        r.validate_plan(default_plan)
        for mutate in (
            lambda change: change["after"].update(ingress="INGRESS_TRAFFIC_INTERNAL_ONLY"),
            lambda change: change["after"]["template"][0].update(timeout="60s"),
            lambda change: change["after"]["template"][0]["containers"][0].update(image=r.REPOSITORY + "@sha256:" + "e" * 64),
            lambda change: change["after"]["template"][0]["containers"][0]["env"].append({"name": "INJECTED", "value": "x"}),
            lambda change: change["after"]["traffic"][0].update(percent=50),
            lambda change: change["after"].update(deletion_policy="DELETE"),
            lambda change: change["before"].update(deletion_policy="ABANDON"),
            lambda change: change.update(after_unknown={"uri": True}),
            lambda change: change.update(replace_paths=[["name"]]),
            lambda change: change.update(actions=["delete", "create"]),
        ):
            rejected, changed = self.state_only_plan()
            mutate(changed["change"])
            with self.subTest(mutate=mutate):
                self.assertIsNone(r.state_only_service_update(r.SERVICE, changed["change"]))
                with self.assertRaises(c.ContractError):
                    r.validate_plan(rejected)
        # Never for any other resource, even with the same attribute change.
        other = deepcopy(item["change"])
        self.assertIsNone(r.state_only_service_update(r.MAINTENANCE_JOB, other))

    def test_refresh_drift_may_be_adopted_only_with_the_reviewed_state_attribute(self):
        _, item = self.state_only_plan()
        observed = deepcopy(item["change"]["before"])
        drift = {"address": r.SERVICE, "mode": "managed", "provider_name": "registry.terraform.io/hashicorp/google",
                 "change": {"actions": ["update"], "before": deepcopy(observed), "after": observed}}
        plan = {"resource_changes": [item], "resource_drift": [drift]}
        resources = {r.SERVICE: {}}
        c.validate_refresh(plan, resources, r.state_only_service_update)
        with self.assertRaisesRegex(c.ContractError, "^refresh-planned-noop$"):
            c.validate_refresh(plan, resources)
        moved = deepcopy(plan)
        moved["resource_drift"][0]["change"]["after"]["ingress"] = "INGRESS_TRAFFIC_INTERNAL_ONLY"
        with self.assertRaisesRegex(c.ContractError, "^refresh-planned-state$"):
            c.validate_refresh(moved, resources, r.state_only_service_update)

    RETIRED_ENV = [{"name": "MYPAGE_PRIVACY_VERSION", "value": "dev-draft-p"},
                   {"name": "MYPAGE_TERMS_VERSION", "value": "dev-draft-t"}]

    def retired_env_plan(self, *, drift=False):
        """既存Serviceの旧同意版env 2個だけをTerraformが外すPlan（他資源はno-op）。"""
        plan = fixture(public=True)
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["no-op"]
        item = next(item for item in plan["resource_changes"] if item["address"] == r.SERVICE)
        after = item["change"]["after"]
        after.update(uid="fixed-service-uid")
        after["template"][0]["containers"][0]["image"] = r.REPOSITORY + "@sha256:" + "c" * 64
        before = deepcopy(after)
        env = before["template"][0]["containers"][0]["env"]
        signer = next(i for i, entry in enumerate(env) if entry["name"] == "MYPAGE_SIGNER_EMAIL")
        env[signer + 1:signer + 1] = deepcopy(self.RETIRED_ENV)
        before.update(etag="synthetic-etag", generation="6", observed_generation="6", reconciling=False,
                      latest_ready_revision="mypage-api-00006-abc", latest_created_revision="mypage-api-00006-abc")
        # 更新で値が決まらない出力専用属性はafterに無く、after_unknownでtrueになる。
        item["change"].update(actions=["update"], before=before, after_unknown={
            "etag": True, "generation": True, "observed_generation": True, "reconciling": True,
            "latest_ready_revision": True, "latest_created_revision": True})
        if drift:
            # CI/CDが配備した新しいimageをrefreshで観測し、計画はその状態から始まる。
            state = deepcopy(before)
            state.update(generation="1", observed_generation="1")
            state["template"][0]["containers"][0]["image"] = value_image = r.REPOSITORY + "@sha256:" + "a" * 64
            self.assertNotEqual(value_image, after["template"][0]["containers"][0]["image"])
            plan["resource_drift"] = [{"address": r.SERVICE, "mode": "managed",
                                       "provider_name": "registry.terraform.io/hashicorp/google",
                                       "change": {"actions": ["update"], "before": state, "after": deepcopy(before)}}]
        return plan, item

    def test_only_the_exact_retired_consent_env_removal_is_accepted_as_an_update(self):
        for drift in (False, True):
            plan, item = self.retired_env_plan(drift=drift)
            self.assertEqual(r.retired_env_removal(r.SERVICE, item["change"]),
                             {"template", "etag", "generation", "observed_generation", "reconciling",
                              "latest_ready_revision", "latest_created_revision"})
            saved = deepcopy(plan)
            with self.subTest(drift=drift):
                self.assertEqual(r.validate_plan(plan), {"resources": len(plan["resource_changes"]), "creates": 0})
                self.assertEqual(plan, saved)
        # 位置に依らず、2変数がそろって外れるなら同じ遷移として扱う。
        plan, item = self.retired_env_plan()
        env = item["change"]["before"]["template"][0]["containers"][0]["env"]
        env[:] = [e for e in env if e["name"] not in r.RETIRED_SERVICE_ENV] + deepcopy(self.RETIRED_ENV)
        self.assertIsNotNone(r.retired_env_removal(r.SERVICE, item["change"]))
        r.validate_plan(plan)

    def test_retired_consent_env_removal_combined_with_any_other_change_is_rejected(self):
        before_env = lambda ch: ch["before"]["template"][0]["containers"][0]["env"]
        after_env = lambda ch: ch["after"]["template"][0]["containers"][0]["env"]
        for name, mutate in (
            ("image", lambda ch: ch["after"]["template"][0]["containers"][0].update(image=r.REPOSITORY + "@sha256:" + "e" * 64)),
            ("other-env-value", lambda ch: after_env(ch)[0].update(value="other")),
            ("other-env-added", lambda ch: after_env(ch).append({"name": "INJECTED", "value": "x"})),
            ("other-env-removed", lambda ch: after_env(ch).pop(0)),
            ("other-env-reordered", lambda ch: after_env(ch).reverse()),
            ("secret-version", lambda ch: after_env(ch)[-1]["value_source"][0]["secret_key_ref"][0].update(version="latest")),
            ("timeout", lambda ch: ch["after"]["template"][0].update(timeout="60s")),
            ("ingress", lambda ch: ch["after"].update(ingress="INGRESS_TRAFFIC_INTERNAL_ONLY")),
            ("traffic", lambda ch: ch["after"]["traffic"][0].update(percent=50)),
            ("deletion-policy", lambda ch: ch["after"].update(deletion_policy="DELETE")),
            ("uid", lambda ch: ch["after"].update(uid="other-service")),
            ("config-unknown", lambda ch: ch["after_unknown"].update(template=[{"containers": [{"env": True}]}])),
            ("uri-unknown", lambda ch: ch["after_unknown"].update(uri=True)),
            ("only-one-removed", lambda ch: after_env(ch).append(deepcopy(self.RETIRED_ENV[1]))),
            ("both-kept", lambda ch: after_env(ch).extend(deepcopy(self.RETIRED_ENV))),
            ("only-one-in-state", lambda ch: before_env(ch).remove(self.RETIRED_ENV[0])),
            ("duplicate-in-state", lambda ch: before_env(ch).append(deepcopy(self.RETIRED_ENV[0]))),
            ("secret-backed-in-state", lambda ch: next(e for e in before_env(ch) if e["name"] == "MYPAGE_TERMS_VERSION").update(
                value_source=[{"secret_key_ref": [{"secret": "mypage-oauth-client-secret", "version": "2"}]}])),
            ("replace-paths", lambda ch: ch.update(replace_paths=[["template"]])),
            ("delete-create", lambda ch: ch.update(actions=["delete", "create"])),
            ("create-delete", lambda ch: ch.update(actions=["create", "delete"])),
            ("importing", lambda ch: ch.update(importing={"id": "x"})),
        ):
            rejected, changed = self.retired_env_plan()
            mutate(changed["change"])
            with self.subTest(name=name):
                self.assertIsNone(r.retired_env_removal(r.SERVICE, changed["change"]))
                with self.assertRaises(c.ContractError):
                    r.validate_plan(rejected)
        # Never for any other resource.
        _, item = self.retired_env_plan()
        self.assertIsNone(r.retired_env_removal(r.MAINTENANCE_JOB, item["change"]))

    def test_retired_consent_env_allowance_expires_once_the_env_is_gone(self):
        # 2変数が消えた後のServiceでは一致する遷移が無く、通常のno-op契約に戻る。
        _, item = self.retired_env_plan()
        gone = deepcopy(item["change"])
        gone["before"]["template"][0]["containers"][0]["env"] = deepcopy(
            gone["after"]["template"][0]["containers"][0]["env"])
        self.assertIsNone(r.retired_env_removal(r.SERVICE, gone))
        # State に旧envが残ったままのno-op（削除しないPlan）は受け入れない。
        plan, item = self.retired_env_plan()
        item["change"].update(actions=["no-op"], after=deepcopy(item["change"]["before"]), after_unknown={})
        with self.assertRaisesRegex(c.ContractError, "^runtime-environment-set$"):
            r.validate_plan(plan)
        # refreshで観測しただけ（Planが外さない）でも同じく拒否する。
        plan, item = self.retired_env_plan(drift=True)
        item["change"].update(actions=["no-op"], after=deepcopy(item["change"]["before"]), after_unknown={})
        with self.assertRaisesRegex(c.ContractError, "^runtime-environment-set$"):
            r.validate_plan(plan)
        # 外すupdateは、refresh後のStateから始まらなければならない。
        plan, item = self.retired_env_plan(drift=True)
        plan["resource_drift"][0]["change"]["after"]["etag"] = "other-etag"
        with self.assertRaisesRegex(c.ContractError, "^runtime-retired-environment-refresh$"):
            r.validate_plan(plan)

    def noop_plan(self, public=True):
        plan = fixture(public)
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["no-op"]
        return plan

    def test_legacy_backend_job_release_bindings_are_removed_from_the_contract(self):
        resources = r.maintenance_resource_contract()
        self.assertNotIn(r.MAINTENANCE_IMAGE_BINDING, resources)
        self.assertNotIn(r.MAINTENANCE_IMAGE_ACT_AS, resources)
        self.assertIn(r.MAINTENANCE_IMAGE_ROLE, resources)
        self.assertIn(r.OPERATOR_IMAGE_BINDING, resources)
        self.assertIn(r.OPERATOR_IMAGE_ACT_AS, resources)
        text = (ROOT / "infra/mypage-platform/modules/runtime/main.tf").read_text()
        self.assertNotIn('"maintenance_image_release_act_as"', text)
        self.assertNotIn('resource "google_cloud_run_v2_job_iam_member" "maintenance_image_release"', text)
        # The Backend deployer never regains any access to the maintenance Job or its runtime SA.
        for block in text.split('\nresource "')[1:]:
            if block.startswith(("google_cloud_run_v2_job_iam_member", "google_service_account_iam_member\" \"operator")):
                self.assertNotIn("backend_deployer", block)
                self.assertNotIn("mypage-backend-deploy", block)

    def test_operator_image_resources_cannot_be_deleted(self):
        base = self.noop_plan()
        for address in (r.OPERATOR_IMAGE_BINDING, r.OPERATOR_IMAGE_ACT_AS, r.MAINTENANCE_IMAGE_ROLE):
            rejected = deepcopy(base)
            next(x for x in rejected["resource_changes"] if x["address"] == address)["change"]["actions"] = ["delete"]
            with self.subTest(address=address), self.assertRaises(c.ContractError):
                r.validate_plan(rejected)

    def test_only_the_operator_deployer_releases_the_job_image_in_both_environments(self):
        value = r.settings(json.loads((ROOT / "infra/mypage-platform/environments/dev/tests/runtime-settings.fixture.json").read_text()))
        for target in ("development", "production"):
            with self.subTest(target=target), patch.object(r.c, "TARGET", target):
                resources = r.maintenance_resource_contract(value)
                self.assertIn(r.MAINTENANCE_IMAGE_ROLE, resources)
                self.assertIn(r.OPERATOR_IMAGE_BINDING, resources)
                self.assertIn(r.OPERATOR_IMAGE_ACT_AS, resources)
                self.assertNotIn(r.MAINTENANCE_IMAGE_BINDING, resources)
                self.assertNotIn(r.MAINTENANCE_IMAGE_ACT_AS, resources)
                self.assertTrue(resources[r.OPERATOR_IMAGE_BINDING]["member"].startswith("serviceAccount:mypage-operator-deploy@"))

    def test_release_identities_are_declared_only_for_production(self):
        self.assertEqual(r.release_resource_contract(), {})
        self.assertFalse(set(r.resource_contract(runtime=True, public=True)) &
                         {address for group in (r.RELEASE_SA, r.RELEASE_POOL, r.RELEASE_PROVIDER, r.RELEASE_FEDERATION)
                          for address in group.values()})

    def test_runtime_with_and_without_public_invoker_has_exact_resources_and_pinned_secrets(self):
        for public in (False, True):
            plan = fixture(public)
            size = len(plan["resource_changes"])
            self.assertEqual(r.validate_plan(plan), {"resources": size, "creates": 11})
            for item in plan["resource_changes"]:
                item["change"]["actions"] = ["no-op"]
            self.assertEqual(r.validate_plan(plan), {"resources": size, "creates": 0})

    def test_without_runtime_settings_only_the_base_resources_are_admitted(self):
        runtime = fixture()
        base = deepcopy(runtime)
        base["variables"] = {"runtime_settings": {"value": None}, "public_invoker": {"value": False}}
        base["resource_changes"] = [item for item in runtime["resource_changes"] if item["address"] in c.resource_contract()]
        self.assertEqual(r.validate_plan(base), {"resources": len(c.resource_contract()), "creates": len(c.resource_contract())})
        self.assertEqual(set(r.plan_resources(base)), set(c.resource_contract()))
        # Runtime resources never appear without the reviewed runtime settings.
        for address in (r.SERVICE, r.MAINTENANCE_JOB, r.FIRESTORE, r.MAINTENANCE_POOL):
            rejected = deepcopy(base)
            rejected["resource_changes"].append(next(item for item in runtime["resource_changes"] if item["address"] == address))
            with self.subTest(address=address), self.assertRaisesRegex(c.ContractError, "^resource-address-mode$"):
                r.validate_plan(rejected)
        # The owner-managed public invoker can only be adopted together with the Service.
        for public in (True, "true", 1):
            rejected = deepcopy(base)
            rejected["variables"]["public_invoker"]["value"] = public
            with self.subTest(public=public), self.assertRaisesRegex(c.ContractError, "^plan-public-invoker$"):
                r.validate_plan(rejected)
        with self.assertRaisesRegex(c.ContractError, "^runtime-public-without-service$"):
            r.resource_contract(runtime=False, public=True)

    def test_runtime_desired_state_can_reconcile_exact_missing_maintenance_boundary(self):
        plan = fixture()
        maintenance = set(r.maintenance_resource_contract())
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["create"] if item["address"] in maintenance else ["no-op"]
            if item["address"] in {r.MAINTENANCE_ROLE, r.MAINTENANCE_IMAGE_ROLE} and item["change"]["actions"] == ["create"]:
                item["change"]["after"].pop("deleted", None)
        self.assertEqual(r.validate_plan(plan),
                         {"resources": len(plan["resource_changes"]), "creates": len(maintenance)})

        rejected = deepcopy(plan)
        binding = next(item for item in rejected["resource_changes"]
                       if item["address"] == r.OPERATOR_IMAGE_BINDING)
        binding["change"]["after"]["role"] = "roles/run.admin"
        with self.assertRaises(ValueError):
            r.validate_plan(rejected)

    def test_only_exact_owner_public_member_can_be_imported_without_cloud_writes(self):
        plan = fixture(public=True)
        self.assertEqual(r.public_invoker_import_config(), 'import {\n  to = ' + r.INVOKER + '\n  id = "' + r.PUBLIC_INVOKER_IMPORT_ID + '"\n}\n')
        qualified = deepcopy(plan)
        next(x['change'] for x in qualified['resource_changes'] if x['address'] == r.INVOKER)['after']['name'] = r.PUBLIC_INVOKER_NAME
        self.assertEqual(r.validate_plan(qualified), {'resources': len(qualified['resource_changes']), 'creates': 11})
        for mutate in (
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER).update(actions=['create']),
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER).update(importing={'id': 'private-other-resource'}),
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER)['after'].update(role='roles/run.admin'),
            lambda p: next(x['change'] for x in p['resource_changes'] if x['address'] == r.INVOKER)['after'].update(member='allAuthenticatedUsers'),
        ):
            rejected = deepcopy(plan)
            mutate(rejected)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(rejected)
        key = 'private-key-for-synthetic-tests-only'
        other = deepcopy(plan)
        next(x['change'] for x in other['resource_changes'] if x['address'] == r.INVOKER).pop('importing')
        self.assertNotEqual(c.commitment(plan, 'state-version', 'a' * 40, key),
                            c.commitment(other, 'state-version', 'a' * 40, key))

    def test_ci_owned_image_drift_allows_multiple_revisions_without_terraform_update(self):
        plan = fixture(public=True)
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

        job_item = next(item for item in plan["resource_changes"] if item["address"] == r.MAINTENANCE_JOB)
        job_before = deepcopy(job_item["change"]["after"])
        job_item["change"]["after"]["template"][0]["template"][0]["containers"][0]["image"] = (
            r.REPOSITORY + "@sha256:" + "d" * 64
        )
        job_drift = deepcopy(job_item)
        job_drift["change"].update(actions=["update"], before=job_before)
        plan["resource_drift"] = [drift, job_drift]
        saved = deepcopy(plan)
        self.assertEqual(r.validate_plan(plan),
                         {"resources": len(plan["resource_changes"]), "creates": 0})
        self.assertEqual(plan, saved)
        # Once Terraform has refreshed the state, it must not restore the old
        # bootstrap digest on a subsequent infrastructure Plan either.
        stable = deepcopy(plan)
        stable.pop("resource_drift")
        self.assertEqual(r.validate_plan(stable),
                         {"resources": len(plan["resource_changes"]), "creates": 0})
        def bad(mutator):
            candidate = deepcopy(plan)
            mutator(candidate)
            with self.assertRaises(ValueError):
                r.validate_plan(candidate)
        bad(lambda p: p["resource_drift"][0]["change"]["before"]["template"][0]["containers"][0]["env"][0].update(value="injected"))
        bad(lambda p: p["resource_drift"][0]["change"]["before"].update(uid="other-service"))
        bad(lambda p: p["resource_drift"][0]["change"]["after"].update(generation="1"))
        bad(lambda p: p["resource_drift"][0]["change"]["after"]["template"][0]["containers"][0].update(image="other/registry@sha256:" + "f" * 64))
        bad(lambda p: p["resource_drift"][1]["change"]["before"]["template"][0]["template"][0]["containers"][0]["env"][0].update(value="wrong-project"))
        bad(lambda p: service(p).update(actions=["update"]))
        bad(lambda p: p["resource_drift"][0]["change"]["after"]["template"][0]["containers"][0]["env"][0].update(value="unreviewed"))
        # Unknown/mismatched planned after cannot be used to bypass refresh.
        bad(lambda p: p["resource_drift"][0]["change"].update(after_unknown={"template": True}))


    def test_owner_added_hosting_backend_host_is_adopted_as_the_only_env_addition(self):
        # A new environment creates the Service before its *.a.run.app host is known.
        # The owner then adds only MYPAGE_HOSTING_BACKEND_HOST and records the same
        # value in the runtime settings; the next Plan adopts it without an update.
        plan = fixture(public=True)
        for item in plan["resource_changes"]:
            item["change"]["actions"] = ["no-op"]
            item["change"].pop("importing", None)
        settings = plan["variables"]["runtime_settings"]["value"]
        settings["hosting_backend_host"] = "mypage-api-observed-et.a.run.app"
        after = service(plan)["after"]
        before = deepcopy(after)
        before.update(uid="fixed-service-uid", generation="1")
        after.update(uid="fixed-service-uid", generation="2")
        after["template"][0]["containers"][0]["env"].append({
            "name": "MYPAGE_HOSTING_BACKEND_HOST", "value": settings["hosting_backend_host"]
        })
        drift = deepcopy(next(item for item in plan["resource_changes"] if item["address"] == r.SERVICE))
        drift["change"].update(actions=["update"], before=before)
        plan["resource_drift"] = [drift]
        self.assertEqual(r.validate_plan(deepcopy(plan)),
                         {"resources": len(plan["resource_changes"]), "creates": 0})
        for mutate in (
            # Any other env addition alongside the host stays rejected.
            lambda p: p["resource_drift"][0]["change"]["after"]["template"][0]["containers"][0]["env"].append(
                {"name": "MYPAGE_UNREVIEWED", "value": "x"}),
            # The added host must equal the reviewed runtime settings value.
            lambda p: p["resource_drift"][0]["change"]["after"]["template"][0]["containers"][0]["env"][-1].update(
                value="mypage-api-other-et.a.run.app"),
        ):
            rejected = deepcopy(plan)
            mutate(rejected)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(rejected)

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
                r.validate_plan(plan)

    def test_updates_unknowns_new_resources_and_public_access_in_private_mode_stop(self):
        for mutate in (
            lambda p: service(p).update(actions=["update"]),
            lambda p: service(p).update(after_unknown={"template": [{"containers": [{"env": True}]}]}),
            lambda p: p["resource_changes"].append(deepcopy(fixture(public=True)["resource_changes"][-1])),
            lambda p: p.update(resource_drift=[{"address": r.SERVICE}]),
        ):
            plan = fixture()
            mutate(plan)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                r.validate_plan(plan)

    def test_image_job_cannot_obtain_state_credentials_or_run_as_other_source(self):
        env = {"GITHUB_REPOSITORY": c.REPOSITORY, "GITHUB_REPOSITORY_ID": "340900071", "GITHUB_REPOSITORY_OWNER_ID": "54093651",
               "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_ACTOR": "kani3camp", "GITHUB_ACTOR_ID": "54093651",
               "GITHUB_TRIGGERING_ACTOR": "kani3camp", "GITHUB_RUN_ATTEMPT": "1", "GITHUB_REF": c.REF,
               "GITHUB_WORKFLOW_REF": c.IMAGE_COMPONENTS["api"]["caller"], "TF_WORKSPACE": "default", "GITHUB_SHA": "a" * 40,
               "MYPAGE_EXPECTED_SHA": "a" * 40, "MYPAGE_MODE": "image-build", "MYPAGE_ROLE": "apply",
               "MYPAGE_IMAGE_COMPONENT": "api",
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
