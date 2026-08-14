package main

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// trainerPassword is what every trainer these tests provision logs in with, so a
// login that fails did so for the reason under test.
const trainerPassword = "correct-horse"

// addTrainers provisions trainers who all log in with trainerPassword. Most tests
// here need at least two, because the last-active-trainer rule refuses an act on
// the only one left.
func addTrainers(t *testing.T, db *sql.DB, usernames ...string) {
	t.Helper()
	for _, u := range usernames {
		if err := createTrainer(db, u, trainerPassword); err != nil {
			t.Fatalf("createTrainer %q: %v", u, err)
		}
	}
}

// atLoginPage asserts a request was not served but sent to the login page, the
// answer requireAuth gives a session it will not honour.
func atLoginPage(t *testing.T, who string, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Errorf("%s: GET / status = %d, want %d", who, resp.StatusCode, http.StatusSeeOther)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("%s: Location = %q, want %q", who, loc, "/login")
	}
}

// servedOK is the mirror of atLoginPage: the request was served, so the session
// it carried is still honoured.
func servedOK(t *testing.T, who string, resp *http.Response) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("%s: GET / status = %d, want %d", who, resp.StatusCode, http.StatusOK)
	}
}

func home(t *testing.T, ts *httptest.Server, client *http.Client) *http.Response {
	t.Helper()
	resp, err := client.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	return resp
}

// The offboarding act's whole point: the credentials that worked yesterday stop
// working, and the account stays (ADR-0010).
func TestDeactivateTrainerRefusesTheNextLogin(t *testing.T) {
	db, ts := startApp(t)
	addTrainers(t, db, "ada", "grace")

	if err := deactivateTrainer(db, "ada"); err != nil {
		t.Fatalf("deactivateTrainer: %v", err)
	}

	if got := loginStatus(t, ts, "ada", trainerPassword); got != http.StatusUnauthorized {
		t.Errorf("deactivated trainer's login = %d, want %d", got, http.StatusUnauthorized)
	}
	if got := loginStatus(t, ts, "grace", trainerPassword); got != http.StatusSeeOther {
		t.Errorf("colleague's login = %d, want %d", got, http.StatusSeeOther)
	}
	if _, err := store.TrainerByUsername(db, "ada"); err != nil {
		t.Errorf("account gone after deactivation: %v", err)
	}
}

// Refusing the next login is not enough on its own: the departed trainer is
// already signed in somewhere, and that device would otherwise keep the access
// the operator just took away.
func TestDeactivateTrainerEndsTheirSessionsOnly(t *testing.T) {
	db, ts := startApp(t)
	addTrainers(t, db, "ada", "grace")
	phone := signIn(t, ts, "ada", trainerPassword)
	laptop := signIn(t, ts, "ada", trainerPassword)
	colleague := signIn(t, ts, "grace", trainerPassword)
	if got := storedSessions(t, db); got != 3 {
		t.Fatalf("stored sessions after three logins = %d, want 3", got)
	}

	if err := deactivateTrainer(db, "ada"); err != nil {
		t.Fatalf("deactivateTrainer: %v", err)
	}

	// Asserted before any request is made, so it is revocation being measured and
	// not requireAuth: the middleware refuses a deactivated trainer too, and would
	// send both devices to the login page even if nothing had been revoked.
	if got := storedSessions(t, db); got != 1 {
		t.Errorf("stored sessions after deactivation = %d, want 1 (the colleague's)", got)
	}

	atLoginPage(t, "phone", home(t, ts, phone))
	atLoginPage(t, "laptop", home(t, ts, laptop))
	servedOK(t, "colleague", home(t, ts, colleague))
}

// storedSessions counts every session the app holds. Counting rows is the only
// way to see a revocation as such: the trainer id lives inside each session's
// encoded values (see web.RevokeSessions), so there is nothing to count by
// trainer, and a request would answer a different question than "was this
// session revoked?".
func storedSessions(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatalf("count sessions: %v", err)
	}
	return n
}

