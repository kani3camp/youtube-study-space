"""環境に固定したランタイム受入検査。認証情報・クラウド入出力・秘密値は扱わない。"""
from __future__ import annotations

from copy import deepcopy
import json
import re

import foundation_contract as c


def commit_sha(value):
    return re.fullmatch(r"[a-f0-9]{40}", value) is not None

ORIGIN = c.ORIGIN
IMAGE_REGISTRY = c.REGION + "-docker.pkg.dev/" + c.PROJECT + "/mypage/"
REPOSITORY = IMAGE_REGISTRY + "mypage-api"
OPERATOR_REPOSITORY = IMAGE_REGISTRY + "mypage-maintenance-operator"
# 移行中のJobは、旧API同梱imageと専用operator imageのどちらかを指す。
# 初回Operator配備でJobが専用packageへ移ったことを確認してから旧packageを外す。
OPERATOR_DEPLOYER = "mypage-operator-deploy@" + c.PROJECT + ".iam.gserviceaccount.com"
MAINTENANCE_JOB_IMAGE = "(?:" + re.escape(REPOSITORY) + "|" + re.escape(OPERATOR_REPOSITORY) + r")@sha256:[a-f0-9]{64}"
SERVICE = "module.platform[0].google_cloud_run_v2_service.api[0]"
FIRESTORE = "module.platform[0].google_project_iam_member.firestore_user[0]"
INVOKER = "module.platform[0].google_cloud_run_v2_service_iam_member.public_invoker[0]"
MAINTENANCE_OPERATOR_SA = "module.platform[0].google_service_account.maintenance_operator[0]"
MAINTENANCE_INVOKER_SA = "module.platform[0].google_service_account.maintenance_invoker[0]"
MAINTENANCE_ROLE = "module.platform[0].google_project_iam_custom_role.maintenance_firestore[0]"
MAINTENANCE_FIRESTORE = "module.platform[0].google_project_iam_member.maintenance_firestore[0]"
MAINTENANCE_POOL = "module.platform[0].google_iam_workload_identity_pool.maintenance_github[0]"
MAINTENANCE_PROVIDER = "module.platform[0].google_iam_workload_identity_pool_provider.maintenance_github[0]"
MAINTENANCE_WIF = "module.platform[0].google_service_account_iam_member.maintenance_workload_identity[0]"
MAINTENANCE_JOB = "module.platform[0].google_cloud_run_v2_job.maintenance_operator[0]"
MAINTENANCE_JOB_INVOKER = "module.platform[0].google_cloud_run_v2_job_iam_member.maintenance_invoker[0]"
MAINTENANCE_IMAGE_ROLE = "module.platform[0].google_project_iam_custom_role.maintenance_image_release[0]"
MAINTENANCE_IMAGE_BINDING = "module.platform[0].google_cloud_run_v2_job_iam_member.maintenance_image_release[0]"
MAINTENANCE_IMAGE_ACT_AS = "module.platform[0].google_service_account_iam_member.maintenance_image_release_act_as[0]"
OPERATOR_IMAGE_BINDING = "module.platform[0].google_cloud_run_v2_job_iam_member.operator_image_release[0]"
OPERATOR_IMAGE_ACT_AS = "module.platform[0].google_service_account_iam_member.operator_image_release_act_as[0]"
MAINTENANCE_FIXED_IAM_MEMBERS = {MAINTENANCE_FIRESTORE, MAINTENANCE_JOB_INVOKER, MAINTENANCE_IMAGE_BINDING, MAINTENANCE_IMAGE_ACT_AS,
                                 OPERATOR_IMAGE_BINDING, OPERATOR_IMAGE_ACT_AS}
MODES = {"maintenance-operator-plan": "maintenance_operator", "maintenance-operator-apply": "maintenance_operator",
         "maintenance-upgrade-plan": "maintenance_upgrade", "maintenance-upgrade-apply": "maintenance_upgrade",
         "runtime-private-plan": "runtime_private", "runtime-private-apply": "runtime_private",
         "runtime-public-plan": "runtime_public", "runtime-public-apply": "runtime_public"}
PUBLIC_INVOKER_NAME = "projects/" + c.PROJECT + "/locations/" + c.REGION + "/services/mypage-api"
PUBLIC_INVOKER_IMPORT_ID = PUBLIC_INVOKER_NAME + " roles/run.invoker allUsers"
FIELDS = {"image_digest", "project_number", "web_app_id", "public_origin", "privacy_version", "terms_version",
          "oauth_client_id_version", "oauth_client_secret_version", "youtube_api_key_version", "history_coverage_json", "image_source_sha", "hosting_backend_host", "previous_image_digest", "read_metrics_enabled", "read_metrics_from_revision"}


def maintenance_provider_condition():
    transition = "kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@" + c.REF
    return (
        "assertion.repository_id == '340900071' && assertion.repository_owner_id == '54093651' && "
        "assertion.actor_id == '54093651' && assertion.actor == 'kani3camp' && assertion.run_attempt == '1' && "
        "assertion.event_name == 'workflow_dispatch' && assertion.ref == '" + c.REF + "' && "
        "assertion.workflow_ref == '" + transition + "' && "
        "assertion.environment == 'mypage-maintenance-" + c.ENVIRONMENT + "' && "
        "assertion.workflow_sha == assertion.sha"
    )


