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
	GlowAnimation             bool
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
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 10 {
		return Rank{
			GreaterThanOrEqualToHours: 5,
			LessThanHours:             10,
			ColorCode:                 "#FFD4CC",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 20 {
		return Rank{
			GreaterThanOrEqualToHours: 10,
			LessThanHours:             20,
			ColorCode:                 "#FF9580",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 30 {
		return Rank{
			GreaterThanOrEqualToHours: 20,
			LessThanHours:             30,
			ColorCode:                 "#FFC880",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 50 {
		return Rank{
			GreaterThanOrEqualToHours: 30,
			LessThanHours:             50,
			ColorCode:                 "#FFFB7F",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 70 {
		return Rank{
			GreaterThanOrEqualToHours: 50,
			LessThanHours:             70,
			ColorCode:                 "#D0FF80",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 100 {
		return Rank{
			GreaterThanOrEqualToHours: 70,
			LessThanHours:             100,
			ColorCode:                 "#9DFF7F",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 150 {
		return Rank{
			GreaterThanOrEqualToHours: 100,
			LessThanHours:             150,
			ColorCode:                 "#80FF95",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 200 {
		return Rank{
			GreaterThanOrEqualToHours: 150,
			LessThanHours:             200,
			ColorCode:                 "#80FFC8",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 300 {
		return Rank{
			GreaterThanOrEqualToHours: 200,
			LessThanHours:             300,
			ColorCode:                 "#80FFFB",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 400 {
		return Rank{
			GreaterThanOrEqualToHours: 300,
			LessThanHours:             400,
			ColorCode:                 "#80D0FF",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 500 {
		return Rank{
			GreaterThanOrEqualToHours: 400,
			LessThanHours:             500,
			ColorCode:                 "#809EFF",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 700 {
		return Rank{
			GreaterThanOrEqualToHours: 500,
			LessThanHours:             700,
			ColorCode:                 "#947FFF",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 1000 {
		return Rank{
			GreaterThanOrEqualToHours: 700,
			LessThanHours:             1000,
			ColorCode:                 "#C880FF",
			GlowAnimation:             false,
		}, nil
	} else if totalHours < 1500 {
		return Rank{
			GreaterThanOrEqualToHours: 1000,
			LessThanHours:             1500,
			ColorCode:                 "#FFC880",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 2000 {
		return Rank{
			GreaterThanOrEqualToHours: 1500,
			LessThanHours:             2000,
			ColorCode:                 "#FFFB7F",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 2500 {
		return Rank{
			GreaterThanOrEqualToHours: 2000,
			LessThanHours:             2500,
			ColorCode:                 "#D0FF80",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 3000 {
		return Rank{
			GreaterThanOrEqualToHours: 2500,
			LessThanHours:             3000,
			ColorCode:                 "#9DFF7F",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 4000 {
		return Rank{
			GreaterThanOrEqualToHours: 3000,
			LessThanHours:             4000,
			ColorCode:                 "#80FF95",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 5000 {
		return Rank{
			GreaterThanOrEqualToHours: 4000,
			LessThanHours:             5000,
			ColorCode:                 "#80FFC8",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 6000 {
		return Rank{
			GreaterThanOrEqualToHours: 5000,
			LessThanHours:             6000,
			ColorCode:                 "#80FFFB",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 7000 {
		return Rank{
			GreaterThanOrEqualToHours: 6000,
			LessThanHours:             7000,
			ColorCode:                 "#80D0FF",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 8000 {
		return Rank{
			GreaterThanOrEqualToHours: 7000,
			LessThanHours:             8000,
			ColorCode:                 "#809EFF",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 9000 {
		return Rank{
			GreaterThanOrEqualToHours: 8000,
			LessThanHours:             9000,
			ColorCode:                 "#947FFF",
			GlowAnimation:             true,
		}, nil
	} else if totalHours < 10000 {
		return Rank{
			GreaterThanOrEqualToHours: 9000,
			LessThanHours:             10000,
			ColorCode:                 "#C880FF",
			GlowAnimation:             true,
		}, nil
	} else {
		return Rank{
			GreaterThanOrEqualToHours: 10000,
			LessThanHours:             math.MaxInt64,
			ColorCode:                 "#FF7FFF",
			GlowAnimation:             true,
		}, nil
	}
}

func GetInvisibleRank() Rank {
	return Rank{
		ColorCode:     "#BBBBBB",
		GlowAnimation: false,
	}
}
