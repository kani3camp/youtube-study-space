"""Pinned native mock projections; never real imports or saved-plan admission.

All identities and values are invented. Terraform test uses in-memory fake state
only for the two-run value/unknown negatives; no backend or Cloud credentials.
"""
from __future__ import annotations
import copy
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT / '.github/scripts'))
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
from test_runtime_ownership import inventory, prepare_module
from test_runtime_ownership_package import fixture, planned_values, PROJECT, SHA, NOW, EMAIL
import prepare_runtime_ownership as preparer
import validate_runtime_ownership_plan as gate
import terraform_plan_diagnostic as diagnostic
from terraform_plan_summary import render_markdown

POOL = gate.POOL
FIELDS = ('display_name', 'description', 'disabled', 'mode')
CANARY = 'DUMMY_POOL_CANARY_NEVER_PUBLIC'
ARGUMENT_LEAVES = (
    *(f'var.inventory.pool.{field}' for field in FIELDS),
    *(f'var.inventory.provider.{field}' for field in ('display_name', 'description', 'disabled',
        'attribute_mapping', 'attribute_condition', 'aws_account_id')),
    'var.inventory.service_account_id', 'var.inventory.grants[each.key].member',
    'var.inventory.grants[each.key].condition == null',
    *(f'var.inventory.grants[each.key].condition.{field}' for field in ('title', 'description', 'expression')),
)


