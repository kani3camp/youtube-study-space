package core

import (
	"app.modules/core/utils"
	"context"
	"fmt"
	"github.com/kr/pretty"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"log"
	"os"
	"reflect"
	"testing"
)

func InitTest() (option.ClientOption, context.Context, error) {
	utils.LoadEnv()
	credentialFilePath := os.Getenv("CREDENTIAL_FILE_LOCATION")
	
	ctx := context.Background()
	clientOption := option.WithCredentialsFile(credentialFilePath)
	
	// 本番GCPプロジェクトの場合はCLI上で確認
	creds, _ := transport.Creds(ctx, clientOption)
	if creds.ProjectID == "youtube-study-space" {
		fmt.Println("本番環境用のcredentialが使われます。よろしいですか？(yes / no)")
		var s string
		_, _ = fmt.Scanf("%s", &s)
		if s != "yes" {
			return nil, nil, errors.New("")
		}
	} else if creds.ProjectID == "test-youtube-study-space" {
		log.Println("credential of test-youtube-study-space")
	} else {
		return nil, nil, errors.New("unknown project id on the credential.")
	}
	return clientOption, ctx, nil
}

func NewTestSystem() (System, error) {
	clientOption, ctx, err := InitTest()
	if err != nil {
		return System{}, err
	}
	s, err := NewSystem(ctx, clientOption)
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
		Input          string
		ExpectedOutput CommandDetails
	}
	testCases := [...]TestCase{
		{
			Input: "in",
			ExpectedOutput: CommandDetails{
				CommandType: NotCommand,
				InOption:    InOption{},
			},
		},
		{
			Input: "!in",
			ExpectedOutput: CommandDetails{
				CommandType: In,
				InOption: InOption{
					WorkName: "",
					WorkMin:  s.Constants.DefaultWorkTimeMin,
				},
			},
		},
		{
			Input: "!in work-てすと min-50",
			ExpectedOutput: CommandDetails{
				CommandType: In,
				InOption: InOption{
					WorkName: "てすと",
					WorkMin:  50,
				},
			},
		},
		{
			Input: "!in min-60 work-わーく",
			ExpectedOutput: CommandDetails{
				CommandType: In,
				InOption: InOption{
					WorkName: "わーく",
					WorkMin:  60,
				},
			},
		},
		{
			Input: "!in min-60 work-w",
			ExpectedOutput: CommandDetails{
				CommandType: In,
				InOption: InOption{
					WorkName: "w",
					WorkMin:  60,
				},
			},
		},
		// TODO: w-やm-のテスト追加
		{
			Input: "out",
			ExpectedOutput: CommandDetails{
				CommandType: NotCommand,
				InOption:    InOption{},
			},
		},
		{
			Input: "!out",
			ExpectedOutput: CommandDetails{
				CommandType: Out,
				InOption:    InOption{},
			},
		},
		{
			Input: "!info",
			ExpectedOutput: CommandDetails{
				CommandType: Info,
				InOption:    InOption{},
			},
		},
		{
			Input: "!my",
			ExpectedOutput: CommandDetails{
				CommandType: My,
				MyOptions:   nil,
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
	isChatModerator := false
	isChatOwner := false
	s.SetProcessedUser(userId, userDisplayName, isChatModerator, isChatOwner)
	
	// 正しくセットされたか
	assert.Equal(t, s.ProcessedUserId, userId)
	assert.Equal(t, s.ProcessedUserDisplayName, userDisplayName)
	assert.Equal(t, s.ProcessedUserIsModeratorOrOwner, isChatModerator || isChatOwner)
}
