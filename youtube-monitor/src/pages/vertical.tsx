import type { GetStaticProps } from 'next'
import { serverSideTranslations } from 'next-i18next/pages/serverSideTranslations'
import type { FC } from 'react'
import VerticalMonitor from '../components/monitor/VerticalMonitor'

const Vertical: FC = () => <VerticalMonitor />

export const getStaticProps: GetStaticProps = async ({ locale }) => ({
	props: {
		...(await serverSideTranslations(locale ?? 'ja', ['common'])),
	},
})

export default Vertical
