#!/usr/bin/env python3
"""Check repository-relative Markdown links in the contributor/developer docs."""

from __future__ import annotations

import re
import sys
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[2]
TARGETS = (
    "AGENTS.md",
    "CLAUDE.md",
    ".github/copilot-instructions.md",
    "system/AI_COLLABORATION_GUIDE.md",
    "youtube-monitor/README.md",
    "system/README.md",
    "firebase/README.md",
    "docs-site/README.md",
    "docs/development/architecture.md",
    "docs/design/DESIGN.md",
)

LINK_RE = re.compile(r"(?<!!)\[[^\]]*\]\(([^)]+)\)")
SKIP_PREFIXES = ("http://", "https://", "mailto:", "tel:", "#")


def iter_local_links(path: Path):
    text = path.read_text(encoding="utf-8")
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


def main() -> int:
    errors: list[str] = []
    for relative in TARGETS:
        source = ROOT / relative
        if not source.exists():
            errors.append(f"missing governed document: {relative}")
            continue
        for destination in iter_local_links(source):
            if destination.startswith("/"):
                target = ROOT / destination.lstrip("/")
            else:
                target = source.parent / destination
            if not target.exists():
                errors.append(f"{relative}: broken relative link -> {destination}")

    if errors:
        print("Documentation reference check failed:", file=sys.stderr)
        for error in errors:
            print(f"- {error}", file=sys.stderr)
        return 1

    print(f"Documentation reference check passed for {len(TARGETS)} documents.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
