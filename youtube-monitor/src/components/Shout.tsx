import { useTranslation } from 'next-i18next'
import { FC, useEffect, useState } from 'react'
import api from '../lib/api-config'
import * as styles from '../styles/Shout.styles'
import { componentStyle, componentBackground } from '../styles/common.style'
import fetcher from '../lib/fetcher'
import { DisplayShoutMessageResponse } from '../types/api'
import { useInterval } from '../lib/common'

type Props = {
    updateShoutMessageIntervalMinutes: number
}

const Shout: FC<Props> = (props) => {
    const { t } = useTranslation()

    const [shoutMessage, setShoutMessage] = useState<string>('')
    const [userId, setUserId] = useState<string>('')
    const [userDisplayName, setUserDisplayName] = useState<string>('')

    useInterval(
        () => {
            requestDisplayShoutMessage()
        },
        props.updateShoutMessageIntervalMinutes * 60 * 1000
    )

    useEffect(() => {
        // TODO: 座席に座っていたら座席番号も出したい
    }, [userId])

    const requestDisplayShoutMessage = async () => {
        await fetcher<DisplayShoutMessageResponse>(api.displayShoutMessage, {
            method: 'GET',
        }).then(async (r) => {
            console.log('requestDisplayShoutMessage succeeded')
            console.debug(r)
            setShoutMessage(r.shout_message)
            setUserId(r.user_id)
            setUserDisplayName(r.user_display_name)
        })
    }

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={[styles.shout, componentStyle]}>
                <h3 css={styles.description}>{t('shout.description')}</h3>

                <div css={styles.shoutContent}>
                    <div css={styles.messageText}>
                        {shoutMessage
                            ? shoutMessage
                            : 'やっほうこれが目標だ！ !shoutで目標を宣言しよう！'}
                    </div>
                    <div css={styles.userName}>
                        {t('shout.username', {
                            name: userDisplayName ? userDisplayName : 'ユーザー',
                        })}
                    </div>
                </div>
            </div>
        </div>
    )
}

export default Shout
