package i18n

import "testing"

// The parity guard from the spec: a translation forgotten in one catalog fails
// here rather than surfacing as a German string in an English UI.
func TestCatalogsHaveIdenticalKeys(t *testing.T) {
	de, en := catalogs[German], catalogs[English]
	if len(de) == 0 {
		t.Fatal("the German catalog is empty")
	}
	for key := range de {
		if _, ok := en[key]; !ok {
			t.Errorf("key %q is missing from the English catalog", key)
		}
	}
	for key := range en {
		if _, ok := de[key]; !ok {
			t.Errorf("key %q is missing from the German catalog", key)
		}
	}
}

func TestCatalogValuesAreNonEmpty(t *testing.T) {
	for locale, c := range catalogs {
		for key, text := range c {
			if text == "" {
				t.Errorf("%s catalog has an empty value for %q", locale, key)
			}
		}
	}
}

// The real catalogs cannot exercise this: the parity test forbids a key that
// exists in one catalog but not the other. So the fallback chain is tested
// against a synthetic pair, one key short on the English side.
func TestMissingKeyFallsBackToTheDefaultLocale(t *testing.T) {
	cs := map[Locale]catalog{
		German:  {"greeting": "Hallo", "only.de": "nur Deutsch"},
		English: {"greeting": "Hello"},
	}

	if got := translate(cs, English, "only.de"); got != "nur Deutsch" {
		t.Errorf("missing English key = %q, want the German value", got)
	}
	if got := translate(cs, English, "absent.everywhere"); got != "absent.everywhere" {
		t.Errorf("key missing everywhere = %q, want the key itself", got)
	}
}
