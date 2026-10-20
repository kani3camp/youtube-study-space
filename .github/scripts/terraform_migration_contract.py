"""Pure, private full-root import contract for the fixed GCP environments.

This module never calls Terraform or a cloud. The controller authenticates the
source, static private inputs, State object versions and native lock separately.
Manifest descriptors are a fixed allowlist, not inventory regenerated per plan.
No returned result includes an address, identity, resource value or private hash.
"""
from __future__ import annotations

from copy import deepcopy
import json
from pathlib import Path
import re
import uuid

GOOGLE = "registry.terraform.io/hashicorp/google"
TERRAFORM_VERSION = (Path(__file__).resolve().parents[2] / "infra/gcp/.terraform-version").read_text().strip()
ENVIRONMENTS = {
    "dev": ("test-youtube-study-space", "development"),
    "prod": ("youtube-study-space", "production"),
}
RESOURCE_TYPES = frozenset({
    "google_storage_bucket", "google_bigquery_dataset", "google_bigquery_table",
    "google_firestore_backup_schedule", "google_monitoring_notification_channel",
    "google_monitoring_alert_policy", "google_pubsub_topic", "google_cloud_scheduler_job",
    "google_cloudfunctions_function", "google_iam_workload_identity_pool",
    "google_iam_workload_identity_pool_provider", "google_service_account_iam_member",
    "google_project_service",
})
OBSERVATIONAL = frozenset({"etag", "last_modified_time"})
API_DISABLE_FLAGS = ("disable_on_destroy", "disable_dependent_services")
# Core can retain literal output references to these zero-instance legacy
# resources. This is a fixed source contract, never a generic address exception.
INACTIVE_REFERENCES = {
    "module.existing_youtube_quota_alerts.google_monitoring_alert_policy.quota": (
        "existing_youtube_quota_alerts", "../../modules/youtube-quota-alerts",
        "google_monitoring_alert_policy", "quota", "for_each_expression", "policy_names",
        ("google_monitoring_alert_policy.quota",)),
    "module.youtube_quota_alerts.google_monitoring_alert_policy.quota": (
        "youtube_quota_alerts", "../../modules/youtube-quota-alerts",
        "google_monitoring_alert_policy", "quota", "for_each_expression", "policy_names",
        ("google_monitoring_alert_policy.quota",)),
    "module.notification_channels.google_monitoring_notification_channel.primary_email[0]": (
        "notification_channels", "../../modules/monitoring-notification-channels",
        "google_monitoring_notification_channel", "primary_email", "count_expression", "primary_email_name",
        ("google_monitoring_notification_channel.primary_email[0].name",
         "google_monitoring_notification_channel.primary_email[0]",
         "google_monitoring_notification_channel.primary_email")),
}
ADDRESS = re.compile(
    r'(?P<modules>(?:module\.[A-Za-z_][A-Za-z0-9_-]*(?:\[(?:[0-9]+|"(?:[^"\\]|\\.)*")\])?\.)*)'
    r'(?P<type>google_[a-z0-9_]+)\.(?P<name>[A-Za-z_][A-Za-z0-9_-]*)'
    r'(?P<index>\[(?:[0-9]+|"(?:[^"\\]|\\.)*")\])?'
)
PLAN_KEYS = frozenset({
    "format_version", "terraform_version", "planned_values", "resource_changes",
    "output_changes", "prior_state", "configuration", "relevant_attributes", "checks",
    "timestamp", "applyable", "complete", "errored", "variables", "resource_drift",
    "deferred_changes", "action_invocations", "deferred_action_invocations",
})
RESOURCE_KEYS = frozenset({
    "address", "module_address", "mode", "type", "name", "index", "provider_name",
})
CHANGE_KEYS = frozenset({
    "actions", "before", "after", "before_sensitive", "after_sensitive", "after_unknown",
    "importing", "replace_paths", "before_identity", "after_identity",
})
INSTANCE_KEYS = frozenset({
    "schema_version", "attributes", "sensitive_attributes", "private", "dependencies",
    "index_key", "identity_schema_version", "identity", "create_before_destroy",
})
INDEX = re.compile(r'\[(?:[0-9]+|"(?:[^"\\]|\\.)*")\]')
CHECK_REF = re.compile(
    r'(?P<modules>(?:module\.[A-Za-z_][A-Za-z0-9_-]*(?:\[(?:[0-9]+|"(?:[^"\\]|\\.)*")\])?\.)*)'
    r'(?P<kind>var|output|check)\.(?P<name>[A-Za-z_][A-Za-z0-9_-]*)'
)


class ContractError(ValueError):
    """Fixed failure text; private input data must never enter an exception."""

    def __init__(self, reason="SHAPE"):
        self.reason = reason if reason in {
            "SHAPE", "MANIFEST", "SCHEMA", "GRAPH", "ACTION", "VALUE", "MASK",
            "DRIFT", "OUTPUT", "CHECK", "STATE", "STATE_DELTA", "REPLAN",
        } else "SHAPE"
        super().__init__("GCP migration contract STOP: " + self.reason)


def need(condition, reason="SHAPE"):
    if not condition:
        raise ContractError(reason)


def canonical(value):
    try:
        return json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False)
    except (ValueError, TypeError, RecursionError):
        raise ContractError() from None


def _bounded(value):
    budget = [500000]

    def walk(item, depth):
        budget[0] -= 1
        need(budget[0] >= 0 and depth <= 64)
        if type(item) is dict:
            need(all(type(key) is str for key in item))
            for child in item.values():
                walk(child, depth + 1)
        elif type(item) is list:
            for child in item:
                walk(child, depth + 1)
        else:
            need(item is None or type(item) in (str, bool, int, float))
    walk(value, 0)
    need(len(canonical(value).encode()) <= 32 * 1024 * 1024)


def loads(raw):
    """Strict JSON decoder for controller-owned private files."""
    def unique(pairs):
        result = {}
        for key, value in pairs:
            need(key not in result)
            result[key] = value
        return result

    try:
        need(type(raw) in (str, bytes) and len(raw) <= 32 * 1024 * 1024)
        value = json.loads(raw, object_pairs_hook=unique, parse_constant=lambda _: need(False))
        _bounded(value)
        return value
    except (json.JSONDecodeError, UnicodeDecodeError, RecursionError):
        raise ContractError() from None


def _same(first, second, reason="VALUE"):
    need(canonical(first) == canonical(second), reason)


