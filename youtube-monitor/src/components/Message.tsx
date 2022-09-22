import { useTranslation } from 'next-i18next'
import { FC } from 'react'
import * as styles from '../styles/Message.styles'
import { Seat } from '../types/api'

type Props = {
    currentPageIndex: number
    currentRoomsLength: number
    seats: Seat[]
}

const Message: FC<Props> = (props) => {
    const { t } = useTranslation()

    if (props.seats) {
        const numWorkers = props.seats.length
        return (
            <div css={styles.message}>
                <div css={styles.roomName}>
                    {t('message.room', {
                        index: props.currentPageIndex + 1,
                        length: props.currentRoomsLength,
                    })}
                </div>
                <div css={styles.numStudyingPeople}>
                    {t('message.num_studying_people', { value: numWorkers })}
                </div>
            </div>
        )
    } else {
        return <div css={styles.message} />
    }
}

export default Message
