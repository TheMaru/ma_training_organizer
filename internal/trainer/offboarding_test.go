package trainer_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/TheMaru/ma_training_organizer/internal/session/sessiontest"
	"github.com/TheMaru/ma_training_organizer/internal/store"
	"github.com/TheMaru/ma_training_organizer/internal/store/storetest"
	"github.com/TheMaru/ma_training_organizer/internal/trainer"
)

// Refusing the next login is not enough on its own: the departed trainer is
// already signed in somewhere, and that device would otherwise keep the access the
// operator just took away. That the next login is refused is web's to assert; what
// the act leaves behind is this.
func TestDeactivateEndsTheirSessionsOnly(t *testing.T) {
	db := club(t, "ada", "grace")
	sessions := sessiontest.NewManager(t, db)
	ada, grace := trainerID(t, db, "ada"), trainerID(t, db, "grace")
	sessiontest.SignIn(t, sessions, ada)
	sessiontest.SignIn(t, sessions, ada)
	sessiontest.SignIn(t, sessions, grace)

	if err := trainer.Deactivate(db, "ada"); err != nil {
		t.Fatalf("trainer.Deactivate: %v", err)
	}

	if got := sessiontest.Count(t, sessions, ada); got != 0 {
		t.Errorf("the deactivated trainer still holds %d sessions", got)
	}
	if got := sessiontest.Count(t, sessions, grace); got != 1 {
		t.Errorf("the colleague holds %d sessions, want 1", got)
	}
	if !account(t, db, "ada").Deactivated() {
		t.Error("the account is not deactivated")
	}
}

// Deactivating twice is the operator running a command they are not sure they ran.
// It must not be an error, and it must not move the date either: the column
// answers "since when?", which the second run is not the answer to.
func TestDeactivateTwiceKeepsTheFirstDate(t *testing.T) {
	db := club(t, "ada", "grace")
	if err := trainer.Deactivate(db, "ada"); err != nil {
		t.Fatalf("first trainer.Deactivate: %v", err)
	}
	// Backdated by hand because CURRENT_TIMESTAMP has second resolution: two calls
	// inside one test would otherwise record the same instant, and a rewrite would
	// be invisible.
	const longAgo = "2020-01-02 03:04:05"
	if _, err := db.Exec(`UPDATE trainers SET deactivated_at = ? WHERE username = ?`, longAgo, "ada"); err != nil {
		t.Fatalf("backdate deactivation: %v", err)
	}

	if err := trainer.Deactivate(db, "ada"); err != nil {
		t.Errorf("second trainer.Deactivate: %v", err)
	}

	if got := account(t, db, "ada").DeactivatedAt.Format("2006-01-02 15:04:05"); got != longAgo {
		t.Errorf("deactivated_at = %q, want the first date %q", got, longAgo)
	}
}

// The club may not be left with nobody who can log in. Three trainers here, two of
// them long gone: a rule counting rows would see three and allow the act.
func TestDeactivateRefusesTheOnlyActiveTrainer(t *testing.T) {
	db := club(t, "ada", "departed-one", "departed-two")
	for _, u := range []string{"departed-one", "departed-two"} {
		if err := trainer.Deactivate(db, u); err != nil {
			t.Fatalf("trainer.Deactivate %q: %v", u, err)
		}
	}
	sessions := sessiontest.NewManager(t, db)
	ada := trainerID(t, db, "ada")
	sessiontest.SignIn(t, sessions, ada)

	err := trainer.Deactivate(db, "ada")

	if !errors.Is(err, trainer.ErrLastActiveTrainer) {
		t.Fatalf("error = %v, want trainer.ErrLastActiveTrainer", err)
	}
	// An operator who cannot do the thing needs to be told what to do instead.
	if !strings.Contains(err.Error(), "create-trainer") {
		t.Errorf("refusal %q does not name the way through", err)
	}
	// Refused, not half-applied: the account still may log in and the session that
	// was live is still live.
	if account(t, db, "ada").Deactivated() {
		t.Error("account deactivated despite the refusal")
	}
	if got := sessiontest.Count(t, sessions, ada); got != 1 {
		t.Errorf("the live session was ended despite the refusal: %d left, want 1", got)
	}
}

