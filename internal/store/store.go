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

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens the SQLite database at path with the pragmas the app depends on:
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
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite %q: %w", path, err)
	}
	return db, nil
}

// Migrate applies all pending migrations, bringing a fresh or partially
// migrated database up to the latest schema. It is safe to call on every boot.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
