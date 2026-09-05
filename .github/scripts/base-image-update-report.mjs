#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import { pathToFileURL } from "node:url";

export const COMMENT_MARKER = "<!-- base-image-update-report -->";

const ALLOWED_REGISTRIES = new Set(["docker.io", "public.ecr.aws", "gcr.io"]);

export function registryHost(image) {
  const firstComponent = image.split("/")[0];
  if (
    image.includes("/") &&
    (firstComponent.includes(".") || firstComponent.includes(":") || firstComponent === "localhost")
  ) {
    return firstComponent.toLowerCase();
  }
  return "docker.io";
}

export function canVerifyRegistryUpdate(oldImage, newImage) {
  return (
    oldImage.image === newImage.image &&
    ALLOWED_REGISTRIES.has(registryHost(newImage.image))
  );
}

export function parseFromImages(content) {
  const images = [];
  for (const [index, line] of content.split(/\r?\n/).entries()) {
    const trimmed = line.trim();
    if (!/^FROM\s+/i.test(trimmed)) continue;

    const parts = trimmed.split(/\s+/);
    let imageIndex = 1;
    while (parts[imageIndex]?.startsWith("--")) imageIndex += 1;
    if (!parts[imageIndex]) continue;

    images.push({
      stage: images.length + 1,
      line: index + 1,
      image: parts[imageIndex],
    });
  }
  return images;
}

export function parsePinnedImage(ref) {
  const match = ref.match(/^(.*)@(sha256:[0-9a-f]{64})$/i);
  if (!match) return null;

  const taggedRef = match[1];
  const digest = match[2].toLowerCase();
  const slashIndex = taggedRef.lastIndexOf("/");
  const colonIndex = taggedRef.lastIndexOf(":");
  if (colonIndex <= slashIndex) return null;

  const image = taggedRef.slice(0, colonIndex);
  const tag = taggedRef.slice(colonIndex + 1);
  if (!/^[A-Za-z0-9._/:~-]+$/.test(image)) return null;
  if (!/^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/.test(tag)) return null;

  return {
    ref,
    taggedRef,
    image,
    tag,
    digest,
  };
}

export function detectDigestUpdates(baseContent, headContent) {
  const before = parseFromImages(baseContent);
  const after = parseFromImages(headContent);
  const updates = [];
  const count = Math.max(before.length, after.length);

  for (let index = 0; index < count; index += 1) {
    const oldStage = before[index];
    const newStage = after[index];
    if (!oldStage || !newStage || oldStage.image === newStage.image) continue;

    const oldImage = parsePinnedImage(oldStage.image);
    const newImage = parsePinnedImage(newStage.image);
    if (!oldImage || !newImage) continue;

    updates.push({
      stage: newStage.stage,
      line: newStage.line,
      oldImage,
      newImage,
    });
  }

  return updates;
}

export function publicGalleryUrl(taggedRef) {
  const prefix = "public.ecr.aws/";
  if (!taggedRef.startsWith(prefix)) return null;

  const withoutRegistry = taggedRef.slice(prefix.length);
  const slashIndex = withoutRegistry.lastIndexOf("/");
  const colonIndex = withoutRegistry.lastIndexOf(":");
  const repository = colonIndex > slashIndex
    ? withoutRegistry.slice(0, colonIndex)
    : withoutRegistry;
  if (!repository) return null;

  return `https://gallery.ecr.aws/${repository}`;
}

export function googleArtifactRegistryUrl(image) {
  const prefix = "gcr.io/";
  if (!image.startsWith(prefix)) return null;

  const parts = image.slice(prefix.length).split("/");
  if (parts.length < 2) return null;

  let encodedProject;
  if (parts[0].includes(".")) {
    if (parts.length < 3) return null;
    const domain = parts.shift();
    const projectId = parts.shift();
    encodedProject = `${encodeURIComponent(domain)}:${encodeURIComponent(projectId)}`;
  } else {
    encodedProject = encodeURIComponent(parts.shift());
  }

  if (!encodedProject || parts.length === 0) return null;
  const encodedImagePath = parts.map(encodeURIComponent).join("/");
  return `https://console.cloud.google.com/artifacts/docker/${encodedProject}/us/gcr.io/${encodedImagePath}`;
}

