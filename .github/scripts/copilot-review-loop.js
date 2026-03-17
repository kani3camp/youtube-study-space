const AI_LOOP_LABEL = "ai-loop";
const COMMENT_MARKER = "copilot-review-loop";
const MAX_ROUNDS = 10;
const WORKFLOW_NAME = "Copilot Review Loop";
const WORKFLOW_JOB_NAME = "Orchestrate Copilot-Cursor Loop";
const COPILOT_ACTOR_LOGINS = new Set(["copilot", "github-copilot[bot]"]);
const MAX_PROMPT_COMMENT_LENGTH = 60000;
const MAX_SECTION_ITEMS = 20;
const COMMENT_METADATA_RESERVE = 4000;

module.exports = async ({ github, context, core }) => {
  const pullNumber = getPullNumber(context);
  if (!pullNumber) {
    core.notice(`No pull request context for event ${context.eventName}.`);
    core.setOutput("action", "none");
    return;
  }

  const { owner, repo } = context.repo;
  const {
    data: pullRequest,
  } = await github.rest.pulls.get({
    owner,
    repo,
    pull_number: pullNumber,
  });

  const labels = pullRequest.labels.map((label) => label.name);
  if (!labels.includes(AI_LOOP_LABEL)) {
    core.notice(`PR #${pullNumber} does not have the ${AI_LOOP_LABEL} label.`);
    core.setOutput("action", "none");
    return;
  }

  if (pullRequest.head.repo.full_name !== `${owner}/${repo}`) {
    core.notice(`PR #${pullNumber} comes from a fork. Skipping for safety.`);
    core.setOutput("action", "none");
    return;
  }

  const serviceAccountLogin = await resolveServiceAccountLogin();
  const state = await loadLoopState({
    github,
    owner,
    repo,
    pullNumber,
    trustedLogin: serviceAccountLogin,
  });
  const headSha = pullRequest.head.sha;
  const {
    newComments: reviewComments,
    reviewIdsWithInlineComments,
  } = await collectCopilotReviewComments({
    github,
    owner,
    repo,
    pullNumber,
    state,
  });
  const reviewBodies = await collectCopilotReviewBodies({
    github,
    owner,
    repo,
    pullNumber,
    handledReviewIds: state.reviewIds,
    reviewIdsWithInlineComments,
  });
  const ciFailures = await collectCiFailures({
    github,
    owner,
    repo,
    headSha,
    state,
  });

  const hasActionableItems =
    reviewComments.length > 0 ||
    reviewBodies.length > 0 ||
    ciFailures.checkRuns.length > 0 ||
    ciFailures.statuses.length > 0;

  core.notice(
    `PR #${pullNumber}: ${reviewComments.length} new Copilot review comments, ` +
      `${reviewBodies.length} new Copilot reviews, ` +
      `${ciFailures.checkRuns.length + ciFailures.statuses.length} new CI failures.`,
  );

  if (!hasActionableItems) {
    core.notice("No new actionable Copilot comments or CI failures found.");
    core.setOutput("action", "none");
    return;
  }

  const nextRound = state.round + 1;
  if (nextRound > MAX_ROUNDS) {
    if (state.maxRoundStopped) {
      core.notice("The loop already stopped at the max round.");
      core.setOutput("action", "none");
      return;
    }

    const stopMetadata = {
      round: state.round,
      stopped: "max-round",
    };
    core.setOutput("action", "stop");
    core.setOutput(
      "comment_body",
      buildStopComment({
        maxRounds: MAX_ROUNDS,
        metadata: stopMetadata,
      }),
    );
    return;
  }

  core.setOutput("action", "comment");
  core.setOutput(
    "comment_body",
    buildPromptComment({
      round: nextRound,
      maxRounds: MAX_ROUNDS,
      headSha,
      reviewComments,
      reviewBodies,
      ciFailures,
    }),
  );
};

