#!/usr/bin/env python3
"""Classify trusted DEV integration pushes; image deployment needs no manifest PR."""
from __future__ import annotations

import os
from pathlib import Path
import re
import subprocess
import sys

import foundation_contract as c

ROOT = Path(__file__).resolve().parents[3]
BRANCH = "integration/mypage-canon-20261006"


def push_context(env):
    expected = {
        "GITHUB_REPOSITORY": c.REPOSITORY,
        "GITHUB_REPOSITORY_ID": "340900071",
        "GITHUB_REPOSITORY_OWNER_ID": "54093651",
        "GITHUB_EVENT_NAME": "push",
        "GITHUB_ACTOR": "kani3camp",
        "GITHUB_ACTOR_ID": "54093651",
        "GITHUB_TRIGGERING_ACTOR": "kani3camp",
        "GITHUB_RUN_ATTEMPT": "1",
        "GITHUB_REF": "refs/heads/" + BRANCH,
        "GITHUB_WORKFLOW_REF": c.CALLER,
    }
    c.need(all(env.get(k) == v for k, v in expected.items()), "cd-push-context")
    c.need(re.fullmatch(r"[0-9a-f]{40}", env.get("GITHUB_SHA", "")), "cd-push-sha")


def classify_paths(paths):
    c.need(all(isinstance(path, str) and path and "\n" not in path for path in paths),
           "cd-changed-paths")
    # A Docker build depends on the entire system context, including dependencies.
    return any(path.startswith("system/") for path in paths)


def classify(env):
    push_context(env)
    before = env.get("MYPAGE_CD_BEFORE", "")
    c.need(re.fullmatch(r"[0-9a-f]{40}", before) and before != "0" * 40,
           "cd-before-sha")
    result = subprocess.run(
        ["git", "diff", "--no-renames", "--name-only", "-z", before, env["GITHUB_SHA"]],
        cwd=ROOT, capture_output=True, timeout=60,
    )
    c.need(result.returncode == 0, "cd-diff")
    paths = [p.decode("utf-8") for p in result.stdout.split(b"\0") if p]
    build = classify_paths(paths)
    with open(env["GITHUB_OUTPUT"], "a", encoding="utf-8") as out:
        out.write(f"build={str(build).lower()}\n")
    print(f"MyPage DEV continuous delivery: image_build={str(build).lower()}.")


def main():
    try:
        c.need(sys.argv[1:] == ["classify"], "cd-command")
        classify(os.environ)
        return 0
    except Exception:
        print("::error::MyPage DEV CD stopped; invalid event or source.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
