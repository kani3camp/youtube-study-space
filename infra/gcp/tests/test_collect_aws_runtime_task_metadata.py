#!/usr/bin/env python3
"""Dummy-only operator metadata and private-output failure contracts."""
from __future__ import annotations

import contextlib
import copy
import importlib.util
import io
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
SCRIPT = ROOT / "infra/gcp/scripts/collect_aws_runtime_task_metadata.py"
spec = importlib.util.spec_from_file_location("collect_aws_runtime_task_metadata", SCRIPT)
collector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(collector)
collector.PROJECTS = {"dev": "example-study-space-dev", "prod": "example-study-space-prod"}


def selectors():
    return {
        "schema_version": 1, "environment": "dev", "aws_account_id": "222222222222",
        "aws_region": "ap-northeast-1", "stack_name": "AwsCdkStack",
        "task_definition_arn": "arn:aws:ecs:ap-northeast-1:222222222222:task-definition/DummyDailyBatch:7",
        "expected_auth": {
            "GOOGLE_CLOUD_PROJECT": "example-study-space-dev",
            "GCP_WIF_AUDIENCE": "//iam.googleapis.com/projects/111111111111/locations/global/"
                                "workloadIdentityPools/aws-runtime/providers/aws-provider",
            "GCP_WIF_SERVICE_ACCOUNT_EMAIL": "example-study-space-dev@appspot.gserviceaccount.com",
        },
    }


def responses():
    source = selectors()
    return [
        {"account": source["aws_account_id"]},
        [source["task_definition_arn"]], [source["task_definition_arn"]],
        {"arn": source["task_definition_arn"], "revision": 7, "status": "ACTIVE",
         "task_role": "arn:aws:iam::222222222222:role/DummyRuntimeTaskRole",
         "execution_role": "arn:aws:iam::222222222222:role/DummyExecutionRole",
         "containers": [{"name": "daily-batch", "auth": [
             {"name": key, "value": value} for key, value in source["expected_auth"].items()
         ]}]},
    ]


class ExactTaskRead(unittest.TestCase):
    def test_fixed_metadata_reads_preserve_task_and_execution_role_distinction(self):
        values = responses()
        calls = []

        def read(*arguments):
            calls.append(arguments)
            return values[len(calls) - 1]

        packet = collector.collect(selectors(), read)
        self.assertEqual([(c[0], c[1]) for c in calls], [
            ("sts", "get-caller-identity"), ("cloudformation", "describe-stacks"),
            ("cloudformation", "list-stack-resources"), ("ecs", "describe-task-definition"),
        ])
        self.assertEqual(calls[-1][2], ["--task-definition", selectors()["task_definition_arn"]])
        self.assertEqual(packet["task_role_arn"], values[-1]["task_role"])
        self.assertEqual(packet["execution_role_arn"], values[-1]["execution_role"])
        self.assertNotEqual(packet["task_role_arn"], packet["execution_role_arn"])
        self.assertEqual(packet["runtime_auth"], selectors()["expected_auth"])
        self.assertEqual(packet["cloud_mutations"], 0)
        self.assertFalse(packet["readiness_attested"])

    def test_account_or_stack_mismatch_stops_before_task_metadata(self):
        cases = [(0, {"account": "333333333333"}),
                 (1, ["private wrong stack revision"]), (2, [])]
        for index, bad_value in cases:
            with self.subTest(index=index):
                values = responses()
                values[index] = bad_value
                calls = []

                def read(*arguments):
                    calls.append(arguments)
                    return values[len(calls) - 1]

                with self.assertRaises(ValueError):
                    collector.collect(selectors(), read)
                self.assertEqual(len(calls), index + 1)

    def test_wrong_environment_project_or_default_runtime_sa_rejected_before_reads(self):
        for change in ("environment", "GOOGLE_CLOUD_PROJECT", "GCP_WIF_SERVICE_ACCOUNT_EMAIL"):
            source = selectors()
            if change == "environment":
                source["environment"] = "prod"
            else:
                source["expected_auth"][change] = "wrong-private-project-or-sa"
            with self.subTest(change=change), self.assertRaises(ValueError):
                collector.collect(source, lambda *args: self.fail("Unexpected AWS read"))

    def test_stale_revision_wrong_auth_missing_or_duplicate_env_rejected(self):
        cases = []
        task = responses()[-1]
        bad = copy.deepcopy(task); bad["revision"] = 8; cases.append(bad)
        bad = copy.deepcopy(task); bad["status"] = "INACTIVE"; cases.append(bad)
        bad = copy.deepcopy(task); bad["task_role"] = "arn:aws:iam::333333333333:role/Other"; cases.append(bad)
        bad = copy.deepcopy(task); bad["containers"][0]["auth"][0]["value"] = "private wrong auth"; cases.append(bad)
        bad = copy.deepcopy(task); bad["containers"][0]["auth"].pop(); cases.append(bad)
        bad = copy.deepcopy(task); bad["containers"][0]["auth"][1] = bad["containers"][0]["auth"][0]; cases.append(bad)
        for candidate in cases:
            values = iter(responses()[:-1] + [candidate])
            with self.assertRaises(ValueError):
                collector.collect(selectors(), lambda *args: next(values))

    def test_cli_capture_inherits_credentials_without_changing_environment(self):
        class Result:
            returncode = 0
            stdout = '{"account":"222222222222"}'

        with patch.dict(os.environ, {"AWS_ACCESS_KEY_ID": "dummy-access", "AWS_PROFILE": "dummy-profile"}):
            original = os.environ.copy()
            with patch.object(collector.subprocess, "run", return_value=Result()) as run:
                collector.aws_read("sts", "get-caller-identity", [], "{account:Account}", "ap-northeast-1")
            arguments, options = run.call_args
            self.assertEqual(arguments[0][:3], ["aws", "sts", "get-caller-identity"])
            self.assertEqual(options["env"].get("AWS_ACCESS_KEY_ID"), "dummy-access")
            self.assertEqual(options["env"].get("AWS_PROFILE"), "dummy-profile")
            # A failed check must not render the real inherited environment.
            self.assertTrue(os.environ.copy() == original)
        with self.assertRaises(ValueError):
            collector.aws_read("ecs", "run-task", [], "{}", "ap-northeast-1")


