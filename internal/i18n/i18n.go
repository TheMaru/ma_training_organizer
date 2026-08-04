// Package i18n holds the UI's translations: one embedded JSON catalog per
// supported locale, a lookup for templates and handlers, and Accept-Language
// matching for visitors whose account language is not known yet.
//
// Scope is what a trainer reads in the browser: UI chrome (ADR-0008) plus the
// rank and grading-system names composed from the keys ADR-0009 added. Internal
// errors and CLI output stay English.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"

	"golang.org/x/text/language"
)

//go:embed catalog/*.json
var catalogFS embed.FS

// Locale is a supported UI language, spelled as its catalog's file name and as
// the value stored on a trainer.
type Locale string

const (
	German  Locale = "de"
	English Locale = "en"

	// Default is what a request falls back to: German, both for a visitor whose
	// Accept-Language matches nothing and for a key a catalog is missing.
	Default = German
)

// supported pairs each locale with the language tag Accept-Language is matched
// against. It is the single ordered list: the matcher is built from it and
// answers with an index back into it, so a locale can never be added to one half
// alone.
var supported = []struct {
	Locale Locale
	Tag    language.Tag
}{
	{German, language.German},
	{English, language.English},
}

// catalog is one locale's flat key→text map, keyed by dotted names like
// "nav.logout".
type catalog map[string]string

var (
	catalogs = mustLoadCatalogs()
	matcher  = language.NewMatcher(supportedTags())
)

// Supported lists the locales the UI can render, Default first.
func Supported() []Locale {
	locales := make([]Locale, 0, len(supported))
	for _, s := range supported {
		locales = append(locales, s.Locale)
	}
	return locales
}

// Parse reads a stored or submitted locale value, reporting false for anything
// that is not one of the supported locales. Callers use Default in that case;
// this is the whitelist that keeps an arbitrary string out of the database.
func Parse(s string) (Locale, bool) {
	for _, sup := range supported {
		if Locale(s) == sup.Locale {
			return sup.Locale, true
		}
	}
	return Default, false
}

// Match picks the best supported locale for an Accept-Language header, falling
// back to Default for an empty, unparseable or unmatched header.
func Match(header string) Locale {
	tags, _, err := language.ParseAcceptLanguage(header)
	if err != nil {
		return Default
	}
	_, index, _ := matcher.Match(tags...)
	return supported[index].Locale
}

// T translates key into the given locale. Any args are interpolated into the
// catalog value's verbs (fmt.Sprintf), which is how a message carries a name. A
// key the locale lacks falls back to Default, and one no catalog has renders as
// the key itself, so a gap shows in the UI rather than as a blank.
func T(locale Locale, key string, args ...any) string {
	return translate(catalogs, locale, key, args...)
}

// Lookup translates key like T but reports whether any catalog carried it, for a
// caller that has a better answer than the key itself. The composed rank and
// system names use it: they fall back to the stored English name (ADR-0009), and
// "rank.degree.7" as an athlete's rank would be worse than "White, 7 stripes".
func Lookup(locale Locale, key string, args ...any) (string, bool) {
	return lookup(catalogs, locale, key, args...)
}

func translate(cs map[Locale]catalog, locale Locale, key string, args ...any) string {
	text, ok := lookup(cs, locale, key, args...)
	if !ok {
		return key
	}
	return text
}

func lookup(cs map[Locale]catalog, locale Locale, key string, args ...any) (string, bool) {
	text, ok := cs[locale][key]
	if !ok {
		text, ok = cs[Default][key]
	}
	if !ok {
		return "", false
	}
	if len(args) == 0 {
		return text, true
	}
	return fmt.Sprintf(text, args...), true
}

func supportedTags() []language.Tag {
	tags := make([]language.Tag, 0, len(supported))
	for _, s := range supported {
		tags = append(tags, s.Tag)
	}
	return tags
}

func mustLoadCatalogs() map[Locale]catalog {
	cs := make(map[Locale]catalog, len(supported))
	for _, sup := range supported {
		raw, err := catalogFS.ReadFile("catalog/" + string(sup.Locale) + ".json")
		if err != nil {
			panic("i18n: read catalog " + string(sup.Locale) + ": " + err.Error())
		}
		var c catalog
		if err := json.Unmarshal(raw, &c); err != nil {
			panic("i18n: parse catalog " + string(sup.Locale) + ": " + err.Error())
		}
		cs[sup.Locale] = c
	}
	return cs
}
