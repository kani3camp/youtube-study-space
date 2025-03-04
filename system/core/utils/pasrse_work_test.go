package utils

import (
	"testing"

	"app.modules/core/i18n"
	"github.com/stretchr/testify/assert"
)

func TestParseWork(t *testing.T) {
	testCases := []ParseCommandTestCase{
		{
			Name:  "基本的な入室",
			Input: "!work",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "作業内容を指定",
			Input: "!work 運動",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: false,
						WorkName:         "運動",
					},
				},
			},
		},
		{
			Name:  "作業時間を指定",
			Input: "!work min=30",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: true,
						DurationMin:      30,
					},
				},
			},
		},
		{
			Name:  "オプションは順不同",
			Input: "!work min=30 運動",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "運動",
						DurationMin:      30,
					},
				},
			},
		},
		{
			Name:  "空白ありの作業内容",
			Input: "!work min=30 hard work",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "hard work",
						DurationMin:      30,
					},
				},
			},
		},
		{
			Name:  "作業内容をクリアする",
			Input: "!work work=",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: false,
						WorkName:         "",
					},
				},
			},
		},
		{
			Name:  "全角！でもOK",
			Input: "！work",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "!の次が空白でもOK",
			Input: "! work",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:     "メンバー席に入室",
			Input:    "/work",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsMemberSeat: true,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:     "メンバー席に入室 全角／",
			Input:    "／work",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsMemberSeat: true,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
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
