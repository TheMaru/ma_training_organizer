package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"
)

// ErrTrainerNotFound is returned when a lookup or update targets a trainer that
// does not exist. Callers match it with errors.Is rather than sql.ErrNoRows so
// the store's row model stays an implementation detail.
var ErrTrainerNotFound = errors.New("store: trainer not found")

// ErrUsernameTaken is returned by CreateTrainer when the username is already in
// use, letting the CLI report a clean message instead of a raw constraint error.
var ErrUsernameTaken = errors.New("store: username already taken")

// Trainer is a login account. Trainers are the only accounts (CONTEXT.md); the
// PasswordHash is an opaque argon2id string owned by the auth package.
//
// Locale is the account's chosen UI language, empty while none was chosen. It is
// stored as the raw column value: which values are supported is the web layer's
// question (internal/i18n), not the store's.
//
// DeactivatedAt is when the account stopped being able to log in, zero while it
// still can. Every lookup selects it, so whoever holds a Trainer can answer
// "may this account log in?" without a second query — which is what the login
// path and the per-request check both need (ADR-0010).
type Trainer struct {
	ID            int64
	Username      string
	PasswordHash  string
	Locale        string
	DeactivatedAt time.Time
}

// Deactivated reports whether the account is refused at login. A Deactivated
// trainer is still a Trainer — the account is kept, only its access is gone
// (CONTEXT.md, ADR-0010).
func (t Trainer) Deactivated() bool {
	return !t.DeactivatedAt.IsZero()
}

// TrainerSummary is one trainer as the operator's listing sees them: who the
// account belongs to, and whether it may still log in. It is a type of its own
// rather than a Trainer so that the password hash is absent by construction —
// the listing is meant to be readable on a shared screen or pasted into a note,
// and a field that is not there cannot leak.
type TrainerSummary struct {
	ID            int64
	Username      string
	DeactivatedAt time.Time
}

// Deactivated mirrors Trainer.Deactivated for a listing entry.
func (s TrainerSummary) Deactivated() bool {
	return !s.DeactivatedAt.IsZero()
}