def _mask(value, *, unknown=False):
    if type(value) is dict:
        need(all(type(key) is str for key in value), "MASK")
        for child in value.values():
            _mask(child, unknown=unknown)
    elif type(value) is list:
        for child in value:
            _mask(child, unknown=unknown)
    else:
        need(type(value) is bool and (not unknown or value is False), "MASK")


def validate_manifest(manifest, environment):
    _bounded(manifest)
    need(type(environment) is str and environment in ENVIRONMENTS and type(manifest) is dict, "MANIFEST")
    need({"schema_version", "environment", "project_id", "resources"} <= set(manifest)
         <= {"schema_version", "environment", "project_id", "project_number", "resources"}, "MANIFEST")
    need(manifest["schema_version"] == "gcp-migration-manifest-v1"
         and manifest["environment"] == environment
         and manifest["project_id"] == ENVIRONMENTS[environment][0], "MANIFEST")
    if "project_number" in manifest:
        need(type(manifest["project_number"]) is str
             and re.fullmatch(r"[0-9]{6,20}", manifest["project_number"]), "MANIFEST")
    need(type(manifest["resources"]) is list and len(manifest["resources"]) <= 1000, "MANIFEST")
    result = {}
    remote_identities = set()
    for descriptor in manifest["resources"]:
        need(type(descriptor) is dict and {"address", "type", "import_id"} <= set(descriptor)
             <= {"address", "type", "import_id", "values", "schema_version"}, "MANIFEST")
        address = descriptor["address"]
        match = ADDRESS.fullmatch(address) if type(address) is str else None
        need(match is not None and match["type"] == descriptor["type"]
             and type(descriptor["type"]) is str and descriptor["type"] in RESOURCE_TYPES and address not in result, "MANIFEST")
        need(type(descriptor["import_id"]) is str and 0 < len(descriptor["import_id"]) <= 4096
             and not any(character in descriptor["import_id"] for character in "\n\r\x00"), "MANIFEST")
        remote_identity = (descriptor["type"], descriptor["import_id"])
        need(remote_identity not in remote_identities, "MANIFEST")
        remote_identities.add(remote_identity)
        if "schema_version" in descriptor:
            need(type(descriptor["schema_version"]) is int and descriptor["schema_version"] >= 0, "MANIFEST")
        if "values" in descriptor:
            need(type(descriptor["values"]) is dict, "MANIFEST")
        result[address] = deepcopy(descriptor)
    return result


def validate_owned_api_inputs(tfvars, manifest, environment):
    """Bind selected existing APIs to non-API consumers in the fixed root graph.

    This proves a concrete address join, never workload usage from API enablement
    alone. Unselected classifications may describe future, not-yet-owned consumers.
    """
    _bounded(tfvars)
    expected = validate_manifest(manifest, environment)
    need(type(tfvars) is dict and tfvars.get("project_id") == ENVIRONMENTS[environment][0], "MANIFEST")
    selected = tfvars.get("owned_api_keys", [])
    service_name = re.compile(r"[a-z][a-z0-9-]*\.googleapis\.com")
    need(type(selected) is list and all(type(service) is str and service_name.fullmatch(service)
                                      for service in selected), "MANIFEST")
    need(len(selected) == len(set(selected)), "MANIFEST")
    manifest_services = set()
    consumers = set()
    for address, descriptor in expected.items():
        if descriptor["type"] != "google_project_service":
            consumers.add(address)
            continue
        project, separator, service = descriptor["import_id"].partition("/")
        need(separator == "/" and project == manifest["project_id"]
             and service_name.fullmatch(service), "MANIFEST")
        manifest_services.add(service)
    need(set(selected) == manifest_services, "MANIFEST")
    classification = tfvars.get("api_classification", {})
    need(type(classification) is dict, "MANIFEST")
    edges = 0
    for service in selected:
        item = classification.get(service)
        need(type(item) is dict and item.get("classification") == "Own"
             and item.get("state") == "ENABLED"
             and type(item.get("reason")) is str and bool(item["reason"].strip()), "MANIFEST")
        dependencies = item.get("dependency_addresses")
        need(type(dependencies) is list and bool(dependencies)
             and all(type(address) is str and address in consumers for address in dependencies), "MANIFEST")
        need(len(dependencies) == len(set(dependencies)), "MANIFEST")
        edges += len(dependencies)
    return {"owned_apis": len(selected), "dependency_edges": edges}


def _schema(provider_schema, resource_type):
    try:
        need(type(provider_schema) is dict and provider_schema.get("format_version") == "1.0", "SCHEMA")
        schema = provider_schema["provider_schemas"][GOOGLE]["resource_schemas"][resource_type]
        need(type(schema) is dict and type(schema.get("version")) is int and schema["version"] >= 0, "SCHEMA")
        block = schema["block"]
        need(type(block) is dict and type(block.get("attributes", {})) is dict
             and type(block.get("block_types", {})) is dict, "SCHEMA")
        return schema
    except (KeyError, TypeError):
        raise ContractError("SCHEMA") from None


def _identity_schema(provider_schema, resource_type):
    """Identity has its own provider-level schema, not a resource block."""
    _schema(provider_schema, resource_type)
    schemas = provider_schema["provider_schemas"][GOOGLE].get("resource_identity_schemas", {})
    need(type(schemas) is dict, "SCHEMA")
    schema = schemas.get(resource_type)
    if schema is None:
        return None
    need(type(schema) is dict and set(schema) == {"version", "attributes"}
         and type(schema["version"]) is int and schema["version"] >= 0
         and type(schema["attributes"]) is dict and bool(schema["attributes"]), "SCHEMA")
    for name, attribute in schema["attributes"].items():
        need(type(name) is str and type(attribute) is dict
             and {"type"} <= set(attribute) <= {"type", "description", "required_for_import", "optional_for_import"}
             # All reviewed Google 8.5.0 allowlist identity attributes are strings.
             # New identity types require a deterministic adapter, never coercion.
             and attribute["type"] == "string"
             and type(attribute.get("required_for_import", False)) is bool
             and type(attribute.get("optional_for_import", False)) is bool
             and attribute.get("required_for_import", False) != attribute.get("optional_for_import", False), "SCHEMA")
    return schema


