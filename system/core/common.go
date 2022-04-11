package core

import (
	"app.modules/core/myfirestore"
	"context"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"log"
	"strings"
	"time"
)

func HasWorkNameOptionPrefix(str string) bool {
	return strings.HasPrefix(str, WorkNameOptionPrefix) ||
		strings.HasPrefix(str, WorkNameOptionShortPrefix) ||
		strings.HasPrefix(str, WorkNameOptionPrefixLegacy) ||
		strings.HasPrefix(str, WorkNameOptionShortPrefixLegacy)
}

func TrimWorkNameOptionPrefix(str string) string {
	if strings.HasPrefix(str, WorkNameOptionPrefix) {
		return strings.TrimPrefix(str, WorkNameOptionPrefix)
	} else if strings.HasPrefix(str, WorkNameOptionShortPrefix) {
		return strings.TrimPrefix(str, WorkNameOptionShortPrefix)
	} else if strings.HasPrefix(str, WorkNameOptionPrefixLegacy) {
		return strings.TrimPrefix(str, WorkNameOptionPrefixLegacy)
	} else if strings.HasPrefix(str, WorkNameOptionShortPrefixLegacy) {
		return strings.TrimPrefix(str, WorkNameOptionShortPrefixLegacy)
	}
	return str
}

func HasTimeOptionPrefix(str string) bool {
	return strings.HasPrefix(str, TimeOptionPrefix) ||
		strings.HasPrefix(str, TimeOptionShortPrefix) ||
		strings.HasPrefix(str, TimeOptionPrefixLegacy) ||
		strings.HasPrefix(str, TimeOptionShortPrefixLegacy)
}

func TrimTimeOptionPrefix(str string) string {
	if strings.HasPrefix(str, TimeOptionPrefix) {
		return strings.TrimPrefix(str, TimeOptionPrefix)
	} else if strings.HasPrefix(str, TimeOptionShortPrefix) {
		return strings.TrimPrefix(str, TimeOptionShortPrefix)
	} else if strings.HasPrefix(str, TimeOptionPrefixLegacy) {
		return strings.TrimPrefix(str, TimeOptionPrefixLegacy)
	} else if strings.HasPrefix(str, TimeOptionShortPrefixLegacy) {
		return strings.TrimPrefix(str, TimeOptionShortPrefixLegacy)
	}
	return str
}

func CreateUpdatedSeatsSeatWorkName(seats []myfirestore.Seat, workName string, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].WorkName = workName
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatBreakWorkName(seats []myfirestore.Seat, breakWorkName string, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].BreakWorkName = breakWorkName
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatColorCode(seats []myfirestore.Seat, colorCode string,
	glowAnimation bool, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].ColorCode = colorCode
			seats[i].GlowAnimation = glowAnimation
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatUntil(seats []myfirestore.Seat, newUntil time.Time, userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].Until = newUntil
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatCurrentStateUntil(seats []myfirestore.Seat, newUntil time.Time,
	userId string) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].CurrentStateUntil = newUntil
			break
		}
	}
	return seats
}

func CreateUpdatedSeatsSeatState(seats []myfirestore.Seat, userId string, state myfirestore.SeatState,
	currentStateStartedAt time.Time, currentStateUntil time.Time, cumulativeWorkSec int, dailyCumulativeWorkSec int,
	workName string,
) []myfirestore.Seat {
	for i, seat := range seats {
		if seat.UserId == userId {
			seats[i].State = state
			seats[i].CurrentStateStartedAt = currentStateStartedAt
			seats[i].CurrentStateUntil = currentStateUntil
			seats[i].CumulativeWorkSec = cumulativeWorkSec
			seats[i].DailyCumulativeWorkSec = dailyCumulativeWorkSec
			switch state {
			case myfirestore.BreakState:
				seats[i].BreakWorkName = workName
			case myfirestore.WorkState:
				seats[i].WorkName = workName
			}
			break
		}
	}
	return seats
}

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println(err)
		err = godotenv.Load("../.env")
		if err != nil {
			log.Println(err.Error())
			log.Fatal("Error loading .env file")
		}
	}
}

func GetGcpProjectId(ctx context.Context, clientOption option.ClientOption) (string, error) {
	creds, err := transport.Creds(ctx, clientOption)
	if err != nil {
		return "", err
	}
	return creds.ProjectID, nil
}
