// Package i18n provides the UI message catalog for Waldi's two supported
// languages (Persian/RTL and English/LTR).
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed locales/*.json
var localeFiles embed.FS

// Default is the language used when nothing else resolves a locale.
const Default = "fa"

var supported = map[string]bool{
	"fa": true,
	"en": true,
}

var catalog map[string]map[string]string

func init() {
	catalog = make(map[string]map[string]string, len(supported))
	for lang := range supported {
		data, err := localeFiles.ReadFile("locales/" + lang + ".json")
		if err != nil {
			panic(fmt.Sprintf("i18n: reading locale %q: %v", lang, err))
		}
		var messages map[string]string
		if err := json.Unmarshal(data, &messages); err != nil {
			panic(fmt.Sprintf("i18n: parsing locale %q: %v", lang, err))
		}
		catalog[lang] = messages
	}
}

// Supported reports whether lang is one of Waldi's supported UI languages.
func Supported(lang string) bool {
	return supported[lang]
}

// ReaderLang normalizes a reader's UI locale for content matching.
func ReaderLang(locale string) string {
	if Supported(locale) {
		return locale
	}
	return Default
}

// Dir returns the text direction for a supported language, defaulting to
// the direction of Default for anything unrecognized.
func Dir(lang string) string {
	if lang == "en" {
		return "ltr"
	}
	return "rtl"
}

// T looks up key in the catalog for lang, falling back to the default
// language and finally to the raw key if no translation exists.
// When args are given, the resolved template is passed through fmt.Sprintf.
func T(lang, key string, args ...any) string {
	if !supported[lang] {
		lang = Default
	}
	message, ok := catalog[lang][key]
	if !ok && lang != Default {
		message, ok = catalog[Default][key]
	}
	if !ok {
		message = key
	}
	if len(args) == 0 || !strings.Contains(message, "%") {
		return message
	}
	return fmt.Sprintf(message, args...)
}

// TN resolves a count-aware message: key+".one" when n == 1, otherwise
// key+".other". Languages that don't inflect by count (e.g. Persian nouns)
// can simply omit the plural variants and fall back to the bare key. When
// args is empty, n itself is used as the sole format argument, since the
// common case is a single "%d ..." placeholder driving both the plural
// choice and the printed count.
func TN(lang, key string, n int, args ...any) string {
	suffix := ".other"
	if n == 1 {
		suffix = ".one"
	}
	if len(args) == 0 {
		args = []any{n}
	}
	pkey := key + suffix
	if !supported[lang] {
		lang = Default
	}
	if _, ok := catalog[lang][pkey]; !ok {
		if _, ok := catalog[Default][pkey]; !ok {
			return T(lang, key, args...)
		}
	}
	return T(lang, pkey, args...)
}
