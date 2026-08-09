import {
	getVerticalRoomViewport,
	getVerticalSectionPositions,
	verticalLayout,
} from './vertical-layout'

describe('verticalLayout', () => {
	it('reserves a 180px YouTube safe area before the full-width room', () => {
		expect(verticalLayout.canvas).toEqual({ width: 1080, height: 1920 })
		expect(verticalLayout.topSafeArea.height).toBe(180)
		expect(verticalLayout.room.width).toBe(1080)
	})

	it('preserves the source aspect ratio at the full canvas width', () => {
		const source = { width: 1520, height: 1000 }
		const viewport = getVerticalRoomViewport(source)

		expect(viewport.width).toBe(1080)
		expect(viewport.height).toBeCloseTo((1000 / 1520) * 1080)
		expect(viewport.width / viewport.height).toBeCloseTo(
			source.width / source.height,
		)
	})

	it('places status, timer, and commands after the actual room bottom', () => {
		const roomHeight = getVerticalRoomViewport({
			width: 1520,
			height: 1000,
		}).height
		const positions = getVerticalSectionPositions(roomHeight)

		expect(positions.room.y).toBe(180)
		expect(positions.status.y).toBeCloseTo(
			positions.room.y + roomHeight + verticalLayout.content.gap,
		)
		expect(positions.timer.y).toBeCloseTo(
			positions.status.y +
				verticalLayout.status.height +
				verticalLayout.content.gap,
		)
		expect(positions.commands.y).toBeCloseTo(
			positions.timer.y +
				verticalLayout.timer.height +
				verticalLayout.content.gap,
		)
		expect(positions.importantContentBottom).toBeLessThan(
			verticalLayout.canvas.height,
		)
	})

	it('moves every section below the room when the room height changes', () => {
		const shortRoom = getVerticalSectionPositions(600)
		const tallRoom = getVerticalSectionPositions(720)

		expect(tallRoom.status.y - shortRoom.status.y).toBe(120)
		expect(tallRoom.timer.y - shortRoom.timer.y).toBe(120)
		expect(tallRoom.commands.y - shortRoom.commands.y).toBe(120)
	})

	it('uses the compact lower stack dimensions', () => {
		expect(verticalLayout.content).toEqual({
			horizontalPadding: 20,
			width: 1040,
			gap: 16,
		})
		expect(verticalLayout.status.height).toBe(64)
		expect(verticalLayout.timer.height).toBe(100)
		expect(verticalLayout.commands.height).toBe(80)
	})
})
