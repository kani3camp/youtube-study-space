#!/usr/bin/env python3
"""Real pinned Terraform/default full-prod root against loopback fake S3+STS only.

This is a local test instrument. No actual AWS/GCP credentials,
external endpoints, remote State, cloud lock, secrets or workflow are used.
"""
from __future__ import annotations
import copy,hashlib,importlib.util,json,os,shutil,subprocess,tempfile,threading,time,unittest,urllib.error,urllib.parse,urllib.request
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
SOURCE_REPO=Path(__file__).resolve().parents[2]
MIRROR=Path(os.environ.get('GCP_TEST_PROVIDER_MIRROR',str(SOURCE_REPO/'infra/gcp/environments/prod/.terraform/providers')))
ROOT=Path(__file__).resolve().parent
ACCOUNT='123456789012'
BUCKET='native-initializer-fixture'
KEY='youtube-study-space/prod/terraform.tfstate'
WORKSPACES='youtube-study-space/prod/workspaces/'
OUTPUTS={'environment':'production','project_id':'youtube-study-space'}
MAX=4*1024*1024
class DummyS3:
 def __init__(self):
  self.objects={};self.versions=[];self.requests=[];self.errors=[];self.phase='starting';self.versioning='Enabled'
 def record(self,method,path,headers,body):
  self.requests.append({'phase':self.phase,'method':method,'path':path,'body':body,'if_none_match':headers.get('If-None-Match'),'sse':headers.get('X-Amz-Server-Side-Encryption'),'expected_owner':headers.get('X-Amz-Expected-Bucket-Owner')})
 def counts(self):
  return {'state_puts':sum(r['method']=='PUT' and r['path'].split('?')[0]=='/'+BUCKET+'/'+KEY for r in self.requests),'lock_puts':sum(r['method']=='PUT' and r['path'].split('?')[0]=='/'+BUCKET+'/'+KEY+'.tflock' for r in self.requests),'lock_deletes':sum(r['method']=='DELETE' and r['path'].split('?')[0]=='/'+BUCKET+'/'+KEY+'.tflock' for r in self.requests)}
