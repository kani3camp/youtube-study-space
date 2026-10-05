"""Exact development Scheduler wave, additive to the unchanged global guard."""
from terraform_email_adoption_gate import canonical, has_unknown
from terraform_export_topic_gate import TOPIC_ID, validate as validate_topic

SCHEDULER = "module.export_scheduler[0].google_cloud_scheduler_job.export"
SCHEDULER_ID = "projects/test-youtube-study-space/locations/asia-southeast2/jobs/scheduledFirestoreCollectionsExport"


def validate(plan, *, phase):
    def require(value):
        if not value:
            raise ValueError("Development Scheduler wave STOP; private diagnostic suppressed.")

    require(phase in {"before", "post"})
    changes = plan.get("resource_changes", [])
    selected = [r for r in changes if r.get("address") == SCHEDULER]
    require(len(changes) == 10 and len(selected) == 1)
    # All nine adopted resources must pass the existing post-adoption contract:
    # no second import, no missing/extra/generated ownership, no drift/unknowns.
    validate_topic(dict(plan, resource_changes=[r for r in changes if r.get("address") != SCHEDULER]), phase="post")
    resource = selected[0]
    change = resource.get("change", {})
    require(resource.get("mode") == "managed" and resource.get("type") == "google_cloud_scheduler_job")
    require(change.get("actions") == ["no-op"] and not has_unknown(change.get("after_unknown", {})))
    require(canonical(change.get("before")) == canonical(change.get("after")))
    importing = change.get("importing")
    require(not importing or (phase == "before" and importing.get("id") == SCHEDULER_ID))
    value = change.get("after", {})
    for key, expected in {
        "id": SCHEDULER_ID, "project": "test-youtube-study-space", "region": "asia-southeast2",
        "name": "scheduledFirestoreCollectionsExport", "schedule": "0 0 * * *", "time_zone": "Asia/Tokyo",
    }.items():
        require(value.get(key) == expected)
    require(value.get("paused") is False)
    require(value.get("description") in (None, "") and value.get("attempt_deadline") in (None, ""))
    require(not value.get("http_target") and not value.get("app_engine_http_target"))
    require(value.get("retry_config") == [{"retry_count": 3, "max_retry_duration": "0s",
        "min_backoff_duration": "5s", "max_backoff_duration": "3600s", "max_doublings": 5}])
    target = value.get("pubsub_target", [])
    require(len(target) == 1 and target[0].get("topic_name") == TOPIC_ID)
    require(target[0].get("data") == "c3RhcnQgZXhwb3J0" and not target[0].get("attributes"))
