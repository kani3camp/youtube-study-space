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



class BaselineDiagnosticTest(unittest.TestCase):
    def source(self):
        source = fixture.history_tests.UserActivityHistoryPlanTest()
        source.setUp()
        plan, metadata = source.fixture(False)
        plan.update(format_version='1.2', terraform_version='1.16.4', complete=True, errored=False)
        return plan, metadata

    def selectors(self):
        return dict(MODE='plan', HISTORY_TARGET='dev', OWNERSHIP_WAVE='none', HISTORY_POST_NOOP='true',
            CURRENT_STATE_ATTESTATION='true', OWNERSHIP_ROOT_BASIS='original-history-import', GITHUB_RUN_ATTEMPT='1')

    def execute(self, plan, metadata, *, operation='plan', environment='dev', phase='post', changes=None,
                raw=None, bad_inputs=False, unknown=False, serialization=False):
        from contextlib import ExitStack
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            inputs=protected.prepare(metadata);inputs.pop('manage_user_activity_history')
            if bad_inputs: inputs['user_activity_history_field_descriptions']={'dummy':CANARY}
            for name,value in [(f'user-history-{phase}.json',metadata),(f'user-history-{phase}.tfvars.json',inputs)]:
                path=root/name;path.write_text(json.dumps(value));path.chmod(0o600)
            args=['protected','--operation',operation,'--phase',phase,'--environment',environment,'--git-sha',fixture.SHA,
                  '--policy','import-only','--json-output',str(root/'summary.json'),'--markdown-output',str(root/'summary.md')]
            env=self.selectors() | dict(TF_VAR_manage_user_activity_history='true',RUNNER_TEMP=directory,
                TF_VAR_export_function_execution_service_account_email=fixture.EMAIL,
                **{'TF_VAR_manage_export_'+kind:'true' for kind in ('function','scheduler','topic')}) | (changes or {})
            with ExitStack() as stack:
                stack.enter_context(patch.dict(os.environ,env,clear=True));stack.enter_context(patch('sys.argv',args))
                stack.enter_context(patch('sys.stdin',io.StringIO(json.dumps(plan) if raw is None else raw)))
                out=stack.enter_context(patch('sys.stdout',io.StringIO()));err=stack.enter_context(patch('sys.stderr',io.StringIO()))
                approve=stack.enter_context(patch('terraform_runtime_execution.approve_plan'))
                if unknown:
                    stack.enter_context(patch('validate_user_activity_history_plan.validate_baseline',side_effect=ValueError(CANARY)))
                if serialization:stack.enter_context(patch.object(protected,'write_private',side_effect=OSError(CANARY)))
                rc=protected.main();approve.assert_not_called()
            result=dict(rc=rc,stdout=out.getvalue(),stderr=err.getvalue(),
                json=(root/'summary.json').read_text() if (root/'summary.json').exists() else None,
                markdown=(root/'summary.md').read_text() if (root/'summary.md').exists() else None,
                diagnostic=diagnostic.read(root) if (root/diagnostic.FILE).exists() else None)
            self.assertNotIn(CANARY, json.dumps(result))
            return result

    def test_success_preserves_prior_summary_bytes_and_creates_no_diagnostic(self):
        from terraform_plan_summary import build_summary, render_markdown
        plan,metadata=self.source();result=self.execute(plan,metadata)
        expected=build_summary(plan,environment='dev',git_sha=fixture.SHA,policy='import-only')
        self.assertEqual(result['rc'],0);self.assertIsNone(result['diagnostic'])
        self.assertEqual(result['json'],json.dumps(expected,sort_keys=True,separators=(',',':'))+'\n')
        self.assertEqual(result['markdown'],render_markdown(expected))
        self.assertEqual(result['stdout']+result['stderr'],'')

    def test_scope_is_exact_and_false_selectors_do_not_record_none_failures(self):
        selectors=self.selectors()
        self.assertTrue(diagnostic.baseline_scope(selectors,operation='plan',environment='dev',phase='post'))
        cases=[(key,'wrong') for key in selectors]
        for key,value in cases:
            with self.subTest(key=key):
                bad=selectors|{key:value}
                self.assertFalse(diagnostic.baseline_scope(bad,operation='plan',environment='dev',phase='post'))
                if key!='OWNERSHIP_WAVE':
                    plan,metadata=self.source();result=self.execute(plan,metadata,changes={key:value},raw='{"'+CANARY)
                    self.assertEqual(result['rc'],3);self.assertIsNone(result['diagnostic'])
        for options in (dict(operation='apply'),dict(operation='quota-plan'),dict(environment='prod'),dict(phase='before')):
            with self.subTest(options=options):
                plan,metadata=self.source();result=self.execute(plan,metadata,raw='{"'+CANARY,**options)
                self.assertEqual(result['rc'],3);self.assertIsNone(result['diagnostic'])

    def test_parse_inputs_unknown_dependency_and_serialization_stay_failed_and_value_free(self):
        cases=[(dict(raw='{"'+CANARY),'INPUT','FORMAT'),(dict(bad_inputs=True),'HISTORY_INPUTS','VALUE'),
            (dict(unknown=True),'BASELINE','UNCLASSIFIED'),(dict(serialization=True),'SERIALIZATION','UNCLASSIFIED')]
        for options,stage,field in cases:
            with self.subTest(stage=stage):
                plan,metadata=self.source();result=self.execute(plan,metadata,**options)
                self.assertEqual(result['rc'],3);self.assertIsNone(result['json']);self.assertIsNone(result['markdown'])
                self.assertEqual((result['diagnostic']['stage'],result['diagnostic']['field']),(stage,field))
                self.assertEqual(result['diagnostic']['outcome'],'STOP')

    def test_history_and_delegated_guards_identify_fixed_fields_without_changing_errors(self):
        import validate_user_activity_history_plan as history
        from terraform_export_function_gate import FUNCTION
        from terraform_export_scheduler_gate import SCHEDULER
        cases=[('function','version_id','BASELINE_FUNCTION','VERSION_ID'),('function','status','BASELINE_FUNCTION','STATUS'),
               ('scheduler','schedule','BASELINE_SCHEDULER','SCHEDULE'),('table','id','HISTORY_TABLE','ID'),
               ('table','schema','HISTORY_SCHEMA','UNCLASSIFIED'),('table','labels','HISTORY_FORBIDDEN','LABELS')]
        for target,key,stage,field in cases:
            with self.subTest(target=target,key=key):
                plan,metadata=self.source();address={'function':FUNCTION,'scheduler':SCHEDULER,'table':history.TABLE}[target]
                resource=next(r for r in plan['resource_changes'] if r['address']==address)
                for side in ('before','after'):
                    resource['change'][side][key]={'dummy':CANARY} if key=='labels' else CANARY
                errors=[]
                for context in (None,diagnostic.Context()):
                    with self.assertRaises(ValueError) as error:
                        history.validate(plan,metadata=metadata,phase='post',execution_email=fixture.EMAIL,diagnostic=context)
                    errors.append((type(error.exception),str(error.exception)))
                self.assertEqual(errors[0],errors[1]);self.assertTrue(issubclass(errors[0][0],ValueError))
                result=self.execute(plan,metadata);self.assertEqual(result['rc'],3)
                self.assertEqual((result['diagnostic']['stage'],result['diagnostic']['field']),(stage,field))
                self.assertIsNone(result['json']);self.assertIsNone(result['markdown'])

    def test_incomplete_actions_graph_and_equal_value_update_never_authorize(self):
        from terraform_export_function_gate import FUNCTION
        mutations=[(lambda p:p.update(complete=False),'BASELINE_TOPIC','COMPLETE'),
            (lambda p:p.update(action_invocations=[{}]),'HISTORY_PLAN','EMPTY_LIST'),
            (lambda p:p['resource_changes'][0].update(address=CANARY),'BASELINE_TOPIC','GRAPH'),
            (lambda p:next(r for r in p['resource_changes'] if r['address']==FUNCTION)['change'].update(actions=['update']),'BASELINE_FUNCTION','ACTION')]
        for mutation,stage,field in mutations:
            with self.subTest(stage=stage,field=field):
                plan,metadata=self.source();mutation(plan);result=self.execute(plan,metadata)
                self.assertEqual(result['rc'],3);self.assertIsNone(result['json'])
                self.assertEqual((result['diagnostic']['stage'],result['diagnostic']['field']),(stage,field))
                if stage=='BASELINE_TOPIC' and field=='COMPLETE' or stage=='HISTORY_PLAN':
                    self.assertIsNone(result['diagnostic']['counts'])

    def test_only_source_known_fields_can_be_published_and_runtime_legacy_is_unchanged(self):
        self.assertEqual(diagnostic.source_field(CANARY),'VALUE')
        context=diagnostic.Context()
        with self.assertRaises(ValueError):context.note_failure(CANARY)
        plan,metadata=self.source();result=self.execute(plan,metadata,changes={'OWNERSHIP_WAVE':'pool'},raw='{"'+CANARY)
        self.assertEqual(result['rc'],3)
        self.assertEqual((result['diagnostic']['stage'],result['diagnostic']['field']),('INPUT','UNCLASSIFIED'))
        self.assertEqual(set(result['diagnostic']),{'schema_version','outcome','stage','field','counts'})

    def test_guard_predicates_and_delegated_calls_match_reviewed_dc834_snapshot(self):
        # Security contract snapshot: changing a guard requires a new review,
        # not updating this digest merely to make a diagnostic change pass.
        import ast,hashlib
        snapshots={
          'infra/gcp/scripts/validate_user_activity_history_plan.py':('e04d01e576ec2dfeddde493a2af7c4d47c207079a281efb968615e0126fae842','a0a777ee7e4c1fe45549aac7e8a453d9cdabbcd3d220b32ee8c0c92c59f9a142'),
          '.github/scripts/terraform_export_function_gate.py':('1187e61895854b517499740b238543a7d76287f891a4ce865974cf259a26183b','80bb22a64afd70053c672b744fad4c7e90546364edf8a99d04d9eb127a485547'),
          '.github/scripts/terraform_export_scheduler_gate.py':('081f008b2d2ad02f214dcb68d287c6d04891fb2c51aa124cd085da3f71877a81','11b86897dd9cf63681c16864f790c4a75d910bd8cd3de4daf171bb1c2f92f687'),
          '.github/scripts/terraform_export_topic_gate.py':('432a650d25530ecc8dd903ac70262dd838caae2b16055b54e1cec2af3bda8127','4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945')}
        for path,(guards,delegates) in snapshots.items():
            tree=ast.parse((ROOT/path).read_text())
            actual=[ast.dump(n.args[0],include_attributes=False) for n in ast.walk(tree)
                if isinstance(n,ast.Call) and isinstance(n.func,ast.Name) and n.func.id=='require']
            calls=[]
            for n in ast.walk(tree):
                if isinstance(n,ast.Call) and isinstance(n.func,ast.Name) and n.func.id in {'validate_baseline','validate_scheduler','validate_topic'}:
                    n.keywords=[kw for kw in n.keywords if kw.arg!='diagnostic'];calls.append(ast.dump(n,include_attributes=False))
            digest=lambda v:hashlib.sha256(json.dumps(v,separators=(',',':')).encode()).hexdigest()
            self.assertEqual(digest(actual),guards);self.assertEqual(digest(calls),delegates)

    def exercise_post(self, *, failure=False, changes=None, summary_failure=False):
        from contextlib import ExitStack
        import test_terraform_history_post_noop_receipt as post_fixture
        import terraform_history_post_noop_receipt as post
        item=post_fixture.IndependentHistoryPostNoopTest();item.setUp()
        try:
            env=item.env|self.selectors()|(changes or {})
            post.before(env,request=item.aws)
            if failure:env['PLAN_VALIDATION_OUTCOME']='failure'
            with ExitStack() as stack:
                out=stack.enter_context(patch('sys.stdout',io.StringIO()))
                record=stack.enter_context(patch('terraform_current_state_attestation.record_verified'))
                if summary_failure:stack.enter_context(patch('builtins.open',side_effect=OSError(CANARY)))
                try:
                    post.after(env,request=item.aws,metadata_request=item.table_get);failed=False
                except Exception:
                    failed=True
                stdout=out.getvalue();calls=record.call_count
            summary=(item.root/'summary').read_text() if (item.root/'summary').exists() else ''
            self.assertNotIn(CANARY,stdout+summary)
            return failed,stdout,summary,calls
        finally:item.doCleanups()

    def test_post_six_fixed_statuses_are_scoped_and_failure_never_records_receipt(self):
        failed,out,summary,calls=self.exercise_post()
        self.assertFalse(failed);self.assertEqual(calls,1)
        self.assertEqual(out.splitlines(),diagnostic.render_post_checks(dict.fromkeys(diagnostic.POST_CHECKS,True)))
        failed,out,summary,calls=self.exercise_post(failure=True)
        self.assertTrue(failed);self.assertEqual(calls,0)
        self.assertIn('Current-state safety: Full-root no-op12 and job outcomes: STOP',out)
        self.assertEqual(len(out.splitlines()),6)
        failed,out,_,calls=self.exercise_post(summary_failure=True)
        self.assertTrue(failed);self.assertEqual(calls,0);self.assertEqual(len(out.splitlines()),6)
        for key,value in [('CURRENT_STATE_ATTESTATION','false'),('GITHUB_RUN_ATTEMPT','2'),('OWNERSHIP_ROOT_BASIS','current-state-bootstrap')]:
            with self.subTest(key=key):
                self.assertEqual(self.exercise_post(changes={key:value})[1],'')

    def test_post_sink_rejects_unknown_missing_and_nonboolean_without_echo(self):
        good=dict.fromkeys(diagnostic.POST_CHECKS,False)
        for bad in ({**good,CANARY:True},{k:v for k,v in good.items() if k!=diagnostic.POST_CHECKS[0]},
                    {**good,diagnostic.POST_CHECKS[0]:CANARY}):
            with self.assertRaises(ValueError):diagnostic.render_post_checks(bad)

    def test_current_state_workflow_renderer_uses_same_scope_and_keeps_failed_rc(self):
        workflow=(ROOT/'.github/workflows/gcp-terraform-authenticated.yml').read_text()
        block=workflow.split('      - name: Sanitize and enforce the selected protected contract\n',1)[1].split('\n      - name:',1)[0]
        original=textwrap.dedent(block.split('        run: |\n',1)[1])
        cases=[{},*[{key:'wrong'} for key in self.selectors() if key!='OWNERSHIP_WAVE']]
        for changes in cases:
            with self.subTest(changes=changes),tempfile.TemporaryDirectory() as directory:
                root=Path(directory);binary=root/'bin';binary.mkdir()
                executable=binary/'terraform';executable.write_text("#!/bin/sh\nprintf '%s' '{\""+CANARY+"'\n");executable.chmod(0o700)
                selected=self.selectors()|changes
                script=original.replace('${{ inputs.mode }}',selected['MODE']).replace('${{ inputs.target }}',selected['HISTORY_TARGET'])
                env=dict(os.environ,**selected,PATH=str(binary)+os.pathsep+os.environ['PATH'],RUNNER_TEMP=directory,
                    TF_ROOT='dummy',GITHUB_SHA='a'*40,GITHUB_OUTPUT=str(root/'output'),GITHUB_STEP_SUMMARY=str(root/'summary'))
                result=subprocess.run(['bash','-c',script],cwd=ROOT,env=env,capture_output=True,text=True)
                # Unsupported fake mode/target may argparse-stop with rc2;
                # diagnostic observation must preserve that original failure.
                self.assertIn(result.returncode,(2,3));self.assertNotIn(CANARY,result.stdout+result.stderr)
                if not changes:self.assertIn('stage=INPUT field=FORMAT',result.stdout)
                else:self.assertNotIn('Runtime plan diagnostic:',result.stdout)
                self.assertFalse((root/'output').exists())


if __name__ == '__main__': unittest.main()