def _resource_identity(identity, version, values, descriptor, manifest, provider_schema):
    """Validate identity type/requiredness and its binding to resource attributes."""
    need(type(version) is int and version >= 0, "SCHEMA")
    if identity is None:
        need(version == 0, "SCHEMA")
        return None
    schema = _identity_schema(provider_schema, descriptor["type"])
    need(schema is not None and type(identity) is dict and bool(identity)
         and version == schema["version"], "SCHEMA")
    attributes = schema["attributes"]
    required = {name for name, attribute in attributes.items() if attribute.get("required_for_import", False)}
    need(required <= set(identity) <= set(attributes), "VALUE")
    for name, value in identity.items():
        if value is None:
            need(name not in required, "VALUE")
            continue
        need(type(value) is str and not any(ord(character) < 32 or ord(character) == 127 for character in value)
             and (name not in required or bool(value)), "VALUE")
        if name == "condition_title" and descriptor["type"] == "google_service_account_iam_member":
            conditions = values.get("condition", [])
            expected = conditions[0]["title"] if conditions else ""
        else:
            need(name in values, "VALUE")
            expected = values[name]
        if name == "project":
            projects = {manifest["project_id"], manifest.get("project_number", manifest["project_id"])}
            need(value in projects and expected in projects, "VALUE")
        else:
            _same(value, expected)
    return identity


def _change_identity(change, planned, descriptor, manifest, provider_schema, *, existing=None):
    """A no-op cannot change, drop or invent an existing resource identity."""
    before, after = change.get("before_identity"), change.get("after_identity")
    schema = _identity_schema(provider_schema, descriptor["type"])
    for side, identity in (("before", before), ("after", after)):
        _resource_identity(identity, schema["version"] if identity is not None and schema is not None else 0,
                           change[side], descriptor, manifest, provider_schema)
    # The pinned Core/provider also shows an import's resolved remote identity
    # on both sides of its no-op. An identity create/delete is not this contract.
    _same(before, after)
    _same(after, planned.get("identity"))
    if existing is not None and existing.get("identity") is not None:
        _same(existing["identity"], before, "STATE")


def observation_keys(provider_schema, resource_type):
    """Read-only metadata shared by migration and normal Plan validation."""
    if resource_type != "google_bigquery_table":
        return frozenset()
    attrs = _schema(provider_schema, resource_type)["block"].get("attributes", {})
    for name in OBSERVATIONAL:
        item = attrs.get(name)
        need(type(item) is dict and item.get("computed") is True
             and item.get("optional", False) is False and item.get("required", False) is False, "SCHEMA")
    return OBSERVATIONAL


def _stable_values(value, resource_type, provider_schema):
    need(type(value) is dict, "VALUE")
    omitted = observation_keys(provider_schema, resource_type)
    return {key: item for key, item in value.items() if key not in omitted}


def _values(value, descriptor, manifest, provider_schema):
    schema = _schema(provider_schema, descriptor["type"])
    need(type(value) is dict and set(value) <= (
        set(schema["block"].get("attributes", {})) | set(schema["block"].get("block_types", {}))), "SCHEMA")
    project = manifest["project_id"]
    projects = {project, manifest.get("project_number", project)}
    identifier = value.get("id")
    need(type(identifier) is str and bool(identifier), "VALUE")
    if descriptor["type"] == "google_service_account_iam_member":
        sa = value.get("service_account_id")
        need(type(sa) is str and sa in {
            f"projects/{item}/serviceAccounts/{project}@appspot.gserviceaccount.com" for item in projects
        } and value.get("role") == "roles/iam.workloadIdentityUser", "VALUE")
        member, conditions = value.get("member"), value.get("condition", [])
        need(type(member) is str and bool(member) and not re.search(r"\s", member)
             and type(conditions) is list and len(conditions) <= 1, "VALUE")
        need("project_number" in manifest and re.fullmatch(
            r"principalSet://iam\.googleapis\.com/projects/" + re.escape(manifest["project_number"])
            + r"/locations/global/workloadIdentityPools/aws-runtime/attribute\.aws_role/[A-Za-z0-9_+=,.@-]+",
            member), "VALUE")
        if conditions:
            condition = conditions[0]
            need(type(condition) is dict and set(condition) == {"title", "description", "expression"}
                 and all(type(item) is str for item in condition.values()) and bool(condition["title"])
                 and not re.search(r"\s", condition["title"]), "VALUE")
        expected_import = " ".join([sa, value["role"], member] + ([conditions[0]["title"]] if conditions else []))
        expected_id = "/".join([sa, value["role"], member] + (
            [conditions[0][key] for key in ("title", "description", "expression")] if conditions else []))
        need(descriptor["import_id"] == expected_import and identifier == expected_id, "VALUE")
    else:
        need(value.get("project") in projects and identifier == descriptor["import_id"], "VALUE")
        if identifier.startswith("projects/"):
            need(any(identifier.startswith(f"projects/{item}/") for item in projects), "VALUE")
    if descriptor["type"] == "google_project_service":
        need(identifier == project + "/" + value.get("service", ""), "VALUE")
        # Google 8.5 imports these optional bools as present JSON nulls even
        # when configured false. Accept no coercion or missing field; ownership
        # additionally requires the exact desired-false configuration below.
        for field in API_DISABLE_FLAGS:
            attribute = schema["block"].get("attributes", {}).get(field)
            need(type(attribute) is dict and attribute.get("type") == "bool"
                 and attribute.get("optional") is True
                 and attribute.get("computed", False) is False
                 and attribute.get("required", False) is False, "SCHEMA")
            need(field in value and (value[field] is None or value[field] is False), "VALUE")
    for name in observation_keys(provider_schema, descriptor["type"]):
        if name in value and value[name] is not None:
            need((type(value[name]) is str and bool(value[name])) if name == "etag"
                 else (type(value[name]) is int and value[name] >= 0), "VALUE")
    if "values" in descriptor:
        _same(_stable_values(value, descriptor["type"], provider_schema),
              _stable_values(descriptor["values"], descriptor["type"], provider_schema))


def _identity(resource, expected, *, shown=False):
    need(type(resource) is dict and resource.get("address") in expected, "GRAPH")
    address = resource["address"]
    descriptor = expected[address]
    match = ADDRESS.fullmatch(address)
    need(resource.get("mode") == "managed" and resource.get("type") == descriptor["type"]
         and resource.get("name") == match["name"] and resource.get("provider_name") == GOOGLE, "GRAPH")
    if not shown or "module_address" in resource:
        need(resource.get("module_address") == (match["modules"].removesuffix(".") or None), "GRAPH")
    if match["index"]:
        _same(resource.get("index"), json.loads(match["index"][1:-1]), "GRAPH")
    else:
        need("index" not in resource, "GRAPH")
    return address, descriptor


