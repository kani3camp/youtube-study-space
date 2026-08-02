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
		const occupiedHeight = rowHeight + verticalLayout.sectionGap * 3

		expect(occupiedHeight).toBe(verticalLayout.content.height)
		expect(verticalLayout.outerPadding * 2 + occupiedHeight).toBe(
			verticalLayout.canvas.height,
		)
	})

	it('keeps the header and information columns on the shared width', () => {
		expect(
			verticalLayout.headerColumns.clock +
				verticalLayout.headerColumns.gap +
				verticalLayout.headerColumns.status,
		).toBe(verticalLayout.content.width)
		expect(
			verticalLayout.informationColumns.timer +
				verticalLayout.informationColumns.gap +
				verticalLayout.informationColumns.details,
		).toBe(verticalLayout.content.width)
	})

	it('fits the information detail rows and room viewport exactly', () => {
		expect(
			verticalLayout.informationRows.usage +
				verticalLayout.informationRows.gap +
				verticalLayout.informationRows.ticker,
		).toBe(verticalLayout.rows.information)
		expect(verticalLayout.roomViewport).toEqual({ width: 1032, height: 688 })
		expect(verticalLayout.rows).not.toHaveProperty('join')
	})
})
