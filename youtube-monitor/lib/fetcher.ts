// export async function fetcher(url: string): Promise<boolean | null> {
//   const response = await fetch(url);
//   return response.json();
// }

// Tはレスポンスのjsonの型を指定する
const wrap = <T>(task: Promise<Response>): Promise<T> => {
  return new Promise((resolve, reject) => {
    task
      .then((response) => {
        if (response.ok) {
          response
            .json()
            .then((json) => {
              // jsonが取得できた場合だけresolve
              resolve(json);
            })
            .catch((error) => {
              reject(error);
            });
        } else {
          reject(response);
        }
      })
      .catch((error) => {
        reject(error);
      });
  });
};

const fetcher = <T = any>(
  input: RequestInfo,
  init?: RequestInit
): Promise<T> => {
  return wrap<T>(fetch(input, init));
};

export default fetcher;