class PrivateFiles(unittest.TestCase):
    def setUp(self):
        # The managed sandbox exposes empty read-only .git guards at its roots.
        # Exclude only those guards in dummy tests; production detection remains
        # strict, including empty .git fixtures and real checkout/worktree paths.
        path_exists = Path.exists
        guards = set()
        for root in (Path("/tmp/.git"), Path("/workspace/.git")):
            if root.is_dir() and root.stat().st_mode & 0o777 == 0o555 and not list(root.iterdir()):
                guards.add(root)
        guard_patch = patch.object(Path, "exists", lambda path: False if path in guards else path_exists(path))
        guard_patch.start()
        self.addCleanup(guard_patch.stop)
        self.temp = tempfile.TemporaryDirectory(prefix="dummy-aws-runtime-read-")
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name)
        self.directory.chmod(0o700)
        self.source = self.directory / "selectors.json"
        self.source.write_text(json.dumps(selectors()))
        self.source.chmod(0o600)
        self.output = self.directory / "capture.json"

    def run_cli(self):
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            code = collector.main(["--selectors", str(self.source), "--output", str(self.output)])
        return code, stdout.getvalue(), stderr.getvalue()

    def test_success_is_exclusive_private_and_stdout_contains_no_metadata(self):
        with patch.object(collector, "collect", return_value={"private": "private role sentinel"}) as collect:
            code, out, err = self.run_cli()
            self.assertEqual(code, 0)
            self.assertEqual(self.output.stat().st_mode & 0o777, 0o600)
            self.assertNotIn("private role sentinel", out + err)
            self.assertEqual(json.loads(self.output.read_text()), {"private": "private role sentinel"})
            self.assertEqual(self.run_cli()[0], 1)
            self.assertEqual(collect.call_count, 1)

    def test_failure_cleans_reserved_file_and_suppresses_private_error(self):
        with patch.object(collector, "collect", side_effect=ValueError("private role sentinel")):
            code, out, err = self.run_cli()
        self.assertEqual(code, 1)
        self.assertFalse(self.output.exists())
        self.assertNotIn("private role sentinel", out + err)

    def test_symlink_world_readable_and_checkout_paths_rejected_before_auth(self):
        link = self.directory / "linked-selectors.json"
        link.symlink_to(self.source)
        original_source = self.source
        self.source = link
        with patch.object(collector, "collect") as collect:
            self.assertEqual(self.run_cli()[0], 1)
            collect.assert_not_called()
        self.source = original_source
        self.source.chmod(0o644)
        with patch.object(collector, "collect") as collect:
            self.assertEqual(self.run_cli()[0], 1)
            collect.assert_not_called()
        self.source.chmod(0o600)
        self.output.symlink_to(self.source)
        with patch.object(collector, "collect") as collect:
            self.assertEqual(self.run_cli()[0], 1)
            collect.assert_not_called()
        with self.assertRaises(ValueError):
            collector.private_parent(ROOT / "dummy-private-capture.json")
        checkout = self.directory / "dummy-checkout"
        checkout.mkdir(mode=0o700)
        (checkout / ".git").mkdir()
        with self.assertRaises(ValueError):
            collector.private_parent(checkout / "private-capture.json")

    def test_duplicate_keys_credentials_and_oversized_input_rejected(self):
        for payload in ('{"schema_version":1,"schema_version":1}', "x" * 65537,
                        json.dumps(dict(selectors(), AWS_SECRET_ACCESS_KEY="dummy secret"))):
            self.source.write_text(payload)
            with patch.object(collector, "collect") as collect:
                code, out, err = self.run_cli()
                self.assertEqual(code, 1)
                collect.assert_not_called()
                self.assertNotIn("dummy secret", out + err)


if __name__ == "__main__":
    unittest.main()
