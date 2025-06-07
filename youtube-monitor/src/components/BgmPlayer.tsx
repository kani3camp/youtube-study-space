import { Wave } from '@foobar404/wave'
import { parseWebStream, parseBlob, type IAudioMetadata } from 'music-metadata'
import type React from 'react'
import { useEffect, useRef, useState, useCallback } from 'react'
import { getCurrentRandomBgm } from '../lib/bgm'
import { Constants } from '../lib/constants'
import { SectionType, getCurrentSection } from '../lib/time-table'
import * as styles from '../styles/BgmPlayer.styles'
import { componentBackground, componentStyle } from '../styles/common.style'

const BgmPlayer: React.FC = () => {
	const [lastSectionType, setLastSectionType] = useState('')
	const [audioTitle, setAudioTitle] = useState('BGM TITLE')
	const [audioArtist, setAudioArtist] = useState('BGM ARTIST')

	const audioDivId = 'music'
	const audioCanvasId = 'audioCanvas'
	const chimeSingleDivId = 'chimeSingle'
	const chimeDoubleDivId = 'chimeDouble'

	const waveRef = useRef<Wave | null>(null)

	useEffect(() => {
		checkChimeCanPlay()
	}, [])

	const checkChimeCanPlay = async () => {
		console.log('checking chime audio files.')
		const chimeDivIdList = [chimeSingleDivId, chimeDoubleDivId]
		for (const divId of chimeDivIdList) {
			const chime = document.getElementById(divId) as HTMLAudioElement
			chime.addEventListener('error', () => {
				alert(`error loading: ${chime.src}`)
			})
			if (!chime.src) {
				alert(`invalid chime src: ${chime.src}`)
			}
			chime.load()
		}
	}

	const updateState = useCallback(() => {
		const currentSection = getCurrentSection()

		// 休憩時間から作業時間に変わるタイミングでチャイムを再生
		if (
			lastSectionType === SectionType.Break &&
			currentSection.sectionType === SectionType.Study
		) {
			playChimeSingle()
		}
		// 作業時間から休憩時間に変わるタイミングでチャイムを再生
		if (
			lastSectionType === SectionType.Study &&
			currentSection.sectionType === SectionType.Break
		) {
			playChimeDouble()
		}
		setLastSectionType(currentSection.sectionType)
	}, [lastSectionType])

	type ID3Tag = {
		tags: {
			title: string | null
			artist: string | null
		}
	}

	const audioNext = useCallback(async () => {
		try {
			const audio = document.getElementById(
				audioDivId,
			) as HTMLAudioElement | null
			if (!audio) {
				console.error(`Audio element with ID '${audioDivId}' not found.`)
				return
			}

			const bgm = await getCurrentRandomBgm()
			audio.src = bgm

			const response = await fetch(audio.src)
			if (!response.ok) {
				throw new Error(
					`Failed to fetch audio: ${response.status} ${response.statusText}`,
				)
			}

			let metadata: IAudioMetadata
			if (!response.body) {
				// Fall back on Blob if web stream is not supported
				const blob = await response.blob()
				metadata = await parseBlob(blob)
			} else {
				const contentLength = response.headers.get('Content-Length')
				const size = contentLength ? Number.parseInt(contentLength) : undefined
				metadata = await parseWebStream(response.body, {
					mimeType: response.headers.get('Content-Type') ?? undefined,
					size,
				})
			}

			setAudioTitle(metadata.common.title ?? 'BGM TITLE')
			setAudioArtist(metadata.common.artist ?? 'BGM ARTIST')

			audio.volume = Constants.bgmVolume
			await audio.play()
		} catch (error) {
			console.error('Failed to play audio or parse metadata:', error)
		}
	}, [])

	const audioStart = useCallback(() => {
		const audio = document.getElementById(audioDivId) as HTMLAudioElement
		audio.addEventListener('ended', () => {
			setAudioTitle('BGM TITLE')
			setAudioArtist('BGM ARTIST')
			audioNext()
		})
		audio.addEventListener('error', () => {
			console.error(`Error loading audio file: ${audio.src}`)
			audioNext()
		})
		audioNext()
	}, [audioNext])

	const stop = () => {
		const audio = document.getElementById(audioDivId) as HTMLAudioElement
		audio.pause()
		setAudioTitle('BGM TITLE')
		setAudioArtist('BGM ARTIST')
	}

	const playChimeSingle = () => {
		const chimeSingle = document.getElementById(
			chimeSingleDivId,
		) as HTMLAudioElement
		chimeSingle.volume = Constants.chimeVolume
		chimeSingle.play()
	}

	const playChimeDouble = () => {
		const chimeDouble = document.getElementById(
			chimeDoubleDivId,
		) as HTMLAudioElement
		chimeDouble.volume = Constants.chimeVolume
		chimeDouble.play()
	}

	useEffect(() => {
		if (!waveRef.current) {
			const audioElement = document.getElementById(
				audioDivId,
			) as HTMLAudioElement
			const canvasElement = document.getElementById(
				audioCanvasId,
			) as HTMLCanvasElement

			const wave = new Wave(audioElement, canvasElement)
			wave.addAnimation(
				new wave.animations.Wave({
					fillColor: '#27272787',

					lineColor: '#0000', // NOTE: alpha=0 が目的。lineWidth: 0が意味なさそうだったので
					rounded: true,
					bottom: true,
					count: 30,
				}),
			)
			audioStart()

			waveRef.current = wave
		}
	}, [audioStart])

	useEffect(() => {
		const intervalId = setInterval(() => updateState(), 1000)
		return () => {
			clearInterval(intervalId)
		}
	}, [updateState])

	return (
		<div css={[styles.shape, componentBackground]}>
			<div css={[styles.bgmPlayer, componentStyle]}>
				<audio autoPlay id={audioDivId}>
					<track kind="captions" />
				</audio>

				<audio id={chimeSingleDivId} src={Constants.chimeSingleFilePath}>
					<track kind="captions" />
				</audio>
				<audio id={chimeDoubleDivId} src={Constants.chimeDoubleFilePath}>
					<track kind="captions" />
				</audio>
				<h4>♪ {audioTitle}</h4>
				<h4>by {audioArtist}</h4>

				<div css={styles.audioCanvasDiv}>
					<canvas id={audioCanvasId} css={styles.audioCanvas} />
				</div>
			</div>
		</div>
	)
}

export default BgmPlayer
