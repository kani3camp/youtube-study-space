#!/usr/bin/env python3
"""Collect one exact ECS runtime caller with an existing AWS operator identity.

This tool performs metadata reads only. It never assumes a role, changes IAM,
accesses Terraform state, invokes workloads, or certifies migration readiness.
All selectors and captured role/Google-auth metadata stay in private files.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys

AUTH_KEYS = {
    "GOOGLE_CLOUD_PROJECT", "GCP_WIF_AUDIENCE", "GCP_WIF_SERVICE_ACCOUNT_EMAIL",
}
SELECTOR_KEYS = {
    "schema_version", "environment", "aws_account_id", "aws_region",
    "stack_name", "task_definition_arn", "expected_auth",
}
MAX_BYTES = 65536
REPOSITORY_ROOT = Path(__file__).resolve().parents[3]
PROJECTS = {"dev": "test-youtube-study-space", "prod": "youtube-study-space"}


class PrivateArgumentParser(argparse.ArgumentParser):
    def error(self, message):
        raise ValueError("Invalid arguments")


def need(value):
    if not value:
        raise ValueError("Private metadata contract rejected")


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        need(key not in result)
        result[key] = value
    return result


def private_parent(path):
    need(path.is_absolute() and path == path.resolve())
    need(not path.is_relative_to(REPOSITORY_ROOT))
    need(not any((parent / ".git").exists() for parent in path.parents))
    metadata = path.parent.stat()
    need(stat.S_ISDIR(metadata.st_mode) and metadata.st_uid == os.getuid()
         and stat.S_IMODE(metadata.st_mode) == 0o700)


def private_input(path):
    private_parent(path)
    descriptor = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(descriptor) as handle:
        metadata = os.fstat(handle.fileno())
        need(stat.S_ISREG(metadata.st_mode) and metadata.st_uid == os.getuid()
             and stat.S_IMODE(metadata.st_mode) == 0o600
             and metadata.st_size <= MAX_BYTES)
        text = handle.read(MAX_BYTES + 1)
    need(len(text) <= MAX_BYTES)
    return json.loads(text, object_pairs_hook=unique_object)


def validate_selectors(value):
    need(type(value) is dict and set(value) == SELECTOR_KEYS)
    need(type(value["schema_version"]) is int and value["schema_version"] == 1)
    need(value["environment"] in {"dev", "prod"})
    need(type(value["aws_account_id"]) is str
         and re.fullmatch(r"[0-9]{12}", value["aws_account_id"]))
    need(value["aws_region"] == "ap-northeast-1"
         and value["stack_name"] == "AwsCdkStack")
    arn = value["task_definition_arn"]
    prefix = "arn:aws:ecs:ap-northeast-1:" + value["aws_account_id"]
    need(type(arn) is str and re.fullmatch(
        re.escape(prefix) + r":task-definition/[A-Za-z0-9_-]{1,255}:[1-9][0-9]*", arn))
    auth = value["expected_auth"]
    need(type(auth) is dict and set(auth) == AUTH_KEYS)
    need(all(type(v) is str and v and len(v) <= 512 for v in auth.values()))
    project = PROJECTS[value["environment"]]
    need(auth["GOOGLE_CLOUD_PROJECT"] == project)
    need(re.fullmatch(r"//iam.googleapis.com/projects/[0-9]+/locations/global/"
                      r"workloadIdentityPools/aws-runtime/providers/aws-provider",
                      auth["GCP_WIF_AUDIENCE"]))
    need(auth["GCP_WIF_SERVICE_ACCOUNT_EMAIL"] == project + "@appspot.gserviceaccount.com")
    return value


def aws_read(service, operation, arguments, query, region):
    # Commands and endpoint selectors are assembled by collect(), never by input.
    need((service, operation) in {
        ("sts", "get-caller-identity"), ("cloudformation", "describe-stacks"),
        ("cloudformation", "list-stack-resources"), ("ecs", "describe-task-definition"),
    })
    command = ["aws", service, operation, *arguments, "--region", region,
               "--output", "json", "--no-cli-pager", "--cli-connect-timeout", "10",
               "--cli-read-timeout", "20", "--query", query]
    child_environment = os.environ.copy()
    child_environment.update(AWS_PAGER="", AWS_CLI_AUTO_PROMPT="off", AWS_MAX_ATTEMPTS="1")
    result = subprocess.run(command, env=child_environment, capture_output=True,
                            text=True, check=False, timeout=45)
    need(result.returncode == 0 and len(result.stdout) <= MAX_BYTES)
    # Never echo AWS error bodies, caller identities, credentials, or raw output.
    return json.loads(result.stdout, object_pairs_hook=unique_object)


def collect(selectors, read=aws_read):
    selectors = validate_selectors(selectors)
    region = selectors["aws_region"]
    account = selectors["aws_account_id"]
    identity = read("sts", "get-caller-identity", [], "{account:Account}", region)
    need(type(identity) is dict and identity == {"account": account})
    stack = read("cloudformation", "describe-stacks",
                 ["--stack-name", selectors["stack_name"]],
                 "Stacks[].Outputs[?OutputKey==`DailyBatchTaskDefinitionArn`].OutputValue[]",
                 region)
    need(stack == [selectors["task_definition_arn"]])
    physical = read("cloudformation", "list-stack-resources",
                    ["--stack-name", selectors["stack_name"]],
                    "StackResourceSummaries[?ResourceType==`AWS::ECS::TaskDefinition` && "
                    "starts_with(LogicalResourceId, `DailyBatchTaskDefinition`)].PhysicalResourceId",
                    region)
    need(physical == [selectors["task_definition_arn"]])
    task = read("ecs", "describe-task-definition",
                ["--task-definition", selectors["task_definition_arn"]],
                "taskDefinition.{arn:taskDefinitionArn,task_role:taskRoleArn,"
                "execution_role:executionRoleArn,status:status,revision:revision,"
                "containers:containerDefinitions[?name==`daily-batch`].{name:name,"
                "auth:environment[?name==`GOOGLE_CLOUD_PROJECT` || name==`GCP_WIF_AUDIENCE` "
                "|| name==`GCP_WIF_SERVICE_ACCOUNT_EMAIL`]}}", region)
    need(type(task) is dict and set(task) == {
        "arn", "task_role", "execution_role", "status", "revision", "containers",
    })
    need(task["arn"] == selectors["task_definition_arn"] and task["status"] == "ACTIVE")
    need(type(task["revision"]) is int
         and task["revision"] == int(task["arn"].rsplit(":", 1)[1]))
    for name in ("task_role", "execution_role"):
        need(type(task[name]) is str and re.fullmatch(
            r"arn:aws:iam::" + account + r":role/[A-Za-z0-9+=,.@_/-]{1,512}", task[name]))
    need(type(task["containers"]) is list and len(task["containers"]) == 1)
    container = task["containers"][0]
    need(type(container) is dict and set(container) == {"name", "auth"}
         and container["name"] == "daily-batch")
    pairs = container["auth"]
    need(type(pairs) is list and len(pairs) == len(AUTH_KEYS))
    need(all(type(pair) is dict and set(pair) == {"name", "value"} for pair in pairs))
    auth = unique_object((pair["name"], pair["value"]) for pair in pairs)
    need(auth == selectors["expected_auth"])
    return {
        "schema_version": 1, "capture_type": "aws-runtime-task-metadata-v1",
        "captured_at": datetime.now(timezone.utc).isoformat(),
        "environment": selectors["environment"], "aws_region": region,
        "task_definition_arn": task["arn"], "task_role_arn": task["task_role"],
        "execution_role_arn": task["execution_role"], "runtime_auth": auth,
        "stack_reference_match": True, "three_auth_env_match": True,
        "metadata_reads": 4, "cloud_mutations": 0, "readiness_attested": False,
    }


def main(argv=None):
    output_path = None
    descriptor = None
    created_identity = None
    try:
        parser = PrivateArgumentParser(description=__doc__)
        parser.add_argument("--selectors", type=Path, required=True)
        parser.add_argument("--output", type=Path, required=True)
        arguments = parser.parse_args(argv)
        selectors = validate_selectors(private_input(arguments.selectors))
        private_parent(arguments.output)
        output_path = arguments.output
        # Reserve exclusive output before authentication; existing files and links fail closed.
        descriptor = os.open(output_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
        metadata = os.fstat(descriptor)
        created_identity = (metadata.st_dev, metadata.st_ino)
        result = collect(selectors)
        with os.fdopen(descriptor, "w") as handle:
            descriptor = None
            os.fchmod(handle.fileno(), 0o600)
            json.dump(result, handle, sort_keys=True, indent=2)
            handle.write("\n")
        print("AWS runtime task metadata captured privately; readiness remains unverified.")
        return 0
    except Exception:
        if descriptor is not None:
            os.close(descriptor)
        if output_path is not None and created_identity is not None:
            try:
                metadata = output_path.lstat()
                if (metadata.st_dev, metadata.st_ino) == created_identity:
                    output_path.unlink()
            except OSError:
                pass
        print("AWS runtime task metadata STOP; private diagnostic suppressed.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
