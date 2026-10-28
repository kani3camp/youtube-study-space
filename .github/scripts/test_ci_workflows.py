"""Contracts for modular PR validation and the authenticated operations boundary."""
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
    'system': {'go-vulnerability-check': 'system', 'mypage-container': 'system',
               'check-i18n-generate': 'system', 'go-lint': 'system',
               'system-go-test': 'system', 'system-firestore-emulator-test': 'firestore_integration'},
    'frontend': {'biome-lint': 'youtube_monitor', 'youtube-monitor-test': 'youtube_monitor',
                 'youtube-monitor-build': 'youtube_monitor', 'mypage-check': 'mypage',
                 'docs-site-check': 'docs_site', 'docs-site-build': 'docs_site'},
    'tools': {'formal-spec': 'formal_spec', 'room-image-prompt-go-lint': 'room_image_prompt',
              'room-image-prompt-go-test': 'room_image_prompt',
              'menu-image-generator-lint-build': 'menu_image_generator',
              'video-maker-simulator-check': 'video_maker_simulator',
              'figma-plugin-room-layout-analyzer-build': 'figma_plugin'},
    'infra': {'aws-cdk-test': 'aws_cdk'},
    'repository': {'base-image-update-report-test': None, 'node-lts-ignore-expiry': None,
                   'documentation-references': None, 'ci-contracts': None},
}
NODE = {'youtube_monitor', 'mypage', 'docs_site', 'menu_image_generator',
        'video_maker_simulator', 'figma_plugin', 'aws_cdk'}
GROUPS = ('system', 'room_image_prompt', 'menu_image_generator', 'video_maker_simulator',
          'figma_plugin', 'youtube_monitor', 'mypage', 'docs_site', 'aws_cdk', 'gcp_terraform',
          'node_projects', 'firestore_integration', 'formal_spec', 'all')
DEPENDENCIES = {'changes', *MODULES, 'gcp-terraform-validate', 'operations'}
AUTH_SECRETS = ('GCP_TERRAFORM_NORMAL_SERVICE_ACCOUNT', 'GCP_TERRAFORM_NORMAL_WIF_PROVIDER',
                'AWS_TERRAFORM_BACKEND_ROLE_ARN', 'AWS_TERRAFORM_BACKEND_ROLE_ID',
                'AWS_TERRAFORM_STATE_BUCKET', 'AWS_TERRAFORM_STATE_ACCOUNT_ID',
                'GCP_TERRAFORM_WIF_PROVIDER', 'GCP_TERRAFORM_SERVICE_ACCOUNT',
                'GCP_TERRAFORM_MANAGED_INPUTS_JSON')
AUDIT_SECRETS = ('GCP_TERRAFORM_WIF_PROVIDER', 'GCP_USER_ACTIVITY_SCHEMA_AUDIT_SERVICE_ACCOUNT')
FOUNDATION_SECRETS = ('MYPAGE_STATE_BUCKET', 'MYPAGE_STATE_ACCOUNT', 'MYPAGE_BACKEND_ROLE_ARN',
                      'MYPAGE_BACKEND_ROLE_ID', 'MYPAGE_GCP_WIF_PROVIDER',
                      'MYPAGE_GCP_SERVICE_ACCOUNT', 'MYPAGE_PLAN_HMAC_KEY',
                      'MYPAGE_RUNTIME_SETTINGS_JSON')
OPERATION_TOGGLES = ('terraform_authenticated', 'gcp_user_activity_schema_audit',
                     'mypage_foundation', 'mypage_hosting', 'mypage_backend')
ALL_OPERATION_SECRETS = set(AUTH_SECRETS) | set(AUDIT_SECRETS) | set(FOUNDATION_SECRETS)


def selected(expression, flags, inputs=None):
    """Interpret only the boolean disjunction grammar allowed by quality routing."""
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


