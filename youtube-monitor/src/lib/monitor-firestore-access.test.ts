const mocks = vi.hoisted(() => ({
	getDocFromServer: vi.fn(),
	doc: vi.fn(() => 'constants-reference'),
}))
vi.mock('firebase/firestore', () => ({
	getFirestore: () => 'monitor-db',
	doc: mocks.doc,
	getDocFromServer: mocks.getDocFromServer,
}))
vi.mock('./firestore', () => ({ getFirebaseApp: () => ({}) }))

import { checkMonitorFirestoreAccess } from './monitor-firestore-access'

describe('monitor Firestore access check', () => {
	beforeEach(() => {
		mocks.getDocFromServer.mockReset()
		mocks.doc.mockClear()
	})

	test('checks the real server read of constants without inspecting the private allowlist or document contents', async () => {
		const data = vi.fn()
		mocks.getDocFromServer.mockResolvedValue({ exists: () => true, data })
		await expect(checkMonitorFirestoreAccess()).resolves.toBe('ready')
		expect(mocks.doc).toHaveBeenCalledExactlyOnceWith(
			'monitor-db',
			'config',
			'constants',
		)
		expect(mocks.getDocFromServer).toHaveBeenCalledExactlyOnceWith(
			'constants-reference',
		)
		expect(data).not.toHaveBeenCalled()
	})

	test('reports a missing constants document separately from access denial', async () => {
		mocks.getDocFromServer.mockResolvedValue({ exists: () => false })
		await expect(checkMonitorFirestoreAccess()).resolves.toBe(
			'firestore-config-missing',
		)
	})

	test.each([
		['permission-denied', 'firestore-denied'],
		['unavailable', 'firestore-error'],
		['deadline-exceeded', 'firestore-error'],
		[undefined, 'firestore-error'],
	])('classifies %s without returning the raw error', async (code, issue) => {
		mocks.getDocFromServer.mockRejectedValue({
			code,
			message: 'private raw error',
		})
		await expect(checkMonitorFirestoreAccess()).resolves.toBe(issue)
	})
})