// The rule guards the count, not the command: re-running the act on somebody who
// already cannot log in takes nothing away, so there is nothing to refuse — even
// when exactly one active trainer is left.
func TestDeactivateAnAlreadyDeactivatedTrainerIsAllowed(t *testing.T) {
	db := club(t, "ada", "departed")
	if err := trainer.Deactivate(db, "departed"); err != nil {
		t.Fatalf("first trainer.Deactivate: %v", err)
	}

	if err := trainer.Deactivate(db, "departed"); err != nil {
		t.Errorf("trainer.Deactivate on an already-deactivated trainer: %v", err)
	}
}

// A return costs nothing and changes nothing else: the password they had is
// untouched, so nobody has to coordinate a handover for a routine homecoming
// (ADR-0010). That the account logs in again is web's to assert — the homecoming
// shows itself only there.
func TestReactivateGivesTheAccountBackUnchanged(t *testing.T) {
	db := club(t, "ada", "grace")
	before := account(t, db, "ada")
	if err := trainer.Deactivate(db, "ada"); err != nil {
		t.Fatalf("trainer.Deactivate: %v", err)
	}

	if err := trainer.Reactivate(db, "ada"); err != nil {
		t.Fatalf("trainer.Reactivate: %v", err)
	}

	after := account(t, db, "ada")
	if after.Deactivated() {
		t.Errorf("still deactivated at %v after reactivation", after.DeactivatedAt)
	}
	// Restoring a password is a separate act with a separate command, so the hash
	// cannot have moved.
	if after.PasswordHash != before.PasswordHash {
		t.Error("password hash changed across deactivation and reactivation")
	}
}

// Both acts touch one account. A colleague signed in throughout keeps their
// sessions, so a departure and a homecoming never sign the club out.
func TestReactivateLeavesOtherSessionsAlone(t *testing.T) {
	db := club(t, "ada", "grace")
	sessions := sessiontest.NewManager(t, db)
	grace := trainerID(t, db, "grace")
	sessiontest.SignIn(t, sessions, grace)

	if err := trainer.Deactivate(db, "ada"); err != nil {
		t.Fatalf("trainer.Deactivate: %v", err)
	}
	if err := trainer.Reactivate(db, "ada"); err != nil {
		t.Fatalf("trainer.Reactivate: %v", err)
	}

	if got := sessiontest.Count(t, sessions, grace); got != 1 {
		t.Errorf("the colleague holds %d sessions, want 1", got)
	}
}

// Deletion is the exception rather than the ordinary offboarding act (ADR-0010):
// it exists for an erasure request, and it gives the username back.
func TestDeleteRemovesTheAccountAndFreesTheUsername(t *testing.T) {
	db := club(t, "ada", "grace")

	if err := trainer.Delete(db, "grace"); err != nil {
		t.Fatalf("trainer.Delete: %v", err)
	}

	if _, err := store.TrainerByUsername(db, "grace"); !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("lookup after the deletion = %v, want ErrTrainerNotFound", err)
	}
	// Freeing the name for reuse is half of why the act exists, so it is asserted
	// rather than inferred from the row being gone.
	if err := trainer.Provision(db, "grace", trainerPassword); err != nil {
		t.Errorf("trainer.Provision on the freed username: %v", err)
	}
}

// The sessions outlive the row they belonged to, so ending them is a real act here
// and not a formality — even though the account being gone is enough for the app
// to turn the device away (see web.Server.requireAuth).
func TestDeleteEndsTheirSessionsOnly(t *testing.T) {
	db := club(t, "ada", "grace")
	sessions := sessiontest.NewManager(t, db)
	// Read while the account is still there: the id is what the question is asked
	// with afterwards.
	ada, grace := trainerID(t, db, "ada"), trainerID(t, db, "grace")
	sessiontest.SignIn(t, sessions, grace)
	sessiontest.SignIn(t, sessions, ada)

	if err := trainer.Delete(db, "grace"); err != nil {
		t.Fatalf("trainer.Delete: %v", err)
	}

	if got := sessiontest.Count(t, sessions, grace); got != 0 {
		t.Errorf("the deleted trainer still holds %d sessions", got)
	}
	if got := sessiontest.Count(t, sessions, ada); got != 1 {
		t.Errorf("the colleague holds %d sessions, want 1", got)
	}
}

