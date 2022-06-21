import React, { useEffect, useState } from 'react'
import api from '../lib/api_config'
import fetcher from '../lib/fetcher'
import { allRooms, numSeatsInAllBasicRooms } from '../rooms/rooms-config'
import * as styles from '../styles/Room.styles'
import {
    RoomsStateResponse,
    Seat,
    SetDesiredMaxSeatsResponse,
} from '../types/api'
import { RoomLayout } from '../types/room-layout'
import CenterLoading from './CenterLoading'
import LayoutDisplay from './LayoutDisplay'
import Message from './Message'

const Room: React.FC = () => {
    const DATA_FETCHING_INTERVAL_MSEC = 5 * 1000
    const PAGING_INTERVAL_MSEC = 8 * 1000

    const [seats, setSeats] = useState<Seat[] | undefined>(undefined)
    const [displayRoomIndex, setDisplayRoomIndex] = useState<number>(0)
    const [firstDisplaySeatId, setFirstDisplaySeatId] = useState<number>(0)
    const [maxSeats, setMaxSeats] = useState<number>(0)
    const [initialized, setInitialized] = useState<boolean>(false)
    const [roomLayouts, setRoomLayouts] = useState<RoomLayout[]>([])
    const [lastUpdated, setLastUpdated] = useState<Date>(new Date())

    useEffect(() => {
        // console.log('useEffect')
        if (!initialized) {
            setInitialized(true)
            init()
        } else {
            updateDisplay(lastUpdated, roomLayouts, displayRoomIndex, seats)
        }
    }, [initialized, seats, roomLayouts, displayRoomIndex, lastUpdated])

    const init = async () => {
        console.log(init.name)
        await checkAndUpdateRoomLayouts()
        setInterval(async () => {
            await checkAndUpdateRoomLayouts()
        }, DATA_FETCHING_INTERVAL_MSEC)
    }

    const checkAndUpdateRoomLayouts = async () => {
        let max_seats = 0

        // seats取得
        await new Promise<void>((resolve, reject) => {
            fetcher<RoomsStateResponse>(api.roomsState)
                .then(async (r) => {
                    console.log('fetchした')
                    setSeats(r.seats)
                    setMaxSeats(r.max_seats)
                    max_seats = r.max_seats

                    // まず、現状の入室状況（seatsとmax_seats）と設定された空席率（min_vacancy_rate）を基に、適切なmax_seatsを求める。
                    let final_desired_max_seats: number
                    const min_seats_by_vacancy_rate = Math.ceil(
                        r.seats.length / (1 - r.min_vacancy_rate)
                    )
                    // console.log(
                    //     '少なくとも',
                    //     min_seats_by_vacancy_rate,
                    //     'は確定'
                    // )
                    // もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
                    if (min_seats_by_vacancy_rate > numSeatsInAllBasicRooms()) {
                        let current_num_seats: number =
                            numSeatsInAllBasicRooms()
                        let current_adding_temporary_room_index = 0
                        while (current_num_seats < min_seats_by_vacancy_rate) {
                            current_num_seats +=
                                allRooms.temporaryRooms[
                                    current_adding_temporary_room_index
                                ].seats.length
                            current_adding_temporary_room_index =
                                (current_adding_temporary_room_index + 1) %
                                allRooms.temporaryRooms.length
                        }
                        final_desired_max_seats = current_num_seats
                    } else {
                        // そうでなければ、基本ルームの席数とするべき
                        final_desired_max_seats = numSeatsInAllBasicRooms()
                    }
                    console.log(final_desired_max_seats, r.max_seats)

                    // 求めたmax_seatsが現状の値と異なったら、リクエストを送る
                    if (final_desired_max_seats !== r.max_seats) {
                        console.log(
                            'リクエストを送る',
                            final_desired_max_seats,
                            r.max_seats
                        )
                        await fetcher<SetDesiredMaxSeatsResponse>(
                            api.setDesiredMaxSeats,
                            {
                                method: 'POST',
                                body: JSON.stringify({
                                    desired_max_seats: final_desired_max_seats,
                                }),
                            }
                        ).then(async (r) => {
                            console.log('リクエストした')
                        })
                    }

                    // リクエストが送信されたら、すぐに反映されるわけではないのでとりあえずレイアウトを用意して表示する
                    const next_display_room_layouts: RoomLayout[] = [
                        ...allRooms.basicRooms,
                    ] // まずは基本ルームを設定
                    // 必要なぶんだけ臨時レイアウトを追加
                    if (max_seats > numSeatsInAllBasicRooms()) {
                        let current_adding_temporary_room_index = 0
                        while (
                            numSeatsOfRoomLayouts(next_display_room_layouts) <
                            max_seats
                        ) {
                            next_display_room_layouts.push(
                                allRooms.temporaryRooms[
                                    current_adding_temporary_room_index
                                ]
                            )
                            current_adding_temporary_room_index =
                                (current_adding_temporary_room_index + 1) %
                                allRooms.temporaryRooms.length
                        }
                    }

                    // TODO: レイアウト的にmax_seatsより大きい番号の席が含まれそうであれば、それらの席は表示しない

                    setRoomLayouts(next_display_room_layouts)
                    resolve()
                })
                .catch((err) => {
                    console.error(err)
                    reject()
                })
        })
    }

    const updateDisplay = (
        last_updated: Date,
        room_layouts: RoomLayout[],
        room_index: number,
        seats_state: Seat[] | undefined
    ) => {
        if (room_layouts && seats_state) {
            const diffMilliSecond =
                new Date().getTime() - last_updated.getTime()
            if (diffMilliSecond >= PAGING_INTERVAL_MSEC) {
                // 次に表示するルームのレイアウトのインデックスを求める
                const nextDisplayRoomIndex =
                    (room_index + 1) % room_layouts.length

                // 次に表示するルームの最初の席の番号を求める
                let firstSeatId = 0
                for (let i = 0; i < nextDisplayRoomIndex; i++) {
                    firstSeatId += room_layouts[i].seats.length
                }

                setFirstDisplaySeatId(firstSeatId)
                setDisplayRoomIndex(nextDisplayRoomIndex)
                setLastUpdated(new Date())
            }
        }
    }

    const numSeatsOfRoomLayouts = (layouts: RoomLayout[]) => {
        let count = 0
        for (const layout of layouts) {
            count += layout.seats.length
        }
        return count
    }

    if (seats) {
        return (
            <div css={styles.defaultRoom}>
                <LayoutDisplay
                    roomLayouts={roomLayouts}
                    roomIndex={displayRoomIndex}
                    seats={seats}
                    firstSeatId={firstDisplaySeatId}
                    maxSeats={maxSeats}
                ></LayoutDisplay>
                <Message
                    current_room_index={displayRoomIndex}
                    current_rooms_length={roomLayouts.length}
                    seats={seats}
                ></Message>
            </div>
        )
    } else {
        return <CenterLoading></CenterLoading>
    }
}

export default Room
