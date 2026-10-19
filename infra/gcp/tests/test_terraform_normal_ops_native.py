#!/usr/bin/env python3
"""Real Core/Google-provider CRUD plans against an isolated loopback dummy API.

No cloud credentials/backend; all non-init egress is denied by a local proxy.
The dummy API contains only this fixture's invented Pub/Sub topic.
"""
import json
import os
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
from threading import Thread
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'scripts'))
import terraform_normal_ops as ops


class NativeCRUD(unittest.TestCase):
    def test_native_create_update_replace_delete_and_post_noop(self):
        topics = {}
        requests = []

        class API(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def reply(self, value, status=200):
                raw = json.dumps(value).encode()
                self.send_response(status)
                self.send_header('Content-Type', 'application/json')
                self.send_header('Content-Length', str(len(raw)))
                self.end_headers(); self.wfile.write(raw)

            def do_GET(self):
                key = self.path.split('?', 1)[0].removeprefix('/v1/').lstrip('/')
                requests.append(('GET', key))
                self.reply(topics[key] if key in topics else {'error': {'code': 404}}, 200 if key in topics else 404)

            def do_PUT(self):
                key = self.path.split('?', 1)[0].removeprefix('/v1/').lstrip('/')
                value = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                topics[key] = value | {'name': key}
                requests.append(('PUT', key)); self.reply(topics[key])

            def do_PATCH(self):
                value = json.loads(self.rfile.read(int(self.headers['Content-Length'])))['topic']
                key = self.path.split('?', 1)[0].removeprefix('/v1/').lstrip('/')
                topics[key].update(value)
                requests.append(('PATCH', key)); self.reply(topics[key])

            def do_DELETE(self):
                key = self.path.split('?', 1)[0].removeprefix('/v1/').lstrip('/')
                topics.pop(key)
                requests.append(('DELETE', key)); self.reply({})

            def do_CONNECT(self):
                self.reply({'error': 'external egress denied'}, 403)

            do_POST = do_CONNECT

        server = HTTPServer(('127.0.0.1', 0), API)
        self.addCleanup(server.server_close)
        self.addCleanup(server.shutdown)
        Thread(target=server.serve_forever, daemon=True).start()
        endpoint = f'http://127.0.0.1:{server.server_port}/'
        with tempfile.TemporaryDirectory(prefix='normal-native-') as directory:
            root = Path(directory)
            terraform = shutil.which('terraform')
            self.assertIsNotNone(terraform)
            shutil.copyfile(ops.REPO / 'infra/gcp/environments/dev/.terraform.lock.hcl', root / '.terraform.lock.hcl')
            cli = root / 'terraform.rc'
            cli.write_text('disable_checkpoint = true\n')
            clean = {'PATH': os.environ.get('PATH', os.defpath), 'HOME': directory,
                'TMPDIR': directory, 'TF_CLI_CONFIG_FILE': str(cli), 'TF_INPUT': 'false',
                'TF_IN_AUTOMATION': 'true', 'CHECKPOINT_DISABLE': '1',
                'NO_PROXY': '127.0.0.1,localhost', 'no_proxy': '127.0.0.1,localhost'}
            (root / 'main.tf').write_text('''terraform {
  required_version = "= ''' + ops.common.VERSION + '''"
  required_providers { google = { source = "hashicorp/google" } }
}
provider "google" {
  project = "''' + ops.PROJECT + '''"
  access_token = "DUMMY_UNUSABLE_TOKEN"
  pubsub_custom_endpoint = "''' + endpoint + '''"
}
variable "name" { default = "fixture-a" }
variable "label" { default = "one" }
variable "enabled" { default = true }
resource "google_pubsub_topic" "probe" {
  count = var.enabled ? 1 : 0
  name = var.name
  project = "''' + ops.PROJECT + '''"
  labels = { verification = var.label }
}
''')
            def command(*args, codes=(0,)):
                environment = dict(clean)
                if args[0] != 'init':
                    environment.update({k: endpoint for k in ('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY',
                        'http_proxy', 'https_proxy', 'all_proxy')})
                result = subprocess.run([terraform, '-chdir=' + directory, *args],
                    env=environment, capture_output=True, timeout=120)
                self.assertIn(result.returncode, codes, 'native fixture failed: ' + args[0] + '\n' + result.stderr.decode())
                return result.stdout
            command('init', '-backend=false', '-input=false', '-lockfile=readonly')
            schema = json.loads(command('providers', 'schema', '-json'))
            address = 'google_pubsub_topic.probe[0]'
            for phase, variables, actions in [
                ('create', [], ['create']),
                ('update', ['-var=label=two'], ['update']),
                ('replace', ['-var=label=two', '-var=name=fixture-b'], ['delete', 'create']),
                ('delete', ['-var=enabled=false'], ['delete'])]:
                with self.subTest(phase=phase):
                    command('plan', '-input=false', '-detailed-exitcode', *variables, '-out=saved.tfplan', codes=(2,))
                    p = json.loads(command('show', '-json', 'saved.tfplan'))
                    self.assertEqual(p['resource_changes'][0]['change']['actions'], actions)
                    approvals = {address: actions} if 'delete' in actions else {}
                    ops.validate(p, approvals)
                    self.assertEqual(ops.summarize_changes(p, schema), [
                        {'resource_type': 'google_pubsub_topic', 'actions': actions, 'count': 1}])
                    command('apply', '-input=false', 'saved.tfplan')
                    command('plan', '-input=false', '-detailed-exitcode', *variables, '-out=post.tfplan')
                    post = json.loads(command('show', '-json', 'post.tfplan'))
                    ops.validate(post, {}, post=True)
                    self.assertEqual(ops.summarize_changes(post, schema), [])
            self.assertEqual(topics, {})
            self.assertEqual([m for m, _ in requests if m != 'GET'], ['PUT', 'PATCH', 'DELETE', 'PUT', 'DELETE'])
            self.assertEqual(json.loads((root / 'terraform.tfstate').read_text())['resources'], [])


if __name__ == '__main__':
    unittest.main()
