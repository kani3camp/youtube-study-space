import { DefaultRoomState } from "../store/default-room/slice";

export type DefaultRoomState = {
  seats: Seat[];
};

export type NoSeatRoomState = {
  seats: Seat[];
};

export type Seat = {
  seat_id: number;
  user_id: string;
  user_display_name: string;
  work_name: string;
  entered_at: Date;
  until: Date;
  color_code: string;
};

// TODO: lambda側とあってるか確認
export type RoomsStateResponse = {
  result: string;
  message: string;
  default_room: DefaultRoomState;
  max_seats: number;
};
