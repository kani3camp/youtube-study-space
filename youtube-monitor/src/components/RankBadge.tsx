/** @jsxImportSource @emotion/react */
import type { FC } from 'react'
import { LuCrown } from 'react-icons/lu'
import { getRankAppearance } from '../lib/rank-appearance'
import * as styles from '../styles/SeatBox.styles'

type RankBadgeProps = {
	rank: number
	fontSizePx: number
}

const RankBadge: FC<RankBadgeProps> = ({ rank, fontSizePx }) => {
	const appearance = getRankAppearance(rank)
	if (appearance === undefined) {
		return null
	}

	return (
		<div
			aria-label={`ランク R${rank.toString()}`}
			role="img"
			css={styles.rankBadge}
			style={{
				color: appearance.base,
				backgroundColor: appearance.background,
				borderColor: appearance.outline,
				fontSize: `${fontSizePx}px`,
			}}
		>
			<LuCrown aria-hidden="true" css={styles.rankBadgeCrown} />
			<span>{`R${rank.toString()}`}</span>
		</div>
	)
}

export default RankBadge