// The same rule as for deactivation, and deliberately the same predicate: an
// erasure request does not get to lock the club out either. Three trainers here,
// two of them long gone, so a rule counting rows would allow the act.
func TestDeleteRefusesTheOnlyActiveTrainer(t *testing.T) {
	db := club(t, "ada", "departed-one", "departed-two")
	for _, u := range []string{"departed-one", "departed-two"} {
		if err := trainer.Deactivate(db, u); err != nil {
			t.Fatalf("trainer.Deactivate %q: %v", u, err)
		}
	}
	sessions := sessiontest.NewManager(t, db)
	ada := trainerID(t, db, "ada")
	sessiontest.SignIn(t, sessions, ada)

	err := trainer.Delete(db, "ada")

	if !errors.Is(err, trainer.ErrLastActiveTrainer) {
		t.Fatalf("error = %v, want trainer.ErrLastActiveTrainer", err)
	}
	if !strings.Contains(err.Error(), "create-trainer") {
		t.Errorf("refusal %q does not name the way through", err)
	}
	// Refused, not half-applied: the account is still there and the session that was
	// live is still live.
	if _, err := store.TrainerByUsername(db, "ada"); err != nil {
		t.Errorf("account gone despite the refusal: %v", err)
	}
	if got := sessiontest.Count(t, sessions, ada); got != 1 {
		t.Errorf("the live session was ended despite the refusal: %d left, want 1", got)
	}
}

// A trainer who already cannot log in is not the last active one, whatever the
// count — so an erasure request for somebody long departed is never refused.
func TestDeleteADeactivatedTrainerIsAllowed(t *testing.T) {
	db := club(t, "ada", "departed")
	if err := trainer.Deactivate(db, "departed"); err != nil {
		t.Fatalf("trainer.Deactivate: %v", err)
	}

	if err := trainer.Delete(db, "departed"); err != nil {
		t.Errorf("trainer.Delete on a deactivated trainer: %v", err)
	}
}

// Offboarding a person never costs the club its data. No athlete belongs to a
// trainer (CONTEXT.md), so there is nothing to reassign and nothing to cascade.
func TestDeleteLeavesTheRosterAlone(t *testing.T) {
	db := club(t, "ada", "grace")
	athleteID, err := store.CreateAthlete(db, store.Athlete{FirstName: "Kenji", LastName: "Tanaka"})
	if err != nil {
		t.Fatalf("CreateAthlete: %v", err)
	}
	if _, err := store.CreatePromotion(db, store.Promotion{
		AthleteID:  athleteID,
		RankID:     storetest.RankID(t, db, "BJJ Adult", "White"),
		PromotedOn: "2025-03-01",
	}); err != nil {
		t.Fatalf("CreatePromotion: %v", err)
	}

	if err := trainer.Delete(db, "grace"); err != nil {
		t.Fatalf("trainer.Delete: %v", err)
	}

	athletes, err := store.ListAthletes(db, false)
	if err != nil {
		t.Fatalf("ListAthletes: %v", err)
	}
	if len(athletes) != 1 {
		t.Errorf("athletes after the deletion = %d, want 1", len(athletes))
	}
	promotions, err := store.ListPromotions(db, athleteID)
	if err != nil {
		t.Fatalf("ListPromotions: %v", err)
	}
	if len(promotions) != 1 {
		t.Errorf("promotions after the deletion = %d, want 1", len(promotions))
	}
}

func TestDeactivateUnknownTrainer(t *testing.T) {
	db := club(t, "ada")

	if err := trainer.Deactivate(db, "ghost"); !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

func TestReactivateUnknownTrainer(t *testing.T) {
	db := club(t, "ada")

	if err := trainer.Reactivate(db, "ghost"); !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}

func TestDeleteUnknownTrainer(t *testing.T) {
	db := club(t, "ada")

	if err := trainer.Delete(db, "ghost"); !errors.Is(err, store.ErrTrainerNotFound) {
		t.Errorf("error = %v, want ErrTrainerNotFound", err)
	}
}
