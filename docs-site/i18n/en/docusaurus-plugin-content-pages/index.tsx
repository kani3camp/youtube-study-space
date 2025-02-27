import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Translate, { translate } from '@docusaurus/Translate';
import Heading from '@theme/Heading';
import Layout from '@theme/Layout';
import clsx from 'clsx';

import styles from './index.module.css';

function HomepageHeader() {
	const { siteConfig } = useDocusaurusContext();
	return (
		<header className={clsx('hero hero--primary', styles.heroBanner)}>
			<div className="container">
				<Heading as="h1" className="hero__title">
					<Translate id="homepage.title" description="The title of the homepage">
						YouTube Study Space Commands
					</Translate>
				</Heading>
				<p className="hero__subtitle">
					<Translate id="homepage.tagline" description="The tagline of the homepage">
						Type in the live chat
					</Translate>
				</p>
				<div className={styles.buttons}>
					<Link className="button button--secondary button--lg" to="/docs/essential">
						<Translate id="homepage.commandList" description="Command list button text">
							Command List
						</Translate>
					</Link>
				</div>
			</div>
		</header>
	);
}

export default function Home(): JSX.Element {
	const { siteConfig } = useDocusaurusContext();
	const title = translate({
		id: 'homepage.title',
		message: 'YouTube Study Space Commands',
		description: 'The title of the homepage',
	});

	return (
		<Layout
			title={title}
			description={translate({
				id: 'homepage.description',
				message: 'A site explaining the commands for YouTube Online Study Space.',
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
								Enter via YouTube Live
							</Translate>
						</Link>
					</div>
				</div>
			</main>
		</Layout>
	);
}
