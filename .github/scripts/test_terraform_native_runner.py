#!/usr/bin/env python3
"""Observable coverage, isolation and failure contracts for the native runner."""
from __future__ import annotations

from collections import Counter
from contextlib import redirect_stdout
from concurrent.futures import ThreadPoolExecutor
import importlib.util
import io
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location(
    "native_runner", Path(__file__).with_name("run_terraform_native_tests.py"))
runner = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = runner
spec.loader.exec_module(runner)

EXPECTED = (
    "test_youtube_quota_alerts.py", "test_backup_bucket.py", "test_export_topic.py",
    "test_export_scheduler.py", "test_export_function.py", "test_notification_channels.py",
    "test_user_activity_history.py", "test_order_history.py",
    "test_terraform_migration_native_fidelity.py", "test_terraform_api_import_fidelity.py",
)


class NativeRunnerTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="native-runner-contract-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.scripts = self.root / ".github/scripts"
        self.scripts.mkdir(parents=True)
        self.mirror = self.root / "provider-mirror"
        self.mirror.mkdir()
        (self.mirror / "immutable").write_text("read-only fixture")
        self.observed = self.root / "observed"
        self.observed.mkdir()
        for name in EXPECTED:
            (self.scripts / name).write_text(
                "import json,os,pathlib,time\n"
                "p=pathlib.Path(__file__).parents[2]/'observed'\n"
                "v={'environment':dict(os.environ),'start':time.monotonic()}\n"
                "time.sleep(.08)\n"
                "v['end']=time.monotonic()\n"
                "(p/(pathlib.Path(__file__).stem+'.json')).write_text(json.dumps(v))\n"
            )

    def test_exact_original_ten_witnesses_are_preserved(self):
        self.assertEqual(runner.CASES, EXPECTED)
        self.assertEqual(Counter(runner.SCHEDULE), Counter(EXPECTED))
        self.assertEqual(runner.SCHEDULE[:2], EXPECTED[-2:])

    def test_all_ten_execute_once_with_bounded_parallel_overlap(self):
        for workers in (2, 4):
            with self.subTest(workers=workers):
                rows = runner.run_suite(self.root, self.mirror, workers=workers)
                self.assertEqual([r.name for r in rows], list(EXPECTED))
                self.assertTrue(all(r.status == "PASS" and r.returncode == 0 for r in rows))
                observations = [json.loads(p.read_text()) for p in self.observed.iterdir()]
                self.assertEqual(len(observations), 10)
                events = [(v["start"], 1) for v in observations] + [(v["end"], -1) for v in observations]
                active = peak = 0
                for _, delta in sorted(events):
                    active += delta
                    peak = max(peak, active)
                self.assertGreaterEqual(peak, 2)
                self.assertLessEqual(peak, workers)

    def test_credentials_overrides_and_shared_writable_directories_are_not_inherited(self):
        forbidden = {
            "AWS_ACCESS_KEY_ID": "canary", "GOOGLE_APPLICATION_CREDENTIALS": "/canary",
            "GITHUB_TOKEN": "canary", "CLOUDSDK_CONFIG": "/canary", "TF_LOG": "TRACE",
            "TF_VAR_secret": "canary", "TF_CLI_ARGS": "-lock=false",
            "TF_CLI_CONFIG_FILE": "/canary", "PYTHONPATH": "/canary",
        }
        with patch.dict(os.environ, forbidden):
            rows = runner.run_suite(self.root, self.mirror, workers=2)
        self.assertTrue(all(r.status == "PASS" for r in rows))
        values = [json.loads(p.read_text())["environment"] for p in self.observed.iterdir()]
        for key in ("HOME", "TMPDIR", "TF_PLUGIN_CACHE_DIR"):
            self.assertEqual(len({v[key] for v in values}), 10)
        for env in values:
            self.assertFalse(set(forbidden) & set(env) - {"TF_CLI_CONFIG_FILE"})
            self.assertNotEqual(env["TF_CLI_CONFIG_FILE"], "/canary")
            self.assertEqual(env["GCP_TEST_PROVIDER_MIRROR"], str(self.mirror))
            self.assertEqual(env["TF_WORKSPACE"], "default")
        self.assertEqual((self.mirror / "immutable").read_text(), "read-only fixture")

    def test_home_mirror_survives_legacy_tf_environment_rebuild(self):
        with tempfile.TemporaryDirectory() as location:
            env = runner.environment(Path(location), self.mirror)
            home_config = (Path(env["HOME"]) / ".terraformrc").read_text()
            self.assertIn(str(self.mirror), home_config)
            self.assertIn("filesystem_mirror", home_config)
            self.assertNotIn("direct", home_config)
            self.assertEqual(Path(env["TF_CLI_CONFIG_FILE"]), Path(env["HOME"]) / ".terraformrc")
            self.assertNotIn("TF_CLI_ARGS", env)

    def test_failure_preserves_nonzero_exit_and_does_not_omit_other_cases(self):
        (self.scripts / EXPECTED[0]).write_text("raise SystemExit(7)\n")
        rows = runner.run_suite(self.root, self.mirror, workers=2)
        self.assertEqual(len(rows), 10)
        self.assertEqual((rows[0].status, rows[0].returncode), ("FAIL", 7))
        self.assertEqual(sum(r.status == "PASS" for r in rows), 9)
        with patch.object(runner, "run_suite", return_value=rows), redirect_stdout(io.StringIO()) as output:
            self.assertEqual(runner.main([]), 1)
        self.assertIn("completed=10; passed=9; failed=1", output.getvalue())

    def test_launch_error_is_recorded_and_other_cases_still_execute(self):
        original = runner.subprocess.Popen
        def launch(args, **kwargs):
            if args[-1].endswith(EXPECTED[0]):
                raise OSError("synthetic launch failure")
            return original(args, **kwargs)
        with patch.object(runner.subprocess, "Popen", side_effect=launch):
            rows = runner.run_suite(self.root, self.mirror, workers=2)
        self.assertEqual(rows[0].status, "LAUNCH_ERROR")
        self.assertEqual(sum(r.status == "PASS" for r in rows), 9)

    def test_timeout_kills_grandchildren_and_remaining_cases_complete(self):
        marker = self.root / "port.json"
        grandchild = (
            "import pathlib,signal,socket,time,json;"
            "signal.signal(signal.SIGTERM,signal.SIG_IGN);"
            "s=socket.socket();s.bind(('127.0.0.1',0));s.listen();"
            f"pathlib.Path({str(marker)!r}).write_text(json.dumps(s.getsockname()[1]));"
            "time.sleep(30)"
        )
        (self.scripts / EXPECTED[0]).write_text(
            "import subprocess,sys,time\n"
            f"subprocess.Popen([sys.executable,'-c',{grandchild!r}])\n"
            "time.sleep(30)\n")
        rows = runner.run_suite(self.root, self.mirror, workers=2, timeout=1)
        self.assertEqual(rows[0].status, "TIMEOUT")
        self.assertEqual(sum(r.status == "PASS" for r in rows), 9)
        self.assertTrue(marker.exists())
        port = json.loads(marker.read_text())
        with socket.socket() as connection:
            self.assertNotEqual(connection.connect_ex(("127.0.0.1", port)), 0)

    def test_sigterm_cancels_queued_cases_and_kills_active_grandchildren(self):
        marker = self.root / "cancel-port.json"
        grandchild = (
            "import pathlib,signal,socket,time,json;"
            "signal.signal(signal.SIGTERM,signal.SIG_IGN);"
            "s=socket.socket();s.bind(('127.0.0.1',0));s.listen();"
            f"pathlib.Path({str(marker)!r}).write_text(json.dumps(s.getsockname()[1]));"
            "time.sleep(30)"
        )
        (self.scripts / EXPECTED[-2]).write_text(
            "import subprocess,sys,time\n"
            f"subprocess.Popen([sys.executable,'-c',{grandchild!r}])\n"
            "time.sleep(30)\n")
        program = (
            "import importlib.util,pathlib,signal,sys;"
            f"s=importlib.util.spec_from_file_location('cancel_runner',{str(Path(runner.__file__))!r});"
            "m=importlib.util.module_from_spec(s);sys.modules[s.name]=m;s.loader.exec_module(m);"
            f"m.ROOT=pathlib.Path({str(self.root)!r});"
            "signal.signal(signal.SIGTERM,m.interrupt);raise SystemExit(m.main())"
        )
        env = {**os.environ, "GCP_TEST_PROVIDER_MIRROR": str(self.mirror)}
        process = subprocess.Popen(
            [sys.executable, "-B", "-c", program], env=env,
            stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
        try:
            deadline = time.monotonic() + 5
            while not marker.exists() and process.poll() is None and time.monotonic() < deadline:
                time.sleep(0.02)
            self.assertTrue(marker.exists())
            process.send_signal(signal.SIGTERM)
            stdout, stderr = process.communicate(timeout=5)
            self.assertEqual(process.returncode, 130)
            self.assertIn(b"child groups terminated", stdout)
            self.assertEqual(stderr, b"")
            with socket.socket() as connection:
                self.assertNotEqual(connection.connect_ex(("127.0.0.1", json.loads(marker.read_text()))), 0)
        finally:
            if process.poll() is None:
                process.kill()
            process.communicate(timeout=5)

    def test_interruption_during_partial_submission_stops_the_started_case(self):
        marker = self.root / "submit-port.json"
        grandchild = (
            "import pathlib,signal,socket,time,json;"
            "signal.signal(signal.SIGTERM,signal.SIG_IGN);"
            "s=socket.socket();s.bind(('127.0.0.1',0));s.listen();"
            f"pathlib.Path({str(marker)!r}).write_text(json.dumps(s.getsockname()[1]));"
            "time.sleep(30)"
        )
        (self.scripts / EXPECTED[-2]).write_text(
            "import subprocess,sys,time\n"
            f"subprocess.Popen([sys.executable,'-c',{grandchild!r}])\n"
            "time.sleep(30)\n")
        class PartialPool:
            def __init__(self, **kwargs):
                self.real = ThreadPoolExecutor(**kwargs)
                self.submitted = 0
            def __enter__(self):
                self.real.__enter__()
                return self
            def submit(self, *args):
                if self.submitted:
                    raise KeyboardInterrupt
                self.submitted += 1
                result = self.real.submit(*args)
                deadline = time.monotonic() + 5
                while not marker.exists() and time.monotonic() < deadline:
                    time.sleep(0.02)
                if not marker.exists():
                    raise AssertionError("fixture case did not start")
                return result
            def __exit__(self, *args):
                return self.real.__exit__(*args)
        started = time.monotonic()
        with patch.object(runner, "ThreadPoolExecutor", PartialPool), self.assertRaises(KeyboardInterrupt):
            runner.run_suite(self.root, self.mirror)
        self.assertLess(time.monotonic() - started, 5)
        self.assertEqual(list(self.observed.iterdir()), [])
        with socket.socket() as connection:
            self.assertNotEqual(connection.connect_ex(("127.0.0.1", json.loads(marker.read_text()))), 0)

    def test_invalid_worker_counts_and_missing_mirror_fail_closed(self):
        for count in (0, 3, 5, 10):
            with self.subTest(workers=count), self.assertRaises(ValueError):
                runner.run_suite(self.root, self.mirror, workers=count)
        with self.assertRaises(ValueError):
            runner.run_suite(self.root, self.root / "absent")
        self.assertEqual(list(self.observed.iterdir()), [])

    def test_workflow_requires_runner_contract_and_bounded_whole_suite(self):
        text = (runner.ROOT / ".github/workflows/ci.yml").read_text()
        self.assertIn("python3 .github/scripts/test_terraform_native_runner.py", text)
        self.assertIn("run: python3 -B .github/scripts/run_terraform_native_tests.py --workers 2", text)
        native = text.split("      - name: Test quota alert mock plans and import-only boundary\n", 1)[1].split("      - name:", 1)[0]
        self.assertNotIn("continue-on-error", native)


if __name__ == "__main__":
    unittest.main()
