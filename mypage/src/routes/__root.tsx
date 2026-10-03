import { createRootRoute, Outlet } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { AppError } from '../components/AppError'
import { AppLayout } from '../components/AppLayout'

export const Route = createRootRoute({
	component: RootComponent,
	errorComponent: AppError,
})

function RootComponent() {
	return (
		<AppLayout>
			<Outlet />
			{import.meta.env.DEV ? <TanStackRouterDevtools /> : null}
		</AppLayout>
	)
}
