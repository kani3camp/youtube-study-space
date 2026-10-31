import type { NextApiRequest, NextApiResponse } from 'next'

describe('monitor authentication notice API', () => {
	beforeEach(() => {
		vi.resetModules()
		vi.stubEnv('PORT', '18080')
		vi.spyOn(console, 'info').mockImplementation(() => undefined)
		vi.spyOn(Date, 'now').mockReturnValue(100_000)
	})
	afterEach(() => {
		vi.unstubAllEnvs()
		vi.restoreAllMocks()
	})

	const response = () => {
		const res = {
			setHeader: vi.fn(),
			status: vi.fn().mockReturnThis(),
			end: vi.fn(),
		}
		return res
	}
	const request = (
		method = 'POST',
		headers: NextApiRequest['headers'] = {
			'x-monitor-auth-required': 'signed-out',
		},
	) => ({ method, headers }) as unknown as NextApiRequest

	test('logs only a fixed local login URL and suppresses repeated notifications for a minute', async () => {
		const { default: handler } = await import(
			'../pages/api/monitor-auth-required'
		)
		expect(console.info).not.toHaveBeenCalled()
		const res = response()
		const req = request()
		req.body = { password: 'must-not-be-logged', url: 'https://untrusted.test' }
		req.headers.host = 'untrusted.test'
		handler(req, res as unknown as NextApiResponse)
		expect(console.info).toHaveBeenCalledExactlyOnceWith(
			'[monitor] 未ログインです。ログインURL: http://localhost:18080/monitor-login',
		)
		expect(res.status).toHaveBeenCalledWith(204)
		expect(res.setHeader).toHaveBeenCalledWith('Cache-Control', 'no-store')
		vi.mocked(Date.now).mockReturnValue(159_999)
		handler(req, res as unknown as NextApiResponse)
		expect(console.info).toHaveBeenCalledTimes(1)
		vi.mocked(Date.now).mockReturnValue(160_000)
		handler(req, res as unknown as NextApiResponse)
		expect(console.info).toHaveBeenCalledTimes(2)
	})

	test('reports missing monitor privileges separately from a recent signed-out notice', async () => {
		const { default: handler } = await import(
			'../pages/api/monitor-auth-required'
		)
		const res = response()
		handler(request(), res as unknown as NextApiResponse)
		handler(
			request('POST', { 'x-monitor-auth-required': 'unauthorized' }),
			res as unknown as NextApiResponse,
		)
		expect(console.info).toHaveBeenLastCalledWith(
			'[monitor] ログイン済みですがモニター閲覧権限がありません。monitor_reader: true を確認してください。ログインURL: http://localhost:18080/monitor-login',
		)
		expect(console.info).toHaveBeenCalledTimes(2)
	})

	test.each(['GET', 'OPTIONS'])(
		'rejects %s without logging',
		async (method) => {
			const { default: handler } = await import(
				'../pages/api/monitor-auth-required'
			)
			const res = response()
			handler(request(method), res as unknown as NextApiResponse)
			expect(res.status).toHaveBeenCalledWith(405)
			expect(res.setHeader).toHaveBeenCalledWith('Allow', 'POST')
			expect(console.info).not.toHaveBeenCalled()
		},
	)

	test('rejects a POST without the browser notification header', async () => {
		const { default: handler } = await import(
			'../pages/api/monitor-auth-required'
		)
		const res = response()
		handler(request('POST', {}), res as unknown as NextApiResponse)
		expect(res.status).toHaveBeenCalledWith(403)
		expect(console.info).not.toHaveBeenCalled()
	})
})
