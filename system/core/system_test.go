package core

import (
	"context"
	"fmt"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/option"
	"os"
	"reflect"
	"testing"
)

func NewTestSystem() (System, error) {
	LoadEnv()
	credentialFilePath := os.Getenv("CREDENTIAL_FILE_LOCATION")
	clientOption := option.WithCredentialsFile(credentialFilePath)
	s, err := NewSystem(context.Background(), clientOption)
	if err != nil {
		return System{}, err
	}
	return s, nil
}

func TestSystem_ParseCommand(t *testing.T) {
	s, err := NewTestSystem()
	if err != nil {
		t.Error("failed NewSystem()", err)
		return
	}
	
	type TestCase struct {
		Input string
		ExpectedOutput CommandDetails
	}
	testCases := [...]TestCase{
		{
			Input:          "in",
			ExpectedOutput: CommandDetails{
				CommandType: NotCommand,
				InOptions:   InOptions{},
			},
		},
		{
			Input: "!in",
			ExpectedOutput: CommandDetails{
				CommandType: In,
				InOptions: InOptions{
					SeatId:   -1,
					WorkName: "",
					WorkMin:  s.DefaultWorkTimeMin,
				},
			},
		},
		{
			Input: "!in work-てすと min-50",
			ExpectedOutput: CommandDetails{
				CommandType: In,
				InOptions:     InOptions{
					SeatId: -1,
					WorkName: "てすと",
					WorkMin: 50,
				},
			},
		},
		{
			Input: "!in min-60 work-わーく",
			ExpectedOutput: CommandDetails{
				CommandType: In,
				InOptions:     InOptions{
					SeatId: -1,
					WorkName: "わーく",
					WorkMin: 60,
				},
			},
		},
		// TODO: w-やm-のテスト追加
		{
			Input: "!0",
			ExpectedOutput: CommandDetails{
				CommandType: SeatIn,
				InOptions:     InOptions{
					SeatId: 0,
					WorkName: "",
					WorkMin: s.DefaultWorkTimeMin,
				},
			},
		},
		{
			Input: "!12 work-てすと",
			ExpectedOutput: CommandDetails{
				CommandType: SeatIn,
				InOptions:     InOptions{
					SeatId:   12,
					WorkName: "てすと",
					WorkMin:  s.DefaultWorkTimeMin,
				},
			},
		},
		{
			Input: "out",
			ExpectedOutput: CommandDetails{
				CommandType: NotCommand,
				InOptions:   InOptions{},
			},
		},
		{
			Input: "!out",
			ExpectedOutput: CommandDetails{
				CommandType: Out,
				InOptions:   InOptions{},
			},
		},
		{
			Input: "!info",
			ExpectedOutput: CommandDetails{
				CommandType: Info,
				InOptions:   InOptions{},
			},
		},
		{
			Input: "!my",
			ExpectedOutput: CommandDetails{
				CommandType: My,
				MyOptions: nil,
			},
		},
		{
			Input: "!my rank=on",
			ExpectedOutput: CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:      RankVisible,
						BoolValue: true,
					},
				},
			},
		},
		{
			Input: "!my rank=off",
			ExpectedOutput: CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:      RankVisible,
						BoolValue: false,
					},
				},
			},
		},
	}
	
	for _, testCase := range testCases {
		commandDetails, err := s.ParseCommand(testCase.Input)
		if err.IsNotNil() {
			t.Error(err)
		}
		if !reflect.DeepEqual(commandDetails, testCase.ExpectedOutput) {
			fmt.Printf("result:\n%# v\n", pretty.Formatter(commandDetails))
			fmt.Printf("expected:\n%# v\n", pretty.Formatter(testCase.ExpectedOutput))
			t.Error("command details do not match.")
		}
		//assert.True(t, reflect.DeepEqual(commandDetails, testCase.ExpectedOutput))
	}
}

func TestSystem_SetProcessedUser(t *testing.T) {
	s, err := NewTestSystem()
	if err != nil {
		t.Error("failed NewSystem()", err)
		return
	}
	
	// 初期値は空文字列のはず
	assert.Equal(t, s.ProcessedUserId, "")
	assert.Equal(t, s.ProcessedUserDisplayName, "")
	
	userId := "user1-id"
	userDisplayName := "user1-display-name"
	s.SetProcessedUser(userId, userDisplayName)
	
	// 正しくセットされたか
	assert.Equal(t, s.ProcessedUserId, userId)
	assert.Equal(t, s.ProcessedUserDisplayName, userDisplayName)
}



