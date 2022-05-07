import React from 'react'
import * as styles from '../styles/BackgroundImage.styles'
import next from 'next'
import { getCurrentSection } from '../lib/time_table'

class BackgroundImage extends React.Component<Record<string, unknown>, any> {
  private intervalId: NodeJS.Timeout | undefined
  private base_url = 'https://source.unsplash.com/featured/1920x1080'
  private args: string =
    '/?' +
    'work,cafe,study,nature,chill,coffee,tea,sea,lake,outdoor,land,spring,summer,fall,winter,hotel' +
    ',green,purple,pink,blue,dark,azure,yellow,orange,gray,brown,red,black,pastel' +
    ',blossom,flower,corridor,door,background,wood,resort,travel,vacation,beach,grass' +
    ',pencil,pen,eraser,stationary,classic,jazz,lo-fi,fruit,vegetable,sky,instrument,cup' + 
    ',star,moon,night,cloud,rain,mountain,river,calm,sun,sunny,water,building,drink,keyboard' + 
    ',morning,evening'
  private unsplash_url = this.base_url + this.args

  updateState() {
    const now = new Date()
    const currentSection = getCurrentSection()

    if (currentSection?.partType !== this.state.lastPartName) {
      this.setState({
        srcUrl: `${this.unsplash_url},${now.getTime()}`,
        lastFetchedDate: now.getDate(),
        lastPartName: currentSection?.partType,
      })
    }
  }

  constructor(props: Record<string, unknown>) {
    super(props)
    this.state = {
      srcUrl: this.unsplash_url,
      lastFetchedDate: new Date().getDate(),
      lastPartName: '',
    }
  }

  componentDidMount() {
    this.intervalId = setInterval(() => {
      this.updateState()
    }, 1000)
  }

  componentWillUnmount() {
    if (this.intervalId) {
      clearInterval(this.intervalId)
    }
  }

  render() {
    return (
      <div css={styles.backgroundImage}>
        <img src={this.state.srcUrl} alt='背景画像' />
      </div>
    )
  }
}

export default BackgroundImage
