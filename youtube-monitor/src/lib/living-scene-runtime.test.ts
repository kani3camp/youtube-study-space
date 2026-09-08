import {
	type CreateLivingSceneApplication,
	type LivingSceneApplication,
	LivingSceneRuntimeManager,
} from './living-scene-runtime'

function createFakeApplication(): LivingSceneApplication & {
	start: jest.Mock
	stop: jest.Mock
	resize: jest.Mock
	setProfile: jest.Mock
	destroy: jest.Mock
} {
	return {
		canvas: document.createElement('canvas'),
		start: jest.fn(),
		stop: jest.fn(),
		resize: jest.fn(),
		setProfile: jest.fn(() => true),
		destroy: jest.fn(),
	}
}

describe('LivingSceneRuntimeManager', () => {
	test('reuses one application while moving the canvas between active room hosts', async () => {
		const application = createFakeApplication()
		const createApplication: jest.MockedFunction<CreateLivingSceneApplication> =
			jest.fn().mockResolvedValue(application)
		const manager = new LivingSceneRuntimeManager(createApplication)
		const firstHost = document.createElement('div')
		const secondHost = document.createElement('div')

		await expect(
			manager.activate({
				host: firstHost,
				width: 1520,
				height: 900,
				profile: 'lume-rainy-poc',
			}),
		).resolves.toBe(true)
		expect(createApplication).toHaveBeenCalledTimes(1)
		expect(application.setProfile).toHaveBeenLastCalledWith('lume-rainy-poc')
		expect(firstHost).toContainElement(application.canvas)
		expect(application.resize).toHaveBeenLastCalledWith(1520, 900)
		expect(application.start).toHaveBeenCalledTimes(1)

		manager.deactivate(firstHost)
		expect(application.stop).toHaveBeenCalledTimes(1)

		await expect(
			manager.activate({
				host: secondHost,
				width: 1200,
				height: 800,
				profile: 'lume-rainy-poc',
			}),
		).resolves.toBe(true)
		expect(createApplication).toHaveBeenCalledTimes(1)
		expect(secondHost).toContainElement(application.canvas)
		expect(application.resize).toHaveBeenLastCalledWith(1200, 800)
		expect(application.start).toHaveBeenCalledTimes(2)
	})

	test('does not attach an async initialization result after the room was hidden', async () => {
		const application = createFakeApplication()
		let resolveApplication:
			| ((value: LivingSceneApplication) => void)
			| undefined
		const createApplication: CreateLivingSceneApplication = () =>
			new Promise((resolve) => {
				resolveApplication = resolve
			})
		const manager = new LivingSceneRuntimeManager(createApplication)
		const host = document.createElement('div')

		const activation = manager.activate({
			host,
			width: 1520,
			height: 900,
			profile: 'lume-rainy-poc',
		})
		manager.deactivate(host)
		resolveApplication?.(application)

		await expect(activation).resolves.toBe(false)
		expect(host).not.toContainElement(application.canvas)
		expect(application.start).not.toHaveBeenCalled()
	})

	test('keeps the static fallback when a profile cannot be configured', async () => {
		const application = createFakeApplication()
		application.setProfile.mockReturnValue(false)
		const manager = new LivingSceneRuntimeManager(() =>
			Promise.resolve(application),
		)
		const host = document.createElement('div')

		await expect(
			manager.activate({
				host,
				width: 1520,
				height: 900,
				profile: 'lume-rainy-poc',
			}),
		).resolves.toBe(false)
		expect(host).not.toContainElement(application.canvas)
		expect(application.start).not.toHaveBeenCalled()
		expect(application.canvas.hidden).toBe(true)
	})

	test('falls back after WebGL context loss', async () => {
		const application = createFakeApplication()
		const manager = new LivingSceneRuntimeManager(() =>
			Promise.resolve(application),
		)
		const host = document.createElement('div')

		await manager.activate({
			host,
			width: 1520,
			height: 900,
			profile: 'lume-rainy-poc',
		})
		application.canvas.dispatchEvent(
			new Event('webglcontextlost', { cancelable: true }),
		)

		expect(application.canvas.hidden).toBe(true)
		expect(application.stop).toHaveBeenCalled()
		await expect(
			manager.activate({
				host,
				width: 1520,
				height: 900,
				profile: 'lume-rainy-poc',
			}),
		).resolves.toBe(false)
	})

	test('destroy releases the shared application', async () => {
		const application = createFakeApplication()
		const manager = new LivingSceneRuntimeManager(() =>
			Promise.resolve(application),
		)
		const host = document.createElement('div')

		await manager.activate({
			host,
			width: 1520,
			height: 900,
			profile: 'lume-rainy-poc',
		})
		manager.destroy()

		expect(application.stop).toHaveBeenCalled()
		expect(application.destroy).toHaveBeenCalledTimes(1)
	})
})
