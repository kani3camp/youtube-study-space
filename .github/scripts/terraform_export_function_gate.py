"""Exact Gen1 development Function import, additive to the global guard."""
from terraform_email_adoption_gate import canonical, has_unknown
from terraform_export_scheduler_gate import validate as validate_scheduler
from terraform_export_topic_gate import TOPIC_ID

FUNCTION = "module.export_function[0].google_cloudfunctions_function.export"
FUNCTION_ID = "projects/test-youtube-study-space/locations/asia-southeast2/functions/firestoreCollectionsExport"


def validate(plan, *, phase, execution_email):
    def require(value):
        if not value:
            raise ValueError("Development Function wave STOP; private diagnostic suppressed.")

    require(phase in {"before", "post"} and bool(execution_email))
    changes = plan.get("resource_changes", [])
    selected = [r for r in changes if r.get("address") == FUNCTION]
    require(len(changes) == 11 and len(selected) == 1)
    validate_scheduler(dict(plan, resource_changes=[r for r in changes if r.get("address") != FUNCTION]), phase="post")
    resource = selected[0]
    change = resource.get("change", {})
    require(resource.get("mode") == "managed" and resource.get("type") == "google_cloudfunctions_function")
    require(change.get("actions") == ["no-op"] and not has_unknown(change.get("after_unknown", {})))
    require(canonical(change.get("before")) == canonical(change.get("after")))
    importing = change.get("importing")
    require(not importing or (phase == "before" and importing.get("id") == FUNCTION_ID))
    value = change.get("after", {})
    for key, expected in {
        "id": FUNCTION_ID, "project": "test-youtube-study-space", "region": "asia-southeast2",
        "name": "firestoreCollectionsExport", "runtime": "nodejs22", "entry_point": "scheduledFirestoreExport",
        "available_memory_mb": 256, "timeout": 60, "min_instances": 0, "max_instances": 1,
        "service_account_email": execution_email, "ingress_settings": "ALLOW_ALL", "status": "ACTIVE", "version_id": "8",
        "environment_variables": {"YSS_EXPORT_ENVIRONMENT": "development", "YSS_EXPORT_PROJECT_ID": "test-youtube-study-space"},
        "effective_labels": {"deployment-tool": "cli-gcloud"},
        "event_trigger": [{"event_type": "google.pubsub.topic.publish", "resource": TOPIC_ID, "failure_policy": [{"retry": False}]}],
    }.items():
        require(value.get(key) == expected)
    require(value.get("description") in (None, ""))
    for key in ("labels", "terraform_labels", "source_archive_bucket", "source_archive_object", "source_repository",
                "https_trigger_url", "trigger_http", "vpc_connector", "vpc_connector_egress_settings", "kms_key_name",
                "docker_repository", "build_worker_pool", "build_environment_variables"):
        require(not value.get(key))