def _change(change, *, action):
    need(type(change) is dict and {"actions", "before", "after"} <= set(change) <= CHANGE_KEYS, "ACTION")
    need(change["actions"] == [action] and change.get("replace_paths", []) == [], "ACTION")
    _mask(change.get("after_unknown", {}), unknown=True)
    for name in ("before_sensitive", "after_sensitive"):
        _mask(change.get(name, {}))
    _same(change.get("before_sensitive", {}), change.get("after_sensitive", {}), "MASK")


def _checks(checks, *, native=False):
    need(checks is None or type(checks) is list, "CHECK")
    result = []
    for check in checks or []:
        allowed = {"object_kind", "config_addr", "status", "objects"} if native else {"address", "status", "instances"}
        need(type(check) is dict and {"status"} <= set(check) <= allowed and check["status"] == "pass", "CHECK")
        if native:
            need(set(check) == allowed and check["object_kind"] in {"resource", "var", "output", "check"}
                 and type(check["config_addr"]) is str and bool(check["config_addr"]), "CHECK")
        field = "objects" if native else "instances"
        children = check.get(field, [])
        children = [] if children is None else children
        need(type(children) is list, "CHECK")
        for child in children:
            allowed_child = {"object_addr", "status", "failure_messages"} if native else {"address", "status"}
            need(type(child) is dict and {"status"} <= set(child) <= allowed_child
                 and child["status"] == "pass" and not child.get("failure_messages", []), "CHECK")
            if native:
                need(type(child.get("object_addr")) is str and bool(child["object_addr"]), "CHECK")
        normalized = dict(check, **{field: sorted(children, key=canonical)})
        need(len({canonical(child) for child in children}) == len(children), "CHECK")
        result.append(normalized)
    need(len({canonical(check) for check in result}) == len(result), "CHECK")
    return sorted(result, key=canonical)


def _check_identity(kind, address):
    need(type(address) is str and kind in {"resource", "var", "output", "check"}, "CHECK")
    match = (ADDRESS if kind == "resource" else CHECK_REF).fullmatch(address)
    need(match is not None and (kind == "resource" or match["kind"] == kind), "CHECK")
    if kind == "resource":
        need(match["type"] in RESOURCE_TYPES, "CHECK")
    return match


def _check_map(checks, *, native=False):
    """Join the documented shown check references to native State check keys.

    Static shown addresses carry kind/name/type; instance references carry only
    to_display/module/instance_key. Neither arbitrary strings nor PASS prose can
    establish the exact check set written by the current saved plan.
    """
    result = {}
    for check in _checks(checks, native=native):
        if native:
            kind, address = check["object_kind"], check["config_addr"]
        else:
            reference = check.get("address")
            need(type(reference) is dict and type(reference.get("kind")) is str, "CHECK")
            kind = "output" if reference["kind"] == "output_value" else reference["kind"]
            address = reference.get("to_display")
        match = _check_identity(kind, address)
        need(INDEX.sub("", address) == address, "CHECK")
        if not native:
            exact = {"to_display": address, "kind": "output_value" if kind == "output" else kind,
                     "name": match["name"]}
            module = match["modules"].removesuffix(".")
            if module:
                exact["module"] = module
            if kind == "resource":
                exact.update(mode="managed", type=match["type"])
            _same(reference, exact, "CHECK")
        key = (kind, address)
        need(key not in result, "CHECK")
        objects = set()
        for child in check["objects" if native else "instances"]:
            if native:
                object_address = child["object_addr"]
            else:
                reference = child.get("address")
                need(type(reference) is dict, "CHECK")
                object_address = reference.get("to_display")
            instance = _check_identity(kind, object_address)
            need(INDEX.sub("", object_address) == address and object_address not in objects, "CHECK")
            if not native:
                exact = {"to_display": object_address}
                module = instance["modules"].removesuffix(".")
                if module:
                    exact["module"] = module
                if kind == "resource" and instance["index"]:
                    exact["instance_key"] = json.loads(instance["index"][1:-1])
                _same(reference, exact, "CHECK")
            objects.add(object_address)
        result[key] = objects
    return result


def _outputs(outputs, environment, *, native=False, changes=False):
    project, label = ENVIRONMENTS[environment]
    need(type(outputs) is dict and set(outputs) == {"environment", "project_id"}, "OUTPUT")
    for name, expected in (("environment", label), ("project_id", project)):
        value = outputs[name]
        if changes:
            need(type(value) is dict and {"actions", "before", "after"} <= set(value)
                 <= {"actions", "before", "after", "after_unknown", "before_sensitive", "after_sensitive"}, "OUTPUT")
            need(value["actions"] == ["no-op"] and value["before"] == value["after"] == expected, "OUTPUT")
            _mask(value.get("after_unknown", False), unknown=True)
            need(value.get("before_sensitive", False) is False and value.get("after_sensitive", False) is False, "OUTPUT")
        else:
            need(type(value) is dict and {"value", "type"} <= set(value) <= {"value", "type", "sensitive"}
                 and value["value"] == expected and value["type"] == "string"
                 and value.get("sensitive", False) is False, "OUTPUT")


