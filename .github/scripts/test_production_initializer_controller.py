#!/usr/bin/env python3
"""Offline official-controller initialization: no real process or cloud call."""
from __future__ import annotations
from copy import deepcopy
from contextlib import redirect_stdout, redirect_stderr
import io,json,os,subprocess,sys,unittest
from pathlib import Path
from unittest.mock import patch
sys.dont_write_bytecode=True
import terraform_migration as migration
import test_production_initializer as fixtures
import test_terraform_migration as ordinary

class InitializerControllerTest(ordinary.OfflineTest):
    def prepare(self, *, role='apply', existing=False, edit_plan=None, edit_after=None):
        self.root=self.directory/'execution';self.root.mkdir(mode=0o700)
        self.env=ordinary.environment(target='prod',role=role)|{'MIGRATION_PRODUCTION_READY':'true','RUNNER_TEMP':str(self.directory),'GITHUB_OUTPUT':str(self.directory/'output')}
        self.plan,self.schema=fixtures.configured_hints(post=existing)
        self.post,_=fixtures.configured_hints(post=True)
        if edit_plan:edit_plan(self.plan)
        state=fixtures.state()
        if edit_after:edit_after(state)
        self.after={'head':{'VersionId':'new-native-version'},'raw':migration.canonical(state).decode(),'state':state}
        self.before=deepcopy(self.after if existing else migration.initializer.ABSENT)
        self.applied=False;self.calls=[];self.snapshot_names=[]
        source_name='infra/gcp/environments/prod/main.tf';source=self.root/'checkout'/source_name;source.parent.mkdir(parents=True);source.write_bytes(b'reviewed-source')
        self.closure={source_name:migration.hashlib.sha256(source.read_bytes()).hexdigest()}
        def snapshot(_env,_root,name):
            self.snapshot_names.append(name);return deepcopy(self.after if self.applied else self.before)
        def terraform(_env,_root,_tfroot,name,args,**kwargs):
            self.calls.append((name,args))
            if name in {'plan','post-plan'}:Path(next(a.split('=',1)[1] for a in args if a.startswith('-out='))).write_bytes(b'private-saved-plan')
            if name=='apply':self.applied=True
            output=self.schema if name=='schema' else self.post if name=='post-show' else self.plan if name=='show' else {}
            return subprocess.CompletedProcess(args,0,migration.canonical(output),b'')
        for name,value in [('verify_identity',None),('verify_permissions',None),('private_root',self.root),('collect_metadata',{}),('copy_source',(self.root,self.closure))]:
            self.stack.enter_context(patch.object(migration,name,return_value=value))
        self.stack.enter_context(patch.object(migration,'snapshot',side_effect=snapshot))
        self.stack.enter_context(patch.object(migration,'terraform',side_effect=terraform))
        self.stack.enter_context(patch.object(migration,'run',return_value=subprocess.CompletedProcess([],0,ordinary.SHA.encode(),b'')))
        validator=migration.contract if existing else migration.initializer
        payload={'source':ordinary.SHA,'closure':self.closure,'inputs':migration.inputs(self.env),'state':self.before,'metadata':{},'plan':validator.plan_commitment_payload(self.plan,manifest=fixtures.manifest(),environment='prod',provider_schema=self.schema,state=self.before['state'])}
        self.payload=payload
        commitment,components=migration.execution_commitment(migration.inputs(self.env),payload)
        self.env.update(MIGRATION_EXPECTED_COMMITMENT=commitment,MIGRATION_EXPECTED_COMPONENTS=json.dumps(components))
    def execute(self):
        out,err=io.StringIO(),io.StringIO()
        with patch.dict(os.environ,self.env,clear=True),patch.object(sys,'argv',['controller','execute']),redirect_stdout(out),redirect_stderr(err):result=migration.main()
        self.assertNotIn(fixtures.CANARY,out.getvalue()+err.getvalue());self.assertEqual(err.getvalue(),'');return result,out.getvalue()
    def test_positive_absence_plan_only_publishes_seals_without_State_or_apply(self):
        self.prepare(role='plan');result,out=self.execute();self.assertEqual(result,0);self.assertIn('"output_creates":2',out)
        self.assertFalse(self.applied);self.assertNotIn('apply',[name for name,_ in self.calls]);self.assertEqual(self.snapshot_names,['before','after-plan'])
        published=dict(row.split('=',1) for row in Path(self.env['GITHUB_OUTPUT']).read_text().splitlines());self.assertEqual(published['commitment'],self.env['MIGRATION_EXPECTED_COMMITMENT']);self.assertEqual(json.loads(published['components']),json.loads(self.env['MIGRATION_EXPECTED_COMPONENTS']))
    def test_saved_apply_is_single_initialization_then_ordinary_post_validation(self):
        self.prepare();result,out=self.execute();self.assertEqual(result,0);self.assertIn('"production_state_initialization":true',out)
        self.assertEqual([name for name,_ in self.calls],['init','schema','plan','show','apply','post-plan','post-show'])
        self.assertEqual(self.snapshot_names,['before','after-plan','pre-apply','after-apply','after-post']);self.assertEqual((self.root/'apply-attempt').read_bytes(),b'1\n')
        self.assertEqual(next(args for name,args in self.calls if name=='apply')[-1],str(self.root/'saved.tfplan'))
    def test_existing_zero_resource_State_uses_ordinary_contract_without_initializing(self):
        self.prepare(existing=True);self.plan.pop('resource_changes');self.post.pop('resource_changes')
        with patch.object(migration.initializer,'validate_plan',side_effect=AssertionError('must use ordinary validator')):
            result,out=self.execute()
        self.assertEqual(result,0);self.assertNotIn('production_state_initialization',out)
    def test_changed_preapply_absence_stops_before_saved_apply(self):
        self.prepare()
        migration.snapshot.side_effect=lambda _env,_root,name:deepcopy(self.after if name=='pre-apply' else self.before)
        result,out=self.execute();self.assertEqual(result,1);self.assertIn('stage=SEAL',out);self.assertFalse(self.applied);self.assertFalse((self.root/'apply-attempt').exists())
    def test_initial_State_output_or_resource_mutation_is_rejected_before_post_plan(self):
        self.prepare(edit_after=lambda state:state['outputs']['project_id'].update(value='foreign-project'))
        result,out=self.execute();self.assertEqual(result,1);self.assertIn('stage=STATE_AFTER',out);self.assertEqual(sum(name=='apply' for name,_ in self.calls),1);self.assertNotIn('post-plan',[name for name,_ in self.calls])
    def test_original_output_hint_operation_and_native_details_are_all_bound(self):
        self.prepare();value=migration.inputs(self.env);seal,tags=migration.execution_commitment(value,self.payload)
        cases=[('outputs',lambda p:p['plan']['initial_output_changes']['project_id'].update(after=fixtures.CANARY),'plan.output_changes'),
               ('hints',lambda p:p['plan']['initial_relevant_attributes']['entries'][0].update(attribute=['labels',0]),'plan.relevant_attributes'),
               ('configuration',lambda p:p['plan']['validated_plan']['configuration'].update(extra=fixtures.CANARY),'plan.configuration')]
        for name,edit,detail in cases:
            changed=deepcopy(self.payload);edit(changed);next_seal,next_tags=migration.execution_commitment(value,changed);comparison=migration.compare_commitments(json.dumps(tags),next_tags)
            with self.subTest(case=name):self.assertNotEqual(seal,next_seal);self.assertEqual(comparison['changed_components'],['plan']);self.assertIn(detail,comparison['changed_details'])
        for edit in (lambda p:p['plan'].update(operation='unknown'),lambda p:p['plan'].pop('initial_relevant_attributes'),lambda p:p['plan'].update(extra=fixtures.CANARY)):
            changed=deepcopy(self.payload);edit(changed)
            with self.assertRaises(ValueError):migration.execution_commitment(value,changed)
    def test_hint_outer_order_only_is_canonical_but_content_presence_and_duplicates_are_bound(self):
        self.prepare();value=migration.inputs(self.env);seal,tags=migration.execution_commitment(value,self.payload)
        reordered=deepcopy(self.plan);reordered['relevant_attributes'].reverse()
        payload=deepcopy(self.payload);payload['plan']=migration.initializer.plan_commitment_payload(reordered,manifest=fixtures.manifest(),environment='prod',provider_schema=self.schema)
        self.assertEqual(migration.execution_commitment(value,payload),(seal,tags))
        for edit in (lambda p:p.pop('relevant_attributes'),lambda p:p['relevant_attributes'].append(deepcopy(p['relevant_attributes'][0])),lambda p:p['relevant_attributes'][0]['extra']['ordered'].reverse()):
            changed=deepcopy(self.plan);edit(changed);payload=deepcopy(self.payload);payload['plan']=migration.initializer.plan_commitment_payload(changed,manifest=fixtures.manifest(),environment='prod',provider_schema=self.schema)
            self.assertNotEqual(migration.execution_commitment(value,payload)[0],seal)
    def test_output_only_mutation_is_rejected_before_any_apply(self):
        self.prepare();self.plan['output_changes']['project_id']['after']='foreign-project'
        result,out=self.execute();self.assertEqual(result,1);self.assertIn('stage=VALIDATE',out);self.assertFalse(self.applied)
    def test_initializer_different_components_stop_before_apply(self):
        self.prepare();tags=json.loads(self.env['MIGRATION_EXPECTED_COMPONENTS']);tags['plan.initial_relevant_attributes']='0'*64;self.env['MIGRATION_EXPECTED_COMPONENTS']=json.dumps(tags)
        result,out=self.execute();self.assertEqual(result,1);self.assertIn('plan.initial_relevant_attributes',out);self.assertFalse(self.applied)
    def test_source_closure_requires_initializer_and_its_executed_contract_files(self):
        self.assertTrue({'.github/scripts/terraform_migration.py','.github/scripts/production_initializer.py','.github/scripts/terraform_migration_contract.py','infra/gcp/scripts/production_execution_contract.py'}<=migration.CONTROLLER_SOURCES)

