import { Store, combineReducers } from "redux";
import logger from "redux-logger";
import { configureStore, getDefaultMiddleware } from "@reduxjs/toolkit";
import defaultRoomSlice, {
  initialState as defaultRoomState,
} from "./default-room/slice";

const rootReducer = combineReducers({
  defaultRoom: defaultRoomSlice.reducer,
});

const preloadedState = () => {
  return {
    defaultRoom: defaultRoomState,
  };
};

export type StoreState = ReturnType<typeof preloadedState>;

export type ReduxStore = Store<StoreState>;

const createStore = () => {
  const middlewareList = [...getDefaultMiddleware(), logger];

  return configureStore({
    reducer: rootReducer,
    middleware: middlewareList,
    devTools: process.env.NODE_ENV !== "production",
    preloadedState: preloadedState(),
  });
};

export default createStore;
