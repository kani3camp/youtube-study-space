#!/usr/bin/env python3
"""DEV MyPageのpushを分類し、検証済みdigestをレビュー用PRに記録する。

この処理はCloud Run、Terraform、IAMを変更しない。公開digest以外の
クラウド認証情報を取得せず、release PRを自動マージしない。
"""
from __future__ import annotations

import base64
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import urllib.parse
import urllib.request

import foundation_contract as c
import runtime_contract as runtime

ROOT = Path(__file__).resolve().parents[3]
BRANCH = "integration/mypage-canon-20261006"
RELEASE_PATH = "infra/mypage-platform/environments/dev/runtime-release.json"
HOST = "mypage-api-eu5mhkcewq-et.a.run.app"


def check(value, label):
    c.need(value, "cd-" + label)


def push_context(env):
    exact = {
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
    check(all(env.get(k) == v for k, v in exact.items()), "push-context")
    check(re.fullmatch(r"[a-f0-9]{40}", env.get("GITHUB_SHA", "")), "push-sha")


def classify_paths(paths):
    check(all(isinstance(path, str) and path and "\n" not in path for path in paths),
          "changed-paths")
    build = any(path.startswith("system/") for path in paths)
    deploy = RELEASE_PATH in paths
    check(not (build and deploy), "mixed-source-and-release")
    return build, deploy


def classify(env):
    push_context(env)
    before = env.get("MYPAGE_CD_BEFORE", "")
    check(re.fullmatch(r"[a-f0-9]{40}", before) and before != "0" * 40,
          "before-sha")
    result = subprocess.run(
        ["git", "diff", "--no-renames", "--name-only", "-z", before, env["GITHUB_SHA"]],
        cwd=ROOT, capture_output=True, timeout=60,
    )
    check(result.returncode == 0, "diff")
    paths = [p.decode("utf-8") for p in result.stdout.split(b"\0") if p]
    build, deploy = classify_paths(paths)
    with open(env["GITHUB_OUTPUT"], "a", encoding="utf-8") as out:
        out.write(f"build={str(build).lower()}\ndeploy={str(deploy).lower()}\n")
    print(f"MyPage CD: build={str(build).lower()}, deploy={str(deploy).lower()}.")


def declaration(raw):
    check(len(raw) <= 4096, "release-size")
    value = json.loads(raw)
    check(type(value) is dict and set(value) == {
        "image_digest", "image_source_sha", "hosting_backend_host", "previous_image_digest"
    }, "release-fields")
    pattern = re.escape(runtime.REPOSITORY) + r"@sha256:[a-f0-9]{64}"
    check(all(type(value[field]) is str and re.fullmatch(pattern, value[field])
              for field in ("image_digest", "previous_image_digest")), "release-digests")
    check(type(value["image_source_sha"]) is str
          and re.fullmatch(r"[a-f0-9]{40}", value["image_source_sha"])
          and value["hosting_backend_host"] == HOST, "release-values")
    return value


def github(env, method, path, body=None):
    check(method in {"GET", "POST", "PUT"} and path.startswith("/"), "api-target")
    token = env["GITHUB_TOKEN"]
    check(bool(token), "token")
    data = None if body is None else json.dumps(body, separators=(",", ":")).encode()
    request = urllib.request.Request(
        "https://api.github.com/repos/" + c.REPOSITORY + path,
        data=data, method=method,
        headers={
            "Authorization": "Bearer " + token,
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
            "User-Agent": "mypage-backend-cd",
            **({"Content-Type": "application/json"} if data is not None else {}),
        },
    )
    with urllib.request.urlopen(request, timeout=30) as response:
        payload = response.read(65537)
    check(len(payload) <= 65536, "api-size")
    return json.loads(payload) if payload else {}


def release_pr(env, api=github):
    push_context(env)
    digest = env.get("MYPAGE_CD_IMAGE_DIGEST", "")
    check(type(digest) is str and re.fullmatch(
        re.escape(runtime.REPOSITORY) + r"@sha256:[a-f0-9]{64}", digest
    ), "new-digest")
    source = env["GITHUB_SHA"]
    path = ROOT / RELEASE_PATH
    check(path.is_file() and not path.is_symlink(), "release-file")
    raw = path.read_bytes()
    current = declaration(raw)
    if current["image_digest"] == digest:
        print("MyPage image digest unchanged; no release PR needed.")
        return None

    # Gitのbaseが動いたときは古い宣言を上書きせず、再レビューする。
    ref = api(env, "GET", "/git/ref/heads/" + BRANCH)
    check(ref.get("object", {}).get("sha") == source, "branch-moved")
    content = api(env, "GET", "/contents/" + RELEASE_PATH +
                  "?ref=" + urllib.parse.quote(BRANCH, safe=""))
    check(content.get("encoding") == "base64", "manifest-encoding")
    check(base64.b64decode(content["content"]) == raw, "manifest-moved")

    updated = {
        "image_digest": digest,
        "image_source_sha": source,
        "hosting_backend_host": HOST,
        "previous_image_digest": current["image_digest"],
    }
    branch = "automation/mypage-dev-image-" + source[:12]
    api(env, "POST", "/git/refs", {"ref": "refs/heads/" + branch, "sha": source})
    api(env, "PUT", "/contents/" + RELEASE_PATH, {
        "message": "chore(mypage): pin reviewed DEV backend image " + source[:12],
        "branch": branch,
        "sha": content["sha"],
        "content": base64.b64encode((json.dumps(updated, indent=2) + "\n").encode()).decode(),
    })
    result = api(env, "POST", "/pulls", {
        "title": "chore(mypage): release DEV backend " + source[:12],
        "base": BRANCH,
        "head": branch,
        "body": (
            "## DEV MyPage backend release declaration\n\n"
            "CI built and checked the image from `" + source + "`. "
            "Reviewing and merging this PR authorizes the separate DEV release workflow.\n\n"
            "- New image: `" + digest + "`\n"
            "- Expected live previous image: `" + current["image_digest"] + "`\n"
            "- Scope: DEV Cloud Run image only. No automatic rollback or production deployment.\n"
            "- After merge: CI checks the exact source/registry/previous revision, performs "
            "the bounded image-only update and Terraform State readback.\n\n"
            "If the previous image is not currently serving, deployment **stops**. "
            "Do not bypass the baseline guard.\n"
        ),
    })
    url = result.get("html_url", "")
    check(url.startswith("https://github.com/" + c.REPOSITORY + "/pull/"), "pr-result")
    # GITHUB_TOKEN-created PRs do not emit pull_request CI events. Explicitly
    # dispatch the normal credentialless validation on the exact PR head.
    # The reviewed branch defaults to operation=validate. Avoid passing inputs whose
    # schema might differ on the repository's default branch.
    api(env, "POST", "/actions/workflows/ci.yml/dispatches", {"ref": branch})
    print("MyPage DEV release PR created and CI validation dispatched: " + url)
    if env.get("GITHUB_STEP_SUMMARY"):
        with open(env["GITHUB_STEP_SUMMARY"], "a", encoding="utf-8") as out:
            out.write("### MyPage DEV release PR\n\n" + url + "\n")
    return url


def main():
    os.umask(0o077)
    try:
        check(len(sys.argv) == 2 and sys.argv[1] in {"classify", "release-pr"}, "command")
        if sys.argv[1] == "classify":
            classify(os.environ)
        else:
            release_pr(os.environ)
        return 0
    except Exception:
        # GitHubやGitの応答本文をログに出さない（トークン・private payload混入対策）。
        print("::error::MyPage DEV CD stopped; see the reviewed source contract.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
