import { useTranslation } from 'next-i18next'
import { FC } from 'react'
import * as styles from '../styles/Message.styles'
import { Seat } from '../types/api'
import { componentBackground } from '../styles/common.style'

type Props = {
    currentPageIndex: number
    currentPagesLength: number
    currentPageIsMember: boolean
    seats: Seat[]
}

const Message: FC<Props> = (props) => {
    const { t } = useTranslation()

    let content = <></>
    if (props.seats) {
        const numWorkers = props.seats.length
        content = (
            <>
                <div css={styles.pageInfo}>
                    <div css={styles.pageIndex}>
                        {t('message.room', {
                            index: props.currentPageIndex + 1,
                            length: props.currentPagesLength,
                        })}
                    </div>
                    {props.currentPageIsMember && <div css={styles.memberOnly}>{t('member')}</div>}
                </div>
                <div css={styles.numStudyingPeople}>
                    {t('message.num_studying_people', { value: numWorkers })} 🫧
                </div>
            </>
        )
    }
    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={styles.message}>{content}</div>
        </div>
    )
}

export default Message