def settings(value, stage=None):
    c.need(stage is None or stage in set(MODES.values()), "runtime-settings-stage")
    operator_only = stage == "maintenance_operator"
    optional = {"history_coverage_json", "image_source_sha", "hosting_backend_host", "previous_image_digest", "read_metrics_enabled", "read_metrics_from_revision"}
    required = {"image_digest"} if operator_only else FIELDS - optional
    c.need(type(value) is dict and set(value) <= FIELDS and set(value) >= required, "runtime-settings-fields")
    defaults = {field: "" for field in FIELDS} | {"read_metrics_enabled": False}
    # Bootstrap-only maintenance stages use the same application image as the API.
    # No per-operator digest/source fields exist in environment JSON.
    value = defaults | ({"image_digest": value["image_digest"],
                         "image_source_sha": value.get("image_source_sha", "")} if operator_only else value)
    # JSONの受入検査は、意図的にTerraformの型変換より厳しくする。
    c.need(type(value["read_metrics_enabled"]) is bool, "runtime-read-metrics-type")
    c.need(c.TARGET == "development" or (not value["read_metrics_enabled"] and value["read_metrics_from_revision"] == ""), "runtime-production-read-metrics")
    c.need(all(type(v) is str for k, v in value.items() if k != "read_metrics_enabled"), "runtime-settings-types")
    c.need(re.fullmatch(re.escape(REPOSITORY) + r"@sha256:[a-f0-9]{64}", value["image_digest"]), "runtime-image")
    c.need(value["image_source_sha"] == "" or re.fullmatch(r"[a-f0-9]{40}", value["image_source_sha"]), "runtime-image-source-sha")
    if not operator_only:
        c.need(value["project_number"] == c.PROJECT_NUMBER, "runtime-project-number")
        c.need(re.fullmatch("1:" + value["project_number"] + r":web:[A-Za-z0-9]+", value["web_app_id"]), "runtime-app-project")
        c.need(value["public_origin"] == ORIGIN, "runtime-origin")
        for field in ("privacy_version", "terms_version"):
            c.need(re.fullmatch(r"[A-Za-z0-9._-]{1,64}", value[field]), "runtime-policy-version")
        for field in ("oauth_client_id_version", "oauth_client_secret_version", "youtube_api_key_version"):
            c.need(re.fullmatch(r"[1-9][0-9]*", value[field]), "runtime-secret-version")
        c.need(value["hosting_backend_host"] == "" or re.fullmatch(r"mypage-api-[a-z0-9-]+\.a\.run\.app", value["hosting_backend_host"]), "runtime-hosting-backend")
        c.need(value["previous_image_digest"] == "" or re.fullmatch(re.escape(REPOSITORY) + r"@sha256:[a-f0-9]{64}", value["previous_image_digest"]), "runtime-previous-image")
        c.need(value["read_metrics_from_revision"] == "" or re.fullmatch(r"mypage-api-[a-z0-9-]{1,50}", value["read_metrics_from_revision"]), "runtime-read-metrics-revision")
        coverage = value["history_coverage_json"]
        c.need(len(coverage.encode()) <= 16384, "runtime-coverage-size")
        if coverage:
            c.need(type(json.loads(coverage)) is list, "runtime-coverage-array")
    return value


