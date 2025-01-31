import { type FC, useEffect, useMemo, useState } from 'react'
import api from '../lib/api-config'
import fetcher from '../lib/fetcher'
import {
	allRooms,
	numSeatsInGeneralAllBasicRooms,
	numSeatsInMemberAllBasicRooms,
} from '../rooms/rooms-config'
import { mainContent } from '../styles/MainContent.styles'
import type { Seat, SetDesiredMaxSeatsResponse } from '../types/api'
import type { RoomLayout } from '../types/room-layout'
import CenterLoading from './CenterLoading'
import Message from './Message'
import SeatsPage, { type LayoutPageProps } from './SeatsPage'

import { initializeApp } from 'firebase/app'
import {
	collection,
	doc,
	getFirestore,
	onSnapshot,
	query,
} from 'firebase/firestore'
import { useRouter } from 'next/router'
import { numSeatsOfRoomLayouts, useInterval } from '../lib/common'
import { Constants } from '../lib/constants'
import {
	type SystemConstants,
	firestoreConstantsConverter,
	firestoreSeatConverter,
	getFirebaseConfig,
} from '../lib/firestore'

const PAGING_INTERVAL_MSEC = Constants.pagingIntervalSeconds * 1000

const Seats: FC = () => {
	const router = useRouter()

	const [latestGeneralSeats, setLatestGeneralSeats] = useState<Seat[]>([])
	const [latestMemberSeats, setLatestMemberSeats] = useState<Seat[]>([])
	const [latestGeneralMaxSeats, setLatestGeneralMaxSeats] = useState<number>()
	const [latestMemberMaxSeats, setLatestMemberMaxSeats] = useState<number>()
	const [latestMinVacancyRate, setLatestMinVacancyRate] = useState<number>()
	const [currentPageIndex, setCurrentPageIndex] = useState<number>(0)
	const [latestGeneralLayouts, setGeneralLayouts] = useState<RoomLayout[]>([])
	const [latestMemberLayouts, setMemberLayouts] = useState<RoomLayout[]>([])
	const [pageProps, setPageProps] = useState<LayoutPageProps[]>([])
	const [latestYoutubeMembershipEnabled, setLatestYoutubeMembershipEnabled] =
		useState<boolean>(false)
	const [latestFixedMaxSeatsEnabled, setLatestFixedMaxSeatsEnabled] =
		useState<boolean>()

	useEffect(() => {
		if (process.env.NEXT_PUBLIC_API_KEY === undefined) {
			alert('NEXT_PUBLIC_API_KEY is not defined')
		}

		initFirestore()
	}, [])

	useInterval(() => {
		refreshPageIndex()
	}, PAGING_INTERVAL_MSEC)

	useEffect(() => {
		console.log('[currentPageIndex]:', currentPageIndex)
		changePage(currentPageIndex)
	}, [currentPageIndex])

	/**
	 * URLのクエリパラメータにpageが指定されており、かつ座席データも読み込めていたらそのページを表示する。
	 */
	useEffect(() => {
		if (router && pageProps.length > 0) {
			if (router.query.page !== undefined) {
				const queryPageIndex = getQueryPageIndex()
				if (queryPageIndex !== undefined) {
					setCurrentPageIndex(queryPageIndex)
				}
			}
		}
	}, [router.query.page, pageProps.length])

	/**
	 * 入室状況もしくはレイアウト編成に変更があったら、全ページを更新する。
	 */
	useEffect(() => {
		updatePageProps()
	}, [
		latestGeneralSeats,
		latestMemberSeats,
		latestGeneralLayouts,
		latestMemberLayouts,
	])

	/**
	 * 入室状況に変更があったら、座席数を見直す。
	 */
	useEffect(() => {
		reviewMaxSeats(
			latestMinVacancyRate,
			latestYoutubeMembershipEnabled,
			latestFixedMaxSeatsEnabled,
		)
	}, [latestGeneralSeats, latestMemberSeats])

	/**
	 * 許容空席率もしくはmax_seatsが変更されたら、座席数の見直しを行う。
	 * システム管理者が手動で更新しない限り、各変数の初期化時のみ実行される。
	 */
	useEffect(() => {
		reviewMaxSeats(
			latestMinVacancyRate,
			latestYoutubeMembershipEnabled,
			latestFixedMaxSeatsEnabled,
		)
	}, [
		latestGeneralMaxSeats,
		latestMemberMaxSeats,
		latestMinVacancyRate,
		latestYoutubeMembershipEnabled,
		latestFixedMaxSeatsEnabled,
	])

	const getQueryPageIndex = (): number | undefined => {
		const queryPageNum = router.query.page
		console.debug('queryPageNum:', queryPageNum)
		if (
			queryPageNum !== undefined &&
			Number(queryPageNum) > 0 &&
			Number(queryPageNum) <= pageProps.length
		) {
			return Number(queryPageNum) - 1
		}
		return undefined
	}

	const refreshPageIndex = () => {
		if (pageProps.length > 0) {
			const queryPageIndex: number | undefined = getQueryPageIndex()
			if (queryPageIndex !== undefined) {
				setCurrentPageIndex(queryPageIndex)
			} else {
				const newPageIndex = (currentPageIndex + 1) % pageProps.length
				setCurrentPageIndex(newPageIndex)
			}
		}
	}

	const initFirestore = () => {
		const app = initializeApp(getFirebaseConfig())
		const db = getFirestore(app)

		const constantsConverter = firestoreConstantsConverter
		const seatConverter = firestoreSeatConverter

		const generalSeatsQuery = query(collection(db, 'seats')).withConverter(
			seatConverter,
		)
		onSnapshot(generalSeatsQuery, (querySnapshot) => {
			const seats: Seat[] = []
			querySnapshot.forEach((doc) => {
				seats.push(doc.data())
			})
			setLatestGeneralSeats(seats)
		})
		const memberSeatsQuery = query(
			collection(db, 'member-seats'),
		).withConverter(seatConverter)
		onSnapshot(memberSeatsQuery, (querySnapshot) => {
			const seats: Seat[] = []
			querySnapshot.forEach((doc) => {
				seats.push(doc.data())
			})
			setLatestMemberSeats(seats)
		})

		onSnapshot(
			doc(db, 'config', 'constants').withConverter(constantsConverter),
			(doc) => {
				const generalMaxSeats = (doc.data() as SystemConstants).max_seats
				const memberMaxSeats = (doc.data() as SystemConstants).member_max_seats
				const minVacancyRate = (doc.data() as SystemConstants).min_vacancy_rate
				const youtubeMembershipEnabled = (doc.data() as SystemConstants)
					.youtube_membership_enabled
				const fixedMaxSeatsEnabled = (doc.data() as SystemConstants)
					.fixed_max_seats_enabled
				setLatestGeneralMaxSeats(generalMaxSeats)
				setLatestMemberMaxSeats(memberMaxSeats)
				setLatestMinVacancyRate(minVacancyRate)
				setLatestYoutubeMembershipEnabled(youtubeMembershipEnabled)
				setLatestFixedMaxSeatsEnabled(fixedMaxSeatsEnabled)
			},
		)
	}

	/**
	 * Changes the page to be displayed.
	 * @param pageIndex The index number of the page you want to display next (starting from 0)
	 */
	const changePage = (pageIndex: number) => {
		const snapshotPageProps = [...pageProps]
		if (pageIndex + 1 > snapshotPageProps.length) {
			pageIndex = 0 // index out of range にならないように１ページ目に。
		}
		const newPageProps: LayoutPageProps[] = snapshotPageProps.map(
			(page, index) => {
				if (index === pageIndex) {
					page.display = true
				} else {
					page.display = false
				}
				return page
			},
		)
		setPageProps(newPageProps)
	}

	const layoutPagesMemo = useMemo(
		() =>
			pageProps.map((pageProp, index) => (
				<SeatsPage
					key={index}
					firstSeatId={pageProp.firstSeatId}
					roomLayout={pageProp.roomLayout}
					usedSeats={pageProp.usedSeats}
					display={pageProp.display}
					memberOnly={pageProp.memberOnly}
				/>
			)),
		[pageProps],
	)

	/**
	 * 座席数の見直しを行う。
	 * 座席数の増減が必要な場合は、APIにリクエストを送信し、ルーム数を調整する。
	 */
	const reviewMaxSeats = async (
		min_vacancy_rate: number | undefined,
		membership_enabled: boolean,
		fixed_max_seats_enabled: boolean | undefined,
	) => {
		const snapshotGeneralMaxSeats = latestGeneralMaxSeats
		const snapshotGeneralSeats = [...latestGeneralSeats]
		const snapshotMemberMaxSeats = latestMemberMaxSeats
		const snapshotMemberSeats = [...latestMemberSeats]

		if (
			snapshotGeneralMaxSeats === undefined ||
			snapshotMemberMaxSeats === undefined ||
			min_vacancy_rate === undefined ||
			fixed_max_seats_enabled === undefined
		) {
			return
		}

		if (fixed_max_seats_enabled) {
			const numSeatsGeneralBasicRooms = numSeatsInGeneralAllBasicRooms()
			const numSeatsMemberBasicRooms = numSeatsInMemberAllBasicRooms()
			if (
				snapshotGeneralMaxSeats !== numSeatsGeneralBasicRooms ||
				snapshotMemberMaxSeats !== numSeatsMemberBasicRooms
			) {
				console.log('sending request to change max_seats')
				console.log(
					`general: ${snapshotGeneralMaxSeats} => ${numSeatsGeneralBasicRooms}`,
				)
				console.log(
					`members-only: ${snapshotMemberMaxSeats} => ${numSeatsMemberBasicRooms}`,
				)
				await requestMaxSeatsUpdate(
					numSeatsGeneralBasicRooms,
					numSeatsMemberBasicRooms,
				)
			}
		} else {
			// GENERAL
			// まず、現状の入室状況（seatsとmax_seats）と設定された空席率（min_vacancy_rate）を基に、適切なmax_seatsを求める。
			let finalDesiredGeneralMaxSeats: number
			const generalMinSeatsByVacancyRate = Math.ceil(
				snapshotGeneralSeats.length / (1 - min_vacancy_rate),
			)
			// もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
			if (generalMinSeatsByVacancyRate > numSeatsInGeneralAllBasicRooms()) {
				let current_num_seats: number = numSeatsInGeneralAllBasicRooms()
				let current_adding_temporary_room_index = 0
				while (current_num_seats < generalMinSeatsByVacancyRate) {
					current_num_seats +=
						allRooms.generalTemporaryRooms[current_adding_temporary_room_index]
							.seats.length
					current_adding_temporary_room_index =
						(current_adding_temporary_room_index + 1) %
						allRooms.generalTemporaryRooms.length
				}
				finalDesiredGeneralMaxSeats = current_num_seats
			} else {
				// そうでなければ、基本ルームの席数とするべき
				finalDesiredGeneralMaxSeats = numSeatsInGeneralAllBasicRooms()
			}

			// MEMBER
			let finalDesiredMemberMaxSeats: number
			if (membership_enabled) {
				const memberMinSeatsByVacancyRate = Math.ceil(
					snapshotMemberSeats.length / (1 - min_vacancy_rate),
				)
				// もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
				if (memberMinSeatsByVacancyRate > numSeatsInMemberAllBasicRooms()) {
					let current_num_seats: number = numSeatsInMemberAllBasicRooms()
					let current_adding_temporary_room_index = 0
					while (current_num_seats < memberMinSeatsByVacancyRate) {
						current_num_seats +=
							allRooms.memberTemporaryRooms[current_adding_temporary_room_index]
								.seats.length
						current_adding_temporary_room_index =
							(current_adding_temporary_room_index + 1) %
							allRooms.memberTemporaryRooms.length
					}
					finalDesiredMemberMaxSeats = current_num_seats
				} else {
					// そうでなければ、基本ルームの席数とするべき
					finalDesiredMemberMaxSeats = numSeatsInMemberAllBasicRooms()
				}
			} else {
				finalDesiredMemberMaxSeats = snapshotMemberMaxSeats
			}
			if (
				finalDesiredGeneralMaxSeats !== snapshotGeneralMaxSeats ||
				finalDesiredMemberMaxSeats !== snapshotMemberMaxSeats
			) {
				console.log('sending request to change max_seats')
				console.log(
					`general: ${snapshotGeneralMaxSeats} => ${finalDesiredGeneralMaxSeats}`,
				)
				console.log(
					`members-only: ${snapshotMemberMaxSeats} => ${finalDesiredMemberMaxSeats}`,
				)
				await requestMaxSeatsUpdate(
					finalDesiredGeneralMaxSeats,
					finalDesiredMemberMaxSeats,
				)
			}
		}

		// TODO: レイアウト的にmaxSeatsより大きい番号の席が含まれそうであれば、それらの席は表示しない

		// リクエストが送信されたら、すぐに反映されるわけではないのでとりあえずレイアウトを用意して表示する
		// 必要分（＝r.seatsにある席は全てカバーする）だけ臨時レイアウトを追加
		const nextGeneralLayouts: RoomLayout[] = [...allRooms.generalBasicRooms] // まずは基本ルームを設定
		if (
			!fixed_max_seats_enabled &&
			snapshotGeneralMaxSeats > numSeatsInGeneralAllBasicRooms()
		) {
			let currentAddingLayoutIndex = 0
			while (
				numSeatsOfRoomLayouts(nextGeneralLayouts) < snapshotGeneralMaxSeats
			) {
				nextGeneralLayouts.push(
					allRooms.generalTemporaryRooms[currentAddingLayoutIndex],
				)
				currentAddingLayoutIndex =
					(currentAddingLayoutIndex + 1) % allRooms.generalTemporaryRooms.length
			}
		}
		setGeneralLayouts(nextGeneralLayouts)

		if (membership_enabled) {
			const nextMemberLayouts: RoomLayout[] = [...allRooms.memberBasicRooms] // まずは基本ルームを設定
			if (
				!fixed_max_seats_enabled &&
				snapshotMemberMaxSeats > numSeatsInMemberAllBasicRooms()
			) {
				let currentAddingLayoutIndex = 0
				while (
					numSeatsOfRoomLayouts(nextMemberLayouts) < snapshotMemberMaxSeats
				) {
					nextMemberLayouts.push(
						allRooms.memberTemporaryRooms[currentAddingLayoutIndex],
					)
					currentAddingLayoutIndex =
						(currentAddingLayoutIndex + 1) %
						allRooms.memberTemporaryRooms.length
				}
			}
			setMemberLayouts(nextMemberLayouts)
		}
	}

	const requestMaxSeatsUpdate = async (
		desiredGeneralMaxSeats: number,
		desiredMemberMaxSeats: number,
	) => {
		await fetcher<SetDesiredMaxSeatsResponse>(api.setDesiredMaxSeats, {
			method: 'POST',
			body: JSON.stringify({
				desired_max_seats: desiredGeneralMaxSeats,
				desired_member_max_seats: desiredMemberMaxSeats,
			}),
		})
			.then(async () => {
				console.log('request succeeded')
			})
			.catch((e) => {
				console.error('request failed', e)
			})
	}

	/**
	 * 全ページのプロパティを再構成する。
	 */
	const updatePageProps = () => {
		// take snapshot
		const snapshotGeneralLayouts = [...latestGeneralLayouts]
		const snapshotMemberLayouts = [...latestMemberLayouts]
		const snapshotGeneralSeats = [...latestGeneralSeats]
		const snapshotMemberSeats = [...latestMemberSeats]
		const snapshotCurrentPageIndex = currentPageIndex

		let sumSeatsGeneral = 0
		let sumSeatsMember = 0
		const mapFunc =
			(member_only: boolean) =>
			(layout: RoomLayout): LayoutPageProps => {
				const numSeats = layout.seats.length
				const firstSeatIdInLayout = member_only
					? sumSeatsMember + 1
					: sumSeatsGeneral + 1 // not index
				if (member_only) {
					sumSeatsMember += numSeats
				} else {
					sumSeatsGeneral += numSeats
				}
				const LastSeatIdInLayout = member_only
					? sumSeatsMember
					: sumSeatsGeneral // not index
				const usedSeatsInLayout: Seat[] = (
					member_only ? snapshotMemberSeats : snapshotGeneralSeats
				).filter(
					(seat) =>
						firstSeatIdInLayout <= seat.seat_id &&
						seat.seat_id <= LastSeatIdInLayout,
				)

				return {
					roomLayout: layout,
					firstSeatId: firstSeatIdInLayout,
					usedSeats: usedSeatsInLayout,
					display: false, // set later in this function
					memberOnly: member_only,
				}
			}

		const newGeneralPageProps: LayoutPageProps[] = snapshotGeneralLayouts.map(
			mapFunc(false),
		)

		let newPageProps: LayoutPageProps[] = [...newGeneralPageProps]
		if (latestYoutubeMembershipEnabled) {
			const newMemberPageProps: LayoutPageProps[] = snapshotMemberLayouts.map(
				mapFunc(true),
			)
			newPageProps = newGeneralPageProps.concat(newMemberPageProps)
		}

		const pageIndexToDisplay =
			newPageProps.length > snapshotCurrentPageIndex
				? snapshotCurrentPageIndex
				: 0
		for (let i = 0; i < newPageProps.length; i++) {
			if (i === pageIndexToDisplay) {
				newPageProps[i].display = true
			} else {
				newPageProps[i].display = false
			}
		}
		setPageProps(newPageProps)

		if (currentPageIndex >= newPageProps.length) {
			setCurrentPageIndex(0)
		}
	}

	const messageMemo = useMemo(
		() => (
			<Message
				currentPageIndex={currentPageIndex}
				currentPagesLength={pageProps.length}
				currentPageIsMember={
					pageProps.length > 0 && currentPageIndex < pageProps.length
						? pageProps[currentPageIndex].memberOnly
						: false
				}
				seats={latestGeneralSeats.concat(latestMemberSeats)}
			/>
		),
		[
			currentPageIndex,
			latestGeneralLayouts,
			latestGeneralSeats,
			latestMemberSeats,
			pageProps,
		],
	)

	if (pageProps.length > 0) {
		return (
			<>
				<div css={mainContent}>
					{layoutPagesMemo}
					{messageMemo}
				</div>
			</>
		)
	}
	return <CenterLoading />
}

export default Seats
