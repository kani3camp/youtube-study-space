const firestore = require('@google-cloud/firestore');
const { fromRuntime } = require('./config');
const target = fromRuntime(process.env);
const client = new firestore.v1.FirestoreAdminClient();
const bucket = `gs://${target.bucket}`;

exports.scheduledFirestoreExport = (_event, _context) => {
  const databaseName = client.databasePath(target.projectId, '(default)');

  return client
    .exportDocuments({
      name: databaseName,
      outputUriPrefix: bucket,
      collectionIds: ['users', 'user-activities', 'order-history'],
    })
    .then((responses) => {
      const response = responses[0];
      console.log(`Operation Name: ${response.name}`);
      return response;
    })
    .catch((err) => {
      // NOTE: Preserve v5's log-and-resolve behavior. Rethrowing is a separate change.
      console.error(err);
    });
};
