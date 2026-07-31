export class MissingYouTubeAccessTokenError extends Error {
	constructor() {
		super('YouTube access token is not available')
		this.name = 'MissingYouTubeAccessTokenError'
	}
}
