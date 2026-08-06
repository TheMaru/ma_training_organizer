package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// isoDate is the yyyy-mm-dd layout used for the optional DATE columns, matching
// what <input type="date"> submits.
const isoDate = "2006-01-02"

// ErrAthleteNotFound is returned when a lookup, update or delete targets an
// athlete that does not exist. Callers match it with errors.Is rather than
// sql.ErrNoRows so the store's row model stays an implementation detail.
var ErrAthleteNotFound = errors.New("store: athlete not found")

// Athlete is a person tracked on the shared roster (CONTEXT.md). The roster has
// no ownership — every trainer sees and edits every athlete. BirthDate and
// JoinedOn are ISO yyyy-mm-dd strings, empty when unset (stored as SQL NULL).
type Athlete struct {
	ID        int64
	FirstName string
	LastName  string
	BirthDate string
	JoinedOn  string
	Notes     string
}

// CreateAthlete inserts a new athlete and returns its id. Blank dates are stored
// as NULL rather than empty strings (portable SQL, ADR-0002).
func CreateAthlete(db *sql.DB, a Athlete) (int64, error) {
	return insertAthlete(db, a)
}

// CreateAthletes inserts every athlete in a single transaction: either all of
// them land or none do. The CSV import is all-or-nothing by spec, so a failure
// half-way through must not leave a partly filled roster behind.
func CreateAthletes(db *sql.DB, athletes []Athlete) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin athlete insert tx: %w", err)
	}
	defer tx.Rollback()

	for _, a := range athletes {
		if _, err := insertAthlete(tx, a); err != nil {
			// Name the athlete: in a bulk insert an unattributed failure says
			// nothing about which row of the source is at fault.
			return fmt.Errorf("%s %s: %w", a.FirstName, a.LastName, err)
		}
	}
	return tx.Commit()
}

// execer is the common subset of *sql.DB and *sql.Tx that inserting an athlete needs, so
// the single-row and the transactional bulk path share one INSERT.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

// insertAthlete writes one athlete row and returns its id.
func insertAthlete(ex execer, a Athlete) (int64, error) {
	res, err := ex.Exec(
		`INSERT INTO athletes (first_name, last_name, birth_date, joined_on, notes)
		 VALUES (?, ?, ?, ?, ?)`,
		a.FirstName, a.LastName, nullIfEmpty(a.BirthDate), nullIfEmpty(a.JoinedOn), a.Notes,
	)
	if err != nil {
		return 0, fmt.Errorf("insert athlete: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id for athlete: %w", err)
	}
	return id, nil
}

// AthleteByID looks up one athlete, returning ErrAthleteNotFound if none matches.
func AthleteByID(db *sql.DB, id int64) (Athlete, error) {
	a, err := scanAthlete(db.QueryRow(
		`SELECT id, first_name, last_name, birth_date, joined_on, notes
		 FROM athletes WHERE id = ?`, id,
	).Scan)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Athlete{}, ErrAthleteNotFound
	case err != nil:
		return Athlete{}, fmt.Errorf("scan athlete %d: %w", id, err)
	}
	return a, nil
}

// ListAthletes returns every athlete ordered by last name, without their derived
// rank. descending flips the direction; first name is the tie-breaker and flips
// with it, so a descending list is a true reverse of the ascending one. ORDER BY
// stays plain SQL (no COLLATE) to remain portable (ADR-0002). The roster view
// uses LoadRoster instead — this is the plain athlete listing (demo CLI, tests).
func ListAthletes(db *sql.DB, descending bool) ([]Athlete, error) {
	// dir is a controlled constant (never user input), so interpolating it is
	// injection-safe. Both keys use it so the whole order reverses together.
	dir := "ASC"
	if descending {
		dir = "DESC"
	}
	rows, err := db.Query(fmt.Sprintf(
		`SELECT id, first_name, last_name, birth_date, joined_on, notes
		 FROM athletes ORDER BY last_name %s, first_name %s`, dir, dir,
	))
	if err != nil {
		return nil, fmt.Errorf("list athletes: %w", err)
	}
	defer rows.Close()

	var athletes []Athlete
	for rows.Next() {
		a, err := scanAthlete(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan athlete row: %w", err)
		}
		athletes = append(athletes, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate athletes: %w", err)
	}
	return athletes, nil
}

// UpdateAthlete overwrites an athlete's fields by id, returning ErrAthleteNotFound
// when no athlete has that id.
func UpdateAthlete(db *sql.DB, a Athlete) error {
	res, err := db.Exec(
		`UPDATE athletes
		 SET first_name = ?, last_name = ?, birth_date = ?, joined_on = ?, notes = ?
		 WHERE id = ?`,
		a.FirstName, a.LastName, nullIfEmpty(a.BirthDate), nullIfEmpty(a.JoinedOn), a.Notes, a.ID,
	)
	if err != nil {
		return fmt.Errorf("update athlete %d: %w", a.ID, err)
	}
	return checkAffected(res, a.ID)
}

// DeleteAthlete hard-deletes an athlete; the foreign key's ON DELETE CASCADE
// removes their promotions (spec, ADR-0003). Returns ErrAthleteNotFound when no
// athlete has that id.
func DeleteAthlete(db *sql.DB, id int64) error {
	res, err := db.Exec(`DELETE FROM athletes WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete athlete %d: %w", id, err)
	}
	return checkAffected(res, id)
}

// checkAffected turns a zero-rows-affected result into ErrAthleteNotFound so
// updates and deletes against a stale id fail loudly instead of silently.
func checkAffected(res sql.Result, id int64) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for athlete %d: %w", id, err)
	}
	if n == 0 {
		return ErrAthleteNotFound
	}
	return nil
}

// scanAthlete maps one row into an Athlete via the given Scan func (works for
// both *sql.Row and *sql.Rows). The SQLite driver returns DATE columns as
// time.Time, so the optional dates are scanned as NullTime and rendered back to
// the yyyy-mm-dd strings the rest of the app works in (empty when NULL).
func scanAthlete(scan func(dest ...any) error) (Athlete, error) {
	var a Athlete
	var birth, joined sql.NullTime
	if err := scan(athleteDest(&a, &birth, &joined)...); err != nil {
		return Athlete{}, err
	}
	a.BirthDate = formatDate(birth)
	a.JoinedOn = formatDate(joined)
	return a, nil
}

// athleteDest lists the scan destinations for the athlete columns, in the order
// every athlete SELECT lists them. Shared with the roster's wider row so the
// column list and its nullable-date handling exist once.
func athleteDest(a *Athlete, birth, joined *sql.NullTime) []any {
	return []any{&a.ID, &a.FirstName, &a.LastName, birth, joined, &a.Notes}
}

// formatDate renders a nullable DATE as a yyyy-mm-dd string, empty when NULL.
func formatDate(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format(isoDate)
}

// nullIfEmpty returns nil for an empty string so it is bound as SQL NULL, and
// the string itself otherwise. Keeps optional DATE columns free of empty-string
// values that a stricter engine would reject (ADR-0002).
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
