# 02 — Offboarding acts belong to a Trainer module

Status: ready-for-agent
Blocked by: 01 (done)
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 1 of 10 in the architecture review (2026-08-18)

**What to build:** a new package `internal/trainer` that owns the Operator's acts on
a Trainer account — provisioning, resetting a password, revoking sessions,
deactivating, reactivating, deleting — and the rules that go with them.
`cmd/organizer` keeps the prompts and the printing and becomes a caller.
`internal/web`'s tests become callers too, which is the point.

The acts live in `package main` today, so nothing outside that package can invoke
them. `internal/web` therefore sets up its enforcement tests with the store instead:
`auth_test.go:82` deactivates with a bare `store.DeactivateTrainer` — no revocation
of the trainer's sessions, no last-active-trainer refusal — and `auth_test.go:56`
provisions a trainer with `auth.Hash` plus `store.CreateTrainer` rather than through
the act that hashes and explains a taken username. So the side that *enforces*
Deactivated is verified against a weaker definition of the word than the one the
Operator can actually invoke, and there are two definitions of provisioning. That is
correctness, not shape: a rule added to the act would not show up in the tests that
claim to cover its effect.

`spec.md:210,251` says "two seams, both already in the repo, and deliberately no
third". This relocates seam A from `cmd/organizer` to `internal/trainer`; it does not
add one. What stays in `cmd/organizer` is the wiring — argument checking, the
confirmation prompt, the listing's rendering — and its tests shrink to that.

## Decisions

Settled in `/grill-with-docs`, 2026-08-18. Two rounds, every recommendation confirmed.

- **The whole family of acts moves, not just the offboarding trio.** That
  `reset-password` refuses a Deactivated trainer, that `create-trainer` explains a
  name held by one, and that both offboarding acts refuse to take the last login
  away are one rule set around one state. A module holding half of it would leave the
  other half reaching for the store. `revokeSessions` moves because `Deactivate`
  calls it.
- **The listing stays in `cmd/organizer`.** `trainerListing.String()` is output for a
  terminal, and the query behind it has no rule in it — so `main` calls
  `store.ListTrainers` directly and `listTrainers` disappears rather than moving. The
  listing's tests stay where they are.
- **The package is `internal/trainer`.** It reads as the parallel of
  `internal/session`: that module owns a Trainer's Session, this one owns the Trainer
  account. Rejected `internal/operator` — a package named after an actor pulls
  `import.go` and `demo.go` in behind it and becomes the drawer for everything the
  CLI does. Rejected `internal/accounts` — "Account" is not a glossary term, and
  `CONTEXT.md` defines a *Trainer* as the account. The cost is that a local variable
  named `trainer` would shadow the package; the code writes `tr` everywhere already.
- **Free functions taking `*sql.DB`, not a type.** `internal/session` earned its
  constructor by hiding scs and by carrying a Policy. Here there is nothing to hide
  and no policy to state, so a type would be ceremony at every call site. The
  revocation builds its own `session.ForCommand(db)` inside the one place revocation
  happens — the same reason as in ticket 01: no caller should have to obtain a
  session manager in order to sign a Trainer out.
- **The verbs are the domain's:** `Provision` · `ResetPassword` · `RevokeSessions` ·
  `Deactivate` · `Reactivate` · `Delete`, each taking `(db, username)`. `Provision`
  is `CONTEXT.md`'s word; `trainer.CreateTrainer` would stutter. `RevokeSessions`
  keeps returning how many it ended, `Deactivate` keeps returning only an error.
- **The refusals keep naming CLI commands.** `ErrLastActiveTrainer` still says
  "create-trainer first", and the two deactivated-account messages still name
  `reactivate-trainer`. The Operator is the reason these acts sit on the command line
  at all (`CONTEXT.md`, ADR-0010), so the CLI is the only caller there will be:
  splitting each message across two packages would buy a layering purity with no
  second consumer and turn one assertion into two halves. The package doc says so.
- **The last-active-trainer guard moves unchanged.** Candidate 4 (ticket `03`, next
  in wave A) pushes it to the store's write seam. Two moves of one function are
  cheap; two tickets with overlapping diffs are not — the same trade ticket 01 made
  when it left the doubled `TrainerByID` to candidate 8. Ticket `03` will have to ask
  the message question again, because a *store* naming `create-trainer` is not the
  same case as an Operator module naming it.
- **The assertions are redistributed to the seam where they are observable, not
  moved wholesale.** `internal/trainer` asserts what an act leaves behind, in the
  module's own vocabulary: after `Deactivate`, `ResetPassword` is refused and
  `Provision` reports the name as deactivated; the sessions are gone, counted; a
  second `Deactivate` keeps the first date; after `Delete` the name is free; a refusal
  is not half-applied. `internal/web` keeps the enforcement — it already covers all of
  it (`session_test.go:58,74,93,129`, `auth_test.go:240`) — and sets its state up
  through `trainer.Deactivate`, `trainer.Delete` and `trainer.Provision`.
  `internal/trainer`'s tests claim nothing about logins, or the same mechanics would
  be asserted at two levels again.
