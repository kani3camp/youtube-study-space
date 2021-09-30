import { DefaultRoomState } from "../store/default-room/slice";

export type DefaultRoomState = {
  seats: seat[];
};

export type NoSeatRoomState = {
  seats: seat[];
};

export type seat = {
  seat_id: number;
  user_id: string;
  user_display_name: string;
  work_name: string;
  entered_at: Date;
  until: Date;
  color_code: string;
};

export type RoomsStateResponse = {
  result: string;
  message: string;
  default_room: DefaultRoomState;
  no_seat_room: NoSeatRoomState;
  default_room_layout: RoomLayout;
};