// A return costs nothing: the same password works again, so nobody has to
// coordinate a handover for a routine homecoming (ADR-0010).
func TestReactivateTrainerRestoresTheOldPassword(t *testing.T) {
	db, ts := startApp(t)
	addTrainers(t, db, "ada", "grace")
	before, err := store.TrainerByUsername(db, "ada")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}

	if err := deactivateTrainer(db, "ada"); err != nil {
		t.Fatalf("deactivateTrainer: %v", err)
	}
	if err := reactivateTrainer(db, "ada"); err != nil {
		t.Fatalf("reactivateTrainer: %v", err)
	}

	if got := loginStatus(t, ts, "ada", trainerPassword); got != http.StatusSeeOther {
		t.Errorf("login after reactivation = %d, want %d", got, http.StatusSeeOther)
	}
	after, err := store.TrainerByUsername(db, "ada")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	// Reactivation is a pure state flip. Restoring a password is a separate act
	// with a separate command, so the hash cannot have moved.
	if after.PasswordHash != before.PasswordHash {
		t.Error("password hash changed across deactivation and reactivation")
	}
	if after.Deactivated() {
		t.Errorf("still deactivated at %v after reactivation", after.DeactivatedAt)
	}
}

// Reactivation touches one account. A colleague signed in throughout both acts
// keeps the session they had, so a homecoming never signs the club out.
func TestReactivateTrainerLeavesOtherSessionsAlone(t *testing.T) {
	db, ts := startApp(t)
	addTrainers(t, db, "ada", "grace")
	colleague := signIn(t, ts, "grace", trainerPassword)

	if err := deactivateTrainer(db, "ada"); err != nil {
		t.Fatalf("deactivateTrainer: %v", err)
	}
	if err := reactivateTrainer(db, "ada"); err != nil {
		t.Fatalf("reactivateTrainer: %v", err)
	}

	servedOK(t, "colleague", home(t, ts, colleague))
}

// Deactivating twice is the operator running a command they are not sure they ran.
// It must not be an error, and it must not move the date either: the column
// answers "since when?", which the second run is not the answer to.
func TestDeactivateTrainerTwiceKeepsTheFirstDate(t *testing.T) {
	db, _ := startApp(t)
	addTrainers(t, db, "ada", "grace")

	if err := deactivateTrainer(db, "ada"); err != nil {
		t.Fatalf("first deactivateTrainer: %v", err)
	}
	// Backdated by hand because CURRENT_TIMESTAMP has second resolution: two calls
	// inside one test would otherwise record the same instant, and a rewrite would
	// be invisible.
	const longAgo = "2020-01-02 03:04:05"
	if _, err := db.Exec(`UPDATE trainers SET deactivated_at = ? WHERE username = ?`, longAgo, "ada"); err != nil {
		t.Fatalf("backdate deactivation: %v", err)
	}

	if err := deactivateTrainer(db, "ada"); err != nil {
		t.Errorf("second deactivateTrainer: %v", err)
	}

	tr, err := store.TrainerByUsername(db, "ada")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	if got := tr.DeactivatedAt.Format("2006-01-02 15:04:05"); got != longAgo {
		t.Errorf("deactivated_at = %q, want the first date %q", got, longAgo)
	}
}

// The club may not be left with nobody who can log in. Three trainers here, two
// of them long gone: a rule counting rows would see three and allow the act.
func TestDeactivateRefusesTheOnlyActiveTrainer(t *testing.T) {
	db, ts := startApp(t)
	addTrainers(t, db, "ada", "departed-one", "departed-two")
	for _, u := range []string{"departed-one", "departed-two"} {
		if err := deactivateTrainer(db, u); err != nil {
			t.Fatalf("deactivateTrainer %q: %v", u, err)
		}
	}
	laptop := signIn(t, ts, "ada", trainerPassword)

	err := deactivateTrainer(db, "ada")
	if !errors.Is(err, errLastActiveTrainer) {
		t.Fatalf("error = %v, want errLastActiveTrainer", err)
	}
	// An operator who cannot do the thing needs to be told what to do instead.
	if !strings.Contains(err.Error(), "create-trainer") {
		t.Errorf("refusal %q does not name the way through", err)
	}

	// Refused, not half-applied: the account still logs in and the session that
	// was live stays live.
	tr, err := store.TrainerByUsername(db, "ada")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	if tr.Deactivated() {
		t.Errorf("account deactivated at %v despite the refusal", tr.DeactivatedAt)
	}
	servedOK(t, "the live session", home(t, ts, laptop))
}

// The rule guards the count, not the command: re-running the act on somebody who
// already cannot log in takes nothing away, so there is nothing to refuse — even
// when exactly one active trainer is left.
func TestDeactivateAnAlreadyDeactivatedTrainerIsAllowed(t *testing.T) {
	db, _ := startApp(t)
	addTrainers(t, db, "ada", "departed")
	if err := deactivateTrainer(db, "departed"); err != nil {
		t.Fatalf("first deactivateTrainer: %v", err)
	}

	if err := deactivateTrainer(db, "departed"); err != nil {
		t.Errorf("deactivateTrainer on an already-deactivated trainer: %v", err)
	}
}

