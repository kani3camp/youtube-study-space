import React, { useState, useEffect, useReducer } from "react";

import * as styles from "./DefaultRoom.styles";


import { RoomLayout } from "../types/room-layout";
import { SeatsState, Seat } from "../types/room-state";
import api from "../lib/api_config";
import fetcher from "../lib/fetcher";
import { RoomsStateResponse } from "../types/room-state";
import { bindActionCreators } from "redux";
import Message from "../components/Message";
import { basicRooms, numSeatsInAllBasicRooms, temporaryRooms } from "../rooms/basic-rooms-config";
import DefaultRoomLayout from "./DefaultRoomLayout";
const DefaultRoom = () => {
  const PAGING_INTERVAL_MSEC = 15 * 1000

  const [seatsState, setSeatsState] = useState<SeatsState | undefined>(undefined)
  // const [displayRoomLayout, setDisplayRoomLayout] = useState<RoomLayout>()
  const [displayRoomIndex, setDisplayRoomIndex] = useState<number>(0)
  const [firstDisplaySeatId, setFirstDisplaySeatId] = useState<number>(0)
  const [displaySeats, setDisplaySeats] = useState<Seat[]>([])
  const [initialized, setInitialized] = useState<boolean>(false)
  const [roomLayouts, setRoomLayouts] = useState<RoomLayout[]>([])

  useEffect(() => {
    console.log('useEffect')
    if (!initialized) {
      init()
    } else {
      const now = new Date()
      // if (canRefreshPage) {
      updateDisplayRoom(roomLayouts, seatsState)
      // }
    }
  }, [initialized, seatsState, roomLayouts]);

  const init = async () => {
    // まず基本ルームを設定
    setRoomLayouts(basicRooms.roomLayouts)

    // seats取得
    await fetcher<RoomsStateResponse>(api.roomsState)
      .then((r) => {
        setSeatsState(r.default_room)
        
        // もしmax_seatsが基本ルームの席数より多ければ、臨時ルームを増やす
        if (r.max_seats >= numSeatsInAllBasicRooms()) {
          // 必要な分だけ臨時ルームを設定していく
          let temporaryRoomLayouts: RoomLayout[] = []
          let currentNumAllSeats: number = numSeatsInAllBasicRooms()
          let currentAddingTemporaryRoomIndex = 0
          while(currentNumAllSeats < r.max_seats) {
            temporaryRoomLayouts.push(temporaryRooms.roomLayouts[currentAddingTemporaryRoomIndex])
            currentNumAllSeats += temporaryRooms.roomLayouts[currentAddingTemporaryRoomIndex].seats.length
            currentAddingTemporaryRoomIndex++
          }
          setRoomLayouts(roomLayouts.concat(temporaryRoomLayouts))
          // max_seats = 全ルームの席数となるようにリクエスト
          // TODO
        } else {  // 少なければ、max_seats = 基本ルームの席数となるようにリクエスト
          // TODO
        }

      })
      .catch((err) => console.error(err));
    
    const fetchIntervalId = setInterval(() => {
      console.log('fetching')
      fetcher<RoomsStateResponse>(api.roomsState)
        .then((r) => {
          setSeatsState(r.default_room)
        })
        .catch((err) => console.error(err));
    }, PAGING_INTERVAL_MSEC);

    setInitialized(true)
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
      
      // 次に表示するルームの最初と最後の席の番号を求める
      let firstSeatId = 0
      for (let i=0; i<nextDisplayRoomIndex; i++) {
        firstSeatId += roomLayouts[i].seats.length
      }
      // const lastSeatId = (firstSeatId + roomLayouts[nextDisplayRoomIndex].seats.length) - 1
      
      // // 次に表示するルームの席リストを求める
      // let nextDisplaySeats: Seat[] = []
      // for (const seat of seatsState.seats) {
      //   if (firstSeatId <= seat.seat_id && seat.seat_id <= lastSeatId) {
      //     nextDisplaySeats.push(seat)
      //   }
      // }
      
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
      <div  css={styles.defaultRoom}>
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
