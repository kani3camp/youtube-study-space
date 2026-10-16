"""Exact production output-only initialization adapter for the migration controller.

Selection consumes only the controller's successful S3 snapshot, never an input
claim of absence. The ordinary import contract is unchanged. No cloud calls,
State writes, credentials, or execution authority live in this module.
"""
from __future__ import annotations

from copy import deepcopy
from pathlib import Path
import sys

import terraform_migration_contract as contract

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "infra/gcp/scripts"))
from production_execution_contract import plan_shape

ABSENT = {"head": None, "raw": None, "state": None}


def selected(environment, manifest, snapshot):
    """A positive absent-object snapshot is required; a failed read never arrives here."""
    contract.validate_manifest(manifest, environment)
    return environment == "prod" and manifest["resources"] == [] and snapshot == ABSENT


def normalize_empty_plan(plan, manifest):
    """Core omits resource_changes for a zero-resource graph; preserve all other fields."""
    result = deepcopy(plan)
    if manifest["resources"] == [] and type(result) is dict and "resource_changes" not in result:
        result["resource_changes"] = []
    return result


def _projection(plan, *, manifest, environment, provider_schema, state):
    contract.need(environment == "prod" and state is None, "STATE")
    contract.need(contract.validate_manifest(manifest, environment) == {}, "MANIFEST")
    contract._bounded(plan)
    # Reuse the separately reviewed production initializer's exact raw plan
    # contract before adapting only the two proven output creates for the
    # ordinary full-graph validator. The saved binary itself is never changed.
    try:
        plan_shape(plan, {}, imports=set(), initialize=True)
    except (ValueError, TypeError, KeyError):
        raise contract.ContractError("OUTPUT") from None
    projected = normalize_empty_plan(plan, manifest)
    for output in projected["output_changes"].values():
        output["actions"] = ["no-op"]
        output["before"] = output["after"]
    contract.validate_plan(projected, manifest=manifest, environment=environment,
                           provider_schema=provider_schema, state=state)
    return projected


def validate_plan(plan, *, manifest, environment, provider_schema, state=None):
    projected = _projection(plan, manifest=manifest, environment=environment,
                            provider_schema=provider_schema, state=state)
    summary = contract.validate_plan(projected, manifest=manifest, environment=environment,
                                     provider_schema=provider_schema, state=state)
    return dict(summary, production_state_initialization=True, output_creates=2)


def plan_commitment_payload(plan, *, manifest, environment, provider_schema, state=None):
    projected = _projection(plan, manifest=manifest, environment=environment,
                            provider_schema=provider_schema, state=state)
    return {
        "operation": "production-state-initialize-v1",
        "initial_output_changes": deepcopy(plan["output_changes"]),
        "validated_plan": contract.plan_commitment_payload(projected, manifest=manifest,
            environment=environment, provider_schema=provider_schema, state=state),
    }


def validate_state_delta(before, after, *, manifest, environment, provider_schema, plan):
    validate_plan(plan, manifest=manifest, environment=environment,
                  provider_schema=provider_schema, state=before)
    contract.need(after is not None and type(after) is dict, "STATE_DELTA")
    contract.need(contract._native_state(after, {}, manifest, provider_schema, environment) == {}, "STATE_DELTA")
    contract.need(after["resources"] == [], "STATE_DELTA")
    contract.need(contract._check_map(plan.get("checks", [])) ==
                  contract._check_map(after.get("check_results"), native=True), "STATE_DELTA")
    return {"managed_before": 0, "managed_after": 0, "imports": 0,
            "production_state_initialization": True, "output_creates": 2,
            "saved_plan_checks_match": True}
