#!/usr/bin/env python3
"""Exercise the shared contract against Terraform's actual verbose mock JSON."""

import json
import pathlib
import sys

from plan_contract import ContractError, check_change, require, target_scope


def main():
    try:
        require(len(sys.argv) == 3, "mock_arguments_invalid")
        records = [json.loads(line) for line in pathlib.Path(sys.argv[1]).read_text().splitlines()]
        target = json.loads(pathlib.Path(sys.argv[2]).read_text())
        summary = [r["test_summary"] for r in records if r.get("type") == "test_summary"]
        require(len(summary) == 1 and summary[0].get("passed") == 4 and
                summary[0].get("failed") == 0 and summary[0].get("errored") == 0, "mock_tests_failed")
        expected_runs = {"default_database_scope", "explicit_named_database"}
        checked = set()
        for record in records:
            name = record.get("@testrun")
            if record.get("type") != "test_plan" or name not in expected_runs:
                continue
            require(name not in checked, "mock_plan_duplicate")
            scope = target_scope(target)
            if name == "explicit_named_database":
                scope["databaseID"] = "demo-mypage-dev-db" if scope["environment"] == "development" else "demo-mypage-prod-db"
            plan = record["test_plan"]
            require(plan.get("plan_format_version") == "1.2", "mock_format_unsupported")
            require(len(plan.get("resource_changes", [])) == 1, "mock_resource_count_invalid")
            require(check_change(plan["resource_changes"][0], scope) == "create", "mock_action_invalid")
            require(plan.get("output_changes", {}).get("scope", {}).get("after") == scope,
                    "mock_environment_binding_mismatch")
            checked.add(name)
        require(checked == expected_runs, "mock_plan_missing")
        print("Mock scope contract passed: 4 tests, 2 actual mock plans.")
        return 0
    except (ContractError, OSError, ValueError, KeyError, TypeError):
        print("Mock scope contract failed.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
