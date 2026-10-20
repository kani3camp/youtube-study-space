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

import terraform_migration_contract as contract

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / 'infra/gcp/scripts'))
import terraform_normal_ops as normal
PROJECT = contract.ENVIRONMENTS['dev'][0]
ADDRESS = 'google_bigquery_table.retained[0]'
IDENTIFIER = f'projects/{PROJECT}/datasets/fixture/tables/native_identity_witness'
TOKEN = 'DUMMY_UNUSABLE_NATIVE_WITNESS_TOKEN'
SCHEMA_EXPRESSION = 'jsonencode([{name="fixture",type="STRING",mode="NULLABLE"}])'
RUNTIME_NUMBER = '111111111111'
RUNTIME_POOL = f'projects/{RUNTIME_NUMBER}/locations/global/workloadIdentityPools/aws-runtime'
RUNTIME_PROVIDER = RUNTIME_POOL + '/providers/aws-provider'
RUNTIME_INVENTORY = {
    'project_number': RUNTIME_NUMBER,
    'pool': {'display_name': 'Dummy pool', 'description': 'Dummy fixture',
             'disabled': False, 'mode': 'FEDERATION_ONLY'},
    'provider': {'display_name': 'Dummy provider', 'description': 'Dummy fixture',
                 'disabled': False, 'aws_account_id': '222222222222',
                 'attribute_mapping': {'google.subject': 'assertion.arn',
                                       'attribute.aws_role': 'assertion.arn',
                                       'attribute.account': 'assertion.account'},
                 'attribute_condition': "assertion.account == '222222222222'"},
    'service_account_id': f'projects/{PROJECT}/serviceAccounts/{PROJECT}@appspot.gserviceaccount.com',
    'grants': {},
}


