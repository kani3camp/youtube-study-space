/** @jsxImportSource @emotion/react */
import { useTranslation } from 'next-i18next/pages'
import { type CSSProperties, type FC, memo } from 'react'
import { componentBackground, componentStyle } from '../styles/common.style'
import * as styles from '../styles/Usage.styles'
import type { MonitorVariant } from './monitor/types'

type UsageProps = {
	variant?: MonitorVariant
	style?: CSSProperties
}

const Usage: FC<UsageProps> = ({ variant = 'horizontal', style }) => {
	const { t } = useTranslation()
	const isVertical = variant === 'vertical'
	const commands = (
		<>
			<div css={[styles.command, isVertical && styles.verticalCommand]}>
				<span
					css={[styles.commandCode, isVertical && styles.verticalCommandCode]}
				>
					!in　{t('usage.work')}
				</span>
				<span
					css={[styles.commandDesc, isVertical && styles.verticalCommandDesc]}
				>
					{t('usage.in')}
				</span>
			</div>
			<div css={[styles.command, isVertical && styles.verticalCommand]}>
				<span
					css={[styles.commandCode, isVertical && styles.verticalCommandCode]}
				>
					!out
				</span>
				<span
					css={[styles.commandDesc, isVertical && styles.verticalCommandDesc]}
				>
					{t('usage.out')}
				</span>
			</div>
		</>
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
			<div
				css={[
					styles.usage,
					isVertical && styles.verticalUsage,
					!isVertical && componentStyle,
				]}
			>
				<h4
					css={[styles.description, isVertical && styles.verticalDescription]}
				>
					{t('usage.description')}
				</h4>
				{isVertical ? (
					<div css={styles.verticalCommands}>{commands}</div>
				) : (
					commands
				)}
			</div>
		</div>
	)
}

export default memo(Usage)
