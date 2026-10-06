"""Verify rendered Latin/Japanese faces and offline font failure in Chromium."""
import argparse
import json
from pathlib import Path

from playwright.sync_api import sync_playwright

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--output', default='/tmp/mypage-font-qa')
parser.add_argument('--chromium', default='/usr/bin/chromium')
args = parser.parse_args()
out = Path(args.output)
out.mkdir(parents=True, exist_ok=True)
base = 'http://127.0.0.1:18081/'
faces = {400: 'Regular', 500: 'Medium', 600: 'Bold', 700: 'Bold', 800: 'ExtraBold'}
results = []

with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=args.chromium, headless=True, args=['--no-sandbox'])
    for width in [320, 390, 1440]:
        for mode in ['loaded', 'font-failure']:
            context = browser.new_context(viewport={'width': width, 'height': 960}, locale='ja-JP', service_workers='block')
            page = context.new_page()
            errors, external, font_requests = [], [], []
            page.on('pageerror', lambda error: errors.append(str(error)))

            def guard(route):
                url = route.request.url
                if not url.startswith((base, 'data:')):
                    external.append(route.request.resource_type)
                    route.abort()
                elif '.woff2' in url:
                    font_requests.append(url.rsplit('/', 1)[-1])
                    if mode == 'font-failure':
                        route.abort()
                    else:
                        route.continue_()
                else:
                    route.continue_()

            page.route('**/*', guard)
            page.goto(base + 'visual.html?state=work')
            page.get_by_role('heading', name='マイページ', exact=True).wait_for()
            page.evaluate('''() => {
                const probes = document.createElement('div');
                probes.id = 'font-qa'; probes.setAttribute('aria-hidden', 'true');
                for (const weight of [400, 500, 600, 700, 800]) {
                    for (const [lang, text] of [['latin', 'MyPage 0123456789'], ['ja', 'オンライン作業部屋 今日 累計 読書']]) {
                        const span = document.createElement('span');
                        span.id = `font-${lang}-${weight}`;
                        span.style.cssText = `font-size:24px;font-weight:${weight};display:block;`;
                        span.textContent = text; probes.append(span);
                    }
                }
                document.body.append(probes);
            }''')
            page.evaluate('document.fonts.ready')
            session = context.new_cdp_session(page)
            session.send('DOM.enable')
            session.send('CSS.enable')
            doc = session.send('DOM.getDocument')['root']['nodeId']
            probes = []
            for weight, suffix in faces.items():
                for lang in ['latin', 'ja']:
                    selector = f'#font-{lang}-{weight}'
                    node = session.send('DOM.querySelector', {'nodeId': doc, 'selector': selector})['nodeId']
                    rendered = session.send('CSS.getPlatformFontsForNode', {'nodeId': node})['fonts']
                    assert rendered and sum(font['glyphCount'] for font in rendered) > 0, (width, mode, lang, weight)
                    if mode == 'loaded':
                        assert all(font['isCustomFont'] and font['postScriptName'] == 'RoundedMplus1c-' + suffix for font in rendered), (width, lang, weight, rendered)
                    else:
                        assert all(not font['isCustomFont'] for font in rendered), (width, lang, weight, rendered)
                    probes.append({'language': lang, 'weight': weight, 'renderedFonts': rendered})
            font_faces = page.evaluate('Array.from(document.fonts).map(f => ({family:f.family,weight:f.weight,status:f.status}))')
            assert len(font_requests) == 4, (width, mode, font_requests)
            assert all(face['status'] == ('loaded' if mode == 'loaded' else 'error') for face in font_faces), (mode, font_faces)
            page.evaluate('document.querySelector("#font-qa").remove()')
            assert not page.evaluate('document.documentElement.scrollWidth > innerWidth'), (width, mode, 'overflow')
            assert not errors and not external, (width, mode, errors, external)
            avatar = page.get_by_role('button', name='アカウントを開く')
            avatar.click()
            page.get_by_role('button', name='ログアウト', exact=True).wait_for()
            page.keyboard.press('Escape')
            assert avatar.evaluate('(el) => el === document.activeElement')
            page.screenshot(path=str(out / f'{mode}-{width}.png'), full_page=True)
            results.append({'width': width, 'mode': mode, 'faces': font_faces, 'probes': probes, 'fontRequests': font_requests, 'pageErrors': errors, 'externalRequests': external})
            context.close()
    browser.close()

(out / 'report.json').write_text(json.dumps(results, ensure_ascii=False, indent=2))
print('PASS: real Latin/Japanese Regular/Medium/Bold/ExtraBold, 320/390/1440, offline load/failure, overflow and account keyboard access.')
