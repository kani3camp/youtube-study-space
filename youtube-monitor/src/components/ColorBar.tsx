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
	{ hex: '#FFFFFF' }, // 0-5h
	{ hex: '#FFD4CC' }, // 5-10h
	{ hex: '#FF9580' }, // 10-20h
	{ hex: '#FFC880' }, // 20-30h
	{ hex: '#FFFB7F' }, // 30-50h
	{ hex: '#D0FF80' }, // 50-70h
	{ hex: '#9DFF7F' }, // 70-100h
	{ hex: '#80FF95' }, // 100-150h
	{ hex: '#80FFC8' }, // 150-200h
	{ hex: '#80FFFB' }, // 200-300h
	{ hex: '#80D0FF' }, // 300-400h
	{ hex: '#809EFF' }, // 400-500h
	{ hex: '#947FFF' }, // 500-700h
	{ hex: '#C880FF' }, // 700-1000h
	{ hex: '#FF7FFF' }, // 1000h+
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
