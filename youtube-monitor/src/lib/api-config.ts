import { CHANNEL_GL, DEBUG } from './constants'

type ApiConfig = {
    setDesiredMaxSeats: string
    displayShoutMessage: string
}

// TODO: read url from .env
const prodApi: ApiConfig = {
    setDesiredMaxSeats:
        'https://r2zodj0jb4.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
    displayShoutMessage:
        'https://r2zodj0jb4.execute-api.ap-northeast-1.amazonaws.com/default/display_shout_message',
}

const testApi: ApiConfig = {
    setDesiredMaxSeats:
        'https://1goygd82bk.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
    displayShoutMessage:
        'https://1goygd82bk.execute-api.ap-northeast-1.amazonaws.com/default/display_shout_message',
}

const glApi: ApiConfig = {
    setDesiredMaxSeats:
        'https://e014jdu68e.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
    displayShoutMessage:
        'https://e014jdu68e.execute-api.ap-northeast-1.amazonaws.com/default/display_shout_message',
}

const api: ApiConfig = CHANNEL_GL ? glApi : DEBUG ? testApi : prodApi

export default api
