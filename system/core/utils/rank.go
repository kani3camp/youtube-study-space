package utils

import (
	"github.com/pkg/errors"
	"math"
	"time"
)

type Rank struct {
	GreaterThanOrEqualToHours int
	LessThanHours             int
	ColorCode                 string
}

func GetRank(totalStudySec int) (Rank, error) {
	if totalStudySec < 0 { // 値チェック
		return Rank{}, errors.New("invalid value")
	}
	// 時間に換算
	totalDuration := time.Duration(totalStudySec) * time.Second
	totalHours := totalDuration.Hours()

	if totalHours < 5 {
		return Rank{
			GreaterThanOrEqualToHours: 0,
			LessThanHours:             5,
			ColorCode:                 "#fff",
		}, nil
	} else if totalHours < 10 {
		return Rank{
			GreaterThanOrEqualToHours: 5,
			LessThanHours:             10,
			ColorCode:                 "#FFD4CC",
		}, nil
	} else if totalHours < 20 {
		return Rank{
			GreaterThanOrEqualToHours: 10,
			LessThanHours:             20,
			ColorCode:                 "#FF9580",
		}, nil
	} else if totalHours < 30 {
		return Rank{
			GreaterThanOrEqualToHours: 20,
			LessThanHours:             30,
			ColorCode:                 "#FFC880",
		}, nil
	} else if totalHours < 50 {
		return Rank{
			GreaterThanOrEqualToHours: 30,
			LessThanHours:             50,
			ColorCode:                 "#FFFB7F",
		}, nil
	} else if totalHours < 70 {
		return Rank{
			GreaterThanOrEqualToHours: 50,
			LessThanHours:             70,
			ColorCode:                 "#D0FF80",
		}, nil
	} else if totalHours < 100 {
		return Rank{
			GreaterThanOrEqualToHours: 70,
			LessThanHours:             100,
			ColorCode:                 "#9DFF7F",
		}, nil
	} else if totalHours < 150 {
		return Rank{
			GreaterThanOrEqualToHours: 100,
			LessThanHours:             150,
			ColorCode:                 "#80FF95",
		}, nil
	} else if totalHours < 200 {
		return Rank{
			GreaterThanOrEqualToHours: 150,
			LessThanHours:             200,
			ColorCode:                 "#80FFC8",
		}, nil
	} else if totalHours < 300 {
		return Rank{
			GreaterThanOrEqualToHours: 200,
			LessThanHours:             300,
			ColorCode:                 "#80FFFB",
		}, nil
	} else if totalHours < 400 {
		return Rank{
			GreaterThanOrEqualToHours: 300,
			LessThanHours:             400,
			ColorCode:                 "#80D0FF",
		}, nil
	} else if totalHours < 500 {
		return Rank{
			GreaterThanOrEqualToHours: 400,
			LessThanHours:             500,
			ColorCode:                 "#809EFF",
		}, nil
	} else if totalHours < 700 {
		return Rank{
			GreaterThanOrEqualToHours: 500,
			LessThanHours:             700,
			ColorCode:                 "#947FFF",
		}, nil
	} else if totalHours < 1000 {
		return Rank{
			GreaterThanOrEqualToHours: 700,
			LessThanHours:             1000,
			ColorCode:                 "#C880FF",
		}, nil
	} else {
		return Rank{
			GreaterThanOrEqualToHours: 1000,
			LessThanHours:             math.MaxInt64,
			ColorCode:                 "#FF7FFF",
		}, nil
	}
}

func GetInvisibleRank() Rank {
	return Rank{
		ColorCode: "#BBBBBB",
	}
}
