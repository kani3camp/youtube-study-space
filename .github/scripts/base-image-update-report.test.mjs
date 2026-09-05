import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  COMMENT_MARKER,
  canVerifyRegistryUpdate,
  detectDigestUpdates,
  parseFromImages,
  parsePinnedImage,
  publicGalleryUrl,
  registryHost,
  renderReport,
} from "./base-image-update-report.mjs";

const OLD = "sha256:d8d858dc6f7a6552ffc131e029802c28969c8211d311c33f9400449e30cf7442";
const NEW = "sha256:6d97a9ef52d2d84ef361172b7cfc12ca673a7e6dd722fd4b195b3f15bfcdb767";

test("parseFromImages extracts multi-stage FROM images", () => {
  const images = parseFromImages(`FROM --platform=linux/amd64 golang:1.26@${OLD} AS build\nFROM public.ecr.aws/lambda/provided:al2023@${NEW}\n`);
  assert.deepEqual(images, [
    { stage: 1, line: 1, image: `golang:1.26@${OLD}` },
    { stage: 2, line: 2, image: `public.ecr.aws/lambda/provided:al2023@${NEW}` },
  ]);
});

test("parsePinnedImage separates image, tag, and digest", () => {
  assert.deepEqual(parsePinnedImage(`public.ecr.aws/lambda/provided:al2023@${NEW}`), {
    ref: `public.ecr.aws/lambda/provided:al2023@${NEW}`,
    taggedRef: "public.ecr.aws/lambda/provided:al2023",
    image: "public.ecr.aws/lambda/provided",
    tag: "al2023",
    digest: NEW,
  });
});

test("detectDigestUpdates finds PR 1045-style digest-only changes", () => {
  const base = `FROM golang:1.26@${OLD} AS build\nFROM public.ecr.aws/lambda/provided:al2023@${OLD}\n`;
  const head = `FROM golang:1.26@${OLD} AS build\nFROM public.ecr.aws/lambda/provided:al2023@${NEW}\n`;
  const updates = detectDigestUpdates(base, head);

  assert.equal(updates.length, 1);
  assert.equal(updates[0].stage, 2);
  assert.equal(updates[0].line, 2);
  assert.equal(updates[0].oldImage.digest, OLD);
  assert.equal(updates[0].newImage.digest, NEW);
});

test("detectDigestUpdates also reports pinned tag changes", () => {
  const base = `FROM golang:1.26@${OLD} AS build\n`;
  const head = `FROM golang:1.27@${NEW} AS build\n`;
  const updates = detectDigestUpdates(base, head);

  assert.equal(updates.length, 1);
  assert.equal(updates[0].oldImage.tag, "1.26");
  assert.equal(updates[0].newImage.tag, "1.27");
});

test("registry verification is restricted to the existing image repository and known registries", () => {
  assert.equal(registryHost("golang"), "docker.io");
  assert.equal(registryHost("public.ecr.aws/lambda/provided"), "public.ecr.aws");
  assert.equal(registryHost("gcr.io/distroless/static-debian12"), "gcr.io");

  const oldImage = parsePinnedImage(`public.ecr.aws/lambda/provided:al2023@${OLD}`);
  const sameRepository = parsePinnedImage(`public.ecr.aws/lambda/provided:al2023@${NEW}`);
  const changedRepository = parsePinnedImage(`evil.example/repo:al2023@${NEW}`);

  assert.equal(canVerifyRegistryUpdate(oldImage, sameRepository), true);
  assert.equal(canVerifyRegistryUpdate(oldImage, changedRepository), false);
});

test("publicGalleryUrl returns a stable ECR Public Gallery repository URL", () => {
  assert.equal(
    publicGalleryUrl("public.ecr.aws/lambda/provided:al2023"),
    "https://gallery.ecr.aws/lambda/provided",
  );
  assert.equal(publicGalleryUrl("golang:1.26"), null);
});

test("renderReport includes verified digests and ECR Public Gallery link", () => {
  const update = detectDigestUpdates(
    `FROM public.ecr.aws/lambda/provided:al2023@${OLD}\n`,
    `FROM public.ecr.aws/lambda/provided:al2023@${NEW}\n`,
  )[0];

  const report = renderReport([
    {
      file: "system/Dockerfile.lambda",
      ...update,
      resolvedDigest: NEW,
      verificationError: null,
    },
  ]);

  assert.match(report, new RegExp(COMMENT_MARKER.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  assert.match(report, /## 🐳 Base image update report/);
  assert.match(report, /✅ Pinned digest matches the current registry tag/);
  assert.match(report, new RegExp(OLD));
  assert.match(report, new RegExp(NEW));
  assert.match(report, /https:\/\/gallery\.ecr\.aws\/lambda\/provided/);
});

test("renderReport is explicit when registry verification fails", () => {
  const update = detectDigestUpdates(
    `FROM public.ecr.aws/lambda/provided:al2023@${OLD}\n`,
    `FROM public.ecr.aws/lambda/provided:al2023@${NEW}\n`,
  )[0];

  const report = renderReport([
    {
      file: "system/Dockerfile.lambda",
      ...update,
      resolvedDigest: OLD,
      verificationError: null,
    },
  ]);

  assert.match(report, /❌ Pinned digest does not match the current registry tag/);
  assert.match(report, /One or more pinned digests could not be verified/);
});

test("report workflow remains advisory and catches stale-comment cleanup", () => {
  const workflow = readFileSync(".github/workflows/base-image-update-report.yml", "utf8");

  assert.match(workflow, /pull_request_target:[\s\S]*?types: \[opened, synchronize, reopened\]/);
  assert.doesNotMatch(workflow, /pull_request_target:[\s\S]*?paths:/);
  assert.doesNotMatch(workflow, /Verify Docker Buildx availability/);
  assert.match(workflow, /name: Checkout trusted base revision[\s\S]*?continue-on-error: true/);
  assert.match(workflow, /name: Generate or update PR report[\s\S]*?continue-on-error: true/);
  assert.match(workflow, /name: Keep report advisory[\s\S]*?if: always\(\)[\s\S]*?continue-on-error: true/);
  assert.match(workflow, /does not block merging/);
});
