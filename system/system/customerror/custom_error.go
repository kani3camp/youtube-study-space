package customerror

import "github.com/pkg/errors"

type ErrorType uint

const (
	Unknown ErrorType = iota
	SeatNotAvailable
	UserNotInTheRoom
	UserNotInAnyRoom
	NoSuchUserExists
	RoomNotExist
	InvalidRoomLayout
	YoutubeLiveChatBotFailed
	SeatNotFound
)

type CustomError struct {
	ErrorType ErrorType
	Body      error
}

func (et ErrorType) New(message string) CustomError {
	return CustomError{ErrorType: et, Body: errors.New(message)}
}
func (et ErrorType) Wrap(err error) CustomError {
	return CustomError{ErrorType: et, Body: err}
}
func (et ErrorType) WrapWithMessage(err error, message string) CustomError {
	return CustomError{ErrorType: et, Body: errors.Wrap(err, message)}
}

func NewNilCustomError() CustomError {
	return CustomError{
		ErrorType: Unknown,
		Body:      nil,
	}
}

