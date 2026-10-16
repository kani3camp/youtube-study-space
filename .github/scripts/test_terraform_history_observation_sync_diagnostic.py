#!/usr/bin/env python3
"""Invented inputs only; finite diagnostics cannot authorize any execution."""
import copy
import io
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import terraform_history_observation_sync as controller
import test_terraform_history_observation_sync as fixture

diagnostic = controller.stop_diagnostic
CANARY = fixture.CANARY


class HistorySyncDiagnosticTest(unittest.TestCase):
    def render(self, value):
        with tempfile.TemporaryDirectory() as temp:
            return '\n'.join(value.render(fixture.environment(Path(temp))))

    def test_complete_full12_projection_has_only_fixed_codes_and_no_private_values(self):
        data = fixture.DummyIO(); value = diagnostic.Diagnostic()
        value.start('OBSERVATION_VALIDATE'); value.observe(data.normal)
        text = self.render(value)
        self.assertIn('drift:1', text); self.assertIn('no-op:12', text)
        self.assertIn('role=USER_ACTIVITY_HISTORY', text)
        self.assertIn('VALUE=ETAG,LAST_MODIFIED_TIME', text)
        self.assertLessEqual(len(diagnostic.profiles.canonical(value.value())), 2048)
        for private in (CANARY, 'DUMMY_OLD', 'DUMMY_NEW', controller.gate.HISTORY, controller.gate.PROJECT):
            self.assertNotIn(private, text)

    def test_unknown_incomplete_malformed_or_overbudget_plan_is_all_unobserved(self):
        mutations = (lambda p:p.update(complete=False), lambda p:p.update(errored=True),
            lambda p:p.pop('errored'), lambda p:p.update(extra=CANARY),
            lambda p:p.update(action_invocations=[{}]), lambda p:p.update(resource_changes=[]),
            lambda p:p['resource_changes'][0]['change'].update(extra=CANARY),
            lambda p:p['resource_changes'][0]['change'].update(after_unknown=True),
            lambda p:p['resource_drift'][0]['change']['after'].update(unknown_private_key=CANARY),
            lambda p:p.update(timestamp='x'*(controller.gate.MAX_BYTES+1)),
            lambda p:p['resource_changes'][0].update(address=CANARY))
        for mutate in mutations:
            plan = copy.deepcopy(fixture.DummyIO().normal); mutate(plan)
            value = diagnostic.Diagnostic(); value.observe(plan)
            self.assertIsNone(value.counts); self.assertIsNone(value.projection)
            self.assertNotIn(CANARY,self.render(value))

    def test_output_validation_rejects_unknown_keys_enums_counts_and_partial_projection(self):
        base = diagnostic.Diagnostic().value()
        for mutate in (lambda p:p.update(extra=CANARY), lambda p:p.update(reason=CANARY),
                lambda p:p.update(step=CANARY), lambda p:p.update(plan_exit=2),
                lambda p:p.update(guard_field=CANARY), lambda p:p.update(state_before=True),
                lambda p:p.update(counts={'drift':1}), lambda p:p.update(drift={'items':[]})):
            value = copy.deepcopy(base); mutate(value)
            with self.assertRaises(ValueError):diagnostic.validate(value)

    def test_normal_exit_codes_timeouts_and_unknown_exceptions_preserve_original_stop(self):
        for result in (0,1,2,3,-9,'timeout','unknown'):
            with tempfile.TemporaryDirectory() as temp:
                root=Path(temp);value=diagnostic.Diagnostic();value.start('OBSERVATION_PLAN')
                def run(args,**kwargs):
                    kwargs['stdout'].write(CANARY.encode());kwargs['stderr'].write(CANARY.encode())
                    if result=='timeout':raise subprocess.TimeoutExpired(args,240,output=CANARY)
                    if result=='unknown':raise RuntimeError(CANARY)
                    return subprocess.CompletedProcess(args,result)
                with patch.object(controller.subprocess,'run',side_effect=run),patch('sys.stdout',io.StringIO()) as out:
                    if result==0:self.assertEqual(controller.terraform(root,root,{},'normal-plan',['plan'],codes=(0,),report=value.event),0)
                    else:
                        with self.assertRaises(Exception):controller.terraform(root,root,{},'normal-plan',['plan'],codes=(0,),report=value.event)
                expected = str(result) if type(result)is int and result in (0,1,2) else (
                    'OTHER' if type(result)is int else 'TIMEOUT' if result=='timeout' else 'NOT_OBSERVED')
                self.assertEqual(value.plan_exit,expected)
                self.assertNotIn(CANARY,out.getvalue()+self.render(value))

    def test_normal_show_parse_duplicate_nonfinite_bounds_and_partial_read_are_finite(self):
        samples = ((b'{"broken":', 'JSON_PARSE'), (b'{"a":1,"a":2}', 'JSON_PARSE'),
            (b'{"a":NaN}', 'JSON_PARSE'), (json.dumps({'a':'x'*(controller.gate.MAX_BYTES+1)}).encode(),'JSON_BOUND'))
        for raw,reason in samples:
            with tempfile.TemporaryDirectory() as temp:
                root=Path(temp);value=diagnostic.Diagnostic();value.start('OBSERVATION_SHOW')
                def run(args,**kwargs):
                    kwargs['stdout'].write(raw);return subprocess.CompletedProcess(args,0)
                with patch.object(controller.subprocess,'run',side_effect=run):
                    with self.assertRaises(Exception):controller.terraform(root,root,{},'normal-show',['show'],show=True,report=value.event)
                self.assertEqual(value.show_exit,'0');self.assertEqual(value.reason,reason)
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);value=diagnostic.Diagnostic();value.start('OBSERVATION_SHOW')
            with patch.object(controller.subprocess,'run',return_value=subprocess.CompletedProcess([],0)),\
                    patch.object(controller,'read_owned',side_effect=ValueError(CANARY)):
                with self.assertRaises(ValueError):controller.terraform(root,root,{},'normal-show',['show'],show=True,report=value.event)
            self.assertEqual(value.reason,'SHOW_READ');self.assertNotIn(CANARY,self.render(value))

    def test_configuration_replay_captures_fixed_guard_context_without_admission_or_input_mutation(self):
        data=fixture.DummyIO();plan=copy.deepcopy(data.normal)
        function=next(item for item in plan['resource_changes'] if item['type']=='google_cloudfunctions_function')
        for side in ('before','after'):function['change'][side]['version_id']=CANARY
        before=copy.deepcopy(plan)
        with self.assertRaises(controller.gate.SyncRejected) as caught:
            controller.gate.validate_observation(plan,metadata=data.metadata,
                execution_email=controller.gate.PROJECT+'@appspot.gserviceaccount.com',source_sha='a'*40)
        value=diagnostic.Diagnostic();value.start('OBSERVATION_VALIDATE')
        value.rejected(caught.exception,plan,metadata=data.metadata,execution_email=controller.gate.PROJECT+'@appspot.gserviceaccount.com')
        self.assertEqual(value.reason,'CONFIGURATION');self.assertEqual(value.guard_stage,'BASELINE_FUNCTION')
        self.assertEqual(value.guard_field,'VERSION_ID');self.assertEqual(plan,before)
        self.assertNotIn(CANARY,self.render(value))

    def test_unknown_exception_or_reason_never_uses_its_text_and_non_sync_scope_is_denied(self):
        data=fixture.DummyIO();value=diagnostic.Diagnostic();error=controller.gate.SyncRejected('SHAPE');error.reason=CANARY
        for error in (RuntimeError(CANARY),error):
            value.rejected(error,data.normal,metadata=data.metadata,execution_email='DUMMY')
            self.assertEqual(value.reason,'UNCLASSIFIED');self.assertNotIn(CANARY,self.render(value))
        with tempfile.TemporaryDirectory() as temp:
            env=fixture.environment(Path(temp))
            for key,other in (('MODE','plan'),('TARGET','prod'),('GITHUB_RUN_ATTEMPT','2'),('GITHUB_JOB','plan')):
                with self.assertRaises(ValueError):value.render(env|{key:other})

    def test_oversize_output_is_fixed_and_presend_call_adjacency_is_preserved(self):
        for bounded in (True,False):
            with tempfile.TemporaryDirectory() as temp:
                root=Path(temp);value=diagnostic.Diagnostic();value.start('OBSERVATION_PLAN');events=[]
                def run(args,**kwargs):
                    events.append('process');kwargs['stdout'].write(CANARY.encode())
                    return subprocess.CompletedProcess(args,0)
                def report(*args):events.append('report');value.event(*args)
                with patch.object(controller.subprocess,'run',side_effect=run),\
                        patch.object(controller,'MAX_LOG_BYTES',16*1024*1024 if bounded else 1):
                    call=lambda:controller.terraform(root,root,{},'normal-plan',['plan'],codes=(0,),report=report,
                        before_send=lambda:events.append('seal'),on_call=lambda:events.append('on-call'))
                    if bounded:self.assertEqual(call(),0)
                    else:
                        with self.assertRaises(ValueError):call()
                self.assertEqual(events[:3],['seal','on-call','process'])
                self.assertEqual(value.reason,'NOT_OBSERVED' if bounded else 'OUTPUT_BOUND')
                self.assertNotIn(CANARY,self.render(value))

    def test_precall_timeout_is_unobserved_and_full_static_projection_fits_two_kib(self):
        with tempfile.TemporaryDirectory() as temp:
            root=Path(temp);value=diagnostic.Diagnostic();value.start('OBSERVATION_PLAN')
            with patch.object(controller.subprocess,'run',side_effect=AssertionError('must not send')):
                with self.assertRaises(subprocess.TimeoutExpired):
                    controller.terraform(root,root,{},'normal-plan',['plan'],report=value.event,
                        before_send=lambda:(_ for _ in ()).throw(subprocess.TimeoutExpired('dummy',1)))
            self.assertEqual(value.plan_exit,'NOT_OBSERVED')
        value=diagnostic.Diagnostic();value.counts=dict(controller.gate.COUNTS);value.counts['drift']=12
        value.projection={'map_version':diagnostic.drift.MAP_VERSION,'items':[]}
        for i,(_,_,kind) in enumerate(diagnostic.drift.ALIASES):
            maximum=format((1<<len(diagnostic.drift.FIELDS[kind]))-1,'x')
            value.projection['items'].append([i,'update',3,*([maximum]*6)])
        diagnostic.validate(value.value())
        self.assertLessEqual(len(diagnostic.profiles.canonical(value.value())),2048)


if __name__=='__main__':unittest.main()
