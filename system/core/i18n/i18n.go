package i18n

import (
	"embed"

	engine "app.modules/core/i18n/internal/engine"
)

//go:embed locales/*.toml
var fs embed.FS

// Re-export selected types/constants for public API
type Language = engine.Language

const (
	LanguageJA = engine.LanguageJA
	LanguageKO = engine.LanguageKO

	LocalesFolderName string = "locales"
)

func SetDefaultLanguage(lang Language) {
	engine.SetDefaultLanguage(engine.Language(lang))
}

func SetDefaultFallback(fallback Language) {
	engine.SetDefaultFallback(engine.Language(fallback))
}

func LoadLocaleFileFS(f embed.FS, name string) error {
	return engine.LoadLocaleFileFS(f, name)
}

// LoadLocaleFolderFS loads all locale files from embedded filesystem.
func LoadLocaleFolderFS() error {
	return engine.LoadLocaleFolderFS(fs, LocalesFolderName)
}
