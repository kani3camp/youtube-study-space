package i18n_test

import (
	"sync"
	"testing"

	"app.modules/core/i18n"
	i18nmsg "app.modules/core/i18n/typed"
	"github.com/stretchr/testify/assert"
)

func TestI18nRealWorldUsage(t *testing.T) {
	i18n.SetDefaultLanguage(i18n.LanguageJA)
	i18n.SetDefaultFallback(i18n.LanguageJA)

	err := i18n.LoadLocaleFolderFS()
	assert.NoError(t, err, "Failed to load locale files")

	t.Run("BasicTranslation", func(t *testing.T) {
		workMsg := i18n.T("common:work")
		assert.NotEmpty(t, workMsg)
		assert.Contains(t, workMsg, "作業中")

		breakMsg := i18n.T("common:break")
		assert.NotEmpty(t, breakMsg)
		assert.Contains(t, breakMsg, "休憩中")
	})

	t.Run("ParameterReplacement", func(t *testing.T) {
		exitMsg := i18n.T("command:exit", "太郎", 45, "3", "+ 5 RP✨")
		assert.Contains(t, exitMsg, "太郎")
		assert.Contains(t, exitMsg, "45分")
		assert.Contains(t, exitMsg, "3番席")
		assert.Contains(t, exitMsg, "+ 5 RP✨")

		startMsg := i18n.T("command-in:start", "花子", "数学の勉強", 120, "5")
		assert.Contains(t, startMsg, "花子")
		assert.Contains(t, startMsg, "数学の勉強")
		assert.Contains(t, startMsg, "120分")
		assert.Contains(t, startMsg, "5番席")
	})

	t.Run("TypedFunctions", func(t *testing.T) {
		workMsg := i18nmsg.CommonWork()
		assert.NotEmpty(t, workMsg)
		assert.Contains(t, workMsg, "作業中")

		exitMsg := i18nmsg.CommandExit("太郎", 45, "3", "+ 5 RP✨")
		assert.Contains(t, exitMsg, "太郎")
		assert.Contains(t, exitMsg, "45分")

		errorMsg := i18nmsg.CommandError("次郎")
		assert.Contains(t, errorMsg, "次郎")
		assert.Contains(t, errorMsg, "エラー")
	})

	t.Run("LanguageSwitching", func(t *testing.T) {
		jaFunc := i18n.GetTFuncWithLang(i18n.LanguageJA)
		koFunc := i18n.GetTFuncWithLang(i18n.LanguageKO)

		jaWork := jaFunc("common:work")
		koWork := koFunc("common:work")

		assert.NotEmpty(t, jaWork)
		assert.NotEmpty(t, koWork)
		assert.NotEqual(t, jaWork, koWork, "Japanese and Korean translations should be different")
	})

	t.Run("LocalizerWithNamespace", func(t *testing.T) {
		localizer := i18n.NewLocalizerWithLang(i18n.LanguageJA)
		localizer.SetNamespace("command")

		errorMsg := localizer.T("error", "ユーザー")
		assert.Contains(t, errorMsg, "ユーザー")
		assert.Contains(t, errorMsg, "エラー")

		exitMsg := localizer.T("exit", "太郎", 30, "1", "")
		assert.Contains(t, exitMsg, "太郎")
		assert.Contains(t, exitMsg, "30分")
	})

	t.Run("NonExistentKey", func(t *testing.T) {
		result := i18n.T("nonexistent:key")
		assert.Contains(t, result, "TRANSLATION DATA NOT FOUND", "Non-existent key should return error message")
	})

	t.Run("SpecialCharactersAndEmoji", func(t *testing.T) {
		workMsg := i18n.T("common:work")
		assert.Contains(t, workMsg, "💪", "Emoji should be preserved")

		breakMsg := i18n.T("common:break")
		assert.Contains(t, breakMsg, "☕", "Emoji should be preserved")

		startMsg := i18n.T("command-in:start", "太郎", "プログラミング", 60, "1")
		assert.Contains(t, startMsg, "🔥", "Emoji in messages should be preserved")
	})

	t.Run("ComplexMessageWithMultipleParameters", func(t *testing.T) {
		seatMoveMsg := i18n.T("command-in:seat-move", 
			"ユーザー", "勉強", "1", "2", 30, "+ 10 RP", 90)
		assert.Contains(t, seatMoveMsg, "ユーザー")
		assert.Contains(t, seatMoveMsg, "勉強")
		assert.Contains(t, seatMoveMsg, "1→2")
		assert.Contains(t, seatMoveMsg, "30分")
		assert.Contains(t, seatMoveMsg, "90分後")
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		var wg sync.WaitGroup
		errors := make([]error, 0)
		var mu sync.Mutex

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				for j := 0; j < 100; j++ {
					msg := i18n.T("common:work")
					if msg == "" {
						mu.Lock()
						errors = append(errors, assert.AnError)
						mu.Unlock()
					}
					
					localizer := i18n.NewLocalizerWithLang(i18n.LanguageJA)
					exitMsg := localizer.T("command:exit", "user", id, j, "")
					if exitMsg == "" {
						mu.Lock()
						errors = append(errors, assert.AnError)
						mu.Unlock()
					}
				}
			}(i)
		}

		wg.Wait()
		assert.Empty(t, errors, "Concurrent access should be thread-safe")
	})
}

func TestI18nFallback(t *testing.T) {
	i18n.SetDefaultLanguage(i18n.LanguageKO)
	i18n.SetDefaultFallback(i18n.LanguageJA)

	err := i18n.LoadLocaleFolderFS()
	assert.NoError(t, err)

	existingKey := i18n.T("common:work")
	assert.NotEmpty(t, existingKey, "Existing key in Korean should return Korean translation")

	koLocalizer := i18n.NewLocalizerWithLang(i18n.LanguageKO)
	koWork := koLocalizer.T("common:work")
	assert.NotEmpty(t, koWork)

	jaLocalizer := i18n.NewLocalizerWithLang(i18n.LanguageJA)
	jaWork := jaLocalizer.T("common:work")
	assert.NotEmpty(t, jaWork)
	assert.NotEqual(t, koWork, jaWork, "Korean and Japanese should have different translations")
}

func TestI18nEdgeCases(t *testing.T) {
	i18n.SetDefaultLanguage(i18n.LanguageJA)
	i18n.SetDefaultFallback(i18n.LanguageJA)
	
	err := i18n.LoadLocaleFolderFS()
	assert.NoError(t, err)

	t.Run("MissingParameters", func(t *testing.T) {
		result := i18n.T("command:exit")
		assert.NotPanics(t, func() {
			_ = i18n.T("command:exit")
		}, "Missing parameters should not panic")
		assert.Contains(t, result, "{0}")
	})

	t.Run("ExtraParameters", func(t *testing.T) {
		assert.NotPanics(t, func() {
			_ = i18n.T("common:work", "extra1", "extra2", "extra3")
		}, "Extra parameters should not panic")
	})

	t.Run("EmptyNamespace", func(t *testing.T) {
		result := i18n.T(":work")
		assert.Contains(t, result, "TRANSLATION DATA NOT FOUND", "Empty namespace should return error message")
	})

	t.Run("EmptyKey", func(t *testing.T) {
		result := i18n.T("common:")
		assert.Contains(t, result, "TRANSLATION DATA NOT FOUND", "Empty key should return error message")
	})
}