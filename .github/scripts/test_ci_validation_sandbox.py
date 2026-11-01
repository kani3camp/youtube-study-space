"""Target tests must run without runner credentials, command files, or host access."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

from test_ci_workflows import ROOT, load

REPOSITORY_KINDS = ('base-image', 'node-policy', 'docs', 'contracts')


class ValidationSandboxTest(unittest.TestCase):
    def jobs(self):
        yield load(ROOT / '.github/workflows/ci-gcp-terraform.yml')['jobs']['validate']
        yield from load(ROOT / '.github/workflows/ci-repository.yml')['jobs'].values()

    def repository_job(self):
        jobs = load(ROOT / '.github/workflows/ci-repository.yml')['jobs']
        self.assertEqual(set(jobs), {'validate'})
        return jobs['validate']

    def test_only_isolated_docker_execution_follows_target_checkout(self):
        for job in self.jobs():
            steps = job['steps']
            self.assertEqual(len(steps), 4)
            self.assertEqual(steps[0]['with'], {'ref': '${{ github.sha }}', 'persist-credentials': False})
            self.assertEqual(steps[1]['uses'], './.github/actions/prepare-ci-validation')
            self.assertEqual(steps[2]['with'], {'ref': '${{ inputs.source_sha }}', 'path': '.ci-source',
                                              'persist-credentials': False})
            self.assertEqual(steps[3]['env'], {'VALIDATION_SHA': '${{ inputs.source_sha }}'})
            self.assertNotIn('uses', steps[3])
            self.assertNotIn('if', steps[3])
            self.assertNotIn('continue-on-error', steps[3])

    def test_repository_builds_image_once_before_target_checkout(self):
        job = self.repository_job()
        self.assertEqual(job['timeout-minutes'], 10)
        action = load(ROOT / '.github/actions/prepare-ci-validation/action.yml')
        self.assertEqual(len(action['runs']['steps']), 1)
        self.assertEqual(action['runs']['steps'][0]['run'].count('docker build '), 1)
        self.assertEqual(sum(step.get('uses') == './.github/actions/prepare-ci-validation'
                             for step in job['steps']), 1)

    def run_docker_step(self, job, fail_kinds):
        """Execute the actual workflow shell; Docker is a recording failure/log adversary."""
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            fake = root / 'docker'
            fake.write_text('#!/usr/bin/env python3\nimport json,os,sys\n'
                'with open(os.environ["ARGS_OUT"], "a") as output:\n'
                '    output.write(json.dumps(sys.argv[1:]) + "\\n")\n'
                'kind = sys.argv[-1]\n'
                'print("::error::untrusted-" + kind, flush=True)\n'
                'print("::stop-commands::attacker", flush=True)\n'
                'print("::attacker::", flush=True)\n'
                'print("::warning::stderr-" + kind, file=sys.stderr, flush=True)\n'
                'sys.exit(17 if kind in json.loads(os.environ["FAIL_KINDS"]) else 0)\n')
            fake.chmod(0o755)
            output = root / 'args'
            result = subprocess.run(['bash', '--noprofile', '--norc', '-eo', 'pipefail', '-c', job['steps'][-1]['run']],
                env=os.environ | {'PATH': directory + os.pathsep + os.environ.get('PATH', os.defpath),
                    'GITHUB_WORKSPACE': directory, 'VALIDATION_SHA': 'a' * 40,
                    'ACTIONS_RUNTIME_TOKEN': 'RUNNER_TOKEN_CANARY', 'GITHUB_TOKEN': 'TOKEN_CANARY',
                    'GITHUB_ENV': 'HOST_COMMAND_FILE_CANARY', 'ARGS_OUT': str(output),
                    'FAIL_KINDS': json.dumps(fail_kinds)},
                stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=30)
            calls = [json.loads(line) for line in output.read_text().splitlines()]
            for args in calls:
                self.assertEqual([args[i + 1] for i, arg in enumerate(args) if arg == '--env'],
                                 ['CI_SOURCE_SHA=' + 'a' * 40])
                self.assertEqual([args[i + 1] for i, arg in enumerate(args) if arg == '--mount'],
                                 ['type=bind,src=' + directory + '/.ci-source,dst=/source,readonly'])
                self.assertEqual([args[i + 1] for i, arg in enumerate(args) if arg == '--tmpfs'],
                                 ['/tmp:rw,exec,mode=1777', '/work:rw,exec,uid=10001,gid=10001,mode=0700'])
                for required in ('--read-only', '--cap-drop=ALL', '--security-opt=no-new-privileges', '--rm', '--init'):
                    self.assertIn(required, args)
                for forbidden in ('--privileged', '--env-file', '--pid=host', '--network=host', '-v'):
                    self.assertNotIn(forbidden, args)
                self.assertNotIn('docker.sock', ' '.join(args))
                self.assertNotIn('CANARY', ' '.join(args))
                self.assertEqual(args[-2], 'yss-ci-validation:local')
                if args[-1] != 'terraform':
                    self.assertIn('--network=none', args)
            # Model Actions' stop/resume protocol. Every stdout AND stderr line
            # from every container must stay inside the unpredictable envelope.
            lines = result.stdout.splitlines()
            self.assertTrue(lines[0].startswith('::stop-commands::'))
            marker = lines[0].removeprefix('::stop-commands::')
            self.assertRegex(marker, r'^[0-9a-f]{64}$')
            self.assertEqual(lines[-1], '::' + marker + '::')
            self.assertEqual(lines.count(lines[-1]), 1)
            for args in calls:
                kind = args[-1]
                self.assertIn('::error::untrusted-' + kind, lines[1:-1])
                self.assertIn('::warning::stderr-' + kind, lines[1:-1])
                self.assertNotIn(marker, ' '.join(args))
            return result, calls

    def test_target_cannot_receive_runner_tokens_or_writable_host_mounts_and_failures_propagate(self):
        for job in self.jobs():
            kinds = ('terraform',) if job['name'] == 'GCP Terraform Validate' else REPOSITORY_KINDS
            for failures in ((), kinds):
                with self.subTest(job=job['name'], failures=failures):
                    result, calls = self.run_docker_step(job, failures)
                    self.assertEqual([args[-1] for args in calls], list(kinds))
                    self.assertEqual(result.returncode, (17 if kinds == ('terraform',) else 1) if failures else 0)

    def test_each_repository_failure_runs_all_four_cases_and_fails_ci_gate(self):
        import test_ci_results
        for kind in REPOSITORY_KINDS:
            with self.subTest(failed=kind):
                result, calls = self.run_docker_step(self.repository_job(), (kind,))
                self.assertEqual([args[-1] for args in calls], list(REPOSITORY_KINDS))
                self.assertEqual(result.returncode, 1)
                for case in REPOSITORY_KINDS:
                    self.assertIn(case + (': FAIL (exit 17)' if case == kind else ': PASS'), result.stdout)
                needs = test_ci_results.GateResultsTest().fixture()
                needs['repository']['result'] = 'failure' if result.returncode else 'success'
                gate = subprocess.run(['python3', str(test_ci_results.SCRIPT)],
                    env=os.environ | {'CI_NEEDS': json.dumps(needs)}, capture_output=True, text=True, timeout=30)
                self.assertNotEqual(gate.returncode, 0, gate.stdout)

    def test_container_uses_nonroot_user_and_rejects_a_different_commit_before_tests(self):
        dockerfile = (ROOT / '.github/ci-validation/Dockerfile').read_text()
        self.assertIn('USER 10001:10001', dockerfile)
        entrypoint = (ROOT / '.github/ci-validation/validate.sh').read_text()
        self.assertLess(entrypoint.index('Source SHA mismatch'), entrypoint.index('case "$1"'))
        self.assertIn('cp -R /source/. /work/repository/', entrypoint)
        self.assertIn('terraform fmt -check -recursive infra/gcp', entrypoint)
        self.assertIn('terraform -chdir="infra/gcp/environments/$env" validate', entrypoint)
        # Exercise the real SHA check with temporary paths instead of container
        # mount points. No target validation may start for a mismatched SHA.
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, work = root / 'source', root / 'work'
            source.mkdir()
            subprocess.run(['git', 'init', '-q', str(source)], check=True)
            subprocess.run(['git', '-C', str(source), '-c', 'user.name=CI test',
                            '-c', 'user.email=ci-test@example.invalid', 'commit', '--allow-empty', '-qm', 'fixture'], check=True)
            for kind in ('terraform', *REPOSITORY_KINDS):
                script = entrypoint.replace('/source/', str(source) + '/').replace('/work/', str(work / kind) + '/')
                result = subprocess.run(['bash', '-s', '--', kind], input=script,
                    env=os.environ | {'HOME': str(root / 'home'), 'RUNNER_TEMP': str(root / 'tmp'),
                                      'CI_SOURCE_SHA': '0' * 40}, capture_output=True, text=True, timeout=30)
                self.assertEqual(result.returncode, 1)
                self.assertIn('Source SHA mismatch', result.stderr)


if __name__ == '__main__':
    unittest.main()