// ListTrainers returns every trainer, ordered by username so two runs of the
// operator's listing are comparable. It is the only way to see who has access:
// the app itself never shows one trainer to another (ADR-0010).
//
// LOWER() rather than the default collation, so "Zoe" does not sort ahead of
// "ada" — an order the operator would read as no order at all. Standard SQL, so
// the Postgres escape hatch stays open (ADR-0002); the username tie-breaker keeps
// two names differing only in case from swapping places between runs.
func ListTrainers(db *sql.DB) ([]TrainerSummary, error) {
	rows, err := db.Query(
		`SELECT id, username, deactivated_at FROM trainers ORDER BY LOWER(username), username`,
	)
	if err != nil {
		return nil, fmt.Errorf("list trainers: %w", err)
	}
	defer rows.Close()

	var trainers []TrainerSummary
	for rows.Next() {
		var s TrainerSummary
		var deactivated sql.NullTime
		if err := rows.Scan(&s.ID, &s.Username, &deactivated); err != nil {
			return nil, fmt.Errorf("scan trainer row: %w", err)
		}
		s.DeactivatedAt = deactivated.Time
		trainers = append(trainers, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trainers: %w", err)
	}
	return trainers, nil
}

// CreateTrainer inserts a new trainer with the given username and password hash,
// returning its id. A duplicate username yields ErrUsernameTaken.
func CreateTrainer(db *sql.DB, username, passwordHash string) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO trainers (username, password_hash) VALUES (?, ?)`,
		username, passwordHash,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrUsernameTaken
		}
		return 0, fmt.Errorf("insert trainer %q: %w", username, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id for trainer %q: %w", username, err)
	}
	return id, nil
}

// The functions below are near-copies within each kind — the lookups differ only
// in their WHERE clause, the updates only in the column they SET — and that
// stays. Deleting them moves their SQL into the handlers rather than
// concentrating it anywhere: the complexity moves, it does not reduce. Recorded
// here rather than as an ADR because it is reversible in an afternoon, and the
// reader who needs it has this file open already.
//
// What the updates do share is their tail, in updateOneTrainer: the rows-affected
// check that turns a statement matching no row into ErrTrainerNotFound. That is
// the part with a decision in it, and it is the same decision every time.

// TrainerByUsername looks up a trainer by username, returning ErrTrainerNotFound
// if none matches. Used by the login handler.
func TrainerByUsername(db *sql.DB, username string) (Trainer, error) {
	return scanTrainer(db.QueryRow(
		`SELECT id, username, password_hash, locale, deactivated_at FROM trainers WHERE username = ?`, username,
	))
}

// TrainerByID looks up a trainer by id, returning ErrTrainerNotFound if none
// matches. Used by the auth middleware to load the session's trainer.
func TrainerByID(db *sql.DB, id int64) (Trainer, error) {
	return scanTrainer(db.QueryRow(
		`SELECT id, username, password_hash, locale, deactivated_at FROM trainers WHERE id = ?`, id,
	))
}

// UpdateTrainerPassword replaces a trainer's password hash. It returns
// ErrTrainerNotFound when no trainer has the given id, so a self-service change
// or CLI reset against a stale id fails loudly.
func UpdateTrainerPassword(db *sql.DB, id int64, passwordHash string) error {
	return updateOneTrainer(db, "password update", id,
		`UPDATE trainers SET password_hash = ? WHERE id = ?`, passwordHash)
}

// UpdateTrainerLocale stores a trainer's chosen UI language. It returns
// ErrTrainerNotFound when no trainer has the given id, so a language switch
// against a stale session id fails loudly rather than silently doing nothing.
func UpdateTrainerLocale(db *sql.DB, id int64, locale string) error {
	return updateOneTrainer(db, "locale update", id,
		`UPDATE trainers SET locale = ? WHERE id = ?`, locale)
}

// DeactivateTrainer records that a trainer's account may no longer log in,
// returning ErrTrainerNotFound when no trainer has the given id.
//
// It keeps a date already recorded rather than moving it, so running it twice is
// neither an error nor a rewrite of history: the column answers "since when did
// this account lose access?", and the second run is not when that happened.
func DeactivateTrainer(db *sql.DB, id int64) error {
	return updateOneTrainer(db, "deactivation", id,
		`UPDATE trainers SET deactivated_at = COALESCE(deactivated_at, CURRENT_TIMESTAMP) WHERE id = ?`)
}

// ReactivateTrainer clears the deactivation date, letting the account log in
// again with the password it already had. It returns ErrTrainerNotFound when no
// trainer has the given id, and changes nothing else: restoring a password is a
// separate act with a separate command (ADR-0010).
func ReactivateTrainer(db *sql.DB, id int64) error {
	return updateOneTrainer(db, "reactivation", id,
		`UPDATE trainers SET deactivated_at = NULL WHERE id = ?`)
}

// CountActiveTrainers counts the trainers who can still log in. It is what the
// CLI's refusal to leave the club without one is made of, so it counts active
// accounts rather than rows: a club with a long line of departed trainers is one
// deactivation away from locking everybody out.
func CountActiveTrainers(db *sql.DB) (int, error) {
	var n int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM trainers WHERE deactivated_at IS NULL`,
	).Scan(&n); err != nil {
		return 0, fmt.Errorf("count active trainers: %w", err)
	}
	return n, nil
}

// updateOneTrainer runs an update whose WHERE clause is a trainer id, turning a
// statement that matched no row into ErrTrainerNotFound — so a write against an
// id that is gone fails loudly instead of silently doing nothing. act names the
// change for the error message; values are the bindings the SET clause needs, in
// order, and the id is bound last.
func updateOneTrainer(db *sql.DB, act string, id int64, query string, values ...any) error {
	res, err := db.Exec(query, append(values, id)...)
	if err != nil {
		return fmt.Errorf("%s for trainer %d: %w", act, id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected on %s for trainer %d: %w", act, id, err)
	}
	if n == 0 {
		return ErrTrainerNotFound
	}
	return nil
}

// scanTrainer maps a single-row query into a Trainer, translating the no-rows
// case into ErrTrainerNotFound. The SQLite driver hands a TIMESTAMP back as
// time.Time, so the nullable deactivation date is scanned as NullTime and left
// zero when the account is active.
func scanTrainer(row *sql.Row) (Trainer, error) {
	var tr Trainer
	var deactivated sql.NullTime
	err := row.Scan(&tr.ID, &tr.Username, &tr.PasswordHash, &tr.Locale, &deactivated)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Trainer{}, ErrTrainerNotFound
	case err != nil:
		return Trainer{}, fmt.Errorf("scan trainer: %w", err)
	}
	tr.DeactivatedAt = deactivated.Time
	return tr, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE-constraint failure.
// The driver-specific check lives here in the SQLite-aware store package; the
// portable-SQL rule (ADR-0002) governs queries, not driver error inspection.
func isUniqueViolation(err error) bool {
	var serr *sqlite3.Error
	return errors.As(err, &serr) && serr.Code() == sqlite3lib.SQLITE_CONSTRAINT_UNIQUE
}
