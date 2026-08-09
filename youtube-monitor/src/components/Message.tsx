import { useTranslation } from 'next-i18next/pages'
import type { CSSProperties, FC } from 'react'
import { componentBackground, componentStyle } from '../styles/common.style'
import * as styles from '../styles/Message.styles'
import type { Seat } from '../types/api'
import type { MonitorVariant } from './monitor/types'

type Props = {
	currentPageIndex: number
	currentPagesLength: number
	currentPageIsMember: boolean
	seats: Seat[]
	variant?: MonitorVariant
	style?: CSSProperties
}

const Message: FC<Props> = (props) => {
	const { t } = useTranslation()
	const isVertical = props.variant === 'vertical'

	let content = <></>
	if (props.seats) {
		const numWorkers = props.seats.length
		content = (
			<>
				<div css={[styles.pageInfo, isVertical && styles.verticalPageInfo]}>
					<div css={[styles.pageIndex, isVertical && styles.verticalPageIndex]}>
						{isVertical ? (
							<>
								<span css={styles.verticalPageLabel}>
									{t('message.page_label')}
								</span>
								<strong css={styles.verticalPageNumber}>
									{props.currentPageIndex + 1} / {props.currentPagesLength}
								</strong>
							</>
						) : (
							t('message.room', {
								index: props.currentPageIndex + 1,
								length: props.currentPagesLength,
							})
						)}
					</div>
					{props.currentPageIsMember && (
						<div
							css={[styles.memberOnly, isVertical && styles.verticalMemberOnly]}
						>
							{t('member')}
						</div>
					)}
				</div>
				<div
					css={[
						styles.numStudyingPeople,
						isVertical && styles.verticalNumStudyingPeople,
					]}
				>
					{t('message.num_studying_people', { value: numWorkers })}
					{!isVertical && ' 🫧'}
				</div>
			</>
		)
	}
	return (
		<div
			css={[
				styles.shape,
				isVertical && styles.verticalShape,
				!isVertical && componentBackground,
			]}
			style={props.style}
		>
			<div
				css={[
					styles.message,
					isVertical && styles.verticalMessage,
					!isVertical && componentStyle,
				]}
			>
				{content}
			</div>
		</div>
	)
}

export default Message
