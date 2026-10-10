"""Validate staged-disabled CodeQL source; never execute analysis or read settings."""
import copy
import hashlib
import json
from pathlib import Path
import re
import subprocess

import yaml

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / ".github/codeql"
WORKFLOWS = ROOT / ".github/workflows"
CALLER = "codeql-advanced.yml"
CALLEE = "codeql-analyzer.yml"
STATIC = "codeql-source-contracts.yml"
DISABLED = "${{ false }}"
PUSH_BRANCHES = ["dev", "main", "feature/gcp-terraform-iac", "integration/mypage-canon-20261006"]
ANALYSIS_PERMISSIONS = {"contents": "read", "security-events": "write", "id-token": "none"}
STATIC_PERMISSIONS = {"contents": "read", "security-events": "none", "id-token": "none"}
ANALYSIS_CONFIG = {"name": "Default queries, remote sources", "disable-default-queries": False,
                   "threat-models": "remote"}
STATIC_PATHS = [".github/codeql/**", ".github/workflows/**",
                ".github/scripts/detect-ci-paths.sh", ".github/scripts/test-detect-ci-paths.sh",
                "go.mod", "go.sum", "**/go.mod", "**/go.sum"]
STATIC_PUSH_BRANCHES = [*PUSH_BRANCHES, "feature/codeql-advanced-setup"]
GO_ONLY = "${{ matrix.language == 'go' }}"
STATIC_RUNS = [
    "python -m pip install --disable-pip-version-check --only-binary=:all: -r .github/codeql/requirements.txt",
    "python .github/codeql/validate-source.py && python -m unittest discover -s .github/codeql -p 'test_*.py' -v",
    'bash .github/codeql/install-actionlint.sh "$RUNNER_TEMP/codeql-source-tools"',
    'python .github/codeql/lint-workflows.py --actionlint "$RUNNER_TEMP/codeql-source-tools/actionlint"',
    "bash -n .github/codeql/build-go.sh .github/codeql/install-actionlint.sh\nbash .github/scripts/test-detect-ci-paths.sh",
]


class WorkflowLoader(yaml.SafeLoader):
    def construct_mapping(self, node, deep=False):
        self.flatten_mapping(node)
        keys = [self.construct_object(key, deep=deep) for key, _ in node.value]
        require(len(keys) == len(set(keys)), "Duplicate YAML keys are not allowed")
        return super().construct_mapping(node, deep=deep)


# GitHub uses YAML 1.2; keep `on` a string and true/false actual booleans.
WorkflowLoader.yaml_implicit_resolvers = copy.deepcopy(yaml.SafeLoader.yaml_implicit_resolvers)
for char, resolvers in WorkflowLoader.yaml_implicit_resolvers.items():
    WorkflowLoader.yaml_implicit_resolvers[char] = [
        (tag, pattern) for tag, pattern in resolvers if tag != "tag:yaml.org,2002:bool"
    ]
WorkflowLoader.add_implicit_resolver("tag:yaml.org,2002:bool", re.compile(r"^(?:true|false)$"), ["t", "f"])


def load_yaml(path):
    return yaml.load(Path(path).read_text(), Loader=WorkflowLoader)


def require(condition, message):
    if not condition:
        raise ValueError(message)


def walk(value):
    if isinstance(value, dict):
        for key, child in value.items():
            yield key, child
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def restricted(document, workflow_permissions, job_permissions):
    require(document.get("permissions") == workflow_permissions, "Exact workflow permissions required")
    for key, value in walk(document):
        require(key != "secrets", "No declared/passed/inherited secrets")
        if key == "permissions":
            require(value in [workflow_permissions, job_permissions], "Exact bounded permissions required")
        if isinstance(value, str):
            require(not re.search(r"\b(secrets|vars|inputs)\s*(?:\.|\[)", value), "No secret/variable/input context")
        if key == "uses" and not value.startswith("./"):
            require(re.fullmatch(r"[\w-]+/[\w-]+(?:/[\w-]+)?@[0-9a-f]{40}", value), "Pin every action to a commit")


