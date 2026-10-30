#!/usr/bin/env python3
"""TerraformのネイティブモックPlan JSONで、配備の許可範囲を検証する。"""
import json
from pathlib import Path
import subprocess

from foundation_contract import ContractError, need, validate_plan
import runtime_contract

root = Path(__file__).resolve().parents[1] / "environments/dev"
result = subprocess.run(["terraform", "test", "-filter=tests/foundation-contract.tftest.hcl", "-verbose", "-json"],
                        cwd=root, capture_output=True, text=True, timeout=180)
need(result.returncode == 0)
plans = [event["test_plan"] for line in result.stdout.splitlines()
         if (event := json.loads(line)).get("type") == "test_plan"]
need(len(plans) == 6)
# terraform testは描画用のPlan形式を出力し、terraform showのネイティブ形式とは異なる。
# プロバイダーが生成した変更とunknownは保持し、テストで既知のstageと成功完了情報だけを補う。
# 認証ありの処理では、引き続き実際のネイティブ形式を必須とする。
for rendered, stage in zip(plans, ("foundation_metadata", "foundation", "registry", "maintenance_operator", "runtime_private", "runtime_public")):
    variables = {"stage": {"value": stage}}
    if stage in set(runtime_contract.MODES.values()):
        variables["runtime_settings"] = {"value": json.loads((root / "tests/runtime-settings.fixture.json").read_text())}
    envelope = {"format_version": rendered["plan_format_version"],
                         "complete": True, "errored": False,
                         "variables": variables,
                         "resource_changes": rendered.get("resource_changes", []),
                         "resource_drift": rendered.get("resource_drift", []),
                         "deferred_changes": rendered.get("deferred_changes", [])}
    validator = runtime_contract.validate_plan if stage in set(runtime_contract.MODES.values()) else validate_plan
    if stage == "runtime_private":
        # desired-state運用では、Stateに無いmaintenance境界もruntimeと同じexact Planで
        # 初回作成できる。個別bootstrap modeを手動選択する必要はない。
        summary = validator(envelope, stage)
        resources = len(envelope["resource_changes"])
        need(summary == {"resources": resources, "creates": resources})
    elif stage == "runtime_public":
        # public Invokerだけはowner管理であり、Stateなしのmock Planがそれをcreate
        # しようとする経路は引き続き拒否する。実workflowでは固定importを使う。
        try:
            validator(envelope, stage)
        except ContractError as error:
            need(str(error) == "runtime-invoker-owner-managed")
        else:
            need(False)
    else:
        summary = validator(envelope, stage)
        resources = len(envelope["resource_changes"])
        need(summary == {"resources": resources, "creates": resources})
print("Native mocked desired-state maintenance reconciliation accepted; public Invoker remains owner-managed; no cloud I/O.")
