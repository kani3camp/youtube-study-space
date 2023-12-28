package studyspaceerror

import "github.com/pkg/errors"

var ErrUserNotInTheRoom = errors.New("user not in the room")
var ErrNoSeatAvailable = errors.New("no seat available")
