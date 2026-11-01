#!/usr/bin/env python3
"""Credentialless order-history schema preservation and import-only contracts."""
from __future__ import annotations

import copy
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile

from terraform_plan_summary import build_summary

ROOT = Path(__file__).resolve().parents[2]
MODULE = ROOT / "infra/gcp/modules/retained-order-history"


def main() -> None:
    env = {key: value for key, value in os.environ.items()
           if not key.startswith(("AWS_", "GOOGLE_", "GCLOUD_", "GCP_", "CLOUDSDK_", "TF_"))}
    with tempfile.TemporaryDirectory(prefix="order-history-contract-") as directory:
        base = Path(directory)
        target, cache = base / "module", base / "cache"
        cache.mkdir()
        env.update(TF_IN_AUTOMATION="true", TF_INPUT="false", TF_WORKSPACE="default",
                   TF_PLUGIN_CACHE_DIR=str(cache))
        target.mkdir()
        for name in ("main.tf", "schema.json"):
            shutil.copyfile(MODULE / name, target / name)
        shutil.copytree(MODULE / "tests", target / "tests")
        shutil.copyfile(ROOT / "infra/gcp/environments/dev/.terraform.lock.hcl", target / ".terraform.lock.hcl")
        for args in (("init", "-backend=false", "-input=false", "-lockfile=readonly"),
                     ("test", "-json", "-verbose")):
            result = subprocess.run(["terraform", f"-chdir={target}", *args], env=env,
                                    capture_output=True, text=True)
            if result.returncode:
                raise RuntimeError("Order-history native mock contracts failed; raw output suppressed.")
        plans = {event["@testrun"]: event["test_plan"] for line in result.stdout.splitlines()
                 if (event := json.loads(line)).get("type") == "test_plan"}
        assert len(plans) == 5
        schema = json.loads((MODULE / "schema.json").read_text())
        for name, environment in (("preserve_existing_development_schema", "dev"),
                                  ("preserve_observed_production_schema_order", "prod")):
            plan = plans[name]
            expected = copy.deepcopy(schema)
            if environment == "prod":
                expected[0], expected[1] = expected[1], expected[0]
            change = plan["resource_changes"][0]["change"]
            assert json.loads(change["after"]["schema"]) == expected
            assert not change.get("after_sensitive", {}).get("schema", False)
            summary = build_summary(plan, environment=environment, git_sha="0" * 40, policy="import-only")
            assert summary["counts"]["create"] == 1 and summary["counts"]["import"] == 0
            assert not summary["policy_passed"]
    print("Order-history schema/order contracts PASS; global import-only guard rejects creates.")


if __name__ == "__main__":
    main()
