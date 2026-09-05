import { useTranslation } from 'next-i18next/pages'
import type { CSSProperties, FC } from 'react'
import Marquee from 'react-fast-marquee'
import { componentBackground } from '../styles/common.style'
import * as styles from '../styles/TickerBoard.styles'
import type { WorkNameTrend } from '../types/api'
import type { MonitorVariant } from './monitor/types'

type Props = {
	workNameTrend: WorkNameTrend
	variant?: MonitorVariant
	style?: CSSProperties
}

const TickerBoard: FC<Props> = ({
	workNameTrend,
	variant = 'horizontal',
	style,
}) => {
	const { t } = useTranslation()
	const isVertical = variant === 'vertical'
	const rankingItems = workNameTrend.ranking.map((ranking) => {
		const exampleKeyCounts = new Map<string, number>()

		return (
			<span
				css={[styles.genreItem, isVertical && styles.verticalGenreItem]}
				key={`tb-${ranking.rank}-${ranking.genre}`}
			>
				<span css={styles.rankBadge}>
					{t('work_name_trend.trend_rank', { rank: ranking.rank })}
				</span>
				<span css={styles.genre}>{ranking.genre}</span>
				<span css={styles.count}>
					<span css={styles.peopleIcon}>👥</span>
					{t('work_name_trend.count', { value: ranking.count })}
				</span>
				<span css={styles.examplesWrapper}>
					{ranking.examples.map((example) => {
						const seenCount = exampleKeyCounts.get(example) ?? 0
						exampleKeyCounts.set(example, seenCount + 1)

						return (
							<span
								css={styles.exampleChip}
								key={`tb-${ranking.rank}-${example}-${seenCount}`}
							>
								{example}
							</span>
						)
					})}
				</span>
			</span>
		)
	})
	const updatedAt = (
		<div css={[styles.updatedAt, isVertical && styles.verticalUpdatedAt]}>
			{t('work_name_trend.ranked_at', {
				date: workNameTrend.ranked_at.toDate().toLocaleDateString(undefined, {
					year: 'numeric',
					month: '2-digit',
					day: '2-digit',
				}),
				time: workNameTrend.ranked_at.toDate().toLocaleTimeString(undefined, {
					hour: '2-digit',
					minute: '2-digit',
				}),
			})}
		</div>
	)

	return (
		<div
			css={[
				styles.shape,
				isVertical && styles.verticalShape,
				!isVertical && componentBackground,
			]}
			style={style}
		>
			<div css={[styles.container, isVertical && styles.verticalContainer]}>
				{isVertical ? (
					<>
						<div css={styles.verticalMarqueeViewport}>
							{workNameTrend.ranking.length > 0 ? (
								<Marquee
									css={[styles.marquee, styles.verticalMarquee]}
									speed={60}
									pauseOnHover
									autoFill
									gradient={false}
								>
									{rankingItems}
								</Marquee>
							) : (
								<div css={styles.verticalEmptyState}>
									{t('work_name_trend.updating')}
								</div>
							)}
						</div>
						{updatedAt}
					</>
				) : (
					<Marquee
						css={styles.marquee}
						speed={85}
						pauseOnHover
						autoFill
						gradient={false}
					>
						{rankingItems}
						{updatedAt}
						{workNameTrend.ranking.length === 0 && (
							<span css={styles.genreItem}>
								{t('work_name_trend.updating')}
							</span>
						)}
					</Marquee>
				)}
			</div>
		</div>
	)
}

export default TickerBoard
