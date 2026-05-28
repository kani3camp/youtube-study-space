package mypage

import "errors"

var (
	ErrUnauthorized                = errors.New("unauthorized")
	ErrInvalidRequest              = errors.New("invalid request")
	ErrInvalidIdentity             = errors.New("invalid identity")
	ErrYouTubeLinkRequired         = errors.New("youtube link required")
	ErrInvalidYouTubeAccessToken   = errors.New("invalid youtube access token")
	ErrYouTubeChannelAlreadyLinked = errors.New("youtube channel already linked")
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}
