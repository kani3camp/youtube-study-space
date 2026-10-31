import { PHASE_PRODUCTION_BUILD } from 'next/constants'

export function register(): void {
	if (
		process.env.NEXT_RUNTIME !== 'nodejs' ||
		process.env.NEXT_PHASE === PHASE_PRODUCTION_BUILD
	) {
		return
	}

	console.info(
		`[monitor] ログインURL: http://localhost:${process.env.PORT ?? '3000'}/monitor-login`,
	)
}
