import { verticalLayout } from './vertical-layout'

describe('verticalLayout', () => {
	it('fits the common grid inside the 1080x1920 canvas', () => {
		expect(verticalLayout.canvas).toEqual({ width: 1080, height: 1920 })
		expect(verticalLayout.outerPadding).toBe(24)
		expect(verticalLayout.content.width).toBe(1032)

		const rowHeight = Object.values(verticalLayout.rows).reduce(
			(total, height) => total + height,
			0,
		)
		const occupiedHeight = rowHeight + verticalLayout.gap * 4

		expect(occupiedHeight).toBe(verticalLayout.content.height)
		expect(verticalLayout.outerPadding * 2 + occupiedHeight).toBe(
			verticalLayout.canvas.height,
		)
	})

	it('keeps both two-column rows on the shared content width', () => {
		expect(
			verticalLayout.headerColumns.clock +
				verticalLayout.headerColumns.gap +
				verticalLayout.headerColumns.status,
		).toBe(verticalLayout.content.width)
		expect(
			verticalLayout.metricColumns.timer +
				verticalLayout.metricColumns.gap +
				verticalLayout.metricColumns.ticker,
		).toBe(verticalLayout.content.width)
	})
})
