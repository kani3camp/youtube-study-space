import { globalStyle } from "../styles/global.styles";
import { Global } from "@emotion/react";
import { AppProps } from "next/app";

export default function App({ Component, pageProps }: AppProps) {
  if (process.env.NEXT_PUBLIC_API_KEY === undefined) {
    console.error("環境変数NEXT_PUBLIC_API_KEYが定義されていません");
  }
  return (
    <>
      <Global styles={globalStyle} />
      <Component {...pageProps} />
    </>
  );
}