// An operator resetting a deactivated trainer's password is working on an account
// that cannot log in either way. Refused, and told the verb for what they may
// actually have meant.
func TestResetPasswordRefusesADeactivatedTrainer(t *testing.T) {
	db, ts := startApp(t)
	addTrainers(t, db, "ada", "grace")
	if err := deactivateTrainer(db, "grace"); err != nil {
		t.Fatalf("deactivateTrainer: %v", err)
	}
	before, err := store.TrainerByUsername(db, "grace")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}

	err = resetPassword(db, "grace", "brand-new-secret")
	if !errors.Is(err, errTrainerDeactivated) {
		t.Fatalf("error = %v, want errTrainerDeactivated", err)
	}
	if !strings.Contains(err.Error(), "reactivate-trainer") {
		t.Errorf("refusal %q does not name reactivation", err)
	}

	// Refused, not half-applied: the old hash stands, and the new password is not
	// a way in either.
	after, err := store.TrainerByUsername(db, "grace")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	if after.PasswordHash != before.PasswordHash {
		t.Error("password hash changed despite the refusal")
	}
	if got := loginStatus(t, ts, "grace", "brand-new-secret"); got != http.StatusUnauthorized {
		t.Errorf("login with the refused password = %d, want %d", got, http.StatusUnauthorized)
	}
}

// Revocation stays permitted on a deactivated trainer, and stays revocation: an
// operator working an incident should not have to reason about command order, and
// a subcommand that quietly did nothing would be worse than a refusal.
//
// The state is set through the store rather than through deactivateTrainer, which
// revokes as it goes — that would leave nothing for the act under test to end.
func TestRevokeSessionsWorksOnADeactivatedTrainer(t *testing.T) {
	db, ts := startApp(t)
	addTrainers(t, db, "ada", "grace")
	signIn(t, ts, "grace", trainerPassword)
	signIn(t, ts, "ada", trainerPassword)
	tr, err := store.TrainerByUsername(db, "grace")
	if err != nil {
		t.Fatalf("TrainerByUsername: %v", err)
	}
	if err := store.DeactivateTrainer(db, tr.ID); err != nil {
		t.Fatalf("DeactivateTrainer: %v", err)
	}

	if err := revokeSessions(db, "grace"); err != nil {
		t.Fatalf("revokeSessions on a deactivated trainer: %v", err)
	}

	if got := storedSessions(t, db); got != 1 {
		t.Errorf("stored sessions after revocation = %d, want 1 (ada's)", got)
	}
}

// create-trainer is where an operator meets a name they cannot see anywhere else:
// the account exists, deactivated, and "already taken" alone would send them
// hunting for it (ADR-0010).
func TestCreateTrainerReportsADeactivatedAccount(t *testing.T) {
	db, _ := startApp(t)
	addTrainers(t, db, "ada", "grace")
	if err := deactivateTrainer(db, "grace"); err != nil {
		t.Fatalf("deactivateTrainer: %v", err)
	}

	err := createTrainer(db, "grace", "another-horse")
	// Still the taken-username case, so a caller matching on it keeps working.
	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Fatalf("error = %v, want ErrUsernameTaken", err)
	}
	if !strings.Contains(err.Error(), "deactivated") {
		t.Errorf("message %q does not say the account is deactivated", err)
	}
	if !strings.Contains(err.Error(), "reactivate-trainer") {
		t.Errorf("message %q does not name the way to that account", err)
	}
}

// The taken-username message only names deactivation when that is true, or the
// operator is sent to reactivate an account that is already active.
func TestCreateTrainerAgainstAnActiveNameStaysPlain(t *testing.T) {
	db, _ := startApp(t)
	addTrainers(t, db, "ada")

	err := createTrainer(db, "ada", "another-horse")
	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Fatalf("error = %v, want ErrUsernameTaken", err)
	}
	if strings.Contains(err.Error(), "deactivated") {
		t.Errorf("message %q calls an active account deactivated", err)
	}
}

func TestDeactivateUnknownTrainer(t *testing.T) {
	db, _ := startApp(t)
	addTrainers(t, db, "ada")

	if err := deactivateTrainer(db, "ghost"); !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

func TestReactivateUnknownTrainer(t *testing.T) {
	db, _ := startApp(t)
	addTrainers(t, db, "ada")

	if err := reactivateTrainer(db, "ghost"); !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}