class ControllerSourceClosureTest(ordinary.OfflineTest):
    def test_all_controller_dependency_bytes_are_copied_and_missing_tracked_dependency_rejects(self):
        repository=self.directory/'repository';repository.mkdir();names=sorted(migration.CONTROLLER_SOURCES)+['infra/gcp/environments/prod/main.tf']
        content={name:('exact-reviewed-'+name).encode() for name in names}
        for name,raw in content.items():
            path=repository/name;path.parent.mkdir(parents=True,exist_ok=True);path.write_bytes(raw)
        def command(args,**kwargs):
            raw=('\n'.join(names)+'\n').encode() if args[1]=='ls-files' else content[args[2].removeprefix('HEAD:')]
            return subprocess.CompletedProcess(args,0,raw,b'')
        with patch.object(migration,'ROOT',repository),patch.object(migration,'run',side_effect=command):
            _,closure=migration.copy_source({'MIGRATION_TARGET':'prod'},self.directory/'copied')
            self.assertEqual(set(closure),set(names))
            for name in migration.CONTROLLER_SOURCES:self.assertEqual((self.directory/'copied/checkout'/name).read_bytes(),content[name])
            names.remove('.github/scripts/production_initializer.py')
            with self.assertRaises(ValueError):migration.copy_source({'MIGRATION_TARGET':'prod'},self.directory/'missing')