class Handler(BaseHTTPRequestHandler):
 protocol_version='HTTP/1.1'
 def log_message(self,*args):pass
 def response(self,status,body=b'',headers=None):
  self.send_response(status);self.send_header('Content-Length',str(len(body)))
  self.send_header('x-amz-request-id','native-fixed-request')
  for k,v in (headers or {}).items():self.send_header(k,str(v))
  self.end_headers()
  if self.command!='HEAD' and body:self.wfile.write(body)
 def fail(self,reason):
  self.server.fixture.errors.append(reason);self.response(400,b'<Error><Code>InvalidRequest</Code></Error>')
 def do_CONNECT(self):self.fail('CONNECT')
 def do_POST(self):
  raw=self.rfile.read(int(self.headers.get('Content-Length','0')));self.server.fixture.record('POST',self.path,self.headers,raw)
  if self.path!='/sts/' or urllib.parse.parse_qs(raw.decode()).get('Action')!=['GetCallerIdentity']:return self.fail('unknown STS request')
  self.response(200,('<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/"><GetCallerIdentityResult><Arn>arn:aws:sts::'+ACCOUNT+':assumed-role/native/fixture</Arn><UserId>NATIVE:fixture</UserId><Account>'+ACCOUNT+'</Account></GetCallerIdentityResult><ResponseMetadata><RequestId>native</RequestId></ResponseMetadata></GetCallerIdentityResponse>').encode(),{'Content-Type':'text/xml'})
 def handle_s3(self):
  fixture=self.server.fixture;parsed=urllib.parse.urlsplit(self.path);path=urllib.parse.unquote(parsed.path);query=urllib.parse.parse_qs(parsed.query,keep_blank_values=True)
  length=int(self.headers.get('Content-Length','0'))
  if length>MAX:return self.fail('body too large')
  body=self.rfile.read(length) if length else b'';fixture.record(self.command,self.path,self.headers,body)
  if self.headers.get('Transfer-Encoding'):return self.fail('unexpected transport chunking')
  if path=='/'+BUCKET and self.command=='GET':
   if set(query)=={'versioning'}:
    if self.headers.get('X-Amz-Expected-Bucket-Owner')!=ACCOUNT:return self.response(403,b'<Error><Code>AccessDenied</Code></Error>')
    return self.response(200,('<VersioningConfiguration xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Status>'+fixture.versioning+'</Status></VersioningConfiguration>').encode(),{'Content-Type':'application/xml'})
   if set(query)<={'list-type','max-keys','prefix','encoding-type'} and query.get('list-type')==['2'] and query.get('prefix')==[WORKSPACES]:
    return self.response(200,('<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>'+BUCKET+'</Name><Prefix>'+WORKSPACES+'</Prefix><KeyCount>0</KeyCount><MaxKeys>1000</MaxKeys><IsTruncated>false</IsTruncated></ListBucketResult>').encode(),{'Content-Type':'application/xml'})
   return self.fail('unknown bucket query')
  if path not in ('/'+BUCKET+'/'+KEY,'/'+BUCKET+'/'+KEY+'.tflock'):return self.fail('foreign object')
  if set(query)-{'x-id','versionId'}:return self.fail('foreign object query')
  if 'versionId' in query:return self.fail('unexpected versioned Core request')
  key=path[len('/'+BUCKET+'/'):]
  if self.command in ('GET','HEAD'):
   if key not in fixture.objects:return self.response(404,b'<Error><Code>NoSuchKey</Code></Error>',{'Content-Type':'application/xml'})
   raw=fixture.objects[key];version='native-v'+str(len(fixture.versions)) if key==KEY else 'native-lock'
   return self.response(200,raw,{'Content-Type':'application/json','ETag':'"'+hashlib.md5(raw).hexdigest()+'"','x-amz-version-id':version,'Last-Modified':'Sat, 17 Oct 2026 00:00:00 GMT','x-amz-server-side-encryption':'AES256'})
  if self.command=='PUT':
   if self.headers.get('X-Amz-Server-Side-Encryption')!='AES256':return self.fail('missing SSE')
   try:value=json.loads(body)
   except ValueError:return self.fail('non JSON native put')
   if key==KEY+'.tflock':
    if self.headers.get('If-None-Match')!='*':return self.fail('nonconditional lock')
    if key in fixture.objects:return self.response(412,b'<Error><Code>PreconditionFailed</Code></Error>')
    if type(value) is not dict or not value.get('ID') or value.get('Path')!=BUCKET+'/'+KEY:return self.fail('bad lock record')
   else:
    if KEY+'.tflock' not in fixture.objects:return self.fail('unlocked State write')
    if type(value) is not dict or value.get('resources')!=[] or {k:v.get('value') for k,v in value.get('outputs',{}).items()}!=OUTPUTS:return self.fail('non initializer State')
    fixture.versions.append(body)
   fixture.objects[key]=body
   return self.response(200,b'',{'ETag':'"'+hashlib.md5(body).hexdigest()+'"','x-amz-version-id':'native-v'+str(len(fixture.versions))})
  if self.command=='DELETE':
   if key!=KEY+'.tflock':return self.fail('State deletion')
   if key not in fixture.objects:return self.fail('absent lock deletion')
   del fixture.objects[key];return self.response(204)
  return self.fail('unknown method')
 do_GET=handle_s3
 do_HEAD=handle_s3
 do_PUT=handle_s3
 do_DELETE=handle_s3

