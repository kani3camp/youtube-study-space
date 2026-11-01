# GitHub Copilot Instructions for YouTube Study Space

Read and follow [`../AGENTS.md`](../AGENTS.md) first. It is the canonical repository-wide contract for language, authorization, security, branching/PR policy, validation, and task-specific references.

This file contains only GitHub Copilot-specific review guidance.

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

## Review focus

- Review comments and summaries are in Japanese unless requested otherwise.
- Prioritize correctness, security, data integrity, regressions, and repository contracts over style trivia.
- For Go changes, pay particular attention to Firestore transaction boundaries and error identity/context.
- Preserve meaningful `NOTE` comments. `[NOTE FOR REVIEW]` is temporary and should not become permanent clutter.
- For seat semantics, privacy/data-retention changes, Firestore security/access, and monitor layout/seat-count changes, follow the task-specific reference table in `AGENTS.md`.
- A PR based on `dev` may be reviewed and updated, but must not be merged by an agent.