class NativeMigrationFidelity(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.temp = tempfile.TemporaryDirectory(prefix='gcp-migration-native-')
        cls.addClassCleanup(cls.temp.cleanup)
        cls.base = Path(cls.temp.name).resolve()
        cls.binary = shutil.which('terraform')
        if cls.binary is None:
            raise AssertionError('Pinned Terraform is required; native witness must not skip')
        cls.api_version, cls.requests, cls.blocked_egress = 1, [], []

        class DummyAPI(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_GET(self):
                cls.requests.append(self.command)
                path = self.path.split('?', 1)[0]
                if path.startswith('/iam/'):
                    pool = '/iam/projects/' + PROJECT + '/locations/global/workloadIdentityPools/aws-runtime'
                    if path == pool:
                        value = {'name': RUNTIME_POOL, 'state': 'ACTIVE', 'disabled': False,
                                 'displayName': 'Dummy pool', 'description': 'Dummy fixture',
                                 'mode': 'FEDERATION_ONLY'}
                    elif path == pool + ':listAttestationRules':
                        value = {'attestationRules': []}
                    elif path == pool + '/providers/aws-provider':
                        provider = RUNTIME_INVENTORY['provider']
                        value = {'name': RUNTIME_PROVIDER, 'state': 'ACTIVE', 'disabled': False,
                                 'displayName': provider['display_name'], 'description': provider['description'],
                                 'aws': {'accountId': provider['aws_account_id']},
                                 'attributeMapping': provider['attribute_mapping'],
                                 'attributeCondition': provider['attribute_condition']}
                    else:
                        self.send_error(404)
                        return
                    self.reply(value)
                    return
                if path not in ('/' + IDENTIFIER, '/' + IDENTIFIER + '_dependent'):
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
                    'numBytes': '0', 'numRows': '0', 'location': 'asia-southeast2',
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
 required_version = "= ''' + contract.TERRAFORM_VERSION + '''"
 required_providers { google = { source = "hashicorp/google", version = "= 8.5.0" } }
}
provider "google" {
 project = "''' + PROJECT + '''"
 access_token = "''' + TOKEN + '''"
 big_query_custom_endpoint = "''' + cls.endpoint + '''"
 iam_custom_endpoint = "''' + cls.endpoint + '''iam/"
 iam_beta_custom_endpoint = "''' + cls.endpoint + '''iam/"
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
                # Ordinary operations must admit the same State-bound,
                # computed-only observations without admitting managed drift.
                normal.validate_observation_drift(plan, before, self.schema)
                self.assertEqual(normal.validate(plan, {}, project=PROJECT,
                    state=before, provider_schema=self.schema)['no-op'], 1)
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


    def test_native_existing_pool_and_one_provider_import_remain_exact(self):
        directory = self.directory('runtime-provider')
        (directory/'terraform.tfstate').write_text(json.dumps(self.after_import))
        source = directory/'main.tf'
        source.write_text(source.read_text() + '''
variable "own_provider" { default = false }
module "runtime_wif" {
 source = ''' + json.dumps(str(ROOT/'infra/gcp/modules/runtime-aws-wif')) + '''
 project_id = var.project_id
 own_pool = true
 own_provider = var.own_provider
 inventory = ''' + json.dumps(RUNTIME_INVENTORY) + '''
}
import {
 to = module.runtime_wif.google_iam_workload_identity_pool.runtime[0]
 id = "projects/''' + PROJECT + '''/locations/global/workloadIdentityPools/aws-runtime"
}
import {
 for_each = var.own_provider ? toset(["existing"]) : toset([])
 to = module.runtime_wif.google_iam_workload_identity_pool_provider.runtime[0]
 id = "projects/''' + PROJECT + '''/locations/global/workloadIdentityPools/aws-runtime/providers/aws-provider"
}
''')
        self.command(directory, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
        self.command(directory, 'apply', '-input=false', '-auto-approve', '-var=manage_table=true')
        before = self.state(directory)
        manifest = deepcopy(self.manifest)
        for kind, suffix in (('google_iam_workload_identity_pool', ''),
                             ('google_iam_workload_identity_pool_provider', '/providers/aws-provider')):
            manifest['resources'].append({'address': f'module.runtime_wif.{kind}.runtime[0]', 'type': kind,
                'import_id': f'projects/{PROJECT}/locations/global/workloadIdentityPools/aws-runtime' + suffix,
                'schema_version': 0})
        projections = []
        for _ in range(12):
            self.command(directory, 'plan', '-input=false', '-detailed-exitcode', '-var=manage_table=true',
                         '-var=own_provider=true', '-out=' + str(directory/'saved.tfplan'), codes=(0, 2))
            plan = json.loads(self.command(directory, 'show', '-json', str(directory/'saved.tfplan')).stdout)
            summary = contract.validate_plan(plan, manifest=manifest, environment='dev',
                                             provider_schema=self.schema, state=before)
            self.assertEqual((summary['no_op'], summary['imports'], summary['mutations']), (3, 1, 0))
            projections.append(contract.plan_commitment_payload(plan, manifest, 'dev', self.schema, state=before))
        # No claim that Core always emits references for accepted no-ops. The
        # genuine import witness must stay accepted and fully bound either way.
        self.assertTrue(all(contract.canonical(item) == contract.canonical(projections[0]) for item in projections))
        self.assertEqual(self.state(directory), before)
        self.assertEqual(set(self.requests), {'GET'})

    def test_native_inactive_output_references_keep_zero_instance_admission(self):
        fixture = self.base / "inactive-output-reference"
        (fixture / "environments").mkdir(parents=True)
        directory = self.directory("inactive-output-reference/environments/dev")
        for name in ("youtube-quota-alerts", "monitoring-notification-channels"):
            shutil.copytree(ROOT / "infra/gcp/modules" / name, fixture / "modules" / name)
        source = directory / "main.tf"
        source.write_text(source.read_text() + """
module "existing_youtube_quota_alerts" {
 source = "../../modules/youtube-quota-alerts"
 project_id = var.project_id
 environment = "development"
 manage_policies = false
 policy_settings = {
  minute_80 = { threshold = 0.8, duration = "60s" }
  day_80 = { threshold = 0.8, duration = "60s" }
  day_60 = { threshold = 0.6, duration = "60s" }
 }
}
module "youtube_quota_alerts" {
 source = "../../modules/youtube-quota-alerts"
 project_id = var.project_id
 environment = "development"
 manage_policies = false
 notification_channels = [for name in [module.notification_channels.primary_email_name] : name if name != null]
 policy_settings = {
  minute_80 = { threshold = 0.8, duration = "60s" }
  day_80 = { threshold = 0.8, duration = "60s" }
  day_60 = { threshold = 0.6, duration = "60s" }
 }
}
module "notification_channels" {
 source = "../../modules/monitoring-notification-channels"
 project_id = var.project_id
 environment = "development"
 manage_primary_email = false
}
""")
        self.command(directory, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
        before = deepcopy(self.before_import)
        (directory / 'terraform.tfstate').write_text(json.dumps(before))
        plan = self.plan(directory)
        entries = plan.get('relevant_attributes', [])
        catalogued = {item['resource'] for item in entries} & set(contract.INACTIVE_REFERENCES)
        self.assertEqual(catalogued, set(contract.INACTIVE_REFERENCES))
        result = self.validate(plan, before)
        self.assertEqual((result['imports'], result['no_op'], result['mutations']), (1, 1, 0))
        projection = contract.plan_commitment_payload(plan, self.manifest, 'dev', self.schema, state=before)
        self.assertEqual(projection['relevant_attributes'], sorted(entries, key=contract.canonical))
        self.assertEqual(self.state(directory), before)
        self.assertEqual(set(self.requests), {'GET'})

    def test_native_reference_metadata_keeps_every_entry_but_rejects_configured_update(self):
        directory = self.directory('reference-metadata')
        (directory/'terraform.tfstate').write_text(json.dumps(self.after_import))
        source = directory/'main.tf'
        source.write_text(source.read_text() + '''
resource "google_bigquery_table" "dependent" {
 count = var.manage_table ? 1 : 0
 project = one(google_bigquery_table.retained).project
 dataset_id = google_bigquery_table.retained[0].dataset_id
 table_id = "native_identity_witness_dependent"
 schema = ''' + SCHEMA_EXPRESSION + '''
 deletion_protection = google_bigquery_table.retained[0].deletion_protection
 lifecycle { prevent_destroy = true }
}
import {
 for_each = var.manage_table ? toset(["existing"]) : toset([])
 to = google_bigquery_table.dependent[0]
 id = "''' + IDENTIFIER + '''_dependent"
}
''')
        self.command(directory, 'init', '-backend=false', '-input=false', '-lockfile=readonly')
        self.command(directory, 'apply', '-input=false', '-auto-approve', '-var=manage_table=true')
        before = self.state(directory)
        manifest = deepcopy(self.manifest)
        manifest['resources'].append({'address': 'google_bigquery_table.dependent[0]',
            'type': 'google_bigquery_table', 'import_id': IDENTIFIER + '_dependent'})
        # This separate native metadata producer is deliberately NOT an accepted
        # migration: an actual schema update must fail before any apply.
        text = source.read_text()
        target = 'table_id = "native_identity_witness_dependent"\n schema = ' + SCHEMA_EXPRESSION
        changed = 'table_id = "native_identity_witness_dependent"\n schema = jsonencode([{name="fixture",type="STRING",mode="NULLABLE"},{name="extra_fixture",type="STRING",mode="NULLABLE"}])'
        self.assertEqual(text.count(target), 1)
        source.write_text(text.replace(target, changed))
        projections = []
        raw_entries = []
        for _ in range(8):
            self.command(directory, 'plan', '-input=false', '-detailed-exitcode', '-var=manage_table=true',
                         '-out=' + str(directory/'saved.tfplan'), codes=(0, 2))
            plan = json.loads(self.command(directory, 'show', '-json', str(directory/'saved.tfplan')).stdout)
            self.assertTrue(any(item['change']['actions'] == ['update'] for item in plan['resource_changes']))
            with self.assertRaises(contract.ContractError) as error:
                contract.validate_plan(plan, manifest=manifest, environment='dev', provider_schema=self.schema, state=before)
            self.assertEqual(error.exception.reason, 'ACTION')
            entries = plan.get('relevant_attributes', [])
            self.assertEqual(len(entries), 3)
            # one(resource) is a legitimate whole-resource reference. Core emits
            # its NoKey form even though the State owns retained[0].
            self.assertEqual(sum(entry['resource'] == 'google_bigquery_table.retained'
                                 and entry['attribute'] == [] for entry in entries), 1)
            projection = contract._comparison_projection(plan, contract.validate_manifest(manifest, 'dev'), self.schema)
            self.assertEqual(projection['relevant_attributes'], sorted(entries, key=contract.canonical))
            raw_entries.append(sorted(entries, key=contract.canonical))
            projections.append(projection)
        self.assertTrue(all(item == raw_entries[0] for item in raw_entries))
        self.assertTrue(all(contract.canonical(item) == contract.canonical(projections[0]) for item in projections))
        self.assertEqual(self.state(directory), before)
        self.assertEqual(set(self.requests), {'GET'})


if __name__ == '__main__':
    unittest.main()
