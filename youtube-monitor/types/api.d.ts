
export type SeatsState = {
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

export type RoomsStateResponse = {
  result: string;
  message: string;
  default_room: SeatsState;
  max_seats: number;
  min_vacancy_rate: number;
};




export type SetDesiredMaxSeatsResponse = {
  result: string;
  message: string;
};