def validate(caller, modules):
    jobs = caller['jobs']
    require(caller['name'] == 'CI', 'CI workflow name is stable')
    require(set(caller['on']) == {'pull_request', 'workflow_dispatch'} and caller['on']['pull_request'] is None,
            'All pull requests use the orchestrator')
    require(caller['permissions'] == {'contents': 'read'}, 'Read-only default permissions')
    require(set(jobs) == DEPENDENCIES | {'ci-gate'}, 'Every caller job is accounted for')

    gate = jobs['ci-gate']
    require(gate['name'] == 'CI Gate' and gate['if'] == 'always()', 'Stable always-running gate')
    require(set(gate['needs']) == DEPENDENCIES, 'Gate depends on every quality module and operations')
    check = gate['steps'][-1]
    require(check['run'] == 'python3 .github/scripts/check-ci-results.py' and 'if' not in check,
            'Execute the tested result gate unconditionally')
    require(check['env'] == {
        'CI_NEEDS': '${{ toJSON(needs) }}',
        'CI_OPERATION_REQUESTED': "${{ github.event_name == 'workflow_dispatch' && (inputs.terraform_authenticated == true || inputs.gcp_user_activity_schema_audit == true || inputs.mypage_foundation == true || inputs.mypage_hosting == true || inputs.mypage_backend == true) }}",
    }, 'Gate receives aggregate operation selection')

    terraform = jobs['gcp-terraform-validate']
    require(set(terraform) == {'name', 'needs', 'if', 'uses'},
            'Terraform validation is a bounded reusable call')
    require(terraform['name'] == 'GCP Terraform'
            and terraform['uses'] == './.github/workflows/ci-gcp-terraform.yml',
            'Terraform validation uses its dedicated workflow')
    require(terraform['needs'] == 'changes', 'Terraform depends on detection')
    require(terraform['if']
            == "${{ needs.changes.outputs.gcp_terraform == 'true' || needs.changes.outputs.all == 'true' }}",
            'Terraform validation follows path selection')
    require(jobs['changes']['outputs'] == {
        g: '${{ steps.detect.outputs.' + g + ' }}' for g in GROUPS
    }, 'All detection outputs reach callers')
    detect = jobs['changes']['steps'][-1]
    require(detect['run'] == 'bash .github/scripts/detect-ci-paths.sh', 'Use canonical detector')
    require(detect['env']['CI_AUTHENTICATED_TERRAFORM_DISPATCH']
            == "${{ github.event_name == 'workflow_dispatch' && (inputs.terraform_authenticated == true || inputs.gcp_user_activity_schema_audit == true || inputs.mypage_foundation == true) }}",
            'Credentialed Terraform-family operations use bounded Terraform validation routing')

    for name, members in MODULES.items():
        call = jobs[name]
        expected_keys = {'name', 'uses'} if name == 'repository' else {'name', 'uses', 'needs', 'if'}
        if name not in ('repository', 'infra'):
            expected_keys.add('with')
        require(set(call) == expected_keys, 'Bounded quality caller configuration')
        require(call['uses'] == f'./.github/workflows/ci-{name}.yml', 'Use same-tree quality modules')
        if name != 'repository':
            require(call['needs'] == 'changes', 'Selected calls depend on detection')
        module = modules[name]
        require(set(module) == {'name', 'on', 'permissions', 'jobs'}, 'No additional quality triggers/configuration')
        inputs = {} if name in ('repository', 'infra') else {
            g: {'required': True, 'type': 'boolean'} for g in set(members.values())
        }
        require(module['on'] == {'workflow_call': {'inputs': inputs} if inputs else None},
                'Typed call-only modules')
        require(module['permissions'] == {'contents': 'read'}, 'Quality modules are read-only')
        require(set(module['jobs']) == set(members), 'Every existing validation remains in its domain')
        for job, group in members.items():
            leaf = module['jobs'][job]
            require('needs' not in leaf and 'runs-on' in leaf and leaf.get('timeout-minutes', 0) > 0,
                    'Independent bounded leaf validators')
            if name in ('repository', 'infra'):
                require('if' not in leaf, 'Selected leaf cannot skip')
            else:
                require(leaf.get('if') == '${{ inputs.' + group + ' }}',
                        'Leaf uses its typed domain selection')
        for key, value in yaml_source.walk(module):
            require(key not in ('secrets', 'environment', 'continue-on-error', 'permissions')
                    or (key == 'permissions' and value == {'contents': 'read'}),
                    'No elevated/ignored quality execution')

    operations = jobs['operations']
    require(set(operations) == {'name', 'needs', 'if', 'permissions', 'uses', 'with', 'secrets'},
            'Operations caller is isolated from quality jobs')
    require(operations['name'] == 'Authenticated Operations'
            and operations['uses'] == './.github/workflows/ci-operations.yml',
            'Operations use the call-only module')
    require(operations['needs'] == ['repository', 'frontend', 'system', 'gcp-terraform-validate'],
            'Operations receive all prerequisite results')
    require(operations['if']
            == "${{ always() && github.event_name == 'workflow_dispatch' && (inputs.terraform_authenticated == true || inputs.gcp_user_activity_schema_audit == true || inputs.mypage_foundation == true || inputs.mypage_hosting == true || inputs.mypage_backend == true) }}",
            'Operations are manual and aggregate every supported operation')
    require(operations['permissions'] == {'contents': 'read', 'id-token': 'write'},
            'Only the operations boundary receives OIDC from the top-level caller')
    expected_with = {
        'terraform_authenticated': '${{ inputs.terraform_authenticated }}',
        'terraform_target': '${{ inputs.terraform_target }}',
        'terraform_mode': '${{ inputs.terraform_mode }}',
        'terraform_expected_sha': '${{ inputs.terraform_expected_sha }}',
        'gcp_user_activity_schema_audit': '${{ inputs.gcp_user_activity_schema_audit }}',
        'mypage_foundation': '${{ inputs.mypage_foundation }}',
        'mypage_foundation_target': '${{ inputs.mypage_foundation_target }}',
        'mypage_foundation_mode': '${{ inputs.mypage_foundation_mode }}',
        'mypage_foundation_expected_sha': '${{ inputs.mypage_foundation_expected_sha }}',
        'mypage_hosting': '${{ inputs.mypage_hosting }}',
        'mypage_hosting_expected_sha': '${{ inputs.mypage_hosting_expected_sha }}',
        'mypage_backend': '${{ inputs.mypage_backend }}',
        'mypage_backend_expected_sha': '${{ inputs.mypage_backend_expected_sha }}',
        'repository_result': '${{ needs.repository.result }}',
        'frontend_result': '${{ needs.frontend.result }}',
        'system_result': '${{ needs.system.result }}',
        'gcp_terraform_validate_result': '${{ needs.gcp-terraform-validate.result }}',
    }
    require(operations['with'] == expected_with, 'Operations receive typed inputs and prerequisite results')
    require(operations['secrets'] == {
        key: '${{ secrets.' + key + ' }}' for key in ALL_OPERATION_SECRETS
    }, 'Operations receive only the explicit union of existing operational secrets')

    for key, value in yaml_source.walk(caller):
        require(key != 'continue-on-error', 'Caller failures cannot be ignored')

    dispatch = caller['on']['workflow_dispatch']['inputs']
    require(set(dispatch) == {
        'terraform_authenticated', 'terraform_target', 'terraform_mode', 'terraform_expected_sha',
        'gcp_user_activity_schema_audit', 'mypage_foundation', 'mypage_foundation_target',
        'mypage_foundation_mode', 'mypage_foundation_expected_sha', 'mypage_hosting',
        'mypage_hosting_expected_sha', 'mypage_backend', 'mypage_backend_expected_sha',
    }, 'No retired operational mode/input returns')
    for toggle in OPERATION_TOGGLES:
        require(dispatch[toggle]['type'] == 'boolean' and dispatch[toggle]['default'] is False
                and dispatch[toggle]['required'] is False, 'Operations remain opt-in')
    for key, default, options in (
        ('terraform_target', 'dev', ['dev', 'prod']),
        ('mypage_foundation_target', 'development', ['development', 'production']),
        ('mypage_foundation_mode', 'foundation-metadata-plan',
         ['foundation-metadata-plan', 'foundation-metadata-apply', 'foundation-plan',
          'foundation-apply', 'registry-plan', 'registry-apply', 'image-build',
          'maintenance-operator-plan', 'maintenance-operator-apply',
          'maintenance-upgrade-plan', 'maintenance-upgrade-apply',
          'runtime-private-plan', 'runtime-private-apply',
          'runtime-public-plan', 'runtime-public-apply']),
        ('terraform_mode', 'migration-plan',
         ['migration-plan', 'migration-apply', 'normal-plan', 'normal-apply', 'normal-e2e']),
    ):
        require(dispatch[key]['type'] == 'choice' and dispatch[key]['default'] == default
                and dispatch[key]['options'] == options and dispatch[key]['required'] is False,
                'Dispatch choices stay fixed')

    require(caller['concurrency'] == {
        'group': '${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}',
        'cancel-in-progress': "${{ !(github.event_name == 'workflow_dispatch' && (inputs.terraform_authenticated == true || inputs.gcp_user_activity_schema_audit == true || inputs.mypage_foundation == true || inputs.mypage_hosting == true || inputs.mypage_backend == true)) }}",
    }, 'Authenticated execution cannot be auto-cancelled')


