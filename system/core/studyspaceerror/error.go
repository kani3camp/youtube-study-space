package studyspaceerror

import "github.com/pkg/errors"

var ErrUnknown = errors.New("unknown error")
var ErrUserNotInTheRoom = errors.New("user not in the room")
var ErrNoSeatAvailable = errors.New("no seat available")
var ErrInvalidCommand = errors.New("invalid command")
var ErrParseFailed = errors.New("parse failed")
var ErrInvalidParsedCommand = errors.New("invalid parsed command")

type ErrorType uint

const (
	Unknown ErrorType = iota

	UserNotInTheRoom
	NoSeatAvailable

	InvalidCommand
	ParseFailed
	InvalidParsedCommand
)

type StudySpaceError struct {
	ErrorType ErrorType
	Body      error
}

func (et ErrorType) New(message string) StudySpaceError {
	return StudySpaceError{ErrorType: et, Body: errors.New(message)}
}
func (et ErrorType) Wrap(err error) StudySpaceError {
	return StudySpaceError{ErrorType: et, Body: err}
}

func NewNil() StudySpaceError {
	return StudySpaceError{
		ErrorType: Unknown,
		Body:      nil,
	}
}

func (e *StudySpaceError) IsNil() bool {
	return e.ErrorType == Unknown && e.Body == nil
}

func (e *StudySpaceError) IsNotNil() bool {
	return !e.IsNil()
}

func (e *StudySpaceError) Error() string {
	if e.IsNil() {
		return "no error"
	}
	return e.Body.Error()
}

func (e *StudySpaceError) Unwrap() error {
	return e.Body
}
