#!/usr/bin/env python3
"""Desired-false API ownership with pinned native null import defaults, offline only."""
from __future__ import annotations

from copy import deepcopy
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
from threading import Thread
import unittest

import terraform_migration_contract as gate
import test_terraform_migration_contract as fixtures

ROOT = Path(__file__).resolve().parents[2]
SERVICE = 'fixture.googleapis.com'
ADDRESS = f'module.owned_apis.google_project_service.owned["{SERVICE}"]'
FLAGS = ('disable_on_destroy', 'disable_dependent_services')


def configuration():
    return {'address': 'google_project_service.owned', 'mode': 'managed',
            'type': 'google_project_service', 'name': 'owned', 'provider_config_key': 'google',
            'expressions': {field: {'constant_value': False} for field in FLAGS}}


def fixture(environment='dev', flag=None):
    plan = fixtures.plan(environment, importing=False)
    before, after = fixtures.state(environment, count=2), fixtures.state(environment, count=2, serial=3)
    manifest, schema = fixtures.manifest(environment), fixtures.provider_schema()
    project = manifest['project_id']
    values = {'id': project + '/' + SERVICE, 'project': project, 'service': SERVICE,
              'disable_on_destroy': flag, 'disable_dependent_services': flag, 'deletion_policy': 'DELETE'}
    descriptor = {'address': ADDRESS, 'type': 'google_project_service', 'import_id': values['id']}
    manifest['resources'].append(descriptor)
    header = {'address': ADDRESS, 'module_address': 'module.owned_apis', 'mode': 'managed',
              'type': descriptor['type'], 'name': 'owned', 'index': SERVICE, 'provider_name': gate.GOOGLE}
    identity = {'project': project, 'service': SERVICE}
    plan['resource_changes'].append(dict(header, change={
        'actions': ['no-op'], 'before': deepcopy(values), 'after': deepcopy(values),
        'before_sensitive': {}, 'after_sensitive': {}, 'after_unknown': {},
        'before_identity': deepcopy(identity), 'after_identity': deepcopy(identity),
        'importing': {'id': descriptor['import_id']}}))
    plan['planned_values']['root_module']['child_modules'].append({
        'address': 'module.owned_apis', 'resources': [dict(
            {key: value for key, value in header.items() if key != 'module_address'},
            schema_version=0, values=deepcopy(values), sensitive_values={}, identity=deepcopy(identity), identity_schema_version=1)]})
    after['resources'].append({'module': 'module.owned_apis', 'mode': 'managed', 'type': descriptor['type'],
                              'name': 'owned', 'provider': f'provider["{gate.GOOGLE}"]', 'instances': [
                                  {'index_key': SERVICE, 'schema_version': 0, 'attributes': deepcopy(values),
                                   'identity': deepcopy(identity), 'identity_schema_version': 1,
                                   'sensitive_attributes': [], 'dependencies': [], 'private': ''}]})
    attributes = {key: {'type': 'string', 'optional': True} for key in values}
    attributes.update({field: {'type': 'bool', 'optional': True} for field in FLAGS})
    provider = schema['provider_schemas'][gate.GOOGLE]
    provider['resource_schemas'][descriptor['type']] = {'version': 0, 'block': {'attributes': attributes}}
    provider['resource_identity_schemas'] = {descriptor['type']: {'version': 1, 'attributes': {
        'project': {'type': 'string', 'optional_for_import': True},
        'service': {'type': 'string', 'required_for_import': True}}}}
    plan['configuration']['root_module']['module_calls']['owned_apis'] = {
        'source': '../../modules/owned-project-apis', 'module': {'resources': [configuration()]}}
    return plan, before, after, manifest, schema


