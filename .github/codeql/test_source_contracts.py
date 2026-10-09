"""Negative security/coverage contracts and compile-only synthetic module fixtures."""
import copy
import importlib.util
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("source_contracts", HERE / "validate-source.py")
contract = importlib.util.module_from_spec(spec)
spec.loader.exec_module(contract)


class SourceContracts(unittest.TestCase):
    def setUp(self):
        self.caller = contract.load_yaml(HERE / "templates/codeql-advanced.yml.template")
        self.callee = contract.load_yaml(HERE / "templates/codeql-analyzer.yml.template")
        self.static = contract.load_yaml(contract.ROOT / ".github/workflows/codeql-source-contracts.yml")

    def validate(self, active=None):
        contract.validate(self.caller, self.callee, self.static, active or [self.static])

    def test_preparation_is_inert_and_complete(self):
        self.validate()

    def test_permission_expansion_rejected_at_each_boundary(self):
        boundaries = [self.caller, self.caller["jobs"]["analyze"], self.callee, self.callee["jobs"]["analyze"], self.static]
        for boundary in boundaries:
            for permission, value in [("id-token", "write"), ("contents", "write"), ("actions", "write")]:
                with self.subTest(permission=permission, boundary=boundary.get("name")):
                    original = copy.deepcopy(boundary["permissions"])
                    boundary["permissions"][permission] = value
                    with self.assertRaisesRegex(ValueError, "permissions"):
                        self.validate()
                    boundary["permissions"] = original

    def test_secret_inheritance_rejected(self):
        self.caller["jobs"]["analyze"]["secrets"] = "inherit"
        with self.assertRaisesRegex(ValueError, "secrets"):
            self.validate()

    def test_static_ci_cannot_upload_security_results(self):
        self.static["permissions"]["security-events"] = "write"
        with self.assertRaisesRegex(ValueError, "permissions"):
            self.validate()

    def test_static_ci_cannot_run_real_go_build_or_analyzer(self):
        steps = self.static["jobs"]["source-contracts"]["steps"]
        for command in ["bash .github/codeql/build-go.sh", "codeql database analyze db", "go run ./cmd/youtube-bot"]:
            with self.subTest(command=command):
                steps.append({"run": command})
                with self.assertRaisesRegex(ValueError, "reviewed static commands"):
                    self.validate()
                steps.pop()

    def test_environment_cannot_be_selected_by_caller(self):
        self.callee["jobs"]["analyze"]["environment"] = "${{ inputs.environment }}"
        with self.assertRaisesRegex(ValueError, "context"):
            self.validate()

    def test_callee_cannot_be_directly_dispatched(self):
        self.callee["on"]["workflow_dispatch"] = None
        with self.assertRaisesRegex(ValueError, "direct trigger"):
            self.validate()

    def test_enabled_caller_rejected(self):
        self.caller["jobs"]["analyze"]["if"] = "${{ true }}"
        with self.assertRaisesRegex(ValueError, "disabled"):
            self.validate()

    def test_all_languages_retained_and_go_none_rejected(self):
        matrix = self.callee["jobs"]["analyze"]["strategy"]["matrix"]["include"]
        for index in range(3):
            with self.subTest(missing=matrix[index]["language"]):
                original = matrix.pop(index)
                with self.assertRaisesRegex(ValueError, "three languages"):
                    self.validate()
                matrix.insert(index, original)
        matrix[1]["build-mode"] = "none"
        with self.assertRaisesRegex(ValueError, "Go must use manual"):
            self.validate()

    def test_all_pr_bases_and_critical_push_branches_retained(self):
        original = copy.deepcopy(self.caller["on"])
        for event in ["push", "pull_request"]:
            self.caller["on"][event]["branches"] = ["feature/gcp-terraform-iac"]
            with self.assertRaisesRegex(ValueError, "coverage|Every PR"):
                self.validate()
            self.caller["on"] = copy.deepcopy(original)
            self.caller["on"][event]["paths"] = ["infra/**"]
            with self.assertRaisesRegex(ValueError, "path filters"):
                self.validate()
            self.caller["on"] = copy.deepcopy(original)

    def test_cloud_auth_or_application_step_rejected(self):
        self.callee["jobs"]["analyze"]["steps"].insert(0, {"run": "go run ./cmd/youtube-bot"})
        with self.assertRaisesRegex(ValueError, "no cloud auth or application"):
            self.validate()

    def test_static_ci_cannot_request_an_environment(self):
        self.static["jobs"]["source-contracts"]["environment"] = "codeql-analysis"
        with self.assertRaisesRegex(ValueError, "Static CI"):
            self.validate()

    def test_static_ci_routes_entire_source_directory(self):
        self.static["on"]["pull_request"]["paths"] = [".github/codeql/templates/**"]
        with self.assertRaisesRegex(ValueError, "routed"):
            self.validate()

    def test_implicit_environment_creation_or_live_analyzer_rejected(self):
        for active in [self.callee, {"jobs": {"run": {"uses": "./.github/workflows/codeql-analyzer.yml"}}}]:
            with self.subTest(active=active.get("name")), self.assertRaisesRegex(ValueError, "No live"):
                self.validate([active])


class CompileOnlyFixtures(unittest.TestCase):
    def test_every_tracked_module_compiles_without_running_applications(self):
        with tempfile.TemporaryDirectory(prefix="codeql-build-fixture-") as directory:
            root = Path(directory)
            subprocess.run(["git", "init", "-q", str(root)], check=True)
            for name in ["go.mod", "system/go.mod", "tools/fixture module/go.mod"]:
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("module example.invalid/synthetic\n")
                subprocess.run(["git", "-C", str(root), "add", name], check=True)
            (root / "untracked").mkdir()
            (root / "untracked/go.mod").write_text("module example.invalid/untracked\n")
            bin_dir = root / "bin"
            bin_dir.mkdir()
            # A command recorder, not a Go compiler. It cannot start a Bot or services.
            fake = bin_dir / "go"
            fake.write_text('#!/bin/bash\nprintf "%s|%s|%s|%s\\n" "$PWD" "$*" "$GOWORK" "$GOFLAGS" >> "$BUILD_FIXTURE_LOG"\n')
            fake.chmod(0o700)
            log = root / "commands.txt"
            env = {**os.environ, "PATH": str(bin_dir) + os.pathsep + os.environ["PATH"], "BUILD_FIXTURE_LOG": str(log)}
            subprocess.run(["bash", str(HERE / "build-go.sh")], cwd=root, env=env, check=True)
            expected = {f"{root / name}|build -a ./...|off|-mod=readonly" for name in [".", "system", "tools/fixture module"]}
            self.assertEqual(set(log.read_text().splitlines()), expected)

    def test_missing_go_modules_fails_closed(self):
        with tempfile.TemporaryDirectory(prefix="codeql-build-empty-") as directory:
            subprocess.run(["git", "init", "-q", directory], check=True)
            result = subprocess.run(["bash", str(HERE / "build-go.sh")], cwd=directory, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("No tracked Go module", result.stderr)


if __name__ == "__main__":
    unittest.main()