async function resolveServiceAccountLogin() {
  const configuredLogin = normalizeLogin(process.env.CURSOR_TRIGGER_LOGIN);
  if (configuredLogin) {
    return configuredLogin;
  }

  throw new Error(
    "CURSOR_TRIGGER_LOGIN is required so the loop can trust only metadata comments authored by the trigger service account.",
  );
}

function getPullNumber(context) {
  if (context.payload.pull_request?.number) {
    return context.payload.pull_request.number;
  }

  if (context.eventName === "workflow_run") {
    return context.payload.workflow_run?.pull_requests?.[0]?.number ?? null;
  }

  return null;
}

async function loadLoopState({ github, owner, repo, pullNumber, trustedLogin }) {
  const comments = await github.paginate(github.rest.issues.listComments, {
    owner,
    repo,
    issue_number: pullNumber,
    per_page: 100,
  });

  const state = {
    round: 0,
    reviewIds: new Set(),
    reviewCommentIds: new Set(),
    checkKeys: new Set(),
    statusKeys: new Set(),
    maxRoundStopped: false,
  };

  for (const comment of comments) {
    if (normalizeLogin(comment.user?.login) !== trustedLogin) {
      continue;
    }

    const metadata = parseMetadata(comment.body ?? "");
    if (!metadata) {
      continue;
    }

    if (typeof metadata.round === "number") {
      state.round = Math.max(state.round, metadata.round);
    }

    for (const reviewId of metadata.reviewIds ?? []) {
      state.reviewIds.add(reviewId);
    }

    for (const reviewCommentId of metadata.reviewCommentIds ?? []) {
      state.reviewCommentIds.add(reviewCommentId);
    }

    for (const checkKey of metadata.checkKeys ?? []) {
      state.checkKeys.add(checkKey);
    }

    for (const statusKey of metadata.statusKeys ?? []) {
      state.statusKeys.add(statusKey);
    }

    if (metadata.stopped === "max-round") {
      state.maxRoundStopped = true;
    }
  }

  return state;
}

function parseMetadata(body) {
  const match = body.match(
    new RegExp(`<!--\\s*${escapeRegExp(COMMENT_MARKER)}:(\\{[\\s\\S]*?\\})\\s*-->`),
  );
  if (!match) {
    return null;
  }

  try {
    return JSON.parse(match[1]);
  } catch {
    return null;
  }
}

async function collectCopilotReviewComments({
  github,
  owner,
  repo,
  pullNumber,
  state,
}) {
  const reviewComments = await github.paginate(
    github.rest.pulls.listReviewComments,
    {
      owner,
      repo,
      pull_number: pullNumber,
      per_page: 100,
    },
  );

  const copilotComments = reviewComments.filter((comment) => {
    return (
      isCopilotActor(comment.user) &&
      !comment.in_reply_to_id &&
      Boolean(normalizeText(comment.body))
    );
  });

  return {
    newComments: copilotComments.filter((comment) => {
      return !state.reviewCommentIds.has(comment.id);
    }),
    reviewIdsWithInlineComments: new Set(
      copilotComments
        .map((comment) => comment.pull_request_review_id)
        .filter(Boolean),
    ),
  };
}

async function collectCopilotReviewBodies({
  github,
  owner,
  repo,
  pullNumber,
  handledReviewIds,
  reviewIdsWithInlineComments,
}) {
  const reviews = await github.paginate(github.rest.pulls.listReviews, {
    owner,
    repo,
    pull_number: pullNumber,
    per_page: 100,
  });

  return reviews.filter((review) => {
    if (!isCopilotActor(review.user)) {
      return false;
    }
    if (!normalizeText(review.body)) {
      return false;
    }
    if (review.state === "APPROVED") {
      return false;
    }
    if (reviewIdsWithInlineComments.has(review.id)) {
      return false;
    }
    return !handledReviewIds.has(review.id);
  });
}

