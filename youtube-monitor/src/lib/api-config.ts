import { CHANNEL_GL, DEBUG } from './constants'

// TODO: read url from .env
const prodApi = {
    setDesiredMaxSeats:
        'https://r2zodj0jb4.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
}

const testApi = {
    setDesiredMaxSeats:
        'https://1goygd82bk.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
}

const glApi = {
    setDesiredMaxSeats:
        'https://e014jdu68e.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
}

const api = CHANNEL_GL ? glApi : DEBUG ? testApi : prodApi

export default api
