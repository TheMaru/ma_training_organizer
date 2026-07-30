package i18n_test

import (
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/i18n"
)

func TestTranslatesIntoTheRequestedLocale(t *testing.T) {
	if got := i18n.T(i18n.German, "nav.logout"); got != "Abmelden" {
		t.Errorf("German nav.logout = %q, want %q", got, "Abmelden")
	}
	if got := i18n.T(i18n.English, "nav.logout"); got != "Log out" {
		t.Errorf("English nav.logout = %q, want %q", got, "Log out")
	}
}

// An unknown key must stay visible rather than render as an empty string, so a
// gap shows up in the UI instead of silently swallowing a label.
func TestUnknownKeyRendersAsItself(t *testing.T) {
	if got := i18n.T(i18n.English, "nav.nope"); got != "nav.nope" {
		t.Errorf("unknown key = %q, want the key itself", got)
	}
}

func TestUnknownLocaleFallsBackToGerman(t *testing.T) {
	if got := i18n.T(i18n.Locale("fr"), "nav.logout"); got != i18n.T(i18n.German, "nav.logout") {
		t.Errorf("unknown locale = %q, want the German value", got)
	}
}

func TestTranslationInterpolatesArguments(t *testing.T) {
	got := i18n.T(i18n.English, "roster.deleteConfirm", "Ada", "Lovelace")
	want := "Delete Ada Lovelace? Their promotions are deleted too."
	if got != want {
		t.Errorf("deleteConfirm = %q, want %q", got, want)
	}
}

func TestMatchResolvesAcceptLanguage(t *testing.T) {
	tests := []struct {
		header string
		want   i18n.Locale
	}{
		{"", i18n.German},
		{"en", i18n.English},
		{"en-US,en;q=0.9", i18n.English},
		{"de-AT,de;q=0.9", i18n.German},
		{"fr-FR,fr;q=0.9", i18n.German},
		{"en-GB;q=0.8,de;q=0.9", i18n.German},
		{"nonsense ;;;", i18n.German},
	}
	for _, tt := range tests {
		if got := i18n.Match(tt.header); got != tt.want {
			t.Errorf("Match(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestParseAcceptsSupportedLocalesOnly(t *testing.T) {
	tests := []struct {
		in    string
		want  i18n.Locale
		valid bool
	}{
		{"de", i18n.German, true},
		{"en", i18n.English, true},
		{"", i18n.German, false},
		{"fr", i18n.German, false},
		{"de-AT", i18n.German, false},
	}
	for _, tt := range tests {
		got, ok := i18n.Parse(tt.in)
		if ok != tt.valid || got != tt.want {
			t.Errorf("Parse(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.valid)
		}
	}
}
