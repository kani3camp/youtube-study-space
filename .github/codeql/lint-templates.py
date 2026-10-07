"""Lint inert templates as local workflows in a disposable directory; never run them."""
import argparse
from pathlib import Path
import shutil
import subprocess
import tempfile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--actionlint", required=True, help="Verified actionlint binary path")
    args = parser.parse_args()
    binary = str(Path(args.actionlint).resolve(strict=True))
    templates = Path(__file__).resolve().parent / "templates"
    with tempfile.TemporaryDirectory(prefix="codeql-template-lint-") as directory:
        root = Path(directory)
        workflows = root / ".github/workflows"
        workflows.mkdir(parents=True)
        for template in sorted(templates.glob("*.yml.template")):
            shutil.copyfile(template, workflows / template.name.removesuffix(".template"))
        subprocess.run(["git", "init", "-q", str(root)], check=True)
        # The contract check requires the intentional constant-false safety barrier.
        # Waive only that exact diagnostic, not the general if-cond rule.
        disabled_gate = r'^constant expression "false" in condition\. remove the if: section$'
        subprocess.run([binary, "-shellcheck=", "-pyflakes=", "-ignore", disabled_gate], cwd=root, check=True)
    print("Inert templates: actionlint PASS (only constant-false gate waived; shellcheck/pyflakes excluded)")


if __name__ == "__main__":
    main()
