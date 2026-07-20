# 02 — Database & migrations

Status: ready-for-agent
Blocked by: 01

SQLite persistence with migrations. **Portable SQL only** (no SQLite-specific features) per ADR-0002.

## Scope

- SQLite driver (`modernc.org/sqlite` preferred — pure Go, no cgo, keeps the single-binary story).
- Migration tooling (goose or equivalent), applied on boot.
- Schema for the domain: `trainers`, session storage (for `scs`), `grading_systems`, `ranks`, `athletes`, `promotions`.
- `ranks.grading_system_id` FK; `promotions` FK to `athletes` with **`ON DELETE CASCADE`** and FK to `ranks`.
- `ranks` carry an explicit order column (display/sort only).

## Acceptance

- Migrations apply cleanly on a fresh DB.
- Deleting an athlete row cascades to their promotions (verified).
