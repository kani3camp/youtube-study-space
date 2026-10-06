from pathlib import Path
from playwright.sync_api import sync_playwright
from urllib.parse import urlencode
import argparse,json
parser=argparse.ArgumentParser(description='Synthetic localhost-only runtime integration QA')
parser.add_argument('--output',default='/tmp/mypage-runtime-qa');parser.add_argument('--chromium',default='/usr/bin/chromium');args=parser.parse_args()
out=Path(args.output);out.mkdir(parents=True,exist_ok=True)
results=[]
with sync_playwright() as p:
 b=p.chromium.launch(executable_path=args.chromium,headless=True,args=['--no-sandbox'])
 page=b.new_page(viewport={'width':390,'height':960});errors=[]
 page.on('pageerror',lambda e:errors.append(str(e)))
 page.route('**/*',lambda r:r.continue_() if r.request.url.startswith('http://127.0.0.1:18081/') else r.abort())
 def load(path,mode=''):
  page.goto('http://127.0.0.1:18081/runtime-visual.html?'+urlencode({'path':path,'mode':mode}))
 load('/login/channel-confirm');page.get_by_role('button',name='このチャンネルで続ける').click();page.get_by_text('読書',exact=True).wait_for()
 page.get_by_role('button',name='アカウントを開く').click();page.get_by_role('button',name='ログアウト',exact=True).click()
 page.get_by_role('heading',name='マイページへログイン').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('confirmation -> session complete -> MyPage -> logout -> login')
 load('/mypage');page.get_by_role('heading',name='マイページへログイン').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('anonymous private-route guard')
 load('/login','authenticated');page.get_by_text('読書',exact=True).wait_for();assert page.get_by_role('heading',name='マイページへログイン').count()==0
 results.append('authenticated login redirect')
 load('/login?logout=true','authenticated');page.get_by_role('heading',name='マイページへログイン').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('explicit fresh-login route')
 load('/login?supportChallenge='+'a'*64,'authenticated');page.get_by_role('heading',name='問い合わせの本人確認').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('support route never reuses a Firebase session as proof')
 load('/login/channel-confirm','missing-channel');page.get_by_role('heading',name='マイページへログイン').wait_for()
 results.append('missing transaction returns to login')
 load('/mypage','late-response');page.get_by_text('読書',exact=True).wait_for();page.locator('#synthetic-hold').click();page.get_by_role('button',name='最新の情報に更新').click()
 page.wait_for_function("document.querySelector('.refresh-button')?.disabled===true");page.locator('#synthetic-switch').click();page.get_by_text('新しい合成作業',exact=True).wait_for()
 page.locator('#synthetic-release').click();page.wait_for_function("document.querySelector('.task-name')?.textContent==='新しい合成作業'")
 assert page.get_by_text('読書',exact=True).count()==0
 results.append('uid switch and delayed aborted-response discard')
 page.evaluate("window.dispatchEvent(new Event('pagehide'))");assert page.get_by_text('新しい合成作業',exact=True).count()==0
 page.evaluate("window.dispatchEvent(new Event('pageshow'))");page.get_by_text('新しい合成作業',exact=True).wait_for()
 results.append('pagehide clear and pageshow auth recheck')
 page.locator('#synthetic-fail-logout').click();page.get_by_role('button',name='アカウントを開く').click();page.get_by_role('button',name='ログアウト',exact=True).click()
 page.get_by_role('button',name='ログアウトを再試行').click();page.get_by_role('button',name='YouTubeでログイン').wait_for()
 assert page.get_by_text('新しい合成作業',exact=True).count()==0
 results.append('signout failure -> retry -> recovery')
 load('/mypage/account','authenticated');page.get_by_role('heading',name='アカウント管理').wait_for()
 assert page.locator('details[open]').count()==0;page.get_by_text('保存データについて',exact=True).click();assert page.locator('details[open]').count()==1
 results.append('account accordions initially closed')
 load('/unknown','authenticated');page.get_by_role('heading',name='ページが見つかりません').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('unknown route without private fallback')
 storage=page.evaluate('({local:Object.keys(localStorage),session:Object.fromEntries(Object.entries(sessionStorage))})');assert storage['local']==[] and all(k=='tsr-scroll-restoration-v1_3' and v=='{}' for k,v in storage['session'].items()),storage
 assert not errors,errors
 page.screenshot(path=str(out/'not-found-390.png'),full_page=True)
 for path in ['/','/login','/privacy','/terms','/contact']:
  page.goto('http://127.0.0.1:18081'+path);page.get_by_role('heading').first.wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('unconfigured product runtime stays public and unavailable')
 b.close()
report={'syntheticOnly':True,'realFirebaseOAuthHosting':False,'externalRequestsBlocked':True,'cases':results,'pageErrors':errors}
(out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
print('PASS:',len(results),'synthetic runtime integration cases; no private storage/page errors. Real OAuth/Hosting E2E not performed.')