class APIConfigurationContract(unittest.TestCase):
    def validate(self, parts):
        plan, before, _, manifest, schema = parts
        return gate.validate_plan(plan, manifest=manifest, environment=manifest['environment'],
                                  provider_schema=schema, state=before)

    def test_both_environments_accept_null_and_existing_false_only_with_desired_false(self):
        for environment in gate.ENVIRONMENTS:
            for flag in (None, False):
                parts = fixture(environment, flag)
                original = deepcopy(parts)
                with self.subTest(environment=environment, flag=flag):
                    self.assertEqual(self.validate(parts)['imports'], 1)
                    plan, before, after, manifest, schema = parts
                    delta = gate.validate_state_delta(before, after, manifest=manifest, environment=environment,
                        provider_schema=schema, expected_imports={ADDRESS}, plan=plan)
                    self.assertTrue(delta['saved_plan_checks_match'])
                    self.assertEqual(parts, original)

    def test_value_flags_are_present_null_or_bool_false_without_coercion(self):
        for field in FLAGS:
            for bad in (True, 0, 1, 'false', [], {}, 'MISSING'):
                parts = fixture();plan, _, _, manifest, schema = parts
                value = plan['resource_changes'][-1]['change']['after']
                if bad == 'MISSING': value.pop(field)
                else: value[field] = bad
                with self.subTest(field=field, bad=bad), self.assertRaises(gate.ContractError):
                    gate._values(value, manifest['resources'][-1], manifest, schema)

    def test_native_optional_bool_schema_is_required(self):
        for bad in ({'type': 'string', 'optional': True}, {'type': 'bool'},
                    {'type': 'bool', 'optional': True, 'computed': True},
                    {'type': 'bool', 'optional': True, 'required': True}, None):
            parts = fixture();schema = parts[-1]
            attrs = schema['provider_schemas'][gate.GOOGLE]['resource_schemas']['google_project_service']['block']['attributes']
            if bad is None: attrs.pop(FLAGS[0])
            else: attrs[FLAGS[0]] = bad
            with self.subTest(schema=bad), self.assertRaises(gate.ContractError): self.validate(parts)

    def test_false_configuration_cannot_be_dynamic_missing_coerced_or_contain_extra_expression_keys(self):
        for field in FLAGS:
            for expression in (None, {}, {'constant_value': True}, {'constant_value': 0},
                               {'constant_value': None}, {'constant_value': 'false'},
                               {'references': ['var.false_flag']},
                               {'constant_value': False, 'references': ['var.false_flag']}):
                parts = fixture();resource = parts[0]['configuration']['root_module']['module_calls']['owned_apis']['module']['resources'][0]
                if expression is None: resource['expressions'].pop(field)
                else: resource['expressions'][field] = expression
                with self.subTest(field=field, expression=expression), self.assertRaises(gate.ContractError): self.validate(parts)

    def test_only_exact_reviewed_module_resource_provider_and_tree_can_authorize_null(self):
        edits = [
            lambda p: p['configuration'].pop('root_module'),
            lambda p: p['configuration']['root_module'].update(module_calls=[]),
            lambda p: p['configuration']['root_module']['module_calls'].pop('owned_apis'),
            lambda p: p['configuration']['root_module']['module_calls'].update(owned_apis=None),
            *[lambda p, source=source: p['configuration']['root_module']['module_calls']['owned_apis'].update(source=source)
              for source in (None, '.', '/tmp/owned-project-apis', '../../modules/unreviewed', 'https://example.invalid/module')],
            lambda p: p['configuration']['root_module']['module_calls']['owned_apis'].update(module=[]),
            lambda p: p['configuration']['root_module']['module_calls']['owned_apis']['module'].update(resources=None),
            lambda p: p['configuration']['root_module']['module_calls']['owned_apis']['module']['resources'].append(configuration()),
            lambda p: p['configuration']['provider_config']['google'].update(full_name='unreviewed/google'),
        ]
        for field, value in (('address', 'google_project_service.other'), ('mode', 'data'),
                             ('type', 'google_pubsub_topic'), ('name', 'other'), ('provider_config_key', 'google.other'),
                             ('expressions', [])):
            edits.append(lambda p, field=field, value=value: p['configuration']['root_module']['module_calls']['owned_apis']['module']['resources'][0].update({field: value}))
        for index, edit in enumerate(edits):
            parts = fixture();edit(parts[0])
            with self.subTest(case=index), self.assertRaises(gate.ContractError): self.validate(parts)

    def test_instance_indexes_resolve_to_static_module_and_resource_configuration(self):
        parts = fixture();plan, before, after, manifest, _ = parts
        old = 'module.owned_apis';new = 'module.owned_apis["dummy-instance"]'
        manifest['resources'][-1]['address'] = ADDRESS.replace(old, new)
        change = plan['resource_changes'][-1]
        change['address'] = change['address'].replace(old, new);change['module_address'] = new
        shown = plan['planned_values']['root_module']['child_modules'][-1]
        shown['address'] = new;shown['resources'][0]['address'] = change['address']
        after['resources'][-1]['module'] = new
        self.assertEqual(self.validate(parts)['imports'], 1)
        # The known direct module source does not authorize an unknown ancestor.
        manifest['resources'][-1]['address'] = change['address'].replace(new, 'module.outer.' + new)
        with self.assertRaises(gate.ContractError): self.validate(parts)

    def test_multiple_selected_api_instances_share_static_config_and_import_only_the_new_service(self):
        plan, _, after, manifest, schema = fixture()
        before = deepcopy(after);after['serial'] += 1
        plan['resource_changes'][-1]['change'].pop('importing')
        service = 'secondfixture.googleapis.com'
        address = ADDRESS.replace(SERVICE, service)
        descriptor = deepcopy(manifest['resources'][-1])
        descriptor.update(address=address, import_id=manifest['project_id'] + '/' + service)
        manifest['resources'].append(descriptor)
        change = deepcopy(plan['resource_changes'][-1])
        change.update(address=address, index=service)
        for side in ('before', 'after'):
            change['change'][side].update(id=descriptor['import_id'], service=service)
            change['change'][side + '_identity']['service'] = service
        change['change']['importing'] = {'id': descriptor['import_id']}
        plan['resource_changes'].append(change)
        shown = deepcopy(plan['planned_values']['root_module']['child_modules'][-1]['resources'][0])
        shown.update(address=address, index=service, values=deepcopy(change['change']['after']),
                     identity=deepcopy(change['change']['after_identity']))
        plan['planned_values']['root_module']['child_modules'][-1]['resources'].append(shown)
        instance = deepcopy(after['resources'][-1]['instances'][0])
        instance.update(index_key=service, attributes=deepcopy(change['change']['after']),
                        identity=deepcopy(change['change']['after_identity']))
        after['resources'][-1]['instances'].append(instance)
        result = gate.validate_plan(plan, manifest=manifest, environment='dev', provider_schema=schema, state=before)
        self.assertEqual((result['imports'], result['managed_before'], result['managed']), (1, 3, 4))
        self.assertTrue(gate.validate_state_delta(before, after, manifest=manifest, environment='dev',
            provider_schema=schema, expected_imports={address}, plan=plan)['saved_plan_checks_match'])

    def test_every_selected_resource_must_resolve_instead_of_reusing_one_configuration(self):
        parts = fixture();plan, _, _, manifest, _ = parts
        other = deepcopy(manifest['resources'][-1])
        other.update(address=ADDRESS.replace('.owned[', '.unreviewed['), import_id=other['import_id'] + '-other')
        manifest['resources'].append(other)
        with self.assertRaises(gate.ContractError): self.validate(parts)

    def test_api_state_delta_and_raw_state_require_saved_plan_configuration_proof(self):
        for flag in (None, False):
            plan, before, after, manifest, schema = fixture(flag=flag)
            for supplied in (None, {'configuration': {}}):
                with self.subTest(flag=flag, plan=supplied), self.assertRaises(gate.ContractError):
                    gate.validate_state_delta(before, after, manifest=manifest, environment='dev',
                        provider_schema=schema, expected_imports={ADDRESS}, plan=supplied)
            with self.assertRaises(gate.ContractError):
                gate._native_state(after, gate.validate_manifest(manifest, 'dev'), manifest, schema, 'dev')

    def test_desired_false_does_not_normalize_state_values_masks_identities_or_replans(self):
        parts = fixture();plan, before, after, manifest, schema = parts
        for edit in (lambda p: p['resource_changes'][-1]['change']['after_sensitive'].update(disable_on_destroy=True),
                     lambda p: p['resource_changes'][-1]['change']['after_identity'].update(service='other.googleapis.com'),
                     lambda p: p['resource_changes'][-1]['change']['after'].update(disable_on_destroy=False)):
            candidate = deepcopy(parts);edit(candidate[0])
            with self.assertRaises(gate.ContractError): self.validate(candidate)
        changed = deepcopy(after);changed['resources'][-1]['instances'][0]['attributes']['disable_on_destroy'] = False
        with self.assertRaises(gate.ContractError):
            gate.validate_state_delta(before, changed, manifest=manifest, environment='dev', provider_schema=schema,
                                      expected_imports={ADDRESS}, plan=plan)
        other = fixture(flag=False)[0]
        with self.assertRaises(gate.ContractError):
            gate.compare_replans(plan, other, manifest=manifest, environment='dev', provider_schema=schema, state=before)
        self.assertIn('configuration', gate.plan_commitment_payload(plan, manifest, 'dev', schema, state=before))

    def test_no_selected_api_preserves_existing_plan_and_empty_configuration_behavior(self):
        for environment in gate.ENVIRONMENTS:
            self.assertEqual(gate.validate_plan(fixtures.plan(environment), manifest=fixtures.manifest(environment),
                environment=environment, provider_schema=fixtures.provider_schema(), state=fixtures.state(environment))['imports'], 1)
        self.assertEqual(gate._api_configuration(None, {}), frozenset())


