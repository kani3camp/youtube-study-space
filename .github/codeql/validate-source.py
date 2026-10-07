"""Source-preparation contracts: this check does not run CodeQL or read settings."""
import copy
from pathlib import Path
import re

import yaml

ROOT = Path(__file__).resolve().parents[2]
SOURCE = ROOT / ".github/codeql"
PUSH_BRANCHES = ["dev", "main", "feature/gcp-terraform-iac", "integration/mypage-canon-20261006"]
ANALYSIS_PERMISSIONS = {"contents": "read", "security-events": "write", "id-token": "none"}
STATIC_PERMISSIONS = {"contents": "read", "security-events": "none", "id-token": "none"}
GO_ONLY = "${{ matrix.language == 'go' }}"
STATIC_COMMANDS = {
    "python -m pip install --disable-pip-version-check --only-binary=:all: -r .github/codeql/requirements.txt",
    "python .github/codeql/validate-source.py && python -m unittest discover -s .github/codeql -p 'test_*.py' -v",
    'bash .github/codeql/install-actionlint.sh "$RUNNER_TEMP/codeql-source-tools"',
    'python .github/codeql/lint-templates.py --actionlint "$RUNNER_TEMP/codeql-source-tools/actionlint"',
    '"$RUNNER_TEMP/codeql-source-tools/actionlint" -shellcheck= -pyflakes= .github/workflows/codeql-source-contracts.yml',
    "bash -n .github/codeql/build-go.sh .github/codeql/install-actionlint.sh",
    "bash .github/scripts/test-detect-ci-paths.sh",
}


class WorkflowLoader(yaml.SafeLoader):
    pass


# GitHub uses YAML 1.2; PyYAML must not turn the event key `on` into True.
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


def restricted(document, permissions):
    for key, value in walk(document):
        require(key != "secrets", "No declared/passed/inherited secrets")
        if key == "permissions":
            require(value == permissions, "Exact bounded permissions required")
        if isinstance(value, str):
            require(not re.search(r"\b(secrets|vars|inputs)\s*\.", value), "No secret/variable/input context")
        if key == "uses" and not value.startswith("./"):
            require(re.fullmatch(r"[\w-]+/[\w-]+(?:/[\w-]+)?@[0-9a-f]{40}", value), "Pin every action to a commit")
    require(document.get("permissions") == permissions, "Workflow permissions required")


def validate(caller, callee, static, active):
    restricted(caller, ANALYSIS_PERMISSIONS)
    restricted(callee, ANALYSIS_PERMISSIONS)
    restricted(static, STATIC_PERMISSIONS)
    require(set(caller["on"]) == {"push", "pull_request", "schedule", "workflow_dispatch"}, "Caller event coverage")
    require(caller["on"]["push"] == {"branches": PUSH_BRANCHES}, "Default/protected/MyPage push coverage; no path filters")
    require(caller["on"]["pull_request"] == {"branches": ["**"]}, "Every PR base covered; no path filters")
    require(len(caller["on"]["schedule"]) == 1, "Default-branch scheduled coverage")
    require(set(caller["jobs"]) == {"analyze"}, "Caller has only reusable analysis job")
    job = caller["jobs"]["analyze"]
    require(job.get("if") == "${{ false }}", "Caller must remain disabled during source preparation")
    require(job.get("uses") == "./.github/workflows/codeql-analyzer.yml", "Local reusable analyzer required")
    require(job.get("permissions") == ANALYSIS_PERMISSIONS, "Caller job permissions required")
    require(set(job) == {"name", "if", "uses", "permissions"}, "No caller inputs/secrets/runner/environment")
    require(callee["on"] == {"workflow_call": None}, "Callee has no direct trigger or configurable inputs")
    require(set(callee["jobs"]) == {"analyze"}, "Single analyzer job")
    job = callee["jobs"]["analyze"]
    require(job.get("environment") == "codeql-analysis", "Fixed dedicated Environment required")
    require(job.get("permissions") == ANALYSIS_PERMISSIONS, "Callee job permissions required")
    require("if" not in job and job.get("runs-on") == "ubuntu-latest", "All analyzers use GitHub-hosted Linux")
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
    require(checkout.get("with") == {"persist-credentials": False}, "No checkout credentials retained")
    require(setup.get("if") == build.get("if") == GO_ONLY, "Only Go compiles between init and analyze")
    require(setup.get("with") == {"go-version-file": "system/go.mod", "cache": False}, "Go version comes from go.mod; no cache")
    require(init.get("with") == {"languages": "${{ matrix.language }}", "build-mode": "${{ matrix.build-mode }}",
                                "dependency-caching": False, "debug": False}, "Init uses complete matrix; no debug/dependency cache")
    require(analyze.get("with") == {"category": "/language:${{ matrix.language }}", "upload-database": False,
                                   "wait-for-processing": True}, "Language categories, processing wait, no database artifacts")
    require(all("if" not in step for step in [checkout, init, analyze]), "No language can silently skip analysis")
    require(set(static["on"]) == {"push", "pull_request"}, "Static-only CI events")
    for event in ["push", "pull_request"]:
        require(static["on"][event]["paths"] == [".github/codeql/**", ".github/workflows/codeql-source-contracts.yml"],
                "All source files routed to deterministic static checks")
    for key, value in walk(static):
        require(key != "environment", "Static CI must not reference/create an Environment")
        if key == "uses":
            require(value.split("@")[0] in {"actions/checkout", "actions/setup-python"}, "Static CI must not call an analyzer")
        if key == "run":
            require(all(line in STATIC_COMMANDS for line in value.strip().splitlines()), "Static CI can run only reviewed static commands")
    for document in active:
        for key, value in walk(document):
            if key == "uses":
                require(not value.startswith("github/codeql-action/") and value != "./.github/workflows/codeql-analyzer.yml",
                        "No live CodeQL workflow during source preparation")
            if key == "environment":
                name = value.get("name") if isinstance(value, dict) else value
                require(name != "codeql-analysis", "No live analysis Environment during source preparation")


def main():
    validate(load_yaml(SOURCE / "templates/codeql-advanced.yml.template"),
             load_yaml(SOURCE / "templates/codeql-analyzer.yml.template"),
             load_yaml(ROOT / ".github/workflows/codeql-source-contracts.yml"),
             [load_yaml(path) for path in sorted((ROOT / ".github/workflows").glob("*.y*ml"))])
    print("Source-only contracts PASS; no CodeQL run or settings read performed")


if __name__ == "__main__":
    main()
