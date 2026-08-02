package engine

import (
	"embed"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
)

var (
	ErrLocaleFile = errors.New("i18n: wrong file name or struct")
)

type Language string

const (
	LanguageJA Language = "JA"
	LanguageKO Language = "KO"
)

type LocaleData map[string]map[string]string

var localeData map[Language]LocaleData = make(map[Language]LocaleData)
var defaultLanguage Language = LanguageJA
var defaultFallback Language = LanguageJA

func SetDefaultLanguage(lang Language)     { defaultLanguage = lang }
func SetDefaultFallback(fallback Language) { defaultFallback = fallback }

func isValidLocale(l string) bool {
	list := []string{string(LanguageJA), string(LanguageKO)}
	for _, d := range list {
		if d == l {
			return true
		}
	}
	return false
}

func validateFileName(name string) (Language, error) {
	name = path.Base(name)
	splitName := strings.Split(name, ".")
	if len(splitName) != 2 || splitName[1] != "toml" {
		return "", ErrLocaleFile
	}
	localeName := strings.ToUpper(splitName[0])
	if !isValidLocale(localeName) {
		return "", ErrLocaleFile
	}
	return Language(localeName), nil
}

// LoadLocaleFileFS loads a single TOML locale file from the given fs
func LoadLocaleFileFS(f embed.FS, name string) error {
	lang, err := validateFileName(name)
	if err != nil {
		return fmt.Errorf("in validateFileName: %w", err)
	}
	var decoded LocaleData
	if _, err := toml.DecodeFS(f, name, &decoded); err != nil {
		return fmt.Errorf("in toml.DecodeFS: %w", err)
	}
	localeData[lang] = decoded
	return nil
}

// LoadLocaleFolderFS loads all locale files from embedded filesystem folder "locales"
func LoadLocaleFolderFS(f embed.FS, folder string) error {
	dir, err := f.ReadDir(folder)
	if err != nil {
		return fmt.Errorf("in fs.ReadDir: %w", err)
	}
	for _, file := range dir {
		if file.IsDir() {
			return ErrLocaleFile
		}
		if err := LoadLocaleFileFS(f, path.Join(folder, file.Name())); err != nil {
			return fmt.Errorf("in LoadLocaleFileFS: %w", err)
		}
	}
	return nil
}

func formatText(str string, args ...interface{}) string {
	if len(args) < 1 {
		return str
	}
	var oldNew []string
	for i, d := range args {
		oldNew = append(oldNew, fmt.Sprintf("{%d}", i), fmt.Sprintf("%v", d))
	}
	r := strings.NewReplacer(oldNew...)
	return r.Replace(str)
}

func t(lang, fallback Language, namespace, key string, args ...interface{}) string {
	if namespace == "" {
		splited := strings.Split(key, ":")
		if len(splited) != 2 {
			return "wrong name"
		}
		namespace = splited[0]
		key = splited[1]
	}
	if value := localeData[lang][namespace][key]; value != "" {
		return formatText(value, args...)
	}
	if value := localeData[fallback][namespace][key]; value != "" {
		return formatText(value, args...)
	}
	return fmt.Sprintf("TRANSLATION DATA NOT FOUND. [%s]: %s:%s", lang, namespace, key)
}

// TranslateDefault translates using default language and fallback
func TranslateDefault(key string, args ...interface{}) string {
	return t(defaultLanguage, defaultFallback, "", key, args...)
}
