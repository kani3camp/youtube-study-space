"""Negative security/coverage contracts and compile-only synthetic module fixtures."""
import copy
import importlib.util
import os
from pathlib import Path
import subprocess
import shutil
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("source_contracts", HERE / "validate-source.py")
contract = importlib.util.module_from_spec(spec)
spec.loader.exec_module(contract)


class SourceContracts(unittest.TestCase):
    def setUp(self):
        self.caller = contract.load_yaml(contract.WORKFLOWS / contract.CALLER)
        self.callee = contract.load_yaml(contract.WORKFLOWS / contract.CALLEE)
        self.static = contract.load_yaml(contract.ROOT / ".github/workflows/codeql-source-contracts.yml")
        self.config = contract.load_yaml(HERE / "analysis-config.yml")

    def validate(self, active=None):
        workflows = {contract.CALLER: self.caller, contract.CALLEE: self.callee, contract.STATIC: self.static}
        if active is not None:
            workflows.update(active)
        contract.validate(self.caller, self.callee, self.static, workflows, self.config)

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
                with self.assertRaisesRegex(ValueError, "static checks"):
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

    def test_either_gate_cannot_be_removed_or_enabled_by_any_event(self):
        for document in [self.caller, self.callee]:
            job = document["jobs"]["analyze"]
            for condition in [None, True, "${{ true }}", "${{ always() }}"] + [
                "${{ github.event_name == '" + event + "' }}" for event in
                ["workflow_dispatch", "push", "pull_request", "schedule", "workflow_call"]
            ]:
                with self.subTest(workflow=document["name"], condition=condition):
                    if condition is None:
                        del job["if"]
                    else:
                        job["if"] = condition
                    with self.assertRaisesRegex(ValueError, "explicitly disabled"):
                        self.validate()
                    job["if"] = contract.DISABLED

    def test_extra_caller_or_callee_job_rejected_even_if_disabled(self):
        for document in [self.caller, self.callee]:
            document["jobs"]["alternate"] = copy.deepcopy(document["jobs"]["analyze"])
            with self.assertRaisesRegex(ValueError, "only reusable|Single analyzer"):
                self.validate()
            del document["jobs"]["alternate"]

    def test_workflow_wide_analysis_permissions_rejected(self):
        for document in [self.caller, self.callee]:
            document["permissions"] = contract.ANALYSIS_PERMISSIONS
            with self.assertRaisesRegex(ValueError, "workflow permissions"):
                self.validate()
            document["permissions"] = {}

    def test_dispatch_or_reusable_activation_inputs_rejected(self):
        self.caller["on"]["workflow_dispatch"] = {"inputs": {"enable": {"type": "boolean"}}}
        with self.assertRaisesRegex(ValueError, "activation switch"):
            self.validate()
        self.caller["on"]["workflow_dispatch"] = None
        self.callee["on"]["workflow_call"] = {"inputs": {"enable": {"type": "boolean"}}}
        with self.assertRaisesRegex(ValueError, "configurable inputs"):
            self.validate()

    def test_default_remote_scope_cannot_be_expanded_or_filtered(self):
        for key, value in [("threat-models", ["remote", "local"]), ("queries", [{"uses": "security-extended"}]),
                           ("disable-default-queries", True), ("paths-ignore", ["tools/**"]),
                           ("packs", ["codeql/go-queries"])]:
            with self.subTest(key=key):
                original = copy.deepcopy(self.config)
                self.config[key] = value
                with self.assertRaisesRegex(ValueError, "default queries"):
                    self.validate()
                self.config = original

    def test_init_cannot_override_config_or_add_queries(self):
        settings = self.callee["jobs"]["analyze"]["steps"][2]["with"]
        for key, value in [("queries", "security-extended"), ("config", "threat-models: local"),
                           ("config-file", "https://example.invalid/config.yml")]:
            original = copy.deepcopy(settings)
            settings[key] = value
            with self.assertRaisesRegex(ValueError, "default/remote scope"):
                self.validate()
            settings.clear()
            settings.update(original)

    def test_analyzer_extra_env_or_step_command_rejected(self):
        job = self.callee["jobs"]["analyze"]
        job["env"] = {"CODEQL_ACTION_EXTRA_OPTIONS": "{}"}
        with self.assertRaisesRegex(ValueError, "extra analyzer"):
            self.validate()
        del job["env"]
        job["steps"][2]["run"] = "echo hidden"
        with self.assertRaisesRegex(ValueError, "hidden commands"):
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

    def test_alternate_live_analyzer_environment_or_helper_rejected(self):
        for job in [{"uses": "./.github/workflows/codeql-analyzer.yml"},
                    {"uses": "kani3camp/youtube-study-space/.github/workflows/codeql-analyzer.yml@dev"},
                    {"steps": [{"uses": "github/codeql-action/analyze@v4"}]},
                    {"environment": {"name": "CodeQL-Analysis"}},
                    {"steps": [{"run": "codeql database analyze db"}]},
                    {"steps": [{"run": "bash .github/codeql/build-go.sh"}]}]:
            with self.subTest(job=job), self.assertRaisesRegex(ValueError, "alternate live"):
                self.validate({"alternate.yml": {"jobs": {"run": job}}})

    def test_actual_workflows_must_exist_and_match_the_validated_documents(self):
        for name in [contract.CALLER, contract.CALLEE, contract.STATIC]:
            workflows = {contract.CALLER: self.caller, contract.CALLEE: self.callee, contract.STATIC: self.static}
            del workflows[name]
            with self.assertRaisesRegex(ValueError, "same tree"):
                contract.validate(self.caller, self.callee, self.static, workflows, self.config)
        with self.assertRaisesRegex(ValueError, "actual workflow"):
            self.validate({contract.CALLEE: {"jobs": {}}})

    def test_static_ci_cannot_skip_hide_or_remove_required_checks(self):
        job = self.static["jobs"]["source-contracts"]
        for target in [job, job["steps"][3]]:
            for key, value in [("if", contract.DISABLED), ("continue-on-error", True), ("env", {"BASH_ENV": "override.sh"})]:
                target[key] = value
                with self.assertRaisesRegex(ValueError, "bypass"):
                    self.validate()
                del target[key]
        original = job["steps"][3]["run"]
        job["steps"][3]["run"] = "true"
        with self.assertRaisesRegex(ValueError, "all reviewed static commands"):
            self.validate()
        job["steps"][3]["run"] = original

    def test_duplicate_yaml_keys_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "duplicate.yml"
            path.write_text("on: push\njobs:\n  analyze:\n    if: '${{ false }}'\n    if: '${{ true }}'\n")
            with self.assertRaisesRegex(ValueError, "Duplicate YAML"):
                contract.load_yaml(path)


