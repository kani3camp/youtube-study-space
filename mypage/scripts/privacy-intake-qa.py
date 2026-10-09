"""Synthetic intake browser QA. Requires runtime-visual Vite on 127.0.0.1:18081."""
from pathlib import Path
from playwright.sync_api import sync_playwright
import argparse, json
from urllib.parse import urlencode

parser = argparse.ArgumentParser()
parser.add_argument('--output', default='/tmp/mypage-privacy-intake-qa')
args = parser.parse_args()
out = Path(args.output); out.mkdir(parents=True, exist_ok=True)
errors = []; external = []; oauth_wakeups = []
origin = 'http://127.0.0.1:18081/'
def synthetic_url(path):
 return origin + 'runtime-visual.html?' + urlencode({'mode': 'privacy-intake-authenticated', 'path': path})
with sync_playwright() as p:
 browser = p.chromium.launch(executable_path='/usr/bin/chromium', headless=True, args=['--no-sandbox'])
 context = browser.new_context(viewport={'width': 390, 'height': 900})
 def route(request_route):
  url = request_route.request.url
  if url.startswith(origin): request_route.continue_()
  elif url.startswith('https://accounts.google.com/o/oauth2/v2/auth?'):
   oauth_wakeups.append(url)
   request_route.fulfill(status=302, headers={'Location': synthetic_url('/login/channel-confirm')}, body='')
  else:
   external.append(url)
   request_route.abort()
 context.route('**/*', route)
 page = context.new_page(); page.on('pageerror', lambda error: errors.append(str(error)))
 page.goto(synthetic_url('/contact'))
 page.get_by_role('heading', name='アプリ内で依頼する').wait_for()
 page.get_by_role('button', name='許可しない', exact=True).click()
 page.locator('#synthetic-restrict-moderation').click()
 assert page.get_by_role('heading', name='アプリ内で依頼する').count() == 1
 page.screenshot(path=str(out / 'intake-form-390.png'), full_page=True)
 page.locator('#privacy-body').fill('Synthetic privacy request A')
 page.locator('#synthetic-hold').click()
 page.get_by_role('button', name='依頼を受け付ける').click()
 assert page.get_by_role('button', name='送信中…').is_disabled()
 assert page.locator('#privacy-purpose').is_disabled() and page.locator('#privacy-body').is_disabled() and page.locator('#privacy-ref').is_disabled()
 page.get_by_role('link', name='プライバシー', exact=True).last.click()
 page.locator('#synthetic-release').click()
 page.get_by_role('link', name='お問い合わせ', exact=True).last.click()
 assert page.get_by_text('受付しました。参照番号', exact=False).count() == 0
 assert page.locator('#privacy-body').input_value() == 'Synthetic privacy request A'
 assert page.locator('#privacy-purpose').input_value() == 'delete'
 page.get_by_role('button', name='依頼を受け付ける').click()
 page.get_by_text('受付しました。参照番号', exact=False).wait_for()
 assert page.locator('#privacy-ref').input_value() == 'c' * 64
 page.get_by_role('button', name='状態を確認').click()
 page.get_by_text('本人確認済みの依頼を確認できません。', exact=False).wait_for()
 page.get_by_role('link', name='YouTubeで本人確認を続ける').click()
 page.get_by_role('heading', name='問い合わせの本人確認').wait_for()
 page.get_by_role('link', name='お問い合わせ', exact=True).last.click()
 page.get_by_text('受付しました。参照番号', exact=False).wait_for()
 assert page.locator('#privacy-ref').input_value() == 'c' * 64
 page.get_by_role('link', name='YouTubeで本人確認を続ける').click()
 page.locator('input[type=checkbox]').nth(0).check()
 page.locator('input[type=checkbox]').nth(1).check()
 page.get_by_role('button', name='YouTubeで本人確認を始める').click()
 page.get_by_text('依頼の目的：保存データの削除依頼').wait_for()
 assert len(oauth_wakeups) == 1
 page.get_by_role('button', name='この依頼の本人確認を完了').click()
 page.get_by_role('heading', name='依頼の本人確認が完了しました').wait_for()
 assert page.locator('.proof-reference').first.inner_text() == 'b' * 64
 assert page.locator('#privacy-ref').input_value() == 'c' * 64
 page.get_by_role('button', name='状態を確認').click()
 page.get_by_text('Synthetic operator response').wait_for()
 assert page.evaluate('Object.keys(localStorage).every(k => k === "oss.analytics-consent.v1")')
 for width in [320, 390, 1440]:
  page.set_viewport_size({'width': width, 'height': 900})
  assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1')
  page.screenshot(path=str(out / f'intake-{width}.png'), full_page=True)
 page.locator('#synthetic-hold').click()
 page.get_by_role('button', name='状態を確認').click()
 assert page.locator('#privacy-ref').is_disabled()
 page.locator('#synthetic-switch').click()
 page.locator('#synthetic-release').click()
 page.wait_for_timeout(50)
 assert page.get_by_text('Synthetic operator response').count() == 0
 assert page.locator('#privacy-ref').input_value() == ''
 assert page.locator('.proof-reference').count() == 0
 page.goto(origin + 'runtime-visual.html?mode=privacy-intake-anonymous&path=/contact')
 page.get_by_text('この端末に利用可能なログインがありません。', exact=False).wait_for()
 assert page.get_by_role('heading', name='アプリ内で依頼する').count() == 0
 assert page.get_by_role('link', name='個人情報・プライバシーに関する問い合わせ・請求').count() == 1
 page.goto(synthetic_url('/contact'))
 page.locator('#privacy-body').fill('Private request for synthetic A')
 page.locator('#privacy-ref').fill('a' * 64)
 page.locator('#synthetic-switch').click()
 assert page.locator('#privacy-body').input_value() == ''
 assert page.locator('#privacy-ref').input_value() == ''
 page.goto(synthetic_url('/contact'))
 page.locator('#privacy-body').fill('Pending request for synthetic A')
 page.locator('#synthetic-hold').click()
 page.get_by_role('button', name='依頼を受け付ける').click()
 assert page.locator('#privacy-body').is_disabled()
 page.locator('#synthetic-switch').click()
 page.locator('#synthetic-release').click()
 page.wait_for_timeout(50)
 assert page.locator('#privacy-body').input_value() == ''
 assert page.locator('#privacy-ref').input_value() == ''
 assert page.get_by_text('受付しました。参照番号', exact=False).count() == 0
 page.locator('#privacy-body').fill('Private request for synthetic B')
 page.locator('#privacy-ref').fill('b' * 64)
 page.evaluate("window.dispatchEvent(new PageTransitionEvent('pagehide'))")
 assert page.locator('#privacy-body').input_value() == ''
 assert page.locator('#privacy-ref').input_value() == ''
 page.locator('#privacy-body').fill('Private request before signout')
 page.locator('#privacy-ref').fill('c' * 64)
 page.locator('#synthetic-logout').click()
 page.locator('#synthetic-switch').click()
 assert page.locator('#privacy-body').input_value() == ''
 assert page.locator('#privacy-ref').input_value() == ''
 assert not external and not errors, (len(external), errors)
 browser.close()
(out / 'report.json').write_text(json.dumps({'cases': ['restricted existing session keeps intake; pending purpose/body/ref disabled; Back and retry preserve draft; synthetic OAuth redirect reload; proof ref distinct from status ref; verified reply; pending status and intake invalidated on UID change; same-page UID change, pagehide and signout clear private draft/ref; anonymous human contact; 320/390/1440 widths'], 'syntheticOAuthRedirects': len(oauth_wakeups), 'externalRequests': len(external), 'pageErrors': errors, 'realOperatorConnected': False}, indent=2))
print('PASS: synthetic intake, OAuth redirect and reply browser QA')
