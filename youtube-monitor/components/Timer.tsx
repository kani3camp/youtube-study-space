import React from "react";
import styles from "./Timer.module.sass";
import {TimeSection, SectionType, remainingTime, getCurrentSection} from "../lib/time_table";
import next from "next";



class Timer extends React.Component<{}, any> {
  private intervalId: NodeJS.Timeout | undefined;

  constructor(props: {}) {
    super(props);
    const now: Date = new Date()
    const currentSection: TimeSection | null = getCurrentSection()
    if (currentSection !== null) {
      console.log('現在のセクション：', currentSection)
      const remaining_min: number = remainingTime(now.getHours(), now.getMinutes(), currentSection.ends.h, currentSection.ends.m)
      const nextSection: string = (currentSection.sectionType === SectionType.Study) ? '休憩' : '作業'
      
      this.state = {
        remaining: remaining_min,
        nextSection: nextSection
      }
    }
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
        <div id={styles.timer}>
            <h3>タイマー</h3>
            <div></div>
            <h4>の部</h4>
            <div>
              <span>次は</span>
              <span>{this.state.remaining}</span>
              <span>分</span>
              <span>{this.state.nextSection}</span>
            </div>
        </div>
    )
  }
}

export default Timer;
