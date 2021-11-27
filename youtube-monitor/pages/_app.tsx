import "../styles/global.sass";
import { AppProps } from "next/app";
import createStore from "../store/createStore";
import { Provider } from "react-redux";

export default function App({ Component, pageProps }: AppProps) {
  
  return (
    <Provider store={createStore()}>
      <Component {...pageProps} />
    </Provider>
  );
}
