import jsmediatags from 'jsmediatags'
import { useTranslation } from 'next-i18next/pages'
import { type FC, useEffect, useState } from 'react'
import { BsFillPersonFill } from 'react-icons/bs'
import { IoMdMusicalNotes } from 'react-icons/io'
import { MdQueueMusic } from 'react-icons/md'
import { getCurrentRandomBgm } from '../lib/bgm'
import { OffsetSec } from '../pages'
import * as styles from '../styles/BGMPlayer.styles'
import * as common from '../styles/common.styles'

type Props = {
	elapsedMinutes: number
}

const AUDIO_DIV_ID = 'music'

const BGMPlayer: FC<Props> = (props) => {
	const { t } = useTranslation()

	const [audioTitle, setAudioTitle] = useState<string>(t('bgm.title'))
	const [audioArtist, setAudioArtist] = useState<string>(t('bgm.artist'))

	useEffect(() => {
		const audio = document.getElementById(AUDIO_DIV_ID) as HTMLAudioElement

		const audioNext = async () => {
			const bgm = await getCurrentRandomBgm()

			audio.src = bgm
			jsmediatags.read(audio.src, {
				onSuccess(tag) {
					const title = tag.tags.title
					const artist = tag.tags.artist
					setAudioTitle(
						title !== null && title !== undefined ? title : t('bgm.title'),
					)
					setAudioArtist(
						artist !== null && artist !== undefined ? artist : t('bgm.artist'),
					)
				},
				onError(error) {
					console.error(error)
				},
			})
			audio.volume = 0.3
		}

		const handleEnded = () => {
			setAudioTitle(t('bgm.title'))
			setAudioArtist(t('bgm.artist'))
			void audioNext()
		}
		const handleError = (event: Event) => {
			console.error('failed loading audio: ', event)
			void audioNext()
		}
		const handleLoadedData = () => {
			void audio.play()
		}

		audio.addEventListener('ended', handleEnded)
		audio.addEventListener('error', handleError)
		audio.addEventListener('loadeddata', handleLoadedData)

		const startTimer = window.setTimeout(() => {
			void audioNext()
		}, OffsetSec * 1000)

		return () => {
			window.clearTimeout(startTimer)
			audio.removeEventListener('ended', handleEnded)
			audio.removeEventListener('error', handleError)
			audio.removeEventListener('loadeddata', handleLoadedData)
		}
	}, [t])

	return (
		<div css={styles.bgmPlayer}>
			<div css={styles.innerCell}>
				<div css={common.heading}>
					<MdQueueMusic size={common.IconSize} css={styles.icon} />
					<span>BGM</span>
				</div>
				<div css={styles.item}>
					<IoMdMusicalNotes />
					<span>{audioTitle}</span>
				</div>
				<div css={styles.item}>
					<BsFillPersonFill />
					<span>{audioArtist}</span>
				</div>

				<audio autoPlay id={AUDIO_DIV_ID} />
			</div>
		</div>
	)
}

export default BGMPlayer
