#!/usr/bin/env node

import { pathToFileURL } from "node:url";

import { parseFromImages, parsePinnedImage } from "./base-image-update-report.mjs";

export function detectDigestUnpins(baseContent, headContent) {
  const before = parseFromImages(baseContent);
  const after = parseFromImages(headContent);
  const violations = [];
  const count = Math.max(before.length, after.length);

  for (let index = 0; index < count; index += 1) {
    const oldStage = before[index];
    const newStage = after[index];
    if (!oldStage || !newStage || oldStage.image === newStage.image) continue;

    const oldImage = parsePinnedImage(oldStage.image);
    if (!oldImage) continue;

    const newImage = parsePinnedImage(newStage.image);
    if (newImage) continue;

    violations.push({
      stage: newStage.stage,
      line: newStage.line,
      oldRef: oldStage.image,
      newRef: newStage.image,
    });
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
  const baseSha = requireEnv("BASE_SHA");
  const headSha = requireEnv("HEAD_SHA");

  if (!Number.isInteger(pullNumber) || pullNumber <= 0) {
    throw new Error(`Invalid PR_NUMBER: ${process.env.PR_NUMBER}`);
  }

  const changedFiles = await paginatedGithubRequest(`/repos/${repository}/pulls/${pullNumber}/files`);
  const dockerfiles = changedFiles.filter(
    (file) => /^system\/Dockerfile[^/]*$/.test(file.filename) && file.status !== "added" && file.status !== "removed",
  );

  const violations = [];
  for (const file of dockerfiles) {
    const basePath = file.status === "renamed" && file.previous_filename ? file.previous_filename : file.filename;
    const [baseContent, headContent] = await Promise.all([
      getFileContent(repository, basePath, baseSha),
      getFileContent(repository, file.filename, headSha),
    ]);

    for (const violation of detectDigestUnpins(baseContent, headContent)) {
      violations.push({ file: file.filename, ...violation });
    }
  }

  if (violations.length === 0) {
    console.log("No base image digest pins were removed.");
    return;
  }

  for (const violation of violations) {
    console.error(
      `::error file=${violation.file},line=${violation.line}::Base image digest pin removed in stage ${violation.stage}: ${violation.oldRef} -> ${violation.newRef}`,
    );
  }

  throw new Error(
    `${violations.length} base image reference(s) removed required sha256 digest pinning`,
  );
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.stack : error);
    process.exitCode = 1;
  });
}