- **`internal/web` gains the one enforcement test it lacks:** after `Reactivate` the
  same password logs in again. The homecoming shows itself only at a login, and that
  is web's seam.
- **Test Sessions come from a new `internal/session/sessiontest`,** not from starting
  the web app inside `internal/trainer`'s tests. A Session can only be made through
  the middleware, and `session/revoke_test.go:40-64` already does exactly that with
  `httptest.NewRequest` and no `internal/web` — unreachable test code, which is the
  same observation this ticket is about. The `device` scaffolding moves there and both
  test packages share it. What `startApp` was for survives: the test holds a
  `ForServer` manager while the act builds its own `ForCommand` over the same store.
- **A boundary test makes the claim checkable,** the way `session/imports_test.go`
  does for scs. Here the claim is about calls, not imports: `store.CreateTrainer`,
  `DeactivateTrainer`, `ReactivateTrainer`, `DeleteTrainer` and `CountActiveTrainers`
  are called only from `internal/trainer` — and from `internal/store`, whose own tests
  exercise its API. Without it, "one definition of Deactivated" is true but not
  checkable, and the next test helper reaches for the store again, which is the road
  the current state was built on. `store.UpdateTrainerPassword` is deliberately not on
  the list: the self-service change in the account area is the Trainer's own act with
  its own rules and stays in `internal/web`.
- **`CONTEXT.md` gained the term *Offboarding*** during the grill. It carried
  ADR-0010's title, a test file's name and half of wave A without being defined
  anywhere.
- **No ADR.** ADR-0010 made the decision that matters; where its code lives is a
  consequence, and this repo carries that kind of reasoning in package docstrings
  (`store.Open`, `storetest`, `internal/session`).
- **Two commits.** First the move — new package, `main` becomes a caller, the tests
  travel unchanged. Then the seam: web's helpers call the real acts, the assertions
  are redistributed, `sessiontest` and the boundary test arrive. In one commit,
  nobody could see which of nine hundred lines was a decision. The state in between
  is not broken; its tests simply do not claim anything new yet.

## Acceptance

- [ ] `internal/trainer` exists and exports `Provision`, `ResetPassword`,
      `RevokeSessions`, `Deactivate`, `Reactivate`, `Delete`, each taking
      `(db *sql.DB, username string)`, plus `ErrLastActiveTrainer` and
      `ErrTrainerDeactivated`. No type, no constructor.
- [ ] Its package doc says whose acts these are, and why the refusals name CLI
      commands.
- [ ] `cmd/organizer` holds no act with a rule in it: `accounts.go` keeps the `cmdXxx`
      wrappers, the prompts, `confirm`, `withDB`, `singleUsernameArg`, `countSessions`
      and `trainerListing`, and calls `store.ListTrainers` directly. `listTrainers` is
      gone. `cmd/organizer` no longer imports `internal/auth`.
- [ ] A test in `internal/trainer` fails if any package other than `internal/trainer`
      or `internal/store` calls `store.CreateTrainer`, `DeactivateTrainer`,
      `ReactivateTrainer`, `DeleteTrainer` or `CountActiveTrainers`. It names the
      offending file and points at the module.
- [ ] `internal/session/sessiontest` exists and starts a Session for a trainer id
      through the middleware. `session/revoke_test.go` uses it instead of its own
      `device` scaffolding.
- [ ] `internal/trainer`'s tests assert the acts and no logins: the refusals with
      their reasons and left unapplied, `ResetPassword` refused after `Deactivate`,
      `Provision` reporting a deactivated name, the sessions counted away, a second
      `Deactivate` keeping the first date, the freed username, another trainer's
      sessions untouched, the roster untouched after `Delete`, and
      `ErrTrainerNotFound` for an unknown username on every act.
- [ ] `internal/web`'s `addTrainer`, `deactivate` and `deleteTrainer` helpers go
      through `internal/trainer`. No test file outside `internal/trainer` and
      `internal/store` writes trainer state through the store.
- [ ] `internal/web` asserts that the same password logs in again after `Reactivate`.
- [ ] `cmd/organizer`'s tests cover the wiring only: the confirmation prompt in both
      directions, argument validation, `countSessions`, the listing's rendering, and
      that a subcommand's database is migrated and seeded. No test there asserts an
      act's rules any more.
- [ ] `.scratch/trainer-offboarding/spec.md` carries a dated note where seam A is
      named: the seam is now `internal/trainer`, `cmd/organizer` keeps the wiring, and
      no third seam was added.
- [ ] Two commits, in the order above; each one leaves `go test ./...` passing.
- [ ] `go test ./...` and `go test -race ./...` pass; the analysers in
      `docs/agents/analysis.md` run clean, and it records the new boundary test beside
      the scs one.
