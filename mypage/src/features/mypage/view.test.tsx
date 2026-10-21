import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import fixture from '../../../../docs/mypage/fixtures/available.json'
import recorded from '../../../../docs/mypage/fixtures/recorded-history.json'
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
	it('graphs recorded positive work with a tentative label while unknown days remain unavailable', () => {
		const s = state()
		s.data = mergeResponse(null, recorded as MyPage)
		const html = markup(s)
		expect(html).toContain(
			'aria-label="2026-10-06: 記録済み 0時間 50分（集計未確定）"',
		)
		expect(html).toContain('recorded-bar')
		expect(html).toContain('height:100%')
		expect(html).toContain('合計は未確定です')
		expect(html).toContain('aria-label="2026-09-30: 取得できません"')
		expect(html).not.toMatch(/aria-label="[^"]+: 0時間 0分"/)
		s.data.recent[6].recorded = { data: 30, asOf: recorded.generatedAt }
		expect(markup(s)).toContain('記録済み 1分未満（集計未確定）')
	})
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
		expect(html).not.toContain('作業サマリー')
		expect(html).not.toContain('最新の情報に更新できませんでした')
	})
	it('preserves available statistics when current work alone is unavailable', () => {
		const s = state()
		if (!s.data) throw new Error('synthetic data missing')
		s.data.current = {
			data: null,
			asOf: null,
			stale: true,
			reason: 'SOURCE_UNAVAILABLE',
		}
		s.error = 'TEMPORARY_UNAVAILABLE'
		const html = markup(s)
		expect(html).toContain('情報を取得できませんでした')
		expect(html).toContain('作業サマリー')
		expect(html).toContain('1時間 0分')
		expect(html).toContain('最近の積み重ね')
		expect(html).not.toContain('現在は入室していません')
	})
	it.each([
		['HISTORY_INCOMPLETE', 'この期間の作業記録がそろっていないため'],
		['HISTORY_LIMIT_EXCEEDED', 'この期間の作業記録が多いため'],
		['DATA_INCONSISTENT', '作業記録の整合性を確認できないため'],
	] as const)('explains %s without inventing zero or a fetch failure', (reason, message) => {
		const s = state()
		if (!s.data) throw new Error('synthetic data missing')
		for (const key of ['today', 'week', 'lifetime'] as const)
			s.data.summary[key] = { data: null, asOf: null, stale: true, reason }
		s.data.recent = s.data.recent.map((item) => ({
			...item,
			data: null,
			stale: true,
			reason,
		}))
		const html = markup(s)
		expect(html).toContain(message)
		expect(html).not.toContain('情報を取得できませんでした')
		expect(html).not.toContain('0時間 0分')
	})
	it('keeps the last verified value and timestamp when later history is incomplete', () => {
		const s = state()
		if (!s.data) throw new Error('synthetic data missing')
		s.data.summary.today = {
			data: 3600,
			asOf: '2026-10-21T00:00:00Z',
			stale: true,
			reason: 'HISTORY_INCOMPLETE',
		}
		const html = markup(s)
		expect(html).toContain('1時間 0分')
		expect(html).toContain('更新できませんでした')
		expect(html).toContain('10/21')
	})
})
