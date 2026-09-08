export type LivingSceneApplication = {
	canvas: HTMLCanvasElement
	start: () => void
	stop: () => void
	resize: (width: number, height: number) => void
	destroy: () => void
}

export type CreateLivingSceneApplication = (
	width: number,
	height: number,
) => Promise<LivingSceneApplication>

export type LivingSceneActivation = {
	host: HTMLElement
	width: number
	height: number
}

async function createPixiLivingSceneApplication(
	width: number,
	height: number,
): Promise<LivingSceneApplication> {
	const { Application } = await import('pixi.js')
	const app = new Application()

	await app.init({
		width,
		height,
		autoStart: false,
		sharedTicker: false,
		backgroundAlpha: 0,
		antialias: false,
		autoDensity: false,
		resolution: 1,
		preference: 'webgl',
		preferWebGLVersion: 2,
		powerPreference: 'high-performance',
	})

	const canvas = app.canvas as HTMLCanvasElement
	canvas.setAttribute('aria-hidden', 'true')
	canvas.style.position = 'absolute'
	canvas.style.inset = '0'
	canvas.style.width = '100%'
	canvas.style.height = '100%'
	canvas.style.pointerEvents = 'none'

	return {
		canvas,
		start: () => app.start(),
		stop: () => app.stop(),
		resize: (nextWidth, nextHeight) =>
			app.renderer.resize(nextWidth, nextHeight),
		destroy: () => app.destroy({ removeView: true }, true),
	}
}

export class LivingSceneRuntimeManager {
	private application: LivingSceneApplication | undefined
	private applicationPromise: Promise<LivingSceneApplication> | undefined
	private requestedHost: HTMLElement | undefined
	private activeHost: HTMLElement | undefined
	private activationRevision = 0
	private unavailable = false

	constructor(
		private readonly createApplication: CreateLivingSceneApplication = (
			width,
			height,
		) => createPixiLivingSceneApplication(width, height),
	) {}

	async activate({
		host,
		width,
		height,
	}: LivingSceneActivation): Promise<boolean> {
		const revision = ++this.activationRevision
		this.requestedHost = host

		if (this.unavailable) {
			return false
		}

		let application: LivingSceneApplication
		try {
			application = await this.ensureApplication(width, height)
		} catch (error) {
			this.unavailable = true
			console.error('[living-scene] failed to initialize PixiJS', error)
			return false
		}

		if (
			revision !== this.activationRevision ||
			this.requestedHost !== host ||
			this.unavailable
		) {
			return false
		}

		if (this.activeHost !== undefined && this.activeHost !== host) {
			application.stop()
		}

		application.resize(width, height)
		if (application.canvas.parentElement !== host) {
			host.appendChild(application.canvas)
		}
		application.canvas.hidden = false
		this.activeHost = host
		application.start()
		return true
	}

	deactivate(host: HTMLElement): void {
		if (this.requestedHost === host) {
			this.requestedHost = undefined
			this.activationRevision++
		}

		if (this.activeHost !== host) {
			return
		}

		this.application?.stop()
		this.activeHost = undefined
	}

	destroy(): void {
		this.activationRevision++
		this.requestedHost = undefined
		this.activeHost = undefined
		this.unavailable = false

		const application = this.application
		this.application = undefined
		this.applicationPromise = undefined
		if (application === undefined) {
			return
		}

		application.canvas.removeEventListener(
			'webglcontextlost',
			this.handleContextLost,
		)
		application.stop()
		application.destroy()
	}

	private async ensureApplication(
		width: number,
		height: number,
	): Promise<LivingSceneApplication> {
		if (this.application !== undefined) {
			return this.application
		}

		if (this.applicationPromise === undefined) {
			this.applicationPromise = this.createApplication(width, height)
		}

		const application = await this.applicationPromise
		if (this.application === undefined) {
			this.application = application
			application.canvas.addEventListener(
				'webglcontextlost',
				this.handleContextLost,
			)
		}
		return this.application
	}

	private readonly handleContextLost = (event: Event): void => {
		event.preventDefault()
		this.activationRevision++
		this.requestedHost = undefined
		this.activeHost = undefined
		this.unavailable = true

		if (this.application !== undefined) {
			this.application.stop()
			this.application.canvas.hidden = true
		}
	}
}

export const livingSceneRuntime = new LivingSceneRuntimeManager()
