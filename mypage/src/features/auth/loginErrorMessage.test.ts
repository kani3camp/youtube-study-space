import { FirebaseError } from 'firebase/app'
import { describe, expect, it } from 'vitest'

import {
	ApiError,
	ChannelAlreadyLinkedError,
	InvalidYouTubeAccessTokenError,
	UnauthorizedError,
} from '../mypage/api'
import { MissingYouTubeAccessTokenError } from './errors'
import { getLoginErrorMessage } from './loginErrorMessage'

describe('getLoginErrorMessage', () => {
	it.each([
		'auth/popup-closed-by-user',
		'auth/cancelled-popup-request',
	])('ログインのキャンセルを案内する: %s', (code) => {
		expect(
			getLoginErrorMessage(new FirebaseError(code, 'cancelled')),
		).toContain('キャンセル')
	})

	it('YouTubeアクセストークンがない場合は権限許可を案内する', () => {
		expect(
			getLoginErrorMessage(new MissingYouTubeAccessTokenError()),
		).toContain('YouTube情報の読み取りを許可')
	})

	it('認証エラーを案内する', () => {
		expect(getLoginErrorMessage(new UnauthorizedError())).toContain(
			'もう一度ログイン',
		)
	})

	it('別アカウントに連携済みのチャンネルを案内する', () => {
		expect(getLoginErrorMessage(new ChannelAlreadyLinkedError())).toContain(
			'別のログインアカウントに連携済み',
		)
	})

	it('YouTubeアクセストークンが無効な場合は再認可を案内する', () => {
		expect(
			getLoginErrorMessage(new InvalidYouTubeAccessTokenError()),
		).toContain('YouTube情報の読み取りを許可')
	})

	it('APIの5xxを一時的なサービス障害として案内する', () => {
		expect(getLoginErrorMessage(new ApiError('failed', 503))).toContain(
			'一時的な問題',
		)
	})

	it('通信失敗をネットワークエラーとして案内する', () => {
		expect(getLoginErrorMessage(new TypeError('Failed to fetch'))).toContain(
			'ネットワーク接続',
		)
	})
})
