from pathlib import Path
from playwright.sync_api import sync_playwright
import argparse,json
parser=argparse.ArgumentParser(description='Synthetic/unconfigured privacy UI QA; no external connections')
parser.add_argument('--output',default='/tmp/mypage-privacy-qa');parser.add_argument('--chromium',default='/usr/bin/chromium');args=parser.parse_args()
out=Path(args.output);out.mkdir(parents=True,exist_ok=True)
external=[];errors=[];results=[]
with sync_playwright() as p:
 b=p.chromium.launch(executable_path=args.chromium,headless=True,args=['--no-sandbox'])
 context=b.new_context(viewport={'width':390,'height':960})
 context.route('**/*',lambda r:r.continue_() if r.request.url.startswith('http://127.0.0.1:18081/') else (external.append(r.request.url),r.abort())[-1])
 page=context.new_page();page.on('pageerror',lambda e:errors.append(str(e)))
 page.goto('http://127.0.0.1:18081/login?supportChallenge=synthetic-private')
 page.get_by_role('heading',name='問い合わせの本人確認').wait_for()
 assert page.get_by_role('heading',name='サイトをもっと使いやすくするために').count()==0
 assert page.evaluate("localStorage.getItem('oss.analytics-consent.v2')===null")
 assert not external
 page.screenshot(path=str(out/'analytics-disabled-390.png'),full_page=True)
 page.get_by_role('button',name='Cookie設定',exact=True).click();dialog=page.get_by_role('dialog',name='Cookie設定');dialog.wait_for()
 dialog.get_by_text('任意のアクセス解析は現在利用していません。利用開始時にあらためてご案内します。',exact=True).wait_for()
 assert dialog.get_by_role('switch').count()==0
 page.keyboard.press('Tab');assert dialog.get_by_role('button',name='閉じる').evaluate('(el)=>el===document.activeElement')
 page.keyboard.press('Escape');dialog.wait_for(state='detached');assert page.get_by_role('button',name='Cookie設定',exact=True).evaluate('(el)=>el===document.activeElement')
 page.reload();page.get_by_role('heading',name='問い合わせの本人確認').wait_for();assert page.get_by_role('heading',name='サイトをもっと使いやすくするために').count()==0
 results.append('unconfigured GA4 hides consent banner, does not store pre-consent, and keeps accessible Cookie settings')
 for width in [320,390,1440]:
  page.set_viewport_size({'width':width,'height':960})
  for route,title in [('/privacy','プライバシーポリシー'),('/terms','利用規約'),('/contact','お問い合わせ')]:
   page.goto('http://127.0.0.1:18081'+route);page.get_by_role('heading',name=title,exact=True).wait_for()
   assert page.evaluate('document.documentElement.scrollWidth<=innerWidth+1')
   if route!='/contact':page.get_by_text('公開前の確認用文面です。運営主体・問い合わせ先・施行日・公開条件は、確定後に掲載します。',exact=True).wait_for()
   page.screenshot(path=str(out/(route[1:]+'-'+str(width)+'.png')),full_page=True)
 results.append('anonymous draft policy/terms/contact at 320/390/1440 without guessed operator data')
 assert page.evaluate("Object.keys(localStorage).length===0")
 assert not external, 'unexpected external request (not printed)'
 assert not errors,errors
 b.close()
(out/'report.json').write_text(json.dumps({'cases':results,'pageErrors':errors,'externalRequests':len(external),'ga4TransportConnected':False,'realMandatoryFirebaseNetworkVerified':False},ensure_ascii=False,indent=2))
print('PASS: unconfigured GA4/off-state/keyboard/public draft-policy QA; no external request or private storage. Consent opt-in and real GA4 require configured HTTPS acceptance.')