def maintenance_resource_contract(value=None):
    operator_email = "mypage-maintenance-job@" + c.PROJECT + ".iam.gserviceaccount.com"
    invoker_email = "mypage-maintenance-invoker@" + c.PROJECT + ".iam.gserviceaccount.com"
    resources = {
        MAINTENANCE_OPERATOR_SA: {"project": c.PROJECT, "account_id": "mypage-maintenance-job",
                                  "display_name": "MyPage maintenance state operator",
                                  "description": "Keyless Cloud Run Job identity for the fixed maintenance operator.",
                                  "disabled": False, "deletion_policy": "PREVENT"},
        MAINTENANCE_INVOKER_SA: {"project": c.PROJECT, "account_id": "mypage-maintenance-invoker",
                                 "display_name": "MyPage maintenance GitHub invoker",
                                 "description": "Federated identity limited to invoking the maintenance Job.",
                                 "disabled": False, "deletion_policy": "PREVENT"},
        MAINTENANCE_ROLE: {"project": c.PROJECT, "role_id": "mypageMaintenanceOperator",
                           "title": "MyPage maintenance control writer",
                           "description": "Minimum Firestore entity permissions for the maintenance control document.",
                           "stage": "GA", "deletion_policy": "PREVENT",
                           "permissions": ["datastore.databases.get", "datastore.entities.create", "datastore.entities.get", "datastore.entities.update"]},
        MAINTENANCE_FIRESTORE: {"project": c.PROJECT, "role": "projects/" + c.PROJECT + "/roles/mypageMaintenanceOperator", "member": "serviceAccount:" + operator_email},
        MAINTENANCE_POOL: {"project": c.PROJECT, "workload_identity_pool_id": "mypage-maintenance", "display_name": "MyPage maintenance GitHub",
                           "description": "Environment-scoped GitHub Actions trust for maintenance Job invocation.",
                           "disabled": False, "mode": "FEDERATION_ONLY", "deletion_policy": "PREVENT"},
        MAINTENANCE_PROVIDER: {"project": c.PROJECT, "workload_identity_pool_id": "mypage-maintenance", "workload_identity_pool_provider_id": "github",
                               "display_name": "MyPage maintenance workflow",
                               "description": "GitHub OIDC provider restricted to the reviewed maintenance workflow.",
                               "disabled": False, "deletion_policy": "PREVENT"},
        MAINTENANCE_WIF: {"role": "roles/iam.workloadIdentityUser"},
        MAINTENANCE_JOB: {"project": c.PROJECT, "name": "mypage-maintenance-operator", "location": c.REGION,
                          "deletion_policy": "PREVENT", "deletion_protection": True, "launch_stage": "GA"},
        MAINTENANCE_JOB_INVOKER: {"project": c.PROJECT, "location": c.REGION, "name": "mypage-maintenance-operator", "role": "roles/run.jobsExecutorWithOverrides", "member": "serviceAccount:" + invoker_email},
    }
    # The backend image deployer is intentionally DEV-only until a separate
    # Production release identity is approved. Never create a dangling prod
    # principal or silently grant a future same-name service account access.
    if c.TARGET == "development":
        resources |= {
            MAINTENANCE_IMAGE_ROLE: {"project": c.PROJECT, "role_id": "mypageMaintenanceImageRelease",
                                     "title": "MyPage maintenance image release",
                                     "description": "Allows the existing backend deployer to read and update only the maintenance Cloud Run Job.",
                                     "stage": "GA", "deletion_policy": "PREVENT",
                                     "permissions": ["run.jobs.get", "run.jobs.update"]},
            MAINTENANCE_IMAGE_BINDING: {"project": c.PROJECT, "location": c.REGION, "name": "mypage-maintenance-operator",
                                        "role": "projects/" + c.PROJECT + "/roles/mypageMaintenanceImageRelease",
                                        "member": "serviceAccount:mypage-backend-deploy@" + c.PROJECT + ".iam.gserviceaccount.com"},
            MAINTENANCE_IMAGE_ACT_AS: {"service_account_id": "projects/" + c.PROJECT + "/serviceAccounts/" + operator_email,
                                       "role": "roles/iam.serviceAccountUser",
                                       "member": "serviceAccount:mypage-backend-deploy@" + c.PROJECT + ".iam.gserviceaccount.com"},
            # 専用Operator deployer。旧Backend deployerの2 bindingは、初回Operator配備の確認後に
            # 削除専用の遷移で外す（このcontractはcreate/no-opだけを許可するため）。
            OPERATOR_IMAGE_BINDING: {"project": c.PROJECT, "location": c.REGION, "name": "mypage-maintenance-operator",
                                     "role": "projects/" + c.PROJECT + "/roles/mypageMaintenanceImageRelease",
                                     "member": "serviceAccount:" + OPERATOR_DEPLOYER},
            OPERATOR_IMAGE_ACT_AS: {"service_account_id": "projects/" + c.PROJECT + "/serviceAccounts/" + operator_email,
                                    "role": "roles/iam.serviceAccountUser",
                                    "member": "serviceAccount:" + OPERATOR_DEPLOYER},
        }
    if value is not None:
        resources[MAINTENANCE_PROVIDER] |= {
            "attribute_condition": maintenance_provider_condition(),
            "attribute_mapping": {
                "attribute.actor": "assertion.actor", "attribute.actor_id": "assertion.actor_id",
                "attribute.environment": "assertion.environment",
                "attribute.event_name": "assertion.event_name", "attribute.ref": "assertion.ref",
                "attribute.repository_id": "assertion.repository_id", "attribute.repository_owner_id": "assertion.repository_owner_id",
                "attribute.run_attempt": "assertion.run_attempt", "attribute.sha": "assertion.sha",
                "attribute.workflow_ref": "assertion.workflow_ref", "attribute.workflow_sha": "assertion.workflow_sha",
                "google.subject": "assertion.sub",
            },
            "oidc": [{"issuer_uri": "https://token.actions.githubusercontent.com", "allowed_audiences": []}],
        }
        resources[MAINTENANCE_WIF] |= {
            "service_account_id": "projects/" + c.PROJECT + "/serviceAccounts/" + invoker_email,
            "member": "principalSet://iam.googleapis.com/projects/" + c.PROJECT_NUMBER + "/locations/global/workloadIdentityPools/mypage-maintenance/attribute.workflow_ref/kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@" + c.REF,
        }
        resources[MAINTENANCE_JOB]["template"] = [{
            "task_count": 1, "parallelism": 1,
            "template": [{
                "service_account": operator_email, "max_retries": 0, "timeout": "60s",
                "execution_environment": "EXECUTION_ENVIRONMENT_GEN2",
                "containers": [{
                    "image": value["image_digest"], "command": ["/app/mypage-maintenance-operator"], "args": [],
                    "resources": [{"limits": {"cpu": "1", "memory": "512Mi"}}],
                    "env": [
                        {"name": "GOOGLE_CLOUD_PROJECT", "value": c.PROJECT},
                        {"name": "MYPAGE_ENVIRONMENT", "value": c.ENVIRONMENT},
                    ],
                }],
            }],
        }]
    return resources


