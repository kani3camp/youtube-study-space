import clsx from 'clsx'
import Link from '@docusaurus/Link'
import useDocusaurusContext from '@docusaurus/useDocusaurusContext'
import Layout from '@theme/Layout'
import HomepageFeatures from '@site/src/components/HomepageFeatures'
import Heading from '@theme/Heading'

import styles from './index.module.css'

function HomepageHeader() {
    const { siteConfig } = useDocusaurusContext()
    return (
        <header className={clsx('hero hero--primary', styles.heroBanner)}>
            <div className='container'>
                <Heading as='h1' className='hero__title'>
                    {siteConfig.title}
                </Heading>
                <p className='hero__subtitle'>{siteConfig.tagline}</p>
                <div className={styles.buttons}>
                    <Link className='button button--secondary button--lg' to='/docs/essential'>
                        コマンド一覧へ
                    </Link>
                </div>
            </div>
        </header>
    )
}

export default function Home(): JSX.Element {
    const { siteConfig } = useDocusaurusContext()
    return (
        <Layout
            title={siteConfig.title}
            description='YouTubeオンライン作業部屋のコマンドについて説明するサイトです。'
        >
            <HomepageHeader />
            <main>
                <div className='container'>
                    <div className={styles.buttons}>
                        <Link
                            className='button button--secondary button--lg'
                            href='https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/live'
                        >
                            YouTubeライブで入室する
                        </Link>
                    </div>
                </div>
            </main>
        </Layout>
    )
}
