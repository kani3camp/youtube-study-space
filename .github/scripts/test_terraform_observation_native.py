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
import sys
import tempfile
from threading import Thread
import unittest


ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
import terraform_normal_ops as normal
PROJECT = normal.PROJECT
IDENTIFIER = f'projects/{PROJECT}/datasets/fixture/tables/native_identity_witness'
TOKEN = 'DUMMY_UNUSABLE_NATIVE_WITNESS_TOKEN'
SCHEMA_EXPRESSION = 'jsonencode([{name="fixture",type="STRING",mode="NULLABLE"}])'


class NativeObservationFidelity(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix='gcp-normal-native-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.base = Path(cls.temp.name).resolve()
        cls.binary = shutil.which('terraform')
        if cls.binary is None:
            raise AssertionError('Pinned Terraform is required; native witness must not skip')
        cls.api_version, cls.requests, cls.blocked_egress = 1, [], []
        cls.metric_growth = False

        class DummyAPI(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_GET(self):
                cls.requests.append(self.command)
                path = self.path.split('?', 1)[0]
                if path != '/' + IDENTIFIER:
                    self.send_error(404)
                    return
                self.reply({
                    'kind': 'bigquery#table',
                    'id': PROJECT + ':fixture.native_identity_witness',
                    'tableReference': {'projectId': PROJECT, 'datasetId': 'fixture',
                                       'tableId': path.rsplit('/', 1)[1]},
                    'creationTime': '1700000000000',
                    'lastModifiedTime': str(1700000000000 + cls.api_version),
                    'etag': 'DUMMY_ETAG_' + str(cls.api_version),
                    'numBytes': str(cls.api_version * 128 if cls.metric_growth else 0),
                    'numLongTermBytes': str(cls.api_version * 64 if cls.metric_growth else 0),
                    'numRows': str(cls.api_version if cls.metric_growth else 0), 'location': 'asia-southeast2',
                    'type': 'TABLE', 'schema': {'fields': [
                        {'name': 'fixture', 'type': 'STRING', 'mode': 'NULLABLE'}]},
                })

            def reply(self, value):
                raw = json.dumps(value).encode()
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def reject_write(self):
                cls.requests.append(self.command)
                self.send_error(405)

            def do_CONNECT(self):
                # Separate denied proxy egress from requests to the dummy API.
                cls.blocked_egress.append(self.path)
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
        if version['terraform_version'] != normal.common.VERSION:
            raise AssertionError('Terraform version must match the protected contract')
        cls.command(cls.seed, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
        cls.schema = json.loads(cls.command(cls.seed, 'providers', 'schema', '-json').stdout)
        cls.command(cls.seed, 'apply', '-input=false', '-auto-approve')
        cls.import_plan = cls.plan(cls.seed, managed=True)
        if cls.import_plan['resource_changes'][0]['change']['actions'] != ['no-op']:
            raise AssertionError('Native import witness must contain no resource mutation')
        cls.command(cls.seed, 'apply', '-input=false', str(cls.seed/'saved.tfplan'))
        cls.after_import = cls.state(cls.seed)

    @classmethod
    def command(cls, directory, *args, codes=(0,)):
        env = dict(cls.env)
        if args[0] != 'init':
            # Init may download the public locked provider. Every later provider
            # process also denies unknown external endpoints via loopback proxy.
            env.update({key: cls.endpoint for key in (
                'HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'http_proxy', 'https_proxy', 'all_proxy')})
        result = subprocess.run([cls.binary, '-chdir=' + str(directory), *args], env=env,
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
 required_version = "= ''' + normal.common.VERSION + '''"
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


    def test_native_normal_plan_with_row_and_storage_growth(self):
        directory = self.directory('normal-metric-growth')
        self.command(directory, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
        before = deepcopy(self.after_import)
        (directory/'terraform.tfstate').write_text(json.dumps(before))
        type(self).metric_growth = True
        self.addCleanup(setattr, type(self), 'metric_growth', False)
        value = self.plan(directory)
        self.assertEqual(value['resource_changes'][0]['change']['actions'], ['no-op'])
        change = value['resource_drift'][0]['change']
        differing = {key for key in change['before'] if change['before'][key] != change['after'][key]}
        self.assertTrue({'num_bytes', 'num_long_term_bytes', 'num_rows'} <= differing)
        self.assertEqual(normal.validate(value, {}, project=PROJECT,
            state=before, provider_schema=self.schema)['no-op'], 1)

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
                # Ordinary operations must admit the same State-bound,
                # computed-only observations without admitting managed drift.
                normal.validate_observation_drift(plan, before, self.schema)
                self.assertEqual(normal.validate(plan, {}, project=PROJECT,
                    state=before, provider_schema=self.schema)['no-op'], 1)
                self.command(directory, 'apply', '-input=false', str(directory/'saved.tfplan'))
                after = self.state(directory)
                post = self.plan(directory, marked=marked)
                self.assertEqual(normal.validate(post, {}, project=PROJECT, state=after,
                    provider_schema=self.schema, post=True)['no-op'], 1)
                self.assertFalse(post.get('resource_drift'))
                counterfactual = self.plan(directory, marked=not marked)
                changed = counterfactual['resource_changes'][0]['change']
                self.assertEqual(changed['actions'], ['update'])
                self.assertEqual(changed['before'], changed['after'])
                with self.assertRaises(ValueError):
                    normal.validate(counterfactual, {}, project=PROJECT, state=after,
                        provider_schema=self.schema, post=True)
        self.assertEqual(set(self.requests), {'GET'})


if __name__ == '__main__':
    unittest.main()
