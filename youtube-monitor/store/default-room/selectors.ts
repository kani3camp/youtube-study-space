import { useSelector } from "react-redux";
import { DefaultRoomState } from "../../types/room-state";

export const useCounterState = () => {
  return useSelector((state: { defaultRoom: DefaultRoomState }) => state);
};
