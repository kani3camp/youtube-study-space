package utils

import (
	"github.com/kr/pretty"
	"reflect"
	"testing"
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

	TestEmojiMin0     = ":_commandMin0:"
	TestEmoji360Min0  = ":_command360Min0:"
	TestEmojiColor0   = ":_commandColor0:"
	TestEmojiWork0    = ":_commandWork0:"
	TestEmojiRankOn0  = ":_commandRankOn0:"
	TestEmojiRankOff0 = ":_commandRankOff0:"
)

func TestParseCommand(t *testing.T) {
	type TestCase struct {
		Input    string
		IsMember bool
		Output   *CommandDetails
		WillErr  bool
	}

	testCases := [...]TestCase{
		{
			Input: "in",
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
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
			Input: "!in work-てすと min-50",
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
			Input: "!in　work-全角すぺーす　min-50",
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
			Input: "!in min-60 work-わーく",
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
			Input: "!in min-60 work-w",
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
			Input: "/in",
			Output: &CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsMemberSeat:       true,
					MinutesAndWorkName: &MinutesAndWorkNameOption{},
				},
			},
		},
		{
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
		{ // no space
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
		{ // no space
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
		{ // no space
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
			Input:    TestEmojiIn0 + TestEmojiMin0 + "  450",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: In,
			},
			WillErr: true,
		},
		{ // no space.
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
			Input:    TestEmojiMemberIn0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
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

		{
			Input: "!out",
			Output: &CommandDetails{
				CommandType: Out,
			},
		},
		{
			Input:    TestEmojiOut0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},

		{
			Input:    "!out",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Out,
			},
		},
		{
			Input:    TestEmojiOut0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Out,
			},
		},

		{
			Input: "!info",
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: false,
				},
			},
		},
		{
			Input: "!info d",
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},
		{
			Input:    TestEmojiInfo0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Input:    TestEmojiInfoD0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},

		{
			Input:    "!info",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
			},
		},
		{
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
			Input:    TestEmojiInfo0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
			},
		},
		{
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
			Input:    TestEmojiInfo0 + " d",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},
		{ // no space.
			Input:    TestEmojiInfo0 + "d",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},

		{
			Input: "!my rank=on",
			Output: &CommandDetails{
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
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:      RankVisible,
						BoolValue: false,
					},
				},
			},
		},
		{
			Input: "!my",
			Output: &CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},
		{
			Input: "!my min=500",
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:     DefaultStudyMin,
						IntValue: 500,
					},
				},
			},
		},
		{
			Input: "!my min=", // リセットの意味
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:     DefaultStudyMin,
						IntValue: 0,
					},
				},
			},
		},
		{
			Input: "!my color=", // リセットの意味
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:        FavoriteColor,
						StringValue: "",
					},
				},
			},
		},
		{
			Input: "!my min=40 color=ピンク  rank=off",
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:     DefaultStudyMin,
						IntValue: 40,
					},
					{
						Type:        FavoriteColor,
						StringValue: "ピンク",
					},
					{
						Type:      RankVisible,
						BoolValue: false,
					},
				},
			},
		},
		{
			Input:    TestEmojiMy0 + TestEmojiColor0 + "白 " + TestEmojiMin0 + "100 " + TestEmojiRankOn0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Input:    "!my " + TestEmojiColor0 + "白 " + TestEmojiMin0 + "100 " + TestEmojiRankOn0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},

		{
			Input:    "!my color=白 min=200 rank=off",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:        FavoriteColor,
						StringValue: "白",
					},
					{
						Type:     DefaultStudyMin,
						IntValue: 200,
					},
					{
						Type:      RankVisible,
						BoolValue: false,
					},
				},
			},
		},
		{
			Input:    "!my ",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},
		{
			Input:    TestEmojiMy0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},
		{
			Input:    TestEmojiMy0 + TestEmojiColor0 + "白 " + TestEmojiMin0 + "100 " + TestEmojiRankOn0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:        FavoriteColor,
						StringValue: "白",
					},
					{
						Type:     DefaultStudyMin,
						IntValue: 100,
					},
					{
						Type:      RankVisible,
						BoolValue: true,
					},
				},
			},
		},
		{ // no space.
			Input:    TestEmojiMy0 + TestEmojiColor0 + "白" + TestEmojiMin0 + "100" + TestEmojiRankOn0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:        FavoriteColor,
						StringValue: "白",
					},
					{
						Type:     DefaultStudyMin,
						IntValue: 100,
					},
					{
						Type:      RankVisible,
						BoolValue: true,
					},
				},
			},
		},

		{
			Input: "!change m=140 w=新",
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "新",
					DurationMin:      140,
				},
			},
		},
		{
			Input: "!change",
			Output: &CommandDetails{
				CommandType: Change,
			},
		},
		{
			Input:    "!change " + TestEmojiWork0 + TestEmoji360Min0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: Change,
			},
		},

		{
			Input:    "!change m=140 w=新",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "新",
					DurationMin:      140,
				},
			},
		},
		{
			Input:    "!change",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
			},
		},
		{
			Input:    TestEmojiChange0 + TestEmojiWork0 + TestEmoji360Min0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					WorkName:         "",
					IsDurationMinSet: true,
					DurationMin:      360,
				},
			},
		},
		{
			Input:    TestEmojiChange0 + " m=140 w=新",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					IsDurationMinSet: true,
					WorkName:         "新",
					DurationMin:      140,
				},
			},
		},
		{ // no space.
			Input:    TestEmojiChange0 + "m=140 w=新",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					WorkName:         "新",
					IsDurationMinSet: true,
					DurationMin:      140,
				},
			},
		},
		{
			Input:    "!change",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
			},
		},
		{
			Input:    "!change " + TestEmojiWork0 + TestEmoji360Min0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Change,
				ChangeOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    true,
					WorkName:         "",
					IsDurationMinSet: true,
					DurationMin:      360,
				},
			},
		},

		{
			Input: "!rank",
			Output: &CommandDetails{
				CommandType: Rank,
			},
		},
		{
			Input:    TestEmojiRank0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},

		{
			Input: "!more 123",
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Input: "!more m=123",
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Input: "!more m＝123",
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Input: "!more min=123",
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},

		{
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
			Input:    TestEmojiMore0 + " 100",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 100,
				},
			},
		},
		{ // no space.
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
			Input:    TestEmojiMore0 + TestEmojiMin0,
			IsMember: true,
			WillErr:  true,
		},

		{
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
		{ // no space.
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
			Input: "!resume",
			Output: &CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: false,
				},
			},
		},
		{
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
			Input:    TestEmojiResume0,
			IsMember: false,
			Output:   &CommandDetails{CommandType: NotCommand},
		},
		{
			Input:    "!resume " + TestEmojiWork0 + "www",
			IsMember: false,
			Output: &CommandDetails{
				CommandType: Resume,
			},
		},

		{
			Input:    "!resume",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Resume,
			},
		},
		{
			Input:    TestEmojiResume0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Resume,
			},
		},
		{
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

		{
			Input: "!seat",
			Output: &CommandDetails{
				CommandType: Seat,
			},
		},
		{
			Input: "!seat d",
			Output: &CommandDetails{CommandType: Seat,
				SeatOption: SeatOption{
					ShowDetails: true,
				},
			},
		},
		{
			Input:    TestEmojiSeat0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},

		{
			Input:    "!seat",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Seat,
			},
		},
		{
			Input:    TestEmojiSeat0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: Seat,
			},
		},
		{
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
			Input:    TestEmojiSeat0 + " d",
			IsMember: true,
			Output: &CommandDetails{CommandType: Seat,
				SeatOption: SeatOption{
					ShowDetails: true,
				},
			},
		},
		{ // no space.
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
			Input: "!kick 12",
			Output: &CommandDetails{
				CommandType: Kick,
				KickOption: KickOption{
					SeatId: 12,
				},
			},
		},

		{
			Input: "!check 14",
			Output: &CommandDetails{
				CommandType: Check,
				CheckOption: CheckOption{
					SeatId: 14,
				},
			},
		},

		{
			Input: "!report めっせーじ",
			Output: &CommandDetails{
				CommandType: Report,
				ReportOption: ReportOption{
					Message: "!report めっせーじ",
				},
			},
		},
		{
			Input: "!report　全角すぺーすめっせーじ",
			Output: &CommandDetails{
				CommandType: Report,
				ReportOption: ReportOption{
					Message: "!report 全角すぺーすめっせーじ",
				},
			},
		},
	}

	for _, testCase := range testCases {
		out, message := ParseCommand(testCase.Input, testCase.IsMember)
		if testCase.WillErr {
			if message == "" {
				t.Error("message expected, but nil.")
				t.Error("input: ", testCase.Input)
				t.Errorf("out: %# v\n", pretty.Formatter(out))
			}
		} else {
			if message != "" {
				t.Error("input: ", testCase.Input)
				t.Error(message)
			}
			if !reflect.DeepEqual(out, testCase.Output) {
				t.Errorf("input: %s\n", testCase.Input)
				t.Errorf("result:\n%# v\n", pretty.Formatter(out))
				t.Errorf("expected:\n%# v\n", pretty.Formatter(testCase.Output))
				t.Error("command details do not match.")
			}
		}
	}
}

