package studyspaceerror

import "errors"

var (
	ErrUserNotInTheRoom = errors.New("user not in the room")
	ErrNoSeatAvailable  = errors.New("no seat available")
)