class WorkflowContractsTest(unittest.TestCase):
    def setUp(self):
        self.caller = load(ROOT / '.github/workflows/ci.yml')
        self.modules = {name: load(ROOT / f'.github/workflows/ci-{name}.yml') for name in MODULES}

    def test_workflow_structure_and_authentication_contract(self):
        validate(self.caller, self.modules)

    def test_all_16384_group_combinations_preserve_leaf_selection_and_gate_policy(self):
        import test_ci_results
        for values in itertools.product((False, True), repeat=len(GROUPS)):
            flags = dict(zip(GROUPS, values))
            needs = {name: {'result': 'skipped'} for name in DEPENDENCIES}
            needs['changes'] = {
                'result': 'success',
                'outputs': {g: str(v).lower() for g, v in flags.items()},
            }
            needs['repository']['result'] = 'success'
            for name, members in MODULES.items():
                call = self.caller['jobs'][name]
                called = name == 'repository' or selected(call['if'], flags)
                inputs = {key: selected(expr, flags) for key, expr in call.get('with', {}).items()}
                expected_called = name == 'repository'
                for job, group in members.items():
                    expected = (group is None or flags['all'] or flags[group]
                                or (group in NODE and flags['node_projects']))
                    expected_called |= expected
                    leaf = self.modules[name]['jobs'][job]
                    actual = called and ('if' not in leaf or selected(leaf['if'], flags, inputs))
                    self.assertEqual(actual, expected, (flags, name, job))
                self.assertEqual(called, expected_called, (flags, name))
                needs[name]['result'] = 'success' if called else 'skipped'
            terraform = selected(self.caller['jobs']['gcp-terraform-validate']['if'], flags)
            self.assertEqual(terraform, flags['gcp_terraform'] or flags['all'])
            needs['gcp-terraform-validate']['result'] = 'success' if terraform else 'skipped'
            test_ci_results.gate.validate(needs, 'false')

    def test_required_display_names_are_unique_across_all_workflows(self):
        names = [job.get('name') for path in (ROOT / '.github/workflows').glob('*.yml')
                 for job in load(path)['jobs'].values()]
        for name in ('CI Gate', 'GCP Terraform Validate'):
            self.assertEqual(names.count(name), 1, name)

    def test_contracts_execute_on_every_ci_run(self):
        checks = self.modules['repository']['jobs']['ci-contracts']['steps']
        commands = '\n'.join(step.get('run', '') for step in checks)
        self.assertIn("python -m unittest discover -s .github/scripts -p 'test_ci_*.py' -v", commands)
        self.assertIn('.github/workflows/ci-*.yml', commands)
        self.assertFalse(any('if' in step or 'continue-on-error' in step for step in checks))

    def test_rejects_regressions_in_gate_selection_permissions_and_operations_boundary(self):
        mutations = [
            lambda c, m: c['jobs']['ci-gate']['needs'].remove('changes'),
            lambda c, m: c['jobs']['ci-gate'].update({'if': 'success()'}),
            lambda c, m: c['jobs']['system'].update(secrets='inherit'),
            lambda c, m: m['system']['jobs']['go-lint'].update({'continue-on-error': True}),
            lambda c, m: m['frontend']['jobs']['youtube-monitor-test'].update({'if': '${{ false }}'}),
            lambda c, m: m['frontend']['jobs']['mypage-check'].update({'if': '${{ false }}'}),
            lambda c, m: m['infra'].update(permissions={'contents': 'read', 'id-token': 'write'}),
            lambda c, m: c['jobs']['gcp-terraform-validate'].update(
                uses='./.github/workflows/ci-infra.yml'),
            lambda c, m: c['jobs']['operations'].update(uses='./.github/workflows/ci-infra.yml'),
            lambda c, m: c['jobs']['operations'].update(permissions={'contents': 'read'}),
            lambda c, m: c['on']['workflow_dispatch']['inputs'][
                'gcp_user_activity_schema_audit'].update(default=True),
        ]
        for mutate in mutations:
            caller, modules = copy.deepcopy(self.caller), copy.deepcopy(self.modules)
            mutate(caller, modules)
            with self.subTest(mutation=mutate), self.assertRaises(ValueError):
                validate(caller, modules)
