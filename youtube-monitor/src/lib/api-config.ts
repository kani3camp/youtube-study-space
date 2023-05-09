import { DEBUG } from './constants'

const prodApi = {
    setDesiredMaxSeats:
        'https://1wzzml51kl.execute-api.ap-northeast-1.amazonaws.com/default/set_desired_max_seats',
}

const testApi = {
    setDesiredMaxSeats:
        'https://q8ff9jqwef.execute-api.us-east-1.amazonaws.com/default/set_desired_max_seats',
}

const api = DEBUG ? testApi : prodApi

export default api