async function collectCiFailures({ github, owner, repo, headSha, state }) {
  const checkRuns = await github.paginate(
    github.rest.checks.listForRef,
    {
      owner,
      repo,
      ref: headSha,
      per_page: 100,
    },
    (response) => response.data.check_runs ?? [],
  );
  const latestCheckRunByName = new Map();
  for (const run of checkRuns) {
    if (
      !run.name ||
      run.name === WORKFLOW_NAME ||
      run.name === WORKFLOW_JOB_NAME
    ) {
      continue;
    }

    const key = `${run.app?.slug ?? "unknown"}:${run.name}`;
    const previous = latestCheckRunByName.get(key);
    if (!previous || isNewerCheckRun(run, previous)) {
      latestCheckRunByName.set(key, run);
    }
  }

  const failingCheckRuns = [...latestCheckRunByName.values()].filter((run) => {
    if (!isFailingCheckRun(run)) {
      return false;
    }
    return !state.checkKeys.has(makeCheckKey(headSha, run));
  });

  const {
    data: combinedStatus,
  } = await github.rest.repos.getCombinedStatusForRef({
    owner,
    repo,
    ref: headSha,
  });
  const latestStatusByContext = new Map();
  for (const status of combinedStatus.statuses) {
    const previous = latestStatusByContext.get(status.context);
    if (!previous || isNewerStatus(status, previous)) {
      latestStatusByContext.set(status.context, status);
    }
  }
  const failingStatuses = [...latestStatusByContext.values()].filter((status) => {
    if (!isFailingStatus(status)) {
      return false;
    }
    return !state.statusKeys.has(makeStatusKey(headSha, status));
  });

  return {
    checkRuns: failingCheckRuns,
    statuses: failingStatuses,
  };
}

function isNewerCheckRun(candidate, current) {
  const candidateCompletedAt = Date.parse(candidate.completed_at ?? "") || 0;
  const currentCompletedAt = Date.parse(current.completed_at ?? "") || 0;
  if (candidateCompletedAt !== currentCompletedAt) {
    return candidateCompletedAt > currentCompletedAt;
  }
  return candidate.id > current.id;
}

function isNewerStatus(candidate, current) {
  const candidateUpdatedAt = Date.parse(candidate.updated_at ?? "") || 0;
  const currentUpdatedAt = Date.parse(current.updated_at ?? "") || 0;
  if (candidateUpdatedAt !== currentUpdatedAt) {
    return candidateUpdatedAt > currentUpdatedAt;
  }
  return candidate.id > current.id;
}

function isCopilotActor(actor) {
  return COPILOT_ACTOR_LOGINS.has(normalizeLogin(actor?.login));
}

function isFailingCheckRun(run) {
  return [
    "failure",
    "timed_out",
    "cancelled",
    "action_required",
    "startup_failure",
  ].includes(run.conclusion ?? "");
}

function isFailingStatus(status) {
  return ["error", "failure"].includes(status.state ?? "");
}

function makeCheckKey(headSha, run) {
  const appSlug = run.app?.slug ?? "unknown";
  return `${headSha}:check:${appSlug}:${run.name}`;
}

function makeStatusKey(headSha, status) {
  return `${headSha}:status:${status.context}`;
}

