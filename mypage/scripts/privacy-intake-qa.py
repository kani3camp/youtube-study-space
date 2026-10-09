"""Synthetic intake browser QA. Requires runtime-visual Vite on 127.0.0.1:18081."""
from pathlib import Path
from playwright.sync_api import sync_playwright
import argparse, json
parser = argparse.ArgumentParser()
parser.add_argument('--output', default='/tmp/mypage-privacy-intake-qa')
args = parser.parse_args()
out = Path(args.output); out.mkdir(parents=True, exist_ok=True)
errors = []; external = []
base = 'http://127.0.0.1:18081/runtime-visual.html?mode=privacy-intake-authenticated&path=/contact'
with sync_playwright() as p:
 browser = p.chromium.launch(executable_path='/usr/bin/chromium', headless=True, args=['--no-sandbox'])
 context = browser.new_context(viewport={'width': 390, 'height': 900})
 context.route('**/*', lambda route: route.continue_() if route.request.url.startswith('http://127.0.0.1:18081/') else (external.append(route.request.url), route.abort())[-1])
 page = context.new_page(); page.on('pageerror', lambda error: errors.append(str(error)))
 page.goto(base)
 page.get_by_role('heading', name='アプリ内で依頼する').wait_for()
 page.get_by_role('button', name='許可しない', exact=True).click()
 page.locator('#synthetic-restrict-moderation').click()
 assert page.get_by_role('heading', name='アプリ内で依頼する').count() == 1
 page.screenshot(path=str(out / 'intake-form-390.png'), full_page=True)
 page.get_by_label('補足').fill('Synthetic privacy request')
 page.locator('#synthetic-hold').click()
 page.get_by_role('button', name='依頼を受け付ける').click()
 assert page.get_by_role('button', name='送信中…').is_disabled()
 page.get_by_role('link', name='プライバシー', exact=True).last.click()
 page.locator('#synthetic-release').click()
 page.get_by_role('link', name='お問い合わせ', exact=True).last.click()
 assert page.get_by_text('受付しました。参照番号', exact=False).count() == 0
 page.get_by_label('補足').fill('Synthetic privacy request')
 page.get_by_role('button', name='依頼を受け付ける').click()
 page.get_by_text('受付しました。参照番号', exact=False).wait_for()
 assert page.get_by_role('link', name='YouTubeで本人確認を続ける').count() == 1
 page.get_by_role('button', name='状態を確認').click()
 page.get_by_text('Synthetic operator response').wait_for()
 assert page.evaluate('Object.keys(localStorage).every(k => k === "oss.analytics-consent.v1")')
 for width in [320, 390, 1440]:
  page.set_viewport_size({'width': width, 'height': 900})
  assert page.evaluate('document.documentElement.scrollWidth <= innerWidth + 1')
  page.screenshot(path=str(out / f'intake-{width}.png'), full_page=True)
 page.goto('http://127.0.0.1:18081/runtime-visual.html?mode=privacy-intake-anonymous&path=/contact')
 page.get_by_text('この端末に利用可能なログインがありません。', exact=False).wait_for()
 assert page.get_by_role('heading', name='アプリ内で依頼する').count() == 0
 assert page.get_by_role('link', name='個人情報・プライバシーに関する問い合わせ・請求').count() == 1
 assert not external and not errors, (len(external), errors)
 browser.close()
(out / 'report.json').write_text(json.dumps({'cases': ['restricted existing session keeps intake; pending submit disabled; Back/Close drops late response; receipt before proof; manual verified reply; anonymous human contact; 320/390/1440 widths'], 'externalRequests': len(external), 'pageErrors': errors, 'realOperatorConnected': False}, indent=2))
print('PASS: synthetic intake and reply browser QA')
