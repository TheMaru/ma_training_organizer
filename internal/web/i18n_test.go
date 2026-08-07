package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// getWithLanguage performs a GET carrying an Accept-Language header, the only
// locale signal a request has before a trainer is known.
func getWithLanguage(t *testing.T, ts *httptest.Server, client *http.Client, path, acceptLanguage string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if err != nil {
		t.Fatalf("build GET %s: %v", path, err)
	}
	req.Header.Set("Accept-Language", acceptLanguage)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// switchTo posts the language switcher's form.
func switchTo(t *testing.T, ts *httptest.Server, client *http.Client, locale, returnTo string) *http.Response {
	t.Helper()
	return post(t, ts, client, "/account/language", url.Values{
		"locale": {locale},
		"return": {returnTo},
	})
}

func TestLoginPageFollowsAcceptLanguage(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	body := readBody(t, getWithLanguage(t, ts, client, "/login", "en-US,en;q=0.9"))
	if !strings.Contains(body, "Sign in") {
		t.Error("English Accept-Language did not yield an English login page")
	}
	if !strings.Contains(body, `<html lang="en">`) {
		t.Error(`want <html lang="en"> on the English login page`)
	}
}

func TestLoginPageFallsBackToGerman(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)

	// A language the app does not speak is not an error — it resolves to German,
	// as does a request with no preference at all.
	for _, header := range []string{"fr-FR,fr;q=0.9", ""} {
		body := readBody(t, getWithLanguage(t, ts, client, "/login", header))
		if !strings.Contains(body, "Anmelden") {
			t.Errorf("Accept-Language %q did not fall back to German", header)
		}
		if !strings.Contains(body, `<html lang="de">`) {
			t.Errorf(`Accept-Language %q: want <html lang="de">`, header)
		}
	}
}

func TestLanguageChoicePersistsOnTheAccount(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp := switchTo(t, ts, client, "en", "/athletes")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusSeeOther)
	}
	if got := resp.Header.Get("Location"); got != "/athletes" {
		t.Errorf("Location = %q, want %q", got, "/athletes")
	}

	body := readBody(t, get(t, ts, client, "/athletes"))
	if !strings.Contains(body, "New athlete") {
		t.Error("the roster stayed German after switching to English")
	}

	// A second login from a fresh session on a different device, whose browser asks
	// for German: the account's choice outranks Accept-Language and the session.
	other := newClient(t)
	login(t, ts, other, testUsername, testPassword).Body.Close()
	body = readBody(t, getWithLanguage(t, ts, other, "/athletes", "de-DE,de;q=0.9"))
	if !strings.Contains(body, "New athlete") {
		t.Error("the stored language did not survive into a new session")
	}
	if !strings.Contains(body, `<html lang="en">`) {
		t.Error(`want <html lang="en"> for a trainer whose account is English`)
	}
}

func TestUnsupportedLanguageResolvesToGerman(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	switchTo(t, ts, client, "en", "/athletes").Body.Close()

	switchTo(t, ts, client, "fr", "/athletes").Body.Close()

	body := readBody(t, get(t, ts, client, "/athletes"))
	if !strings.Contains(body, "Neuer Athlet") {
		t.Error("an unsupported language did not resolve to German")
	}
}

func TestLanguageSwitchOnlyReturnsWithinTheApp(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	// Each of these travels percent-encoded in the form body and arrives at the
	// handler as written here — the tab as %09, which is how it reaches the check
	// at all. The character classes themselves are pinned in locale_test.go.
	for _, target := range []string{
		"https://evil.example/",
		"//evil.example/",
		`/\evil.example/`,
		"/athletes\r\nX: y",
		"/\t/evil.example",
		"/athletes\x7f",
		"",
	} {
		resp := switchTo(t, ts, client, "en", target)
		location := resp.Header.Get("Location")
		resp.Body.Close()
		if location != "/" {
			t.Errorf("return target %q redirected to %q, want /", target, location)
		}
	}
}

func TestGoSideMessagesFollowTheLocale(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	switchTo(t, ts, client, "en", "/athletes").Body.Close()

	// A validation message the handler produces itself, not the template.
	body := readBody(t, post(t, ts, client, "/athletes", url.Values{
		"firstName": {""},
		"lastName":  {""},
	}))
	if !strings.Contains(body, "First and last name are required.") {
		t.Error("the validation message stayed German for an English trainer")
	}
	// And the form heading, which the handler passes in as a catalog key.
	if !strings.Contains(body, "New athlete") {
		t.Error("the form heading stayed German for an English trainer")
	}
}

// The desktop headers and the phone cards' data-labels must move together.
func TestRosterColumnLabelsAndCardLabelsStayInSync(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	mixedRoster(t, db)
	switchTo(t, ts, client, "en", "/athletes").Body.Close()

	body := readBody(t, get(t, ts, client, "/athletes"))
	for _, label := range []string{"Last name", "First name", "Date of birth", "Joined", "Current rank"} {
		if !strings.Contains(headerCell(t, body, label), label) {
			t.Errorf("no English <th> for %q", label)
		}
		if !strings.Contains(body, `data-label="`+label+`"`) {
			t.Errorf("no card label for %q — the two spellings have drifted", label)
		}
	}
}

// The switcher returns to the roster the trainer is actually looking at, which
// is the normalised query rather than whatever the URL happened to say.
func TestSwitcherReturnsToTheNormalisedRoster(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	mixedRoster(t, db)

	body := readBody(t, get(t, ts, client, "/athletes?sort=lastName&dir=desc&system=bjj-elderly"))
	form := langSwitchForm(t, body)
	if got := attrValue(t, form, "value"); got != "/athletes?sort=lastName&amp;dir=desc" {
		t.Errorf("return target = %q, want the roster without the unrepresented filter", got)
	}
}

// langSwitchForm returns the markup of the nav's language form.
func langSwitchForm(t *testing.T, body string) string {
	t.Helper()
	start := strings.Index(body, `<form class="lang-switch"`)
	if start < 0 {
		t.Fatal("no language switcher in the nav")
	}
	end := strings.Index(body[start:], "</form>")
	if end < 0 {
		t.Fatal("the language switcher is not closed")
	}
	return body[start : start+end]
}

// Every page renders in either language: the template sets are parsed per locale
// at boot, so a key used in a page nothing else exercises would otherwise only
// fail in the browser.
func TestEveryPageRendersInEnglish(t *testing.T) {
	ts, client, db := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()
	mixedRoster(t, db)
	switchTo(t, ts, client, "en", "/athletes").Body.Close()

	for _, path := range []string{"/", "/athletes", "/athletes/new", "/athletes/1", "/athletes/1/edit", "/account/password"} {
		resp := get(t, ts, client, path)
		body := readBody(t, resp)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
		if !strings.Contains(body, `<html lang="en">`) {
			t.Errorf("GET %s: want an English document", path)
		}
	}
}
