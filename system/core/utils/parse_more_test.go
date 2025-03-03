package utils

import (
	"app.modules/core/i18n"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseMore(t *testing.T) {
	testCases := []ParseCommandTestCase{
		{
			Name:  "追加時間（数値直接指定）",
			Input: "!more 123",
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Name:  "追加時間（m=指定）",
			Input: "!more m=123",
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Name:  "追加時間（全角＝指定）",
			Input: "!more m＝123",
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Name:  "追加時間（min=指定）",
			Input: "!more min=123",
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},

		{
			Name:     "メンバーによる追加時間",
			Input:    "!more 20",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 20,
				},
			},
		},
		{
			Name:     "メンバーによる追加時間（m=指定）",
			Input:    "!more m=210",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 210,
				},
			},
		},
		{
			Name:     "メンバーによる絵文字追加時間",
			Input:    TestEmojiMore0 + " 100",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 100,
				},
			},
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可",
			Input:    TestEmojiMore0 + "100",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 100,
				},
			},
		},
		{
			Name:     "絵文字追加時間と360分指定",
			Input:    TestEmojiMore0 + TestEmoji360Min0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 360,
				},
			},
		},
		{
			Name:     "絵文字追加時間と時間指定",
			Input:    TestEmojiMore0 + TestEmojiMin0 + "40",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 40,
				},
			},
		},
		{
			Name:     "絵文字追加時間と無効な時間指定（エラーケース）",
			Input:    TestEmojiMore0 + TestEmojiMin0,
			IsMember: true,
			WillErr:  true,
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
