package i18n

import (
	"embed"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"strings"
	
	"github.com/BurntSushi/toml"
)

//go:embed locales/*.toml
var fs embed.FS

var (
	ErrLocaleFile = errors.New("i18n: wrong file name or struct")
)

type Language string

const (
	LanguageEN Language = "EN"
	LanguageJP Language = "JP"
	LanguageKO Language = "KO"
	
	LocalesFolderName string = "locales"
)

func isValidLocale(l string) bool {
	list := []string{string(LanguageEN), string(LanguageJP), string(LanguageKO)}
	for _, d := range list {
		if d == l {
			return true
		}
	}
	return false
}

type LocaleData map[string]map[string]string

// LocaleFile is type for {{lang}}.toml
type LocaleFile LocaleData

var localeData map[Language]LocaleData = make(map[Language]LocaleData)
var defaultLanguage Language = LanguageJP
var defaultFallback Language = LanguageEN

type Localizer struct {
	language  Language
	fallback  Language
	namespace string
}

type TFuncType func(key string, args ...interface{}) string

func SetDefaultLanguage(lang Language) {
	defaultLanguage = lang
}

func SetDefaultFallback(fallback Language) {
	defaultFallback = fallback
}

func validateFileName(name string) (Language, error) {
	name = path.Base(name)
	splitedName := strings.Split(name, ".")
	if len(splitedName) != 2 || splitedName[1] != "toml" {
		return LanguageEN, ErrLocaleFile
	}
	localeName := strings.ToUpper(splitedName[0])
	if !isValidLocale(localeName) {
		return LanguageEN, ErrLocaleFile
	}
	return Language(localeName), nil
}

func LoadLocaleFile(name string) error {
	lang, err := validateFileName(name)
	if err != nil {
		return err
	}
	
	var decoded LocaleFile
	if _, err := toml.DecodeFile(name, &decoded); err != nil {
		return err
	}
	
	localeData[lang] = LocaleData(decoded)
	fmt.Printf("%+v\n", localeData)
	return nil
}

func LoadLocaleFileFS(f embed.FS, name string) error {
	lang, err := validateFileName(name)
	if err != nil {
		return err
	}
	
	var decoded LocaleFile
	if _, err := toml.DecodeFS(f, name, &decoded); err != nil {
		return err
	}
	
	localeData[lang] = LocaleData(decoded)
	return nil
}

func LoadLocaleFolder(name string) error {
	files, err := ioutil.ReadDir(name)
	if err != nil {
		return err
	}
	
	for _, file := range files {
		if file.IsDir() {
			return ErrLocaleFile
		}
		if err := LoadLocaleFile(path.Join(name, file.Name())); err != nil {
			return err
		}
	}
	return nil
}

func LoadLocaleFolderFS() error {
	dir, err := fs.ReadDir(LocalesFolderName)
	if err != nil {
		return err
	}
	
	for _, file := range dir {
		if file.IsDir() {
			return ErrLocaleFile
		}
		if err := LoadLocaleFileFS(fs, path.Join(LocalesFolderName, file.Name())); err != nil {
			return err
		}
	}
	return nil
}

func formatText(str string, args ...interface{}) string {
	if len(args) < 1 {
		return str
	}
	oldnew := []string{}
	for i, d := range args {
		oldnew = append(oldnew, fmt.Sprintf("{%d}", i), fmt.Sprintf("%v", d))
	}
	r := strings.NewReplacer(oldnew...)
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
	
	// Fallback
	if value := localeData[fallback][namespace][key]; value != "" {
		return formatText(value, args...)
	}
	
	return fmt.Sprintf("NO DATA[%s]: %s:%s", lang, namespace, key)
}

func T(key string, args ...interface{}) string {
	return t(defaultLanguage, defaultFallback, "", key, args...)
}

func NewLocalizer(namespaces ...string) *Localizer {
	ns := ""
	if len(namespaces) > 0 {
		ns = namespaces[0]
	}
	return &Localizer{
		language:  defaultLanguage,
		namespace: ns,
		fallback:  defaultFallback,
	}
}

func NewLocalizerWithLang(lang Language, namespaces ...string) *Localizer {
	ns := ""
	if len(namespaces) > 0 {
		ns = namespaces[0]
	}
	return &Localizer{
		language:  lang,
		fallback:  defaultFallback,
		namespace: ns,
	}
}

func (l *Localizer) SetLang(lang Language) {
	l.language = lang
}
func (l *Localizer) SetNamespace(namespace string) {
	l.namespace = namespace
}

func (l *Localizer) T(key string, args ...interface{}) string {
	return t(l.language, l.fallback, l.namespace, key, args...)
}

func (l *Localizer) GetTFunc() TFuncType {
	return getTFunc(l.language, l.fallback, l.namespace)
}

func getTFunc(lang, fallback Language, namespace ...string) TFuncType {
	ns := ""
	if len(namespace) > 0 {
		ns = namespace[0]
	}
	return func(key string, args ...interface{}) string {
		return t(lang, fallback, ns, key, args...)
	}
}

func GetTFunc(namespaces ...string) TFuncType {
	return getTFunc(defaultLanguage, defaultFallback, namespaces...)
}

func GetTFuncWithLang(lang Language, namespaces ...string) TFuncType {
	return getTFunc(lang, defaultFallback, namespaces...)
}
