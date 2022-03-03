import { css } from "@emotion/react";
import { Constants } from "../lib/constants";

export const globalStyle = css`
  html {
    font-family: ${Constants.fontFamily};
    font-size: xx-large;
  }

  body {
    margin: 0;
  }
`;
