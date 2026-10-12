import type { MyPage } from './contract'
import { type Loader, RequestError } from './memory'

export type TokenSource = {
	currentUID: () => string | null
	idToken: (uid: string, force: boolean) => Promise<string>
	appCheck: (force: boolean) => Promise<string>
}

const reasons = new Set([
	'SOURCE_UNAVAILABLE',
	'HISTORY_INCOMPLETE',
	'HISTORY_LIMIT_EXCEEDED',
	'DATA_INCONSISTENT',
	'METADATA_REFRESH_FAILED',
	'METADATA_TOO_OLD',
])
const publicErrors = new Set([
	'INVALID_REQUEST',
	'PAYLOAD_TOO_LARGE',
	'PRIVACY_CONSENT_REQUIRED',
	'POLICY_VERSION_OUTDATED',
	'AUTH_REQUIRED',
	'APP_CHECK_REQUIRED',
	'OAUTH_TRANSACTION_REQUIRED',
	'OAUTH_TRANSACTION_PENDING',
	'OAUTH_TRANSACTION_EXPIRED',
	'OAUTH_TRANSACTION_CONSUMED',
	'OAUTH_TRANSACTION_CHANGED',
	'PRIVACY_RECONSENT_REQUIRED',
	'SUPPORT_CHALLENGE_INVALID',
	'SUPPORT_CHANNEL_MISMATCH',
	'INTAKE_KEY_CONFLICT',
	'OAUTH_FAILED',
	'OAUTH_SCOPE_INSUFFICIENT',
	'CHANNEL_UNAVAILABLE',
	'CHANNEL_AMBIGUOUS',
	'WEB_ACCOUNT_REQUIRED',
	'SERVICE_ACCESS_RESTRICTED',
	'DATA_DELETION_IN_PROGRESS',
	'RATE_LIMITED',
	'TEMPORARY_UNAVAILABLE',
	'INTERNAL_ERROR',
])

function object(value: unknown): Record<string, unknown> | null {
	return typeof value === 'object' && value !== null && !Array.isArray(value)
		? (value as Record<string, unknown>)
		: null
}
function date(value: unknown) {
	return (
		typeof value === 'string' &&
		/^\d{4}-\d{2}-\d{2}T/.test(value) &&
		Number.isFinite(Date.parse(value))
	)
}
function nullableString(value: unknown) {
	return value === null || typeof value === 'string'
}
function metric(value: unknown) {
	const v = object(value)
	return (
		v !== null &&
		((v.availability === 'available' &&
			v.reasonCode === null &&
			typeof v.workSec === 'number' &&
			Number.isSafeInteger(v.workSec) &&
			v.workSec >= 0) ||
			(v.availability === 'unavailable' &&
				v.workSec === null &&
				typeof v.reasonCode === 'string' &&
				reasons.has(v.reasonCode)))
	)
}
function section(value: unknown, dataCheck: (data: unknown) => boolean) {
	const v = object(value)
	if (!v) return false
	if (v.availability === 'unavailable')
		return (
			v.data === null &&
			typeof v.reasonCode === 'string' &&
			reasons.has(v.reasonCode)
		)
	if (v.availability === 'available')
		return v.reasonCode === null && dataCheck(v.data)
	return (
		v.availability === 'partial' &&
		typeof v.reasonCode === 'string' &&
		reasons.has(v.reasonCode) &&
		dataCheck(v.data)
	)
}

