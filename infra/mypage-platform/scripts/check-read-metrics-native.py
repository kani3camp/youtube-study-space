#!/usr/bin/env python3
"""DEVの読み取り計測設定(MYPAGE_READ_METRICS)だけを変えるTerraformのモック計画を、実際の受け入れ検査に通して確かめる。

目的: 計測設定の変更計画が、他の設定変更に紛れ込まず、受け入れ検査が想定どおり通すことを、実際のTerraform出力で確認する。
使われる場面: infra/mypage-platform/scripts/check-local.sh から呼ばれ、CI GCP Terraform の terraform-tests(validate.sh 経由)で実行される。terraform コマンドが必要。
失敗したとき: check-local.sh が失敗し、CI GCP Terraform が失敗してPRをマージできない。
なくした場合: 計測設定の変更計画をTerraformの実出力で確かめる手段がなくなる。tests/test_read_metrics*.py が作り物のデータで同じ受け入れ規則を試しているため、一部重なる。薄い呼び出し役で、check-local.sh からだけ使われる。
"""
import json
from pathlib import Path
import subprocess

from foundation_contract import need
import runtime_contract as r

root = Path(__file__).resolve().parents[1] / "environments/dev"
result = subprocess.run(["terraform", "test", "-filter=tests/read-metrics-transition.tftest.hcl", "-verbose", "-json"],
                        cwd=root, capture_output=True, text=True, timeout=180)
need(result.returncode == 0, "metrics-native-tests")
events = [json.loads(line) for line in result.stdout.splitlines()]
plans = [event["test_plan"] for event in events if event.get("type") == "test_plan"]
need(len(plans) == 2, "metrics-native-plan-count")
for rendered, enabled in zip(plans, (True, False)):
    settings = r.settings(json.loads((root / "tests/runtime-settings.fixture.json").read_text()) | {
        "read_metrics_enabled": enabled, "read_metrics_from_revision": "mypage-api-before",
    })
    # 既存のネイティブチェッカーと同様に、テスト描画用メタデータを変換する。
    # プロバイダーが生成したaction・before/after・unknownは書き換えない。
    envelope = {"format_version": rendered["plan_format_version"], "complete": True, "errored": False,
                "variables": {"stage": {"value": "runtime_private"}, "runtime_settings": {"value": settings}},
                "resource_changes": rendered.get("resource_changes", []),
                "resource_drift": rendered.get("resource_drift", []),
                "deferred_changes": rendered.get("deferred_changes", [])}
    need(r.validate_plan(envelope, "runtime_private") == {"resources": len(envelope["resource_changes"]), "creates": 0, "updates": 1},
         "metrics-native-admission")
print("Native mocked metrics enable/disable Plans each admitted exactly one update; no cloud I/O.")
