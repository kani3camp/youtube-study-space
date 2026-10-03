const assert = require('node:assert/strict');
const { test } = require('node:test');
const { fromRuntime, getEnvironment } = require('../config');
const { deploymentPlan, parseArgs } = require('../deployment-plan');

const expected = {
  development: {
    projectId: 'test-youtube-study-space',
    bucket: 'firestore-backup-test-youtube-study-space',
    functionName: 'firestoreCollectionsExport',
    region: 'asia-southeast2',
    topic: 'initiateFirestoreCollectionsExport',
    serviceAccount: 'test-youtube-study-space@appspot.gserviceaccount.com',
    memory: '256MB',
    timeout: '60s',
  },
  production: {
    projectId: 'youtube-study-space',
    bucket: 'firestore-backup-youtube-study-space',
    functionName: 'firestoreExport',
    region: 'asia-northeast2',
    topic: 'initiateFirestoreExport',
    serviceAccount: 'youtube-study-space@appspot.gserviceaccount.com',
    memory: '512MB',
    timeout: '120s',
  },
};

for (const [environment, target] of Object.entries(expected)) {
  test(`${environment}: explicit runtime project selects the target without platform aliases`, () => {
    assert.deepEqual(
      fromRuntime({
        YSS_EXPORT_ENVIRONMENT: environment,
        YSS_EXPORT_PROJECT_ID: target.projectId,
      }),
      target,
    );
  });

  test(`${environment}: optional platform project aliases must agree when present`, () => {
    for (const projectKeys of [
      { GCLOUD_PROJECT: target.projectId },
      { GCP_PROJECT: target.projectId },
      { GCLOUD_PROJECT: target.projectId, GCP_PROJECT: target.projectId },
    ]) {
      assert.deepEqual(
        fromRuntime({
          YSS_EXPORT_ENVIRONMENT: environment,
          YSS_EXPORT_PROJECT_ID: target.projectId,
          ...projectKeys,
        }),
        target,
      );
    }

    const other =
      environment === 'development'
        ? expected.production.projectId
        : expected.development.projectId;

    assert.throws(
      () =>
        fromRuntime({
          YSS_EXPORT_ENVIRONMENT: environment,
          YSS_EXPORT_PROJECT_ID: target.projectId,
          GCP_PROJECT: other,
        }),
      /disagree/,
    );
  });

  test(`${environment}: explicit project mismatch fails closed`, () => {
    const other =
      environment === 'development'
        ? expected.production.projectId
        : expected.development.projectId;

    assert.deepEqual(getEnvironment(environment, target.projectId), target);
    assert.throws(() => getEnvironment(environment, other), /Project mismatch/);
    assert.throws(
      () =>
        fromRuntime({
          YSS_EXPORT_ENVIRONMENT: environment,
          YSS_EXPORT_PROJECT_ID: other,
        }),
      /Project mismatch/,
    );
  });

  test(`${environment}: plan preserves Gen1 trigger/SA/resource limits and targets Node22`, () => {
    const plan = deploymentPlan({
      environment,
      projectId: target.projectId,
      allowProduction: true,
      confirm: `production:${target.projectId}:${target.functionName}`,
    });

    assert.equal(plan.generation, 1);
    assert.equal(plan.runtime, 'nodejs22');
    assert.deepEqual(plan.args.slice(0, -1), [
      'functions',
      'deploy',
      target.functionName,
      `--project=${target.projectId}`,
      '--no-gen2',
      '--runtime=nodejs22',
      '--entry-point=scheduledFirestoreExport',
      `--region=${target.region}`,
      `--trigger-topic=${target.topic}`,
      `--service-account=${target.serviceAccount}`,
      `--memory=${target.memory}`,
      `--timeout=${target.timeout}`,
      '--max-instances=1',
      '--no-retry',
      `--update-env-vars=YSS_EXPORT_ENVIRONMENT=${environment},YSS_EXPORT_PROJECT_ID=${target.projectId}`,
    ]);

    assert.match(
      plan.args.at(-1),
      /^--source=.*gcp\/functions\/firestore-export$/,
    );
  });
}

test('unknown, missing, inherited or abbreviated environment/project values fail closed', () => {
  for (const environment of [
    undefined,
    '',
    'dev',
    'prod',
    'toString',
    '__proto__',
  ]) {
    assert.throws(
      () => getEnvironment(environment, expected.development.projectId),
      /must be development or production/,
    );
  }

  for (const environment of Object.keys(expected)) {
    for (const project of [undefined, '', 'unknown']) {
      assert.throws(
        () => getEnvironment(environment, project),
        /Project mismatch/,
      );
    }

    assert.throws(
      () =>
        fromRuntime({
          YSS_EXPORT_ENVIRONMENT: environment,
        }),
      /Project mismatch/,
    );

    assert.throws(
      () =>
        fromRuntime({
          YSS_EXPORT_ENVIRONMENT: environment,
          YSS_EXPORT_PROJECT_ID: '',
        }),
      /Project mismatch/,
    );
  }
});

test('production plan requires both exact environment-bound confirmation and explicit allowance', () => {
  const options = {
    environment: 'production',
    projectId: 'youtube-study-space',
  };

  for (const extra of [
    {},
    { allowProduction: true },
    { confirm: 'production:youtube-study-space:firestoreExport' },
    {
      allowProduction: true,
      confirm:
        'development:test-youtube-study-space:firestoreCollectionsExport',
    },
  ]) {
    assert.throws(
      () => deploymentPlan({ ...options, ...extra }),
      /Production plan requires/,
    );
  }
});

test('CLI rejects typos, empty values and duplicate flags', () => {
  for (const args of [
    ['--apply'],
    ['--environment='],
    ['--environment=development', '--environment=production'],
    ['--allow-production', '--allow-production'],
  ]) {
    assert.throws(() => parseArgs(args), /argument/i);
  }

  assert.deepEqual(
    parseArgs([
      '--environment=development',
      '--project=test-youtube-study-space',
    ]),
    { environment: 'development', projectId: 'test-youtube-study-space' },
  );
});