def _shown_values(shown, expected, manifest, provider_schema, environment):
    need(type(shown) is dict and set(shown) <= {"outputs", "root_module"}
         and "root_module" in shown, "GRAPH")
    _outputs(shown.get("outputs"), environment)
    found, seen_modules = {}, set()

    def walk(module, module_address=None):
        need(type(module) is dict and set(module) <= {"address", "resources", "child_modules"}, "GRAPH")
        need(module.get("address") == module_address, "GRAPH")
        if module_address is not None:
            need(module_address not in seen_modules and any(address.startswith(module_address + ".") for address in expected), "GRAPH")
            seen_modules.add(module_address)
        resources, children = module.get("resources", []), module.get("child_modules", [])
        need(type(resources) is list and type(children) is list, "GRAPH")
        for resource in resources:
            need(type(resource) is dict and RESOURCE_KEYS | {
                "schema_version", "values", "sensitive_values", "identity", "identity_schema_version", "depends_on"
            } >= set(resource)
                 and {"address", "mode", "type", "name", "provider_name", "schema_version", "values"} <= set(resource), "GRAPH")
            address, descriptor = _identity(resource, expected, shown=True)
            need((ADDRESS.fullmatch(address)["modules"].removesuffix(".") or None) == module_address
                 and address not in found, "GRAPH")
            schema = _schema(provider_schema, descriptor["type"])
            need(type(resource["schema_version"]) is int and resource["schema_version"] == schema["version"], "SCHEMA")
            if "schema_version" in descriptor:
                need(descriptor["schema_version"] == resource["schema_version"], "SCHEMA")
            _values(resource["values"], descriptor, manifest, provider_schema)
            _mask(resource.get("sensitive_values", {}))
            need(type(resource.get("identity_schema_version", 0)) is int and resource.get("identity_schema_version", 0) >= 0
                 and (resource.get("identity") is None or type(resource["identity"]) is dict)
                 and type(resource.get("depends_on", [])) is list
                 and all(type(item) is str for item in resource.get("depends_on", [])), "GRAPH")
            _resource_identity(resource.get("identity"), resource.get("identity_schema_version", 0),
                               resource["values"], descriptor, manifest, provider_schema)
            found[address] = resource
        for child in children:
            need(type(child) is dict and type(child.get("address")) is str, "GRAPH")
            child_address = child["address"]
            if module_address is None:
                need(re.fullmatch(r'module\.[A-Za-z_][A-Za-z0-9_-]*(?:\[(?:[0-9]+|"(?:[^"\\]|\\.)*")\])?', child_address), "GRAPH")
            else:
                need(child_address.startswith(module_address + ".module.")
                     and ".module." not in child_address[len(module_address + ".module."):], "GRAPH")
            walk(child, child_address)
    walk(shown["root_module"])
    return found


def _api_configuration(configuration, expected):
    """Join each API instance to its reviewed static desired-false resource.

    The reviewed roots call the local API module directly. Module/resource
    instance indexes resolve to that same static configuration, but unknown
    nesting or source paths require a separately reviewed source contract.
    """
    selected = {address for address, item in expected.items() if item["type"] == "google_project_service"}
    if not selected:
        return frozenset()
    need(type(configuration) is dict and type(configuration.get("root_module")) is dict, "GRAPH")
    providers = configuration.get("provider_config")
    need(type(providers) is dict and type(providers.get("google")) is dict
         and providers["google"].get("full_name") == GOOGLE, "GRAPH")
    calls = configuration["root_module"].get("module_calls")
    need(type(calls) is dict, "GRAPH")
    for address in selected:
        match = ADDRESS.fullmatch(address)
        module = INDEX.sub("", match["modules"].removesuffix("."))
        need(re.fullmatch(r"module\.[A-Za-z_][A-Za-z0-9_-]*", module) is not None, "GRAPH")
        call = calls.get(module.removeprefix("module."))
        need(type(call) is dict and call.get("source") == "../../modules/owned-project-apis"
             and type(call.get("module")) is dict, "GRAPH")
        resources = call["module"].get("resources")
        need(type(resources) is list and all(type(item) is dict for item in resources), "GRAPH")
        static = match["type"] + "." + match["name"]
        found = [item for item in resources if item.get("address") == static]
        need(len(found) == 1, "GRAPH")
        resource = found[0]
        need(resource.get("mode") == "managed" and resource.get("type") == match["type"]
             and resource.get("name") == match["name"] and resource.get("provider_config_key") == "google", "GRAPH")
        expressions = resource.get("expressions")
        need(type(expressions) is dict, "VALUE")
        for field in API_DISABLE_FLAGS:
            expression = expressions.get(field)
            need(type(expression) is dict and set(expression) == {"constant_value"}
                 and expression["constant_value"] is False, "VALUE")
    return frozenset(selected)


def _native_state(state, expected, manifest, provider_schema, environment, *, api_configuration=frozenset()):
    if state is None:
        return {}
    _bounded(state)
    need(type(state) is dict and {"version", "terraform_version", "serial", "lineage", "outputs", "resources"} <= set(state)
         <= {"version", "terraform_version", "serial", "lineage", "outputs", "resources", "check_results"}, "STATE")
    need(type(state["version"]) is int and state["version"] == 4 and state["terraform_version"] == TERRAFORM_VERSION
         and type(state["serial"]) is int and state["serial"] >= 0, "STATE")
    try:
        need(type(state["lineage"]) is str and str(uuid.UUID(state["lineage"])) == state["lineage"], "STATE")
    except (ValueError, AttributeError):
        raise ContractError("STATE") from None
    _outputs(state["outputs"], environment, native=True)
    _check_map(state.get("check_results"), native=True)
    need(type(state["resources"]) is list, "STATE")
    found, containers = {}, set()
    for resource in state["resources"]:
        need(type(resource) is dict and {"mode", "type", "name", "provider", "instances"} <= set(resource)
             <= {"module", "mode", "type", "name", "provider", "instances"}, "STATE")
        need(resource["mode"] == "managed" and resource["type"] in RESOURCE_TYPES
             and resource["provider"] == f'provider["{GOOGLE}"]'
             and type(resource["instances"]) is list and bool(resource["instances"]), "STATE")
        prefix = (resource["module"] + "." if "module" in resource else "") + resource["type"] + "." + resource["name"]
        need(prefix not in containers, "STATE")
        containers.add(prefix)
        for instance in resource["instances"]:
            need(type(instance) is dict and {"schema_version", "attributes"} <= set(instance) <= INSTANCE_KEYS, "STATE")
            index = instance.get("index_key")
            need("index_key" not in instance or (type(index) is int and index >= 0) or type(index) is str, "STATE")
            address = prefix + ("[" + canonical(index) + "]" if "index_key" in instance else "")
            need(address in expected and address not in found, "STATE")
            descriptor = expected[address]
            need(resource["type"] == descriptor["type"] and ADDRESS.fullmatch(address), "STATE")
            schema = _schema(provider_schema, descriptor["type"])
            need(type(instance["schema_version"]) is int and instance["schema_version"] == schema["version"], "SCHEMA")
            if "schema_version" in descriptor:
                need(descriptor["schema_version"] == instance["schema_version"], "SCHEMA")
            if descriptor["type"] == "google_project_service":
                need(address in api_configuration, "STATE")
            _values(instance["attributes"], descriptor, manifest, provider_schema)
            need(type(instance.get("sensitive_attributes", [])) is list and type(instance.get("private", "")) is str
                 and type(instance.get("dependencies", [])) is list
                 and all(type(dependency) is str for dependency in instance.get("dependencies", []))
                 and instance.get("create_before_destroy", False) is False
                 and type(instance.get("identity_schema_version", 0)) is int and instance.get("identity_schema_version", 0) >= 0
                 and (instance.get("identity") is None or type(instance["identity"]) is dict), "STATE")
            _resource_identity(instance.get("identity"), instance.get("identity_schema_version", 0),
                               instance["attributes"], descriptor, manifest, provider_schema)
            normalized = deepcopy(instance)
            for field in ("sensitive_attributes", "dependencies"):
                items = normalized.get(field, [])
                need(len({canonical(item) for item in items}) == len(items), "STATE")
                normalized[field] = sorted(items, key=canonical)
            found[address] = ({key: value for key, value in resource.items() if key != "instances"}, normalized)
    return found


