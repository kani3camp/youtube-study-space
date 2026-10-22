"""Security validation must block selected PRs without deploy credentials."""
import copy
import importlib.util
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('security_yaml', ROOT / '.github/codeql/validate-source.py')
source = importlib.util.module_from_spec(spec)
spec.loader.exec_module(source)


def validate(frontend, system):
    audit = [s for s in frontend['jobs']['mypage-check']['steps']
             if s.get('run', '').startswith('pnpm audit')]
    source.require(len(audit) == 1 and audit[0]['run'] == 'pnpm audit --audit-level=high',
                   'Audit runtime and development dependencies, blocking High and Critical')
    checks = system['jobs']['go-vulnerability-check']['steps']
    go = [s for s in checks if s.get('run', '').startswith('govulncheck')]
    source.require(len(go) == 1 and go[0]['run'] == 'govulncheck -test ./...',
                   'Check every Go package and test for reachable vulnerabilities')
    scans = [s for s in system['jobs']['mypage-container']['steps']
             if s.get('uses', '').startswith('aquasecurity/trivy-action@')]
    source.require(len(scans) == 1, 'Scan the built deployment image')
    scan = scans[0]
    source.require(len(scan['uses'].split('@')[1]) == 40, 'Pin the scanner action')
    options = scan['with']
    source.require(options['image-ref'] == 'mypage-api:ci' and options['scanners'] == 'vuln'
                   and set(options['vuln-type'].split(',')) == {'os', 'library'}
                   and set(options['severity'].split(',')) == {'HIGH', 'CRITICAL'}
                   and options['ignore-unfixed'] is False and options['exit-code'] == '1',
                   'Reject High/Critical OS and library findings including unpatched ones')
    for step in audit + go + scans:
        source.require('if' not in step and 'continue-on-error' not in step,
                       'Security failures cannot be skipped or ignored')


class SecurityContractsTest(unittest.TestCase):
    def setUp(self):
        self.frontend = source.load_yaml(ROOT / '.github/workflows/ci-frontend.yml')
        self.system = source.load_yaml(ROOT / '.github/workflows/ci-system.yml')

    def test_dependency_and_built_image_findings_block_ci(self):
        validate(self.frontend, self.system)

    def test_fail_open_scanner_configuration_is_rejected(self):
        for option, value in [('exit-code', '0'), ('ignore-unfixed', True),
                              ('vuln-type', 'os'), ('severity', 'CRITICAL')]:
            with self.subTest(option=option), self.assertRaises(ValueError):
                system = copy.deepcopy(self.system)
                scan = system['jobs']['mypage-container']['steps'][-1]
                scan['with'][option] = value
                validate(self.frontend, system)
        with self.assertRaises(ValueError):
            system = copy.deepcopy(self.system)
            system['jobs']['mypage-container']['steps'][-1]['continue-on-error'] = True
            validate(self.frontend, system)
        with self.assertRaises(ValueError):
            system = copy.deepcopy(self.system)
            system['jobs']['mypage-container']['steps'].pop()
            validate(self.frontend, system)

    def test_every_node_setup_uses_the_canonical_patch_version(self):
        self.assertEqual((ROOT / '.node-version').read_text(), (ROOT / '.nvmrc').read_text())
        for path in (ROOT / '.github/workflows').glob('*.yml'):
            for job in source.load_yaml(path)['jobs'].values():
                for step in job.get('steps', []):
                    if step.get('uses', '').startswith('actions/setup-node@'):
                        with self.subTest(workflow=path.name, step=step.get('name')):
                            self.assertEqual(step['with']['node-version-file'], '.node-version')
                            self.assertNotIn('node-version', step['with'])

    def test_dependabot_covers_the_unreleased_mypage_stack(self):
        updates = source.load_yaml(ROOT / '.github/dependabot.yml')['updates']
        required = {('npm', '/mypage'), ('gomod', '/system'), ('docker', '/system')}
        found = {(u['package-ecosystem'], u.get('directory')) for u in updates
                 if u.get('target-branch') == 'integration/mypage-canon-20261006'
                 and u['schedule']['interval'] == 'daily'}
        self.assertTrue(required <= found)


if __name__ == '__main__':
    unittest.main()
