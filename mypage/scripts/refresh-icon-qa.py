"""合成MyPageで更新アイコンの接続・表示とヘッダーの操作性を実ブラウザ検証する。"""
import argparse
import json
from pathlib import Path

from playwright.sync_api import expect, sync_playwright

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--chromium', default='/usr/bin/chromium')
parser.add_argument('--output', type=Path)
args = parser.parse_args()
if args.output:
    args.output.mkdir(parents=True, exist_ok=True)
base = 'http://127.0.0.1:18081'
results = []

with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=args.chromium, headless=True)
    for width in [320, 390, 768, 1024, 1440]:
        context = browser.new_context(
            viewport={'width': width, 'height': 900}, device_scale_factor=3,
            is_mobile=width < 560, has_touch=width < 560, service_workers='block',
        )
        page = context.new_page()
        errors, external = [], []
        page.on('pageerror', lambda error: errors.append(str(error)))

        def guard(route):
            if route.request.url.startswith(base + '/'):
                route.continue_()
            else:
                external.append(route.request.resource_type)
                route.abort()

        page.route('**/*', guard)
        for state, busy in [('work', False), ('loading', True)]:
            page.goto(base + '/visual.html?state=' + state)
            page.evaluate('document.fonts.ready')
            button = page.get_by_role('button', name='最新の情報に更新', exact=True)
            expect(button).to_be_visible()
            expect(button).to_have_text('更新中…' if busy else '更新')
            if busy:
                expect(button).to_be_disabled()
            else:
                expect(button).to_be_enabled()
                button.focus()
                expect(button).to_be_focused()
                button.press('Enter')
            icon = button.locator('svg')
            expect(icon).to_have_attribute('aria-hidden', 'true')
            # 各矢じりは等長の2辺。円弧を途中の軸へつなぐと、先端から軸が突き出す。
            # パス文字列のスナップショットではなく、描画エンジンの接続点を比較する。
            geometry = icon.evaluate('''svg => {
                const split = p => p.getAttribute('d').match(/M[^M]+/g).map(d => {
                    const path = document.createElementNS(svg.namespaceURI, 'path');
                    path.setAttribute('d', d);
                    return path;
                });
                const [heads, arcs] = [...svg.querySelectorAll('path')].map(split);
                const at = (p, fraction) => p.getPointAtLength(p.getTotalLength() * fraction);
                const distance = (a, b) => Math.hypot(a.x - b.x, a.y - b.y);
                return {
                    joins: [distance(at(heads[0], .5), at(arcs[0], 1)),
                            distance(at(heads[1], .5), at(arcs[1], 0))],
                    symmetry: [0, .25, .5, .75, 1].map(t => {
                        const a = at(heads[0], t), b = at(heads[1], t);
                        return Math.hypot(a.x + b.x - 24, a.y + b.y - 24);
                    }),
                    bounds: svg.getBoundingClientRect().toJSON(),
                };
            }''')
            assert max(geometry['joins'] + geometry['symmetry']) < 0.05, geometry
            bounds = geometry['bounds']
            assert bounds['width'] == bounds['height'] == 17, bounds
            assert 0 <= bounds['left'] < bounds['right'] <= width, bounds
            assert button.bounding_box()['height'] >= 44
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth')
            if args.output:
                page.locator('.site-header').screenshot(path=str(args.output / f'{width}-{state}.png'))
            results.append({'width': width, 'state': state, 'joins': geometry['joins']})
        assert not errors and not external, (errors, external)
        context.close()
    browser.close()

print(json.dumps(results, ensure_ascii=False, indent=2))
print('PASS: connected symmetric refresh arrows, 17px square, 44px control, focus and busy state; Chromium viewport/touch emulation, not a physical iPhone.')
