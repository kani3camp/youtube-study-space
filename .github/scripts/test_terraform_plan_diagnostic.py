#!/usr/bin/env python3
"""Dummy-only failure diagnostics cannot publish values or permit a plan."""
import copy
import io
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import textwrap
import unittest
from unittest.mock import patch
import terraform_plan_diagnostic as diagnostic
import terraform_protected_plan as protected

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT/'infra/gcp/tests'))
import test_runtime_ownership_package as fixture

CANARY = 'arn:aws:iam::999999999999:role/PRIVATE_CANARY_nonce_member'

class DiagnosticTest(unittest.TestCase):
    def run_renderer(self, directory):
        with patch.dict(os.environ, {'RUNNER_TEMP': str(directory)}, clear=True), patch('sys.stdout', io.StringIO()) as out:
            result = diagnostic.main()
        self.assertNotIn(CANARY, out.getvalue())
        self.assertLess(len(out.getvalue()), 512)
        return result, out.getvalue()

    def test_failed_runtime_guard_preserves_stage_field_and_counts_without_values(self):
        for kind, stage, field in [('sensitivity', 'RESOURCE_CHANGE', 'SENSITIVITY'),
                                   ('description', 'RUNTIME_VALUES', 'DESCRIPTION'),
                                   ('address', 'RESOURCE_CHANGE', 'TYPE')]:
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                plan, metadata, packet = fixture.fixture()
                resource = plan['resource_changes'][-1]
                if kind == 'sensitivity':
                    resource['change']['after_sensitive'] = {'description': CANARY}
                elif kind == 'address':
                    resource['address'] = CANARY
                else:
                    for side in ('before', 'after'):
                        resource['change'][side]['description'] = CANARY
                    fixture.planned_values(plan)
                context = diagnostic.Context()
                with patch.object(fixture.preparer, 'PROJECT', fixture.PROJECT), patch.object(fixture.gate, 'PROJECT', fixture.PROJECT):
                    with self.assertRaises(ValueError) as caught:
                        fixture.gate.validate(plan, packet=packet, wave='pool', git_sha=fixture.SHA, metadata=metadata,
                            phase='before', execution_email=fixture.EMAIL, now=fixture.NOW, diagnostic=context)
                diagnostic.record(directory, plan, context, caught.exception)
                value = diagnostic.read(directory)
                self.assertEqual((value['stage'], value['field']), (stage, field))
                self.assertEqual((value['counts']['import'], value['counts']['no-op']), (1, 13))
                self.assertNotIn(CANARY, (Path(directory)/diagnostic.FILE).read_text())
                self.assertEqual(self.run_renderer(directory)[0], 0)

    def test_malformed_or_unobserved_projection_never_fabricates_counts(self):
        projections = [None, {}, {'complete':False,'resource_changes':[]},
            {'complete':True,'errored':True,'resource_changes':[]},
            *[dict(complete=True, resource_changes=[], **change) for change in (
                {'deferred_changes':[{}]}, {'action_invocations':[{}]}, {'resource_drift':None})],
            *[dict(complete=True, resource_changes=changes) for changes in (
                [None], [{'change':{}}], [{}]*10001)]]
        for plan in projections:
            self.assertIsNone(diagnostic.counts(plan))
        with tempfile.TemporaryDirectory() as directory:
            diagnostic.record(directory, None, diagnostic.Context(), ValueError(CANARY))
            self.assertIn('counts=NOT_OBSERVED', self.run_renderer(directory)[1])

    def test_strict_renderer_rejects_unknown_partial_oversized_and_private_fields(self):
        good = dict(schema_version=1, outcome='STOP', stage='INPUT', field='UNCLASSIFIED', counts=None)
        bad = [b'{"private":"'+CANARY.encode(), b'x'*2049,
               json.dumps(dict(good, stage=CANARY)).encode(), json.dumps(dict(good, field=CANARY)).encode(),
               json.dumps(dict(good, message=CANARY)).encode(), json.dumps(dict(good, outcome='PASS')).encode(),
               json.dumps(dict(good, counts=dict.fromkeys(diagnostic.COUNT_KEYS, True))).encode(),
               json.dumps(dict(good, counts=dict.fromkeys(diagnostic.COUNT_KEYS, 10001))).encode(),
               b'{"schema_version":1,"schema_version":1}', b'{"counts":NaN}']
        for raw in bad:
            with self.subTest(raw_length=len(raw)), tempfile.TemporaryDirectory() as directory:
                path=Path(directory)/diagnostic.FILE;path.write_bytes(raw);path.chmod(0o600)
                code, line = self.run_renderer(directory)
                self.assertEqual(code, 3)
                self.assertIn('stage=NOT_OBSERVED field=NOT_OBSERVED counts=NOT_OBSERVED', line)

    def test_unsafe_or_missing_files_and_overwrite_are_rejected(self):
        for kind in ('missing','symlink','hardlink','mode'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                path=Path(directory)/diagnostic.FILE
                if kind != 'missing':
                    diagnostic.record(directory, None, diagnostic.Context(), ValueError(CANARY))
                    if kind == 'mode': path.chmod(0o644)
                    if kind == 'hardlink': os.link(path, Path(directory)/'duplicate')
                    if kind == 'symlink':
                        path.rename(Path(directory)/'target');path.symlink_to(Path(directory)/'target')
                self.assertEqual(self.run_renderer(directory)[0], 3)
        with tempfile.TemporaryDirectory() as directory:
            diagnostic.record(directory, None, diagnostic.Context(), ValueError(CANARY))
            with self.assertRaises(FileExistsError):
                diagnostic.record(directory, None, diagnostic.Context(), ValueError(CANARY))

    def test_protected_entrypoint_keeps_partial_json_stop_and_no_summary(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            args=['protected','--operation','plan','--phase','before','--environment','dev','--git-sha','a'*40,
                  '--policy','import-only','--json-output',str(root/'summary.json'),'--markdown-output',str(root/'summary.md')]
            with patch.dict(os.environ, {'RUNNER_TEMP':directory,'OWNERSHIP_WAVE':'pool'}, clear=True), \
                 patch('sys.argv', args), patch('sys.stdin',io.StringIO('{"'+CANARY)), \
                 patch('sys.stdout',io.StringIO()) as out, patch('sys.stderr',io.StringIO()) as err:
                self.assertEqual(protected.main(),3)
            self.assertEqual(out.getvalue(),'')
            self.assertNotIn(CANARY,err.getvalue())
            self.assertFalse((root/'summary.json').exists());self.assertFalse((root/'summary.md').exists())
            self.assertIn('stage=INPUT',self.run_renderer(root)[1])

    def test_runtime_failure_survives_protected_wrapper_without_approval(self):
        plan, metadata, packet = fixture.fixture()
        plan['resource_changes'][-1]['change']['after_sensitive'] = {'description':CANARY}
        with tempfile.TemporaryDirectory() as directory, \
             patch.object(fixture.preparer,'PROJECT',fixture.PROJECT), patch.object(fixture.gate,'PROJECT',fixture.PROJECT):
            root=Path(directory)
            candidate=fixture.packet_module.validate_packet(packet,wave='pool',git_sha=fixture.SHA,now=fixture.NOW)
            inputs=protected.prepare(metadata);inputs.pop('manage_user_activity_history')
            for name,value in [('user-history-before.json',metadata),('user-history-before.tfvars.json',inputs),
                               ('runtime-ownership-packet.json',packet),('runtime-ownership.tfvars.json',candidate)]:
                path=root/name;path.write_text(json.dumps(value));path.chmod(0o600)
            args=['protected','--operation','plan','--phase','before','--environment','dev','--git-sha',fixture.SHA,
                  '--policy','import-only','--json-output',str(root/'summary.json'),'--markdown-output',str(root/'summary.md')]
            env=dict(TF_VAR_manage_user_activity_history='true',OWNERSHIP_WAVE='pool',RUNNER_TEMP=directory,
                TF_VAR_export_function_execution_service_account_email=fixture.EMAIL,
                **{'TF_VAR_manage_export_'+kind:'true' for kind in ('function','scheduler','topic')})
            packet_validator, runtime_validator = protected.validate_packet, protected.validate_runtime
            with patch.dict(os.environ,env,clear=True),patch('sys.argv',args),patch('sys.stdin',io.StringIO(json.dumps(plan))), \
                 patch.object(protected,'require_verified_inputs'),patch('terraform_runtime_execution.require_fresh_inputs'), \
                 patch('terraform_runtime_execution.approve_plan') as approve, \
                 patch.object(protected,'validate_packet',side_effect=lambda p,**kw:packet_validator(p,now=fixture.NOW,**kw)), \
                 patch.object(protected,'validate_runtime',side_effect=lambda p,**kw:runtime_validator(p,now=fixture.NOW,**kw)), \
                 patch('sys.stdout',io.StringIO()) as out,patch('sys.stderr',io.StringIO()) as err:
                self.assertEqual(protected.main(),3)
                approve.assert_not_called()
            self.assertEqual(out.getvalue(),'');self.assertNotIn(CANARY,err.getvalue())
            self.assertFalse((root/'summary.json').exists());self.assertFalse((root/'summary.md').exists())
            self.assertIn('stage=RESOURCE_CHANGE field=SENSITIVITY',self.run_renderer(root)[1])

    def test_workflow_failure_renderer_cannot_change_failed_pipeline_status(self):
        workflow=(ROOT/'.github/workflows/gcp-terraform-authenticated.yml').read_text()
        block=workflow.split('      - name: Sanitize and enforce the selected protected contract\n',1)[1].split('\n      - name:',1)[0]
        script=textwrap.dedent(block.split('        run: |\n',1)[1])
        for expression,value in [('${{ inputs.mode }}','plan'),('${{ inputs.target }}','dev')]:
            script=script.replace(expression,value)
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);binary=root/'bin';binary.mkdir()
            executable=binary/'terraform';executable.write_text("#!/bin/sh\nprintf '%s' '{\""+CANARY+"'\n");executable.chmod(0o700)
            env=dict(os.environ, PATH=str(binary)+os.pathsep+os.environ['PATH'], RUNNER_TEMP=directory,
                TF_ROOT='dummy', HISTORY_POST_NOOP='false', OWNERSHIP_WAVE='pool', GITHUB_SHA='a'*40,
                GITHUB_OUTPUT=str(root/'output'), GITHUB_STEP_SUMMARY=str(root/'summary'))
            result=subprocess.run(['bash','-c',script],cwd=ROOT,env=env,capture_output=True,text=True)
            self.assertEqual(result.returncode,3)
            self.assertIn('stage=INPUT',result.stdout)
            self.assertNotIn(CANARY,result.stdout+result.stderr)
            self.assertFalse((root/'summary').exists());self.assertFalse((root/'output').exists())
        for job in (workflow.split('  plan:\n',1)[1].split('  current_state_read:\n',1)[0], workflow.split('  apply:\n',1)[1]):
            self.assertIn('"${RUNNER_TEMP}/'+diagnostic.FILE+'"',job.split('Cleanup sensitive temporary files',1)[1])

if __name__ == '__main__': unittest.main()
