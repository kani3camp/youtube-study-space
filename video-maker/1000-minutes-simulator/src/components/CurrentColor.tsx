import { css } from '@emotion/react'
import { useTranslation } from 'next-i18next'
import type { FC } from 'react'
import { MdInvertColors } from 'react-icons/md'
import { type Rank, ranks } from '../lib/ranks'
import * as styles from '../styles/CurrentColor.styles'
import * as common from '../styles/common.styles'

type Props = {
	elapsedMinutes: number
}

const CurrentColor: FC<Props> = (props) => {
	const { t } = useTranslation()

	let currentColorCode = 'inherit'
	ranks.forEach((rank: Rank) => {
		if (
			rank.FromHours <= props.elapsedMinutes &&
			props.elapsedMinutes < rank.ToHours
		) {
			currentColorCode = rank.ColorCode
		}
	})

	return (
		<div css={styles.currentColor}>
			<div css={styles.innerCell}>
				<div css={common.heading}>
					<MdInvertColors size={common.IconSize} css={styles.icon} />
					<span>{t('color.title')}</span>
				</div>
				<div
					css={css`
						${styles.colorBox};
						background-color: ${currentColorCode};
						box-shadow: 0 0 50px ${currentColorCode};
					`}
				/>
				<div css={styles.annotation}>
					{/* 値は時間ではなく分を使うことに注意 */}
					{t('color.annotation_1', { value: props.elapsedMinutes })}
					<br />
					{t('color.annotation_2')}
				</div>
			</div>
		</div>
	)
}

export default CurrentColor
