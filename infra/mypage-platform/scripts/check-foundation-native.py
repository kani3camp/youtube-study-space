#!/usr/bin/env python3
"""Terraformのモック計画(terraform test)の結果を、MyPage基盤の許可範囲ルールで検査する。

目的: MyPage基盤のTerraformが、許可した種類のリソース変更以外(削除・公開・外部への権限付与など)を計画に含めていないか、実際のクラウドに触れずに確かめる。
使われる場面: infra/mypage-platform/scripts/check-local.sh から dev と prod に対して呼ばれ、CI GCP Terraform の terraform-tests(validate.sh 経由)で実行される。terraform コマンドが必要。
失敗したとき: check-local.sh が失敗し、CI GCP Terraform が失敗してPRをマージできない。
なくした場合: モック計画が foundation_contract.py と runtime_contract.py のルールに合うかの確認が check-local.sh から消える。同じルールの単体テストは infra/mypage-platform/tests/ にあるが、こちらは実際のTerraform出力で確かめる点が違う。
"""
import json
import sys
from pathlib import Path
import subprocess

from foundation_contract import ContractError, need
import runtime_contract

envdir = sys.argv[1] if len(sys.argv) > 1 else "dev"
need(envdir in ("dev", "prod"), "foundation-native-environment")
root = Path(__file__).resolve().parents[1] / "environments" / envdir
result = subprocess.run(["terraform", "test", "-filter=tests/foundation-contract.tftest.hcl", "-verbose", "-json"],
                        cwd=root, capture_output=True, text=True, timeout=180)
need(result.returncode == 0)
plans = [event["test_plan"] for line in result.stdout.splitlines()
         if (event := json.loads(line)).get("type") == "test_plan"]
need(len(plans) == 3)
# terraform testは描画用のPlan形式を出力し、terraform showのネイティブ形式とは異なる。
# プロバイダーが生成した変更とunknownは保持し、テストで既知の入力変数と成功完了情報だけを補う。
# 認証ありの処理では、引き続き実際のネイティブ形式を必須とする。
settings = json.loads((root / "tests/runtime-settings.fixture.json").read_text())
for rendered, (value, public) in zip(plans, ((None, False), (settings, False), (settings, True))):
    variables = {"runtime_settings": {"value": value}, "public_invoker": {"value": public}}
    envelope = {"format_version": rendered["plan_format_version"],
                "complete": True, "errored": False,
                "variables": variables,
                "resource_changes": rendered.get("resource_changes", []),
                "resource_drift": rendered.get("resource_drift", []),
                "deferred_changes": rendered.get("deferred_changes", [])}
    if value is not None:
        # Check the actual Terraform Plan, independently of Python's fixtures:
        # the invoker must be able to validate overrides and poll completion.
        resources = {item["address"]: item["change"]["after"] for item in envelope["resource_changes"]}
        reader_role = resources[runtime_contract.MAINTENANCE_READ_ROLE]
        need(set(reader_role["permissions"]) == {"run.jobs.get", "run.executions.get"})
        reader = resources[runtime_contract.MAINTENANCE_JOB_READER]
        need(reader["project"] == reader_role["project"] and
             reader["location"] == resources[runtime_contract.MAINTENANCE_JOB]["location"] and
             reader["name"] == "mypage-maintenance-operator" and
             reader["role"] == "projects/" + reader_role["project"] + "/roles/" + reader_role["role_id"] and
             reader["member"] == resources[runtime_contract.MAINTENANCE_JOB_INVOKER]["member"])
    if public:
        # public Invokerだけはowner管理であり、Stateなしのmock Planがそれをcreate
        # しようとする経路は引き続き拒否する。実workflowでは固定importを使う。
        try:
            runtime_contract.validate_plan(envelope)
        except ContractError as error:
            need(str(error) == "runtime-invoker-owner-managed")
        else:
            need(False)
    else:
        # 空のStateから、基盤だけ・実行系まで、のどちらも同じexact契約で一括作成できる。
        summary = runtime_contract.validate_plan(envelope)
        resources = len(envelope["resource_changes"])
        need(summary == {"resources": resources, "creates": resources})
        need({item["address"] for item in envelope["resource_changes"]} ==
             set(runtime_contract.resource_contract(runtime=value is not None, public=False)))
print("Native mocked base and runtime Plans accepted; public Invoker remains owner-managed; no cloud I/O.")
