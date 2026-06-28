#!/usr/bin/env bash
set -euo pipefail

today="$(date -u +%Y-%m-%d)"
expiry="2026-10-28"

if [[ "$today" > "$expiry" || "$today" == "$expiry" ]]; then
  echo "::error::Node 26 is expected to become LTS on ${expiry}. Review .github/dependabot.yml and update/remove the @types/node >=25.0.0 ignore rule, or advance it to >=27.0.0."
  exit 1
fi
