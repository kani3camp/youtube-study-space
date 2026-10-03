const assert = require('node:assert/strict');
const { createHash } = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const { test } = require('node:test');
const vm = require('node:vm');
const { fromRuntime } = require('../config');

const fixture = fs.readFileSync(
  path.join(__dirname, 'fixtures/version5/index.js.txt'),
  'utf8',
);
const canonical = fs.readFileSync(path.join(__dirname, '../index.js'), 'utf8');
const expectedCollections = ['users', 'user-activities', 'order-history'];

function load(source, env, rpc) {
  const calls = [];
  const logs = [];
  const errors = [];
  const databasePaths = [];
  class FirestoreAdminClient {
    databasePath(project, database) {
      databasePaths.push([project, database]);
      return `projects/${project}/databases/${database}`;
    }
    exportDocuments(request) {
      calls.push(JSON.parse(JSON.stringify(request)));
      return rpc(request);
    }
  }
  const sandbox = {
    exports: {},
    process: { env },
    console: {
      log: (...args) => logs.push(args),
      error: (...args) => errors.push(args),
    },
    require: (name) => {
      if (name === '@google-cloud/firestore')
        return { v1: { FirestoreAdminClient } };
      if (name === './config') return { fromRuntime };
      throw new Error(`Unexpected dependency: ${name}`);
    },
  };
  vm.runInNewContext(source, sandbox);
  return {
    handler: sandbox.exports.scheduledFirestoreExport,
    calls,
    logs,
    errors,
    databasePaths,
  };
}

test('recovered fixture remains byte-identical to development v5', () => {
  assert.equal(
    createHash('sha256').update(fixture).digest('hex'),
    '71ed9e254006755734d6e890e58f70c8d08da2201855436062e60389ecc49568',
  );
});

for (const [label, source, environment, project, bucket] of [
  [
    'deployed v5 baseline',
    fixture,
    'development',
    'test-youtube-study-space',
    'firestore-backup-test-youtube-study-space',
  ],
  [
    'canonical development',
    canonical,
    'development',
    'test-youtube-study-space',
    'firestore-backup-test-youtube-study-space',
  ],
  [
    'canonical production',
    canonical,
    'production',
    'youtube-study-space',
    'firestore-backup-youtube-study-space',
  ],
]) {
  const env = { YSS_EXPORT_ENVIRONMENT: environment, GCLOUD_PROJECT: project };
  test(`${label}: exact groups/default database/bucket are independent of payload and context`, async () => {
    const h = load(source, env, () =>
      Promise.resolve([{ name: 'operations/started' }]),
    );
    for (const payload of [
      undefined,
      {},
      { data: 'not-base64' },
      {
        data: Buffer.from(
          JSON.stringify({
            collectionIds: ['live-chat-history'],
            projectId: 'other',
            bucket: 'other',
          }),
        ).toString('base64'),
      },
    ]) {
      await h.handler(payload, { eventId: 'same-event', projectId: 'other' });
    }
    assert.equal(h.calls.length, 4);
    for (const request of h.calls) {
      assert.deepEqual(request, {
        name: `projects/${project}/databases/(default)`,
        outputUriPrefix: `gs://${bucket}`,
        collectionIds: expectedCollections,
      });
      assert.equal(request.collectionIds.includes('live-chat-history'), false);
      assert.ok(
        request.collectionIds.length > 0,
        'must not export the full database',
      );
    }
    assert.deepEqual(h.databasePaths, Array(4).fill([project, '(default)']));
  });
  test(`${label}: awaits start RPC only; returns the still-running LRO without polling`, async () => {
    let start;
    let polls = 0;
    const operation = {
      name: 'operations/started',
      done: false,
      promise: () => {
        polls++;
        throw new Error('must not poll');
      },
    };
    const h = load(
      source,
      env,
      () =>
        new Promise((resolve) => {
          start = resolve;
        }),
    );
    const result = h.handler({}, {});
    let settled = false;
    result.then(() => {
      settled = true;
    });
    await Promise.resolve();
    assert.equal(settled, false);
    start([operation]);
    assert.equal(await result, operation);
    assert.equal(polls, 0);
    assert.deepEqual(h.logs, [['Operation Name: operations/started']]);
    assert.equal(h.calls.length, 1);
  });
  test(`${label}: API rejection is logged once and resolves undefined (no application retry)`, async () => {
    const failure = new Error('export rejected');
    const h = load(source, env, () => Promise.reject(failure));
    assert.equal(await h.handler({}, {}), undefined);
    assert.deepEqual(h.errors, [[failure]]);
    assert.equal(h.calls.length, 1);
  });
  test(`${label}: synchronous SDK throw still propagates`, () => {
    const failure = new Error('synchronous SDK error');
    const h = load(source, env, () => {
      throw failure;
    });
    assert.throws(
      () => h.handler({}, {}),
      (error) => error === failure,
    );
    assert.deepEqual(h.errors, []);
  });
}

test('canonical entry point refuses mismatched environment at load time', () => {
  assert.throws(
    () =>
      load(
        canonical,
        {
          YSS_EXPORT_ENVIRONMENT: 'development',
          GCLOUD_PROJECT: 'youtube-study-space',
        },
        () => assert.fail('must not start an export'),
      ),
    /Project mismatch/,
  );
});
