package utils

import (
	"testing"

	"app.modules/core/i18n"
	"github.com/stretchr/testify/assert"
)

func TestParseMy(t *testing.T) {
	testCases := []ParseCommandTestCase{
		{
			Name:  "ランク表示オンの設定",
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
			Name:  "ランク表示オフの設定",
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
			Name:  "オプションなしの設定",
			Input: "!my",
			Output: &CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},
		{
			Name:  "デフォルト勉強時間設定",
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
			Name:  "デフォルト勉強時間リセット",
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
			Name:  "お気に入り色リセット",
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
			Name:  "複数オプション設定",
			Input: "!my min=40 color=ピンク  rank=off",
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:      RankVisible,
						BoolValue: false,
					},
					{
						Type:     DefaultStudyMin,
						IntValue: 40,
					},
					{
						Type:        FavoriteColor,
						StringValue: "ピンク",
					},
				},
			},
		},
		{
			Name:     "非メンバーによる絵文字!my設定（無効）",
			Input:    TestEmojiMy0 + TestEmojiColor0 + "白 " + TestEmojiMin0 + "100 " + TestEmojiRankOn0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: NotCommand,
			},
		},
		{
			Name:     "非メンバーによる絵文字!my設定（無効）",
			Input:    "!my " + TestEmojiColor0 + "白 " + TestEmojiMin0 + "100 " + TestEmojiRankOn0,
			IsMember: false,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},

		{
			Name:     "メンバーによる!my設定",
			Input:    "!my color=白 min=200 rank=off",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:      RankVisible,
						BoolValue: false,
					},
					{
						Type:     DefaultStudyMin,
						IntValue: 200,
					},
					{
						Type:        FavoriteColor,
						StringValue: "白",
					},
				},
			},
		},
		{
			Name:     "メンバーによる空の設定",
			Input:    "!my ",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},
		{
			Name:     "メンバーによる絵文字!my設定",
			Input:    TestEmojiMy0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions:   []MyOption{},
			},
		},
		{
			Name:     "メンバーによる絵文字!my設定（ランクオン）",
			Input:    TestEmojiMy0 + TestEmojiRankOn0,
			IsMember: true,
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
			Name:     "メンバーによる絵文字!my設定（ランクオフ）",
			Input:    TestEmojiMy0 + TestEmojiRankOff0,
			IsMember: true,
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
			Name:     "メンバーによる絵文字!my設定（デフォルト作業時間）",
			Input:    TestEmojiMy0 + TestEmojiMin0 + "100",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:     DefaultStudyMin,
						IntValue: 100,
					},
				},
			},
		},
		{
			Name:     "メンバーによる絵文字!my設定（お気に入りカラー）",
			Input:    TestEmojiMy0 + TestEmojiColor0 + "白",
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:        FavoriteColor,
						StringValue: "白",
					},
				},
			},
		},
		{
			Name:     "メンバーによる絵文字!my設定（複数オプション）",
			Input:    TestEmojiMy0 + TestEmojiColor0 + "白 " + TestEmojiMin0 + "100 " + TestEmojiRankOn0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:      RankVisible,
						BoolValue: true,
					},
					{
						Type:     DefaultStudyMin,
						IntValue: 100,
					},
					{
						Type:        FavoriteColor,
						StringValue: "白",
					},
				},
			},
		},
		{
			Name:     "絵文字コマンドの隣は空白なしも可",
			Input:    TestEmojiMy0 + TestEmojiColor0 + "白" + TestEmojiMin0 + "100" + TestEmojiRankOn0,
			IsMember: true,
			Output: &CommandDetails{
				CommandType: My,
				MyOptions: []MyOption{
					{
						Type:      RankVisible,
						BoolValue: true,
					},
					{
						Type:     DefaultStudyMin,
						IntValue: 100,
					},
					{
						Type:        FavoriteColor,
						StringValue: "白",
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
