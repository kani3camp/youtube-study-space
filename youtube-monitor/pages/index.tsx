import React from "react";
import Clock from "../components/Clock";
import Message from "../components/Message";
import DefaultRoom from "../components/DefaultRoom";
import StandingRoom from "../components/StandingRoom";
import Timer from "../components/Timer";
import BackgroundImage from "../components/BackgroundImage";
import fetcher from "../lib/fetcher";
import { RoomsStateResponse, seat } from "../types/room-state";
import BgmPlayer from "../components/BgmPlayer";
import api from "../lib/api_config";


export default class Home extends React.Component<{}, any> {
  private intervalId: NodeJS.Timeout | undefined;

  constructor(props: any) {
    super(props);
    this.state = {
      layout: null,
      default_room_state: null,
      no_seat_room_state: null,
    };
  }

  componentDidMount () {
    const component = this;
    this.intervalId = setInterval(() => {
      fetcher<RoomsStateResponse>(api.roomsState)
        .then((r) => {
          // r.default_room.seats.forEach((item: seat) =>
          //   console.log(item.seat_id, item.user_display_name)
          // );
          // console.log("fetch完了");
          component.setState({
            layout: r.default_room_layout,
            default_room_state: r.default_room,
            no_seat_room_state: r.no_seat_room,
          });
        })
        .catch((err) => console.error(err));
    }, 2000);
  }

  componentWillUnmount () {
    if (this.intervalId) {
      clearInterval(this.intervalId);
    }
  }

  render () {
    return (
      <div
        style={{
          height: 1080,
          width: 1920,
          margin: 0,
          position: "relative",
        }}
      >
        <BackgroundImage></BackgroundImage>
        <BgmPlayer></BgmPlayer>
        <Clock />
        <Message
          default_room_state={this.state.default_room_state}
          no_seat_room_state={this.state.no_seat_room_state}
        />
        <DefaultRoom
          layout={this.state.layout}
          roomState={this.state.default_room_state}
        />
        <StandingRoom no_seat_room_state={this.state.no_seat_room_state} />
        <Timer />
      </div>
    );
  }
}
