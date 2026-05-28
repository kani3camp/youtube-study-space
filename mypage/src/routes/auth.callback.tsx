import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'

type CallbackSearch = {
	redirect?: string
}

export const Route = createFileRoute('/auth/callback')({
	validateSearch: (search): CallbackSearch => {
		return {
			redirect: typeof search.redirect === 'string' ? search.redirect : '/',
		}
	},
	component: AuthCallbackPage,
})

function AuthCallbackPage() {
	const navigate = useNavigate()
	const search = Route.useSearch()

	useEffect(() => {
		void navigate({
			to: search.redirect ?? '/',
			replace: true,
		})
	}, [navigate, search.redirect])

	return (
		<section className="cardStack">
			<div className="card">
				<h2>連携を確認しています</h2>
				<p className="mutedText">しばらくするとマイページへ移動します。</p>
			</div>
		</section>
	)
}
