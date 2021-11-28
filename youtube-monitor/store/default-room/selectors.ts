import { useSelector } from "react-redux";
import { SeatsState } from "../../types/api";

// TODO: これ使ってる？
export const useCounterState = () => {
  return useSelector((state: { defaultRoom: SeatsState }) => state);
};
