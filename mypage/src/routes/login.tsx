import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { signInWithGoogleAndYouTube } from '../features/auth/auth'
import { linkYouTube } from '../features/mypage/api'

type LoginSearch = {
	redirect?: string
	reason?: string
}

export const Route = createFileRoute('/login')({
	validateSearch: (search): LoginSearch => {
		return {
			redirect: typeof search.redirect === 'string' ? search.redirect : '/',
			reason: typeof search.reason === 'string' ? search.reason : undefined,
		}
	},
	component: LoginPage,
})

function LoginPage() {
	const navigate = useNavigate()
	const search = Route.useSearch()
	const [isSubmitting, setIsSubmitting] = useState(false)
	const [errorMessage, setErrorMessage] = useState<string | null>(null)

	async function handleLogin() {
		setIsSubmitting(true)
		setErrorMessage(null)

		try {
			const result = await signInWithGoogleAndYouTube()

			await linkYouTube({
				idToken: result.idToken,
				youtubeAccessToken: result.youtubeAccessToken,
			})

			await navigate({
				to: search.redirect ?? '/',
				replace: true,
			})
		} catch (error) {
			console.error(error)
			setErrorMessage(
				'YouTubeチャンネル情報を確認するため、Googleの確認画面でYouTube情報の読み取りを許可してください。',
			)
		} finally {
			setIsSubmitting(false)
		}
	}

	return (
		<section className="cardStack">
			<div className="card">
				<h2>ログイン</h2>

				{search.reason === 'link_required' ? (
					<p className="mutedText">
						YouTube連携が必要です。Googleの確認画面でYouTube情報の読み取りを許可してください。
					</p>
				) : (
					<p className="mutedText">
						YouTube
						アカウントで連携すると、オンライン作業部屋の現在状態と作業時間を確認できます。
					</p>
				)}

				<p className="mutedText">
					マイページMVPでは、入室・退室・作業内容変更などの書き込み操作は行いません。
				</p>

				{errorMessage ? <p className="errorText">{errorMessage}</p> : null}

				<button
					className="primaryButton"
					type="button"
					disabled={isSubmitting}
					onClick={handleLogin}
				>
					{isSubmitting ? '連携中...' : 'Google / YouTube でログイン'}
				</button>
			</div>
		</section>
	)
}
