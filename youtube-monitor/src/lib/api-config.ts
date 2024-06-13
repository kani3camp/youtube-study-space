import { CHANNEL_GL, DEBUG } from './constants'

const prodApi = {
    setDesiredMaxSeats:
        'https://1wzzml51kl.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
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
