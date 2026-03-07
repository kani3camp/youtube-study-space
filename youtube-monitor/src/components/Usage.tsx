/** @jsxImportSource @emotion/react */
import { useTranslation } from 'next-i18next'
import { type FC, memo } from 'react'
import { componentBackground, componentStyle } from '../styles/common.style'
import * as styles from '../styles/Usage.styles'

const Usage: FC = () => {
	const { t } = useTranslation()

	return (
		<div css={[styles.shape, componentBackground]}>
			<div css={[styles.usage, componentStyle]}>
				<h4 css={styles.description}>{t('usage.description')}</h4>
				<div css={styles.command}>
					<span css={styles.commandCode}>!in　{t('usage.work')}</span>
					<span css={styles.commandDesc}>{t('usage.in')}</span>
				</div>
				<div css={styles.command}>
					<span css={styles.commandCode}>!out</span>
					<span css={styles.commandDesc}>{t('usage.out')}</span>
				</div>
			</div>
		</div>
	)
}

export default memo(Usage)
