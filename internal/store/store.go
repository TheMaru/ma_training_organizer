// Package store owns the SQLite database: opening it with the pragmas the app
// relies on and applying schema migrations. Schema SQL is kept portable
// (ADR-0002); the SQLite-only concerns here — foreign-key enforcement, WAL,
// busy timeout — are connection pragmas rather than schema, so the Postgres
// escape hatch stays open.
package store

import (
	"database/sql"
	"embed"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open returns the SQLite database at path ready to use: connected with the
// pragmas the app depends on, migrated to the latest schema, and seeded with the
// built-in reference data. Migrating and seeding are steps of opening rather than
// calls of their own, so there is no order for a caller to get right — the one
// that existed was written down nowhere and was got wrong three times, each
// leaving a database with no GradingSystem for a Trainer to promote into.
//
// Both steps are idempotent, so this is what every boot calls. The consequence,
// accepted: no caller can observe a migrated-but-unseeded database. Nothing
// needs one; a read-only tool later would reopen that.
//
// The pragmas:
//
//   - foreign_keys(1): SQLite leaves FK enforcement off by default, so without
//     this ON DELETE CASCADE would silently not fire.
//   - journal_mode(WAL): required by Litestream backups (issue 07) and allows
//     readers to proceed alongside the single writer.
//   - busy_timeout(5000): briefly wait for the writer instead of erroring.
//
// The pool is capped at a single connection: SQLite permits only one writer,
// and one connection sidesteps "database is locked" entirely at this app's
// tiny, trainer-only scale.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite %q: %w", path, err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := seed(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// dsn renders the driver connection string for a database file. The pragmas ride
// in a query string, so path goes over as an escaped file: URI — both the driver
// and SQLite cut the string at its first "?", and a path holding one would
// otherwise open a different file with none of the pragmas applied.
func dsn(path string) string {
	// SQLite percent-decodes a file: URI, so a literal "%" needs escaping too.
	uri := strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23").Replace(path)
	return "file:" + uri + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
}

// migrate applies all pending migrations, bringing a fresh or partially
// migrated database up to the latest schema. It is safe to call on every boot.
func migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
