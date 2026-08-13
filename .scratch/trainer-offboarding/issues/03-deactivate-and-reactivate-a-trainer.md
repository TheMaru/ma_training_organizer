# 03 — Deactivate and reactivate a trainer

Status: done
Blocked by: 02
Spec: `.scratch/trainer-offboarding/spec.md`
Parent: `01-deactivate-or-delete-a-trainer.md`

**What to build:** the operator can take a departed trainer's access away, and
give it back. `deactivate-trainer <username>` keeps the account and refuses it at
login from that moment on, ending the trainer's sessions as it goes.
`reactivate-trainer <username>` undoes it, with the same password as before.

The state is a recorded date, not a flag (ADR-0010), and a deactivated account is
still a `Trainer` — it is `Deactivated`, the glossary's term.

Neither this ticket's act nor [[05]]'s may leave the club with no active trainer,
so the rule arrives here with the state it depends on. It counts trainers who can
log in, not rows in the table.

- [x] A migration adds the nullable deactivation timestamp to the trainers table,
      with a `Down` that drops it. `NULL` means active.
- [x] `deactivate-trainer <username>` records the timestamp and reports what it
      did, in English (operator surface, [[cli-language]] 01).
- [x] After deactivation the trainer's login fails **indistinguishably** from a
      wrong password: same status, same body, same catalog key. No "account
      deactivated" message anywhere.
- [x] The password is verified **before** the state is checked, so a deactivated
      account costs the same argon2 work as any other failed login and the state
      is not measurable in the response time.
- [x] Deactivation revokes the trainer's sessions, through the existing revocation
      mechanism rather than a second copy of it. A live session on another device
      lands on the login page.
- [x] A colleague's sessions are untouched.
- [x] `requireAuth` refuses a session whose trainer is deactivated, extending the
      check [[02]] introduced.
- [x] `reactivate-trainer <username>` clears the timestamp, and the trainer's
      **unchanged** password logs in again.
- [x] Reactivation does nothing else — no password reset, no session changes.
- [x] Deactivating the only active trainer is refused, the account is left
      untouched, and the message names the way through (create the replacement
      first). No override flag.
- [x] The refusal counts **active** trainers: a test with several deactivated
      trainers present fails if the rule counts rows.
- [x] An unknown username fails as a not-found error, like the sibling
      subcommands.
- [x] Deactivating an already-deactivated trainer is not an error.
- [x] CLI-side tests use the `cmd/organizer` seam against a migrated database, as
      the account tests do; the login and session behaviour is tested in the
      `internal/web` black-box seam. No store-level tests are added.
- [x] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.

## Comments

**2026-08-13 — built.**

Migration `00006_trainer_deactivation.sql` adds the nullable `deactivated_at`, and
`Trainer` carries it as a `time.Time` that is zero while the account is active,
with a `Deactivated()` predicate over it. The `Down` was checked by running goose
down and up again against a migrated database in a throwaway test, which was then
deleted: the spec asks for no migration test, and every test database is built by
the same open-and-migrate path the application uses.

Enforcement is in two places, both reading the state off the `Trainer` the caller
already holds. `handleLogin` refuses with `if !ok || tr.Deactivated()`, one
condition and therefore one response — which is what makes the answer identical to
a wrong password by construction rather than by two branches agreeing. `requireAuth`
asks the new `trainerMayUseTheApp`, where a missing and a deactivated account are
both a plain "no" and only a database that cannot answer is an error.

**The acceptance line about verify-before-check is met but not pinned by a test.**
Go's short-circuit puts `auth.Verify` first and nothing observable in a response
distinguishes the two orders — the difference is argon2 work, so the only test that
could fail on a reorder is a timing test, and those are flaky by nature. The
ordering is stated where the code makes it, in `handleLogin`.

**The revocation test was strengthened after review.** Its first version asserted
only that the deactivated trainer's devices land on the login page — which
`requireAuth` now also causes, so deleting the revocation call left the test green.
It now counts the stored sessions before any request is made, and fails at 3 when
it wants 1. Every other new test was mutation-checked as it was written: the
last-active rule against a version counting rows, the idempotent second
deactivation against a plain `SET deactivated_at = CURRENT_TIMESTAMP`, and the
login refusal against dropping the state check.

**One change beyond the ticket, from the standards review.** The two pre-existing
trainer updates now share the new `updateOneTrainer` tail with the two added here.
Writing the new ones in the old hand-copied shape would have left the file with two
shapes for the same rows-affected check, and the comment recording the deliberate
duplication in this file — cited from `docs/agents/analysis.md` — asserting a
decision the file no longer kept. The comment now says what holds: the near-copies
stay, their shared tail does not repeat. Side effect worth having: the
`ErrTrainerNotFound` tail is covered by the existing store tests for the password
and locale updates, which the CLI seam cannot reach (it fails earlier, in
`TrainerByUsername`).

`ErrUsernameTaken`'s wording, which ADR-0010 says has to name a deactivated
account, is [[04]]'s acceptance line and was left alone here.
