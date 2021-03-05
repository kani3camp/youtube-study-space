import React from "react";

type ClockState = {
  state: Date;
};

class Clock extends React.Component<{}, any> {
  private intervalId: NodeJS.Timeout | undefined;

  constructor(props: ClockState) {
    super(props);
    this.state = {
      now: new Date(),
    };
  }

  componentDidMount() {
    this.intervalId = setInterval(() => {
      this.setState({
        now: new Date(),
      });
    }, 1000);
  }

  componentWillUnmount() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
    }
  }

  render() {
    return (
      <div>
        <div>
          {this.state.now.getFullYear()} 年 {this.state.now.getMonth()} 月{" "}
          {this.state.now.getDate()} 日
        </div>
        <div>
          {this.state.now.getHours()}：{this.state.now.getMinutes()}
        </div>
        <div>{this.state.now.getSeconds()} 秒</div>
      </div>
    );
  }
}

export default Clock;
