package system

import (
	"strconv"
	"strings"
)

const (
	InCommand = "!in"
	OutCommand = "!out"
	TimeLimitCommand = "!tl"
	InfoCommand = "!info"
)

// Command: 入力コマンドを解析して実行
func Command(commandString string) error {
	if strings.HasPrefix(commandString, "!") {
		slice := strings.Split(commandString, " ")
		switch slice[0] {
		case InCommand:
			return In(slice)
		case OutCommand:
			return Out(slice)
		case TimeLimitCommand:
			return SetTimeLimit(slice)
		case InfoCommand:
			return ShowUserInfo(slice)
		}
	}
	return nil
}

func In(slice []string) error {
	seatId, err := strconv.Atoi(slice[1])
	if err != nil {
		return err
	}
	// check if seat available
	isOk, err := IfSeatAvailable(seatId)
	if err != nil {
		return err
	}
	// todo seatIdに着席
	if ! isOk {
		// todo
	} else {
		// todo
	}
	return nil
}

func Out(slice []string) error {
	
}

func SetTimeLimit(slice []string) error {

}

func ShowUserInfo(slice []string) error {

}

func IfSeatAvailable(seatId int) (bool, error) {
	// todo
}