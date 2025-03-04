package utils

import (
	"app.modules/core/i18n"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseBreak(t *testing.T) {
	testCases := []ParseCommandTestCase{
		{
			Name:  "休憩",
			Input: "!break",
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    false,
					IsDurationMinSet: false,
				},
			},
		},
		{
			Name:  "休憩（オプション付き）",
			Input: "!break min=23 work=休憩",
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "休憩",
					DurationMin:      23,
				},
			},
		},

		{
			Name:     "メンバーによる絵文字休憩",
			Input:    TestEmojiBreak0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    false,
					IsDurationMinSet: false,
				},
			},
		},
		{
			Name:     "メンバーによる休憩（オプション付き）",
			Input:    "!break min=23 work=休憩",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "休憩",
					DurationMin:      23,
				},
			},
		},
		{
			Name:     "メンバーによる絵文字休憩（オプション付き）",
			Input:    TestEmojiBreak0 + TestEmojiMin0 + "20 " + TestEmojiWork0 + "coffee",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "coffee",
					DurationMin:      20,
				},
			},
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可",
			Input:    TestEmojiBreak0 + TestEmojiMin0 + "20" + TestEmojiWork0 + "coffee",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Break,
				BreakOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					WorkName:         "coffee",
					IsDurationMinSet: true,
					DurationMin:      20,
				},
			},
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
			Name:  "再開（作業名付き）",
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