def validate(caller, callee, static, active, config):
    restricted(caller, {}, ANALYSIS_PERMISSIONS)
    restricted(callee, {}, ANALYSIS_PERMISSIONS)
    restricted(static, STATIC_PERMISSIONS, STATIC_PERMISSIONS)
    require(config == ANALYSIS_CONFIG, "Only default queries and the remote threat model; no scope filters/packs")
    require(set(caller) == {"name", "on", "permissions", "concurrency", "jobs"}, "No extra caller configuration")
    require(set(caller["on"]) == {"push", "pull_request", "schedule", "workflow_dispatch"}, "Caller event coverage")
    require(caller["on"]["push"] == {"branches": PUSH_BRANCHES}, "Default/protected/MyPage push coverage; no path filters")
    require(caller["on"]["pull_request"] == {"branches": ["**"]}, "Every PR base covered; no path filters")
    require(caller["on"]["schedule"] == [{"cron": "23 4 * * 1"}], "Fixed weekly schedule")
    require(caller["on"]["workflow_dispatch"] is None, "No dispatch inputs or activation switch")
    require(caller["concurrency"] == {"group": "codeql-${{ github.event.pull_request.number || github.ref }}",
                                     "cancel-in-progress": True}, "Reviewed caller concurrency")
    require(set(caller["jobs"]) == {"analyze"}, "Caller has only reusable analysis job")
    job = caller["jobs"]["analyze"]
    require(job.get("if") == DISABLED, "Caller must remain explicitly disabled")
    require(job.get("uses") == "./.github/workflows/" + CALLEE, "Same-tree local reusable analyzer required")
    require(job.get("permissions") == ANALYSIS_PERMISSIONS, "Caller job permissions required")
    require(set(job) == {"name", "if", "uses", "permissions"}, "No caller inputs/secrets/runner/environment")
    require(set(callee) == {"name", "on", "permissions", "jobs"}, "No extra callee configuration")
    require(callee["on"] == {"workflow_call": None}, "Callee has no direct trigger or configurable inputs")
    require(set(callee["jobs"]) == {"analyze"}, "Single analyzer job")
    job = callee["jobs"]["analyze"]
    require(job.get("if") == DISABLED, "Callee must remain explicitly disabled even under another caller")
    require(job.get("environment") == "codeql-analysis", "Fixed dedicated Environment required")
    require(job.get("permissions") == ANALYSIS_PERMISSIONS, "Callee job permissions required")
    require(set(job) == {"name", "if", "runs-on", "timeout-minutes", "environment", "permissions", "strategy", "steps"},
            "No extra analyzer jobs/services/env/configuration")
    require(job["runs-on"] == "ubuntu-latest" and job["timeout-minutes"] == 40, "Standard GitHub-hosted Linux runner")
    require(job["strategy"] == {"fail-fast": False, "matrix": {"include": [
        {"language": "actions", "build-mode": "none"},
        {"language": "go", "build-mode": "manual"},
        {"language": "javascript-typescript", "build-mode": "none"},
    ]}}, "All three languages required; Go must use manual build")
    steps = job["steps"]
    operations = [step["uses"].split("@")[0] if "uses" in step else "run:" + step.get("run", "") for step in steps]
    require(operations == ["actions/checkout", "actions/setup-go", "github/codeql-action/init",
                           "run:bash .github/codeql/build-go.sh", "github/codeql-action/analyze"],
            "Only checkout/toolchain/CodeQL/compile steps; no cloud auth or application execution")
    checkout, setup, init, build, analyze = steps
    for step, keys in zip(steps, [{"name", "uses", "with"}, {"name", "if", "uses", "with"},
                                  {"name", "uses", "with"}, {"name", "if", "run"}, {"name", "uses", "with"}]):
        require(set(step) == keys, "No extra analysis step settings or hidden commands")
    require(checkout["with"] == {"persist-credentials": False}, "No checkout credentials retained")
    require(setup["if"] == build["if"] == GO_ONLY, "Only Go compiles between init and analyze")
    require(setup["with"] == {"go-version-file": "system/go.mod", "cache": False}, "Go version comes from go.mod; no cache")
    require(init["with"] == {"languages": "${{ matrix.language }}", "build-mode": "${{ matrix.build-mode }}",
                             "dependency-caching": False, "debug": False,
                             "config-file": ".github/codeql/analysis-config.yml"}, "Init preserves default/remote scope; no debug/cache")
    require(analyze["with"] == {"category": "/language:${{ matrix.language }}", "upload-database": False,
                                "wait-for-processing": True}, "Language categories, processing wait, no database artifacts")
    require(set(static) == {"name", "on", "permissions", "jobs"}, "No extra Static CI configuration")
    require(static["on"] == {"pull_request": {"paths": STATIC_PATHS},
                             "push": {"branches": STATIC_PUSH_BRANCHES, "paths": STATIC_PATHS}},
            "All workflow/source/helper/dependency/routing files routed to static checks")
    require(set(static["jobs"]) == {"source-contracts"}, "Static CI has only the contract job")
    job = static["jobs"]["source-contracts"]
    require(set(job) == {"name", "runs-on", "timeout-minutes", "steps"}, "Static CI cannot bypass checks or request Environment")
    require(job["runs-on"] == "ubuntu-latest" and job["timeout-minutes"] == 10, "Reviewed Static CI runner")
    steps = job["steps"]
    require(len(steps) == 7, "All static checks must run")
    require([step.get("uses", "run") for step in steps][:2] == [
        "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1",
        "actions/setup-python@5fda3b95a4ea91299a34e894583c3862153e4b97"], "Static CI uses only reviewed checkout/Python actions")
    require(steps[0].get("with") == {"persist-credentials": False}, "Static checkout retains no credentials")
    require(steps[1].get("with") == {"python-version": "3.13"}, "Reviewed Static CI Python")
    require([step.get("run", "").strip() for step in steps[2:]] == STATIC_RUNS,
            "Static CI must run all reviewed static commands, in order")
    for index, step in enumerate(steps):
        require(set(step) == ({"name", "uses", "with"} if index < 2 else {"name", "run"}),
                "Static CI cannot bypass checks or add hidden commands")
    require(all(name in active for name in [CALLER, CALLEE, STATIC]), "All staged workflows must exist in the same tree")
    require(active[CALLER] == caller and active[CALLEE] == callee and active[STATIC] == static,
            "Validate the actual workflow files, not templates or substitutes")
    for name, document in active.items():
        if name in [CALLER, CALLEE, STATIC]:
            continue
        for key, value in walk(document):
            require(not isinstance(value, str) or "codeql" not in value.lower(),
                    "No alternate live CodeQL workflow, helper, caller, or Environment")


