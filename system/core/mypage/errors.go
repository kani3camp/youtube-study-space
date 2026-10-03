package mypage

import "errors"

var (
	ErrUnauthorized                = errors.New("unauthorized")
	ErrInvalidRequest              = errors.New("invalid request")
	ErrInvalidIdentity             = errors.New("invalid identity")
	ErrYouTubeLinkRequired         = errors.New("youtube link required")
	ErrInvalidYouTubeAccessToken   = errors.New("invalid youtube access token")
	ErrYouTubeChannelAlreadyLinked = errors.New("youtube channel already linked")
	ErrYouTubeChannelNotFound      = errors.New("youtube channel not found")
	ErrYouTubeForbidden            = errors.New("youtube request forbidden")
	ErrYouTubeRateLimited          = errors.New("youtube rate limited")
	ErrYouTubeUpstream             = errors.New("youtube upstream error")
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}
