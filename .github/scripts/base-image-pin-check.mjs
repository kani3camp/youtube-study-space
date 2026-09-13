#!/usr/bin/env node

import { pathToFileURL } from "node:url";

import { parsePinnedImage } from "./base-image-update-report.mjs";

export function parseFromStages(content) {
  const stages = [];
  for (const [index, line] of content.split(/\r?\n/).entries()) {
    const trimmed = line.trim();
    if (!/^FROM\s+/i.test(trimmed)) continue;

    const parts = trimmed.split(/\s+/);
    let imageIndex = 1;
    while (parts[imageIndex]?.startsWith("--")) imageIndex += 1;
    if (!parts[imageIndex]) continue;

    const alias = /^AS$/i.test(parts[imageIndex + 1] || "")
      ? parts[imageIndex + 2] || null
      : null;

    stages.push({
      stage: stages.length + 1,
      line: index + 1,
      image: parts[imageIndex],
      alias,
    });
  }
  return stages;
}

export function detectUnpinnedExternalBaseImages(headContent) {
  const declaredAliases = new Set();
  const violations = [];

  for (const stage of parseFromStages(headContent)) {
    const normalizedRef = stage.image.toLowerCase();
    const isInternalStage = declaredAliases.has(normalizedRef);
    const isScratch = normalizedRef === "scratch";

    if (!isInternalStage && !isScratch && !parsePinnedImage(stage.image)) {
      violations.push({
        stage: stage.stage,
        line: stage.line,
        ref: stage.image,
      });
    }

    if (stage.alias) {
      declaredAliases.add(stage.alias.toLowerCase());
    }
  }

  return violations;
}

function requireEnv(name) {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required environment variable: ${name}`);
  return value;
}

async function githubRequest(path) {
  const apiUrl = process.env.GITHUB_API_URL || "https://api.github.com";
  const token = requireEnv("GITHUB_TOKEN");
  const response = await fetch(`${apiUrl}${path}`, {
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${token}`,
      "X-GitHub-Api-Version": "2022-11-28",
    },
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`GitHub API GET ${path} failed: ${response.status} ${body}`);
  }

  return response.json();
}

async function paginatedGithubRequest(path) {
  const items = [];
  for (let page = 1; ; page += 1) {
    const separator = path.includes("?") ? "&" : "?";
    const pageItems = await githubRequest(`${path}${separator}per_page=100&page=${page}`);
    items.push(...pageItems);
    if (pageItems.length < 100) return items;
  }
}

function encodeRepositoryPath(path) {
  return path.split("/").map(encodeURIComponent).join("/");
}

async function getFileContent(repository, path, ref) {
  const response = await githubRequest(
    `/repos/${repository}/contents/${encodeRepositoryPath(path)}?ref=${encodeURIComponent(ref)}`,
  );
  if (response.type !== "file" || response.encoding !== "base64") {
    throw new Error(`Unexpected GitHub contents response for ${path}@${ref}`);
  }
  return Buffer.from(response.content.replaceAll("\n", ""), "base64").toString("utf8");
}

async function main() {
  const repository = requireEnv("GITHUB_REPOSITORY");
  const pullNumber = Number(requireEnv("PR_NUMBER"));
  const headSha = requireEnv("HEAD_SHA");

  if (!Number.isInteger(pullNumber) || pullNumber <= 0) {
    throw new Error(`Invalid PR_NUMBER: ${process.env.PR_NUMBER}`);
  }

  const changedFiles = await paginatedGithubRequest(`/repos/${repository}/pulls/${pullNumber}/files`);
  const dockerfiles = changedFiles.filter(
    (file) => /^system\/Dockerfile[^/]*$/.test(file.filename) && file.status !== "removed",
  );

  const violations = [];
  for (const file of dockerfiles) {
    const headContent = await getFileContent(repository, file.filename, headSha);
    for (const violation of detectUnpinnedExternalBaseImages(headContent)) {
      violations.push({ file: file.filename, ...violation });
    }
  }

  if (violations.length === 0) {
    console.log("All external base images in changed system Dockerfiles are digest pinned.");
    return;
  }

  for (const violation of violations) {
    console.error(
      `::error file=${violation.file},line=${violation.line}::External base image in stage ${violation.stage} must be pinned by sha256 digest: ${violation.ref}`,
    );
  }

  throw new Error(
    `${violations.length} external base image reference(s) are missing required sha256 digest pinning`,
  );
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.stack : error);
    process.exitCode = 1;
  });
}