def resource_contract(stage, value=None):
    c.need(stage in {"maintenance_operator", "maintenance_upgrade", "runtime_private", "runtime_public"}, "runtime-stage")
    base_stage = "registry"
    resources = c.resource_contract(base_stage) | maintenance_resource_contract(value)
    if stage == "maintenance_operator":
        return resources
    resources |= {
        FIRESTORE: {"project": c.PROJECT, "role": "roles/datastore.user", "member": "serviceAccount:" + c.EMAIL},
        SERVICE: {"project": c.PROJECT, "name": "mypage-api", "location": c.REGION,
                  "ingress": "INGRESS_TRAFFIC_ALL", "deletion_policy": "PREVENT",
                  "deletion_protection": True, "invoker_iam_disabled": False, "launch_stage": "GA"},
    }
    if stage in {"maintenance_upgrade", "runtime_public"}:
        resources[INVOKER] = {"project": c.PROJECT, "location": c.REGION, "name": "mypage-api",
                              "role": "roles/run.invoker", "member": "allUsers"}
    return resources


def public_invoker_import_config():
    # 取り込めるのは承認済みのこのメンバーだけとし、入力値は埋め込まない。
    return 'import {\n  to = ' + INVOKER + '\n  id = "' + PUBLIC_INVOKER_IMPORT_ID + '"\n}\n'


def plain_environment(value):
    env = {"GOOGLE_CLOUD_PROJECT": c.PROJECT, "MYPAGE_ENVIRONMENT": c.ENVIRONMENT, "MYPAGE_REGION": c.REGION,
           "MYPAGE_SIGNER_EMAIL": c.EMAIL, "MYPAGE_INFRASTRUCTURE_READY": "true"}
    for name, field in (("MYPAGE_PROJECT_NUMBER", "project_number"), ("MYPAGE_WEB_APP_ID", "web_app_id"),
                        ("MYPAGE_PUBLIC_ORIGIN", "public_origin"), ("MYPAGE_PRIVACY_VERSION", "privacy_version"),
                        ("MYPAGE_TERMS_VERSION", "terms_version")):
        env[name] = value[field]
    if value["history_coverage_json"]:
        env["MYPAGE_HISTORY_COVERAGE"] = value["history_coverage_json"]
    if value["hosting_backend_host"]:
        env["MYPAGE_HOSTING_BACKEND_HOST"] = value["hosting_backend_host"]
    if value["read_metrics_enabled"]:
        env["MYPAGE_READ_METRICS"] = "true"
    return env


def one(value, check):
    c.need(type(value) is list and len(value) == 1 and type(value[0]) is dict, check)
    return value[0]


def empty_options(value, names):
    for name in names:
        c.need(value.get(name) in (None, "", False, 0, [], {}), "runtime-unsupported-option")


def validate_planned_revision(after, desired, service, actions, check):
    """作成時はリビジョン固定を認めず、no-opでは完全一致するReadyリビジョンだけを公開できる。"""
    if not desired:
        return
    c.need(actions == ["no-op"] and type(desired) is str, check)
    ready = after.get("latest_ready_revision")
    prefix = "projects/" + c.PROJECT + "/locations/" + c.REGION + "/services/" + service + "/revisions/"
    if type(ready) is str and ready.startswith(prefix):
        ready = ready[len(prefix):]
    c.need(type(ready) is str and desired == ready
           and re.fullmatch(re.escape(service) + r"-[a-z0-9-]{1,50}", desired), check)


