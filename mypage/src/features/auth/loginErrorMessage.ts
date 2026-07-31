import { FirebaseError } from 'firebase/app'

import { ApiError, UnauthorizedError } from '../mypage/api'
import { MissingYouTubeAccessTokenError } from './errors'

const permissionMessage =
	'YouTubeチャンネル情報を確認するため、Googleの確認画面でYouTube情報の読み取りを許可してください。'
const networkMessage =
	'通信エラーが発生しました。ネットワーク接続を確認して、もう一度お試しください。'

export function getLoginErrorMessage(error: unknown): string {
	if (error instanceof MissingYouTubeAccessTokenError) {
		return permissionMessage
	}

	if (error instanceof FirebaseError) {
		switch (error.code) {
			case 'auth/popup-closed-by-user':
			case 'auth/cancelled-popup-request':
				return 'Google / YouTube ログインがキャンセルされました。もう一度お試しください。'
			case 'auth/popup-blocked':
				return 'ログイン用ポップアップがブロックされました。ブラウザの設定を確認して、もう一度お試しください。'
			case 'auth/network-request-failed':
				return networkMessage
			default:
				return 'Google / YouTube ログインに失敗しました。もう一度お試しください。'
		}
	}

	if (error instanceof UnauthorizedError) {
		return '認証の有効期限が切れました。もう一度ログインしてください。'
	}

	if (error instanceof ApiError) {
		if (error.status >= 500) {
			return 'サービスで一時的な問題が発生しています。時間を置いてもう一度お試しください。'
		}

		return 'YouTube連携に失敗しました。もう一度お試しください。'
	}

	if (error instanceof TypeError) {
		return networkMessage
	}

	return '予期しないエラーが発生しました。時間を置いてもう一度お試しください。'
}
