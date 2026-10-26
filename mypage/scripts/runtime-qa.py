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
 load('/login/channel-confirm');page.get_by_role('button',name='このチャンネルでログイン').click();page.get_by_text('読書',exact=True).wait_for()
 page.get_by_role('button',name='アカウントを開く').click();page.get_by_role('button',name='ログアウト',exact=True).click()
 page.get_by_role('heading',name='マイページへログイン').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('confirmation -> session complete -> MyPage -> logout -> login')
 for mode in ['maintenance-race-confirm','maintenance-race-session-complete']:
  load('/login/channel-confirm',mode);page.get_by_role('button',name='このチャンネルでログイン').click()
  page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('normal OAuth confirm and session-complete races preserve maintenance and replace-navigate')
 load('/login/channel-confirm','channel-avatar-failure')
 page.locator('.channel-avatar svg').wait_for();assert page.locator('.channel-avatar img').count()==0
 page.get_by_role('button',name='このチャンネルでログイン').click();page.get_by_text('読書',exact=True).wait_for()
 results.append('failed confirmation avatar falls back to a generic icon and confirmation still works')
 load('/mypage');page.get_by_role('heading',name='マイページへログイン').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('anonymous private-route guard')
 load('/login','authenticated');page.get_by_text('読書',exact=True).wait_for();assert page.get_by_role('heading',name='マイページへログイン').count()==0
 results.append('authenticated login redirect')
 load('/login?logout=true','authenticated');page.get_by_role('heading',name='マイページへログイン').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('explicit fresh-login route')
 load('/login?supportChallenge='+'a'*64,'authenticated');page.get_by_role('heading',name='問い合わせの本人確認').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('support route never reuses a Firebase session as proof')
 for purpose,label in [('delete','保存データの削除依頼'),('revoke','すべてのログインの解除依頼'),('disclosure','保存データの開示依頼')]:
  load('/login/channel-confirm','support-confirm-'+purpose)
  page.get_by_text('依頼の目的：'+label,exact=True).wait_for()
  if purpose=='delete':page.screenshot(path=str(out/'support-confirm-390.png'),full_page=True)
  page.get_by_role('button',name='この依頼の本人確認を完了').click()
  page.get_by_role('heading',name='依頼の本人確認が完了しました').wait_for();assert page.locator('.proof-reference').inner_text()=='b'*64
  assert page.get_by_text('読書',exact=True).count()==0
  assert page.evaluate('document.documentElement.scrollWidth<=innerWidth+1')
  if purpose=='delete':page.screenshot(path=str(out/'support-receipt-390.png'),full_page=True)
  page.evaluate("window.dispatchEvent(new Event('pagehide'))");page.locator('.proof-reference').wait_for(state='detached')
 results.append('all support purposes -> receipt without signIn/session-complete; pagehide clears proof')
 for change in ['uid-change','logout-relogin','pagehide-pageshow','route-leave']:
  load('/login/channel-confirm','support-confirm-delete')
  page.get_by_role('button',name='この依頼の本人確認を完了').wait_for()
  page.locator('#synthetic-hold').click();page.get_by_role('button',name='この依頼の本人確認を完了').click()
  page.get_by_role('button',name='手続きを完了しています…',exact=True).wait_for()
  if change=='uid-change':page.locator('#synthetic-switch').click()
  if change=='logout-relogin':
   page.locator('#synthetic-switch').click();page.locator('#synthetic-logout').click();page.locator('#synthetic-switch').click()
  if change=='pagehide-pageshow':page.evaluate("window.dispatchEvent(new Event('pagehide'));window.dispatchEvent(new Event('pageshow'))")
  if change=='route-leave':page.get_by_role('link',name='受付窓口へ戻る').click()
  page.locator('#synthetic-release').click();page.wait_for_timeout(150)
  assert page.locator('.proof-reference').count()==0
  assert page.get_by_role('heading',name='依頼の本人確認が完了しました').count()==0
  if change!='route-leave':assert page.get_by_role('heading',name='お問い合わせ').count()==0
 results.append('pending support success discarded after uid change/logout-relogin/pagehide-pageshow/route leave; no receipt or late navigation')
 load('/login?supportChallenge=invalid','authenticated');page.get_by_role('heading',name='問い合わせの本人確認').wait_for();assert page.get_by_role('button',name='YouTubeで本人確認を始める').is_disabled()
 results.append('invalid support link never falls back to normal authenticated login')
 load('/login/channel-confirm','support-mismatch');page.get_by_text('依頼の対象チャンネルと一致しません。窓口から案内された本人確認リンクを使って、対象のチャンネルを確認してください。',exact=True).wait_for();page.get_by_role('link',name='受付窓口へ戻る').wait_for()
 results.append('support channel mismatch returns to the support desk')

 load('/login/channel-confirm','missing-channel');page.get_by_role('heading',name='マイページへログイン').wait_for()
 results.append('missing transaction returns to login')
 for mode in ['maintenance-active-delayed-login','maintenance-active-delayed-failure']:
  load('/login/channel-confirm',mode)
  page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
  page.wait_for_timeout(400)
  assert page.get_by_role('heading',name='マイページの利用状況を確認しています').count()==0
  assert page.get_by_text('Sample Channel',exact=True).count()==0
 results.append('ACTIVE channel lookup pending or failed shows actionable maintenance guidance instead of a stuck checking page')
 load('/login/channel-confirm','maintenance-active-delayed-support')
 page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
 page.get_by_role('button',name='この依頼の本人確認を完了').wait_for()
 results.append('ACTIVE pending channel lookup still admits the purpose-bound support exception')
 load('/login/channel-confirm','channel-visibility-refresh')
 page.get_by_text('Sample Channel 1',exact=True).wait_for()
 page.locator('#synthetic-hold-channel').click()
 page.evaluate("Object.defineProperty(document,'visibilityState',{configurable:true,value:'hidden'});document.dispatchEvent(new Event('visibilitychange'));Object.defineProperty(document,'visibilityState',{configurable:true,value:'visible'});document.dispatchEvent(new Event('visibilitychange'))")
 page.get_by_text('Sample Channel 1',exact=True).wait_for(state='detached')
 page.wait_for_timeout(100)
 assert page.get_by_text('Sample Channel 2',exact=True).count()==0
 page.locator('#synthetic-release-channel').click()
 page.get_by_text('Sample Channel 2',exact=True).wait_for()
 paths=json.loads(page.locator('#synthetic-requests').inner_text());assert paths.count('/api/auth/youtube/channel')==2,paths
 results.append('visibility recovery discards login channel metadata, blocks the delayed replacement, and reads a fresh confirmation')
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

 load('/mypage','maintenance-delayed-normal-authenticated')
 page.get_by_role('heading',name='マイページの利用状況を確認しています').wait_for()
 assert page.get_by_text('読書',exact=True).count()==0
 page.get_by_text('読書',exact=True).wait_for()
 assert page.url.endswith('runtime-visual.html?path=%2Fmypage&mode=maintenance-delayed-normal-authenticated')
 results.append('direct protected route stays put, hides private data while checking, then resumes on NORMAL')
 load('/mypage','maintenance-api-recovery-authenticated')
 page.get_by_role('heading',name='自分のペースで、少しずつ。').wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('API-induced maintenance route immediately rechecks NORMAL and replace-navigates home')
 load('/mypage','maintenance-unknown-authenticated')
 page.get_by_role('heading',name='現在、マイページの状態を確認できません').wait_for()
 assert page.get_by_text('ただいまメンテナンス中です',exact=True).count()==0
 assert page.get_by_text('読書',exact=True).count()==0
 assert page.get_by_role('link',name='プライバシー',exact=True).count()==1
 assert page.get_by_role('link',name='利用規約',exact=True).count()==1
 assert page.get_by_role('button',name='Cookie設定',exact=True).count()==1
 assert page.get_by_role('link',name='お問い合わせ',exact=True).count()==1
 results.append('UNKNOWN is fail-safe without falsely claiming planned maintenance')
 for width in [320,390,768,1024,1440]:
  page.set_viewport_size({'width':width,'height':960})
  load('/mypage','maintenance-active-authenticated')
  heading=page.get_by_role('heading',name='ただいまメンテナンス中です');heading.wait_for()
  assert page.evaluate('document.activeElement===document.querySelector("#maintenance-heading")')
  assert page.get_by_text('読書',exact=True).count()==0
  page.get_by_text('終了見込み：2026/10/27 12:00（JST）',exact=True).wait_for()
  assert page.get_by_role('link',name='プライバシー',exact=True).count()==1
  assert page.get_by_role('link',name='利用規約').count()==1
  assert page.get_by_role('link',name='お問い合わせ').count()==1
  assert page.evaluate('document.documentElement.scrollWidth<=innerWidth'),(width,'maintenance','horizontal overflow')
  paths=json.loads(page.locator('#synthetic-requests').inner_text());assert paths and set(paths)=={'/api/public/mypage-status'},paths
  if width==320:
   page.keyboard.press('Tab');assert page.evaluate('document.activeElement?.textContent?.trim()==="再読み込み"')
   for label in ['再読み込み','ログアウト','トップへ戻る']:
    assert page.get_by_role('button' if label!='トップへ戻る' else 'link',name=label,exact=True).bounding_box()['height']>=44
  if width==390:
   before=page.evaluate('JSON.stringify(localStorage)')
   page.get_by_role('button',name='Cookie設定',exact=True).click();page.get_by_role('heading',name='Cookie設定',exact=True).wait_for()
   page.get_by_role('button',name='閉じる',exact=True).click();assert page.evaluate('JSON.stringify(localStorage)')==before
  if width in [320,390,1024,1440]:page.screenshot(path=str(out/f'maintenance-{width}.png'),full_page=True)
 results.append('ACTIVE maintenance UI, focus, legal footer, private-data clearing, and responsive widths')
 load('/maintenance','maintenance-active-delayed-auth');page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
 page.get_by_role('button',name='ログアウト',exact=True).wait_for();paths=json.loads(page.locator('#synthetic-requests').inner_text());assert paths and set(paths)=={'/api/public/mypage-status'},paths
 page.get_by_role('button',name='ログアウト',exact=True).click();page.get_by_role('button',name='ログアウト',exact=True).wait_for(state='detached')
 results.append('direct ACTIVE route reacts to delayed persisted auth and offers logout without private APIs')
 load('/maintenance','maintenance-active-authenticated');page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
 page.get_by_role('button',name='ログアウト',exact=True).click();page.get_by_role('button',name='ログアウト',exact=True).wait_for(state='detached')
 page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
 results.append('authenticated user can log out while ACTIVE')
 with page.expect_navigation(wait_until='load'):
  page.get_by_role('button',name='再読み込み',exact=True).click()
 page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
 results.append('maintenance primary action performs a browser reload of the same route')
 page.evaluate("history.replaceState(null,'','/runtime-visual.html?path=%2Fmaintenance&mode=maintenance-normal-authenticated')")
 with page.expect_navigation(wait_until='load'):
  page.get_by_role('button',name='再読み込み',exact=True).click()
 page.get_by_role('heading',name='自分のペースで、少しずつ。').wait_for()
 results.append('maintenance reload observes NORMAL and replace-navigates home')
 load('/maintenance','maintenance-active-authenticated');page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
 page.evaluate("Object.defineProperty(document,'visibilityState',{configurable:true,value:'hidden'});document.dispatchEvent(new Event('visibilitychange'))")
 page.locator('#synthetic-maintenance-normal').click()
 page.evaluate("Object.defineProperty(document,'visibilityState',{configurable:true,value:'visible'});document.dispatchEvent(new Event('visibilitychange'))")
 page.get_by_role('heading',name='自分のペースで、少しずつ。').wait_for()
 results.append('visible return rechecks ACTIVE, observes NORMAL, and replace-navigates home')
 load('/mypage','maintenance-active-authenticated');page.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
 other=b.new_page(viewport={'width':390,'height':960});other.goto('http://127.0.0.1:18081/runtime-visual.html?'+urlencode({'path':'/mypage','mode':'maintenance-active-authenticated'}));other.get_by_role('heading',name='ただいまメンテナンス中です').wait_for()
 for tab in [page,other]:
  paths=json.loads(tab.locator('#synthetic-requests').inner_text());assert paths and set(paths)=={'/api/public/mypage-status'},paths
 other.close();results.append('two tabs independently gate ACTIVE without personal API requests')
 load('/maintenance');page.get_by_role('heading',name='自分のペースで、少しずつ。').wait_for()
 results.append('direct maintenance route returns home after confirmed NORMAL')
 storage=page.evaluate('({local:Object.keys(localStorage),session:Object.fromEntries(Object.entries(sessionStorage))})');assert storage['local']==[] and all(k=='tsr-scroll-restoration-v1_3' and v=='{}' for k,v in storage['session'].items()),storage
 assert not errors,errors
 page.screenshot(path=str(out/'not-found-390.png'),full_page=True)
 for width in [320,390,768,1024,1440]:
  page.set_viewport_size({'width':width,'height':960})
  for path in ['/mypage/account','/contact','/privacy','/terms']:
   load(path,'authenticated')
   page.get_by_role('heading',level=1).wait_for()
   assert page.locator('.site-header .brand-name').inner_text()=='オンライン作業部屋'
   assert page.locator('.site-header').count()==1
   assert page.get_by_role('heading',level=1).count()==1
   assert page.evaluate('document.documentElement.scrollWidth<=innerWidth'),(width,path,'horizontal overflow')
   if path=='/mypage/account' and width in [390,1440]:
    page.evaluate('document.fonts.ready')
    page.screenshot(path=str(out/f'account-manage-{width}.png'),full_page=True)
 results.append('shared brand header, one page title, and no overflow on four supporting routes at five widths')
 for path in ['/','/login','/privacy','/terms','/contact']:
  page.goto('http://127.0.0.1:18081'+path);page.get_by_role('heading').first.wait_for();assert page.get_by_text('読書',exact=True).count()==0
 results.append('unconfigured product runtime stays public and unavailable')
 b.close()
report={'syntheticOnly':True,'realFirebaseOAuthHosting':False,'externalRequestsBlocked':True,'cases':results,'pageErrors':errors}
(out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
print('PASS:',len(results),'synthetic runtime integration cases; no private storage/page errors. Real OAuth/Hosting E2E not performed.')
