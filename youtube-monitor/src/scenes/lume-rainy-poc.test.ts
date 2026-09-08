import { createLumeRainDropSpecs } from './lume-rainy-poc'

describe('Lume rainy Living Scene', () => {
	test('uses a small deterministic rain field', () => {
		const first = createLumeRainDropSpecs()
		const second = createLumeRainDropSpecs()

		expect(first).toEqual(second)
		expect(first).toHaveLength(24)
	})

	test('keeps rain motion slow and visually sparse', () => {
		for (const drop of createLumeRainDropSpecs()) {
			expect(drop.x).toBeGreaterThanOrEqual(0)
			expect(drop.x).toBeLessThanOrEqual(1520)
			expect(drop.y).toBeGreaterThanOrEqual(0)
			expect(drop.y).toBeLessThanOrEqual(1000)
			expect(drop.length).toBeGreaterThanOrEqual(18)
			expect(drop.length).toBeLessThanOrEqual(34)
			expect(drop.speed).toBeGreaterThanOrEqual(26)
			expect(drop.speed).toBeLessThanOrEqual(46)
			expect(drop.alpha).toBeGreaterThanOrEqual(0.1)
			expect(drop.alpha).toBeLessThanOrEqual(0.21)
		}
	})
})