def validate_maintenance_resource(address, after, unknown, value, actions, bootstrap_image=None):
    operator_email = "mypage-maintenance-job@" + c.PROJECT + ".iam.gserviceaccount.com"
    invoker_email = "mypage-maintenance-invoker@" + c.PROJECT + ".iam.gserviceaccount.com"
    if address in {MAINTENANCE_OPERATOR_SA, MAINTENANCE_INVOKER_SA}:
        c.need(after.get("disabled") is False and after.get("deletion_policy") == "PREVENT",
               "maintenance-service-account-disabled")
        return
    if address in {MAINTENANCE_ROLE, MAINTENANCE_IMAGE_ROLE}:
        # deletedはプロバイダーの計算値なので、作成時だけunknownになる。
        # 既存ロールのno-opでは、毎回既知のfalseでなければならない。
        c.need(after.get("deletion_policy") == "PREVENT" and
               (after.get("deleted") is None if actions == ["create"] else after.get("deleted") in {None, False}),
               "maintenance-role-deleted")
        return
    if address == MAINTENANCE_POOL:
        c.need(after.get("display_name") == "MyPage maintenance GitHub", "maintenance-pool-display")
        return
    if address == MAINTENANCE_PROVIDER:
        c.need(not c.has_unknown({k: unknown.get(k) for k in ("attribute_condition", "attribute_mapping", "oidc")}), "maintenance-provider-unknown")
        c.need(after.get("attribute_condition") == maintenance_provider_condition(), "maintenance-provider-condition")
        c.need(after.get("attribute_mapping") == {
            "attribute.actor": "assertion.actor",
            "attribute.actor_id": "assertion.actor_id",
            "attribute.environment": "assertion.environment",
            "attribute.event_name": "assertion.event_name",
            "attribute.ref": "assertion.ref",
            "attribute.repository_id": "assertion.repository_id",
            "attribute.repository_owner_id": "assertion.repository_owner_id",
            "attribute.run_attempt": "assertion.run_attempt",
            "attribute.sha": "assertion.sha",
            "attribute.workflow_ref": "assertion.workflow_ref",
            "attribute.workflow_sha": "assertion.workflow_sha",
            "google.subject": "assertion.sub",
        }, "maintenance-provider-mapping")
        oidc = one(after.get("oidc"), "maintenance-provider-oidc")
        empty_options(oidc, ("jwks_json",))
        c.need(oidc.get("issuer_uri") == "https://token.actions.githubusercontent.com"
               and oidc.get("allowed_audiences") in (None, []), "maintenance-provider-issuer")
        return
    if address == MAINTENANCE_WIF:
        c.need(after.get("service_account_id") in {
            "projects/" + c.PROJECT + "/serviceAccounts/" + invoker_email,
            "projects/-/serviceAccounts/" + invoker_email,
        }, "maintenance-wif-service-account")
        expected_member = "principalSet://iam.googleapis.com/projects/" + c.PROJECT_NUMBER + "/locations/global/workloadIdentityPools/mypage-maintenance/attribute.workflow_ref/kani3camp/youtube-study-space/.github/workflows/mypage-maintenance.yml@" + c.REF
        c.need(after.get("member") == expected_member, "maintenance-wif-member")
        return
    if address in MAINTENANCE_FIXED_IAM_MEMBERS:
        c.need(not c.has_unknown(unknown.get("condition")), "maintenance-iam-unknown")
        return
    if address in {MAINTENANCE_JOB}:
        template = one(after.get("template"), "maintenance-job-template")
        c.need(template.get("task_count") == 1 and template.get("parallelism") == 1, "maintenance-job-single-task")
        task = one(template.get("template"), "maintenance-job-task")
        c.need(task.get("service_account") == operator_email and task.get("max_retries") == 0 and task.get("timeout") == "60s"
               and task.get("execution_environment") == "EXECUTION_ENVIRONMENT_GEN2", "maintenance-job-runtime")
        container = one(task.get("containers"), "maintenance-job-container")
        expected_command = "/app/mypage-maintenance-operator"
        image = container.get("image")
        c.need(type(image) is str and re.fullmatch(MAINTENANCE_JOB_IMAGE, image),
               "maintenance-job-image")
        if actions == ["create"]:
            c.need(image == (bootstrap_image or value["image_digest"]), "maintenance-job-bootstrap-image")
        c.need(container.get("command") == [expected_command] and not container.get("args"), "maintenance-job-image-command")
        environment = container.get("env")
        c.need(type(environment) is list and all(type(item) is dict for item in environment), "maintenance-job-environment")
        env = {item.get("name"): item.get("value") for item in environment}
        c.need(len(env) == len(environment), "maintenance-job-environment")
        c.need(env == {"GOOGLE_CLOUD_PROJECT": c.PROJECT, "MYPAGE_ENVIRONMENT": c.ENVIRONMENT}, "maintenance-job-environment")
        resources = one(container.get("resources"), "maintenance-job-resources")
        c.need(resources.get("limits") == {"cpu": "1", "memory": "512Mi"}, "maintenance-job-limits")
        return
def maintenance_job_image(job):
    template = one(job.get("template"), "maintenance-job-template")
    task = one(template.get("template"), "maintenance-job-task")
    container = one(task.get("containers"), "maintenance-job-container")
    image = container.get("image")
    c.need(type(image) is str and re.fullmatch(MAINTENANCE_JOB_IMAGE, image),
           "maintenance-job-image")
    return image


def empty_list(value):
    """Treat the provider's null and [] as the same unset list; any other value is kept as-is."""
    return [] if value is None else value


def maintenance_job_configuration(job):
    """Return only Terraform-owned Job configuration; ignore execution/status metadata."""
    template = one(job.get("template"), "maintenance-job-template")
    task = one(template.get("template"), "maintenance-job-task")
    container = one(task.get("containers"), "maintenance-job-container")
    return {
        "project": job.get("project"),
        "name": job.get("name"),
        "location": job.get("location"),
        "deletion_policy": job.get("deletion_policy"),
        "deletion_protection": job.get("deletion_protection"),
        "launch_stage": job.get("launch_stage"),
        "template": {
            "task_count": template.get("task_count"),
            "parallelism": template.get("parallelism"),
            "task": {
                "service_account": task.get("service_account"),
                "max_retries": task.get("max_retries"),
                "timeout": task.get("timeout"),
                "execution_environment": task.get("execution_environment"),
                "container": {
                    "command": container.get("command"),
                    # providerは作成直後のStateでnull、refresh後に[]を返す。どちらも「指定なし」。
                    "args": empty_list(container.get("args")),
                    "resources": container.get("resources"),
                    "env": container.get("env"),
                },
            },
        },
    }


