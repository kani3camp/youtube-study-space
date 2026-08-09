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
	const verticalCommands = (
		<>
			<div css={[styles.command, styles.verticalCommand]}>
				<span css={[styles.commandCode, styles.verticalCommandCode]}>!in</span>
				<span css={[styles.commandDesc, styles.verticalCommandDesc]}>
					{t('usage.work')} {t('usage.in')}
				</span>
			</div>
			<div css={[styles.command, styles.verticalCommand]}>
				<span css={[styles.commandCode, styles.verticalCommandCode]}>!out</span>
				<span css={[styles.commandDesc, styles.verticalCommandDesc]}>
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
				{isVertical ? (
					<div css={styles.verticalCommands}>{verticalCommands}</div>
				) : (
					<>
						<h4 css={styles.description}>{t('usage.description')}</h4>
						{commands}
					</>
				)}
			</div>
		</div>
	)
}

export default memo(Usage)
