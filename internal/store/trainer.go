package store

import (
	"database/sql"
	"errors"
	"fmt"

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
type Trainer struct {
	ID           int64
	Username     string
	PasswordHash string
	Locale       string
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

// TrainerByUsername looks up a trainer by username, returning ErrTrainerNotFound
// if none matches. Used by the login handler.
func TrainerByUsername(db *sql.DB, username string) (Trainer, error) {
	return scanTrainer(db.QueryRow(
		`SELECT id, username, password_hash, locale FROM trainers WHERE username = ?`, username,
	))
}

// TrainerByID looks up a trainer by id, returning ErrTrainerNotFound if none
// matches. Used by the auth middleware to load the session's trainer.
func TrainerByID(db *sql.DB, id int64) (Trainer, error) {
	return scanTrainer(db.QueryRow(
		`SELECT id, username, password_hash, locale FROM trainers WHERE id = ?`, id,
	))
}

// UpdateTrainerPassword replaces a trainer's password hash. It returns
// ErrTrainerNotFound when no trainer has the given id, so a self-service change
// or CLI reset against a stale id fails loudly.
func UpdateTrainerPassword(db *sql.DB, id int64, passwordHash string) error {
	res, err := db.Exec(
		`UPDATE trainers SET password_hash = ? WHERE id = ?`, passwordHash, id,
	)
	if err != nil {
		return fmt.Errorf("update trainer %d password: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected updating trainer %d: %w", id, err)
	}
	if n == 0 {
		return ErrTrainerNotFound
	}
	return nil
}

// UpdateTrainerLocale stores a trainer's chosen UI language. It returns
// ErrTrainerNotFound when no trainer has the given id, so a language switch
// against a stale session id fails loudly rather than silently doing nothing.
func UpdateTrainerLocale(db *sql.DB, id int64, locale string) error {
	res, err := db.Exec(
		`UPDATE trainers SET locale = ? WHERE id = ?`, locale, id,
	)
	if err != nil {
		return fmt.Errorf("update trainer %d locale: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected updating trainer %d: %w", id, err)
	}
	if n == 0 {
		return ErrTrainerNotFound
	}
	return nil
}

// scanTrainer maps a single-row query into a Trainer, translating the no-rows
// case into ErrTrainerNotFound.
func scanTrainer(row *sql.Row) (Trainer, error) {
	var tr Trainer
	err := row.Scan(&tr.ID, &tr.Username, &tr.PasswordHash, &tr.Locale)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Trainer{}, ErrTrainerNotFound
	case err != nil:
		return Trainer{}, fmt.Errorf("scan trainer: %w", err)
	}
	return tr, nil
}

// isUniqueViolation reports whether err is a SQLite UNIQUE-constraint failure.
// The driver-specific check lives here in the SQLite-aware store package; the
// portable-SQL rule (ADR-0002) governs queries, not driver error inspection.
func isUniqueViolation(err error) bool {
	var serr *sqlite3.Error
	return errors.As(err, &serr) && serr.Code() == sqlite3lib.SQLITE_CONSTRAINT_UNIQUE
}
