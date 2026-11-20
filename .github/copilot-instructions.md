# GitHub Copilot Instructions for YouTube Study Space

Read and follow [`../AGENTS.md`](../AGENTS.md) first. It is the canonical repository-wide contract for language, authorization, security, branching/PR policy, documentation/comment lifecycle, validation, task-specific references, and pull request review standards (`Pull Request Reviews`).

This file contains only GitHub Copilot-specific review formatting guidance.

## Pull request review format

Use [Conventional Comments](https://conventionalcomments.org/) for actionable review comments:

- `issue:` for a defect; add `(blocking)` when it must be fixed before merge.
- `suggestion:` for a concrete improvement and brief rationale.
- `question:` for a material uncertainty.
- `todo:` for a small required fix.
- `chore:` for a required pre-merge procedure/check.
- `note:` for relevant non-blocking information.
- Avoid praise-only comments and low-value nitpicks.

Examples:

- `issue (blocking): トランザクション内でエラーハンドリングが抜けています。`
- `suggestion: このエラーメッセージにコンテキストを足すとデバッグしやすくなります。`
- `question: この分岐で〇〇のケースは考慮済みでしょうか。`
