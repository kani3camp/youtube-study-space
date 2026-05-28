export const env = {
	mypageApiBaseUrl: import.meta.env.VITE_MYPAGE_API_BASE_URL ?? '',
	useMock: import.meta.env.VITE_USE_MOCK === 'true',
	clientVersion: import.meta.env.VITE_CLIENT_VERSION ?? 'local',
	clientBuildTime: import.meta.env.VITE_CLIENT_BUILD_TIME ?? '',
} as const
