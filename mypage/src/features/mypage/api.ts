import { env } from '../../lib/env'
import { createClientHeaders } from './clientHeaders'
import { createMockMyPageResponse } from './mock'
import type { MyPageResponse } from './types'

export class UnauthorizedError extends Error {
	constructor() {
		super('Unauthorized')
		this.name = 'UnauthorizedError'
	}
}

export class LinkRequiredError extends Error {
	constructor() {
		super('YouTube link is required')
		this.name = 'LinkRequiredError'
	}
}

export class ApiError extends Error {
	readonly status: number

	constructor(message: string, status: number) {
		super(message)
		this.name = 'ApiError'
		this.status = status
	}
}

type FetchMyPageOptions = {
	idToken: string
	signal?: AbortSignal
}

type LinkYouTubeOptions = {
	idToken: string
	youtubeAccessToken: string
	signal?: AbortSignal
}

export async function fetchMyPage(
	options: FetchMyPageOptions,
): Promise<MyPageResponse> {
	if (env.useMock) {
		await sleep(300)
		return createMockMyPageResponse()
	}

	const response = await fetch(`${env.mypageApiBaseUrl}/mypage/me`, {
		method: 'GET',
		signal: options.signal,
		headers: createClientHeaders(options.idToken),
	})

	if (response.status === 401) {
		throw new UnauthorizedError()
	}

	if (response.status === 409) {
		throw new LinkRequiredError()
	}

	if (!response.ok) {
		throw new ApiError('マイページ情報の取得に失敗しました', response.status)
	}

	return response.json() as Promise<MyPageResponse>
}

export async function linkYouTube(options: LinkYouTubeOptions): Promise<void> {
	if (env.useMock) {
		await sleep(300)
		return
	}

	const response = await fetch(
		`${env.mypageApiBaseUrl}/mypage/auth/youtube-link`,
		{
			method: 'POST',
			signal: options.signal,
			headers: {
				...createClientHeaders(options.idToken),
				'Content-Type': 'application/json',
			},
			body: JSON.stringify({
				youtubeAccessToken: options.youtubeAccessToken,
			}),
		},
	)

	if (response.status === 401) {
		throw new UnauthorizedError()
	}

	if (!response.ok) {
		throw new ApiError('YouTube連携に失敗しました', response.status)
	}
}

function sleep(ms: number) {
	return new Promise((resolve) => {
		window.setTimeout(resolve, ms)
	})
}
