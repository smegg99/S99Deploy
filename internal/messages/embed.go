// internal/messages/embed.go

// Package messages carries the generated S99Locale catalogs and their accessors.
package messages

import (
	"embed"
	"encoding/json"
	"os"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// Files holds the catalogs the logger and both command lines read.
//
//go:embed locales/*.json
var Files embed.FS

// Names are the embedded catalog paths, one per generated locale.
var Names = catalogNames()

var bundle = newBundle()

// catalogNames is the embedded path loc writes each locale to.
func catalogNames() []string {
	names := make([]string, 0, len(Locales))
	for _, locale := range Locales {
		names = append(names, "locales/"+locale+".json")
	}
	return names
}

// newBundle loads every catalog once, at init.
func newBundle() *i18n.Bundle {
	built := i18n.NewBundle(language.Make(DefaultLocale))
	built.RegisterUnmarshalFunc("json", json.Unmarshal)
	for _, name := range Names {
		// A catalog that does not parse is a build problem, not a runtime one:
		// it is generated and embedded, so it is the same bytes every run.
		if _, err := built.LoadMessageFileFS(Files, name); err != nil {
			panic("messages: load " + name + ": " + err.Error())
		}
	}
	return built
}

// Localizer resolves messages in one language, falling back to the default.
func Localizer(lang string) *i18n.Localizer {
	if lang == "" {
		lang = DefaultLocale
	}
	return i18n.NewLocalizer(bundle, lang, DefaultLocale)
}

// Has reports whether this tool ships a catalog for lang.
func Has(lang string) bool {
	for _, locale := range Locales {
		if locale == lang {
			return true
		}
	}
	return false
}

// LanguageFromEnv is the precedence below --lang, ending at the catalog default.
func LanguageFromEnv() string {
	for _, name := range []string{"S99DEPLOY_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			continue
		}

		// A shell writes pl_PL.UTF-8; a catalog is named for the language.
		value, _, _ = strings.Cut(value, ".")
		value, _, _ = strings.Cut(value, "@")
		if strings.EqualFold(value, "C") || strings.EqualFold(value, "POSIX") {
			return DefaultLocale
		}
		value, _, _ = strings.Cut(value, "_")
		return strings.ToLower(value)
	}
	return DefaultLocale
}
