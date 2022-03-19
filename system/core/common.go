package core

import (
	"app.modules/core/myfirestore"
	"context"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"log"
	"time"
)

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
