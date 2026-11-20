#!/usr/bin/env python3
"""Check repository-relative Markdown links and reject ephemeral notes."""

from __future__ import annotations

import os
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[2]
TARGETS = (
    "AGENTS.md",
    ".github/copilot-instructions.md",
    "system/AI_COLLABORATION_GUIDE.md",
    "youtube-monitor/README.md",
    "system/README.md",
    "aws-cdk/README.md",
    "firebase/README.md",
    "infra/gcp/README.md",
    "infra/gcp/docs/terraform-authentication-boundary.md",
    "infra/gcp/modules/monitor-auth/README.md",
    "docs-site/README.md",
    "docs/development/architecture.md",
    "docs/design/DESIGN.md",
)
GOVERNED_DOC_DIRS = ("docs", "infra/gcp")

LINK_RE = re.compile(r"(?<!!)\[[^\]]*\]\(([^)]+)\)")
SKIP_PREFIXES = ("http://", "https://", "mailto:", "tel:", "#")
EPHEMERAL_DOC_RE = re.compile(r"このPR|本PR")
REVIEW_ONLY_MARKER_RE = re.compile(r"\[NOTE" + r" FOR REVIEW\]", re.IGNORECASE)

SCANNED_EXTENSIONS = frozenset(
    {
        ".md",
        ".mdc",
        ".go",
        ".ts",
        ".tsx",
        ".js",
        ".jsx",
        ".mjs",
        ".cjs",
        ".py",
        ".sh",
        ".tf",
        ".tfvars",
        ".yml",
        ".yaml",
        ".json",
        ".toml",
    }
)
SKIP_DIR_NAMES = frozenset(
    {
        ".git",
        ".next",
        ".pnpm-store",
        ".terraform",
        ".worktrees",
        ".claude",
        "build",
        "cdk.out",
        "dist",
        "node_modules",
        "plans",
    }
)


def iter_local_links(text: str):
    for match in LINK_RE.finditer(text):
        raw = match.group(1).strip()
        # Optional Markdown title: (path "title"). Governed docs do not use
        # spaces in repository paths, so the first token is the destination.
        destination = raw.split()[0].strip("<>")
        if not destination or destination.startswith(SKIP_PREFIXES):
            continue
        destination = unquote(destination.split("#", 1)[0].split("?", 1)[0])
        if destination:
            yield destination


def list_git_tracked_files(root: Path) -> list[str] | None:
    if not (root / ".git").exists():
        return None
    result = subprocess.run(
        ["git", "-C", str(root), "ls-files", "-z"],
        capture_output=True,
        check=False,
    )
    if result.returncode != 0:
        return None
    return [
        entry.decode("utf-8", errors="surrogateescape")
        for entry in result.stdout.split(b"\0")
        if entry
    ]


def collect_governed_docs(root: Path, tracked_files: list[str] | None = None) -> list[str]:
    targets = set(TARGETS)
    if tracked_files is not None:
        prefixes = tuple(f"{rel_dir}/" for rel_dir in GOVERNED_DOC_DIRS)
        for relative in tracked_files:
            if not relative.endswith(".md") or not relative.startswith(prefixes):
                continue
            if any(part in SKIP_DIR_NAMES for part in Path(relative).parts):
                continue
            if (root / relative).is_file():
                targets.add(relative)
        return sorted(targets)

    for rel_dir in GOVERNED_DOC_DIRS:
        base = root / rel_dir
        if not base.is_dir():
            continue
        for path in base.rglob("*.md"):
            if any(part in SKIP_DIR_NAMES for part in path.relative_to(root).parts):
                continue
            targets.add(path.relative_to(root).as_posix())
    return sorted(targets)


def iter_scanned_files(root: Path, tracked_files: list[str] | None = None):
    if tracked_files is not None:
        for relative in sorted(tracked_files):
            path = root / relative
            if any(part in SKIP_DIR_NAMES for part in Path(relative).parts):
                continue
            if path.suffix.lower() in SCANNED_EXTENSIONS and path.is_file():
                yield path
        return

    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = sorted(d for d in dirnames if d not in SKIP_DIR_NAMES)
        for filename in sorted(filenames):
            path = Path(dirpath) / filename
            if path.suffix.lower() in SCANNED_EXTENSIONS:
                yield path


def find_line_matches(text: str, pattern: re.Pattern[str]):
    for lineno, line in enumerate(text.splitlines(), start=1):
        match = pattern.search(line)
        if match:
            yield lineno, match.group(0)


def main() -> int:
    errors: list[str] = []
    tracked_files = list_git_tracked_files(ROOT)
    governed_docs = collect_governed_docs(ROOT, tracked_files)

    for relative in governed_docs:
        source = ROOT / relative
        if not source.exists():
            errors.append(f"missing governed document: {relative}")
            continue
        text = source.read_text(encoding="utf-8")
        for destination in iter_local_links(text):
            if destination.startswith("/"):
                target = ROOT / destination.lstrip("/")
            else:
                target = source.parent / destination
            if not target.exists():
                errors.append(f"{relative}: broken relative link -> {destination}")
        for lineno, matched in find_line_matches(text, EPHEMERAL_DOC_RE):
            errors.append(
                f"{relative}:{lineno}: ephemeral PR-scoped phrasing is not allowed in governed docs -> {matched}"
            )

    for path in iter_scanned_files(ROOT, tracked_files):
        relative = path.relative_to(ROOT).as_posix()
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        for lineno, _ in find_line_matches(text, REVIEW_ONLY_MARKER_RE):
            errors.append(
                f"{relative}:{lineno}: temporary review-only marker is not allowed in tracked files"
            )

    if errors:
        print("Documentation reference check failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    print(f"Documentation reference check passed for {len(governed_docs)} documents.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
