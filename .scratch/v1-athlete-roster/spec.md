# Spec — v1: Athlete Roster with Graduation

Status: shipped except deployment (issue 07, `ready-for-human`)

Foundation for the martial-arts organizer. Grounded in `CONTEXT.md` (glossary) and `docs/adr/0001`–`0003`.

## Goal

Trainers log in and maintain a shared roster of athletes and their graduation history.

## In scope (v1)

- **Trainer login** (session auth) + self-service password change. Accounts and password resets via CLI, no self-registration, no email.
- **Athlete CRUD**: `firstName`, `lastName`, `birthDate`, `joinedOn`, `notes`. List sortable by last name. Hard delete with cascade.
- **Graduation**: record promotions (grading system + rank + date); show derived current rank + full history per athlete.
- **Seeded grading systems**: "BJJ Kids" and "BJJ Adult", including stripe ranks.
- **Deployment** to Fly.io (EU) with an SQLite volume + Litestream backups.

## Out of scope (parked for later)

Trainer hours · billing / administrative membership · training planning · attendance & regularity · training groups (kids vs adults) · multiple grading systems in concurrent use · in-app consent flag · data export · injury / health tracking · email password-reset flow · self-registration · in-app roles/permissions.

## Domain (see `CONTEXT.md`)

`Athlete`, `Trainer`, `GradingSystem`, `Rank`, `Promotion`. Current rank = the athlete's most recent promotion by date; history may cross grading systems (kids → adult).

## Non-functional constraints

- **Portable SQL** — no SQLite-specific features (ADR-0002).
- **EU hosting**; hard-delete erasure; ~30-day backup retention; **no Art. 9 data**; HTTPS; trainer-only access (ADR-0003).
- Ships as a single Go binary + one SQLite file.

## Acceptance — v1 is done when

1. A provisioned trainer can log in, add / edit / delete an athlete, record a promotion, and see the current rank + history.
2. Deleting an athlete also removes their promotions.
3. The app runs on Fly.io (EU) with a persistent SQLite file and working, pruned backups.

## Tickets

See `issues/`. Dependency order is recorded via `Blocked by:` lines.

## Implementation conventions

Agreed for this feature; applies to every ticket unless a ticket says otherwise.

- **Testing (TDD).** Test test-first at three seams: the store/data-access layer,
  the pure domain logic (notably current-rank derivation), and the HTTP handlers.
  Handler tests are **behaviour-based** — assert on status codes, redirects, DB
  side effects and single semantic signals, **never** against rendered HTML markup
  (that couples tests to layout and drifts on every change).
- **Issue 07 (deployment).** Produce the config artifacts only (`Dockerfile`,
  `fly.toml`, `litestream.yml`, a short `DEPLOY.md`); do **not** deploy. It stays
  `ready-for-human` — the human runs `fly deploy` after a local review.
- **Workflow.** One `/implement` per ticket, clearing context between tickets, so
  each ticket is built fresh within the model's sharp-reasoning window. Commit
  each ticket as its own checkpoint on `main`.
