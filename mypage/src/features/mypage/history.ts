import { checkedJSON, type TokenSource } from './client'
import { RequestError } from './memory'

export type HistoryInput = {
	from: string
	to: string
	limit?: number
	cursor?: string
}
export type HistoryEntry = {
	key: string
	startedAt: string
	endedAt: string
	workSec: number
	workName: string
}
export type HistoryPage = {
	asOf: string
	timezone: 'Asia/Tokyo'
	selection: 'completed_work'
	coverage: 'recorded_only'
	sourceConsistent: boolean
	from: string
	to: string
	items: HistoryEntry[]
	nextCursor: string | null
}
export class HistoryRequestError extends RequestError {
	readonly resetAt: string | null
	constructor(error: RequestError, resetAt: string | null) {
		super(error.status, error.code, error.retryAfter)
		this.resetAt = resetAt
	}
}

function object(value: unknown): Record<string, unknown> | null {
	return value !== null && typeof value === 'object' && !Array.isArray(value)
		? (value as Record<string, unknown>)
		: null
}
function date(value: unknown): value is string {
	return typeof value === 'string' && Number.isFinite(Date.parse(value))
}

export function parseHistoryPage(
	value: unknown,
	input: HistoryInput,
): HistoryPage {
	const page = object(value)
	const fields = new Set([
		'asOf',
		'timezone',
		'selection',
		'coverage',
		'sourceConsistent',
		'from',
		'to',
		'items',
		'nextCursor',
	])
	if (
		!page ||
		!Object.keys(page).every((key) => fields.has(key)) ||
		!date(page.asOf) ||
		page.timezone !== 'Asia/Tokyo' ||
		page.selection !== 'completed_work' ||
		page.coverage !== 'recorded_only' ||
		typeof page.sourceConsistent !== 'boolean' ||
		page.from !== input.from ||
		page.to !== input.to ||
		!Array.isArray(page.items) ||
		page.items.length > (input.limit ?? 50) ||
		!(
			page.nextCursor === null ||
			(typeof page.nextCursor === 'string' &&
				/^[A-Za-z0-9_-]{28,2048}$/.test(page.nextCursor))
		)
	)
		throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
	const keys = new Set<string>()
	for (const raw of page.items) {
		const entry = object(raw)
		if (
			!entry ||
			!Object.keys(entry).every((key) =>
				['key', 'startedAt', 'endedAt', 'workSec', 'workName'].includes(key),
			) ||
			typeof entry.key !== 'string' ||
			!/^[a-f0-9]{64}$/.test(entry.key) ||
			keys.has(entry.key) ||
			!date(entry.startedAt) ||
			!date(entry.endedAt) ||
			Date.parse(entry.endedAt) < Date.parse(entry.startedAt) ||
			typeof entry.workSec !== 'number' ||
			!Number.isSafeInteger(entry.workSec) ||
			entry.workSec < 0 ||
			typeof entry.workName !== 'string'
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		keys.add(entry.key)
	}
	return value as HistoryPage
}

export function createHistoryLoader(
	tokens: TokenSource,
	request: typeof fetch = fetch,
) {
	return async (
		uid: string,
		input: HistoryInput,
		signal: AbortSignal,
		requestKey: string,
	): Promise<HistoryPage> => {
		const query = new URLSearchParams({
			from: input.from,
			to: input.to,
			limit: String(input.limit ?? 50),
		})
		if (input.cursor) query.set('cursor', input.cursor)
		let forceToken = false
		let forceAppCheck = false
		for (;;) {
			try {
				const [idToken, appCheck] = await Promise.all([
					tokens.idToken(uid, forceToken),
					tokens.appCheck(forceAppCheck),
				])
				if (signal.aborted || tokens.currentUID() !== uid)
					throw new RequestError(401, 'AUTH_REQUIRED')
				const response = await request(`/api/mypage/history?${query}`, {
					headers: {
						Authorization: `Bearer ${idToken}`,
						'X-Firebase-AppCheck': appCheck,
						'Idempotency-Key': requestKey,
					},
					signal: AbortSignal.any([signal, AbortSignal.timeout(10_000)]),
					credentials: 'same-origin',
					cache: 'no-store',
				})
				let value: unknown
				try {
					value = await checkedJSON(response)
				} catch (error) {
					if (error instanceof RequestError && error.status === 429) {
						const reset = response.headers.get('X-Quota-Reset')
						throw new HistoryRequestError(error, date(reset) ? reset : null)
					}
					throw error
				}
				if (signal.aborted || tokens.currentUID() !== uid)
					throw new RequestError(401, 'AUTH_REQUIRED')
				return parseHistoryPage(value, input)
			} catch (error) {
				if (signal.aborted || tokens.currentUID() !== uid)
					throw new RequestError(401, 'AUTH_REQUIRED')
				if (error instanceof RequestError) {
					if (error.status === 401 && !forceToken) {
						forceToken = true
						continue
					}
					if (error.code === 'APP_CHECK_REQUIRED' && !forceAppCheck) {
						forceAppCheck = true
						continue
					}
					throw error
				}
				throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
			}
		}
	}
}
