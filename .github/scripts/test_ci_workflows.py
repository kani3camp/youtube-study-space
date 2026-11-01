"""Contracts for credentialless modular PR/manual validation. Terraform Plan/Apply is terraform.yml."""
import copy
import importlib.util
import itertools
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location('codeql_source', ROOT / '.github/codeql/validate-source.py')
yaml_source = importlib.util.module_from_spec(spec)
spec.loader.exec_module(yaml_source)
load = yaml_source.load_yaml
require = yaml_source.require

MODULES = {
    'system': {'check-i18n-generate': 'system', 'go-lint': 'system', 'system-go-test': 'system',
               'system-firestore-emulator-test': 'firestore_integration'},
    'frontend': {'biome-lint': 'youtube_monitor', 'youtube-monitor-test': 'youtube_monitor',
                 'youtube-monitor-build': 'youtube_monitor', 'docs-site-check': 'docs_site',
                 'docs-site-build': 'docs_site'},
    'tools': {'formal-spec': 'formal_spec', 'room-image-prompt-go-lint': 'room_image_prompt',
              'room-image-prompt-go-test': 'room_image_prompt',
              'menu-image-generator-lint-build': 'menu_image_generator',
              'video-maker-simulator-check': 'video_maker_simulator',
              'figma-plugin-room-layout-analyzer-build': 'figma_plugin'},
    'infra': {'aws-cdk-test': 'aws_cdk'},
    'repository': {'validate': None},
}
NODE = {'youtube_monitor', 'docs_site', 'menu_image_generator', 'video_maker_simulator',
        'figma_plugin', 'aws_cdk'}
GROUPS = ('system', 'room_image_prompt', 'menu_image_generator', 'video_maker_simulator',
          'figma_plugin', 'youtube_monitor', 'docs_site', 'aws_cdk', 'gcp_terraform',
          'node_projects', 'firestore_integration', 'formal_spec', 'all')
DEPENDENCIES = {'changes', *MODULES, 'gcp-terraform-validate'}
TERRAFORM_MODULE = './.github/workflows/ci-gcp-terraform.yml'


def selected(expression, flags, inputs=None):
    """Interpret only the boolean disjunction grammar allowed by this contract."""
    expression = expression.removeprefix('${{').removesuffix('}}').strip()
    terms = []
    for term in expression.split(' || '):
        match = re.fullmatch(r"needs.changes.outputs.(\w+) == 'true'", term)
        if match:
            terms.append(flags[match[1]])
        elif re.fullmatch(r'inputs.\w+', term) and inputs is not None:
            terms.append(inputs[term.split('.')[1]])
        else:
            raise ValueError('Unsupported selection expression: ' + expression)
    return any(terms)


