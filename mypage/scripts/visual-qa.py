from pathlib import Path
from playwright.sync_api import sync_playwright
import json
import argparse
parser=argparse.ArgumentParser(description='Synthetic localhost-only MyPage visual and keyboard QA')
parser.add_argument('--output',default='/tmp/mypage-visual-qa')
parser.add_argument('--chromium',default='/usr/bin/chromium')
args=parser.parse_args()
out=Path(args.output);out.mkdir(parents=True,exist_ok=True)
states=['work','break','not-seated','unregistered','long','overdue','zero','loading','failure','refresh-failure','metadata-failure','metadata-expired']
results=[]
with sync_playwright() as p:
 browser=p.chromium.launch(executable_path=args.chromium,headless=True,args=['--no-sandbox'])
 page=browser.new_page()
 page.route('**/*',lambda route: route.continue_() if route.request.url.startswith(('http://127.0.0.1:18081/','data:')) else route.abort())
 errors=[];page.on('pageerror',lambda e:errors.append(str(e)))
 for width in [320,390,768,1024,1440]:
  page.set_viewport_size({'width':width,'height':960})
  for state in states:
   page.goto('http://127.0.0.1:18081/visual.html?state='+state)
   page.get_by_role('heading',name='マイページ',exact=True).wait_for()
   overflow=page.evaluate('document.documentElement.scrollWidth > innerWidth')
   assert not overflow,(width,state,'horizontal overflow')
   assert not errors,(width,state,errors)
   if state in ['work','overdue']:
    track=page.locator('.timeline').bounding_box();label=page.locator('.timeline-caption').bounding_box()
    assert label['x']>=track['x']-1 and label['x']+label['width']<=track['x']+track['width']+1,(width,state,'timestamp outside timeline')
   if state=='zero':
    assert page.locator('.metric-today .duration-number').first.inner_text()=='0'
    assert page.locator('.metric-today .sr-only').inner_text()=='0時間 0分'
   if state=='loading':assert page.locator('.skeleton').count()==3
   if state=='failure':
    assert page.get_by_text('情報を取得できませんでした',exact=True).count()==1
    assert page.locator('.summary-card,.recent-card,.notice').count()==0
    assert page.get_by_role('button',name='再読み込み',exact=True).count()==1
   if state=='break':
    assert page.get_by_text('休憩中',exact=True).count()==1
    assert page.get_by_text('読書',exact=True).count()==1
   if state=='long':
    assert len(page.locator('.task-name').inner_text())>100
    assert page.locator('.task-name').evaluate('(el)=>el.scrollHeight <= el.clientHeight+1')
   if state in ['work','break','long','failure'] and width in [320,390,1440]:
    page.screenshot(path=str(out/f'{state}-{width}.png'),full_page=True)
   results.append({'width':width,'state':state,'horizontalOverflow':False,'pageErrors':0})
 # Expired metadata must clear a previous successful value and explain why.
 page.goto('http://127.0.0.1:18081/visual.html?state=metadata-expired')
 page.get_by_role('button',name='アカウントを開く').click()
 assert page.get_by_text('Sample Channel',exact=True).count()==0
 assert page.get_by_text('@sample',exact=True).count()==0
 assert page.get_by_text('チャンネル情報の有効期限が切れました。再取得まで表示できません。',exact=True).count()==1
 page.keyboard.press('Escape')
 # native modal, focus containment, Escape restoration, keyboard-selected zero.
 for width in [390,1440]:
  page.set_viewport_size({'width':width,'height':960})
  page.goto('http://127.0.0.1:18081/visual.html')
  avatar=page.get_by_role('button',name='アカウントを開く');avatar.click()
  dialog=page.get_by_role('dialog',name='アカウント');dialog.wait_for()
  for _ in range(8):
   page.keyboard.press('Tab')
   assert page.evaluate('document.querySelector("dialog").contains(document.activeElement)')
  page.screenshot(path=str(out/f'account-{width}.png'),full_page=True)
  page.keyboard.press('Escape');assert dialog.count()==0
  assert avatar.evaluate('(el)=>document.activeElement===el')
  zero=page.get_by_role('button',name='2026-09-30: 0時間 0分');zero.focus()
  assert '2026-09-30: 0時間 0分' in page.locator('.selected-day').inner_text()
  assert zero.get_attribute('aria-pressed')=='true'
  avatar.click();page.get_by_role('button',name='ログアウト',exact=True).click()
  assert page.get_by_text('ログアウトしました。',exact=True).count()==1
  assert page.get_by_text('読書',exact=True).count()==0
  assert page.get_by_role('dialog').count()==0
  page.goto('http://127.0.0.1:18081/visual.html?state=avatar-failure')
  avatar=page.get_by_role('button',name='アカウントを開く')
  avatar.locator('svg').wait_for();assert avatar.locator('img').count()==0
  avatar.click();page.get_by_role('button',name='ログアウト',exact=True).wait_for()
 browser.close()
report={'syntheticOnly':True,'cases':results,'dialogKeyboardAndLogout':[390,1440],'avatarFailureFallback':[390,1440],'externalRequestsBlocked':True,'approvedReference':'Supplied Approved HTML/runtime compared locally; exact pixel parity is not a target because Current Canon overrides prototype semantics and font transport is blocked'}
(out/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2))
print('PASS: 60 responsive/state cases, bounded timestamp, skeleton/first failure/zero metrics, avatar failure fallback, native modal focus/Escape/zero-selection/logout at 390 and 1440.')
