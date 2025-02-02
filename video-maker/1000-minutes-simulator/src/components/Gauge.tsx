import { css } from '@emotion/react'
import { useTranslation } from 'next-i18next'
import type { FC } from 'react'
import { AiFillTrophy } from 'react-icons/ai'
import { type Rank, ranks } from '../lib/ranks'
import * as styles from '../styles/Gauge.styles'
import * as common from '../styles/common.styles'

const BarHeight = 860
const BarWidth = 70
const ArrowFontSize = 60

type Props = {
	elapsedMinutes: number
}

const Gauge: FC<Props> = (props) => {
	const { t } = useTranslation()

	// ゲージ生成
	const oneRankHeight = BarHeight / ranks.length

	// 現在到達点の矢印
	let arrow: JSX.Element = <></>
	ranks.forEach((r, i) => {
		if (r.FromHours <= props.elapsedMinutes && props.elapsedMinutes < r.ToHours) {
			// 矢印の位置が決定
			const thatRankRange = r.ToHours - r.FromHours
			const heightRatio = (props.elapsedMinutes - r.FromHours) / thatRankRange
			const arrowPositionBottom: number = i * oneRankHeight + oneRankHeight * heightRatio
			arrow = (
				<div
					css={css`
						position: absolute;
						font-weight: bold;
						right: calc(${BarWidth}px + 3rem);
						bottom: calc(
							${arrowPositionBottom}px - ${ArrowFontSize / 2}px
						);
						color: red;
						font-size: ${ArrowFontSize}px;
						line-height: ${ArrowFontSize}px;
					`}
				>
					{'▶'}
				</div>
			)
		}
	})

	return (
		<div css={styles.gauge}>
			<div css={styles.innerCell}>
				<div css={common.heading}>
					<AiFillTrophy size={common.IconSize} css={styles.icon} />
					<span>{t('gauge.title')}</span>
				</div>
				<div
					css={css`
						display: flex;
						flex-direction: column-reverse;
						width: ${BarWidth}px;
						position: absolute;
						right: 2rem;
						border: solid #292a4b8f 0.08rem;
					`}
				>
					{arrow}
					{ranks.map((e: Rank, i) => {
						return (
							<div
								key={e.FromHours}
								css={css`
									background-color: ${e.ColorCode};
									height: ${oneRankHeight}px;
									position: relative;
									font-size: 30px;
									${
										e.ToHours !== Number.POSITIVE_INFINITY &&
										css`
											border-top: solid black 0.05rem;
										`
									}
								`}
							>
								<div
									css={css`
										position: absolute;
										bottom: -0.5rem;
										line-height: 1rem;
										right: calc(${BarWidth}px + 0.3rem);
									`}
								>
									{e.FromHours}
								</div>
							</div>
						)
					})}
				</div>
				<div css={styles.unit}>{t('gauge.unit')}</div>
			</div>
		</div>
	)
}

export default Gauge