def validate(caller, modules, terraform):
    jobs = caller['jobs']
    require(caller['name'] == 'CI', 'CI workflow name is stable')
    require(set(caller['on']) == {'pull_request', 'workflow_dispatch'} and caller['on']['pull_request'] is None
            and caller['on']['workflow_dispatch'] is None,
            'All pull requests use the orchestrator; manual runs take no operational input')
    require(caller['permissions'] == {'contents': 'read'}, 'Read-only default permissions')
    require(set(jobs) == DEPENDENCIES | {'ci-gate'}, 'Every caller job is accounted for')
    gate = jobs['ci-gate']
    require(gate['name'] == 'CI Gate' and gate['if'] == 'always()', 'Stable always-running gate')
    require(set(gate['needs']) == DEPENDENCIES, 'Gate depends on every quality module and detection')
    check = gate['steps'][-1]
    require(check['run'] == 'python3 .github/scripts/check-ci-results.py' and 'if' not in check,
            'Execute the tested result gate unconditionally')
    require(check['env'] == {'CI_NEEDS': '${{ toJSON(needs) }}'},
            'Gate receives real validation results')
    call = jobs['gcp-terraform-validate']
    require(set(call) == {'name', 'needs', 'if', 'uses', 'with'} and call['uses'] == TERRAFORM_MODULE
            and call['needs'] == 'changes' and call['with'] == {'source_sha': '${{ github.sha }}'},
            'Terraform validation is the shared credentialless module on this commit')
    require(terraform['on'] == {'workflow_call': {'inputs': {'source_sha': {
                'description': 'Exact commit to validate, pinned by the caller', 'required': True, 'type': 'string'}}}}
            and terraform['permissions'] == {'contents': 'read'}
            and set(terraform['jobs']) == {'validate'}
            and terraform['jobs']['validate']['name'] == 'GCP Terraform Validate',
            'Typed read-only Terraform validation module')
    require(jobs['changes']['outputs'] == {g: '${{ steps.detect.outputs.' + g + ' }}' for g in GROUPS},
            'All detection outputs reach callers')
    require(jobs['changes']['steps'][-1]['run'] == 'bash .github/scripts/detect-ci-paths.sh', 'Use canonical detector')
    for name, members in MODULES.items():
        call = jobs[name]
        expected_keys = {'name', 'uses', 'needs', 'with'} if name == 'repository' else {'name', 'uses', 'needs', 'if'}
        if name not in ('repository', 'infra'):
            expected_keys.add('with')
        require(set(call) == expected_keys, 'Bounded quality caller configuration')
        require(call['uses'] == f'./.github/workflows/ci-{name}.yml', 'Use same-tree quality modules')
        require(call['needs'] == 'changes', 'Selected calls depend on detection')
        module = modules[name]
        require(set(module) == {'name', 'on', 'permissions', 'jobs'}, 'No additional quality triggers/configuration')
        inputs = {} if name in ('repository', 'infra') else {
            g: {'required': True, 'type': 'boolean'} for g in set(members.values())}
        if name == 'repository':
            inputs = {'source_sha': {'description': 'Exact commit to validate, pinned by the CI controller',
                                     'required': True, 'type': 'string'}}
            require(call['with'] == {'source_sha': '${{ github.sha }}'},
                    'Repository validation checks this commit')
        require(module['on'] == {'workflow_call': {'inputs': inputs} if inputs else None}, 'Typed call-only modules')
        require(module['permissions'] == {'contents': 'read'}, 'Quality modules are read-only')
        require(set(module['jobs']) == set(members), 'Every existing validation remains in its domain')
        for job, group in members.items():
            leaf = module['jobs'][job]
            require('needs' not in leaf and 'runs-on' in leaf and leaf.get('timeout-minutes', 0) > 0,
                    'Independent bounded leaf validators')
            if name in ('repository', 'infra'):
                require('if' not in leaf, 'Selected leaf cannot skip')
            else:
                require(leaf.get('if') == '${{ inputs.' + group + ' }}', 'Leaf uses its typed domain selection')
    for module in (*modules.values(), terraform):
        for key, value in yaml_source.walk(module):
            require(key not in ('secrets', 'environment', 'continue-on-error', 'permissions')
                    or (key == 'permissions' and value == {'contents': 'read'}), 'No elevated/ignored quality execution')
    for key, value in yaml_source.walk(caller):
        require(key not in ('continue-on-error', 'secrets', 'environment'), 'Caller failures cannot be ignored; no credentials')
        require(key != 'permissions' or value == {'contents': 'read'}, 'CI never grants OIDC or write permissions')
        require(not (isinstance(value, str) and 'gcp-terraform-authenticated.yml' in value),
                'CI never calls authenticated Terraform')
    require(caller['concurrency'] == {
        'group': '${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}',
        'cancel-in-progress': True},
        'Superseded validation runs are cancelled')


