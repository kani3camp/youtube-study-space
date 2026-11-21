import {
	createMemoryHistory,
	createRootRoute,
	createRouter,
	RouterContextProvider,
} from '@tanstack/react-router'
import { renderToStaticMarkup } from 'react-dom/server'
import { SiteHeader } from './components/site-header'
import { PrivacyPolicy, TermsOfUse } from './policies'

export function renderStaticPolicies() {
	const router = createRouter({
		routeTree: createRootRoute(),
		history: createMemoryHistory(),
	})

	return [
		{
			filename: 'privacy.html',
			title: 'プライバシーポリシー',
			Policy: PrivacyPolicy,
		},
		{ filename: 'terms.html', title: '利用規約', Policy: TermsOfUse },
	].map(({ filename, title, Policy }) => ({
		filename,
		title,
		body: renderToStaticMarkup(
			<RouterContextProvider router={router}>
				<div className="app-shell public-shell">
					<SiteHeader />
					<Policy />
				</div>
			</RouterContextProvider>,
		),
	}))
}