class SameTreeContracts(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory(prefix="codeql-tree-fixture-")
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        tracked = contract.tracked_files(contract.ROOT)
        for name in tracked:
            if name.startswith(".github/codeql/") or name.startswith(".github/workflows/codeql-") or name.endswith(("go.mod", "go.sum")):
                source = contract.ROOT / name
                if source.is_file():
                    target = self.root / name
                    target.parent.mkdir(parents=True, exist_ok=True)
                    shutil.copyfile(source, target)
        subprocess.run(["git", "init", "-q", str(self.root)], check=True)
        subprocess.run(["git", "add", "."], cwd=self.root, check=True)

    def test_same_tree_helper_config_parser_and_module_dependencies_are_complete(self):
        self.assertEqual(contract.validate_tree(self.root), ["system/go.mod", "tools/room-image-prompt/go.mod"])

    def test_required_dependencies_cannot_be_missing_or_untracked(self):
        for name in [".github/codeql/build-go.sh", ".github/codeql/analysis-config.yml", ".github/codeql/requirements.txt",
                     ".github/workflows/codeql-analyzer.yml", "system/go.sum", "tools/room-image-prompt/go.sum"]:
            path = self.root / name
            original = path.read_bytes()
            path.unlink()
            with self.subTest(missing=name), self.assertRaisesRegex(ValueError, "missing"):
                contract.validate_tree(self.root)
            path.write_bytes(original)
        subprocess.run(["git", "rm", "--cached", "-q", ".github/codeql/build-go.sh"], cwd=self.root, check=True)
        with self.assertRaisesRegex(ValueError, "tracked dependency"):
            contract.validate_tree(self.root)

    def test_unreviewed_go_helper_commands_rejected_without_executing_them(self):
        path = self.root / ".github/codeql/build-go.sh"
        path.write_text(path.read_text() + "curl https://example.invalid\n")
        with self.assertRaisesRegex(ValueError, "Compile-only helper changed"):
            contract.validate_tree(self.root)

    def test_requirements_cannot_install_extra_commands(self):
        (self.root / ".github/codeql/requirements.txt").write_text("PyYAML==6.0.3\nexample-package==1.0.0\n")
        with self.assertRaisesRegex(ValueError, "pinned YAML parser"):
            contract.validate_tree(self.root)

    def test_manifest_and_checksum_mismatch_rejected(self):
        path = self.root / "tools/room-image-prompt/go.mod"
        path.write_text(path.read_text().replace("v0.1.4", "v9.9.9"))
        with self.assertRaisesRegex(ValueError, "Dependency checksum missing"):
            contract.validate_tree(self.root)

    def test_new_module_requires_sums_and_supported_canonical_toolchain(self):
        path = self.root / "tools/new-module/go.mod"
        path.parent.mkdir(parents=True)
        path.write_text("module example.invalid/new\n\ngo 1.99.0\n")
        subprocess.run(["git", "add", str(path.relative_to(self.root))], cwd=self.root, check=True)
        with self.assertRaisesRegex(ValueError, "go.sum missing"):
            contract.validate_tree(self.root)
        path.with_suffix(".sum").write_text("")
        subprocess.run(["git", "add", "."], cwd=self.root, check=True)
        with self.assertRaisesRegex(ValueError, "cover every tracked module"):
            contract.validate_tree(self.root)


class CompileOnlyFixtures(unittest.TestCase):
    def test_every_tracked_module_compiles_without_running_applications(self):
        with tempfile.TemporaryDirectory(prefix="codeql-build-fixture-") as directory:
            root = Path(directory).resolve()
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
