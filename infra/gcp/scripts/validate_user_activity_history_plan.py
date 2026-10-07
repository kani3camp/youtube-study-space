#!/usr/bin/env python3
"""Offline review of a private canonical history import/post-import plan.

No cloud client, execution, activation or approval is provided. The caller must
separately establish fresh metadata and plan provenance and live prerequisites.
"""
from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from prepare_user_activity_history_adoption import normalize_field, prepare, private_json, unique_object

# Reuse the already adopted eleven-resource contract, including its function
# identity and generated-resource exclusions, without changing its CI wiring.
sys.path.insert(0, str(Path(__file__).resolve().parents[3] / ".github/scripts"))
from terraform_email_adoption_gate import canonical, has_unknown
from terraform_export_function_gate import validate as validate_baseline
from terraform_plan_summary import build_summary

TABLE = "module.user_activity_history[0].google_bigquery_table.retained"
TABLE_ID = "projects/test-youtube-study-space/datasets/firestore_export/tables/user-activity-history"


def require(condition: bool) -> None:
    if not condition:
        raise ValueError("History plan contract STOP; private diagnostic suppressed.")


def validate(plan: object, *, metadata: object, phase: str, execution_email: str,
             allow_adopted: bool = False) -> None:
    require(phase in {"before", "post"} and type(plan) is dict and type(allow_adopted) is bool)
    # Terraform 1.16.4 serializes provider actions outside resource_changes.
    # Import/no-op resources cannot authorize a side-effecting action or one
    # deferred until apply. Omitted or empty arrays are the only accepted forms.
    for key in ("action_invocations", "deferred_action_invocations"):
        require(type(plan.get(key, [])) is list and not plan.get(key, []))
    order = prepare(metadata)["user_activity_history_field_order"]
    expected_schema = [normalize_field(field) for field in metadata["schema"]["fields"]]
    changes = plan.get("resource_changes")
    require(type(changes) is list and len(changes) == 12 and all(type(resource) is dict for resource in changes))
    # A moved address can be reported with no-op actions; this adoption must
    # preserve every existing state address as well as its remote values.
    require(all("previous_address" not in resource for resource in changes))
    selected = [resource for resource in changes if resource.get("address") == TABLE]
    require(len(selected) == 1)
    validate_baseline(dict(plan, resource_changes=[resource for resource in changes if resource.get("address") != TABLE]),
                      phase="post", execution_email=execution_email)
    summary = build_summary(plan, environment="dev", git_sha="offline", policy="import-only")
    allowed_imports = {0, 1} if phase == "before" and allow_adopted else {1 if phase == "before" else 0}
    require(summary["policy_passed"] and summary["counts"]["import"] in allowed_imports)
    resource = selected[0]
    require(resource.get("mode") == "managed" and resource.get("type") == "google_bigquery_table")
    require(resource.get("provider_name") == "registry.terraform.io/hashicorp/google")
    change = resource.get("change", {})
    require(change.get("actions") == ["no-op"] and not has_unknown(change.get("after_unknown", {})))
    require(canonical(change.get("before")) == canonical(change.get("after")))
    require(change.get("importing") == {"id": TABLE_ID} if summary["counts"]["import"] else "importing" not in change)
    value = change.get("after")
    require(type(value) is dict)
    for key, expected in {"id": TABLE_ID, "project": "test-youtube-study-space", "dataset_id": "firestore_export",
                          "table_id": "user-activity-history", "location": "asia-southeast2", "deletion_protection": True}.items():
        require(value.get(key) == expected)
    require(type(value.get("schema")) is str)
    fields = json.loads(value["schema"], object_pairs_hook=unique_object)
    require(type(fields) is list)
    actual = [normalize_field(field) for field in fields]
    require([field["name"] for field in actual] == order and actual == expected_schema)
    for key in ("description", "friendly_name", "labels", "effective_labels", "terraform_labels", "resource_tags",
                "time_partitioning", "range_partitioning", "clustering", "expiration_time", "encryption_configuration",
                "external_data_configuration", "view", "materialized_view", "snapshot_definition", "clone_definition",
                "table_constraints", "biglake_configuration", "max_staleness", "require_partition_filter"):
        require(not value.get(key))


class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message: str) -> None:
        raise ValueError("invalid offline arguments")


def main() -> int:
    parser = PrivateArgumentParser(description=__doc__)
    parser.add_argument("--phase", required=True, choices=("before", "post"))
    parser.add_argument("--metadata", required=True)
    parser.add_argument("--plan", required=True)
    try:
        args = parser.parse_args()
        validate(private_json(args.plan), metadata=private_json(args.metadata), phase=args.phase,
                 execution_email=os.environ.get("TF_VAR_export_function_execution_service_account_email", ""))
    except Exception:
        print("STOP: offline history plan rejected; private diagnostic suppressed.", file=sys.stderr)
        return 3
    print("Offline history plan contract passed. Execution remains unapproved.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
