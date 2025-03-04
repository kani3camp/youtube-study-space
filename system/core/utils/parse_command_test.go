package utils

import (
	"testing"

	"app.modules/core/i18n"
	"github.com/stretchr/testify/assert"
)

const (
	TestEmojiIn0       = ":_commandIn0:"
	TestEmojiIn1       = ":_commandIn1:"
	TestEmojiInZero0   = ":_command0InZero0:"
	TestEmojiOut0      = ":_commandOut0:"
	TestEmojiInfo0     = ":_commandInfo0:"
	TestEmojiInfoD0    = ":_commandInfoD0:"
	TestEmojiSeat0     = ":_commandSeat0:"
	TestEmojiSeatD0    = ":_commandSeatD0:"
	TestEmojiChange0   = ":_commandChange0:"
	TestEmojiBreak0    = ":_commandBreak0:"
	TestEmojiResume0   = ":_commandResume0:"
	TestEmojiMore0     = ":_commandMore0:"
	TestEmojiMy0       = ":_commandMy0:"
	TestEmojiRank0     = ":_commandRank0:"
	TestEmojiMemberIn0 = ":_commandMemberIn0:"
	TestEmojiOrder0    = ":_commandOrder0:"

	TestEmojiMin0     = ":_commandMin0:"
	TestEmoji360Min0  = ":_command360Min0:"
	TestEmojiColor0   = ":_commandColor0:"
	TestEmojiWork0    = ":_commandWork0:"
	TestEmojiRankOn0  = ":_commandRankOn0:"
	TestEmojiRankOff0 = ":_commandRankOff0:"
)

type ParseCommandTestCase struct {
	Name     string
	Input    string
	IsMember bool
	Output   *CommandDetails
	WillErr  bool
}

func TestParseCommand(t *testing.T) {

	testCases := []ParseCommandTestCase{
		{
			Name:  "非コマンド",
			Input: "in",
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Name:  "非コマンド（空文字）",
			Input: "",
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Name:  "無効コマンド",
			Input: "!",
			Output: &CommandDetails{
				CommandType: InvalidCommand,
			},
		},
		{
			Name:  "存在しないコマンド",
			Input: "!unknown",
			Output: &CommandDetails{
				CommandType: InvalidCommand,
			},
		},

		{
			Name:     "基本的な退室",
			Input:    "!out",
			IsMember: false,
			Output: &CommandDetails{
				CommandType: Out,
			},
		},
		{
			Name:     "全角の！による退室",
			Input:    "！out",
			IsMember: false,
			Output: &CommandDetails{
				CommandType: Out,
			},
		},
		{
			Name:     "非メンバーによる絵文字退室（無効）",
			Input:    TestEmojiOut0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Name:     "メンバーによる絵文字退室",
			Input:    TestEmojiOut0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Out,
			},
		},

		{
			Name:  "ランク表示",
			Input: "!rank",
			Output: &CommandDetails{
				CommandType: Rank,
			},
		},
		{
			Name:     "メンバーによる絵文字コマンドランク表示",
			Input:    TestEmojiRank0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Rank,
			},
		},
		{
			Name:     "非メンバーによる絵文字ランク表示（無効）",
			Input:    TestEmojiRank0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},

		{
			Name:  "席表示",
			Input: "!seat",
			Output: &CommandDetails{
				CommandType: Seat,
			},
		},
		{
			Name:  "詳細席表示",
			Input: "!seat d",
			Output: &CommandDetails{
				CommandType: Seat,
				SeatOption: SeatOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:  "詳細席表示（全角スペース）",
			Input: "!seat　d",
			Output: &CommandDetails{
				CommandType: Seat,
				SeatOption: SeatOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:  "詳細席表示（スペース複数）",
			Input: "!seat   d",
			Output: &CommandDetails{
				CommandType: Seat,
				SeatOption: SeatOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:     "非メンバーによる絵文字席表示（無効）",
			Input:    TestEmojiSeat0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},

		{
			Name:     "メンバーによる席表示",
			Input:    "!seat",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Seat,
			},
		},
		{
			Name:     "メンバーによる絵文字席表示",
			Input:    TestEmojiSeat0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Seat,
			},
		},
		{
			Name:     "メンバーによる絵文字詳細席表示",
			Input:    TestEmojiSeatD0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Seat,
				SeatOption: SeatOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:     "メンバーによる絵文字席表示と詳細オプション",
			Input:    TestEmojiSeat0 + " d",
			IsMember: true,
			Output: &CommandDetails{CommandType: Seat,
				SeatOption: SeatOption{
					ShowDetails: true,
				},
			},
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可",
			Input:    TestEmojiSeat0 + "d",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Seat,
				SeatOption: SeatOption{
					ShowDetails: true,
				},
			},
		},

		{
			Name:  "キック",
			Input: "!kick 12",
			Output: &CommandDetails{
				CommandType: Kick,
				KickOption: KickOption{
					SeatId: 12,
				},
			},
		},

		{
			Name:  "チェック",
			Input: "!check 14",
			Output: &CommandDetails{
				CommandType: Check,
				CheckOption: CheckOption{
					SeatId: 14,
				},
			},
		},

		{
			Name:  "オーダー",
			Input: "!order 22",
			Output: &CommandDetails{
				CommandType: Order,
				OrderOption: OrderOption{
					IntValue: 22,
				},
			},
		},
		{
			Name:  "オーダーキャンセル",
			Input: "!order -",
			Output: &CommandDetails{
				CommandType: Order,
				OrderOption: OrderOption{
					ClearFlag: true,
				},
			},
		},
		{
			Name:  "全角スペース付きオーダー",
			Input: "!order　8",
			Output: &CommandDetails{
				CommandType: Order,
				OrderOption: OrderOption{
					IntValue: 8,
				},
			},
		},

		{
			Name:  "レポート",
			Input: "!report めっせーじ",
			Output: &CommandDetails{
				CommandType: Report,
				ReportOption: ReportOption{
					Message: "!report めっせーじ",
				},
			},
		},
		{
			Name:  "全角スペース付きレポート",
			Input: "!report　全角すぺーすめっせーじ",
			Output: &CommandDetails{
				CommandType: Report,
				ReportOption: ReportOption{
					Message: "!report 全角すぺーすめっせーじ",
				},
			},
		},

		{
			Name:  "作業名クリア",
			Input: "!clear",
			Output: &CommandDetails{
				CommandType: Clear,
			},
		},
		{
			Name:  "全角！クリア",
			Input: "！clear",
			Output: &CommandDetails{
				CommandType: Clear,
			},
		},
		{
			Name:  "スペース付きクリア",
			Input: "! clear",
			Output: &CommandDetails{
				CommandType: Clear,
			},
		},
		{
			Name:  "作業名クリア（短縮形）",
			Input: "!clr",
			Output: &CommandDetails{
				CommandType: Clear,
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
