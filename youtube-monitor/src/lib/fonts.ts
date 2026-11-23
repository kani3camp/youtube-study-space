import { M_PLUS_Rounded_1c, Source_Code_Pro } from 'next/font/google'

function normalizeFontFamily(fontFamilyValue: string): string {
	return fontFamilyValue
		.split(',')
		.map((familyName) => familyName.trim())
		.filter((familyName) => familyName !== '')
		.map((familyName) => {
			const firstChar = familyName[0]
			const lastChar = familyName[familyName.length - 1]
			if ((firstChar === "'" || firstChar === '"') && lastChar === firstChar) {
				return familyName
			}
			return `"${familyName.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`
		})
		.join(', ')
}

const mPlusRounded1c = M_PLUS_Rounded_1c({
	subsets: ['latin'],
	weight: ['100', '300', '400', '500', '700', '800', '900'],
	display: 'swap',
	adjustFontFallback: false,
})
export const fontFamily = normalizeFontFamily(mPlusRounded1c.style.fontFamily)
/** Next.js 15ではフォントを有効にするため、ルート要素にこの className を付与する必要がある */
export const fontClassName = mPlusRounded1c.className

const sourceCodePro = Source_Code_Pro({
	subsets: ['latin'],
	weight: ['200', '300', '400', '500', '600', '700', '800', '900'],
	display: 'swap',
	adjustFontFallback: false,
})
export const sourceCodeProFontFamily = normalizeFontFamily(
	sourceCodePro.style.fontFamily,
)
export const sourceCodeProClassName = sourceCodePro.className