func TestExtractAllEmojiCommands(t *testing.T) {
	type TestCase struct {
		Input   string
		Output1 []EmojiElement
		Output2 string
	}
	testCases := [...]TestCase{
		{
			Input: TestEmojiIn0 + TestEmoji360Min0,
			Output1: []EmojiElement{
				EmojiIn,
				EmojiMin,
			},
			Output2: "",
		},
		{
			Input:   "!in",
			Output1: []EmojiElement{},
			Output2: "!in",
		},
		{
			Input: " " + TestEmojiMy0 + TestEmojiColor0 + "ピンク",
			Output1: []EmojiElement{
				EmojiMy,
				EmojiColor,
			},
			Output2: " ピンク",
		},
	}

	for _, testCase := range testCases {
		emojis, emojiExcludedString := ExtractAllEmojiCommands(testCase.Input)
		if !reflect.DeepEqual(emojis, testCase.Output1) {
			t.Errorf("input: %s\n", testCase.Input)
			t.Errorf("emojis:\n%# v\n", pretty.Formatter(emojis))
			t.Errorf("emojiExcludedString:\n%# v\n", pretty.Formatter(emojiExcludedString))
			t.Errorf("expected emojis:\n%# v\n", pretty.Formatter(testCase.Output1))
			t.Errorf("expected emojiExcludedString:\n%# v\n", pretty.Formatter(testCase.Output2))
			t.Error("command details do not match.")
		}
	}

}

func TestParseEmojiWorkNameOption(t *testing.T) {
	type TestCase struct {
		Input  string
		Output string
	}
	testCases := [...]TestCase{
		{
			Input:  TestEmojiIn1 + TestEmojiWork0 + "テスト作業名 min=60",
			Output: "テスト作業名",
		},
	}
	for _, testCase := range testCases {
		out := ParseEmojiWorkNameOption(testCase.Input)
		if out != testCase.Output {
			t.Errorf("input: %s\n", testCase.Input)
			t.Errorf("expected: %s\n", testCase.Output)
			t.Errorf("output: %s\n", out)
		}
	}
}