class WorkflowContractsTest(unittest.TestCase):
    def setUp(self):
        self.caller = load(ROOT / '.github/workflows/ci.yml')
        self.modules = {name: load(ROOT / f'.github/workflows/ci-{name}.yml') for name in MODULES}
        self.terraform = load(ROOT / '.github/workflows/ci-gcp-terraform.yml')

    def test_workflow_structure_and_credentialless_contract(self):
        validate(self.caller, self.modules, self.terraform)

    def test_manual_runs_revalidate_every_domain(self):
        import os
        import subprocess
        result = subprocess.run(['bash', '.github/scripts/detect-ci-paths.sh'], cwd=ROOT,
            env={'PATH': os.environ.get('PATH', os.defpath), 'GITHUB_EVENT_NAME': 'workflow_dispatch'},
            capture_output=True, text=True, check=True)
        outputs = dict(line.split('=', 1) for line in result.stdout.splitlines() if '=' in line)
        self.assertEqual(outputs['all'], 'true')

    def test_all_8192_group_combinations_preserve_leaf_selection_and_gate_policy(self):
        import test_ci_results
        for values in itertools.product((False, True), repeat=len(GROUPS)):
            flags = dict(zip(GROUPS, values))
            needs = {name: {'result': 'skipped'} for name in DEPENDENCIES}
            needs['changes'] = {'result': 'success', 'outputs': {g: str(v).lower() for g, v in flags.items()}}
            needs['repository']['result'] = 'success'
            for name, members in MODULES.items():
                call = self.caller['jobs'][name]
                called = name == 'repository' or selected(call['if'], flags)
                inputs = {key: selected(expr, flags) for key, expr in call.get('with', {}).items() if key != 'source_sha'}
                expected_called = name == 'repository'
                for job, group in members.items():
                    expected = group is None or flags['all'] or flags[group] or (group in NODE and flags['node_projects'])
                    expected_called |= expected
                    leaf = self.modules[name]['jobs'][job]
                    actual = called and ('if' not in leaf or selected(leaf['if'], flags, inputs))
                    self.assertEqual(actual, expected, (flags, name, job))
                self.assertEqual(called, expected_called, (flags, name))
                needs[name]['result'] = 'success' if called else 'skipped'
            terraform = selected(self.caller['jobs']['gcp-terraform-validate']['if'], flags)
            self.assertEqual(terraform, flags['gcp_terraform'] or flags['all'])
            needs['gcp-terraform-validate']['result'] = 'success' if terraform else 'skipped'
            test_ci_results.gate.validate(needs)

    def test_required_display_names_are_unique_across_all_workflows(self):
        names = [job.get('name') for path in (ROOT / '.github/workflows').glob('*.yml')
                 for job in load(path)['jobs'].values()]
        for name in ('CI Gate', 'GCP Terraform Validate'):
            self.assertEqual(names.count(name), 1, name)

    def test_contracts_execute_on_every_ci_run(self):
        checks = self.modules['repository']['jobs']['validate']['steps']
        commands = '\n'.join(step.get('run', '') for step in checks)
        self.assertIn('for kind in base-image node-policy docs contracts; do', commands)
        self.assertIn('yss-ci-validation:local "$kind"', commands)
        self.assertEqual(sum(step.get('uses') == './.github/actions/prepare-ci-validation' for step in checks), 1)
        isolated = (ROOT / '.github/ci-validation/validate.sh').read_text()
        self.assertIn("python -m unittest discover -s .github/scripts -p 'test_ci_*.py' -v", isolated)
        self.assertIn('.github/workflows/ci-*.yml', isolated)
        self.assertFalse(any('if' in step or 'continue-on-error' in step for step in checks))

    def test_rejects_regressions_in_gate_selection_permissions_and_operational_caller(self):
        mutations = [
            lambda c, m, t: c['on'].update(workflow_dispatch={'inputs': {'execution_mode': {'type': 'choice'}}}),
            lambda c, m, t: c['jobs'].update({'gcp-terraform-authenticated': {'uses': './.github/workflows/gcp-terraform-authenticated.yml'}}),
            lambda c, m, t: c['jobs']['gcp-terraform-validate'].update(permissions={'contents': 'read', 'id-token': 'write'}),
            lambda c, m, t: c['jobs']['gcp-terraform-validate'].update(secrets='inherit'),
            lambda c, m, t: c['jobs']['gcp-terraform-validate']['with'].update(source_sha='${{ inputs.sha }}'),
            lambda c, m, t: c['jobs']['ci-gate']['needs'].remove('changes'),
            lambda c, m, t: c['jobs']['ci-gate']['needs'].remove('gcp-terraform-validate'),
            lambda c, m, t: c['jobs']['ci-gate'].update({'if': 'success()'}),
            lambda c, m, t: c['jobs']['system'].update(secrets='inherit'),
            lambda c, m, t: c.update(concurrency={'group': 'x', 'cancel-in-progress': False}),
            lambda c, m, t: m['system']['jobs']['go-lint'].update({'continue-on-error': True}),
            lambda c, m, t: m['frontend']['jobs']['youtube-monitor-test'].update({'if': '${{ false }}'}),
            lambda c, m, t: m['infra'].update(permissions={'contents': 'read', 'id-token': 'write'}),
            lambda c, m, t: t['jobs']['validate'].update(environment='terraform-dev-plan'),
            lambda c, m, t: t.update(permissions={'contents': 'read', 'id-token': 'write'}),
            lambda c, m, t: c['jobs']['gcp-terraform-validate'].update(uses='./.github/workflows/ci-infra.yml'),
        ]
        for mutate in mutations:
            caller, modules, terraform = copy.deepcopy(self.caller), copy.deepcopy(self.modules), copy.deepcopy(self.terraform)
            mutate(caller, modules, terraform)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                validate(caller, modules, terraform)
