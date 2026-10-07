import { describe, expect, it, vi } from 'vitest'
import available from '../../../../docs/mypage/fixtures/available.json'
import partial from '../../../../docs/mypage/fixtures/partial.json'
import {
	checkedJSON,
	createLoader,
	parseMyPage,
	type TokenSource,
} from './client'
import { RequestError } from './memory'

function tokens(): TokenSource {
	return {
		currentUID: () => 'synthetic',
		idToken: async () => 'synthetic-id',
		appCheck: async () => 'synthetic-app',
	}
}

describe('same-origin MyPage client boundary', () => {
	it('accepts canonical available and partial fixtures without filling missing data', () => {
		expect(parseMyPage(available)).toEqual(available)
		expect(parseMyPage(partial)).toEqual(partial)
	})
	it('rejects unsafe seconds, malformed availability, date gaps and wrong snapshot day', () => {
		for (const mutate of [
			(v: typeof available) => {
				v.summary.data.today.workSec = -1
			},
			(v: typeof available) => {
				v.summary.data.today.workSec = Number.MAX_SAFE_INTEGER + 1
			},
			(v: typeof available) => {
				v.current.availability = 'unavailable'
			},
			(v: typeof available) => {
				v.recent7Days.data[0].date = '2026-02-30'
			},
			(v: typeof available) => {
				v.recent7Days.data[1].date = v.recent7Days.data[0].date
			},
			(v: typeof available) => {
				v.generatedAt = '2026-10-07T03:00:00Z'
			},
		]) {
			const input = structuredClone(available)
			mutate(input)
			expect(() => parseMyPage(input)).toThrow(RequestError)
		}
	})
	it('sends proofs in headers with no-store and never accepts a client target id', async () => {
		const request = vi.fn<typeof fetch>(async () => Response.json(available))
		await createLoader(tokens(), request)(
			'synthetic',
			new AbortController().signal,
			false,
			false,
		)
		expect(request.mock.calls[0]?.[0]).toBe('/api/mypage')
		const init = request.mock.calls[0]?.[1] as RequestInit
		expect(init.headers).toEqual({
			Authorization: 'Bearer synthetic-id',
			'X-Firebase-AppCheck': 'synthetic-app',
		})
		expect(init.cache).toBe('no-store')
		expect(init.credentials).toBe('same-origin')
	})
	it('does not dispatch after an identity change or an abort during proof retrieval', async () => {
		for (const aborted of [true, false]) {
			const source = tokens()
			const controller = new AbortController()
			source.idToken = async () => {
				if (aborted) controller.abort()
				else source.currentUID = () => 'another'
				return 'synthetic-id'
			}
			const request = vi.fn<typeof fetch>(async () => Response.json(available))
			await expect(
				createLoader(source, request)(
					'synthetic',
					controller.signal,
					false,
					false,
				),
			).rejects.toMatchObject({ code: 'AUTH_REQUIRED' })
			expect(request).not.toHaveBeenCalled()
		}
	})
	it('allows stable error codes only and caps numeric Retry-After', async () => {
		await expect(
			checkedJSON(
				Response.json(
					{ error: { code: 'private dependency detail', message: 'private' } },
					{ status: 503 },
				),
			),
		).rejects.toMatchObject({
			code: 'TEMPORARY_UNAVAILABLE',
			message: 'MyPage request failed',
		})
		await expect(
			checkedJSON(
				Response.json(
					{ error: { code: 'RATE_LIMITED' } },
					{ status: 429, headers: { 'Retry-After': '9999999999' } },
				),
			),
		).rejects.toMatchObject({ retryAfter: 3600 })
	})
	it.each([
		'SERVICE_ACCESS_RESTRICTED',
		'DATA_DELETION_IN_PROGRESS',
	])('preserves the stable restriction code %s without exposing server details', async (code) => {
		await expect(
			checkedJSON(
				Response.json(
					{ error: { code, message: 'synthetic-private-channel' } },
					{ status: 403 },
				),
			),
		).rejects.toMatchObject({
			status: 403,
			code,
			message: 'MyPage request failed',
		})
	})
})
