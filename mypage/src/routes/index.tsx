import { createFileRoute, redirect } from '@tanstack/react-router'
import { AppLoading } from '../components/AppLoading'
import { waitForCurrentUser } from '../features/auth/auth'
import {
	fetchMyPage,
	LinkRequiredError,
	UnauthorizedError,
} from '../features/mypage/api'
import { MyPageView } from '../features/mypage/components/MyPageView'

export const Route = createFileRoute('/')({
	loader: async ({ abortController, location }) => {
		const user = await waitForCurrentUser()

		if (!user) {
			throw redirect({
				to: '/login',
				search: {
					redirect: location.href,
				},
			})
		}

		const idToken = await user.getIdToken()

		try {
			return await fetchMyPage({
				idToken,
				signal: abortController.signal,
			})
		} catch (error) {
			if (error instanceof UnauthorizedError) {
				throw redirect({
					to: '/login',
					search: {
						redirect: location.href,
					},
				})
			}

			if (error instanceof LinkRequiredError) {
				throw redirect({
					to: '/login',
					search: {
						redirect: location.href,
						reason: 'link_required',
					},
				})
			}

			throw error
		}
	},
	pendingComponent: AppLoading,
	component: IndexPage,
})

function IndexPage() {
	const data = Route.useLoaderData()

	return <MyPageView data={data} />
}
