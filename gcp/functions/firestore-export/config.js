const environments = require('./environments.json');

function getEnvironment(environment, projectId) {
  if (!Object.hasOwn(environments, environment)) {
    throw new Error('YSS_EXPORT_ENVIRONMENT must be development or production');
  }
  const target = environments[environment];
  if (projectId !== target.projectId) {
    throw new Error(
      `Project mismatch for ${environment}: expected ${target.projectId}`,
    );
  }
  return Object.freeze({ ...target });
}

function fromRuntime(env) {
  const target = getEnvironment(
    env.YSS_EXPORT_ENVIRONMENT,
    env.YSS_EXPORT_PROJECT_ID,
  );

  // Platform-provided aliases are optional. If present, they must agree with
  // the explicit YSS project ID, but runtime correctness never depends on them.
  for (const projectId of [env.GCLOUD_PROJECT, env.GCP_PROJECT]) {
    if (projectId !== undefined && projectId !== target.projectId) {
      throw new Error(
        'Runtime project aliases disagree with explicit export project',
      );
    }
  }
  return target;
}

module.exports = { getEnvironment, fromRuntime };