def validate_cicd_job_image_refresh(before, after, value):
    """Accept only CI-owned maintenance Job image refresh; executions may change output metadata."""
    c.need(type(before) is dict and type(after) is dict, "maintenance-job-refresh-shape")
    old_image, new_image = maintenance_job_image(before), maintenance_job_image(after)
    validate_maintenance_resource(MAINTENANCE_JOB, before, {}, value, ["no-op"])
    validate_maintenance_resource(MAINTENANCE_JOB, after, {}, value, ["no-op"])
    c.need(maintenance_job_configuration(before) == maintenance_job_configuration(after),
           "maintenance-job-refresh-config")
    return new_image



def validate_service(after, unknown, value, actions):
    # security入力だけknownを要求。revision等のprovider出力はTerraformに任せる。
    for template_mask in unknown.get("template", []) or []:
        c.need(not c.has_unknown({key: template_mask.get(key) for key in ("service_account", "timeout", "max_instance_request_concurrency", "execution_environment", "scaling")}), "runtime-config-unknown")
        for container_mask in template_mask.get("containers", []) or []:
            c.need(not c.has_unknown({key: container_mask.get(key) for key in ("image", "env", "command", "args", "resources")}), "runtime-config-unknown")
    scaling = one(after.get("scaling"), "runtime-service-scaling")
    c.need(scaling.get("min_instance_count") == 0 and scaling.get("max_instance_count") == (1 if c.TARGET == "development" else 3)
           and scaling.get("scaling_mode") == "AUTOMATIC", "runtime-service-capacity")
    empty_options(scaling, ("manual_instance_count",))
    traffic = one(after.get("traffic"), "runtime-traffic")
    c.need(traffic.get("type") == "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST" and traffic.get("percent") == 100, "runtime-latest-traffic")
    empty_options(traffic, ("tag",))
    validate_planned_revision(after, traffic.get("revision"), "mypage-api", actions, "runtime-traffic-revision")
    template = one(after.get("template"), "runtime-template")
    c.need(template.get("service_account") == c.EMAIL and template.get("timeout") == "30s"
           and template.get("max_instance_request_concurrency") == 40
           and template.get("execution_environment") == "EXECUTION_ENVIRONMENT_GEN2", "runtime-template-boundary")
    validate_planned_revision(after, template.get("revision"), "mypage-api", actions, "runtime-owned-revision")
    scaling = one(template.get("scaling"), "runtime-revision-scaling")
    c.need(scaling.get("min_instance_count") == 0 and scaling.get("max_instance_count") == (1 if c.TARGET == "development" else 3), "runtime-revision-capacity")
    container = one(template.get("containers"), "runtime-container")
    c.need(container.get("image") == value["image_digest"], "runtime-image-value")
    empty_options(container, ("command", "args"))
    probe = one(container.get("startup_probe"), "runtime-startup-probe")
    c.need(probe.get("initial_delay_seconds") == 0 and probe.get("timeout_seconds") == 1
           and probe.get("period_seconds") == 10 and probe.get("failure_threshold") == 3
           and probe.get("tcp_socket") == [{"port": 8080}], "runtime-startup-probe-value")
    port = one(container.get("ports"), "runtime-port")
    c.need(port == {"name": "http1", "container_port": 8080}, "runtime-port-value")
    resources = one(container.get("resources"), "runtime-resources")
    c.need(resources.get("limits") == {"cpu": "1", "memory": "512Mi"}
           and resources.get("cpu_idle") is True and resources.get("startup_cpu_boost") is False, "runtime-billing-resources")
    env = container.get("env")
    c.need(type(env) is list and all(type(e) is dict for e in env), "runtime-environment-shape")
    by_name = {e.get("name"): e for e in env}
    plain = plain_environment(value)
    secret = {"MYPAGE_OAUTH_CLIENT_ID": ("mypage-oauth-client-id", "oauth_client_id_version"),
              "MYPAGE_OAUTH_CLIENT_SECRET": ("mypage-oauth-client-secret", "oauth_client_secret_version"),
              "MYPAGE_YOUTUBE_API_KEY": ("mypage-youtube-api-key", "youtube_api_key_version")}
    c.need(len(by_name) == len(env) and set(by_name) == set(plain) | set(secret), "runtime-environment-set")
    for name, expected in plain.items():
        c.need(by_name[name].get("value") == expected and not by_name[name].get("value_source"), "runtime-environment-value")
    for name, (secret_id, field) in secret.items():
        c.need(not by_name[name].get("value"), "runtime-no-secret-payload")
        source = one(by_name[name].get("value_source"), "runtime-secret-source")
        c.need(one(source.get("secret_key_ref"), "runtime-secret-ref") == {"secret": secret_id, "version": value[field]}, "runtime-secret-pinned")


