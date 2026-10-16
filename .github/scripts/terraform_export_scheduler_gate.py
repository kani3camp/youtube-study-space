"""Exact development Scheduler wave, additive to the unchanged global guard."""
from terraform_plan_diagnostic import source_field
from terraform_email_adoption_gate import canonical, has_unknown
from terraform_export_topic_gate import TOPIC_ID, validate as validate_topic

SCHEDULER = "module.export_scheduler[0].google_cloud_scheduler_job.export"
SCHEDULER_ID = "projects/test-youtube-study-space/locations/asia-southeast2/jobs/scheduledFirestoreCollectionsExport"


def validate(plan, *, phase, diagnostic=None):
    def require(value, field="CONTRACT"):
        if not value:
            if diagnostic is not None:
                diagnostic.note_failure(field)
            raise ValueError("Development Scheduler wave STOP; private diagnostic suppressed.")

    if diagnostic is not None:
        diagnostic.mark("BASELINE_SCHEDULER")
    require(phase in {"before", "post"}, "MODE")
    changes = plan.get("resource_changes", [])
    selected = [r for r in changes if r.get("address") == SCHEDULER]
    require(len(changes) == 10 and len(selected) == 1, "GRAPH")
    # All nine adopted resources must pass the existing post-adoption contract:
    # no second import, no missing/extra/generated ownership, no drift/unknowns.
    validate_topic(dict(plan, resource_changes=[r for r in changes if r.get("address") != SCHEDULER]), phase="post", diagnostic=diagnostic)
    if diagnostic is not None:
        diagnostic.mark("BASELINE_SCHEDULER")
    resource = selected[0]
    change = resource.get("change", {})
    require(resource.get("mode") == "managed" and resource.get("type") == "google_cloud_scheduler_job", "TYPE")
    require(change.get("actions") == ["no-op"] and not has_unknown(change.get("after_unknown", {})), "ACTION" if change.get("actions") != ["no-op"] else "UNKNOWN")
    require(canonical(change.get("before")) == canonical(change.get("after")), "VALUE")
    importing = change.get("importing")
    require(not importing or (phase == "before" and importing.get("id") == SCHEDULER_ID), "IMPORT")
    value = change.get("after", {})
    for key, expected in {
        "id": SCHEDULER_ID, "project": "test-youtube-study-space", "region": "asia-southeast2",
        "name": "scheduledFirestoreCollectionsExport", "schedule": "0 0 * * *", "time_zone": "Asia/Tokyo",
    }.items():
        require(value.get(key) == expected, source_field(key))
    require(value.get("paused") is False, "PAUSED")
    require(value.get("description") in (None, "") and value.get("attempt_deadline") in (None, ""), "DEADLINE")
    require(not value.get("http_target") and not value.get("app_engine_http_target"), "TARGET")
    require(value.get("retry_config") == [{"retry_count": 3, "max_retry_duration": "0s",
        "min_backoff_duration": "5s", "max_backoff_duration": "3600s", "max_doublings": 5}], "RETRY_CONFIG")
    target = value.get("pubsub_target", [])
    require(len(target) == 1 and target[0].get("topic_name") == TOPIC_ID, "TARGET")
    require(target[0].get("data") == "c3RhcnQgZXhwb3J0" and not target[0].get("attributes"), "DATA")
