"""Lint actual staged-disabled workflows and their local call; never execute them."""
import argparse
from pathlib import Path
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--actionlint", required=True, help="Verified actionlint binary path")
    args = parser.parse_args()
    binary = str(Path(args.actionlint).resolve(strict=True))
    source = Path(__file__).resolve().parent
    root = source.parents[1]
    # Check both gates before waiving their exact constant-false diagnostic.
    subprocess.run([sys.executable, str(source / "validate-source.py")], cwd=root, check=True)
    disabled_gate = r'^constant expression "false" in condition\. remove the if: section$'
    paths = [str(root / ".github/workflows" / name) for name in
             ["codeql-advanced.yml", "codeql-analyzer.yml", "codeql-source-contracts.yml"]]
    subprocess.run([binary, "-shellcheck=", "-pyflakes=", "-ignore", disabled_gate, *paths], cwd=root, check=True)
    print("Staged workflows: actionlint PASS (only constant-false gates waived; shellcheck/pyflakes excluded)")


if __name__ == "__main__":
    main()
