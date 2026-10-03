import { env } from '../../lib/env'

export function createClientHeaders(idToken: string): HeadersInit {
	const headers: Record<string, string> = {
		Authorization: `Bearer ${idToken}`,
		Accept: 'application/json',
		'X-Client-App': 'mypage',
		'X-Client-Version': env.clientVersion,
		'X-Client-Request-Id': crypto.randomUUID(),
	}

	if (env.clientBuildTime !== '') {
		headers['X-Client-Build-Time'] = env.clientBuildTime
	}

	const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone
	if (timezone) {
		headers['X-Client-Timezone'] = timezone
	}

	return headers
}
