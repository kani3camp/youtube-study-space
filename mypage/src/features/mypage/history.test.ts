import { describe, expect, it, vi } from 'vitest'
import type { TokenSource } from './client'
import {
	createHistoryLoader,
	type HistoryPage,
	HistoryRequestError,
	parseHistoryPage,
} from './history'

const input = { from: '2026-10-01', to: '2026-10-02', limit: 50 }
function sample(): HistoryPage {
	return {
		asOf: '2026-10-24T03:00:00Z',
		timezone: 'Asia/Tokyo',
		selection: 'completed_work',
		coverage: 'recorded_only',
		sourceConsistent: true,
		from: input.from,
		to: input.to,
		items: [],
		nextCursor: null,
	} as HistoryPage
}

describe('bounded history client', () => {
	it('accepts recorded empty pages but rejects null numeric values, extra private data, wrong period and duplicate row keys', () => {
		const page = sample()
		delete (page as unknown as Record<string, unknown>).limit
		expect(parseHistoryPage(page, input).items).toEqual([])
		const entry = {
			key: 'a'.repeat(64),
			startedAt: '2026-10-01T00:00:00Z',
			endedAt: '2026-10-01T00:01:00Z',
			workSec: 60,
			workName: 'synthetic',
		}
		for (const invalid of [
			{ ...page, uid: 'private-owner' },
			{ ...page, from: '2026-09-01' },
			{ ...page, nextCursor: '' },
			{ ...page, items: [{ ...entry, workSec: null }] },
			{ ...page, items: [entry, entry] },
			{ ...page, items: [{ ...entry, documentPath: 'private-source' }] },
		])
			expect(() => parseHistoryPage(invalid, input)).toThrow()
	})
	it('retries Auth and App Check once with the same logical operation key and never sends uid/plan', async () => {
		const tokens: TokenSource = {
			currentUID: () => 'synthetic',
			idToken: vi.fn(async () => 'synthetic-id'),
			appCheck: vi.fn(async () => 'synthetic-app'),
		}
		const page = sample()
		delete (page as unknown as Record<string, unknown>).limit
		const request = vi
			.fn<typeof fetch>()
			.mockResolvedValueOnce(
				Response.json({ error: { code: 'AUTH_REQUIRED' } }, { status: 401 }),
			)
			.mockResolvedValueOnce(
				Response.json(
					{ error: { code: 'APP_CHECK_REQUIRED' } },
					{ status: 403 },
				),
			)
			.mockResolvedValue(Response.json(page))
		await expect(
			createHistoryLoader(tokens, request)(
				'synthetic',
				input,
				new AbortController().signal,
				'synthetic-operation-key',
			),
		).resolves.toEqual(page)
		expect(request).toHaveBeenCalledTimes(3)
		for (const [url, options] of request.mock.calls) {
			expect(String(url)).not.toMatch(/uid=|plan=|offset=/)
			expect(options).toMatchObject({
				cache: 'no-store',
				credentials: 'same-origin',
			})
			expect(new Headers(options?.headers).get('Idempotency-Key')).toBe(
				'synthetic-operation-key',
			)
		}
		expect(tokens.idToken).toHaveBeenNthCalledWith(2, 'synthetic', true)
		expect(tokens.appCheck).toHaveBeenLastCalledWith(true)
	})
	it('retains the server daily reset independently of the ordinary one-hour retry clamp', async () => {
		const tokens: TokenSource = {
			currentUID: () => 'synthetic',
			idToken: async () => 'synthetic-id',
			appCheck: async () => 'synthetic-app',
		}
		const reset = '2026-10-24T15:00:00Z'
		const request = vi.fn<typeof fetch>(async () =>
			Response.json(
				{ error: { code: 'ANALYSIS_BUDGET_EXHAUSTED' } },
				{
					status: 429,
					headers: { 'Retry-After': '80000', 'X-Quota-Reset': reset },
				},
			),
		)
		const failure = await createHistoryLoader(tokens, request)(
			'synthetic',
			input,
			new AbortController().signal,
			'synthetic-operation-key',
		).catch((error) => error)
		expect(failure).toBeInstanceOf(HistoryRequestError)
		expect(failure).toMatchObject({
			status: 429,
			code: 'ANALYSIS_BUDGET_EXHAUSTED',
			resetAt: reset,
		})
		expect(request).toHaveBeenCalledTimes(1)
	})
	it('discards a late page after identity changes even if fetch ignores abort', async () => {
		let uid = 'old'
		let release: (response: Response) => void = () => {}
		const tokens: TokenSource = {
			currentUID: () => uid,
			idToken: async () => 'synthetic-id',
			appCheck: async () => 'synthetic-app',
		}
		const request: typeof fetch = async () =>
			new Promise<Response>((resolve) => {
				release = resolve
			})
		const pending = createHistoryLoader(tokens, request)(
			'old',
			input,
			new AbortController().signal,
			'synthetic-operation-key',
		)
		for (let i = 0; i < 8; i++) await Promise.resolve()
		uid = 'new'
		release(Response.json(sample()))
		await expect(pending).rejects.toMatchObject({ status: 401 })
	})
})