def _inactive_configuration_reference(address, configuration, provider_schema):
    """Prove one fixed literal hint against its exact source/provider schema.

    This static proof supplies no claim about State or resource inactivity. The
    caller must prove that independently: normal imports use real native State;
    the output-only initializer proves the complete empty graph before adapting.
    """
    need(address in INACTIVE_REFERENCES, "SHAPE")
    module, source, kind, name, repeat, output, literals = INACTIVE_REFERENCES[address]
    local = kind + "." + name
    need(type(configuration) is dict and type(configuration.get("root_module")) is dict, "SHAPE")
    providers = configuration.get("provider_config")
    need(type(providers) is dict and type(providers.get("google")) is dict
         and providers["google"].get("full_name") == GOOGLE, "SHAPE")
    calls = configuration["root_module"].get("module_calls")
    need(type(calls) is dict and type(calls.get(module)) is dict, "SHAPE")
    call = calls[module]
    need(call.get("source") == source and "count_expression" not in call
         and "for_each_expression" not in call and type(call.get("module")) is dict, "SHAPE")
    contents = call["module"]
    resources = contents.get("resources")
    need(type(resources) is list and all(type(item) is dict for item in resources), "SHAPE")
    matches = [item for item in resources if item.get("address") == local]
    need(len(matches) == 1, "SHAPE")
    resource = matches[0]
    need(resource.get("mode") == "managed" and resource.get("type") == kind
         and resource.get("name") == name and resource.get("provider_config_key") == "google"
         and type(resource.get("schema_version")) is int
         and resource["schema_version"] == _schema(provider_schema, kind)["version"] == 0
         and type(resource.get(repeat)) is dict
         and set(resource[repeat]) == {"references"}
         and type(resource[repeat]["references"]) is list
         and resource[repeat]["references"]
         and all(type(item) is str for item in resource[repeat]["references"]), "SHAPE")
    need(("count_expression" if repeat == "for_each_expression" else "for_each_expression") not in resource, "SHAPE")
    outputs = contents.get("outputs")
    need(type(outputs) is dict and type(outputs.get(output)) is dict, "SHAPE")
    expression = outputs[output].get("expression")
    need(type(expression) is dict and set(expression) == {"references"}, "SHAPE")
    references = expression["references"]
    need(type(references) is list and all(type(item) is str for item in references)
         and len(references) == len(literals) and set(references) == set(literals)
         and address.removeprefix("module." + module + ".") in references, "SHAPE")



def _inactive_configured_reference(address, plan, state, expected, provider_schema):
    """Authorize only catalogued literal references with no active family."""
    need(address in INACTIVE_REFERENCES, "SHAPE")
    module, source, kind, name, repeat, output, literals = INACTIVE_REFERENCES[address]
    local = kind + "." + name
    family = "module." + module + "." + local

    def resource_family(value):
        need(type(value) is str and ADDRESS.fullmatch(value) is not None, "SHAPE")
        return INDEX.sub("", value)

    active = {resource_family(item) for item in expected}
    changes = plan.get("resource_changes")
    need(type(changes) is list, "SHAPE")
    for item in changes:
        need(type(item) is dict, "SHAPE")
        active.add(resource_family(item.get("address")))

    def shown(module):
        need(type(module) is dict and type(module.get("resources", [])) is list
             and type(module.get("child_modules", [])) is list, "SHAPE")
        for item in module.get("resources", []):
            need(type(item) is dict, "SHAPE")
            active.add(resource_family(item.get("address")))
        for child in module.get("child_modules", []):
            shown(child)

    planned = plan.get("planned_values")
    need(type(planned) is dict, "SHAPE")
    shown(planned.get("root_module"))
    if "prior_state" in plan:
        prior = plan["prior_state"]
        need(type(prior) is dict and type(prior.get("values")) is dict, "SHAPE")
        shown(prior["values"].get("root_module"))
    need(type(state) is dict and type(state.get("resources")) is list, "SHAPE")
    _bounded(state)
    for item in state["resources"]:
        need(type(item) is dict and type(item.get("instances")) is list, "SHAPE")
        if item["instances"]:
            need(type(item.get("module", "")) is str and type(item.get("type")) is str
                 and type(item.get("name")) is str, "SHAPE")
            active.add(resource_family((item.get("module", "") + "." if item.get("module") else "")
                                       + item["type"] + "." + item["name"]))
    need(family not in active, "SHAPE")

    _inactive_configuration_reference(address, plan.get("configuration"), provider_schema)


