import { Wave } from '@foobar404/wave'
import { type IAudioMetadata, parseBlob, parseWebStream } from 'music-metadata'
import type React from 'react'
import { memo, useCallback, useEffect, useRef, useState } from 'react'
import { getCurrentRandomBgm } from '../lib/bgm'
import { Constants, getBgmVolume } from '../lib/constants'
import { getCurrentSection, SectionType } from '../lib/time-table'
import * as styles from '../styles/BgmPlayer.styles'
import { componentBackground, componentStyle } from '../styles/common.style'

/**
 * BGM の読み込み・再生に連続して失敗した場合に、次の曲への切り替えを諦めるまでの回数。
 * 再生に成功すると 0 に戻る。
 */
export const MAX_CONSECUTIVE_BGM_ERRORS = 5

const BgmPlayer: React.FC = () => {
	const [lastSectionType, setLastSectionType] = useState('')
	const [audioTitle, setAudioTitle] = useState('BGM TITLE')
	const [audioArtist, setAudioArtist] = useState('BGM ARTIST')

	const waveRef = useRef<Wave | null>(null)
	const bgmAudioRef = useRef<HTMLAudioElement>(null)
	const audioCanvasRef = useRef<HTMLCanvasElement>(null)
	const chimeSingleRef = useRef<HTMLAudioElement>(null)
	const chimeDoubleRef = useRef<HTMLAudioElement>(null)
	const consecutiveBgmErrorsRef = useRef(0)

	// マウント時に一度だけチャイム用 audio 要素の error ハンドラ登録と読み込みを実行する
	useEffect(() => {
		const chimeList = [chimeSingleRef.current, chimeDoubleRef.current]
		const cleanupFns: Array<() => void> = []
		for (const chime of chimeList) {
			if (!chime) continue
			const handleError = () => {
				console.error(`error loading: ${chime.src}`)
			}
			chime.addEventListener('error', handleError)
			cleanupFns.push(() => {
				chime.removeEventListener('error', handleError)
			})
			if (!chime.src) {
				console.error(`invalid chime src: ${chime.src}`)
				continue
			}
			chime.load()
		}
		console.log('checking chime audio files.')

		return () => {
			for (const cleanup of cleanupFns) {
				cleanup()
			}
		}
	}, [])

	const playChimeSingle = useCallback(() => {
		const el = chimeSingleRef.current
		if (el) {
			el.volume = Constants.chimeVolume
			void el.play().catch((err) => {
				console.error('Chime single play failed:', err)
			})
		}
	}, [])

	const playChimeDouble = useCallback(() => {
		const el = chimeDoubleRef.current
		if (el) {
			el.volume = Constants.chimeVolume
			void el.play().catch((err) => {
				console.error('Chime double play failed:', err)
			})
		}
	}, [])

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
	}, [lastSectionType, playChimeSingle, playChimeDouble])

	const audioNext = useCallback(async () => {
		try {
			const audio = bgmAudioRef.current
			if (!audio) {
				console.error('BGM audio element ref is not attached.')
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
				const size = contentLength
					? Number.parseInt(contentLength, 10)
					: undefined
				metadata = await parseWebStream(response.body, {
					mimeType: response.headers.get('Content-Type') ?? undefined,
					size,
				})
			}

			setAudioTitle(metadata.common.title ?? 'BGM TITLE')
			setAudioArtist(metadata.common.artist ?? 'BGM ARTIST')

			audio.volume = getBgmVolume()
			await audio.play()
		} catch (error) {
			console.error('Failed to play audio or parse metadata:', error)
		}
	}, [])

	// BGM の audio 要素のイベントリスナーはマウント中だけ有効にし、アンマウント時に解除する。
	// error による次曲への切り替えは、連続失敗が上限に達したら打ち切る。
	useEffect(() => {
		const audio = bgmAudioRef.current
		if (!audio) {
			return
		}
		const handleEnded = () => {
			setAudioTitle('BGM TITLE')
			setAudioArtist('BGM ARTIST')
			void audioNext()
		}
		const handleError = () => {
			consecutiveBgmErrorsRef.current += 1
			console.error(`Error loading audio file: ${audio.src}`)
			if (consecutiveBgmErrorsRef.current >= MAX_CONSECUTIVE_BGM_ERRORS) {
				console.error(
					`BGM failed ${consecutiveBgmErrorsRef.current.toString()} times in a row; giving up.`,
				)
				return
			}
			void audioNext()
		}
		const handlePlaying = () => {
			consecutiveBgmErrorsRef.current = 0
		}
		audio.addEventListener('ended', handleEnded)
		audio.addEventListener('error', handleError)
		audio.addEventListener('playing', handlePlaying)
		return () => {
			audio.removeEventListener('ended', handleEnded)
			audio.removeEventListener('error', handleError)
			audio.removeEventListener('playing', handlePlaying)
		}
	}, [audioNext])

	const _stop = () => {
		const audio = bgmAudioRef.current
		if (!audio) {
			return
		}
		audio.pause()
		setAudioTitle('BGM TITLE')
		setAudioArtist('BGM ARTIST')
	}

	useEffect(() => {
		if (!waveRef.current) {
			const audioElement = bgmAudioRef.current
			const canvasElement = audioCanvasRef.current
			if (!audioElement || !canvasElement) {
				return
			}

			const wave = new Wave(audioElement, canvasElement)
			wave.addAnimation(
				new wave.animations.Wave({
					fillColor: '#27272787',

					lineColor: '#0000', // NOTE: alpha=0 で線を非表示にする目的（lineWidth: 0 では効果が見られなかったため）
					rounded: true,
					bottom: true,
					count: 30,
				}),
			)
			void audioNext()

			waveRef.current = wave
		}
	}, [audioNext])

	useEffect(() => {
		const intervalId = setInterval(() => updateState(), 1000)
		return () => {
			clearInterval(intervalId)
		}
	}, [updateState])

	return (
		<div css={[styles.shape, componentBackground]}>
			<div css={[styles.bgmPlayer, componentStyle]}>
				<audio ref={bgmAudioRef} autoPlay>
					<track kind="captions" />
				</audio>

				<audio ref={chimeSingleRef} src={Constants.chimeSingleFilePath}>
					<track kind="captions" />
				</audio>
				<audio ref={chimeDoubleRef} src={Constants.chimeDoubleFilePath}>
					<track kind="captions" />
				</audio>
				<h4>♪ {audioTitle}</h4>
				<h4>by {audioArtist}</h4>

				<div css={styles.audioCanvasDiv}>
					<canvas ref={audioCanvasRef} css={styles.audioCanvas} />
				</div>
			</div>
		</div>
	)
}

export default memo(BgmPlayer)
