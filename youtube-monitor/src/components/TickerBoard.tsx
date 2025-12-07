import { useTranslation } from 'next-i18next'
import type { FC } from 'react'
import Marquee from 'react-fast-marquee'
import { componentBackground } from '../styles/common.style'
import * as styles from '../styles/TickerBoard.styles'
import type { WorkNameTrend } from '../types/api'

type Props = {
	workNameTrend: WorkNameTrend
}

const TickerBoard: FC<Props> = ({ workNameTrend }) => {
	const { t } = useTranslation()

	return (
		<div css={[styles.shape, componentBackground]}>
			<div css={styles.container}>
				<Marquee
					css={styles.marquee}
					speed={85}
					pauseOnHover
					autoFill
					gradient={false}
				>
					{workNameTrend.ranking.map((r) => {
						return (
							<span css={styles.genreItem} key={`tb-${r.rank}-${r.genre}`}>
								<span css={styles.rankBadge}>
									{t('work_name_trend.trend_rank', { rank: r.rank })}
								</span>
								<span css={styles.genre}>{r.genre}</span>
								<span css={styles.count}>
									<span css={styles.peopleIcon}>👥</span>
									{t('work_name_trend.count', { value: r.count })}
								</span>
								<span css={styles.examplesWrapper}>
									{r.examples.map((e, exampleIndex) => {
										return (
											<span
												css={styles.exampleChip}
												key={`tb-${r.rank}-${exampleIndex}-${e}`}
											>
												{e}
											</span>
										)
									})}
								</span>
							</span>
						)
					})}
					<div css={styles.updatedAt}>
						{t('work_name_trend.ranked_at', {
							date: workNameTrend.ranked_at
								.toDate()
								.toLocaleDateString(undefined, {
									year: 'numeric',
									month: '2-digit',
									day: '2-digit',
								}),
							time: workNameTrend.ranked_at
								.toDate()
								.toLocaleTimeString(undefined, {
									hour: '2-digit',
									minute: '2-digit',
								}),
						})}
					</div>
					{workNameTrend.ranking.length === 0 && (
						<span css={styles.genreItem}>{t('work_name_trend.updating')}</span>
					)}
				</Marquee>
			</div>
		</div>
	)
}

export default TickerBoard
