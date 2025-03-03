package utils

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestExtractAllEmojiCommands(t *testing.T) {
	type TestCase struct {
		Name    string
		Input   string
		Output1 []EmojiElement
		Output2 string
	}
	testCases := []TestCase{
		{
			Name:  "Multiple emoji commands",
			Input: TestEmojiIn0 + TestEmoji360Min0,
			Output1: []EmojiElement{
				EmojiIn,
				EmojiMin,
			},
			Output2: "",
		},
		{
			Name:    "No emoji commands",
			Input:   "!in",
			Output1: []EmojiElement{},
			Output2: "!in",
		},
		{
			Name:  "Emoji commands with text",
			Input: " " + TestEmojiMy0 + TestEmojiColor0 + "ピンク",
			Output1: []EmojiElement{
				EmojiMy,
				EmojiColor,
			},
			Output2: "ピンク",
		},
		{
			Name:    "Multiple emoji commands at different positions",
			Input:   "Hello " + TestEmojiIn0 + " world " + TestEmojiOut0,
			Output1: []EmojiElement{EmojiIn, EmojiOut},
			Output2: "Hello   world  ",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			emojis, emojiExcludedString := ExtractAllEmojiCommands(testCase.Input)
			assert.Equal(t, testCase.Output1, emojis, "Extracted emoji elements don't match")
			assert.Equal(t, testCase.Output2, emojiExcludedString, "Emoji excluded string doesn't match")
		})
	}
}

func TestParseEmojiWorkNameOption(t *testing.T) {
	type TestCase struct {
		Name   string
		Input  string
		Output string
	}
	testCases := []TestCase{
		{
			Name:   "Basic work name extraction",
			Input:  TestEmojiIn1 + TestEmojiWork0 + "テスト作業名 min=60",
			Output: "テスト作業名",
		},
		{
			Name:   "Empty work name",
			Input:  TestEmojiIn1 + TestEmojiWork0 + " min=60",
			Output: "",
		},
		{
			Name:   "Work name with special characters",
			Input:  TestEmojiIn1 + TestEmojiWork0 + "特殊文字!@#$%^&*() min=60",
			Output: "特殊文字!@#$%^&*()",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			result := ParseEmojiWorkNameOption(testCase.Input)
			assert.Equal(t, testCase.Output, result, "Parsed work name doesn't match expected output")
		})
	}
}
