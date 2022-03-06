import React from 'react'
import * as styles from '../styles/Clock.styles'

class Clock extends React.Component<Record<string, unknown>, any> {
  private intervalId: NodeJS.Timeout | undefined

  constructor(props: Record<string, unknown>) {
    super(props)
    this.state = {
      now: new Date(),
    }
  }

  componentDidMount() {
    this.intervalId = setInterval(() => {
      this.setState({
        now: new Date(),
      })
    }, 1000)
  }

  componentWillUnmount() {
    if (this.intervalId) {
      clearInterval(this.intervalId)
    }
  }

  render() {
    return (
      <div css={styles.clockStyle}>
        <div css={styles.dateStringStyle}>
          {/*{this.state.now.getMonth() + 1} / {this.state.now.getDate()} /{" "}*/}
          {/*{this.state.now.getFullYear()}*/}
          {this.state.now.getFullYear()} 年 {this.state.now.getMonth() + 1} 月{' '}
          {this.state.now.getDate()} 日
        </div>
        <div css={styles.timeStringStyle}>
          {this.state.now.getHours()}：
          {this.state.now.getMinutes() < 10
            ? `0${this.state.now.getMinutes().toString()}`
            : this.state.now.getMinutes()}
        </div>
      </div>
    )
  }
}

export default Clock
