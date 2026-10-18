"""Exact development topic wave; additive to the unchanged import-only guard."""
from terraform_plan_diagnostic import source_field
from terraform_email_adoption_gate import canonical, has_unknown
from terraform_quota_create_gate import ADDRESSES, MANAGED

TOPIC = "module.export_topic[0].google_pubsub_topic.export"
TOPIC_ID = "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"


def validate(plan, *, phase, diagnostic=None):
    def require(value, field="CONTRACT"):
        if not value:
            if diagnostic is not None:
                diagnostic.note_failure(field)
            raise ValueError("Development topic wave STOP; private diagnostic suppressed.")

    if diagnostic is not None:
        diagnostic.mark("BASELINE_TOPIC")
    require(phase in {"before", "post"}, "MODE")
    require(plan.get("complete") is True and plan.get("errored") is False, "COMPLETE")
    require(not plan.get("resource_drift") and not plan.get("deferred_changes"), "EMPTY_LIST")
    require(all(c.get("status") == "pass" for c in plan.get("checks", [])), "CHECKS")
    changes = plan.get("resource_changes", [])
    require(len(changes) == 9 and {r.get("address") for r in changes} == MANAGED | ADDRESSES | {TOPIC}, "GRAPH")
    imports = []
    for resource in changes:
        change = resource.get("change", {})
        require(resource.get("mode") == "managed" and change.get("actions") == ["no-op"], "ACTION")
        require(not has_unknown(change.get("after_unknown", {})), "UNKNOWN")
        require(canonical(change.get("before")) == canonical(change.get("after")), "VALUE")
        if change.get("importing"):
            require(resource["address"] == TOPIC and change["importing"].get("id") == TOPIC_ID, "IMPORT")
            imports.append(resource["address"])
        if resource["address"] == TOPIC:
            value = change.get("after", {})
            require(resource.get("type") == "google_pubsub_topic", "TYPE")
            require(value.get("id") == TOPIC_ID and value.get("project") == "test-youtube-study-space", "ID")
            require(value.get("name") == "initiateFirestoreCollectionsExport" and value.get("labels") == {}, "NAME")
    # A repeated normal plan after adoption is valid. Post-plan never imports.
    require(imports in ([], [TOPIC]) if phase == "before" else imports == [], "IMPORT")
    if diagnostic is not None:
        diagnostic.mark("BASELINE_OUTPUTS")
    require(set(plan.get("output_changes", {})) == {"environment", "project_id"}, "OUTPUTS")
    for output in plan["output_changes"].values():
        require(output.get("actions") == ["no-op"] and not has_unknown(output.get("after_unknown", {})), "ACTION")
        require(canonical(output.get("before")) == canonical(output.get("after")), "VALUE")
