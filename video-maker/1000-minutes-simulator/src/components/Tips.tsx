import { FC } from 'react'
import { MdTipsAndUpdates } from 'react-icons/md'
import tipsJson from '../lib/tips.json'
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

const TipsIntervalSeconds = 30 * 60

const Tips: FC<Props> = (props) => {
    const tipsIndex = Math.floor(props.elapsedSeconds / TipsIntervalSeconds)
    const tips = tipsList[tipsIndex]
    let tipsText: string
    let poster: string
    let note: string
    if (tips !== undefined) {
        tipsText = tips.tips
        poster = tips.name
        note = tips.annotation
    } else {
        tipsText = '名言・Tips'
        poster = '投稿者'
        note = 'コメント'
    }

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
                    <div css={styles.tipsTextPrefix}>No. {tipsIndex + 1}</div>
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