class NativePoolContracts(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        binary = shutil.which('terraform')
        if not binary:
            raise AssertionError('Pinned Terraform is required; this contract must not skip')
        cls.temp = tempfile.TemporaryDirectory(prefix='dummy-pool-plan-fidelity-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.directory = Path(cls.temp.name)
        cls.module = cls.directory / 'runtime-aws-wif/main.tf'
        cls.original = (ROOT / 'infra/gcp/modules/runtime-aws-wif/main.tf').read_text()
        cls.module.parent.mkdir()
        cls.module.write_text(cls.original)
        shutil.copyfile(ROOT / 'infra/gcp/environments/dev/.terraform.lock.hcl', cls.directory / '.terraform.lock.hcl')
        cls.cli = cls.directory / 'terraform.rc'
        cli = 'disable_checkpoint = true\n'
        mirror = os.environ.get('GCP_TEST_PROVIDER_MIRROR')
        if mirror:
            path = Path(mirror)
            if not path.is_absolute() or path.is_symlink():
                raise AssertionError('An absolute public provider mirror is required')
            cli += 'provider_installation {\n filesystem_mirror {\n path = '+json.dumps(str(path))+'\n }\n}\n'
        cls.cli.write_text(cli)
        # Deliberately exclude all AWS/Google credentials, profiles, ADC and HOME.
        cls.env = {key: os.environ[key] for key in ('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'NO_PROXY',
            'http_proxy', 'https_proxy', 'all_proxy', 'no_proxy', 'SSL_CERT_FILE', 'SSL_CERT_DIR',
            'REQUESTS_CA_BUNDLE', 'CURL_CA_BUNDLE') if key in os.environ}
        cls.env.update(PATH=str(Path(binary).parent)+os.pathsep+os.defpath,
            TF_CLI_CONFIG_FILE=str(cls.cli), TF_IN_AUTOMATION='true', TF_INPUT='false', CHECKPOINT_DISABLE='1')
        cls.binary = binary
        version = cls.command('version', '-json')
        if json.loads(version.stdout)['terraform_version'] != '1.16.4':
            raise AssertionError('Terraform version must match the protected contract')
        raw = inventory(); raw['pool']['description'] = CANARY
        cls.inventory = prepare_module.prepare(raw)['runtime_wif_inventory']
        cls.values = {'id': 'projects/'+PROJECT+'/locations/global/workloadIdentityPools/aws-runtime',
            'name': 'projects/111111111111/locations/global/workloadIdentityPools/aws-runtime',
            'project': PROJECT, 'workload_identity_pool_id': 'aws-runtime', 'state': 'ACTIVE',
            **cls.inventory['pool'], 'deletion_policy': 'DELETE', 'attestation_rules': [],
            'inline_certificate_issuance_config': [], 'inline_trust_config': []}
        cls.tests = cls.directory / 'tests'; cls.tests.mkdir()
        cls.configure()
        cls.command('init', '-backend=false', '-input=false', '-lockfile=readonly')
        cls.fixed = cls.plan()
        cls.counterfactual = cls.plan(old_expression=True)
        cls.null_mode = cls.plan(mode=None)
        cls.empty_mode = cls.plan(mode='')
        cls.default_off = cls.plan(own_pool=False)
        cls.changed = cls.plan(seeded='value')
        cls.unknown = cls.plan(seeded='unknown')

    @classmethod
    def command(cls, *args):
        result = subprocess.run([cls.binary, '-chdir='+str(cls.directory), *args],
            env=cls.env, stdin=subprocess.DEVNULL, capture_output=True, timeout=120)
        if result.returncode:
            # Even diagnostics stay private: a future fixture must not leak raw values.
            raise AssertionError('Credentialless Terraform mock contract failed: '+args[0])
        return result

    @classmethod
    def configure(cls, *, mode='FEDERATION_ONLY', own_pool=True, seeded=None):
        candidate = copy.deepcopy(cls.inventory); candidate['pool']['mode'] = mode
        if not own_pool:
            candidate = None  # Exercise count=0 with the private inventory absent.
        expression = 'var.inventory'
        if seeded == 'unknown':
            expression = 'merge(var.inventory, {pool=merge(var.inventory.pool, {description=var.pending ? terraform_data.pending[0].output : var.inventory.pool.description})})'
        text = '''terraform {
 required_providers {
  google = { source = "hashicorp/google", version = "= 8.5.0" }
 }
}
provider "google" {
 project = "'''+PROJECT+'''"
 access_token = "dummy-offline-not-a-credential"
}
variable "inventory" {
 type = any
 sensitive = true
 default = '''+json.dumps(candidate)+'''
}
module "runtime_wif" {
 source = "./runtime-aws-wif"
 project_id = "'''+PROJECT+'''"
 own_pool = '''+str(own_pool).lower()+'''
 inventory = '''+expression+'''
}
'''
        if own_pool and not seeded:
            text += '\nimport {\n to = '+POOL+'\n id = '+json.dumps(cls.values['id'])+'\n}\n'
        if seeded == 'unknown':
            text += '\nvariable "pending" { default = false }\nresource "terraform_data" "pending" {\n count = var.pending ? 1 : 0\n input = "dummy not applied"\n}\n'
        (cls.directory/'main.tf').write_text(text)

    @classmethod
    def plan(cls, *, old_expression=False, mode='FEDERATION_ONLY', own_pool=True, seeded=None):
        module = cls.original
        if old_expression:
            for field in FIELDS:
                expression = 'nonsensitive(var.inventory.pool.'+field+')'
                if module.count(expression) != 1:
                    raise AssertionError('Only the four Pool leaves may change sensitivity')
                module = module.replace(expression, 'var.inventory.pool.'+field)
        cls.module.write_text(module)
        cls.configure(mode=mode, own_pool=own_pool, seeded=seeded)
        values = dict(cls.values)
        if mode == '': values['mode'] = ''
        override = '\n override_resource {\n target = '+POOL+'\n values = '+json.dumps(values)+'\n }\n' if own_pool else ''
        if seeded:
            change = ('pending = true' if seeded == 'unknown' else
                'inventory = '+json.dumps({**cls.inventory, 'pool': {**cls.inventory['pool'], 'description': 'dummy changed'}}))
            tests = 'mock_provider "google" {}\nrun "seed" {\n command = apply\n'+override+'}\nrun "changed" {\n command = plan\n variables {\n'+change+'\n }\n'+override+'}\n'
        else:
            tests = 'mock_provider "google" {}\nrun "pool_import" {\n command = plan\n'+override+'}\n'
        (cls.tests/'pool.tftest.hcl').write_text(tests)
        result = cls.command('test', '-json', '-verbose')
        plans = [event['test_plan'] for line in result.stdout.splitlines()
                 if (event := json.loads(line)).get('type') == 'test_plan']
        if not plans: raise AssertionError('Expected a native Terraform test projection')
        return plans[-1]

    def setUp(self):
        for target in (preparer, gate):
            item = patch.object(target, 'PROJECT', PROJECT); item.start(); self.addCleanup(item.stop)

    def resource(self, native):
        return next(resource for resource in native['resource_changes'] if resource['address'] == POOL)

    def joined(self, native, mode='FEDERATION_ONLY'):
        # Synthetic full graph only exercises the unchanged validator with an
        # authentic Core/provider projection. This is not saved-plan admission.
        plan, metadata, packet = fixture()
        packet['inventory']['pool']['description'] = CANARY
        packet['inventory']['pool']['mode'] = mode
        plan['resource_changes'][-1] = copy.deepcopy(self.resource(native))
        planned_values(plan)
        return plan, metadata, packet

    def review(self, plan, metadata, packet):
        return gate.validate(plan, packet=packet, wave=packet['wave'], git_sha=SHA,
            metadata=metadata, phase='before', execution_email=EMAIL, now=NOW)

    def test_import_is_a_genuine_noop_with_identical_values_and_masks(self):
        change = self.resource(self.fixed)['change']
        self.assertEqual(change['actions'], ['no-op'])
        self.assertEqual(change['before'], change['after'])
        self.assertEqual(change['before_sensitive'], change['after_sensitive'])
        self.assertEqual(change['after_unknown'], {})
        self.assertEqual(change['importing'], {'id': self.values['id']})
        self.assertIsNone(change['after']['timeouts'])
        summary = self.review(*self.joined(self.fixed))
        self.assertEqual(summary['counts']['import'], 1)
        self.assertEqual(summary['counts']['update'], 0)

    def test_old_expression_reproduces_sensitivity_only_update_and_still_stops(self):
        change = self.resource(self.counterfactual)['change']
        self.assertEqual(change['actions'], ['update'])
        self.assertEqual(change['before'], change['after'])
        self.assertNotEqual(change['before_sensitive'], change['after_sensitive'])
        self.assertTrue(all(change['after_sensitive'][field] is True for field in FIELDS))
        with self.assertRaises(ValueError): self.review(*self.joined(self.counterfactual))

    def test_null_and_empty_mode_do_not_invent_values_or_actions(self):
        for native, mode in ((self.null_mode, None), (self.empty_mode, '')):
            with self.subTest(mode=mode):
                change = self.resource(native)['change']
                self.assertEqual(change['actions'], ['no-op'])
                self.assertEqual(change['before'], change['after'])
                self.assertEqual(change['before_sensitive'], change['after_sensitive'])
                self.review(*self.joined(native, mode))

    def test_fake_state_real_value_and_unknown_changes_remain_rejected(self):
        changed, unknown = (self.resource(item)['change'] for item in (self.changed, self.unknown))
        self.assertEqual(changed['actions'], ['update'])
        self.assertNotEqual(changed['before']['description'], changed['after']['description'])
        self.assertIs(unknown['after_unknown']['description'], True)
        for native in (self.changed, self.unknown):
            with self.assertRaises(ValueError): self.review(*self.joined(native))

    def test_forged_noop_value_unknown_mask_and_replace_are_not_admitted(self):
        mutations = [lambda c: c['after'].update(description='dummy changed'),
            lambda c: c['after_unknown'].update(description=True),
            lambda c: c['after_sensitive'].update(description=True),
            lambda c: c.update(actions=['delete', 'create'], replace_paths=[['id']])]
        for mutate in mutations:
            plan, metadata, packet = self.joined(self.fixed)
            mutate(plan['resource_changes'][-1]['change']); planned_values(plan)
            with self.subTest(change=mutations.index(mutate)), self.assertRaises(ValueError):
                self.review(plan, metadata, packet)

    def test_public_summary_diagnostic_sinks_never_publish_canary_values(self):
        plan, metadata, packet = self.joined(self.fixed)
        self.assertIn(CANARY, json.dumps(self.fixed))  # Raw JSON is always private.
        summary = self.review(plan, metadata, packet)
        self.assertNotIn(CANARY, json.dumps(summary)+render_markdown(summary))
        plan, metadata, packet = self.joined(self.counterfactual)
        context = diagnostic.Context()
        with self.assertRaises(diagnostic.Rejected) as rejected:
            gate.validate(plan, packet=packet, wave=packet['wave'], git_sha=SHA, metadata=metadata,
                phase='before', execution_email=EMAIL, now=NOW, diagnostic=context)
        with tempfile.TemporaryDirectory() as directory:
            diagnostic.record(directory, plan, context, rejected.exception)
            output = diagnostic.render(diagnostic.read(directory))
        self.assertNotIn(CANARY, output)
        self.assertIn('field=ACTION', output)

    def test_default_off_and_only_resource_argument_leaves_are_declassified(self):
        # Core omits resource_changes when this native test has no resources.
        self.assertEqual(self.default_off.get('resource_changes', []), [])
        self.assertEqual(self.original.count('nonsensitive('), len(ARGUMENT_LEAVES))
        for expression in ARGUMENT_LEAVES:
            self.assertEqual(self.original.count('nonsensitive('+expression+')'), 1)
        self.assertRegex(self.original, r'variable "inventory"\s*\{[\s\S]*?sensitive\s*=\s*true')
        root_inputs = (ROOT/'infra/gcp/environments/dev/runtime-ownership.tf').read_text()
        self.assertRegex(root_inputs, r'variable "runtime_wif_inventory"\s*\{[\s\S]*?sensitive\s*=\s*true')


class NativeProviderGrantContracts(unittest.TestCase):
    """Core/provider import projections; mock state and all inputs are invented."""

    command = classmethod(NativePoolContracts.command.__func__)
    setUp = NativePoolContracts.setUp

    @classmethod
    def setUpClass(cls):
        binary = shutil.which('terraform')
        if not binary:
            raise AssertionError('Pinned Terraform is required; this contract must not skip')
        cls.temp = tempfile.TemporaryDirectory(prefix='dummy-provider-grant-fidelity-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.directory = Path(cls.temp.name)
        cls.module = cls.directory/'runtime-aws-wif/main.tf'
        cls.module.parent.mkdir()
        cls.original = (ROOT/'infra/gcp/modules/runtime-aws-wif/main.tf').read_text()
        cls.module.write_text(cls.original)
        shutil.copyfile(ROOT/'infra/gcp/environments/dev/.terraform.lock.hcl', cls.directory/'.terraform.lock.hcl')
        cli = cls.directory/'terraform.rc'
        text = 'disable_checkpoint = true\n'
        mirror = os.environ.get('GCP_TEST_PROVIDER_MIRROR')
        if mirror:
            path = Path(mirror)
            if not path.is_absolute() or path.is_symlink():
                raise AssertionError('An absolute public provider mirror is required')
            text += 'provider_installation {\n filesystem_mirror {\n path = '+json.dumps(str(path))+'\n }\n}\n'
        cli.write_text(text)
        cls.env = {key: os.environ[key] for key in ('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'NO_PROXY',
            'http_proxy', 'https_proxy', 'all_proxy', 'no_proxy', 'SSL_CERT_FILE', 'SSL_CERT_DIR',
            'REQUESTS_CA_BUNDLE', 'CURL_CA_BUNDLE') if key in os.environ}
        cls.env.update(PATH=str(Path(binary).parent)+os.pathsep+os.defpath,
            TF_CLI_CONFIG_FILE=str(cli), TF_IN_AUTOMATION='true', TF_INPUT='false', CHECKPOINT_DISABLE='1')
        cls.binary = binary
        if json.loads(cls.command('version', '-json').stdout)['terraform_version'] != '1.16.4':
            raise AssertionError('Terraform version must match the protected contract')
        cls.raw = inventory()
        cls.raw['iam_policy']['bindings'][1]['condition']['description'] = CANARY
        cls.inventory = prepare_module.prepare(cls.raw)['runtime_wif_inventory']
        candidate = dict(runtime_wif_inventory=cls.inventory, own_runtime_wif_pool=True,
            own_runtime_wif_provider=True, runtime_wif_grant_keys=['grant-01', 'grant-02'], owned_api_keys=[])
        with patch.object(gate, 'PROJECT', PROJECT):
            cls.identities = gate.identities(candidate)
        cls.tests = cls.directory/'tests'; cls.tests.mkdir()
        cls.configure()
        cls.command('init', '-backend=false', '-input=false', '-lockfile=readonly')
        cls.fixed = cls.plan()
        cls.counterfactual = cls.plan(old_expression=True)
        cls.changed = cls.plan(changed=True)

    @classmethod
    def configure(cls, *, seeded=False):
        text = '''terraform {
 required_providers {
  google = { source = "hashicorp/google", version = "= 8.5.0" }
 }
}
provider "google" { access_token = "dummy-offline-not-a-credential" }
variable "inventory" {
 type = any
 sensitive = true
 default = '''+json.dumps(cls.inventory)+'''
}
module "runtime_wif" {
 source = "./runtime-aws-wif"
 project_id = "'''+PROJECT+'''"
 own_pool = true
 own_provider = true
 grant_keys = ["grant-01", "grant-02"]
 inventory = var.inventory
}
'''
        if not seeded:
            for address, (_, import_id, _) in cls.identities.items():
                text += '\nimport {\n to = '+address+'\n id = '+json.dumps(import_id)+'\n}\n'
        (cls.directory/'main.tf').write_text(text)

    @classmethod
    def plan(cls, *, old_expression=False, changed=False):
        module = cls.original
        if old_expression:
            for expression in ARGUMENT_LEAVES[4:]:
                marked = 'nonsensitive('+expression+')'
                if module.count(marked) != 1:
                    raise AssertionError('Expected one explicit resource argument leaf')
                module = module.replace(marked, expression)
        cls.module.write_text(module)
        cls.configure(seeded=changed)
        overrides = ''
        for address, (_, _, values) in cls.identities.items():
            if address != POOL and address != gate.PROVIDER:
                values = dict(values, etag='DUMMY_IAM_ETAG')
            overrides += '\n override_resource {\n target = '+address+'\n values = '+json.dumps(values)+'\n }\n'
        if changed:
            value = copy.deepcopy(cls.inventory)
            value['provider']['description'] = 'DUMMY_CHANGED_PROVIDER'
            tests = ('mock_provider "google" {}\nrun "seed" {\n command = apply\n'+overrides+
                '}\nrun "changed" {\n command = plan\n variables {\n inventory = '+json.dumps(value)+'\n }\n'+overrides+'}\n')
        else:
            tests = 'mock_provider "google" {}\nrun "imports" {\n command = plan\n'+overrides+'}\n'
        (cls.tests/'imports.tftest.hcl').write_text(tests)
        result = cls.command('test', '-json', '-verbose')
        plans = [event['test_plan'] for line in result.stdout.splitlines()
                 if (event := json.loads(line)).get('type') == 'test_plan']
        if not plans:
            raise AssertionError('Expected native Terraform import projections')
        return plans[-1]

    def joined(self, native, index):
        plan, metadata, packet = fixture(index)
        packet['inventory'] = copy.deepcopy(self.raw)
        address = plan['resource_changes'][-1]['address']
        # Only the new wave's native projection replaces its synthetic entry;
        # the remaining graph is a dummy validator witness, not saved admission.
        plan['resource_changes'][-1] = copy.deepcopy(next(
            item for item in native['resource_changes'] if item['address'] == address))
        planned_values(plan)
        return plan, metadata, packet

    def review(self, plan, metadata, packet):
        return gate.validate(plan, packet=packet, wave=packet['wave'], git_sha=SHA,
            metadata=metadata, phase='before', execution_email=EMAIL, now=NOW)

    def test_provider_and_both_grant_imports_are_noop_with_equal_masks(self):
        for index in (1, 2, 3):
            with self.subTest(wave=index):
                joined = self.joined(self.fixed, index)
                change = joined[0]['resource_changes'][-1]['change']
                self.assertEqual(change['actions'], ['no-op'])
                self.assertEqual(change['before'], change['after'])
                self.assertEqual(change['before_sensitive'], change['after_sensitive'])
                self.assertEqual(change['after_unknown'], {})
                self.assertIn('importing', change)
                summary = self.review(*joined)
                self.assertEqual(summary['counts']['import'], 1)
                self.assertEqual(summary['counts']['update'], 0)
                self.assertNotIn(CANARY, json.dumps(summary)+render_markdown(summary))

    def test_old_provider_and_grant_expressions_reproduce_mark_only_updates(self):
        for index in (1, 2, 3):
            with self.subTest(wave=index):
                joined = self.joined(self.counterfactual, index)
                change = joined[0]['resource_changes'][-1]['change']
                self.assertEqual(change['actions'], ['update'])
                self.assertEqual(change['before'], change['after'])
                self.assertNotEqual(change['before_sensitive'], change['after_sensitive'])
                with self.assertRaises(ValueError):
                    self.review(*joined)

    def test_changed_provider_value_and_forged_grant_values_remain_rejected(self):
        joined = self.joined(self.changed, 1)
        change = joined[0]['resource_changes'][-1]['change']
        self.assertEqual(change['actions'], ['update'])
        self.assertNotEqual(change['before']['description'], change['after']['description'])
        with self.assertRaises(ValueError):
            self.review(*joined)
        for index in (2, 3):
            joined = self.joined(self.fixed, index)
            joined[0]['resource_changes'][-1]['change']['after']['member'] += 'Changed'
            planned_values(joined[0])
            with self.subTest(wave=index), self.assertRaises(ValueError):
                self.review(*joined)


if __name__ == '__main__':
    unittest.main()
