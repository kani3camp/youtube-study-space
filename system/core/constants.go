package core

import (
	"github.com/joho/godotenv"
	"log"
)

const (
	EnterAction = "enter"
	ExitAction  = "exit"
	
	CommandPrefix      = "!"
	WrongCommandPrefix = "！"
	
	InCommand      = "!in"
	OutCommand     = "!out"
	InfoCommand    = "!info"
	MyCommand      = "!my"
	ChangeCommand  = "!change"
	SeatCommand    = "!seat"
	ReportCommand  = "!report"
	MoreCommand    = "!more"
	OkawariCommand = "!okawari"
	RankCommand    = "!rank"
	
	KickCommand = "!kick"
	SeeCommand  = "!see"
	
	LegacyAddCommand = "!add"
	
	WorkNameOptionPrefix            = "work="
	WorkNameOptionShortPrefix       = "w="
	WorkNameOptionPrefixLegacy      = "work-"
	WorkNameOptionShortPrefixLegacy = "w-"
	
	WorkTimeOptionPrefix            = "min="
	WorkTimeOptionShortPrefix       = "m="
	WorkTimeOptionPrefixLegacy      = "min-"
	WorkTimeOptionShortPrefixLegacy = "m-"
	
	InfoDetailsOption = "d"
	
	RankVisibleMyOptionPrefix = "rank="
	RankVisibleMyOptionOn     = "on"
	RankVisibleMyOptionOff    = "off"
	
	DefaultMinMyOptionPrefix     = "min="
	DefaultMinMyOptionShorPrefix = "m="
	
	FullWidthSpace = "　"
	HalfWidthSpace = " "
)

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
