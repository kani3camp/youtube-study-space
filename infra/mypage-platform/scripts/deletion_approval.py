"""MyPageのTerraformで削除を伴う変更を適用してよいか、GitHubの公開APIだけで確認する。

目的: リソースの削除を、実際に承認者が付いた保護済みEnvironmentで、管理者の迂回なしに、同じ実行(run)で承認されたときだけ進める。
使われる場面: MyPage Foundation Authenticated ワークフロー(.github/workflows/mypage-foundation-authenticated.yml)の削除承認の確認(check)と承認の照合(verify)の各ステップ。
失敗したとき: 削除を含む配備(apply)が進まず中断される。
なくした場合: 削除を伴うApplyの承認確認ができなくなり、上記ワークフローのステップが動かなくなる。
"""
from __future__ import annotations

import json
import os
import re
import sys
import urllib.request

import foundation_contract as contract

API = "https://api.github.com/repos/" + contract.REPOSITORY


def fetch(path):
    request = urllib.request.Request(API + path, headers={
        "Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28",
        "Cache-Control": "no-cache",
    })
    with urllib.request.urlopen(request, timeout=15) as response:
        return json.load(response)


def protected_environment(name):
    environment = fetch("/environments/" + name)
    contract.need(environment.get("name") == name and environment.get("can_admins_bypass") is False,
                  "deletion-environment-admin-bypass")
    reviewers = [rule["reviewers"] for rule in environment.get("protection_rules", [])
                 if rule.get("type") == "required_reviewers"]
    contract.need(len(reviewers) == 1 and reviewers[0], "deletion-environment-required-reviewers")
    environment_id = environment.get("id")
    contract.need(type(environment_id) is int and environment_id > 0, "deletion-environment-id")
    return environment_id


def verify(env, name, environment_id):
    run_id = env.get("GITHUB_RUN_ID", "")
    contract.need(env.get("GITHUB_RUN_ATTEMPT") == "1" and re.fullmatch(r"[1-9][0-9]*", run_id),
                  "deletion-approval-run")
    contract.need(env.get("MYPAGE_APPROVAL_ENVIRONMENT_ID") == str(environment_id),
                  "deletion-environment-changed")
    reviews = fetch("/actions/runs/" + run_id + "/approvals")
    states = [review.get("state") for review in reviews
              if any(item.get("name") == name and item.get("id") == environment_id
                     for item in review.get("environments", []))]
    contract.need(states and all(state == "approved" for state in states), "deletion-run-not-approved")


def main():
    try:
        name = contract.GITHUB_ENVIRONMENT + "-delete-approval"
        environment_id = protected_environment(name)
        if sys.argv[1:] == ["check"]:
            with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
                output.write(f"environment_id={environment_id}\n")
        else:
            contract.need(sys.argv[1:] == ["verify"], "deletion-approval-command")
            verify(os.environ, name, environment_id)
    except contract.ContractError as error:
        print(f"MyPage deletion stopped: {error}.", file=sys.stderr)
        return 1
    except Exception:
        print("MyPage deletion stopped: GitHub approval settings or review history unavailable.", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
