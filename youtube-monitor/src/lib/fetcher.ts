// Tはレスポンスのjsonの型を指定する
const wrap = <T>(task: Promise<Response>): Promise<T> =>
    new Promise((resolve, reject) => {
        task.then((response) => {
            if (response.ok) {
                response
                    .json()
                    .then((json) => {
                        resolve(json)
                    })
                    .catch((error) => {
                        reject(error)
                    })
            } else {
                reject(response)
            }
        }).catch((error) => {
            reject(error)
        })
    })

const fetcher = <T = any>(url: RequestInfo, init: RequestInit = {}): Promise<T> => {
    const requestHeaders: HeadersInit = new Headers()
    const apiKey = process.env.NEXT_PUBLIC_API_KEY
    if (apiKey === undefined) {
        throw new Error('process.env.NEXT_PUBLIC_API_KEY is not defined.')
    }
    requestHeaders.append('x-api-key', apiKey)
    init.headers = requestHeaders
    return wrap<T>(fetch(url, init))
}

export default fetcher
