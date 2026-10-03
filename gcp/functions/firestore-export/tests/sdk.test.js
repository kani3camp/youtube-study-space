const assert = require('node:assert/strict');
const { test } = require('node:test');
const firestore = require('@google-cloud/firestore');

for (const fallback of [false, true]) {
  test(`installed SDK on Node22 starts export through ${fallback ? 'REST' : 'gRPC'} API surface`, async () => {
    assert.equal(
      process.versions.node.split('.')[0],
      '22',
      'run this package with Node.js 22',
    );
    const client = new firestore.v1.FirestoreAdminClient({
      projectId: 'test-youtube-study-space',
      fallback,
      credentials: {
        client_email: 'offline@example.invalid',
        private_key: 'offline-placeholder',
        universe_domain: 'googleapis.com',
      },
    });
    assert.equal(
      client.databasePath('test-youtube-study-space', '(default)'),
      'projects/test-youtube-study-space/databases/(default)',
    );
    const request = {
      name: client.databasePath('test-youtube-study-space', '(default)'),
      outputUriPrefix: 'gs://firestore-backup-test-youtube-study-space',
      collectionIds: ['users', 'user-activities', 'order-history'],
    };
    const operation = {
      name: 'operations/offline',
      done: false,
      promise: () => assert.fail('must not wait for LRO completion'),
    };
    let called = 0;
    // Stub at the RPC boundary, before initialize can load credentials or perform network IO.
    client.initialize = () => Promise.resolve();
    client.innerApiCalls.exportDocuments = (actual, options) => {
      called++;
      assert.deepEqual(actual, request);
      assert.equal(
        options.otherArgs.headers['x-goog-request-params'],
        'name=projects%2Ftest-youtube-study-space%2Fdatabases%2F(default)',
      );
      return Promise.resolve([operation]);
    };
    const [started] = await client.exportDocuments(request);
    assert.equal(started, operation);
    assert.equal(called, 1);
  });
}

test('locked SDK export start keeps the baseline no-retry/60s RPC policy', () => {
  const path = require('node:path');
  const policy = require(
    path.join(
      path.dirname(require.resolve('@google-cloud/firestore')),
      'v1/firestore_admin_client_config.json',
    ),
  ).interfaces['google.firestore.admin.v1.FirestoreAdmin'];
  const start = policy.methods.ExportDocuments;
  assert.deepEqual(policy.retry_codes[start.retry_codes_name], []);
  assert.equal(start.timeout_millis, 60000);
});
