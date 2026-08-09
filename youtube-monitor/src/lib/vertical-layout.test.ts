import { verticalLayout } from './vertical-layout'

describe('verticalLayout', () => {
	it('keeps the room full width at the top of the 1080x1920 canvas', () => {
		expect(verticalLayout.canvas).toEqual({ width: 1080, height: 1920 })
		expect(verticalLayout.room).toEqual({
			x: 0,
			y: 0,
			width: 1080,
			height: 720,
		})
		expect(verticalLayout.roomViewport).toEqual({ width: 1080, height: 720 })
	})

	it('keeps every important element above the chat-safe boundary', () => {
		expect(
			verticalLayout.timer.y + verticalLayout.timer.height,
		).toBeLessThanOrEqual(verticalLayout.importantContentBottom)
		expect(verticalLayout.usage.y + verticalLayout.usage.height).toBe(
			verticalLayout.importantContentBottom,
		)
	})

	it('keeps the HUD and lower cards inside the canvas', () => {
		const rectangles = [
			verticalLayout.hud.clock,
			verticalLayout.hud.page,
			verticalLayout.hud.member,
			verticalLayout.hud.workers,
			verticalLayout.timer,
			verticalLayout.usage,
		]

		for (const rectangle of rectangles) {
			expect(rectangle.x).toBeGreaterThanOrEqual(0)
			expect(rectangle.y).toBeGreaterThanOrEqual(0)
			expect(rectangle.x + rectangle.width).toBeLessThanOrEqual(
				verticalLayout.canvas.width,
			)
			expect(rectangle.y + rectangle.height).toBeLessThanOrEqual(
				verticalLayout.canvas.height,
			)
		}
	})
})
