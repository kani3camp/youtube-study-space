import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import fixture from '../../../../docs/mypage/fixtures/available.json'
import type { MyPage } from './contract'
import { type MemoryState, mergeResponse } from './memory'
import { MyPageView } from './view'

function markup(state: MemoryState) {
	return renderToStaticMarkup(
		<MyPageView
			state={state}
			refresh={() => {}}
			logout={() => {}}
			cookieSettings={() => {}}
		/>,
	)
}
function state(): MemoryState {
	return {
		phase: 'authenticated',
		data: mergeResponse(null, structuredClone(fixture) as MyPage),
		busy: false,
		error: null,
		receivedAt: null,
	}
}

describe('MyPage observable display', () => {
	it('keeps work name during break and never infers a state transition from time', () => {
		const s = state()
		if (!s.data?.current.data) throw new Error('synthetic current missing')
		s.data.current.data.state = 'break'
		s.data.current.data.workName = '<script>synthetic</script>'
		s.data.current.data.expectedEndAt = '2026-10-06T02:30:00Z'
		const html = markup(s)
		expect(html).toContain('休憩中')
		expect(html).toContain('いまの作業')
		expect(html).toContain('休憩開始')
		expect(html).toContain('終了予定を過ぎています')
		expect(html).toContain('&lt;script&gt;synthetic&lt;/script&gt;')
		expect(html).not.toContain('<script>synthetic')
	})
	it('does not render retained private values while auth is absent or unresolved', () => {
		for (const phase of [
			'anonymous',
			'bootstrapping',
			'signing-out',
		] as const) {
			const s = state()
			s.phase = phase
			const html = markup(s)
			expect(html).not.toContain('読書')
			expect(html).not.toContain('Sample')
			expect(html).not.toContain('席 <strong>12')
		}
	})
	it('gives every day including zero a named selectable button and text outside the graph', () => {
		const html = markup(state())
		expect(html).toContain('aria-label="2026-09-30: 0時間 0分"')
		expect(html.match(/class="day-button/g)?.length).toBe(7)
		expect(html).toContain('aria-live="polite"')
	})
	it('shows missing first-load values as unavailable rather than normal zero', () => {
		const s = state()
		s.data = null
		s.error = 'TEMPORARY_UNAVAILABLE'
		const html = markup(s)
		expect(html).toContain('情報を取得できませんでした')
		expect(html).not.toContain('0時間 0分')
	})
})
