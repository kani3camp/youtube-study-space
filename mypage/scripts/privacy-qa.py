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
 page.get_by_role('heading',name='利用状況の計測について').wait_for()
 assert page.evaluate("localStorage.getItem('oss.analytics-consent.v1')===null")
 assert not external
 page.screenshot(path=str(out/'consent-unset-390.png'),full_page=True)
 page.get_by_role('button',name='許可しない',exact=True).click()
 assert page.evaluate("localStorage.getItem('oss.analytics-consent.v1')==='denied'")
 assert page.get_by_role('heading',name='利用状況の計測について').count()==0
 page.get_by_role('button',name='Cookie設定',exact=True).click();dialog=page.get_by_role('dialog',name='Cookie設定');dialog.wait_for()
 switch=dialog.get_by_role('switch',name='利用状況の計測を許可');assert not switch.is_checked()
 switch.click();assert switch.is_checked();assert page.evaluate("localStorage.getItem('oss.analytics-consent.v1')==='granted'")
 page.keyboard.press('Tab');assert dialog.get_by_role('button',name='閉じる').evaluate('(el)=>el===document.activeElement')
 page.keyboard.press('Tab');assert switch.evaluate('(el)=>el===document.activeElement')
 page.keyboard.press('Shift+Tab');assert dialog.get_by_role('button',name='閉じる').evaluate('(el)=>el===document.activeElement')
 switch.click();assert not switch.is_checked();page.screenshot(path=str(out/'cookie-settings-390.png'),full_page=True)
 page.keyboard.press('Escape');dialog.wait_for(state='detached');assert page.get_by_role('button',name='Cookie設定',exact=True).evaluate('(el)=>el===document.activeElement')
 page.reload();page.get_by_role('heading',name='問い合わせの本人確認').wait_for();assert page.get_by_role('heading',name='利用状況の計測について').count()==0
 results.append('unset nonblocking banner, explicit deny/grant/withdraw, persistence and native modal Tab/Escape/focus')
 page.get_by_role('button',name='Cookie設定',exact=True).click();page.get_by_role('dialog').wait_for()
 other=context.new_page();other.goto('http://127.0.0.1:18081/');other.get_by_role('button',name='Cookie設定',exact=True).click();other.get_by_role('dialog').get_by_role('switch').click()
 page.wait_for_function("document.querySelector('input[role=switch]')?.checked===true")
 other.get_by_role('dialog').get_by_role('switch').click();page.wait_for_function("document.querySelector('input[role=switch]')?.checked===false")
 results.append('actual same-origin cross-tab withdrawal updates consent')
 page.keyboard.press('Escape')
 for width in [320,390,1440]:
  page.set_viewport_size({'width':width,'height':960})
  for route,title in [('/privacy','プライバシーポリシー'),('/terms','利用規約'),('/contact','お問い合わせ')]:
   page.goto('http://127.0.0.1:18081'+route);page.get_by_role('heading',name=title,exact=True).wait_for()
   assert page.evaluate('document.documentElement.scrollWidth<=innerWidth+1')
   if route!='/contact':page.get_by_text('公開前の確認用文面です。運営主体・問い合わせ先・施行日・公開条件は、確定後に掲載します。',exact=True).wait_for()
   page.screenshot(path=str(out/(route[1:]+'-'+str(width)+'.png')),full_page=True)
 results.append('anonymous draft policy/terms/contact at 320/390/1440 without guessed operator data')
 assert page.evaluate("Object.keys(localStorage).length===1 && localStorage.getItem('oss.analytics-consent.v1')==='denied'")
 assert not external, 'unexpected external request (not printed)'
 assert not errors,errors
 b.close()
(out/'report.json').write_text(json.dumps({'cases':results,'pageErrors':errors,'externalRequests':len(external),'ga4TransportConnected':False,'realMandatoryFirebaseNetworkVerified':False},ensure_ascii=False,indent=2))
print('PASS: consent/cross-tab/keyboard/public draft-policy QA; no external request or private storage. Real GA4/mandatory Firebase transport not tested.')
