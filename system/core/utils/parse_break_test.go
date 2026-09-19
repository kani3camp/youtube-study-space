package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"app.modules/core/i18n"
)

func TestParseBreak(t *testing.T) {
	testCases := []ParseCommandTestCase{
		{
			Name:  "休憩",
			Input: "!break",
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: BreakOption{},
			},
		},
		{
			Name:  "休憩（時間オプション付き）",
			Input: "!break min=23",
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: BreakOption{
					IsDurationMinSet: true,
					DurationMin:      23,
				},
			},
		},
		{
			Name:    "休憩（workオプションは無効）",
			Input:   "!break work coffee",
			WillErr: true,
		},
		{
			Name:    "休憩（orderオプションは無効）",
			Input:   "!break order 1",
			WillErr: true,
		},
		{
			Name:     "メンバーによる絵文字休憩",
			Input:    TestEmojiBreak0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: BreakOption{},
			},
		},
		{
			Name:     "メンバーによる絵文字休憩（時間オプション付き）",
			Input:    TestEmojiBreak0 + TestEmojiMin0 + "20",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: BreakOption{
					IsDurationMinSet: true,
					DurationMin:      20,
				},
			},
		},
		{
			Name:     "メンバーによる絵文字休憩（workオプションは無効）",
			Input:    TestEmojiBreak0 + TestEmojiWork0 + "coffee",
			IsMember: true,
			WillErr:  true,
		},

		{
			Name:  "再開",
			Input: "!resume",
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: false,
				},
			},
		},
		{
			Name:  "再開（作業名付き１）",
			Input: "!resume work=再開！",
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: true,
					WorkName:      "再開！",
				},
			},
		},
		{
			Name:  "再開（作業名付き２）",
			Input: "!resume 再開！",
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: true,
					WorkName:      "再開！",
				},
			},
		},
		{
			Name:  "再開（作業名指定あるけど無効１）",
			Input: "!resume work",
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: true,
					WorkName:      "",
				},
			},
		},
		{
			Name:  "再開（作業名指定あるけど無効２）",
			Input: "!resume work=",
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: true,
					WorkName:      "",
				},
			},
		},
		{
			Name:     "非メンバーによる絵文字再開（無効）",
			Input:    TestEmojiResume0,
			IsMember: false,
			Output:   &CommandDetails{CommandType: NotCommand},
		},
		{
			Name:     "非メンバーによる絵文字作業名付き再開（無効）",
			Input:    "!resume " + TestEmojiWork0 + "www",
			IsMember: false,
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: true,
					WorkName:      ":_commandWork0:www",
				},
			},
		},

		{
			Name:     "メンバーによる再開",
			Input:    "!resume",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Resume,
			},
		},
		{
			Name:     "メンバーによる絵文字再開",
			Input:    TestEmojiResume0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Resume,
			},
		},
		{
			Name:     "メンバーによる絵文字再開（作業名付き）",
			Input:    TestEmojiResume0 + "work-再開",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: true,
					WorkName:      "再開",
				},
			},
		},
		{
			Name:     "メンバーによる絵文字再開（絵文字作業名付き）",
			Input:    TestEmojiResume0 + TestEmojiWork0 + "再開",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: true,
					WorkName:      "再開",
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