export function renderReport(entries) {
  const lines = [
    COMMENT_MARKER,
    "## 🐳 Base image update report",
    "",
    `Detected **${entries.length}** digest-pinned base image update${entries.length === 1 ? "" : "s"}.`,
    "",
  ];

  for (const entry of entries) {
    const { file, stage, line, oldImage, newImage, resolvedDigest, verificationError } = entry;
    const matches = resolvedDigest === newImage.digest;
    const galleryUrl = publicGalleryUrl(newImage.taggedRef);
    const artifactRegistryUrl = googleArtifactRegistryUrl(newImage.image);

    lines.push(`### ${matches ? "✅" : "❌"} \`${newImage.taggedRef}\``);
    lines.push("");
    lines.push(`> \`${oldImage.digest}\` → \`${newImage.digest}\``);
    lines.push("");
    lines.push("| Item | Value |");
    lines.push("| --- | --- |");
    lines.push(`| Dockerfile | \`${file}\` (stage ${stage}, line ${line}) |`);
    if (oldImage.tag !== newImage.tag || oldImage.image !== newImage.image) {
      lines.push(`| Before image | \`${oldImage.taggedRef}\` |`);
      lines.push(`| After image | \`${newImage.taggedRef}\` |`);
    } else {
      lines.push(`| Image tag | \`${newImage.taggedRef}\` |`);
    }
    lines.push(`| Before digest | \`${oldImage.digest}\` |`);
    lines.push(`| After digest | \`${newImage.digest}\` |`);

    if (resolvedDigest) {
      lines.push(`| Current tag digest | \`${resolvedDigest}\` |`);
      lines.push(`| Verification | ${matches ? "✅ Pinned digest matches the current registry tag" : "❌ Pinned digest does not match the current registry tag"} |`);
    } else {
      lines.push("| Current tag digest | _Unavailable_ |");
      lines.push(`| Verification | ❌ Could not verify the current registry tag${verificationError ? `: \`${escapeInlineCode(verificationError)}\`` : ""} |`);
    }

    lines.push("");
    if (galleryUrl) {
      lines.push(`🔗 [View \`${newImage.image}\` in Amazon ECR Public Gallery](${galleryUrl})`);
      lines.push("");
    }
    if (artifactRegistryUrl) {
      lines.push(`🔗 [View \`${newImage.image}\` in Google Artifact Registry](${artifactRegistryUrl})`);
      lines.push("");
    }
  }

  if (entries.every((entry) => entry.resolvedDigest === entry.newImage.digest)) {
    lines.push("> ✅ All pinned digests in this update match their current registry tags.");
  } else {
    lines.push("> ❌ One or more pinned digests could not be verified against the current registry tag.");
  }
  lines.push("");
  lines.push("_Generated automatically from the PR Dockerfile diff and the container registry._");
  lines.push("");

  return lines.join("\n");
}

function escapeInlineCode(value) {
  return String(value).replaceAll("`", "'").replaceAll("\n", " ").trim();
}

function requireEnv(name) {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required environment variable: ${name}`);
  return value;
}

async function githubRequest(path, options = {}) {
  const apiUrl = process.env.GITHUB_API_URL || "https://api.github.com";
  const token = requireEnv("GITHUB_TOKEN");
  const response = await fetch(`${apiUrl}${path}`, {
    ...options,
    headers: {
      Accept: "application/vnd.github+json",
      Authorization: `Bearer ${token}`,
      "X-GitHub-Api-Version": "2022-11-28",
      ...(options.headers || {}),
    },
  });

  if (!response.ok) {
    const body = await response.text();
    throw new Error(`GitHub API ${options.method || "GET"} ${path} failed: ${response.status} ${body}`);
  }

  if (response.status === 204) return null;
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

function resolveTagDigest(taggedRef) {
  return execFileSync(
    "docker",
    ["buildx", "imagetools", "inspect", taggedRef, "--format", "{{.Manifest.Digest}}"],
    { encoding: "utf8", stdio: ["ignore", "pipe", "pipe"], timeout: 120_000 },
  ).trim().toLowerCase();
}

async function findExistingReportComment(repository, pullNumber) {
  const comments = await paginatedGithubRequest(`/repos/${repository}/issues/${pullNumber}/comments`);
  return comments.find(
    (comment) => comment.user?.login === "github-actions[bot]" && comment.body?.includes(COMMENT_MARKER),
  );
}

async function upsertReportComment(repository, pullNumber, body) {
  const existing = await findExistingReportComment(repository, pullNumber);
  if (existing) {
    await githubRequest(`/repos/${repository}/issues/comments/${existing.id}`, {
      method: "PATCH",
      body: JSON.stringify({ body }),
      headers: { "Content-Type": "application/json" },
    });
    return;
  }

  await githubRequest(`/repos/${repository}/issues/${pullNumber}/comments`, {
    method: "POST",
    body: JSON.stringify({ body }),
    headers: { "Content-Type": "application/json" },
  });
}

async function deleteExistingReportComment(repository, pullNumber) {
  const existing = await findExistingReportComment(repository, pullNumber);
  if (!existing) return;
  await githubRequest(`/repos/${repository}/issues/comments/${existing.id}`, { method: "DELETE" });
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
  const dockerfiles = changedFiles
    .map((file) => file.filename)
    .filter((filename) => /^system\/Dockerfile[^/]*$/.test(filename));

  const entries = [];
  for (const file of dockerfiles) {
    const [baseContent, headContent] = await Promise.all([
      getFileContent(repository, file, baseSha),
      getFileContent(repository, file, headSha),
    ]);

    for (const update of detectDigestUpdates(baseContent, headContent)) {
      let resolvedDigest = null;
      let verificationError = null;
      if (!canVerifyRegistryUpdate(update.oldImage, update.newImage)) {
        verificationError =
          "Automatic verification skipped because the image repository changed or the registry is not allowlisted.";
      } else {
        try {
          resolvedDigest = resolveTagDigest(update.newImage.taggedRef);
        } catch (error) {
          verificationError = error instanceof Error ? error.message : String(error);
        }
      }
      entries.push({ file, ...update, resolvedDigest, verificationError });
    }
  }

  if (entries.length === 0) {
    await deleteExistingReportComment(repository, pullNumber);
    console.log("No digest-pinned base image updates detected.");
    return;
  }

  const report = renderReport(entries);
  await upsertReportComment(repository, pullNumber, report);
  process.stdout.write(`${report}\n`);

  const failed = entries.filter((entry) => entry.resolvedDigest !== entry.newImage.digest);
  if (failed.length > 0) {
    throw new Error(`${failed.length} base image digest verification(s) failed`);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error instanceof Error ? error.stack : error);
    process.exitCode = 1;
  });
}
