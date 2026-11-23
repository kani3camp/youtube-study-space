import { fireEvent, render, screen } from '@testing-library/react'
import type { ComponentPropsWithoutRef } from 'react'
import BackgroundImage from './BackgroundImage'

vi.mock('next/image', () => ({
	__esModule: true,
	default: (props: ComponentPropsWithoutRef<'img'>) => {
		const { alt, onError } = props
		// biome-ignore lint/performance/noImgElement: next/image の代替としてテスト内で描画する
		return <img alt={alt} onError={onError} />
	},
}))

describe('BackgroundImage', () => {
	test('retries the fallback source only once when it keeps failing', () => {
		render(<BackgroundImage />)
		const image = screen.getByAltText('background image')
		const setSrc = vi.fn()
		Object.defineProperty(image, 'src', { set: setSrc, configurable: true })

		fireEvent.error(image)
		fireEvent.error(image)
		fireEvent.error(image)

		expect(setSrc).toHaveBeenCalledTimes(1)
	})
})
