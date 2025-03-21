package utils

import (
	"app.modules/core/i18n"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseChange(t *testing.T) {
	testCases := []ParseCommandTestCase{
		{
			Name:  "変更",
			Input: "!change m=140 w=新",
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "新",
					DurationMin:      140,
				},
			},
		},
		{
			Name:    "オプションなしは不可",
			Input:   "!change",
			WillErr: true,
		},
		{
			Name:     "非メンバーによる絵文字コマンド変更（無効）",
			Input:    "!change " + TestEmojiWork0 + TestEmoji360Min0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet: true,
					WorkName:      TestEmojiWork0 + TestEmoji360Min0,
				},
			},
		},

		{
			Name:     "メンバーによる変更",
			Input:    "!change m=140 w=新",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "新",
					DurationMin:      140,
				},
			},
		},
		{
			Name:  "work=は不要",
			Input: "!change てすと m=140",
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "てすと",
					DurationMin:      140,
				},
			},
		},
		{
			Name:  "work=は不要 オプション順不同",
			Input: "!change m=140 てすと",
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "てすと",
					DurationMin:      140,
				},
			},
		},
		{
			Name:  "一番左の作業内容を優先",
			Input: "!change テスト1 m=140 w=テスト2",
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "テスト1",
					DurationMin:      140,
				},
			},
		},
		{
			Name:     "メンバーによる絵文字コマンド変更",
			Input:    TestEmojiChange0 + TestEmojiWork0 + TestEmoji360Min0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					WorkName:         "",
					IsDurationMinSet: true,
					DurationMin:      360,
				},
			},
		},
		{
			Name:     "メンバーによる絵文字変更とオプション",
			Input:    TestEmojiChange0 + " m=140 w=新",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "新",
					DurationMin:      140,
				},
			},
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可",
			Input:    TestEmojiChange0 + "m=140 w=新",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					WorkName:         "新",
					IsDurationMinSet: true,
					DurationMin:      140,
				},
			},
		},

		{
			Name:     "メンバーによる絵文字変更",
			Input:    "!change " + TestEmojiWork0 + TestEmoji360Min0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinWorkOrderOption{
					IsWorkNameSet:    true,
					WorkName:         "",
					IsDurationMinSet: true,
					DurationMin:      360,
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
