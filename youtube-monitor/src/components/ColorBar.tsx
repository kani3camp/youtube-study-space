import type { FC } from 'react'
import { useTranslation } from 'next-i18next'
import * as styles from '../styles/ColorBar.styles'
import { componentBackground, componentStyle } from '../styles/common.style'

/** 目盛りの値と、15分割中の位置（0〜14）。leftPercent = position * 100 / 15 */
const SCALE_VALUES = [0, 20, 70, 200, 500, 1000] as const
const SCALE_POSITIONS = [0, 3, 6, 9, 12, 14] as const
const SEGMENTS = 15

const SCALE_LABELS = SCALE_VALUES.map((value, i) => ({
	value,
	leftPercent: (100 * SCALE_POSITIONS[i]) / SEGMENTS,
}))

const COLOR_BOXES: { hex: string; title: string }[] = [
	{ hex: '#F2F1EE', title: '0〜5時間' },
	{ hex: '#F0D6D2', title: '5〜10時間' },
	{ hex: '#E8B4A8', title: '10〜20時間' },
	{ hex: '#E8C09C', title: '20〜30時間' },
	{ hex: '#E7D796', title: '30〜50時間' },
	{ hex: '#D2DEA0', title: '50〜70時間' },
	{ hex: '#BDD7A8', title: '70〜100時間' },
	{ hex: '#A9D6B5', title: '100〜150時間' },
	{ hex: '#A8D8C7', title: '150〜200時間' },
	{ hex: '#A9D6D2', title: '200〜300時間' },
	{ hex: '#AFCFE8', title: '300〜400時間' },
	{ hex: '#AFC2E8', title: '400〜500時間' },
	{ hex: '#B5B2E8', title: '500〜700時間' },
	{ hex: '#CBB6E8', title: '700〜1000時間' },
	{ hex: '#E7A9CF', title: '1000時間〜' },
]

const ColorBar: FC = () => {
	const { t } = useTranslation()

	return (
		<div css={[styles.shape, componentBackground]}>
			<div css={[styles.colorBar, componentStyle]}>
				<div css={styles.title}>{t('colorBar.title')}</div>
				<div css={styles.scaleWrapper}>
					<div css={styles.labels}>
						{SCALE_LABELS.map(({ value, leftPercent }) => (
							<div
								key={value}
								css={[styles.label, styles.labelPosition(leftPercent)]}
							>
								{value}h
							</div>
						))}
					</div>
					<div css={styles.colorBarStrip}>
						{COLOR_BOXES.map(({ hex, title }) => (
							<div
								key={hex}
								css={styles.colorBox}
								style={{ backgroundColor: hex }}
								title={title}
							/>
						))}
					</div>
				</div>
			</div>
		</div>
	)
}

export default ColorBar