def service_image(service):
    container = one(one(service.get("template"), "runtime-image-template").get("containers"),
                    "runtime-image-container")
    image = container.get("image")
    # Terraform owns only the initial image. Every subsequent revision must be
    # an immutable digest in the fixed, environment-specific Artifact Registry.
    c.need(type(image) is str and re.fullmatch(
        re.escape(REPOSITORY) + r"@sha256:[a-f0-9]{64}", image),
        "runtime-serving-image")
    return image


def validate_cicd_image_refresh(before, after, value):
    """Allow CI-owned image revisions, never Terraform-owned config changes.

    Provider-computed revision metadata belongs to Terraform's native schema.
    Validate actual security configuration and relevant image provenance instead
    of restoring the retired full-field allowlist.
    """
    c.need(type(before) is dict and type(after) is dict, "runtime-refresh-shape")
    old_image, new_image = service_image(before), service_image(after)
    old_config = value | {"image_digest": old_image}
    old_env = one(one(before.get("template"), "runtime-refresh-template").get("containers"),
                  "runtime-refresh-container").get("env", [])
    c.need(type(old_env) is list, "runtime-refresh-env")
    if not any(entry.get("name") == "MYPAGE_HOSTING_BACKEND_HOST" for entry in old_env):
        # Historical initial Hosting binding is the sole permitted env addition.
        old_config["hosting_backend_host"] = ""
    validate_service(before, {}, old_config, ["no-op"])
    validate_service(after, {}, value | {"image_digest": new_image}, ["no-op"])
    # Keep only the configured security boundary stable. Revision/etag and
    # provider-normalized computed fields may legitimately differ.
    for key in ("project", "name", "location", "ingress", "deletion_policy",
                "deletion_protection", "invoker_iam_disabled", "launch_stage"):
        c.need(before.get(key) == after.get(key), "runtime-refresh-config")
    old_template = one(before.get("template"), "runtime-refresh-template")
    new_template = one(after.get("template"), "runtime-refresh-template")
    for key in ("service_account", "timeout", "max_instance_request_concurrency",
                "execution_environment", "scaling"):
        c.need(old_template.get(key) == new_template.get(key), "runtime-refresh-template-config")
    old_container = one(old_template.get("containers"), "runtime-refresh-container")
    new_container = one(new_template.get("containers"), "runtime-refresh-container")
    for key in ("ports", "resources", "startup_probe"):
        c.need(old_container.get(key) == new_container.get(key), "runtime-refresh-container-config")
    old_env, new_env = old_container.get("env"), new_container.get("env")
    c.need(type(new_env) is list, "runtime-refresh-env")
    if old_config["hosting_backend_host"] == "":
        # Permit only the original fixed Hosting host, not arbitrary env drift.
        c.need(old_env == [item for item in new_env
                           if item.get("name") != "MYPAGE_HOSTING_BACKEND_HOST"],
               "runtime-refresh-env")
    else:
        c.need(old_env == new_env, "runtime-refresh-env")
    if old_image != new_image:
        c.need(type(before.get("uid")) is str and before["uid"] == after.get("uid") and before["uid"],
               "runtime-image-identity")
        c.need(str(before.get("generation", "")).isdigit() and
               str(after.get("generation", "")).isdigit() and
               int(after["generation"]) > int(before["generation"]), "runtime-image-generation")
    return new_image


def validate_plan(plan, stage):
    value = settings(plan.get("variables", {}).get("runtime_settings", {}).get("value"), stage)
    if value["read_metrics_from_revision"]:
        from read_metrics_transition import validate_plan as validate_transition
        return validate_transition(plan, stage, value)
    return validate_initial_plan(plan, stage)


# Terraform-side attributes the Service may adopt without a remote write.
# #1466 added deletion_policy after the last DEV Apply, so the existing State
# holds no value or the provider default; Terraform plans an update that only
# records PREVENT in State (provider 8.5.0 skips the API update for it).
STATE_ONLY_SERVICE_ATTRIBUTES = {"deletion_policy": ({None, "DELETE"}, "PREVENT")}


def state_only_service_update(address, change):
    """Return the adopted attribute names for the reviewed Service State-only update, else None."""
    if (address != SERVICE or change.get("actions") != ["update"] or change.get("replace_paths")
            or change.get("importing")):
        return None
    before, after, unknown = change.get("before"), change.get("after"), change.get("after_unknown") or {}
    if type(before) is not dict or type(after) is not dict or type(unknown) is not dict or c.has_unknown(unknown):
        return None
    changed = {key for key in set(before) | set(after) if before.get(key) != after.get(key)}
    if not changed or not changed <= set(STATE_ONLY_SERVICE_ATTRIBUTES):
        return None
    if any(before.get(key) not in STATE_ONLY_SERVICE_ATTRIBUTES[key][0] or
           after.get(key) != STATE_ONLY_SERVICE_ATTRIBUTES[key][1] for key in changed):
        return None
    return changed