class SnapshotErrorRenderingTest(ordinary.OfflineTest):
    def test_any_cli_error_rendering_stops_without_proving_absence(self):
        for operation,code in [('HeadObject','404'),('HeadObject','403'),('HeadObject','500'),('GetObject','NoSuchKey'),('GetObject','AccessDenied'),('GetObject','InternalError')]:
            for prefix,suffix in [(b'',b''),(b'\naws: [ERROR]: ',b' (reached max retries: 0)')]:
                stderr=prefix+b'An error occurred ('+code.encode()+b') when calling the '+operation.encode()+b' operation'+suffix+b': '+ordinary.CANARY.encode()+b'\n\nAdditional error details:\nfixed fixture footer\n'
                response=subprocess.CompletedProcess([],254,b'',stderr)
                with self.subTest(operation=operation,code=code),patch.object(migration.subprocess,'run',return_value=response),self.assertRaises(ValueError):migration.aws({},'s3api','head-object' if operation=='HeadObject' else 'get-object')
    def test_listed_existing_state_then_Head_or_Get_error_never_becomes_absence(self):
        helper=ordinary.SnapshotTest();helper.setUp()
        try:
            for action in ('head-object','get-object'):
                env,settings,aws=helper.setup_snapshot(target='prod');original=aws.side_effect
                def failed(_env,service,current,*args):
                    if current==action:raise ValueError(ordinary.CANARY)
                    return original(_env,service,current,*args)
                with self.subTest(action=action),patch.object(migration,'aws',side_effect=failed),self.assertRaises(ValueError):migration.snapshot(env,self.directory,'error')
        finally:helper.doCleanups()
if __name__=='__main__':unittest.main()
