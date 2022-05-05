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