class NativeAPIImportFidelity(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix='gcp-api-native-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.base = Path(cls.temp.name).resolve();cls.requests = []
        cls.binary = shutil.which('terraform')
        if cls.binary is None: raise AssertionError('Pinned Terraform is required; native regression must not skip')
        cls.project = gate.ENVIRONMENTS['dev'][0]

        class DummyAPI(BaseHTTPRequestHandler):
            def log_message(self, *_): pass
            def do_GET(self):
                cls.requests.append(self.command)
                if self.headers.get('Host') != f'127.0.0.1:{self.server.server_port}':
                    self.send_error(403);return
                path = self.path.split('?', 1)[0]
                if path == '/v1/projects/' + cls.project:
                    value = {'projectId': cls.project, 'projectNumber': '111111111111',
                             'name': 'Invented native fixture', 'lifecycleState': 'ACTIVE'}
                elif path == '/v1/projects/' + cls.project + '/services':
                    value = {'services': [{'name': 'projects/111111111111/services/' + SERVICE,
                                          'config': {'name': SERVICE}, 'state': 'ENABLED'}]}
                else:
                    self.send_error(404);return
                raw = json.dumps(value).encode();self.send_response(200)
                self.send_header('Content-Type', 'application/json');self.send_header('Content-Length', str(len(raw)))
                self.end_headers();self.wfile.write(raw)
            def reject_write(self):
                cls.requests.append(self.command);self.send_error(405)
            do_POST = do_PUT = do_PATCH = do_DELETE = do_HEAD = reject_write

        cls.server = HTTPServer(('127.0.0.1', 0), DummyAPI)
        Thread(target=cls.server.serve_forever, daemon=True).start()
        cls.addClassCleanup(cls.server.server_close);cls.addClassCleanup(cls.server.shutdown)
        endpoint = f'http://127.0.0.1:{cls.server.server_port}/'
        (cls.base/'cache').mkdir();(cls.base/'home').mkdir()
        cli = 'disable_checkpoint = true\n'
        if mirror := os.environ.get('GCP_TEST_PROVIDER_MIRROR'):
            location = Path(mirror)
            if not location.is_absolute() or location.is_symlink(): raise AssertionError('Absolute public provider mirror required')
            cli += 'provider_installation {\n filesystem_mirror {\n path = ' + json.dumps(str(location)) + '\n }\n}\n'
        (cls.base/'terraform.rc').write_text(cli)
        cls.env = {key: os.environ[key] for key in (
            'HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'http_proxy', 'https_proxy', 'all_proxy',
            'SSL_CERT_FILE', 'SSL_CERT_DIR', 'REQUESTS_CA_BUNDLE', 'CURL_CA_BUNDLE') if key in os.environ}
        cls.env.update(PATH=str(Path(cls.binary).parent) + os.pathsep + os.defpath,
            HOME=str(cls.base/'home'), TMPDIR=str(cls.base), NO_PROXY='127.0.0.1,localhost',
            no_proxy='127.0.0.1,localhost', TF_CLI_CONFIG_FILE=str(cls.base/'terraform.rc'),
            TF_PLUGIN_CACHE_DIR=str(cls.base/'cache'), TF_IN_AUTOMATION='true', TF_INPUT='false', CHECKPOINT_DISABLE='1')
        cls.directory = cls.base/'environments/dev';cls.directory.mkdir(parents=True)
        target = cls.base/'modules/owned-project-apis';target.mkdir(parents=True)
        shutil.copyfile(ROOT/'infra/gcp/modules/owned-project-apis/main.tf', target/'main.tf')
        shutil.copyfile(ROOT/'infra/gcp/environments/dev/.terraform.lock.hcl', cls.directory/'.terraform.lock.hcl')
        endpoints = ('cloud_resource_manager_custom_endpoint', 'resource_manager_custom_endpoint',
                     'resource_manager3_custom_endpoint', 'resource_manager_v3_custom_endpoint', 'service_usage_custom_endpoint')
        (cls.directory/'main.tf').write_text('''terraform {
 required_version = "= ''' + gate.TERRAFORM_VERSION + '''"
 required_providers { google = { source = "hashicorp/google", version = "= 8.5.0" } }
}
provider "google" {
 project = "''' + cls.project + '''"
 access_token = "DUMMY_UNUSABLE_NATIVE_API_TOKEN"
''' + ''.join(' ' + key + ' = ' + json.dumps(endpoint + ('v1/' if key == 'service_usage_custom_endpoint' else '')) + '\n' for key in endpoints) + '''}
variable "manage_api" { default = false }
module "owned_apis" {
 source = "../../modules/owned-project-apis"
 project_id = "''' + cls.project + '''"
 service_keys = var.manage_api ? toset(["''' + SERVICE + '''"]) : toset([])
 classification = { "''' + SERVICE + '''" = {
  classification = "Own"
  state = "ENABLED"
  dependency_addresses = ["google_pubsub_topic.dummy_consumer"]
  reason = "Invented local serializer fixture; no real API consumer claim."
 } }
}
import {
 for_each = var.manage_api ? toset(["existing"]) : toset([])
 to = module.owned_apis.google_project_service.owned["''' + SERVICE + '''"]
 id = "''' + cls.project + '/' + SERVICE + '''"
}
output "project_id" { value = "''' + cls.project + '''" }
output "environment" { value = "development" }
''')
        cls.command('init', '-backend=false', '-input=false', '-lockfile=readonly')
        cls.schema = json.loads(cls.command('providers', 'schema', '-json').stdout)
        attributes = cls.schema['provider_schemas'][gate.GOOGLE]['provider']['block']['attributes']
        if any(attributes.get(name, {}).get('type') != 'string' for name in endpoints):
            raise AssertionError('All local endpoints must exist in the pinned provider schema')
        cls.env.update({key: 'http://127.0.0.1:9' for key in ('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'http_proxy', 'https_proxy', 'all_proxy')})
        version = json.loads(cls.command('version', '-json').stdout)
        if version['terraform_version'] != gate.TERRAFORM_VERSION or version['provider_selections'][gate.GOOGLE] != '8.5.0':
            raise AssertionError('Native Core/provider must match the protected pins')
        cls.plan(managed=False);cls.command('apply', '-input=false', str(cls.directory/'saved.tfplan'))
        cls.before = cls.state();cls.import_plan = cls.plan()
        cls.manifest = {'schema_version': 'gcp-migration-manifest-v1', 'environment': 'dev',
                        'project_id': cls.project, 'resources': [{'address': ADDRESS, 'type': 'google_project_service',
                                                               'import_id': cls.project + '/' + SERVICE}]}
        cls.result = gate.validate_plan(cls.import_plan, manifest=cls.manifest, environment='dev', provider_schema=cls.schema, state=cls.before)
        if cls.result['imports'] != 1 or cls.result['mutations'] != 0:
            raise AssertionError('Only one exact native API import is permitted')
        cls.command('apply', '-input=false', str(cls.directory/'saved.tfplan'));cls.after = cls.state();cls.post = cls.plan()

    @classmethod
    def command(cls, *args, codes=(0,)):
        result = subprocess.run([cls.binary, '-chdir=' + str(cls.directory), *args], env=cls.env,
                                stdin=subprocess.DEVNULL, capture_output=True, timeout=120)
        if result.returncode not in codes: raise AssertionError('Private native API regression failed: ' + args[0])
        return result

    @classmethod
    def plan(cls, managed=True):
        cls.command('plan', '-input=false', '-detailed-exitcode', '-var=manage_api=' + str(managed).lower(),
                    '-out=' + str(cls.directory/'saved.tfplan'), codes=(0, 2))
        return json.loads(cls.command('show', '-json', str(cls.directory/'saved.tfplan')).stdout)

    @classmethod
    def state(cls): return json.loads((cls.directory/'terraform.tfstate').read_text())

    def test_native_import_nulls_saved_state_identity_and_post_noop(self):
        change = self.import_plan['resource_changes'][0]['change']
        self.assertEqual(change['actions'], ['no-op'])
        self.assertEqual(change['before'], change['after'])
        self.assertEqual(change['before_identity'], change['after_identity'])
        self.assertEqual(change['after']['deletion_policy'], 'DELETE')
        attributes = self.after['resources'][0]['instances'][0]['attributes']
        for field in FLAGS:
            self.assertIn(field, change['after']);self.assertIsNone(change['after'][field])
            self.assertIn(field, attributes);self.assertIsNone(attributes[field])
        self.assertEqual(attributes, change['after'])
        self.assertEqual(self.after['resources'][0]['instances'][0]['identity'], change['after_identity'])
        delta = gate.validate_state_delta(self.before, self.after, manifest=self.manifest, environment='dev',
                                         provider_schema=self.schema, expected_imports={ADDRESS}, plan=self.import_plan)
        self.assertTrue(delta['saved_plan_checks_match'])
        result = gate.validate_plan(self.post, manifest=self.manifest, environment='dev', provider_schema=self.schema,
                                    state=self.after, phase='post')
        self.assertEqual((result['imports'], result['raw_observation_drift'], result['mutations']), (0, 0, 0))
        self.assertEqual(set(self.requests), {'GET'})
        self.assertEqual((ROOT/'infra/gcp/modules/owned-project-apis/main.tf').read_bytes(),
                         (self.base/'modules/owned-project-apis/main.tf').read_bytes())


if __name__ == '__main__':
    unittest.main()
