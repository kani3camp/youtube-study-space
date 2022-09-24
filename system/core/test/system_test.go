package test

import (
	"app.modules/core"
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
	"strconv"
	"testing"
)

func InitTest() (option.ClientOption, context.Context, error) {
	utils.LoadEnv("../../.env")
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

func NewTestSystem() (core.System, error) {
	clientOption, ctx, err := InitTest()
	if err != nil {
		return core.System{}, err
	}
	s, err := core.NewSystem(ctx, clientOption)
	if err != nil {
		return core.System{}, err
	}
	return s, nil
}

func TestSystem_ParseCommand(t *testing.T) {
	type TestCase struct {
		Input  string
		Output core.CommandDetails
	}
	testCases := [...]TestCase{
		{
			Input: "out",
			Output: core.CommandDetails{
				CommandType: core.NotCommand,
				InOption:    core.InOption{},
			},
		},
		{
			Input: "!out",
			Output: core.CommandDetails{
				CommandType: core.Out,
				InOption:    core.InOption{},
			},
		},
		{
			Input: "!info",
			Output: core.CommandDetails{
				CommandType: core.Info,
				InOption:    core.InOption{},
			},
		},
		{
			Input: "!my",
			Output: core.CommandDetails{
				CommandType: core.My,
				MyOptions:   []core.MyOption{},
			},
		},
		{
			Input: "!my rank=on",
			Output: core.CommandDetails{
				CommandType: core.My,
				MyOptions: []core.MyOption{
					{
						Type:      core.RankVisible,
						BoolValue: true,
					},
				},
			},
		},
		{
			Input: "!my rank=off",
			Output: core.CommandDetails{
				CommandType: core.My,
				MyOptions: []core.MyOption{
					{
						Type:      core.RankVisible,
						BoolValue: false,
					},
				},
			},
		},
	}
	
	for i, testCase := range testCases {
		commandDetails, err := core.ParseCommand(testCase.Input)
		if err.IsNotNil() {
			t.Error(err)
		}
		if !reflect.DeepEqual(commandDetails, testCase.Output) {
			fmt.Printf("result:\n%# v\n", pretty.Formatter(commandDetails))
			fmt.Printf("expected:\n%# v\n", pretty.Formatter(testCase.Output))
			t.Error("command details do not match. (i=" + strconv.Itoa(i) + ")")
		}
		//assert.True(t, reflect.DeepEqual(commandDetails, testCase.Output))
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
