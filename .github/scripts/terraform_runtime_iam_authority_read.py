#!/usr/bin/env python3
"""Observe bounded IAM prerequisites; never authorize or perform IAM changes."""
from __future__ import annotations

import os
import stat
import sys
from pathlib import Path

import terraform_runtime_metadata_read as metadata

REQUESTS = ("authority-project", "authority-project-roles", "authority-pool-policy", "authority-sa-policy")
GROUPS = {
    "authority-project-roles": (("authority-role-create", "iam.roles.create"),
                                ("authority-role-get", "iam.roles.get"),
                                ("authority-role-update", "iam.roles.update")),
    "authority-pool-policy": (("authority-pool-get-policy", "iam.workloadIdentityPools.getIamPolicy"),
                              ("authority-pool-set-policy", "iam.workloadIdentityPools.setIamPolicy")),
    "authority-sa-policy": (("authority-sa-get-policy", "iam.serviceAccounts.getIamPolicy"),
                            ("authority-sa-set-policy", "iam.serviceAccounts.setIamPolicy")),
}
PERMISSION_ALIASES = tuple(alias for group in GROUPS.values() for alias, _ in group)
OBSERVATIONS = {"HELD", "NOT_RETURNED", "UNKNOWN", "未実行"}
FLAGS = ("authority-iam-change-authorized", "authority-writer-ready", "authority-bootstrap-ready", "authority-metadata-ready", "authority-migration-ready")


def context(env, number=None):
    try:
        metadata.need(env.get("DEV_RUNTIME_IAM_AUTHORITY_READ_ENABLED") == "true"
                      and env.get("AUTHORITY_AUTH_OUTCOME") == "success"
                      and env.get("AUTHORITY_JOB_STATUS") in {"success", "failure"})
        observed = metadata.context(env, token=True)
        metadata.need(number is None or observed == number)
        return observed
    except Exception:
        raise metadata.RuntimeAdmission("IAM authority read STOP") from None


def permission_response(value, requested):
    metadata.shape(value, {"permissions"})
    granted = value.get("permissions", [])
    metadata.need(type(granted) is list and all(type(item) is str and item in requested for item in granted)
                  and len(granted) == len(set(granted)))
    return set(granted)


def public_lines(statuses, reasons, observations, window):
    metadata.need(set(statuses) == set(reasons) == set(REQUESTS)
                  and all(value in {"PASS", "STOP", "未実行"} for value in statuses.values())
                  and all(value in metadata.REASONS for value in reasons.values())
                  and set(observations) == set(PERMISSION_ALIASES)
                  and all(value in OBSERVATIONS for value in observations.values())
                  and window in {"PASS", "STOP"})
    passed = all(value == "PASS" for value in statuses.values()) and window == "PASS"
    lines = ["runtime-iam-authority-read: " + ("PASS" if passed else "STOP"),
             "runtime-iam-authority-schema: 1", "runtime-iam-authority-purpose: prerequisites-only"]
    for alias in REQUESTS:
        lines.extend((alias + ": " + statuses[alias], alias + "-reason: " + reasons[alias]))
    lines.extend(alias + ": " + observations[alias] for alias in PERMISSION_ALIASES)
    lines.extend(("authority-window: " + window, "authority-cleanup: PASS"))
    lines.extend(alias + ": false" for alias in FLAGS)
    return lines


def publish(env, number, statuses, reasons, observations, window):
    try:
        context(env, number)  # Recheck immediately before either public sink.
    except Exception:
        window = "STOP"
    lines = public_lines(statuses, reasons, observations, window)
    try:
        path = Path(env["GITHUB_STEP_SUMMARY"])
        metadata.need(path.is_absolute() and not path.is_symlink()
                      and path.parent.resolve() == path.parent)
        fd = os.open(path, os.O_WRONLY | os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o600)
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            info = os.fstat(handle.fileno())
            metadata.need(stat.S_ISREG(info.st_mode) and info.st_uid == os.geteuid())
            handle.write("\n".join(lines) + "\n")
    except Exception:
        lines[0] = "runtime-iam-authority-read: STOP"
        raise
    finally:
        print("\n".join(lines))
    return lines[0] == "runtime-iam-authority-read: PASS"


def read(env, *, request=None):
    number = context(env)
    request = request or metadata.google
    statuses = dict.fromkeys(REQUESTS, "未実行")
    reasons = dict.fromkeys(REQUESTS, "未実行")
    observations = dict.fromkeys(PERMISSION_ALIASES, "未実行")
    window = "STOP"

    def response(host, path, body=None, *, method):
        context(env, number)
        value = metadata.exact_response(request, path, env["GCP_SMOKE_ACCESS_TOKEN"], body, host=host, method=method)
        context(env, number)  # A response arriving after admission expires is not accepted.
        return value

    def probe(alias, operation):
        context(env, number)
        try:
            value = operation()
        except Exception as error:
            statuses[alias] = "STOP"
            if alias in GROUPS:
                for permission_alias, _ in GROUPS[alias]:
                    observations[permission_alias] = "UNKNOWN"
            if isinstance(error, metadata.RuntimeAdmission):
                reasons[alias] = "ADMISSION_STOP"
                raise
            reasons[alias] = error.reason if isinstance(error, metadata.ProbeFailure) else "RESPONSE_REJECTED"
            return None
        statuses[alias], reasons[alias] = "PASS", "PASS"
        return value

    def project():
        value = response("cloudresourcemanager.googleapis.com", "v1/projects/" + metadata.PROJECT, method="GET")
        metadata.validate_project(value, number)
        return value

    try:
        if probe("authority-project", project) is not None:
            pool = f"projects/{number}/locations/global/workloadIdentityPools/aws-runtime"
            sa = f"projects/{metadata.PROJECT}/serviceAccounts/{metadata.PROJECT}@appspot.gserviceaccount.com"
            paths = {
                "authority-project-roles": ("cloudresourcemanager.googleapis.com", "v3/projects/" + metadata.PROJECT + ":testIamPermissions"),
                "authority-pool-policy": ("iam.googleapis.com", "v1/" + pool + ":testIamPermissions"),
                "authority-sa-policy": ("iam.googleapis.com", "v1/" + sa + ":testIamPermissions"),
            }
            for alias, group in GROUPS.items():
                host, path = paths[alias]
                requested = [permission for _, permission in group]
                def observe():
                    value = response(host, path, {"permissions": requested}, method="POST")
                    return permission_response(value, requested)
                granted = probe(alias, observe)
                if granted is not None:
                    for permission_alias, permission in group:
                        observations[permission_alias] = "HELD" if permission in granted else "NOT_RETURNED"
        context(env, number)
        window = "PASS"
    except Exception:
        pass
    # Only literal observations persist. Raw responses and tokens are never
    # written to temporary files, receipts, outputs, caches or artifacts.
    metadata.need(publish(env, number, statuses, reasons, observations, window))


def main(argv=None):
    try:
        parser = metadata.PrivateArgumentParser(description=__doc__)
        parser.parse_args(argv)
        read(dict(os.environ))
    except Exception:
        print("runtime-iam-authority-read: STOP", file=sys.stderr)
        return 3
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
