// Dev-server-only synthetic verification entrypoint, excluded from index.html
// and production build inputs. Never enables mock mode in the product runtime.
import { useState } from 'react'
import { createRoot } from 'react-dom/client'
import fixture from '../../docs/mypage/fixtures/available.json'
import type { MyPage } from './features/mypage/contract'
import { type MemoryState, mergeResponse } from './features/mypage/memory'
import { MyPageView } from './features/mypage/view'
import './style.css'

function VisualApp({ initial }: { initial: MemoryState }) {
	const [state, setState] = useState(initial)
	return (
		<MyPageView
			state={state}
			refresh={() => {}}
			logout={() => setState({ ...state, phase: 'anonymous', data: null })}
			cookieSettings={() => {}}
		/>
	)
}

if (import.meta.env.DEV) {
	const sample = structuredClone(fixture) as MyPage
	const mode = new URLSearchParams(location.search).get('state')
	if (sample.current.data) {
		if (mode === 'break') sample.current.data.state = 'break'
		if (mode === 'not-seated') sample.current.data.state = 'not_seated'
		if (mode === 'unregistered') sample.current.data.state = 'unregistered'
		if (mode === 'long')
			sample.current.data.workName =
				'長い作業名の確認です。途中で省略せず、内容がすべて読めることを確認するための合成文章です。'.repeat(
					4,
				)
		if (mode === 'overdue')
			sample.current.data.expectedEndAt = '2026-10-06T02:30:00Z'
	}
	if (mode === 'zero' && sample.summary.data) {
		for (const metric of Object.values(sample.summary.data))
			if (metric.workSec !== null) metric.workSec = 0
	}
	const state: MemoryState = {
		phase: 'authenticated',
		data: mergeResponse(null, sample),
		busy: false,
		error: null,
		receivedAt: Date.parse(sample.generatedAt),
	}
	if (mode === 'metadata-expired') {
		sample.account = {
			availability: 'unavailable',
			reasonCode: 'METADATA_TOO_OLD',
			data: null,
		}
		state.data = mergeResponse(state.data, sample)
	}
	if (mode === 'loading') {
		state.busy = true
		state.data = null
	}
	if (mode === 'failure') {
		state.error = 'TEMPORARY_UNAVAILABLE'
		state.data = null
		state.receivedAt = null
	}
	if (mode === 'refresh-failure') state.error = 'TEMPORARY_UNAVAILABLE'
	if (mode === 'metadata-failure' && state.data)
		state.data.account = {
			data: null,
			asOf: null,
			stale: true,
			reason: 'METADATA_TOO_OLD',
		}
	const root = document.getElementById('root')
	if (root) createRoot(root).render(<VisualApp initial={state} />)
}
