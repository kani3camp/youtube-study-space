import React from "react";
import styles from "./BackgroundImage.module.sass";
import next from "next";
import { getCurrentSection } from "../lib/time_table";



class BackgroundImage extends React.Component<{}, any> {
  private intervalId: NodeJS.Timeout | undefined;
  private base_url: string = 'https://source.unsplash.com/featured/1920x1080'
  private args: string = '/?' + 'work,cafe,study,nature,chill,coffee,sea,lake'
  private unsplash_url = this.base_url + this.args

  updateState() {
    const now = new Date()
    const currentSection = getCurrentSection()

    if (currentSection?.partName !== this.state.lastPartName) {
        this.setState({
            srcUrl: this.unsplash_url + ',' + now.getTime(),
            lastFetchedDate: now.getDate(),
            lastPartName: currentSection?.partName,
        })
    }
  }

  constructor(props: {}) {
    super(props);
    this.state = {
        srcUrl: this.unsplash_url,
        lastFetchedDate: new Date().getDate(),
        lastPartName: '',
    }
  }

  componentDidMount() {
    this.intervalId = setInterval(() => {
      this.updateState()
    }, 1000);
  }

  componentWillUnmount() {
    if (this.intervalId) {
      clearInterval(this.intervalId);
    }
  }



  render() {
    return (
        <div id={styles.backgroundImage}>
            <img src={this.state.srcUrl} alt="背景画像" />
        </div>
    )
  }
}

export default BackgroundImage;