class NativeS3Initializer(unittest.TestCase):
 def setUp(self):
  self.tmp=tempfile.TemporaryDirectory(prefix='init-s3-',dir=Path('/tmp').resolve());self.root=Path(self.tmp.name)
  self.fixture=DummyS3();self.server=ThreadingHTTPServer(('127.0.0.1',0),Handler);self.server.fixture=self.fixture
  self.thread=threading.Thread(target=self.server.serve_forever,daemon=True);self.thread.start();self.endpoint='http://127.0.0.1:'+str(self.server.server_port)
  self.binary=shutil.which('terraform');self.assertIsNotNone(self.binary)
  self.env={'PATH':str(Path(self.binary).parent)+os.pathsep+os.defpath,'HOME':str(self.root),'TMPDIR':str(self.root),'TF_INPUT':'false','TF_IN_AUTOMATION':'true','TF_WORKSPACE':'default','CHECKPOINT_DISABLE':'1',
   'TF_CLI_CONFIG_FILE':str(self.root/'terraform.rc'),'AWS_ACCESS_KEY_ID':'NATIVE_DUMMY_KEY','AWS_SECRET_ACCESS_KEY':'NATIVE_DUMMY_SECRET','AWS_SESSION_TOKEN':'NATIVE_DUMMY_TOKEN','AWS_REGION':'ap-northeast-1',
   'AWS_EC2_METADATA_DISABLED':'true','AWS_MAX_ATTEMPTS':'1','AWS_CONFIG_FILE':str(self.root/'NO_CONFIG'),'AWS_SHARED_CREDENTIALS_FILE':str(self.root/'NO_CREDENTIALS'),'AWS_IGNORE_CONFIGURED_ENDPOINT_URLS':'true',
   'GOOGLE_OAUTH_ACCESS_TOKEN':'unused-zero-resource-fixture','HTTP_PROXY':'http://127.0.0.1:1','HTTPS_PROXY':'http://127.0.0.1:1','ALL_PROXY':'http://127.0.0.1:1','http_proxy':'http://127.0.0.1:1','https_proxy':'http://127.0.0.1:1','all_proxy':'http://127.0.0.1:1','NO_PROXY':'127.0.0.1,localhost','no_proxy':'127.0.0.1,localhost'}
  self.assertTrue(MIRROR.is_absolute() and MIRROR.is_dir());(self.root/'terraform.rc').write_text('disable_checkpoint = true\nprovider_installation { filesystem_mirror { path = '+json.dumps(str(MIRROR))+' } }\n')
  names=subprocess.run(['git','ls-files','infra/gcp'],cwd=SOURCE_REPO,capture_output=True,text=True,timeout=20);self.assertEqual(names.returncode,0)
  self.source=subprocess.run(['git','rev-parse','HEAD'],cwd=SOURCE_REPO,capture_output=True,text=True,check=True).stdout.strip();self.source_pins={}
  for name in names.stdout.splitlines():
   relative=Path(name)
   if relative.suffix not in {'.tf','.tftpl'} and relative.name not in {'.terraform.lock.hcl','schema.json'}:continue
   source=SOURCE_REPO/relative;self.assertFalse(source.is_symlink());raw=source.read_bytes();self.source_pins[name]=hashlib.sha256(raw).hexdigest();destination=self.root/'checkout'/relative;destination.parent.mkdir(parents=True,exist_ok=True);destination.write_bytes(raw)
  self.assertEqual(subprocess.run(['git','diff','--quiet','HEAD','--',*self.source_pins],cwd=SOURCE_REPO,timeout=20).returncode,0)
  self.tfroot=self.root/'checkout/infra/gcp/environments/prod'
  self.backend={'bucket':BUCKET,'key':KEY,'region':'ap-northeast-1','encrypt':True,'use_lockfile':True,'workspace_key_prefix':WORKSPACES.rstrip('/'),'allowed_account_ids':[ACCOUNT],'use_path_style':True,'endpoints':{'s3':self.endpoint,'sts':self.endpoint+'/sts'},'max_retries':0}
  self.backend_path=self.root/'backend.hcl';self.save_backend()
  self.assertEqual(json.loads(self.command('version','-json'))['terraform_version'],'1.16.4')
  import sys
  sys.path.insert(0,str(SOURCE_REPO/'.github/scripts'));self.controller=__import__('terraform_migration')
  self.gate=__import__('sys').modules.get('production_initializer')
  if self.gate is None:
   import sys
   sys.path.insert(0,str(SOURCE_REPO/'.github/scripts'));self.gate=__import__('production_initializer')
 def tearDown(self):
  self.server.shutdown();self.server.server_close();self.thread.join(timeout=5);self.tmp.cleanup()
 def save_backend(self):self.backend_path.write_text(''.join(k+' = '+json.dumps(v)+'\n' for k,v in self.backend.items()))
 def command(self,*args,codes=(0,)):
  self.fixture.phase=args[0]
  result=subprocess.run([self.binary,'-chdir='+str(self.tfroot),*args],env=self.env,stdin=subprocess.DEVNULL,capture_output=True,timeout=90)
  if result.returncode not in codes:
   # Dummy-only output kept private for local diagnosis; never emitted by tests.
   (self.root/'last.stderr').write_bytes(result.stderr);(self.root/'last.stdout').write_bytes(result.stdout)
  self.assertIn(result.returncode,codes,'native S3 fixture command failed; raw output suppressed')
  self.assertEqual(self.fixture.errors,[],'fake S3 rejected an unexpected operation')
  return result.stdout
 def external_guard(self,owner=ACCOUNT):
  # Mirrors the outer controller bucket guard. Core S3 has allowed_account_ids,
  # but no expected_bucket_owner backend schema option or versioning check.
  request=urllib.request.Request(self.endpoint+'/'+BUCKET+'?versioning',headers={'x-amz-expected-bucket-owner':owner})
  try:
   with urllib.request.build_opener(urllib.request.ProxyHandler({})).open(request,timeout=3) as response:
    value=response.read(MAX+1);self.assertEqual(response.status,200);self.assertIn(b'<Status>Enabled</Status>',value)
  except urllib.error.HTTPError as error:
   code=error.code;error.close();self.assertEqual(code,200,'expected-owner guard rejected metadata')
 def manifest(self):return {'schema_version':'gcp-migration-manifest-v1','environment':'prod','project_id':'youtube-study-space','resources':[]}
 def init(self):self.command('init','-input=false','-lockfile=readonly','-backend-config='+str(self.backend_path))
 def test_default_full_prod_s3_saved_apply_is_the_only_State_object_write(self):
  self.external_guard();self.assertNotIn(KEY,self.fixture.objects);self.init();self.assertEqual(self.fixture.counts()['state_puts'],0);self.assertNotIn(KEY,self.fixture.objects)
  schema=json.loads(self.command('providers','schema','-json'));plans=[]
  for n in range(2):
   self.command('plan','-input=false','-lock-timeout=0s','-out='+str(self.root/('initial'+str(n)+'.plan')))
   plans.append(json.loads(self.command('show','-json',str(self.root/('initial'+str(n)+'.plan')))))
  self.assertEqual(self.fixture.counts()['state_puts'],0);self.assertNotIn(KEY,self.fixture.objects);self.assertNotIn(KEY+'.tflock',self.fixture.objects)
  self.assertTrue(self.gate.selected('prod',self.manifest(),{'head':None,'raw':None,'state':None}))
  payloads=[self.gate.plan_commitment_payload(v,manifest=self.manifest(),environment='prod',provider_schema=schema) for v in plans]
  self.assertEqual(self.gate.contract.canonical(payloads[0]),self.gate.contract.canonical(payloads[1]));self.assertEqual(len(payloads[0]['initial_relevant_attributes']['entries']),3)
  inputs={'manifest':self.manifest(),'tfvars':{'project_id':'youtube-study-space'},'plan_integrity_key':'b'*64}
  sealed=[self.controller.execution_commitment(inputs,{'source':self.source,'closure':self.source_pins,'inputs':inputs,'state':{'head':None,'raw':None,'state':None},'metadata':{},'plan':payload}) for payload in payloads]
  self.assertEqual(sealed[0],sealed[1]);self.assertEqual(set(sealed[0][1]),self.controller.COMMITMENT_COMPONENTS|self.controller.COMMITMENT_DETAILS|{'key'})
  self.command('apply','-input=false',str(self.root/'initial0.plan'));self.assertEqual(self.fixture.counts()['state_puts'],1);self.assertEqual([r['phase'] for r in self.fixture.requests if r['method']=='PUT' and r['path'].split('?')[0]=='/'+BUCKET+'/'+KEY],['apply']);self.assertEqual(len(self.fixture.versions),1);self.assertNotIn(KEY+'.tflock',self.fixture.objects)
  after=json.loads(self.fixture.objects[KEY]);result=self.gate.validate_state_delta(None,after,manifest=self.manifest(),environment='prod',provider_schema=schema,plan=plans[0]);self.assertEqual(result['managed_after'],0)
  self.command('plan','-input=false','-lock-timeout=0s','-out='+str(self.root/'post.plan'));post=self.gate.normalize_empty_plan(json.loads(self.command('show','-json',str(self.root/'post.plan'))),self.manifest())
  result=self.gate.contract.validate_plan(post,manifest=self.manifest(),environment='prod',provider_schema=schema,state=after,phase='post');self.assertEqual((result['managed'],result['imports'],result['raw_observation_drift']),(0,0,0))
  self.assertEqual(self.fixture.counts()['state_puts'],1);self.assertEqual(self.fixture.objects[KEY],self.fixture.versions[0]);self.assertNotIn(KEY+'.tflock',self.fixture.objects)
  self.assertEqual(self.fixture.counts()['lock_puts'],self.fixture.counts()['lock_deletes']);self.assertGreaterEqual(self.fixture.counts()['lock_puts'],4)
  self.assertTrue(all(r['if_none_match']=='*' for r in self.fixture.requests if r['method']=='PUT' and r['path'].split('?')[0].endswith('.tflock')))
  self.assertEqual(subprocess.run(['git','rev-parse','HEAD'],cwd=SOURCE_REPO,capture_output=True,text=True,check=True).stdout.strip(),self.source)
  self.assertTrue(all(hashlib.sha256((SOURCE_REPO/name).read_bytes()).hexdigest()==digest for name,digest in self.source_pins.items()))
 def test_existing_native_lock_refuses_plan_without_State_write_or_foreign_unlock(self):
  self.init();locked=json.dumps({'ID':'12345678-1234-1234-1234-123456789abc','Operation':'apply','Info':'native fixture','Who':'dummy-local','Version':'1.16.4','Created':'2026-10-17T00:00:00Z','Path':BUCKET+'/'+KEY}).encode();self.fixture.objects[KEY+'.tflock']=locked
  self.command('plan','-input=false','-lock-timeout=0s','-out='+str(self.root/'blocked.plan'),codes=(1,))
  self.assertEqual(self.fixture.counts(),{'state_puts':0,'lock_puts':1,'lock_deletes':0});self.assertNotIn(KEY,self.fixture.objects);self.assertEqual(self.fixture.objects[KEY+'.tflock'],locked)
 def test_wrong_account_stops_before_any_S3_State_or_lock_write(self):
  self.backend['allowed_account_ids']=['000000000000'];self.save_backend();self.command('init','-input=false','-lockfile=readonly','-backend-config='+str(self.backend_path),codes=(1,))
  self.assertEqual(self.fixture.counts(),{'state_puts':0,'lock_puts':0,'lock_deletes':0});self.assertEqual(self.fixture.objects,{})
 def test_versioning_and_expected_owner_outer_guards_fail_without_Core_or_State_write(self):
  with self.assertRaises(AssertionError):self.external_guard('000000000000')
  self.fixture.versioning='Suspended'
  with self.assertRaises(AssertionError):self.external_guard()
  self.assertEqual(self.fixture.counts(),{'state_puts':0,'lock_puts':0,'lock_deletes':0});self.assertEqual(self.fixture.objects,{})
if __name__=='__main__':unittest.main()
