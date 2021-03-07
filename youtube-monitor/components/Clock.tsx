import React from "react";
import styles from './Clock.module.sass'


class Clock extends React.Component<{}, any> {
  private intervalId: NodeJS.Timeout | undefined;

  constructor(props: {}) {
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
      <div id={styles.clock}>
        <div className={styles.dateString}>
          {this.state.now.getFullYear()} 年 {' '}
          {this.state.now.getMonth()} 月 {' '}
          {this.state.now.getDate()} 日
        </div>
        <div className={styles.timeString}>
          {this.state.now.getHours()}
          ：
          {(this.state.now.getMinutes() < 10)
              ? ('0' + this.state.now.getMinutes().toString())
              : (this.state.now.getMinutes())}
        </div>
      </div>
    );
  }
}

export default Clock;
