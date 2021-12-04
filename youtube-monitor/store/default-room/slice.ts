import { createSlice, PayloadAction } from "@reduxjs/toolkit";
import { SeatsState } from "../../types/api";

export const initialState: SeatsState = {
  seats: [],
};

const defaultRoomSlice = createSlice({
  name: "defaultRoom",
  initialState,
  reducers: {
    updateState: (
      state: SeatsState,
      action: PayloadAction<SeatsState>
    ) => action.payload,
  },
});

export default defaultRoomSlice;
