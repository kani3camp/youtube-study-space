package core

import (
	"github.com/kr/pretty"
	"reflect"
	"testing"
)

type TestCase struct {
	Input  string
	Output CommandDetails
}

func TestParseCommand(t *testing.T) {
	testCases := [...]TestCase{
		{
			Input: "in",
			Output: CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Input: "!in",
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Input: "!in work-てすと min-50",
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: MinutesAndWorkNameOption{
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
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: MinutesAndWorkNameOption{
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
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: MinutesAndWorkNameOption{
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
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: false,
					MinutesAndWorkName: MinutesAndWorkNameOption{
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
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      0,
					MinutesAndWorkName: MinutesAndWorkNameOption{
						IsWorkNameSet:    false,
						IsDurationMinSet: false,
					},
				},
			},
		},
		{
			Input: "!1 work=work min=35",
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      1,
					MinutesAndWorkName: MinutesAndWorkNameOption{
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
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      300,
					MinutesAndWorkName: MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: false,
						WorkName:         "ｙ",
					},
				},
			},
		},
		{
			Input: "!300 w＝全角イコール m＝165",
			Output: CommandDetails{
				CommandType: In,
				InOption: InOption{
					IsSeatIdSet: true,
					SeatId:      300,
					MinutesAndWorkName: MinutesAndWorkNameOption{
						IsWorkNameSet:    true,
						IsDurationMinSet: true,
						WorkName:         "全角イコール",
						DurationMin:      165,
					},
				},
			},
		},
		
		{
			Input: "!out",
			Output: CommandDetails{
				CommandType: Out,
			},
		},
		
		{
			Input: "!info",
			Output: CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: false,
				},
			},
		},
		{
			Input: "!info d",
			Output: CommandDetails{
				CommandType: Info,
				InfoOption: InfoOption{
					ShowDetails: true,
				},
			},
		},
		
		{
			Input: "!my rank=on",
			Output: CommandDetails{
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
			Output: CommandDetails{
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
			Output: CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},
		{
			Input: "!my min=500",
			Output: CommandDetails{
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
			Output: CommandDetails{
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
			Output: CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:     FavoriteColor,
						IntValue: -1,
					},
				},
			},
		},
		
		{
			Input: "!change m=140 w=新",
			Output: CommandDetails{
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
			Input: "!rank",
			Output: CommandDetails{
				CommandType: Rank,
			},
		},
		
		{
			Input: "!more 123",
			Output: CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Input: "!more m=123",
			Output: CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Input: "!more m＝123",
			Output: CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		{
			Input: "!more min=123",
			Output: CommandDetails{
				CommandType: More,
				MoreOption: MoreOption{
					DurationMin: 123,
				},
			},
		},
		
		{
			Input: "!break",
			Output: CommandDetails{
				CommandType: Break,
				BreakOption: MinutesAndWorkNameOption{
					IsWorkNameSet:    false,
					IsDurationMinSet: false,
				},
			},
		},
		{
			Input: "!break min=23 work=休憩",
			Output: CommandDetails{
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
			Input: "!resume",
			Output: CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: false,
				},
			},
		},
		{
			Input: "!resume work=再開！",
			Output: CommandDetails{
				CommandType: Resume,
				ResumeOption: WorkNameOption{
					IsWorkNameSet: true,
					WorkName:      "再開！",
				},
			},
		},
		
		{
			Input: "!seat",
			Output: CommandDetails{
				CommandType: Seat,
			},
		},
		
		{
			Input: "!kick 12",
			Output: CommandDetails{
				CommandType: Kick,
				KickOption: KickOption{
					SeatId: 12,
				},
			},
		},
		
		{
			Input: "!check 14",
			Output: CommandDetails{
				CommandType: Check,
				CheckOption: CheckOption{
					SeatId: 14,
				},
			},
		},
		
		{
			Input: "!report めっせーじ",
			Output: CommandDetails{
				CommandType: Report,
				ReportOption: ReportOption{
					Message: "!report めっせーじ",
				},
			},
		},
		{
			Input: "!report　全角すぺーすめっせーじ",
			Output: CommandDetails{
				CommandType: Report,
				ReportOption: ReportOption{
					Message: "!report 全角すぺーすめっせーじ",
				},
			},
		},
	}
	
	for _, testCase := range testCases {
		commandDetails, err := ParseCommand(testCase.Input)
		if err.IsNotNil() {
			t.Error(err)
		}
		if !reflect.DeepEqual(commandDetails, testCase.Output) {
			t.Errorf("input: %s\n", testCase.Input)
			t.Errorf("result:\n%# v\n", pretty.Formatter(commandDetails))
			t.Errorf("expected:\n%# v\n", pretty.Formatter(testCase.Output))
			t.Error("command details do not match.")
		}
	}
}
