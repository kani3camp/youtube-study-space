#!/usr/bin/env python3
"""Pinned Core/provider identity witness using local dummy State and HTTP only.

The project name exercises the fixed environment contract, but the table,
metadata and unusable token are invented. The sole API endpoint is loopback;
all non-GET methods fail. A second fixture manually seeds a sensitive schema
mark in local dummy State, never in an actual backend or cloud resource.
"""
from __future__ import annotations

from copy import deepcopy
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
from threading import Thread
import unittest

import terraform_migration_contract as contract

ROOT = Path(__file__).resolve().parents[2]
PROJECT = contract.ENVIRONMENTS['dev'][0]
ADDRESS = 'google_bigquery_table.retained[0]'
IDENTIFIER = f'projects/{PROJECT}/datasets/fixture/tables/native_identity_witness'
TOKEN = 'DUMMY_UNUSABLE_NATIVE_WITNESS_TOKEN'
SCHEMA_EXPRESSION = 'jsonencode([{name="fixture",type="STRING",mode="NULLABLE"}])'


class NativeMigrationFidelity(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix='gcp-migration-native-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.base = Path(cls.temp.name).resolve()
        cls.binary = shutil.which('terraform')
        if cls.binary is None:
            raise AssertionError('Pinned Terraform is required; native witness must not skip')
        cls.api_version, cls.requests = 1, []

        class DummyAPI(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_GET(self):
                cls.requests.append(self.command)
                if self.path.split('?', 1)[0] != '/' + IDENTIFIER:
                    self.send_error(404)
                    return
                raw = json.dumps({
                    'kind': 'bigquery#table',
                    'id': PROJECT + ':fixture.native_identity_witness',
                    'tableReference': {'projectId': PROJECT, 'datasetId': 'fixture',
                                       'tableId': 'native_identity_witness'},
                    'creationTime': '1700000000000',
                    'lastModifiedTime': str(1700000000000 + cls.api_version),
                    'etag': 'DUMMY_ETAG_' + str(cls.api_version),
                    'numBytes': '0', 'numRows': '0', 'location': 'asia-southeast2',
                    'type': 'TABLE', 'schema': {'fields': [
                        {'name': 'fixture', 'type': 'STRING', 'mode': 'NULLABLE'}]},
                }).encode()
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def reject_write(self):
                cls.requests.append(self.command)
                self.send_error(405)

            do_POST = do_PUT = do_PATCH = do_DELETE = reject_write

        cls.server = HTTPServer(('127.0.0.1', 0), DummyAPI)
        cls.thread = Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        cls.addClassCleanup(cls.server.server_close)
        cls.addClassCleanup(cls.server.shutdown)
        cls.endpoint = f'http://127.0.0.1:{cls.server.server_port}/'
        (cls.base/'cache').mkdir()
        cli = cls.base/'terraform.rc'
        configuration = 'disable_checkpoint = true\n'
        mirror = os.environ.get('GCP_TEST_PROVIDER_MIRROR')
        if mirror:
            location = Path(mirror)
            if not location.is_absolute() or location.is_symlink():
                raise AssertionError('An absolute public provider mirror is required')
            configuration += 'provider_installation {\n filesystem_mirror {\n path = ' + json.dumps(str(location)) + '\n }\n}\n'
        cli.write_text(configuration)
        # Never inherit credentials, ADC, TF_VAR/CLI overrides, logs or HOME.
        cls.env = {key: os.environ[key] for key in (
            'HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'http_proxy', 'https_proxy', 'all_proxy',
            'SSL_CERT_FILE', 'SSL_CERT_DIR', 'REQUESTS_CA_BUNDLE', 'CURL_CA_BUNDLE') if key in os.environ}
        cls.env.update(PATH=str(Path(cls.binary).parent) + os.pathsep + os.defpath,
            HOME=str(cls.base), TMPDIR=str(cls.base), NO_PROXY='127.0.0.1,localhost',
            no_proxy='127.0.0.1,localhost', TF_CLI_CONFIG_FILE=str(cli),
            TF_PLUGIN_CACHE_DIR=str(cls.base/'cache'), TF_IN_AUTOMATION='true',
            TF_INPUT='false', CHECKPOINT_DISABLE='1')
        cls.seed = cls.directory('import')
        version = json.loads(cls.command(cls.seed, 'version', '-json').stdout)
        if version['terraform_version'] != contract.TERRAFORM_VERSION:
            raise AssertionError('Terraform version must match the protected contract')
        cls.command(cls.seed, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
        cls.schema = json.loads(cls.command(cls.seed, 'providers', 'schema', '-json').stdout)
        cls.command(cls.seed, 'apply', '-input=false', '-auto-approve')
        cls.before_import = cls.state(cls.seed)
        cls.import_plan = cls.plan(cls.seed, managed=True)
        if cls.import_plan['resource_changes'][0]['change']['actions'] != ['no-op']:
            raise AssertionError('Native import witness must contain no resource mutation')
        cls.command(cls.seed, 'apply', '-input=false', str(cls.seed/'saved.tfplan'))
        cls.after_import = cls.state(cls.seed)
        cls.manifest = {'schema_version': 'gcp-migration-manifest-v1', 'environment': 'dev',
            'project_id': PROJECT, 'resources': [{'address': ADDRESS, 'type': 'google_bigquery_table',
                                                'import_id': IDENTIFIER}]}

    @classmethod
    def command(cls, directory, *args, codes=(0,)):
        result = subprocess.run([cls.binary, '-chdir=' + str(directory), *args], env=cls.env,
            stdin=subprocess.DEVNULL, capture_output=True, timeout=120)
        if result.returncode not in codes:
            raise AssertionError('Private native witness command failed: ' + args[0])
        return result

    @classmethod
    def directory(cls, name):
        directory = cls.base/name
        directory.mkdir()
        shutil.copyfile(ROOT/'infra/gcp/environments/dev/.terraform.lock.hcl', directory/'.terraform.lock.hcl')
        (directory/'main.tf').write_text('''terraform {
 required_version = "= ''' + contract.TERRAFORM_VERSION + '''"
 required_providers { google = { source = "hashicorp/google", version = "= 8.5.0" } }
}
provider "google" {
 project = "''' + PROJECT + '''"
 access_token = "''' + TOKEN + '''"
 big_query_custom_endpoint = "''' + cls.endpoint + '''"
}
variable "project_id" {
 default = "''' + PROJECT + '''"
 validation {
  condition = var.project_id == "''' + PROJECT + '''"
  error_message = "The dummy witness must use the fixed environment contract."
 }
}
variable "manage_table" { default = false }
resource "google_bigquery_table" "retained" {
 count = var.manage_table ? 1 : 0
 project = var.project_id
 dataset_id = "fixture"
 table_id = "native_identity_witness"
 schema = ''' + SCHEMA_EXPRESSION + '''
 deletion_protection = true
 lifecycle { prevent_destroy = true }
}
import {
 for_each = var.manage_table ? toset(["existing"]) : toset([])
 to = google_bigquery_table.retained[0]
 id = "''' + IDENTIFIER + '''"
}
output "project_id" { value = var.project_id }
output "environment" { value = "development" }
''')
        return directory

    @classmethod
    def state(cls, directory):
        return json.loads((directory/'terraform.tfstate').read_text())

    @classmethod
    def plan(cls, directory, *, managed=True, marked=False):
        expression = 'sensitive(' + SCHEMA_EXPRESSION + ')' if marked else SCHEMA_EXPRESSION
        source = directory/'main.tf'
        source.write_text(re.sub(r'(?m)^ schema = .+$', ' schema = ' + expression, source.read_text()))
        cls.command(directory, 'plan', '-input=false', '-detailed-exitcode',
            '-var=manage_table=' + str(managed).lower(),
            '-out=' + str(directory/'saved.tfplan'), codes=(0, 2))
        return json.loads(cls.command(directory, 'show', '-json', str(directory/'saved.tfplan')).stdout)

    def validate(self, plan, state, phase='before'):
        return contract.validate_plan(plan, manifest=self.manifest, environment='dev',
            provider_schema=self.schema, state=state, phase=phase)

    def test_native_import_identity_and_saved_plan_to_state_join(self):
        change = self.import_plan['resource_changes'][0]['change']
        self.assertEqual(change['actions'], ['no-op'])
        self.assertIsInstance(change['after_identity'], dict)
        self.assertEqual(change['before_identity'], change['after_identity'])
        result = self.validate(self.import_plan, self.before_import)
        self.assertEqual((result['imports'], result['mutations']), (1, 0))
        contract.validate_state_delta(self.before_import, self.after_import, manifest=self.manifest,
            environment='dev', provider_schema=self.schema, expected_imports={ADDRESS}, plan=self.import_plan)
        self.assertEqual(set(self.requests), {'GET'})

    def test_native_noop_observations_and_both_sensitivity_seeds(self):
        for marked in (False, True):
            with self.subTest(schema_sensitive=marked):
                directory = self.directory('marked' if marked else 'unmarked')
                self.command(directory, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
                before = deepcopy(self.after_import)
                if marked:
                    before['resources'][0]['instances'][0]['sensitive_attributes'] = [
                        [{'type': 'get_attr', 'value': 'schema'}]]
                (directory/'terraform.tfstate').write_text(json.dumps(before))
                type(self).api_version += 1
                plan = self.plan(directory, marked=marked)
                change = plan['resource_changes'][0]['change']
                self.assertEqual(change['actions'], ['no-op'])
                self.assertEqual(change['before_identity'], change['after_identity'])
                self.assertEqual(change['before_sensitive'], change['after_sensitive'])
                self.assertEqual(bool(change['after_sensitive'].get('schema', False)), marked)
                self.assertEqual(self.validate(plan, before)['raw_observation_drift'], 1)
                self.command(directory, 'apply', '-input=false', str(directory/'saved.tfplan'))
                after = self.state(directory)
                contract.validate_state_delta(before, after, manifest=self.manifest, environment='dev',
                    provider_schema=self.schema, expected_imports=set(), plan=plan)
                self.assertEqual(self.validate(self.plan(directory, marked=marked), after, 'post')['raw_observation_drift'], 0)
                counterfactual = self.plan(directory, marked=not marked)
                changed = counterfactual['resource_changes'][0]['change']
                self.assertEqual(changed['actions'], ['update'])
                self.assertEqual(changed['before'], changed['after'])
                with self.assertRaises(contract.ContractError):
                    self.validate(counterfactual, after)
        self.assertEqual(set(self.requests), {'GET'})


if __name__ == '__main__':
    unittest.main()
