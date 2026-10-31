import { PHASE_PRODUCTION_BUILD } from 'next/constants'
import { register } from './instrumentation'

describe('monitor startup login URL', () => {
	afterEach(() => {
		vi.unstubAllEnvs()
		vi.restoreAllMocks()
	})

	test.each(['18080', '3001', undefined])(
		'logs the login URL for server port %s',
		(port) => {
			vi.stubEnv('NEXT_RUNTIME', 'nodejs')
			vi.stubEnv('NEXT_PHASE', undefined)
			vi.stubEnv('PORT', port)
			const log = vi.spyOn(console, 'info').mockImplementation(() => undefined)

			register()

			expect(log).toHaveBeenCalledExactlyOnceWith(
				`[monitor] ログインURL: http://localhost:${port ?? '3000'}/monitor-login`,
			)
		},
	)

	test('does not announce a running server during build', () => {
		vi.stubEnv('NEXT_RUNTIME', 'nodejs')
		vi.stubEnv('NEXT_PHASE', PHASE_PRODUCTION_BUILD)
		const log = vi.spyOn(console, 'info').mockImplementation(() => undefined)

		register()

		expect(log).not.toHaveBeenCalled()
	})

	test('does not log from the edge runtime', () => {
		vi.stubEnv('NEXT_RUNTIME', 'edge')
		const log = vi.spyOn(console, 'info').mockImplementation(() => undefined)

		register()

		expect(log).not.toHaveBeenCalled()
	})
})
