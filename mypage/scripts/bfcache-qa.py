from pathlib import Path
from urllib.parse import urlencode,urlparse
from playwright.sync_api import sync_playwright
import argparse,json
parser=argparse.ArgumentParser(description='Real bfcache return QA against the isolated synthetic static build; no real Firebase/provider')
parser.add_argument('--base-url',default='http://127.0.0.1:18082')
parser.add_argument('--output',default='/tmp/mypage-bfcache-report')
parser.add_argument('--chromium',default='/usr/bin/chromium')
args=parser.parse_args()
assert urlparse(args.base_url).hostname in ['127.0.0.1','localhost']
out=Path(args.output);out.mkdir(parents=True,exist_ok=True)
results=[];external=[];errors=[]
with sync_playwright() as p:
 browser=p.chromium.launch(executable_path=args.chromium,headless=True,args=['--no-sandbox'],ignore_default_args=['--disable-back-forward-cache'])
 page=browser.new_page(viewport={'width':390,'height':960});page.set_default_timeout(10000)
 page.on('pageerror',lambda e:errors.append(str(e)))
 page.on('request',lambda r:external.append(r.url) if not r.url.startswith(args.base_url+'/') else None)
 page.add_init_script("window.__bfRestored=false;window.addEventListener('pageshow',e=>{window.__bfRestored=e.persisted})")
 for support in [False,True]:
  for pending in [False,True]:
   page.goto(args.base_url+'/runtime-visual.html?'+urlencode({'path':'/login/channel-confirm','mode':'support-confirm-delete' if support else ''}))
   label='この依頼の本人確認を完了' if support else 'このチャンネルでログイン'
   page.get_by_role('button',name=label).wait_for()
   if pending:
    page.locator('#synthetic-hold').click();page.get_by_role('button',name=label).click()
    page.get_by_role('button',name='手続きを完了しています…').wait_for()
   # A real cross-document navigation, not a dispatched pageshow/pagehide.
   # The localhost static server's directory page is a harmless away document.
   page.goto(args.base_url+'/');page.go_back(wait_until='commit')
   page.get_by_role('alert').get_by_text('確認が中断されました。下のリンクから手続きをやり直してください。',exact=True).wait_for()
   assert page.evaluate('window.__bfRestored===true'), 'Browser returned by reload; actual bfcache remains unverified'
   assert page.get_by_text('チャンネルを確認しています…',exact=True).count()==0
   if pending:
    page.locator('#synthetic-release').click();page.wait_for_timeout(100)
   assert page.locator('.proof-reference').count()==0
   assert page.get_by_text('読書',exact=True).count()==0
   assert page.get_by_role('heading',name='依頼の本人確認が完了しました').count()==0
   restart='受付窓口へ戻る' if support else 'ログインからやり直す'
   page.get_by_role('link',name=restart).click()
   page.get_by_role('heading',name='お問い合わせ' if support else 'マイページへログイン',exact=True).wait_for()
   results.append({'purpose':'support' if support else 'login','pending':pending,'persisted':True,'restartWorks':True,'staleSuccessDisplayed':False})
 assert not external,'Unexpected external request (URLs not printed)'
 assert not errors,errors
 browser.close()
(out/'report.json').write_text(json.dumps({'syntheticOnly':True,'actualBfcache':True,'cases':results,'externalRequests':len(external),'pageErrors':errors},indent=2))
print('PASS: 4 real bfcache confirmation-return cases, explicit restart works, no late receipt/navigation.')
