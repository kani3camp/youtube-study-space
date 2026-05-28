export type StageName = 'dev' | 'prod'

export type StageConfig = {
	stage: StageName
	mypageAllowedOrigin: string
	mypageApiFunctionName: string
	mypageApiName: string
}

export type StageConfigOptions = {
	mypageAllowedOrigin?: string
}

const resolveAllowedOrigin = (
	stage: StageName,
	options?: StageConfigOptions,
): string => {
	if (options?.mypageAllowedOrigin) {
		return options.mypageAllowedOrigin
	}

	if (stage === 'dev') {
		return 'http://localhost:18081'
	}

	throw new Error(
		'Context value "mypageAllowedOrigin" is required for prod stage',
	)
}

export const getStageConfig = (
	stage: string | undefined,
	options?: StageConfigOptions,
): StageConfig => {
	switch (stage) {
		case 'prod':
			return {
				stage: 'prod',
				mypageAllowedOrigin: resolveAllowedOrigin('prod', options),
				mypageApiFunctionName: 'mypage_api',
				mypageApiName: 'mypage-api',
			}

		case 'dev':
		case undefined:
		case '':
			return {
				stage: 'dev',
				mypageAllowedOrigin: resolveAllowedOrigin('dev', options),
				mypageApiFunctionName: 'dev_mypage_api',
				mypageApiName: 'dev-mypage-api',
			}

		default:
			throw new Error(`Unknown stage: ${stage}`)
	}
}