// Fail malformed/unsafe numeric responses as unavailable; never repair them to
// zero or persist any raw response. React escapes user-controlled text later.
export function parseMyPage(value: unknown): MyPage {
	const v = object(value)
	const current = (data: unknown) => {
		const d = object(data)
		return (
			!!d &&
			['work', 'break', 'not_seated', 'unregistered'].includes(
				String(d.state),
			) &&
			nullableString(d.workName) &&
			(d.roomType === null ||
				d.roomType === 'standard' ||
				d.roomType === 'member') &&
			(d.seatNumber === null ||
				(typeof d.seatNumber === 'number' &&
					Number.isSafeInteger(d.seatNumber) &&
					d.seatNumber > 0)) &&
			(d.stateStartedAt === null || date(d.stateStartedAt)) &&
			(d.expectedEndAt === null || date(d.expectedEndAt))
		)
	}
	const summary = (data: unknown) => {
		const d = object(data)
		return !!d && metric(d.today) && metric(d.week) && metric(d.lifetime)
	}
	const account = (data: unknown) => {
		const d = object(data)
		return (
			!!d &&
			typeof d.displayName === 'string' &&
			d.displayName.length > 0 &&
			nullableString(d.handle) &&
			nullableString(d.avatarUrl)
		)
	}
	const recent = object(v?.recent7Days)
	if (
		!v ||
		!date(v.generatedAt) ||
		v.timezone !== 'Asia/Tokyo' ||
		typeof v.partial !== 'boolean' ||
		!section(v.current, current) ||
		!section(v.summary, summary) ||
		!section(v.account, account) ||
		!recent ||
		!['available', 'partial', 'unavailable'].includes(
			String(recent.availability),
		) ||
		!(
			recent.reasonCode === null ||
			(typeof recent.reasonCode === 'string' && reasons.has(recent.reasonCode))
		) ||
		!Array.isArray(recent.data) ||
		recent.data.length !== 7
	)
		throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
	let previous = Number.NEGATIVE_INFINITY
	for (const item of recent.data) {
		const d = object(item)
		if (
			!d ||
			typeof d.date !== 'string' ||
			!/^\d{4}-\d{2}-\d{2}$/.test(d.date) ||
			!metric(d)
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		const at = Date.parse(`${d.date}T00:00:00+09:00`)
		if (
			!Number.isFinite(at) ||
			new Date(at + 9 * 3600000).toISOString().slice(0, 10) !== d.date ||
			(Number.isFinite(previous) && at - previous !== 86400000)
		)
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		previous = at
	}
	if (
		new Date(Date.parse(String(v.generatedAt)) + 9 * 3600000)
			.toISOString()
			.slice(0, 10) !== object(recent.data.at(-1))?.date
	)
		throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
	return value as MyPage
}

export async function checkedJSON(response: Response): Promise<unknown> {
	if (!response.ok) {
		let code =
			response.status === 401 ? 'AUTH_REQUIRED' : 'TEMPORARY_UNAVAILABLE'
		try {
			const body = object(await response.json())
			const error = object(body?.error)
			if (typeof error?.code === 'string' && publicErrors.has(error.code))
				code = error.code
		} catch {
			/* Never show dependency/parser details. */
		}
		const rawRetry = response.headers.get('Retry-After') ?? ''
		const retryAfter = /^\d+$/.test(rawRetry)
			? Math.min(3600, Number(rawRetry))
			: 0
		throw new RequestError(response.status, code, retryAfter)
	}
	if (response.status === 204) return null
	try {
		return await response.json()
	} catch {
		throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
	}
}

export function createLoader(
	tokens: TokenSource,
	request: typeof fetch = fetch,
): Loader {
	return async (uid, signal, forceToken, forceAppCheck) => {
		try {
			const [idToken, appCheck] = await Promise.all([
				tokens.idToken(uid, forceToken),
				tokens.appCheck(forceAppCheck),
			])
			if (signal.aborted || tokens.currentUID() !== uid)
				throw new RequestError(401, 'AUTH_REQUIRED')
			const response = await request('/api/mypage', {
				headers: {
					Authorization: `Bearer ${idToken}`,
					'X-Firebase-AppCheck': appCheck,
				},
				signal: AbortSignal.any([signal, AbortSignal.timeout(10000)]),
				credentials: 'same-origin',
				cache: 'no-store',
			})
			return parseMyPage(await checkedJSON(response))
		} catch (error) {
			if (error instanceof RequestError) throw error
			throw new RequestError(503, 'TEMPORARY_UNAVAILABLE')
		}
	}
}