def tracked_files(root):
    output = subprocess.check_output(["git", "ls-files", "-z"], cwd=root)
    return {path.decode() for path in output.split(b"\0") if path}


def validate_tree(root):
    """Check dependencies in this checkout/index without Go, downloads, or services."""
    root = Path(root)
    tracked = tracked_files(root)
    needed = [".github/workflows/" + name for name in [CALLER, CALLEE, STATIC]] + [
        ".github/codeql/" + name for name in ["build-go.sh", "analysis-config.yml", "requirements.txt", "tooling.json",
                                             "validate-source.py", "test_source_contracts.py", "lint-workflows.py", "install-actionlint.sh"]]
    for name in needed:
        path = root / name
        require(name in tracked and path.is_file() and not path.is_symlink(), "Same-tree tracked dependency missing: " + name)
    tooling = json.loads((root / ".github/codeql/tooling.json").read_text())
    helper = (root / ".github/codeql/build-go.sh").read_bytes()
    require(hashlib.sha256(helper).hexdigest() == tooling["go_build_helper_sha256"], "Compile-only helper changed: review checksum and fixtures")
    requirements = (root / ".github/codeql/requirements.txt").read_text()
    require(re.fullmatch(r"PyYAML==\d+\.\d+\.\d+\n", requirements), "Only the pinned YAML parser dependency is allowed")
    manifests = sorted(name for name in tracked if name == "go.mod" or name.endswith("/go.mod"))
    require("system/go.mod" in manifests, "Canonical Go toolchain module missing")
    versions = {}
    for name in manifests:
        path = root / name
        require(path.is_file() and not path.is_symlink(), "Tracked Go manifest must exist in the same tree")
        text = path.read_text()
        version = re.search(r"^go (\d+)\.(\d+)(?:\.(\d+))?$", text, re.M)
        require(version is not None, "Go version directive missing: " + name)
        versions[name] = tuple(int(part or 0) for part in version.groups())
        sum_name = str(Path(name).with_suffix(".sum"))
        sums = root / sum_name
        require(sum_name in tracked and sums.is_file() and not sums.is_symlink(), "Same-tree tracked go.sum missing: " + name)
        sum_pairs = {tuple(line.split()[:2]) for line in sums.read_text().splitlines() if len(line.split()) == 3}
        in_requires = False
        for line in text.splitlines():
            clean = line.split("//", 1)[0].strip()
            if clean == "require (":
                in_requires = True
                continue
            if in_requires and clean == ")":
                in_requires = False
                continue
            parts = clean.removeprefix("require ").split() if in_requires or clean.startswith("require ") else []
            if len(parts) == 2:
                require(tuple(parts) in sum_pairs, "Dependency checksum missing for " + "@".join(parts) + " in " + sum_name)
    require(all(version <= versions["system/go.mod"] for version in versions.values()),
            "Canonical setup-go version must cover every tracked module")
    require(not list((root / ".github/codeql").glob("templates/*.template")), "No stale template alternative to the staged workflow")
    return manifests


def main():
    active = {path.name: load_yaml(path) for path in sorted(WORKFLOWS.glob("*")) if path.suffix in {".yml", ".yaml"}}
    validate(active[CALLER], active[CALLEE], active[STATIC], active, load_yaml(SOURCE / "analysis-config.yml"))
    modules = validate_tree(ROOT)
    print("Staged-disabled contracts PASS: caller + callee gates, default/remote scope, same-tree dependencies: " + ", ".join(modules))


if __name__ == "__main__":
    main()
