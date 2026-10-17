"""Actual AWS CLI projection parser over dummy pages; no AWS/authentication calls."""
import importlib.util,json,pathlib,sys,unittest
sys.dont_write_bytecode=True
import jmespath
ROOT=pathlib.Path(__file__).resolve().parents[3]
SCRIPT=ROOT/'infra/gcp/scripts/collect_aws_runtime_versioned_config.py'
spec=importlib.util.spec_from_file_location('query_target',SCRIPT)
collector=importlib.util.module_from_spec(spec);spec.loader.exec_module(collector)

class QueryPrivacy(unittest.TestCase):
    def test_raw_page_filters_fixed_names_and_all_unrelated_values_or_marker_tokens(self):
        functions=[]
        for name in reversed(collector.CALLERS):
            functions.append({'FunctionName':name,
                'FunctionArn':'arn:aws:lambda:ap-northeast-1:222222222222:function:'+name,
                'Role':'arn:aws:iam::222222222222:role/Dummy-'+name,'Version':'$LATEST',
                'RevisionId':'dummy-revision-'+name,'LastModified':'2026-01-02T00:00:00.000+0000',
                'Environment':{'Variables':{'GOOGLE_CLOUD_PROJECT':'dummy-project',
                    'GCP_WIF_AUDIENCE':'dummy-audience','GCP_WIF_SERVICE_ACCOUNT_EMAIL':'dummy-sa',
                    'UNRELATED_SECRET':'PRIVATE_ENV_SENTINEL'}},
                'UnrelatedProperty':'PRIVATE_PROPERTY_SENTINEL'})
        functions.append({'FunctionName':'unrelated-function','Environment':{'Variables':{
            'UNRELATED_SECRET':'PRIVATE_FUNCTION_SENTINEL'}}})
        page={'Functions':functions,'NextMarker':'PRIVATE_MARKER_SENTINEL',
              'UnrelatedPageValue':'PRIVATE_PAGE_SENTINEL'}
        projected=jmespath.search(collector.FUNCTION_PAGE_QUERY,page)
        self.assertEqual(set(projected),{'selected','has_more'})
        self.assertTrue(projected['has_more'])
        self.assertEqual({x['name'] for x in projected['selected']},set(collector.CALLERS))
        self.assertEqual(len(projected['selected']),6)
        for record in projected['selected']:
            self.assertEqual(set(record),{'name','arn','role','version','revision_id','last_modified','auth'})
            self.assertEqual(set(record['auth']),set(collector.AUTH_KEYS))
        encoded=json.dumps(projected)
        for sentinel in ('PRIVATE_ENV_SENTINEL','PRIVATE_PROPERTY_SENTINEL','PRIVATE_FUNCTION_SENTINEL',
                         'PRIVATE_MARKER_SENTINEL','PRIVATE_PAGE_SENTINEL'):
            self.assertNotIn(sentinel,encoded)
        page.pop('NextMarker')
        self.assertFalse(jmespath.search(collector.FUNCTION_PAGE_QUERY,page)['has_more'])
        page['NextMarker']=''
        self.assertFalse(jmespath.search(collector.FUNCTION_PAGE_QUERY,page)['has_more'])

    def test_projection_does_not_invent_a_missing_function_or_fetch_another_page(self):
        projected=jmespath.search(collector.FUNCTION_PAGE_QUERY,{
            'Functions':[{'FunctionName':name} for name in collector.CALLERS[:-1]],
            'NextMarker':'PRIVATE_MARKER_SENTINEL'})
        self.assertEqual(len(projected['selected']),5)
        self.assertTrue(projected['has_more'])
        self.assertNotIn('PRIVATE_MARKER_SENTINEL',json.dumps(projected))

if __name__=='__main__':unittest.main()
