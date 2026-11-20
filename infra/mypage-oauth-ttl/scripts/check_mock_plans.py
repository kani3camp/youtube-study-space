#!/usr/bin/env python3
"""Terraformのモック実行(テスト)で出た変更計画を、plan_contract.py のルールで検査する小さな補助スクリプト。

目的: OAuthの一時データを自動削除する設定(TTL)の変更計画が、想定どおり4つのテストに合格し、許可した変更だけを含むことを確かめる。
使われる場面: infra/mypage-oauth-ttl/scripts/check-local.sh から dev と prod の環境ごとに呼ばれ、CI GCP Terraform の terraform-tests(validate.sh 経由)で実行される。
失敗したとき: check-local.sh が失敗し、CI GCP Terraform が失敗してPRをマージできない。
なくした場合: Terraformのモック計画が plan_contract.py のルールに合うかの確認が check-local.sh から消える。plan_contract.py 自体のテストは test_plan_contract.py が別に行う。薄い補助で、check-local.sh にしか使われない。
"""

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