def validate_plan(plan, *, manifest, environment, provider_schema, state=None, phase="before"):
    """Accept only the exact full graph, existing no-ops and missing-State imports.

    State ``None`` means a positively established absent object, never a failed
    read; the controller is responsible for proving absence. State initialization
    with output creates is deliberately not accepted by this import contract.
    """
    _bounded(plan)
    expected = validate_manifest(manifest, environment)
    need(phase in {"before", "post"}, "ACTION")
    need(type(plan) is dict and set(plan) <= PLAN_KEYS and plan.get("format_version") == "1.2"
         and plan.get("terraform_version") == TERRAFORM_VERSION and plan.get("complete") is True
         and plan.get("errored", False) is False, "SHAPE")
    need("applyable" not in plan or type(plan["applyable"]) is bool, "SHAPE")
    for field in ("deferred_changes", "action_invocations", "deferred_action_invocations"):
        need(type(plan.get(field, [])) is list and plan.get(field, []) == [], "ACTION")
    if "relevant_attributes" in plan:
        relevant = plan["relevant_attributes"]
        need(type(relevant) is list, "SHAPE")
        relevant_resources = set(expected)
        for address in expected:
            match = ADDRESS.fullmatch(address)
            # Core may reference the whole repeated resource as NoKey. Keep
            # module instances exact and admit only this known leaf alias.
            relevant_resources.add(match["modules"] + match["type"] + "." + match["name"])
        for entry in relevant:
            need(type(entry) is dict and {"resource", "attribute"} <= set(entry)
                 and type(entry["resource"]) is str
                 and type(entry["attribute"]) is list
                 and all(type(step) is str or type(step) is int and step >= 0
                         for step in entry["attribute"]), "SHAPE")
            if entry["resource"] not in relevant_resources:
                _inactive_configured_reference(entry["resource"], plan, state, expected, provider_schema)
    _check_map(plan.get("checks", []))
    for field in ("configuration", "variables"):
        need(type(plan.get(field)) is dict, "SHAPE")
    api_configuration = _api_configuration(plan["configuration"], expected)
    native = _native_state(state, expected, manifest, provider_schema, environment,
                           api_configuration=api_configuration)
    _outputs(plan.get("output_changes"), environment, changes=True)
    planned = _shown_values(plan.get("planned_values"), expected, manifest, provider_schema, environment)
    need(set(planned) == set(expected), "GRAPH")
    changes = plan.get("resource_changes")
    need(type(changes) is list and len(changes) == len(expected), "GRAPH")
    found, imports = {}, set()
    for resource in changes:
        need(type(resource) is dict and set(resource) <= RESOURCE_KEYS | {"change", "action_reason"}, "GRAPH")
        address, descriptor = _identity(resource, expected)
        need(address not in found, "GRAPH")
        change = resource.get("change")
        _change(change, action="no-op")
        _same(change["before"], change["after"])
        _values(change["after"], descriptor, manifest, provider_schema)
        _same(change["after"], planned[address]["values"])
        _same(change.get("after_sensitive", {}), planned[address].get("sensitive_values", {}), "MASK")
        _change_identity(change, planned[address], descriptor, manifest, provider_schema,
                         existing=native[address][1] if address in native else None)
        if "importing" in change:
            need(change["importing"] == {"id": descriptor["import_id"]}, "ACTION")
            imports.add(address)
        found[address] = change
    need(set(found) == set(expected) and imports == set(expected) - set(native), "GRAPH")
    need(phase != "post" or not imports, "ACTION")
    drift = plan.get("resource_drift", [])
    need(type(drift) is list, "DRIFT")
    observed = {}
    for resource in drift:
        need(type(resource) is dict and set(resource) <= RESOURCE_KEYS | {"change", "action_reason"}, "DRIFT")
        address, descriptor = _identity(resource, expected)
        need(address in native and address not in observed and descriptor["type"] == "google_bigquery_table", "DRIFT")
        change = resource.get("change")
        _change(change, action="update")
        need("importing" not in change, "DRIFT")
        for value in (change["before"], change["after"]):
            _values(value, descriptor, manifest, provider_schema)
        need(set(change["before"]) == set(change["after"]), "DRIFT")
        differing = {key for key in change["before"] if canonical(change["before"][key]) != canonical(change["after"][key])}
        need(bool(differing) and differing <= observation_keys(provider_schema, descriptor["type"]), "DRIFT")
        _same(change["before"], native[address][1]["attributes"], "DRIFT")
        _same(change["after"], found[address]["before"], "DRIFT")
        _same(change.get("before_sensitive", {}), found[address].get("before_sensitive", {}), "MASK")
        if "before_identity" in change or "after_identity" in change:
            _change_identity(change, planned[address], descriptor, manifest, provider_schema,
                             existing=native[address][1])
        observed[address] = differing
    for address, (_, instance) in native.items():
        _same(instance["attributes"],
              next(resource["change"]["before"] for resource in drift if resource["address"] == address)
              if address in observed else found[address]["before"], "DRIFT")
    if "prior_state" in plan:
        prior = plan["prior_state"]
        need(type(prior) is dict and {"format_version", "values"} <= set(prior)
             <= {"format_version", "terraform_version", "values"}
             and prior["format_version"] == "1.0"
             and prior.get("terraform_version", TERRAFORM_VERSION) == TERRAFORM_VERSION, "STATE")
        prior_values = _shown_values(prior["values"], expected, manifest, provider_schema, environment)
        # Configuration-driven imports may already be in the shown refreshed
        # prior State. Existing owned resources can never disappear from it.
        need(set(native) <= set(prior_values) <= set(expected), "GRAPH")
        for address, resource in prior_values.items():
            _same(resource["values"], found[address]["before"], "STATE")
            _same(resource.get("identity"), found[address].get("before_identity"), "STATE")
    return {"schema_version": "gcp-migration-result-v1", "environment": environment,
            "managed": len(expected), "managed_before": len(native), "imports": len(imports),
            "no_op": len(expected), "raw_observation_drift": len(drift),
            "observation_leaf_changes": sum(len(keys) for keys in observed.values()),
            "mutations": 0, "historical_delta_attested": False}


def _comparison_projection(plan, expected, provider_schema):
    projection = deepcopy(plan)
    projection.pop("timestamp", None)
    # Observation records describe a read, never an executable resource action.
    # They were validated against the native State before reaching this function.
    projection.pop("resource_drift", None)
    projection["checks"] = _checks(projection.get("checks", []))
    if "relevant_attributes" in projection:
        # Core's heuristic reference collection is unordered. Sort complete
        # entries only: retain path order, duplicates and all extra fields.
        # https://github.com/hashicorp/terraform/blob/v1.16.4/internal/command/jsonplan/plan.go#L760-L776
        projection["relevant_attributes"].sort(key=canonical)
    for field in ("resource_changes",):
        for resource in projection[field]:
            descriptor = expected[resource["address"]]
            for side in ("before", "after"):
                resource["change"][side] = _stable_values(resource["change"][side], descriptor["type"], provider_schema)
        projection[field].sort(key=lambda item: item["address"])

    def shown(values):
        def walk(module):
            for resource in module.get("resources", []):
                descriptor = expected[resource["address"]]
                resource["values"] = _stable_values(resource["values"], descriptor["type"], provider_schema)
            if "resources" in module:
                module["resources"].sort(key=lambda item: item["address"])
            for child in module.get("child_modules", []):
                walk(child)
            if "child_modules" in module:
                module["child_modules"].sort(key=lambda item: item["address"])
        walk(values["root_module"])
    shown(projection["planned_values"])
    if "prior_state" in projection:
        shown(projection["prior_state"]["values"])
    return projection


