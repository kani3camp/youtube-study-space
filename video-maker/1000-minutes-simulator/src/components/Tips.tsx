import { FC } from 'react'
import { MdTipsAndUpdates } from 'react-icons/md'
import tipsJson from '../lib/tips.json'
import * as styles from '../styles/Tips.styles'
import * as common from '../styles/common.styles'

type Props = {
    elapsedMinutes: number
}

type Tips = {
    tips: string
    name: string
    annotation: string
}
const tipsIterator = tipsJson.values()
const tipsList: Tips[] = []
// eslint-disable-next-line no-constant-condition
while (true) {
    const next = tipsIterator.next()
    if (next.done) {
        break
    }
    tipsList.push(next.value as unknown as Tips)
}

const Tips: FC<Props> = (props) => {
    const tipsIndex = Math.floor(props.elapsedMinutes / 30)
    const tips = tipsList[tipsIndex]
    const tipsText = tips.tips
    const poster = tips.name
    const note = tips.annotation

    return (
        <div css={styles.tips}>
            <div css={styles.innerCell}>
                <div css={common.heading}>
                    <MdTipsAndUpdates
                        size={common.IconSize}
                        css={styles.icon}
                    ></MdTipsAndUpdates>
                    <span>Tips</span>
                </div>

                <div css={styles.tipsTextContainer}>
                    <div css={styles.tipsTextPrefix}>
                        名言 No. {tipsIndex + 1}
                    </div>
                    <div css={styles.tipsText}>{tipsText}</div>
                </div>
                <div css={styles.tipsPosterContainer}>
                    <div css={styles.tipsPosterPrefix}>投稿者</div>
                    <div css={styles.tipsPoster}>
                        {poster}
                        <span>さん</span>
                    </div>
                </div>
                <div css={styles.tipsNoteContainer}>
                    <div css={styles.tipsNotePrefix}>コメント</div>
                    <div css={styles.tipsNote}>{note}</div>
                </div>
            </div>
        </div>
    )
}

export default Tips
