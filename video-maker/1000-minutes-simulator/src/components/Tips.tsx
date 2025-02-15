import { useTranslation } from 'next-i18next'
import type { FC } from 'react'
import { MdTipsAndUpdates } from 'react-icons/md'
import tipsENJson from '../assets/tips.en.json'
import tipsJPJson from '../assets/tips.jp.json'
import tipsKOJson from '../assets/tips.ko.json'
import * as styles from '../styles/Tips.styles'
import * as common from '../styles/common.styles'

type Props = {
	elapsedSeconds: number
}

type Tips = {
	tips: string
	name: string
	annotation: string
}
const tipsList: { [key: string]: Tips[] } = {
	jp: tipsJPJson.map((value) => value),
	ko: tipsKOJson.map((value) => value),
	en: tipsENJson.map((value) => value),
}

const TipsIntervalSeconds = 30 * 60

const Tips: FC<Props> = (props) => {
	const { t, i18n } = useTranslation()

	const tipsIndex = Math.floor(props.elapsedSeconds / TipsIntervalSeconds)
	const tips = tipsList[i18n.language][tipsIndex]
	let tipsText: string
	let poster: string
	let note: string
	if (tips !== undefined) {
		tipsText = tips.tips
		poster = tips.name
		note = tips.annotation
	} else {
		tipsText = t('tips.text')
		poster = t('tips.poster')
		note = t('tips.comment')
	}

	return (
		<div css={styles.tips}>
			<div css={styles.innerCell}>
				<div css={common.heading}>
					<MdTipsAndUpdates size={common.IconSize} css={styles.icon} />
					<span>Tips</span>
				</div>

				<div css={styles.tipsTextContainer}>
					<div css={styles.tipsTextPrefix}>No. {tipsIndex + 1}</div>
					<div css={styles.tipsText}>{tipsText}</div>
				</div>
				<div css={styles.tipsPosterContainer}>
					<div css={styles.tipsPosterPrefix}>{t('tips.poster')}</div>
					<div css={styles.tipsPoster}>
						{poster}
						<span>{t('tips.poster_footer')}</span>
					</div>
				</div>
				<div css={styles.tipsNoteContainer}>
					<div css={styles.tipsNotePrefix}>{t('tips.comment')}</div>
					<div css={styles.tipsNote}>{note}</div>
				</div>
			</div>
		</div>
	)
}

export default Tips
