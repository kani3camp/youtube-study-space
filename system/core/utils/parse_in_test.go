package utils

import (
	"app.modules/core/i18n"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParseIn(t *testing.T) {

	testCases := []ParseCommandTestCase{
		{
			Name:  "基本的な入室",
			Input: "!in",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "文頭にスペースがある場合",
			Input: " !in",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "文頭に全角スペースがある場合",
			Input: "　!in",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "!の隣に空白",
			Input: "! in",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "全角の！も対応",
			Input: "！in",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "従来のオプション指定（work-, min-）",
			Input: "!in min-30 work-テスト",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "テスト",
						DurationMin:      30,
					},
				},
			},
		},
		{
			Name:  "作業名と時間指定付き入室",
			Input: "!in work=てすと min=50",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "てすと",
						DurationMin:      50,
					},
				},
			},
		},
		{
			Name:  "全角スペース付き入室",
			Input: "!in　work=全角すぺーす　min=50",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "全角すぺーす",
						DurationMin:      50,
					},
				},
			},
		},
		{
			Name:  "時間指定が先の入室",
			Input: "!in min=60 work=わーく",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "わーく",
						DurationMin:      60,
					},
				},
			},
		},
		{
			Name:  "短い作業名付き入室",
			Input: "!in min=60 work=w",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "w",
						DurationMin:      60,
					},
				},
			},
		},
		{
			Name:  "work=なしで作業名指定可能",
			Input: "!in テスト",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: false,
						WorkName:         "テスト",
					},
				},
			},
		},
		{
			Name:  "work=なしで時間指定 時間指定あり",
			Input: "!in テスト min=60",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "テスト",
						DurationMin:      60,
					},
				},
			},
		},
		{
			Name:  "席番号0指定の入室",
			Input: "!0",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      0,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "全角の！による席番号0入室",
			Input: "！0",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      0,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Name:  "席番号0と作業名と時間指定付き入室",
			Input: "!0  min=180 work=work",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      0,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "work",
						DurationMin:      180,
					},
				},
			},
		},
		{
			Name:  "席番号1と作業名と時間指定付き入室",
			Input: "!1 work=work min=35",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      1,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "work",
						DurationMin:      35,
					},
				},
			},
		},
		{
			Name:  "席番号300と短縮形作業名指定付き入室",
			Input: "!300 w=ｙ",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      300,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: false,
						WorkName:         "ｙ",
					},
				},
			},
		},
		{
			Name:  "オプションは全角の＝も対応",
			Input: "!300 w＝全角イコール m＝165",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      300,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "全角イコール",
						DurationMin:      165,
					},
				},
			},
		},
		{
			Name:  "work=なしで作業名指定 オプション順不同",
			Input: "!in m=165 テスト",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "テスト",
						DurationMin:      165,
					},
				},
			},
		},
		{
			Name:  "work=を優先",
			Input: "!in テスト m=165 w=work",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "work",
						DurationMin:      165,
					},
				},
			},
		},
		{
			Name:  "全角の／によるメンバー入室",
			Input: "／in",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsMemberSeat:       true,
					MinutesAndWorkName: &MinutesAndWorkNameOption{},
				},
			},
		},
		{
			Name:  "メンバー席番号1と作業名と時間指定付き入室",
			Input: "/1 work=work min=35",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      1,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "work",
						DurationMin:      35,
					},
					IsMemberSeat: true,
				},
			},
		},
		{
			Name:  "work=なしで作業名指定可能 オプション順不同",
			Input: "/1 てすと m=165",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      1,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "てすと",
						DurationMin:      165,
					},
					IsMemberSeat: true,
				},
			},
		},
		{
			Name:  "全角の／によるメンバー席番号1入室",
			Input: "／1",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      1,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
					IsMemberSeat: true,
				},
			},
		},

		{
			Name:     "絵文字コマンド入室",
			Input:    TestEmojiIn0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet:        false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{},
				},
			},
		},
		{
			Name:     "絵文字コマンド入室 作業名指定",
			Input:    TestEmojiIn1 + TestEmojiWork0 + "dev",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet:        false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{IsWorkNameSet: true, WorkName: "dev"},
				},
			},
		},
		{
			Name:     "絵文字コマンド入室 席番号0指定",
			Input:    TestEmojiInZero0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet:        true,
					SeatId:             0,
					MinutesAndWorkName: &MinutesAndWorkNameOption{},
				},
			},
		},
		{
			Name:     "絵文字コマンド入室 作業名指定",
			Input:    "!in " + TestEmojiWork0 + "dev",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet: true,
						WorkName:      "dev",
					},
				},
			},
		},
		{
			Name:     "絵文字コマンド入室 絵文字作業名と絵文字時間指定",
			Input:    "!in" + TestEmojiWork0 + " " + TestEmojiMin0 + "111",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						WorkName:         "",
						IsDurationMinSet: true,
						DurationMin:      111,
					},
				},
			},
		},
		{
			Name:     "絵文字コマンド入室 絵文字作業名と絵文字時間指定",
			Input:    "!in" + TestEmojiWork0 + "わーく" + TestEmojiMin0 + "111",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						WorkName:         "わーく",
						IsDurationMinSet: true,
						DurationMin:      111,
					},
				},
			},
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可",
			Input:    "!in" + TestEmojiWork0 + "わーく" + TestEmojiInfo0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet: true,
						WorkName:      "わーく",
					},
				},
			},
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可（作業名リセット）",
			Input:    "!in" + TestEmojiWork0 + TestEmojiMin0 + "111",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						WorkName:         "",
						IsDurationMinSet: true,
						DurationMin:      111,
					},
				},
			},
		},
		{
			Name:     "絵文字コマンド入室と360分時間指定",
			Input:    TestEmojiIn0 + TestEmoji360Min0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsDurationMinSet: true,
						DurationMin:      360,
					},
				},
			},
		},
		{
			Name:     "無効な絵文字コマンドが入っていたら無視（入室コマンドで色指定はできない）",
			Input:    TestEmojiIn0 + TestEmojiColor0 + TestEmojiMin0 + "30",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsDurationMinSet: true,
						DurationMin:      30,
					},
				},
			},
		},
		{
			Name:     "絵文字入室と無効な時間指定（エラーケース）",
			Input:    TestEmojiIn0 + TestEmojiMin0 + "  450",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
			},
			WillErr: true,
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可",
			Input:    TestEmojiIn0 + TestEmojiMin0 + "300" + TestEmojiWork0 + "w",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "w",
						DurationMin:      300,
					},
				},
			},
		},
		{
			Name:     "席番号0と作業名（ハイフン区切り）",
			Input:    "!0 w-英単語",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      0,
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsWorkNameSet: true,
						WorkName:      "英単語",
					},
				},
			},
		},
		{
			Name:     "非メンバーによるメンバー用絵文字入室（無効）",
			Input:    TestEmojiMemberIn0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Name:     "メンバー用絵文字入室と色指定と時間指定",
			Input:    TestEmojiMemberIn0 + TestEmojiColor0 + TestEmojiMin0 + "30",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					MinutesAndWorkName: &MinutesAndWorkNameOption{
						IsDurationMinSet: true,
						DurationMin:      30,
					},
					IsMemberSeat: true,
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
