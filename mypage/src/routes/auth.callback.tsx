import { createFileRoute, useRouter } from '@tanstack/react-router'
import { useEffect } from 'react'

import { sanitizeRedirectPath } from '../lib/safeRedirect'

type CallbackSearch = {
	redirect: string
}

export const Route = createFileRoute('/auth/callback')({
	validateSearch: (search): CallbackSearch => {
		return {
			redirect: sanitizeRedirectPath(
				typeof search.redirect === 'string' ? search.redirect : undefined,
			),
		}
	},
	component: AuthCallbackPage,
})

function AuthCallbackPage() {
	const router = useRouter()
	const search = Route.useSearch()

	useEffect(() => {
		router.history.replace(search.redirect)
	}, [router, search.redirect])

	return (
		<section className="cardStack">
			<div className="card">
				<h2>連携を確認しています</h2>
				<p className="mutedText">しばらくするとマイページへ移動します。</p>
			</div>
		</section>
	)
}
