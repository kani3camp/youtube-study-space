import Link from '@docusaurus/Link';
import Translate, { translate } from '@docusaurus/Translate';
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';
import clsx from 'clsx';

import styles from './index.module.css';

function HomepageHeader() {
	return (
		<header className={clsx('hero hero--primary', styles.heroBanner)}>
			<div className="container">
				<Heading as="h1" className="hero__title">
					<Translate id="homepage.title" description="The title of the homepage">
						YouTubeオンライン作業部屋 コマンド一覧
					</Translate>
				</Heading>
				<p className="hero__subtitle">
					<Translate id="homepage.tagline" description="The tagline of the homepage">
						ライブチャットに書き込もう
					</Translate>
				</p>
				<div className={styles.buttons}>
					<Link className="button button--secondary button--lg" to="/docs/essential">
						<Translate id="homepage.commandList" description="Command list button text">
							コマンド一覧へ
						</Translate>
					</Link>
				</div>
			</div>
		</header>
	);
}

export default function Home(): JSX.Element {
	const title = translate({
		id: 'homepage.title',
		message: 'YouTubeオンライン作業部屋 コマンド一覧',
		description: 'The title of the homepage',
	});

	return (
		<Layout
			title={title}
			description={translate({
				id: 'homepage.description',
				message: 'YouTubeオンライン作業部屋のコマンドについて説明するサイトです。',
				description: 'The description of the homepage',
			})}
		>
			<HomepageHeader />
			<main>
				<div className="container">
					<div className={styles.buttons}>
						<Link
							className="button button--secondary button--lg"
							href="https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/live"
						>
							<Translate id="homepage.enterYouTube" description="Enter YouTube button text">
								YouTubeライブで入室する
							</Translate>
						</Link>
					</div>
				</div>
			</main>
		</Layout>
	);
}