function buildPromptComment({
  round,
  maxRounds,
  headSha,
  reviewComments,
  reviewBodies,
  ciFailures,
}) {
  const lines = [
    "@cursor Copilot review 指摘または CI failure の対応候補があります。",
    "",
    `- ラウンド: ${round}/${maxRounds}`,
    "- まず各項目の妥当性を自分で検証してください。Copilot コメントや CI failure を鵜呑みにしないでください。",
    "- 妥当だと判断した項目だけを、最小変更で修正してください。",
    "- 妥当でない、または今回の PR では対応不要だと判断した項目は、その理由を PR コメントで簡潔に説明してください。",
    "- 無関係な変更や大きなリファクタはしないでください。",
    "- `NOTE` コメントは消さず、既存の repo 規約に従ってください。",
    "- 必要なら関連テストやチェックを実行してください。",
  ];
  const included = {
    reviewBodies: [],
    reviewComments: [],
    checkRuns: [],
    statuses: [],
  };

  if (reviewComments.length > 0 || reviewBodies.length > 0) {
    appendBoundedSection({
      lines,
      title: "### Copilot review",
      entries: [
        ...reviewBodies.map((review) => ({
          text: formatReviewBody(review),
          include: () => included.reviewBodies.push(review),
        })),
        ...reviewComments.map((comment) => ({
          text: formatReviewComment(comment),
          include: () => included.reviewComments.push(comment),
        })),
      ],
    });
  }

  if (ciFailures.checkRuns.length > 0 || ciFailures.statuses.length > 0) {
    appendBoundedSection({
      lines,
      title: "### CI failures",
      entries: [
        ...ciFailures.checkRuns.map((run) => ({
          text: formatCheckRun(run),
          include: () => included.checkRuns.push(run),
        })),
        ...ciFailures.statuses.map((status) => ({
          text: formatStatus(status),
          include: () => included.statuses.push(status),
        })),
      ],
    });
  }

  const metadata = {
    round,
    headSha,
    reviewIds: included.reviewBodies.map((review) => review.id),
    reviewCommentIds: included.reviewComments.map((comment) => comment.id),
    checkKeys: included.checkRuns.map((run) => makeCheckKey(headSha, run)),
    statusKeys: included.statuses.map((status) => makeStatusKey(headSha, status)),
  };
  lines.push("", buildMetadataComment(metadata));
  return lines.join("\n");
}

function buildStopComment({ maxRounds, metadata }) {
  return [
    `Copilot-Cursor review loop は最大 ${maxRounds} ラウンドに到達したため、自動の \`@cursor\` 依頼を停止します。`,
    "",
    "- 追加対応が必要な場合は、人手で残件を確認してください。",
    "",
    buildMetadataComment(metadata),
  ].join("\n");
}

function buildMetadataComment(metadata) {
  return `<!-- ${COMMENT_MARKER}:${JSON.stringify(metadata)} -->`;
}

function appendBoundedSection({ lines, title, entries }) {
  if (entries.length === 0) {
    return;
  }

  lines.push("", title);
  let includedCount = 0;
  let omittedCount = 0;
  for (const entry of entries) {
    const projectedLength =
      lines.join("\n").length +
      entry.text.length +
      COMMENT_METADATA_RESERVE;
    if (
      includedCount >= MAX_SECTION_ITEMS ||
      projectedLength > MAX_PROMPT_COMMENT_LENGTH
    ) {
      omittedCount += 1;
      continue;
    }

    lines.push(entry.text);
    entry.include();
    includedCount += 1;
  }

  if (omittedCount > 0) {
    lines.push(`- 他 ${omittedCount} 件は省略しました。`);
  }
}

function formatReviewBody(review) {
  const summary = truncateText(normalizeText(review.body), 280);
  const state = review.state?.toLowerCase() ?? "commented";
  return `- [review:${state}] ${summary} (${review.html_url})`;
}

function formatReviewComment(comment) {
  const summary = truncateText(normalizeText(comment.body), 280);
  const line = comment.line ?? comment.original_line ?? "?";
  return `- [comment] \`${comment.path}:${line}\` ${summary} (${comment.html_url})`;
}

function formatCheckRun(run) {
  const title = truncateText(
    normalizeText(run.output?.title || run.output?.summary || run.name),
    280,
  );
  return `- [check:${run.conclusion}] \`${run.name}\` ${title} (${run.html_url})`;
}

function formatStatus(status) {
  const summary = truncateText(
    normalizeText(status.description || status.context),
    280,
  );
  return `- [status:${status.state}] \`${status.context}\` ${summary} (${status.target_url || "no-link"})`;
}

function normalizeText(text) {
  return String(text ?? "").replace(/\s+/g, " ").trim();
}

function normalizeLogin(login) {
  return String(login ?? "").trim().toLowerCase();
}

function escapeRegExp(text) {
  return String(text).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function truncateText(text, maxLength) {
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.slice(0, maxLength - 1)}…`;
}
