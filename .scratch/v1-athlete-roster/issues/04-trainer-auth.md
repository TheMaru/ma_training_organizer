# 04 — Trainer authentication

Status: done
Blocked by: 02

Session-based auth for trainers. No self-registration, no email (ADR-0002).

## Scope

- Password hashing with `alexedwards/argon2id`.
- Server-side sessions with `alexedwards/scs`, SQLite-backed store (revocable sessions).
- Login / logout; auth middleware gating all app routes.
- Self-service password change (verify current → set new).
- CLI commands: `create-trainer` (provision account) and `reset-password`.

## Acceptance

- Unauthenticated requests to app routes redirect to login.
- Correct credentials log in; wrong password fails.
- Password change works end-to-end.
- `create-trainer` produces a working login; `reset-password` changes it.

## Comments

Closed 2026-07-24 — implemented and merged to main (ace41ae).
