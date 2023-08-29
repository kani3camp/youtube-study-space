import { DEBUG } from './constants'

const prodApi = {
    setDesiredMaxSeats:
        'https://1wzzml51kl.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
}

const testApi = {
    setDesiredMaxSeats:
        'https://mmlcz4c490.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
}

const api = DEBUG ? testApi : prodApi

export default api
