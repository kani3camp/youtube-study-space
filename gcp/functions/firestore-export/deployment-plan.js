const { getEnvironment } = require('./config');

function deploymentPlan({ environment, projectId, allowProduction, confirm }) {
  const target = getEnvironment(environment, projectId);
  if (
    environment === 'production' &&
    (allowProduction !== true ||
      confirm !== `production:${target.projectId}:${target.functionName}`)
  ) {
    throw new Error(
      'Production plan requires --allow-production and the exact target-bound --confirm token',
    );
  }
  const args = [
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
    `--source=${__dirname}`,
  ];
  return { environment, ...target, generation: 1, runtime: 'nodejs22', args };
}

function parseArgs(args) {
  const options = {};
  for (const arg of args) {
    if (arg === '--allow-production' && options.allowProduction === undefined) {
      options.allowProduction = true;
    } else {
      const match = /^--(environment|project|confirm)=(.+)$/.exec(arg);
      if (!match) throw new Error(`Unknown or empty argument: ${arg}`);
      const key = match[1] === 'project' ? 'projectId' : match[1];
      if (options[key] !== undefined)
        throw new Error(`Duplicate argument: ${arg}`);
      options[key] = match[2];
    }
  }
  return options;
}

if (require.main === module) {
  try {
    const plan = deploymentPlan(parseArgs(process.argv.slice(2)));
    // Only print a reviewable command. This tool never executes gcloud.
    const quote = (value) => `'${value.replaceAll("'", "'\"'\"'")}'`;
    console.log(
      `# PLAN ONLY: ${plan.environment} / ${plan.projectId} / ${plan.functionName}`,
    );
    console.log(['gcloud', ...plan.args].map(quote).join(' '));
  } catch (error) {
    console.error(error.message);
    process.exitCode = 1;
  }
}

module.exports = { deploymentPlan, parseArgs };
