"""Target tests must run without runner credentials, command files, or host access."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

from test_ci_workflows import ROOT, load

REPOSITORY_KINDS = ('base-image', 'node-policy', 'docs', 'contracts')
TERRAFORM_KINDS = ('terraform-static', 'terraform-tests')


class ValidationSandboxTest(unittest.TestCase):
    def jobs(self):
        # Only Terraform validation checks out a selected commit (terraform.yml).
        # Repository checks always validate the CI run's own commit on the runner.
        yield load(ROOT / '.github/workflows/ci-gcp-terraform.yml')['jobs']['validate']

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
                'print("| injected row | PASS |", flush=True)\n'
                'sys.exit(17 if kind in json.loads(os.environ["FAIL_KINDS"]) else 0)\n')
            fake.chmod(0o755)
            output = root / 'args'
            summary = root / 'summary'
            summary.write_text('host summary sentinel\n')
            result = subprocess.run(['bash', '--noprofile', '--norc', '-eo', 'pipefail', '-c', job['steps'][-1]['run']],
                env=os.environ | {'PATH': directory + os.pathsep + os.environ.get('PATH', os.defpath),
                    'GITHUB_WORKSPACE': directory, 'VALIDATION_SHA': 'a' * 40,
                    'ACTIONS_RUNTIME_TOKEN': 'RUNNER_TOKEN_CANARY', 'GITHUB_TOKEN': 'TOKEN_CANARY',
                    'GITHUB_ENV': 'HOST_COMMAND_FILE_CANARY', 'ARGS_OUT': str(output),
                    'GITHUB_STEP_SUMMARY': str(summary),
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
                if not args[-1].startswith('terraform-'):
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
            expected_summary = 'host summary sentinel\n'
            if job['name'] == 'Repository Validation':
                expected_summary += '### Repository validation\n\n| Validation | Result |\n| --- | --- |\n'
                for kind in REPOSITORY_KINDS:
                    status = 'FAIL (exit 17)' if kind in fail_kinds else 'PASS'
                    expected_summary += f'| {kind} | {status} |\n'
            self.assertEqual(summary.read_text(), expected_summary)
            self.assertNotIn('injected row', summary.read_text())
            return result, calls

    def test_target_cannot_receive_runner_tokens_or_writable_host_mounts_and_failures_propagate(self):
        for job in self.jobs():
            kinds = TERRAFORM_KINDS if job['name'] == 'GCP Terraform Validate' else REPOSITORY_KINDS
            for failures in ((), kinds):
                with self.subTest(job=job['name'], failures=failures):
                    result, calls = self.run_docker_step(job, failures)
                    self.assertEqual([args[-1] for args in calls], list(kinds))
                    self.assertEqual(result.returncode, 1 if failures else 0)

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
        self.assertIn('terraform -chdir=/source fmt -check -recursive infra/gcp', entrypoint)
        self.assertIn('terraform -chdir="/source/infra/gcp/environments/$env" validate', entrypoint)
        # Exercise the real SHA check with temporary paths instead of container
        # mount points. No target validation may start for a mismatched SHA.
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, work = root / 'source', root / 'work'
            source.mkdir()
            subprocess.run(['git', 'init', '-q', str(source)], check=True)
            subprocess.run(['git', '-C', str(source), '-c', 'user.name=CI test',
                            '-c', 'user.email=ci-test@example.invalid', 'commit', '--allow-empty', '-qm', 'fixture'], check=True)
            for kind in (*TERRAFORM_KINDS, *REPOSITORY_KINDS):
                script = entrypoint.replace('/source/', str(source) + '/').replace('/work/', str(work / kind) + '/')
                result = subprocess.run(['bash', '-s', '--', kind], input=script,
                    env=os.environ | {'HOME': str(root / 'home'), 'RUNNER_TEMP': str(root / 'tmp'),
                                      'CI_SOURCE_SHA': '0' * 40}, capture_output=True, text=True, timeout=30)
                self.assertEqual(result.returncode, 1)
                self.assertIn('Source SHA mismatch', result.stderr)


    def entrypoint_kind(self, kind):
        entrypoint = (ROOT / '.github/ci-validation/validate.sh').read_text()
        return entrypoint.split('\n  ' + kind + ')\n', 1)[1].split('\n    ;;', 1)[0]

    def test_fixed_terraform_checks_run_first_in_their_own_container_and_execute_no_target_code(self):
        # Regression: target tests used to run before fmt/validate in the same
        # writable copy, so a test could delete an invalid .tf and pass validation.
        static = self.entrypoint_kind('terraform-static')
        tests = self.entrypoint_kind('terraform-tests')
        for command in ('terraform -chdir=/source fmt -check -recursive infra/gcp',
                        'terraform -chdir="/source/infra/gcp/environments/$env" validate',
                        'providers lock', 'git diff --exit-code', 'cmp /source/infra/gcp/environments/dev/.terraform.lock.hcl'):
            self.assertIn(command, static)
            self.assertNotIn(command, tests)
        # Providers only from the HashiCorp registry namespace; no implied local mirror.
        self.assertIn('include = ["registry.terraform.io/hashicorp/*"]', static)
        self.assertIn('export TF_CLI_CONFIG_FILE=/work/terraformrc', static)
        self.assertNotIn('filesystem_mirror', static)
        # The only provider-starting step reads the read-only source and runs last.
        self.assertLess(static.index('providers lock'), static.index(' init -backend=false'))
        self.assertLess(static.index('cmp /source/'), static.index(' init -backend=false'))
        self.assertEqual(sum(line.rstrip().endswith(' validate') for line in static.splitlines()), 2)  # Core and MyPage roots
        self.assertEqual(static.rstrip().splitlines()[-1].strip(), 'done')
        # MyPage fixed checks also run on the read-only source after the lock checks.
        self.assertIn('terraform -chdir=/source fmt -check -recursive "infra/$layer"', static)
        for line in static.splitlines():
            if ' init ' in line or line.rstrip().endswith(' validate'):
                self.assertRegex(line, r'-chdir="/source/infra/(gcp|\$layer)/environments/\$env"')
                self.assertIn('TF_DATA_DIR="/work/', line)
        # The tests container prepares its own provider mirror for the native suites.
        self.assertIn('terraform -chdir="infra/gcp/environments/$env" init -backend=false -input=false -lockfile=readonly', tests)
        for target_code in ('python', 'node', 'bash ', '.github/scripts', 'infra/gcp/tests', 'infra/gcp/scripts'):
            self.assertNotIn(target_code, static)
        run = load(ROOT / '.github/workflows/ci-gcp-terraform.yml')['jobs']['validate']['steps'][-1]['run']
        self.assertIn('for kind in terraform-static terraform-tests; do', run)
        self.assertEqual(run.count('docker run '), 1)

    def test_fresh_container_static_checks_reject_invalid_hcl_regardless_of_a_previous_test_run(self):
        entrypoint = (ROOT / '.github/ci-validation/validate.sh').read_text()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / 'source'
            (source / 'infra/gcp').mkdir(parents=True)
            (source / 'infra/gcp/invalid.tf').write_text('resource "terraform_data" "x" {\n  input =\n}\n')
            subprocess.run(['git', 'init', '-q', str(source)], check=True)
            subprocess.run(['git', '-C', str(source), 'add', '.'], check=True)
            subprocess.run(['git', '-C', str(source), '-c', 'user.name=CI test', '-c', 'user.email=ci-test@example.invalid',
                            'commit', '-qm', 'invalid HCL'], check=True)
            sha = subprocess.run(['git', '-C', str(source), 'rev-parse', 'HEAD'], check=True,
                                 capture_output=True, text=True).stdout.strip()
            # A previous container's writable copy (where a test removed the file)
            # is never reused: each kind copies the read-only source again.
            previous = root / 'work-tests/repository'
            previous.mkdir(parents=True)
            script = entrypoint.replace('/source', str(source)).replace('/work/', str(root / 'work-static') + '/')
            result = subprocess.run(['bash', '-s', '--', 'terraform-static'], input=script,
                env=os.environ | {'HOME': str(root / 'home'), 'RUNNER_TEMP': str(root / 'tmp'), 'CI_SOURCE_SHA': sha,
                                  'CHECKPOINT_DISABLE': '1'},
                capture_output=True, text=True, timeout=60)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn('Source SHA mismatch', result.stderr)
            self.assertIn('Invalid expression', result.stdout + result.stderr)
            self.assertTrue((root / 'work-static/repository/infra/gcp/invalid.tf').exists())


if __name__ == '__main__':
    unittest.main()
