# 11 — One fixture for provisioning a trainer

Status: ready-for-agent
Blocked by: None
Plan: `.scratch/architecture-deepening/plan.md`
Origin: Wave A's `/code-review` (2026-08-21), not one of the ten candidates —
which is why this is `11`: the numbers `01`–`10` are reserved for candidates and
`03` stays unused.

**What to build:** one answer to "what does a provisioned trainer look like in a
test?", the way `storetest` is one answer to what a test database looks like — or
a recorded decision that the duplication stays.

Three test packages provision trainers, and each spells the same fixture itself:

- `internal/trainer/trainer_test.go:14,18` — `trainerPassword` plus `club`, which
  makes the database and provisions into it.
- `cmd/organizer/offboarding_test.go:16,21` — `trainerPassword` again, byte for
  byte, plus `addTrainers`, which provisions into a database it is handed.
- `internal/web/auth_test.go:23,68` — `testPassword`, the same string under a
  different name, plus `addTrainer` for one trainer at a time.

`"correct-horse"` therefore appears four times: those three constants and a fourth
as a bare literal in `cmd/organizer/accounts_test.go:21`, which does not use its
own package's `trainerPassword`.

The id lookup is duplicated too — `trainerID` (`internal/trainer/trainer_test.go:45`)
and `trainerIDOf` (`internal/web/auth_test.go:93`) are the same three lines under
two names, and now carry the same pointer at `session.Manager.RevokeAll`. So does
the reason each fixture exists: `club` and `addTrainers` both explain in their own
words that most tests need two trainers because the last-active-trainer rule
refuses an act on the only one left.

`internal/web` also wraps each act in a `t.Fatalf` helper of its own — `deactivate`,
`reactivate`, `deleteTrainer` (`auth_test.go:74-110`). Nothing else has those yet.

## Decisions

Not grilled, and deliberately not: this is test scaffolding, it touches no term in
`CONTEXT.md`, no ADR and no production code, and it is reversible by deleting a
directory. What it does have is one question worth answering before any code
moves, which the first acceptance box makes the ticket's first step.

- **Answer the deletion question first, and close this unbuilt if it says no.**
  The repo has form here: candidate 4 was closed without a ticket once its reason
  had already been met, and `plan.md`'s "Deliberately left alone" list is the
  deletion test saying no ten times over. A fixture package that saves three lines
  per caller and costs an import is exactly the shallow module this repo keeps
  declining. The case *for* is that the duplication is not the three lines but the
  fourth thing — the reason, currently written out three times, that a fixture
  holds two trainers rather than one. Weigh those, decide, and record the decision
  under `## Comments` either way. **A close with reasons is a successful outcome
  of this ticket**, not a failure to finish it.
- **If it is built: `internal/trainer/trainertest`, and it goes through the acts.**
  `sessiontest` and `storetest` set the precedent, including the placement — a
  fixture lives beside the module whose vocabulary it deals in. It must call
  `trainer.Provision` and the other acts, never `store.CreateTrainer`: that is what
  keeps it on the right side of `TestTrainerIsTheOnlyPackageThatActsOnAnAccount`
  without being added to that test's allowed list, and it is the same reason
  `internal/web`'s helpers already go through the acts (`auth_test.go:64-67`).
- **The password is the fixture's, not each caller's.** One exported constant, one
  spelling. `cmd/organizer/accounts_test.go:21` uses it instead of the bare
  literal.
- **`club` keeps making the database; the fixture does not.** `internal/trainer`
  wants a database with trainers on it, `internal/web` builds a whole server round
  one, and `cmd/organizer` needs a *path* because the subcommands open it
  themselves (`cliDatabase`, `offboarding_test.go:73`). Those three are genuinely
  different and stay where they are — the fixture provisions into a database it is
  handed, which is the part all three share.
- **The act wrappers are a judgement call, not a requirement.** Moving
  `deactivate`/`reactivate`/`deleteTrainer` out of `internal/web` would give
  `internal/trainer`'s own tests a wrapper round the function under test, which is
  worse than the two lines it saves. Take the provision helper and the id lookup;
  leave the act wrappers with their one caller unless a second one appears.
- **No ADR, and no `CONTEXT.md` change.** Where a test fixture lives is trivially
  reversible and surprises nobody (`docs/agents/analysis.md` states this repo's ADR
  bar).

## Acceptance

- [ ] The deletion question is answered in writing under `## Comments`: is the
      shared thing worth a package, or does the duplication stay? If it stays,
      every box below is struck through with that reason and the ticket is `done`.
- [ ] One spelling of the test password across the three packages, and
      `cmd/organizer/accounts_test.go` no longer carries the bare literal.
- [ ] One provision helper, called by all three packages, which goes through
      `internal/trainer`'s acts. `trainerPassword`, `testPassword`, `addTrainers`
      and `addTrainer` are gone.
- [ ] One id lookup. `trainerID` and `trainerIDOf` are gone, and the pointer at
      `session.Manager.RevokeAll` is stated once rather than twice.
- [ ] The reason a fixture holds two trainers is written once, in the fixture, and
      no longer restated by each caller.
- [ ] `TestTrainerIsTheOnlyPackageThatActsOnAnAccount` passes **unchanged** — the
      new package is not added to its allowed list, because it calls the acts and
      not the store.
- [ ] `club` still makes its own database, `newAuthTestServer` still builds its
      server, and `cliDatabase` still returns a path. No test's behaviour changes:
      the suite is the same assertions with less scaffolding.
- [ ] `go test ./...` passes and the analysers in `docs/agents/analysis.md` run
      clean.
