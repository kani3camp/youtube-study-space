// export async function fetcher(url: string): Promise<boolean | null> {
//   const response = await fetch(url);
//   return response.json();
// }

// Tはレスポンスのjsonの型を指定する
const wrap = <T>(task: Promise<Response>): Promise<T> =>
  new Promise((resolve, reject) => {
    task
      .then((response) => {
        if (response.ok) {
          response
            .json()
            .then((json) => {
              // jsonが取得できた場合だけresolve
              resolve(json)
            })
            .catch((error) => {
              reject(error)
            })
        } else {
          reject(response)
        }
      })
      .catch((error) => {
        reject(error)
      })
  })

const fetcher = <T = any>(
  url: RequestInfo,
  init: RequestInit = {}
): Promise<T> => {
  const requestHeaders: HeadersInit = new Headers()
  requestHeaders.append('x-api-key', process.env.NEXT_PUBLIC_API_KEY!)
  init.headers = requestHeaders
  return wrap<T>(fetch(url, init))
}

export default fetcher
