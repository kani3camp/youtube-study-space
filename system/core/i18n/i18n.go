package i18n

import (
	"embed"
	"fmt"

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
	if err := engine.LoadLocaleFileFS(f, name); err != nil {
		return fmt.Errorf("load locale file %q: %w", name, err)
	}
	return nil
}

// LoadLocaleFolderFS loads all locale files from embedded filesystem.
func LoadLocaleFolderFS() error {
	if err := engine.LoadLocaleFolderFS(fs, LocalesFolderName); err != nil {
		return fmt.Errorf("load locale folder %q: %w", LocalesFolderName, err)
	}
	return nil
}
