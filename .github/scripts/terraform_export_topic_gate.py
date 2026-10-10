"""Exact development topic wave; additive to the unchanged import-only guard."""
from terraform_email_adoption_gate import canonical, has_unknown
from terraform_quota_create_gate import ADDRESSES, MANAGED

TOPIC = "module.export_topic[0].google_pubsub_topic.export"
TOPIC_ID = "projects/test-youtube-study-space/topics/initiateFirestoreCollectionsExport"


def validate(plan, *, phase):
    def require(value):
        if not value:
            raise ValueError("Development topic wave STOP; private diagnostic suppressed.")

    require(phase in {"before", "post"})
    require(plan.get("complete") is True and plan.get("errored") is False)
    require(not plan.get("resource_drift") and not plan.get("deferred_changes"))
    require(all(c.get("status") == "pass" for c in plan.get("checks", [])))
    changes = plan.get("resource_changes", [])
    require(len(changes) == 9 and {r.get("address") for r in changes} == MANAGED | ADDRESSES | {TOPIC})
    imports = []
    for resource in changes:
        change = resource.get("change", {})
        require(resource.get("mode") == "managed" and change.get("actions") == ["no-op"])
        require(not has_unknown(change.get("after_unknown", {})))
        require(canonical(change.get("before")) == canonical(change.get("after")))
        if change.get("importing"):
            require(resource["address"] == TOPIC and change["importing"].get("id") == TOPIC_ID)
            imports.append(resource["address"])
        if resource["address"] == TOPIC:
            value = change.get("after", {})
            require(resource.get("type") == "google_pubsub_topic")
            require(value.get("id") == TOPIC_ID and value.get("project") == "test-youtube-study-space")
            require(value.get("name") == "initiateFirestoreCollectionsExport" and value.get("labels") == {})
    # A repeated normal plan after adoption is valid. Post-plan never imports.
    require(imports in ([], [TOPIC]) if phase == "before" else imports == [])
    require(set(plan.get("output_changes", {})) == {"environment", "project_id"})
    for output in plan["output_changes"].values():
        require(output.get("actions") == ["no-op"] and not has_unknown(output.get("after_unknown", {})))
        require(canonical(output.get("before")) == canonical(output.get("after")))
