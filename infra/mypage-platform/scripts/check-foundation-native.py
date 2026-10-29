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
    if stage in {"runtime_private", "runtime_public"}:
        # この認証不要のネイティブテストは毎回Stateなしで開始するため、
        # プロバイダーが生成するランタイムPlanはメンテナンス資源を再作成しようとする。
        # 運用担当者・初期構築段階の省略を拒否することを確認する。既存資源の完全なno-op Planは別に検証する。
        try:
            validator(envelope, stage)
        except ContractError as error:
            need(str(error) == "runtime-maintenance-bootstrap-complete")
        else:
            need(False)
    else:
        summary = validator(envelope, stage)
        resources = len(envelope["resource_changes"])
        need(summary == {"resources": resources, "creates": resources})
print("Native mocked maintenance operator Plan accepted; direct runtime creation without completed bootstrap rejected; no cloud I/O.")
