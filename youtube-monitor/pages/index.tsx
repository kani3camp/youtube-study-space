import React from "react";
import Clock from "../components/Clock";
import Message from "../components/Message";
import DefaultRoom from "../components/DefaultRoom";
import StandingRoom from "../components/StandingRoom";

export default function Home() {
  return (
    <div style={{height: 1080, width: 1920, backgroundColor: "pink", margin: 0, position: "relative"}}>
      <Clock />
      <Message />
      <DefaultRoom/>
      <StandingRoom/>
    </div>
  );
}
