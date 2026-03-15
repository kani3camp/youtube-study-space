import { useTranslation } from 'next-i18next'
import type { FC } from 'react'
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

const COLOR_BOXES: { hex: string }[] = [
	{ hex: '#F2F1EE' }, // 0-5h
	{ hex: '#F0D6D2' }, // 5-10h
	{ hex: '#E8B4A8' }, // 10-20h
	{ hex: '#E8C09C' }, // 20-30h
	{ hex: '#E7D796' }, // 30-50h
	{ hex: '#D2DEA0' }, // 50-70h
	{ hex: '#BDD7A8' }, // 70-100h
	{ hex: '#A9D6B5' }, // 100-150h
	{ hex: '#A8D8C7' }, // 150-200h
	{ hex: '#A9D6D2' }, // 200-300h
	{ hex: '#AFCFE8' }, // 300-400h
	{ hex: '#AFC2E8' }, // 400-500h
	{ hex: '#B5B2E8' }, // 500-700h
	{ hex: '#CBB6E8' }, // 700-1000h
	{ hex: '#E7A9CF' }, // 1000h+
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
						{COLOR_BOXES.map(({ hex }) => (
							<div
								key={hex}
								css={styles.colorBox}
								style={{ backgroundColor: hex }}
							/>
						))}
					</div>
				</div>
			</div>
		</div>
	)
}

export default ColorBar
