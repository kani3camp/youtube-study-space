#!/usr/bin/env python3
"""Run all credentialless Terraform witnesses in isolated bounded processes."""
from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass
import argparse
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
from threading import Event
import time

ROOT = Path(__file__).resolve().parents[2]
CASES = (
    "test_youtube_quota_alerts.py",
    "test_backup_bucket.py",
    "test_export_topic.py",
    "test_export_scheduler.py",
    "test_export_function.py",
    "test_notification_channels.py",
    "test_user_activity_history.py",
    "test_order_history.py",
    "test_terraform_migration_native_fidelity.py",
    "test_terraform_api_import_fidelity.py",
)
# Start the two longer RPC witnesses first so shorter mocks overlap them.
SCHEDULE = CASES[-2:] + CASES[:-2]
CASE_TIMEOUT = 180
MAX_OUTPUT = 64 * 1024


@dataclass(frozen=True)
class Result:
    name: str
    status: str
    returncode: int | None
    seconds: float
    output: str


def environment(directory: Path, mirror: Path) -> dict[str, str]:
    home, temporary, cache = (directory / name for name in ("home", "tmp", "cache"))
    for path in (home, temporary, cache):
        path.mkdir(mode=0o700)
    config = (
        "disable_checkpoint = true\nprovider_installation {\n"
        " filesystem_mirror {\n  path = " + json.dumps(str(mirror))
        + '\n  include = ["registry.terraform.io/hashicorp/google"]\n }\n}\n'
    )
    # Two older tests deliberately rebuild TF_* env. Their isolated HOME must
    # have the same mirror; no shared writable provider cache is introduced.
    cli = home / ".terraformrc"
    cli.write_text(config)
    cli.chmod(0o600)
    return {
        "PATH": os.environ.get("PATH", os.defpath),
        "HOME": str(home),
        "TMPDIR": str(temporary),
        "TF_CLI_CONFIG_FILE": str(cli),
        "TF_PLUGIN_CACHE_DIR": str(cache),
        "GCP_TEST_PROVIDER_MIRROR": str(mirror),
        "TF_IN_AUTOMATION": "true",
        "TF_INPUT": "false",
        "TF_WORKSPACE": "default",
        "CHECKPOINT_DISABLE": "1",
        "PYTHONDONTWRITEBYTECODE": "1",
        "PYTHONUTF8": "1",
    }


def stop_group(process: subprocess.Popen) -> None:
    # Terraform/provider grandchildren share the case session. Killing only
    # Python leaves native providers alive after a timeout or interruption.
    try:
        os.killpg(process.pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    try:
        process.wait(timeout=1)
    except subprocess.TimeoutExpired:
        pass
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        pass
    # This wait reaps the case leader. Descendant SIGKILL delivery and resource
    # closure are asynchronous; cleanup witnesses observe their own resources.
    process.wait()


def run_case(root: Path, name: str, mirror: Path, timeout: float, cancelled: Event) -> Result:
    started = time.monotonic()
    if cancelled.is_set():
        return Result(name, "CANCELLED", None, 0, "")
    status, code, text = "FAIL", None, ""
    # Native fixtures nest another temporary directory. Keep their IPC paths
    # short even when macOS supplies a long per-user parent TMPDIR.
    with tempfile.TemporaryDirectory(prefix="gcp-native-", dir=Path("/tmp").resolve()) as location:
        directory = Path(location).resolve()
        env = environment(directory, mirror)
        with tempfile.TemporaryFile() as output:
            process = None
            try:
                process = subprocess.Popen(
                    [sys.executable, "-B", str(root / ".github/scripts" / name)],
                    cwd=root, env=env, stdin=subprocess.DEVNULL,
                    stdout=output, stderr=subprocess.STDOUT, start_new_session=True,
                )
                deadline = time.monotonic() + timeout
                while True:
                    if cancelled.is_set():
                        status = "CANCELLED"
                        break
                    remaining = deadline - time.monotonic()
                    if remaining <= 0:
                        status = "TIMEOUT"
                        break
                    try:
                        code = process.wait(timeout=min(remaining, 0.2))
                        status = "PASS" if code == 0 else "FAIL"
                        break
                    except subprocess.TimeoutExpired:
                        continue
            except OSError:
                status = "LAUNCH_ERROR"
            finally:
                if process is not None:
                    stop_group(process)
                    code = process.returncode
            output.seek(0)
            captured = output.read(MAX_OUTPUT + 1)
            text = captured[:MAX_OUTPUT].decode("utf-8", errors="replace")
            if len(captured) > MAX_OUTPUT:
                text += "\n[case output truncated]\n"
    return Result(name, status, code, time.monotonic() - started, text)


def run_suite(root: Path, mirror: Path, *, workers: int = 2,
              timeout: float = CASE_TIMEOUT) -> list[Result]:
    if workers not in (1, 2, 4) or timeout <= 0:
        raise ValueError("bounded workers and a positive timeout are required")
    if not mirror.is_absolute() or not mirror.is_dir() or mirror.is_symlink():
        raise ValueError("an existing absolute provider mirror is required")
    # Submit every case once, including after a sibling failure. Each result is
    # retained; a failed case never cancels or silently omits another witness.
    cancelled = Event()
    with ThreadPoolExecutor(max_workers=workers) as pool:
        try:
            pending = {
                name: pool.submit(run_case, root, name, mirror, timeout, cancelled)
                for name in SCHEDULE
            }
            results = [pending[name].result() for name in CASES]
        except BaseException:
            cancelled.set()
            raise
    if len(results) != len(CASES) or {row.name for row in results} != set(CASES):
        raise RuntimeError("native witness coverage is incomplete")
    return results


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--workers", type=int, choices=(1, 2, 4), default=2)
    args = parser.parse_args(argv)
    mirror = Path(os.environ.get(
        "GCP_TEST_PROVIDER_MIRROR",
        str(ROOT / "infra/gcp/environments/dev/.terraform/providers"),
    ))
    try:
        results = run_suite(ROOT, mirror, workers=args.workers)
    except KeyboardInterrupt:
        print("TERRAFORM_NATIVE_TESTS: STOP; interrupted; process-group termination requested")
        return 130
    except Exception:
        print("TERRAFORM_NATIVE_TESTS: STOP; suite preflight or execution failed")
        return 1
    for row in results:
        print(f"::group::{row.name}")
        print(row.output, end="" if row.output.endswith("\n") else "\n")
        print(f"TERRAFORM_NATIVE_CASE: {row.status}; case={row.name}; "
              f"exit={row.returncode}; seconds={row.seconds:.3f}")
        print("::endgroup::")
    passed = sum(row.status == "PASS" for row in results)
    print(f"TERRAFORM_NATIVE_TESTS: completed={len(results)}; "
          f"passed={passed}; failed={len(results) - passed}; workers={args.workers}")
    return 0 if passed == len(CASES) else 1


def interrupt(_signal, _frame) -> None:
    raise KeyboardInterrupt


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, interrupt)
    raise SystemExit(main())
