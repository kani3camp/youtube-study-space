const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');

const packageRoot = path.join(__dirname, '..');

const expectedRuntimeFiles = [
  '.gcloudignore',
  'index.js',
  'config.js',
  'environments.json',
  'package.json',
  'package-lock.json',
];

function readRules(content) {
  return content
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line !== '' && !line.startsWith('#'));
}

function validateDeployIgnore(rules) {
  assert.equal(rules[0], '*', 'deployment allowlist must ignore by default');
  assert.equal(
    rules[1],
    '!.',
    'root directory must be re-included before root files; otherwise Gen1 gcloud packaging can create an empty ZIP',
  );
  assert.deepEqual(
    rules.slice(2),
    expectedRuntimeFiles.map((file) => `!/${file}`),
    'deployment allowlist must contain exactly the canonical runtime files',
  );
}

test('.gcloudignore preserves the root directory and exact runtime allowlist', () => {
  const content = fs.readFileSync(
    path.join(packageRoot, '.gcloudignore'),
    'utf8',
  );
  validateDeployIgnore(readRules(content));
});

test('known-bad root-pruning rule is rejected', () => {
  const broken = ['*', ...expectedRuntimeFiles.map((file) => `!/${file}`)];
  assert.throws(
    () => validateDeployIgnore(broken),
    /root directory must be re-included/,
  );
});

test('package main and every allowed runtime file exist at archive root', () => {
  const pkg = JSON.parse(
    fs.readFileSync(path.join(packageRoot, 'package.json'), 'utf8'),
  );
  assert.equal(pkg.main, 'index.js');

  for (const file of expectedRuntimeFiles) {
    const filePath = path.join(packageRoot, file);
    assert.equal(
      fs.statSync(filePath).isFile(),
      true,
      `${file} must exist as a root file`,
    );
  }

  assert.equal(
    expectedRuntimeFiles.includes(pkg.main),
    true,
    'package main must be part of the deployment allowlist',
  );
});
