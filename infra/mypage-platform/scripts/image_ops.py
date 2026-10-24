#!/usr/bin/env python3
"""Build and push the reviewed DEV image; never obtain AWS/State credentials."""
import json
import os
import re
import subprocess
import sys

import foundation_contract as c
import foundation_ops as ops
import runtime_contract as runtime

PHASE = "context"
PREFIX = "projects/" + c.PROJECT + "/locations/asia-southeast2/repositories/mypage"


def preflight(env):
    c.context(env, "apply", image=True)
    c.need(env.get("MYPAGE_ROLE") == "apply", "image-role")
    c.need(env.get("MYPAGE_GCP_SERVICE_ACCOUNT") == "mypage-foundation-apply@" + c.PROJECT + ".iam.gserviceaccount.com", "image-identity")
    c.need(re.fullmatch(r"projects/[0-9]+/locations/global/workloadIdentityPools/mypage-foundation/providers/github-actions",
                        env.get("MYPAGE_GCP_WIF_PROVIDER", "")), "image-provider")
    c.need(not any(env.get(key) for key in ("AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN")), "image-no-aws-credentials")


def run(args, *, input_value=None):
    result = subprocess.run(args, cwd=ops.REPO, input=input_value, capture_output=True, text=True, timeout=1200)
    c.need(result.returncode == 0, "image-command")
    return result.stdout


def build_push(env):
    global PHASE
    preflight(env)
    token = env["MYPAGE_GCP_ACCESS_TOKEN"]
    PHASE = "registry"
    repository = ops.request("https://artifactregistry.googleapis.com/v1/" + PREFIX, token)
    c.need(repository.get("name") == PREFIX and repository.get("format") == "DOCKER"
           and repository.get("mode") == "STANDARD_REPOSITORY"
           and repository.get("dockerConfig", {}).get("immutableTags") is True, "image-registry")
    tag_url = "https://artifactregistry.googleapis.com/v1/" + PREFIX + "/packages/mypage-api/tags/" + env["GITHUB_SHA"]
    try:
        ops.request(tag_url, token)
    except ops.urllib.error.HTTPError as error:
        c.need(error.code == 404, "image-tag-lookup")
    else:
        c.need(False, "image-tag-already-exists")
    PHASE = "build"
    tag = runtime.REPOSITORY + ":" + env["GITHUB_SHA"]
    run(["docker", "build", "--platform", "linux/amd64", "--file", "system/Dockerfile.mypage",
         "--label", "org.opencontainers.image.revision=" + env["GITHUB_SHA"], "--tag", tag, "system"])
    PHASE = "offline-check"
    run(["bash", "system/scripts/check-mypage-image.sh", tag])
    PHASE = "push"
    run(["docker", "login", "asia-southeast2-docker.pkg.dev", "--username", "oauth2accesstoken", "--password-stdin"], input_value=token)
    try:
        run(["docker", "push", tag])
    finally:
        run(["docker", "logout", "asia-southeast2-docker.pkg.dev"])
    PHASE = "readback"
    metadata = ops.request(tag_url, token)
    c.need(re.fullmatch(re.escape(PREFIX + "/packages/mypage-api/versions/sha256:") + r"[a-f0-9]{64}", metadata.get("version", "")), "image-digest-readback")
    local_digests = json.loads(run(["docker", "image", "inspect", "--format", "{{json .RepoDigests}}", tag]))
    expected_digest = runtime.REPOSITORY + "@" + metadata["version"].rsplit("/", 1)[-1]
    c.need(type(local_digests) is list and expected_digest in local_digests, "image-local-remote-digest")
    print("Development image built, checked offline and pushed; immutable exact-source tag verified. No Terraform or deployment.")


def main():
    os.umask(0o077)
    try:
        if sys.argv[1:] == ["preflight"]:
            preflight(os.environ)
        elif sys.argv[1:] == ["oidc"]:
            preflight(os.environ)
            ops.oidc(os.environ, image=True)
        elif sys.argv[1:] == ["build-push"]:
            build_push(os.environ)
        else:
            c.need(False, "image-command-selection")
        return 0
    except c.ContractError as error:
        print("MyPage image STOP at " + PHASE + "; check=" + str(error) + ".", file=sys.stderr)
    except Exception:
        print("MyPage image STOP at " + PHASE + "; private diagnostics suppressed.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    raise SystemExit(main())
