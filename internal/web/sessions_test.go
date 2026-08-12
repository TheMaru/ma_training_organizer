package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const passwordPagePath = "/account/password"

// revoke posts the account area's "sign out other devices" control.
func revoke(t *testing.T, ts *httptest.Server, client *http.Client) *http.Response {
	t.Helper()
	return post(t, ts, client, "/account/sessions/revoke", url.Values{})
}

func TestRevokeEndsTheOtherDeviceButNotThisOne(t *testing.T) {
	ts, phone, _ := newAuthTestServer(t)
	login(t, ts, phone, testUsername, testPassword).Body.Close()

	laptop := newClient(t)
	login(t, ts, laptop, testUsername, testPassword).Body.Close()

	revoke(t, ts, laptop).Body.Close()

	// The phone's cookie still names a session, but its row is gone, so the next
	// request arrives unauthenticated.
	fromPhone := get(t, ts, phone, "/")
	fromPhone.Body.Close()
	if fromPhone.StatusCode != http.StatusSeeOther {
		t.Errorf("revoked device: GET / status = %d, want %d", fromPhone.StatusCode, http.StatusSeeOther)
	}
	if loc := fromPhone.Header.Get("Location"); loc != "/login" {
		t.Errorf("revoked device: Location = %q, want %q", loc, "/login")
	}

	fromLaptop := get(t, ts, laptop, "/")
	fromLaptop.Body.Close()
	if fromLaptop.StatusCode != http.StatusOK {
		t.Errorf("revoking device: GET / status = %d, want %d", fromLaptop.StatusCode, http.StatusOK)
	}
}

// Revocation is per trainer, so a colleague logged in on the same club's
// installation must not be swept up by it.
func TestRevokeSparesTheOtherTrainersSessions(t *testing.T) {
	ts, mine, db := newAuthTestServer(t)
	login(t, ts, mine, testUsername, testPassword).Body.Close()

	const colleague = "grace"
	addTrainer(t, db, colleague)
	theirs := newClient(t)
	login(t, ts, theirs, colleague, testPassword).Body.Close()

	revoke(t, ts, mine).Body.Close()

	resp := get(t, ts, theirs, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("colleague's GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// The counterpart to the two tests above, and the reason they exist: changing the
// password deliberately leaves the other devices signed in. Revocation is the
// separate action, not a side effect — pinned here so flipping it needs a
// decision rather than a patch.
func TestChangingThePasswordLeavesTheOtherDeviceSignedIn(t *testing.T) {
	ts, phone, _ := newAuthTestServer(t)
	login(t, ts, phone, testUsername, testPassword).Body.Close()

	laptop := newClient(t)
	login(t, ts, laptop, testUsername, testPassword).Body.Close()

	const newPassword = "brand-new-secret"
	post(t, ts, laptop, passwordPagePath, url.Values{
		"current": {testPassword},
		"new":     {newPassword},
		"confirm": {newPassword},
	}).Body.Close()

	resp := get(t, ts, phone, "/")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("other device after a password change: GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

// The control carries a confirmation, because a stray tap would sign the trainer
// out of the phone they are holding at the mat side.
func TestTheAccountAreaOffersTheControlBehindAConfirmation(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	body := readBody(t, get(t, ts, client, passwordPagePath))
	for _, want := range []string{
		`hx-post="/account/sessions/revoke"`,
		`hx-confirm="Alle anderen Geräte abmelden?`,
		"Andere Geräte abmelden",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the account page does not contain %q", want)
		}
	}
}

// Revocation is silent by nature — nothing on this device changes — so the page
// it returns to has to say that it happened.
func TestRevokeReportsBackOnTheAccountPage(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	resp := revoke(t, ts, client)
	resp.Body.Close()
	if got := resp.Header.Get("Location"); got != passwordPagePath {
		t.Errorf("Location = %q, want %q", got, passwordPagePath)
	}

	const notice = "Alle anderen Geräte wurden abgemeldet."
	body := readBody(t, get(t, ts, client, passwordPagePath))
	if !strings.Contains(body, notice) {
		t.Errorf("the account page does not report the revocation")
	}
	// The notice is a one-off, not a banner that outlives the action it reports.
	if again := readBody(t, get(t, ts, client, passwordPagePath)); strings.Contains(again, notice) {
		t.Error("the notice survived into a second page load")
	}
}

// The notice waits as a catalog key, so a trainer who switches language before
// reading it gets it in the language they are now reading (ADR-0008).
func TestThePendingNoticeFollowsALanguageSwitch(t *testing.T) {
	ts, client, _ := newAuthTestServer(t)
	login(t, ts, client, testUsername, testPassword).Body.Close()

	revoke(t, ts, client).Body.Close()
	switchTo(t, ts, client, "en", "/athletes").Body.Close()

	body := readBody(t, get(t, ts, client, passwordPagePath))
	if !strings.Contains(body, "All other devices have been signed out.") {
		t.Error("the pending notice stayed German after the switch to English")
	}
}
