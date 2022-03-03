import React, { FC } from "react";
import Clock from "../components/Clock";
import Room from "../components/Room";
import StandingRoom from "../components/StandingRoom";
import Timer from "../components/Timer";
import BackgroundImage from "../components/BackgroundImage";
import BgmPlayer from "../components/BgmPlayer";

const Home: FC = () => {
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
      <Room />

      <StandingRoom />
      <Timer />
    </div>
  );
};

export default Home;
