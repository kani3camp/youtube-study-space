# CLAUDE.md

Claude Code must read and follow [`AGENTS.md`](./AGENTS.md) before doing repository work. `AGENTS.md` is the canonical repository-wide operating contract for authorization, branching/PR policy, validation, security, and task-specific references.

This file intentionally contains only Claude-specific guidance so shared rules do not drift.

## Claude-specific guidance

- Use Japanese for user-facing communication unless asked otherwise.
- Resolve implementation questions from the closest code, tests, package README, and the task-specific references listed in `AGENTS.md`.
- Do not copy tool/runtime versions into this file. Read the canonical config (`package.json`, `go.mod`, workflow, etc.) when a version matters.
- Before running a command whose effects are unclear, inspect the corresponding script/config first.
- Treat `go run ./cmd/youtube-bot` as an external-service/real-state operation, not as a routine backend verification command.
- For `youtube-monitor`, read [`youtube-monitor/README.md`](./youtube-monitor/README.md) before local build/run work because required `NEXT_PUBLIC_*` variables are validated at module load/build time.
- For seat-count or room-layout changes, also read [`docs/development/architecture.md`](./docs/development/architecture.md). The monitor currently participates in max-seat control.
