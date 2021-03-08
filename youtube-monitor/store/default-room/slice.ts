import { createSlice, PayloadAction } from "@reduxjs/toolkit";
import { DefaultRoomState } from "../../types/room-state";

export const initialState: DefaultRoomState = {
  seats: [],
};

const defaultRoomSlice = createSlice({
  name: "defaultRoom",
  initialState,
  reducers: {
    updateState: (
      state: DefaultRoomState,
      action: PayloadAction<DefaultRoomState>
    ) => action.payload,
  },
});

export default defaultRoomSlice;