def compare_replans(first, second, *, manifest, environment, provider_schema, state=None, second_state=None):
    """Compare private execution projections, retaining config/variables/checks.

    Only timestamps, validated BQ observation leaves and semantic ordering of
    resource/module/check collections and whole relevant-attribute entries are
    normalized. No binary plan digest belongs in
    Terraform's JSON; the controller seals each binary immediately before apply.
    """
    before_result = validate_plan(first, manifest=manifest, environment=environment,
                                  provider_schema=provider_schema, state=state)
    after_result = validate_plan(second, manifest=manifest, environment=environment,
                                 provider_schema=provider_schema, state=state if second_state is None else second_state)
    expected = validate_manifest(manifest, environment)
    _same(_comparison_projection(first, expected, provider_schema),
          _comparison_projection(second, expected, provider_schema), "REPLAN")
    return {"matched": True, "imports": before_result["imports"],
            "raw_observation_drift": [before_result["raw_observation_drift"], after_result["raw_observation_drift"]]}


def plan_commitment_payload(plan, manifest, environment, provider_schema, *, state=None, phase="before"):
    """Private canonicalizable payload for the controller's purpose-bound HMAC.

    Pass the same native State used by ``validate_plan``. Never publish this
    payload or an unkeyed digest; only the controller's HMAC crosses job outputs.
    """
    validate_plan(plan, manifest=manifest, environment=environment,
                  provider_schema=provider_schema, state=state, phase=phase)
    expected = validate_manifest(manifest, environment)
    return _comparison_projection(plan, expected, provider_schema)


def validate_state_delta(before, after, *, manifest, environment, provider_schema, expected_imports, plan=None):
    """Require exact ownership addition and unchanged existing State semantics."""
    expected = validate_manifest(manifest, environment)
    api_configuration = frozenset()
    if any(item["type"] == "google_project_service" for item in expected.values()):
        # Native nulls are not standalone State authorization. Every API delta
        # must be bound to the saved plan's exact constant-false configuration.
        need(type(plan) is dict, "STATE_DELTA")
        _bounded(plan)
        api_configuration = _api_configuration(plan.get("configuration"), expected)
    old = _native_state(before, expected, manifest, provider_schema, environment,
                        api_configuration=api_configuration)
    new = _native_state(after, expected, manifest, provider_schema, environment,
                        api_configuration=api_configuration)
    need(type(expected_imports) in (set, frozenset, list, tuple)
         and all(type(address) is str for address in expected_imports), "STATE_DELTA")
    imports = set(expected_imports)
    need(len(imports) == len(expected_imports) and set(old) <= set(expected)
         and set(new) == set(expected) and set(new) - set(old) == imports, "STATE_DELTA")
    need(before is not None and after is not None, "STATE_DELTA")
    need(before["lineage"] == after["lineage"] and before["version"] == after["version"]
         and before["terraform_version"] == after["terraform_version"], "STATE_DELTA")
    need(after["serial"] > before["serial"] if imports else after["serial"] >= before["serial"], "STATE_DELTA")
    _same(before["outputs"], after["outputs"], "STATE_DELTA")
    for address, (header, instance) in old.items():
        new_header, new_instance = new[address]
        _same(header, new_header, "STATE_DELTA")
        original, current = deepcopy(instance), deepcopy(new_instance)
        descriptor = expected[address]
        original["attributes"] = _stable_values(original["attributes"], descriptor["type"], provider_schema)
        current["attributes"] = _stable_values(current["attributes"], descriptor["type"], provider_schema)
        _same(original, current, "STATE_DELTA")
        if canonical(instance["attributes"]) != canonical(new_instance["attributes"]):
            need(after["serial"] > before["serial"], "STATE_DELTA")
    if plan is not None:
        validated = validate_plan(plan, manifest=manifest, environment=environment,
                                  provider_schema=provider_schema, state=before)
        need(validated["imports"] == len(imports), "STATE_DELTA")
        approved_values = {resource["address"]: resource["change"]["after"]
                           for resource in plan["resource_changes"]}
        approved_identity = {resource["address"]: resource["change"].get("after_identity")
                             for resource in plan["resource_changes"]}
        # A manifest may intentionally omit provider-computed values. The saved
        # plan still fixes every resulting instance: importing the right ID must
        # never excuse unrelated attributes appearing in its new State instance.
        for address, (_, instance) in new.items():
            resource_type = expected[address]["type"]
            _same(_stable_values(instance["attributes"], resource_type, provider_schema),
                  _stable_values(approved_values[address], resource_type, provider_schema), "STATE_DELTA")
            _same(instance.get("identity"), approved_identity[address], "STATE_DELTA")
        planned_checks = _check_map(plan.get("checks", []))
        previous_checks = _check_map(before.get("check_results"), native=True)
        current_checks = _check_map(after.get("check_results"), native=True)
        need(planned_checks == current_checks and set(previous_checks) <= set(current_checks)
             and all(objects <= current_checks[key] for key, objects in previous_checks.items()), "STATE_DELTA")
        return {"managed_before": len(old), "managed_after": len(new), "imports": len(imports),
                "existing_values_unchanged": True, "saved_plan_checks_match": True}
    old_checks = _checks(before.get("check_results"), native=True)
    new_checks = _checks(after.get("check_results"), native=True)
    # Imports may add checks for their own configuration addresses. Preserve every
    # old check, and require every added resource object to be a new instance.
    old_by_address = {check.get("config_addr"): check for check in old_checks}
    new_by_address = {check.get("config_addr"): check for check in new_checks}
    need(len(old_by_address) == len(old_checks) and len(new_by_address) == len(new_checks)
         and set(old_by_address) <= set(new_by_address), "STATE_DELTA")
    for address, check in old_by_address.items():
        if canonical(check) != canonical(new_by_address[address]):
            previous = {canonical(item) for item in check.get("objects", [])}
            current = {canonical(item) for item in new_by_address[address].get("objects", [])}
            _same({key: value for key, value in check.items() if key != "objects"},
                  {key: value for key, value in new_by_address[address].items() if key != "objects"}, "STATE_DELTA")
            need(previous <= current and all(json.loads(item).get("object_addr") in imports for item in current - previous), "STATE_DELTA")
    for address in set(new_by_address) - set(old_by_address):
        check = new_by_address[address]
        need(check.get("object_kind") == "resource" and bool(check.get("objects"))
             and all(item.get("object_addr") in imports for item in check["objects"]), "STATE_DELTA")
    return {"managed_before": len(old), "managed_after": len(new), "imports": len(imports), "existing_values_unchanged": True}