def validate_initial_plan(plan, stage):
    value = settings(plan.get("variables", {}).get("runtime_settings", {}).get("value"), stage)
    c.need(plan.get("variables", {}).get("stage", {}).get("value") == stage, "runtime-plan-stage")
    expected = resource_contract(stage, value)
    changes = plan.get("resource_changes", [])
    c.need(type(changes) is list, "runtime-plan-resources")
    base_stage = "registry"
    base = set(c.resource_contract(base_stage))
    foundation = deepcopy(plan)
    foundation["variables"]["stage"]["value"] = base_stage
    foundation["resource_changes"] = [item for item in changes if item.get("address") in base]
    foundation["resource_drift"] = []
    c.validate_refresh(plan, expected, state_only_service_update)
    # Native refresh must remain a no-op. Only the CI/CD-owned Service/Job
    # images and provider-computed metadata may drift; never mask config changes.
    for observation in plan.get("resource_drift", []) or []:
        change = observation.get("change", {})
        if observation.get("address") == SERVICE:
            c.need(change.get("actions") == ["update"] and
                   not c.has_unknown(change.get("after_unknown")), "runtime-refresh-action")
            validate_cicd_image_refresh(change.get("before"), change.get("after"), value)
        elif observation.get("address") == MAINTENANCE_JOB:
            c.need(change.get("actions") == ["update"] and
                   not c.has_unknown(change.get("after_unknown")), "runtime-refresh-action")
            validate_cicd_job_image_refresh(change.get("before"), change.get("after"), value)
    c.validate_plan(foundation, base_stage)
    bootstrap_input = plan.get("variables", {}).get("maintenance_bootstrap_image_digest", {}).get("value", "")
    c.need(bootstrap_input == "" or
           (c.TARGET == "development" and re.fullmatch(
               re.escape(REPOSITORY) + r"@sha256:[a-f0-9]{64}", bootstrap_input)),
           "maintenance-bootstrap-image")
    service_after = next((item.get("change", {}).get("after") for item in changes
                          if item.get("address") == SERVICE), None)
    maintenance_bootstrap_image = (bootstrap_input or
                                   (service_image(service_after)
                                    if type(service_after) is dict else value["image_digest"]))
    seen = set()
    for item in changes:
        address = item.get("address")
        c.need(address in expected and address not in seen, "runtime-resource-address")
        seen.add(address)
        if address in base:
            continue
        c.need(item.get("mode") == "managed" and item.get("provider_name") == "registry.terraform.io/hashicorp/google", "runtime-resource-provider")
        change = item.get("change", {})
        # 初回配備とno-opだけを許可する。配信リビジョンの更新には別途レビューした遷移を使い、
        # この検査の許可範囲を暗黙に広げない。例外は、remote writeを伴わない
        # ServiceのState取り込み（state_only_service_update）だけで、no-opとして検証する。
        state_only = bool(state_only_service_update(address, change))
        actions = ["no-op"] if state_only else change.get("actions")
        c.need(actions in [["create"], ["no-op"]] and not change.get("replace_paths"), "runtime-resource-action")
        # desired-state runtime stages may create an exact missing maintenance
        # boundary. Foreign same-name resources are rejected before Apply and
        # all maintenance fields remain constrained by the contract below.
        if stage == "maintenance_upgrade" and address not in maintenance_resource_contract():
            c.need(actions == ["no-op"], "maintenance-upgrade-runtime-noop")
        importing = change.get("importing")
        if address == INVOKER:
            c.need(stage in {"maintenance_upgrade", "runtime_public"} and change.get("actions") == ["no-op"], "runtime-invoker-owner-managed")
            if importing:
                c.need(stage == "runtime_public" and importing == {"id": PUBLIC_INVOKER_IMPORT_ID}, "runtime-invoker-import")
        else:
            c.need(not importing, "runtime-resource-action")
        after, unknown = change.get("after"), change.get("after_unknown", {})
        c.need(type(after) is dict and not after.get("condition"), "runtime-resource-after")
        for key, expected_value in expected[address].items():
            if address in {MAINTENANCE_JOB} and key == "template":
                continue
            if address == MAINTENANCE_PROVIDER and key in {"attribute_condition", "attribute_mapping", "oidc"}:
                continue
            if address == MAINTENANCE_WIF and key in {"service_account_id", "member"}:
                continue
            if address == INVOKER and key == "name":
                valid = after.get(key) in {expected_value, PUBLIC_INVOKER_NAME}
            else:
                valid = after.get(key) == expected_value
            c.need(valid and not c.has_unknown(unknown.get(key)), "runtime-resource-value-" + address + "-" + key)
        if address in maintenance_resource_contract():
            validate_maintenance_resource(address, after, unknown, value, change.get("actions"),
                                          maintenance_bootstrap_image)
        elif address == SERVICE:
            if actions == ["no-op"]:
                # ignore_changes makes the actual serving digest authoritative.
                # Never compare it to the stale initial bootstrap digest or
                # trigger an infrastructure rollback.
                validate_service(after, unknown, value | {"image_digest": service_image(after)}, actions)
            else:
                validate_service(after, unknown, value, change.get("actions"))
    c.need(seen == set(expected), "runtime-resource-set")
    return {"resources": len(seen), "creates": sum(item["change"]["actions"] == ["create"] for item in changes)}
