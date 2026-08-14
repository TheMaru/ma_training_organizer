# 04 — Make deactivated accounts legible to the operator

Status: done
Blocked by: 03
Spec: `.scratch/trainer-offboarding/spec.md`
Parent: `01-deactivate-or-delete-a-trainer.md`

**What to build:** the operator can see which trainers exist and which of them are
deactivated since when, and the two neighbouring subcommands stop being silent
about the state.

The app never shows one trainer to another and has no trainer list at all, so
without this the operator is working blind — including on the last-active-trainer
rule from [[03]], whose refusal is unexplainable if you cannot see who is active.

- [x] `list-trainers` prints every trainer with their username and state, and for
      a deactivated one the date it happened.
- [x] The listing carries **no password material**, asserted directly rather than
      implied by a happy-path test.
- [x] The listing's core returns data and prints nothing; the subcommand wrapper
      renders it — the shape the athlete import already uses, and what makes the
      output assertable without capturing stdout.
- [x] `list-trainers` takes no username and rejects extra arguments, like the demo
      subcommands.
- [x] Ordering is stable and obvious (by username), so two runs are comparable.
- [x] `reset-password` on a deactivated trainer is refused with a message that
      names reactivation, so an operator who meant a homecoming learns the verb.
- [x] `revoke-sessions` on a deactivated trainer still succeeds — harmless and
      idempotent, and nobody should have to reason about command order during an
      incident.
- [x] `create-trainer` against a deactivated trainer's username reports that the
      name belongs to a deactivated account, rather than only that it is taken.
      Otherwise the operator hunts for an account the app never displays.
- [x] Tests use the `cmd/organizer` seam against a migrated database. No
      store-level tests are added.
- [x] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.

## Comments

**2026-08-14 — shipped.** `store.ListTrainers` returns a new `TrainerSummary`
rather than a `Trainer`: the password hash is then absent by construction, which is
what the "no password material" line is really asking for, and the test that
asserts it searches every field the entries carry rather than the one column a hash
was expected in. The CLI side is `trainerListing`, a named slice with a `String()`
method — the `importReport` shape — so the core returns data, the wrapper prints,
and the rendering is assertable without capturing stdout.

**Ordering is `ORDER BY LOWER(username), username`.** The plain form was written
first and the standards review caught what it means: under the database's default
collation every capital sorts ahead of every lowercase letter, so `Zoe` would come
before `ada` — an order, but not one an operator reads as one. `LOWER()` is
standard SQL, so ADR-0002's escape hatch stays open; the second key keeps two names
differing only in case from swapping between runs. The test now provisions a
capitalised name, and fails on the plain form.

**Dates are printed as stored, which is UTC.** Rendering them in local time would
put the listing and the database a day apart for an operator reading it near
midnight, which is worse than the offset it fixes. Stated where the rendering
happens.

**Two neighbouring commands, and one that was left alone.** `reset-password` now
refuses a deactivated trainer with `errTrainerDeactivated`, whose message names
`reactivate-trainer`; `create-trainer` keeps returning `ErrUsernameTaken` but wraps
it with the deactivated-account wording, so a caller matching on the sentinel is
unaffected — with a negative test that an *active* name is never called
deactivated. `revoke-sessions` needed no code change, and its test asserts the
revocation happened rather than only that the command was permitted: the first
version passed against a `revokeSessions` that silently no-ops on a deactivated
trainer. Setting the state through the store rather than `deactivateTrainer` is
what leaves a session for the act under test to end.

**The empty listing prints a line naming `create-trainer`.** Not asked for, and
kept: printing nothing at all on a fresh database reads like a failure.

**`govulncheck` was not clean, and this ticket's acceptance says it must be.** Four
reachable standard-library advisories had landed against unchanged code since the
last scan — exactly the "question about the world" `docs/agents/analysis.md`
describes. Fixed by raising the `toolchain` directive to `go1.26.6` (not the `go`
directive, per that file), which brings it back to zero. Committed separately from
the feature.
