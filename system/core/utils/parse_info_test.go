package utils

import (
	"app.modules/core/i18n"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseInfo(t *testing.T) {
	testCases := []ParseCommandTestCase{
		{
			Name:  "基本的な情報表示",
			Input: "!info",
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: false,
				},
			},
		},
		{
			Name:  "詳細情報表示",
			Input: "!info d",
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:     "非メンバーによる絵文字情報表示（無効）",
			Input:    TestEmojiInfo0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Name:     "非メンバーによる絵文字詳細情報表示（無効）",
			Input:    TestEmojiInfoD0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Name:     "メンバーによる情報表示",
			Input:    "!info",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
			},
		},
		{
			Name:     "メンバーによる詳細情報表示",
			Input:    "!info d",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:     "メンバーによる絵文字情報表示",
			Input:    TestEmojiInfo0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
			},
		},
		{
			Name:     "メンバーによる絵文字詳細情報表示",
			Input:    TestEmojiInfoD0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:     "メンバーによる絵文字情報表示と詳細オプション",
			Input:    TestEmojiInfo0 + " d",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可",
			Input:    TestEmojiInfo0 + "d",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},
	}

	if err := i18n.LoadLocaleFolderFS(); err != nil {
		panic(err)
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			out, message := ParseCommand(testCase.Input, testCase.IsMember)
			if testCase.WillErr {
				assert.NotEmpty(t, message, "Expected error message but got none")
			} else {
				assert.Empty(t, message, "Expected no error message but got: %s", message)
				assert.Equal(t, testCase.Output, out, "Command details do not match")
			}
		})
	}

}
