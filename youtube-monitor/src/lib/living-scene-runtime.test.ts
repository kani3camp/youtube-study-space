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

const createActivation = (
	host: HTMLElement,
): {
	host: HTMLElement
	width: number
	height: number
	profile: 'lume-rainy-poc'
} => ({
	host,
	width: 1520,
	height: 900,
	profile: 'lume-rainy-poc',
})

describe('LivingSceneRuntimeManager', () => {
	test('reuses one application while moving the canvas between active room hosts', async () => {
		const application = createFakeApplication()
		const createApplication: jest.MockedFunction<CreateLivingSceneApplication> =
			jest.fn().mockResolvedValue(application)
		const manager = new LivingSceneRuntimeManager(createApplication)
		const firstHost = document.createElement('div')
		const secondHost = document.createElement('div')

		await expect(manager.activate(createActivation(firstHost))).resolves.toBe(
			true,
		)
		expect(createApplication).toHaveBeenCalledTimes(1)
		expect(application.setProfile).toHaveBeenLastCalledWith('lume-rainy-poc')
		expect(firstHost).toContainElement(application.canvas)
		expect(application.resize).toHaveBeenLastCalledWith(1520, 900)
		expect(application.start).toHaveBeenCalledTimes(1)

		manager.deactivate(firstHost)
		expect(application.stop).toHaveBeenCalledTimes(1)

		await expect(
			manager.activate({
				...createActivation(secondHost),
				width: 1200,
				height: 800,
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

		const activation = manager.activate(createActivation(host))
		manager.deactivate(host)
		resolveApplication?.(application)

		await expect(activation).resolves.toBe(false)
		expect(host).not.toContainElement(application.canvas)
		expect(application.start).not.toHaveBeenCalled()
	})

	test('retries a transient initialization failure on the next activation', async () => {
		const application = createFakeApplication()
		const createApplication: jest.MockedFunction<CreateLivingSceneApplication> =
			jest
				.fn()
				.mockRejectedValueOnce(
					new Error('temporary WebGL initialization error'),
				)
				.mockResolvedValue(application)
		const manager = new LivingSceneRuntimeManager(createApplication)
		const host = document.createElement('div')
		const consoleError = jest
			.spyOn(console, 'error')
			.mockImplementation(() => undefined)

		await expect(manager.activate(createActivation(host))).resolves.toBe(false)
		await expect(manager.activate(createActivation(host))).resolves.toBe(true)

		expect(createApplication).toHaveBeenCalledTimes(2)
		expect(host).toContainElement(application.canvas)
		consoleError.mockRestore()
	})

	test('keeps the static fallback when a profile cannot be configured', async () => {
		const application = createFakeApplication()
		application.setProfile.mockReturnValue(false)
		const manager = new LivingSceneRuntimeManager(() =>
			Promise.resolve(application),
		)
		const host = document.createElement('div')

		await expect(manager.activate(createActivation(host))).resolves.toBe(false)
		expect(host).not.toContainElement(application.canvas)
		expect(application.start).not.toHaveBeenCalled()
		expect(application.canvas.hidden).toBe(true)
	})

	test('restores the active scene after WebGL context recovery', async () => {
		const application = createFakeApplication()
		const manager = new LivingSceneRuntimeManager(() =>
			Promise.resolve(application),
		)
		const host = document.createElement('div')
		document.body.appendChild(host)

		await manager.activate(createActivation(host))
		application.canvas.dispatchEvent(
			new Event('webglcontextlost', { cancelable: true }),
		)
		expect(application.canvas.hidden).toBe(true)
		expect(application.stop).toHaveBeenCalled()

		application.canvas.dispatchEvent(new Event('webglcontextrestored'))
		await Promise.resolve()
		await Promise.resolve()

		expect(application.canvas.hidden).toBe(false)
		expect(application.start).toHaveBeenCalledTimes(2)
		host.remove()
	})

	test('does not restart a scene hidden before WebGL context recovery', async () => {
		const application = createFakeApplication()
		const manager = new LivingSceneRuntimeManager(() =>
			Promise.resolve(application),
		)
		const host = document.createElement('div')
		document.body.appendChild(host)

		await manager.activate(createActivation(host))
		application.canvas.dispatchEvent(
			new Event('webglcontextlost', { cancelable: true }),
		)
		manager.deactivate(host)
		application.canvas.dispatchEvent(new Event('webglcontextrestored'))
		await Promise.resolve()
		await Promise.resolve()

		expect(application.canvas.hidden).toBe(true)
		expect(application.start).toHaveBeenCalledTimes(1)
		host.remove()
	})

	test('does not restart a detached host after WebGL context recovery', async () => {
		const application = createFakeApplication()
		const manager = new LivingSceneRuntimeManager(() =>
			Promise.resolve(application),
		)
		const host = document.createElement('div')
		document.body.appendChild(host)

		await manager.activate(createActivation(host))
		application.canvas.dispatchEvent(
			new Event('webglcontextlost', { cancelable: true }),
		)
		host.remove()
		application.canvas.dispatchEvent(new Event('webglcontextrestored'))
		await Promise.resolve()
		await Promise.resolve()

		expect(application.canvas.hidden).toBe(true)
		expect(application.start).toHaveBeenCalledTimes(1)
	})

	test('destroy releases the shared application and prevents later recovery', async () => {
		const application = createFakeApplication()
		const manager = new LivingSceneRuntimeManager(() =>
			Promise.resolve(application),
		)
		const host = document.createElement('div')
		document.body.appendChild(host)

		await manager.activate(createActivation(host))
		application.canvas.dispatchEvent(
			new Event('webglcontextlost', { cancelable: true }),
		)
		manager.destroy()
		application.canvas.dispatchEvent(new Event('webglcontextrestored'))
		await Promise.resolve()

		expect(application.destroy).toHaveBeenCalledTimes(1)
		expect(application.start).toHaveBeenCalledTimes(1)
		host.remove()
	})
})
