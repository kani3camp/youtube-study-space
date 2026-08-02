export type RgbaColor = {
	r: number
	g: number
	b: number
	a: number
}

const clamp = (value: number, min: number, max: number) =>
	Math.min(max, Math.max(min, value))

const linearizeSrgbChannel = (channel: number): number => {
	const srgb = clamp(channel, 0, 255) / 255
	return srgb <= 0.04045 ? srgb / 12.92 : ((srgb + 0.055) / 1.055) ** 2.4
}

/** WCAG 2.xのsRGB相対輝度を返します。alphaは合成後に評価してください。 */
export function getRelativeLuminance(color: RgbaColor): number {
	return (
		0.2126 * linearizeSrgbChannel(color.r) +
		0.7152 * linearizeSrgbChannel(color.g) +
		0.0722 * linearizeSrgbChannel(color.b)
	)
}

/** 2つの不透明色のWCAGコントラスト比を返します。 */
export function getContrastRatio(
	foreground: RgbaColor,
	background: RgbaColor,
): number {
	const foregroundLuminance = getRelativeLuminance(foreground)
	const backgroundLuminance = getRelativeLuminance(background)
	const lighter = Math.max(foregroundLuminance, backgroundLuminance)
	const darker = Math.min(foregroundLuminance, backgroundLuminance)
	return (lighter + 0.05) / (darker + 0.05)
}

/** foregroundをbackgroundへ通常のsource-overでalpha合成します。 */
export function compositeColors(
	foreground: RgbaColor,
	background: RgbaColor,
): RgbaColor {
	const alpha = foreground.a + background.a * (1 - foreground.a)
	if (alpha === 0) {
		return { r: 0, g: 0, b: 0, a: 0 }
	}

	return {
		r:
			(foreground.r * foreground.a +
				background.r * background.a * (1 - foreground.a)) /
			alpha,
		g:
			(foreground.g * foreground.a +
				background.g * background.a * (1 - foreground.a)) /
			alpha,
		b:
			(foreground.b * foreground.a +
				background.b * background.a * (1 - foreground.a)) /
			alpha,
		a: alpha,
	}
}

/**
 * CSS Colorのlegacy sRGB補間に合わせ、premultiplied alphaで近似補間します。
 */
export function interpolateColors(
	from: RgbaColor,
	to: RgbaColor,
	progress: number,
): RgbaColor {
	const ratio = clamp(progress, 0, 1)
	const alpha = from.a + (to.a - from.a) * ratio
	if (alpha === 0) {
		return { r: 0, g: 0, b: 0, a: 0 }
	}

	const interpolatePremultipliedChannel = (
		fromChannel: number,
		toChannel: number,
	) => (fromChannel * from.a * (1 - ratio) + toChannel * to.a * ratio) / alpha

	return {
		r: interpolatePremultipliedChannel(from.r, to.r),
		g: interpolatePremultipliedChannel(from.g, to.g),
		b: interpolatePremultipliedChannel(from.b, to.b),
		a: alpha,
	}
}

export const rgba = (r: number, g: number, b: number, a = 1): RgbaColor => ({
	r,
	g,
	b,
	a,
})

export const toCssRgba = ({ r, g, b, a }: RgbaColor): string =>
	`rgba(${r}, ${g}, ${b}, ${a})`
