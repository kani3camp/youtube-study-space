"""Target tests must run without runner credentials, command files, or host access."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

from test_ci_workflows import ROOT, load


class ValidationSandboxTest(unittest.TestCase):
    def jobs(self):
        # Only Terraform validation checks out a selected commit (terraform.yml).
        # Repository checks always validate the CI run's own commit on the runner.
        yield load(ROOT / '.github/workflows/ci-gcp-terraform.yml')['jobs']['validate']

    def test_only_isolated_docker_execution_follows_target_checkout(self):
        for job in self.jobs():
            steps = job['steps']
            self.assertEqual(len(steps), 4)
            self.assertEqual(steps[0]['with'], {'ref': '${{ github.sha }}', 'persist-credentials': False})
            self.assertEqual(steps[1]['uses'], './.github/actions/prepare-ci-validation')
            self.assertEqual(steps[2]['with']['path'], '.ci-source')
            self.assertIs(steps[2]['with']['persist-credentials'], False)
            self.assertNotIn('uses', steps[3])
            self.assertNotIn('if', steps[3])
            self.assertNotIn('continue-on-error', steps[3])

    def test_target_cannot_receive_runner_tokens_or_writable_host_mounts_and_failures_propagate(self):
        for job in self.jobs():
            for status in (0, 17):
                with self.subTest(job=job['name'], status=status), tempfile.TemporaryDirectory() as directory:
                    root = Path(directory)
                    fake = root / 'docker'
                    fake.write_text('#!/usr/bin/env python3\nimport json,os,sys\n'
                        'from pathlib import Path\n'
                        'Path(os.environ["ARGS_OUT"]).write_text(json.dumps(sys.argv[1:]))\n'
                        'print("::error::untrusted log command")\n'
                        'sys.exit(int(os.environ["DOCKER_EXIT"]))\n')
                    fake.chmod(0o755)
                    output = root / 'args'
                    result = subprocess.run(['bash', '-e', '-c', job['steps'][-1]['run']],
                        env=os.environ | {'PATH': directory + os.pathsep + os.environ.get('PATH', os.defpath),
                            'GITHUB_WORKSPACE': directory, 'VALIDATION_SHA': 'a' * 40,
                            'ACTIONS_RUNTIME_TOKEN': 'RUNNER_TOKEN_CANARY', 'GITHUB_TOKEN': 'TOKEN_CANARY',
                            'GITHUB_ENV': 'HOST_COMMAND_FILE_CANARY', 'ARGS_OUT': str(output), 'DOCKER_EXIT': str(status)},
                        capture_output=True, text=True)
                    self.assertEqual(result.returncode, status)
                    args = json.loads(output.read_text())
                    self.assertEqual([args[i + 1] for i, arg in enumerate(args) if arg == '--env'],
                                     ['CI_SOURCE_SHA=' + 'a' * 40])
                    self.assertEqual([args[i + 1] for i, arg in enumerate(args) if arg == '--mount'],
                                     ['type=bind,src=' + directory + '/.ci-source,dst=/source,readonly'])
                    for required in ('--read-only', '--cap-drop=ALL', '--security-opt=no-new-privileges', '--rm', '--init'):
                        self.assertIn(required, args)
                    for forbidden in ('--privileged', '--env-file', '--pid=host', '--network=host', '-v'):
                        self.assertNotIn(forbidden, args)
                    self.assertNotIn('docker.sock', ' '.join(args))
                    self.assertNotIn('CANARY', ' '.join(args))
                    if args[-1] != 'terraform':
                        self.assertIn('--network=none', args)
                    lines = result.stdout.splitlines()
                    self.assertTrue(lines[0].startswith('::stop-commands::'))
                    self.assertEqual(lines[-1], '::' + lines[0].removeprefix('::stop-commands::') + '::')

    def test_container_uses_nonroot_user_and_rejects_a_different_commit_before_tests(self):
        dockerfile = (ROOT / '.github/ci-validation/Dockerfile').read_text()
        self.assertIn('USER 10001:10001', dockerfile)
        entrypoint = (ROOT / '.github/ci-validation/validate.sh').read_text()
        self.assertLess(entrypoint.index('Source SHA mismatch'), entrypoint.index('case "$1"'))
        self.assertIn('cp -R /source/. /work/repository/', entrypoint)
        self.assertIn('terraform fmt -check -recursive infra/gcp', entrypoint)
        self.assertIn('terraform -chdir="infra/gcp/environments/$env" validate', entrypoint)


if __name__ == '__main__':
    unittest.main()
