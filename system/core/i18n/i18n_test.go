package i18n_test

import (
	"fmt"
	"testing"
	
	"app.modules/core/i18n"
)

const (
	CommonTestKO     = "안녕"
	CommonTestEN     = "Hello"
	CommonTestArgsEN = "First: 1 | Second: 2"
	CommonTestArgsKO = "두번째: 2 | 첫번째: 1"
)

var (
	CommonTestArgs = []interface{}{1, "2"}
)

// //go:embed *.toml
// var f embed.FS

func TestI18n(test *testing.T) {
	i18n.SetDefaultLanguage(i18n.LanguageJP)
	i18n.SetDefaultFallback(i18n.LanguageEN)
	
	// if err := i18n.LoadLocaleFileFS(f, "ko.toml"); err != nil {
	// 	test.Fatal(err)
	// }
	
	if err := i18n.LoadLocaleFile("ko.toml"); err != nil {
		test.Fatal(err)
	}
	if err := i18n.LoadLocaleFile("en.toml"); err != nil {
		test.Fatal(err)
	}
	
	if i18n.T("common:test") != CommonTestEN { // Check Fallback
		test.Fatal()
	}
	
	{
		t := i18n.GetTFuncWithLang(i18n.LanguageKO, "common")
		if t("test") != CommonTestKO {
			test.Fatal()
		}
	}
	
	{
		t := i18n.NewWithLang(i18n.LanguageKO)
		if t.T("common:test") != CommonTestKO {
			test.Fatal()
		}
		t.SetNamespace("common")
		if t.T("test") != CommonTestKO {
			test.Fatal()
		}
	}
	{
		t := i18n.NewWithLang(i18n.LanguageKO, "common").GetTFunc()
		if t("test") != CommonTestKO {
			test.Fatal()
		}
	}
	{
		ko := i18n.GetTFuncWithLang(i18n.LanguageKO)
		if ko("common:test-args", CommonTestArgs...) != CommonTestArgsKO {
			test.Fatal()
		}
		en := i18n.GetTFuncWithLang(i18n.LanguageEN)
		fmt.Println(en("common:test-args", CommonTestArgs...))
		if en("common:test-args", CommonTestArgs...) != CommonTestArgsEN {
			test.Fatal()
		}
	}
}
