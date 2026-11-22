import type { SyntheticEvent } from 'react'
import { createImageReloader, MAX_IMAGE_RELOAD_ATTEMPTS } from './SeatBox'

vi.mock('next/font/google', () => ({
	M_PLUS_Rounded_1c: vi.fn(() => ({
		style: { fontFamily: 'M PLUS Rounded 1c' },
		className: 'mock-font-class',
	})),
	Source_Code_Pro: vi.fn(() => ({
		style: { fontFamily: 'mock-source-code-pro' },
		className: 'mock-source-code-pro-class',
	})),
}))

const failingImageEvent = () => {
	const image = { src: '' }
	return {
		image,
		event: { currentTarget: image } as unknown as SyntheticEvent<
			HTMLImageElement,
			Event
		>,
	}
}

describe('createImageReloader', () => {
	beforeEach(() => {
		vi.spyOn(console, 'error').mockImplementation(() => undefined)
	})

	test('retries a failing image with a cache-busting query up to the attempt cap', () => {
		const reload = createImageReloader()
		for (let attempt = 0; attempt < MAX_IMAGE_RELOAD_ATTEMPTS; attempt++) {
			const { image, event } = failingImageEvent()
			reload(event, 'https://example.com/a.png')
			expect(image.src).toMatch(/^https:\/\/example\.com\/a\.png\?\d+$/)
		}
	})

	test('stops retrying once the cap is reached', () => {
		const reload = createImageReloader()
		for (let attempt = 0; attempt < MAX_IMAGE_RELOAD_ATTEMPTS; attempt++) {
			reload(failingImageEvent().event, 'https://example.com/a.png')
		}
		const { image, event } = failingImageEvent()
		reload(event, 'https://example.com/a.png')
		expect(image.src).toBe('')
	})

	test('counts attempts per image source', () => {
		const reload = createImageReloader(1)
		reload(failingImageEvent().event, 'https://example.com/a.png')
		const { image, event } = failingImageEvent()
		reload(event, 'https://example.com/b.png')
		expect(image.src).toMatch(/^https:\/\/example\.com\/b\.png\?\d+$/)
	})
})
