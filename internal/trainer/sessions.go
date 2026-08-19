package trainer

import (
	"context"
	"database/sql"

	"github.com/TheMaru/ma_training_organizer/internal/session"
	"github.com/TheMaru/ma_training_organizer/internal/store"
)

// RevokeSessions ends every Session a named trainer holds, the Operator's path
// for a lost phone or a suspected takeover, and reports how many it ended.
//
// It stays permitted on a deactivated trainer: an operator working an incident
// should not have to reason about command order.
func RevokeSessions(db *sql.DB, username string) (int, error) {
	tr, err := store.TrainerByUsername(db, username)
	if err != nil {
		return 0, err
	}
	return revokeAll(db, tr.ID)
}

// revokeAll ends every Session a trainer holds, sparing none. Offboarding is a
// caller of session.Manager.RevokeAll, not a second copy of it — and it builds
// the manager here rather than taking one, so no caller has to obtain a session
// manager in order to sign a Trainer out.
func revokeAll(db *sql.DB, trainerID int64) (int, error) {
	return session.ForCommand(db).RevokeAll(context.Background(), trainerID)
}
