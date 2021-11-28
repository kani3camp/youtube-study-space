import React, { useState, useEffect, useReducer } from "react";

import * as styles from "./DefaultRoom.styles";


import { RoomLayout } from "../types/room-layout";
import { SeatsState, Seat, SetDesiredMaxSeatsResponse } from "../types/api";
import api from "../lib/api_config";
import fetcher from "../lib/fetcher";
import { RoomsStateResponse } from "../types/api";
import { bindActionCreators } from "redux";
import Message from "../components/Message";
import { basicRooms, numSeatsInAllBasicRooms, temporaryRooms } from "../rooms/basic-rooms-config";
import DefaultRoomLayout from "./DefaultRoomLayout";
const DefaultRoom = () => {
  const PAGING_INTERVAL_MSEC = 10 * 1000

  const [seatsState, setSeatsState] = useState<SeatsState | undefined>(undefined)
  // const [displayRoomLayout, setDisplayRoomLayout] = useState<RoomLayout>()
  const [displayRoomIndex, setDisplayRoomIndex] = useState<number>(0)
  const [firstDisplaySeatId, setFirstDisplaySeatId] = useState<number>(0)
  const [displaySeats, setDisplaySeats] = useState<Seat[]>([])
  const [maxSeats, setMaxSeats] = useState<number>(0)
  const [initialized, setInitialized] = useState<boolean>(false)
  const [roomLayouts, setRoomLayouts] = useState<RoomLayout[]>([])

  useEffect(() => {
    console.log('useEffect')
    if (!initialized) {
      setInitialized(true)
      init()
    }
  }, [initialized, seatsState, roomLayouts]);

  const init = async () => {
    console.log(init.name)
    await checkAndUpdateRoomLayouts()
    const fetchIntervalId = setInterval(async () => {
      await checkAndUpdateRoomLayouts()
      updateDisplayRoom(roomLayouts, seatsState)
    }, PAGING_INTERVAL_MSEC);
    // const updateDisplayIntervalId = setInterval(async () => {
    //   await updateDisplayRoom(roomLayouts, seatsState)
    // }, )
  }
  
  const checkAndUpdateRoomLayouts = async () => {
    let seats_state: SeatsState
    let max_seats: number
    
    console.log(checkAndUpdateRoomLayouts.name)
    // seats取得
    await fetcher<RoomsStateResponse>(api.roomsState)
      .then(async (r) => {
        setSeatsState(r.default_room)
        seats_state = r.default_room
        setMaxSeats(r.max_seats)
        max_seats = r.max_seats
        
        // まず、現状の入室状況（seatsとmax_seats）と設定された空席率（min_vacancy_rate）を基に、適切なmax_seatsを求める。
        let final_desired_max_seats: number
        const min_seats_by_vacancy_rate = Math.ceil(r.default_room.seats.length / r.min_vacancy_rate)
        // もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
        if (min_seats_by_vacancy_rate > numSeatsInAllBasicRooms()) {
          let current_num_seats: number = numSeatsInAllBasicRooms()
          let current_adding_temporary_room_index = 0
          while(current_num_seats < min_seats_by_vacancy_rate) {
            current_num_seats += temporaryRooms.roomLayouts[current_adding_temporary_room_index].seats.length
            current_adding_temporary_room_index++
          }
          final_desired_max_seats = current_num_seats
        } else {  // そうでなければ、基本ルームの席数とするべき
          final_desired_max_seats = numSeatsInAllBasicRooms()
        }
        
        // 求めたmax_seatsが現状の値と異なったら、リクエストを送る
        if (final_desired_max_seats !== r.max_seats) {
          await fetcher<SetDesiredMaxSeatsResponse>(api.setDesiredMaxSeats, {
            method: 'POST',
            body: JSON.stringify({
              desired_max_seats: final_desired_max_seats,
            })
          }).then(async (r) => {
            console.log('リクエストした')
          })
        }
        
        // リクエストが送信されたら、すぐに反映されるわけではないのでとりあえずレイアウトを用意して表示する
        let next_display_room_layouts: RoomLayout[] = basicRooms.roomLayouts  // まずは基本ルームを設定
        // 必要なぶんだけ臨時レイアウトを追加
        if (max_seats > numSeatsInAllBasicRooms()) {
          let current_adding_temporary_room_index = 0
          while (numSeatsOfRoomLayouts(next_display_room_layouts) < max_seats) {
            next_display_room_layouts.concat(temporaryRooms.roomLayouts[current_adding_temporary_room_index])
            current_adding_temporary_room_index++
          }
        }
        
        // レイアウト的にmax_seatsより大きい番号の席が含まれそうであれば、それらの席は表示しない
        
        // TODO: ルーム数が減るときは、displayRoomIndexは確認したほうがいいかも
        setRoomLayouts(next_display_room_layouts)

      })
      .catch((err) => console.error(err));
  }
  
  const numSeatsOfRoomLayouts = (layouts: RoomLayout[]) => {
    let count = 0
    for (const layout of layouts) {
      count += layout.seats.length
    }
    return count
  }

  const maxSeatIndex = (seats: Seat[]): number => {
    let maxSeatIndex = 0
    seats.forEach((each_seat: Seat) => {
      if (each_seat.seat_id > maxSeatIndex) {
        maxSeatIndex = each_seat.seat_id
      }
    })
    return maxSeatIndex
  }

  const updateDisplayRoom = (roomLayouts: RoomLayout[], seatsState: SeatsState | undefined) => {
    if (seatsState && seatsState?.seats.length > 0) {
      // 次に表示するルームのレイアウトのインデックスを求める
      const nextDisplayRoomIndex = (displayRoomIndex + 1) % roomLayouts.length
      
      // 次に表示するルームの最初の席の番号を求める
      let firstSeatId = 0
      for (let i=0; i<nextDisplayRoomIndex; i++) {
        firstSeatId += roomLayouts[i].seats.length
      }
            
      setFirstDisplaySeatId(firstSeatId)
      setDisplayRoomIndex(nextDisplayRoomIndex)
      console.log('完了')
    } else {
      setFirstDisplaySeatId(0)
      setDisplayRoomIndex(0)
    }
  }

  if (seatsState) {
    return (
      <div css={styles.defaultRoom}>
        <DefaultRoomLayout roomLayout={roomLayouts[displayRoomIndex]} seats={seatsState.seats} firstSeatId={firstDisplaySeatId}>
        </DefaultRoomLayout>
        <Message
          default_room_state={seatsState}
        ></Message>
      </div>
    );
  } else {
    return <div>Loading</div>;
  }


}

export default DefaultRoom;
