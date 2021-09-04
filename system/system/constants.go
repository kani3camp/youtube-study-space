package system

import (
	"github.com/joho/godotenv"
	"log"
)

const (
	ProjectId = "youtube-study-space"

	EnterAction = "enter"
	ExitAction = "exit"
	
	InCommand = "!in"
	OutCommand = "!out"
	InfoCommand = "!info"
	CommandPrefix = "!"
	
	WorkNameOptionPrefix = "work-"
	WorkNameOptionShortPrefix = "w-"
	WorkTimeOptionPrefix = "min-"
	WorkTimeOptionShortPrefix = "m-"
	
	FullWidthSpace = "　"
	HalfWidthSpace = " "

)


func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}