"""Lint actual CodeQL workflows and their local call; never execute them."""
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
    # Validate unconditional coverage and bounded permissions before linting.
    subprocess.run([sys.executable, str(source / "validate-source.py")], cwd=root, check=True)
    paths = [str(root / ".github/workflows" / name) for name in
             ["codeql-advanced.yml", "codeql-analyzer.yml", "codeql-source-contracts.yml"]]
    subprocess.run([binary, "-shellcheck=", "-pyflakes=", *paths], cwd=root, check=True)
    print("CodeQL workflows: actionlint PASS (no diagnostic waivers; shellcheck/pyflakes excluded)")


if __name__ == "__main__":
    main()
