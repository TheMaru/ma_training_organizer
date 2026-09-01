# 11 — One fixture for provisioning a trainer

Status: done
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

- [x] The deletion question is answered in writing under `## Comments`: is the
      shared thing worth a package, or does the duplication stay? If it stays,
      every box below is struck through with that reason and the ticket is `done`.
- [x] One spelling of the test password across the three packages, and
      `cmd/organizer/accounts_test.go` no longer carries the bare literal.
- [x] One provision helper, called by all three packages, which goes through
      `internal/trainer`'s acts. `trainerPassword`, `testPassword`, `addTrainers`
      and `addTrainer` are gone.
- [x] One id lookup. `trainerID` and `trainerIDOf` are gone, and the pointer at
      `session.Manager.RevokeAll` is stated once rather than twice.
- [x] The reason a fixture holds two trainers is written once, in the fixture, and
      no longer restated by each caller.
- [x] `TestTrainerIsTheOnlyPackageThatActsOnAnAccount` passes **unchanged** — the
      new package is not added to its allowed list, because it calls the acts and
      not the store.
- [x] `club` still makes its own database, `newAuthTestServer` still builds its
      server, and `cliDatabase` still returns a path. No test's behaviour changes:
      the suite is the same assertions with less scaffolding.
- [x] `go test ./...` passes and the analysers in `docs/agents/analysis.md` run
      clean.

## Comments

### The deletion question, answered: build it (2026-09-01)

**Built.** `internal/trainer/trainertest` now holds `Password`, `Provision` and
`ID`, and the three suites call it.

The case for a close was real: a fixture package that saves three lines per caller
and costs an import is the shallow module this repo keeps declining, and
`plan.md`'s "Deliberately left alone" list is that judgement made ten times over.
Three things outweighed it.

**`storetest`'s own package doc already states the deciding constraint.** "The CLI
tests live in package main and cannot import another suite's unexported helper — a
shared package is the only way for 'what a test database looks like' to have one
answer." That sentence is true word for word of a provisioned trainer. The three
suites that need one are `internal/trainer`, `internal/web` and `cmd/organizer`,
and the last is `package main`. There is no cheaper place for the shared answer to
live; the alternative is not a smaller abstraction, it is three copies.

**The precedent is not merely available, it has already survived this test.**
`store/storetest` sits on the "Deliberately left alone" list — the deletion test
was asked of it and said keep. `sessiontest` was built by ticket `01` for exactly
this reason and placement. A third fixture beside a third module is the pattern
being followed, not a new one being invented.

**The duplication was never the three lines.** It was four facts, and each was
written more than once:

- the password, four spellings of one string;
- that a fixture provisions through the acts rather than the table, so a test
  starts from a state the Operator can reach — three times;
- that a fixture holds two trainers because an Offboarding act refuses to take the
  club's last login away — three times, in three wordings;
- that the id is read while the account is still there, `session.Manager.RevokeAll`
  for why — twice.

The last two are domain rules, not scaffolding. Wave A's own `/code-review` found
"a Session outlives the Trainer row" written out four times and gave it one owner;
this is the same finding one layer down. A rule stated in three places drifts in
three directions, and a test comment that has drifted is worse than none: it
describes a constraint the fixture no longer has.

### What was left alone, and why

- **`deactivate`, `reactivate`, `deleteTrainer` stay in `internal/web`.** One caller
  each. Moving them would wrap `internal/trainer`'s own tests round the function
  under test. The shared paragraph above them — that they go through the acts —
  stays, now as a comment over the group rather than on a helper that is gone.
- **`account` moved after all, as `trainertest.Account`.** It stayed put in the
  first pass, and `/code-review` called that what it was: `trainertest.ID` had
  reimplemented it line for line, format string included, so the ticket's "one id
  lookup" held by name and not by count. `Account` is now the one lookup and `ID`
  is `Account(...).ID`, which is what the deleted `trainerID` already was.
- **The three database fixtures stay put.** `club` still calls `storetest.NewDB`,
  `newAuthTestServerIdle` still builds its server, `cliDatabase` still returns a
  path. Only the provisioning moved.
- **No ADR, no `CONTEXT.md` change**, as the ticket decided.

### Verification

`go test ./...` green, `go vet ./...` clean, `staticcheck ./...` exit 0, `gofmt -l`
empty. `TestTrainerIsTheOnlyPackageThatActsOnAnAccount` passes with its allowed
list unchanged: `trainertest` calls `trainer.Provision`, never
`store.CreateTrainer`. No assertion in the suite changed: every reference to
`testPassword` became `trainertest.Password` and nothing else about those tests
moved. `govulncheck` was not re-run: this touches no dependency (see
`docs/agents/analysis.md`).

### The cost, priced

The ticket named it as "costs an import", and the honest figure is 17 files, not
one — deleting a package-level const reaches every file that read it. That is a
one-off, paid in a single mechanical pass, and it is the same shape of cost
`sessiontest` and `storetest` already carry. It does not change the answer, but the
answer should not have been recorded without it.

### What `/code-review` changed afterwards

Both axes ran; nothing was implemented wrongly and no assertion had moved. Six
things were applied.

- **A comment that was confidently wrong.** `Provision` claimed a database holding
  one trainer is "a state the Operator cannot reach". It is not — provisioning the
  club's first Trainer produces exactly that, and two tests in this repo do it.
  Only an *Offboarding* act is refused there. The sentence was true of
  `internal/web`'s fixtures and became false the moment it was widened to every
  caller, which is the failure mode `docs/agents/comments.md` names.
- **`trainertest.Account`**, above: the duplication the first pass left.
- **A free-floating paragraph.** `internal/web`'s "these helpers go through the
  acts" note had lost its declaration when `addTrainer` went, so godoc dropped it
  and its "three helpers below" also covered `newClient`. It hangs on
  `deleteTrainer` now.
- **A restatement instead of a pointer.** The package doc reproduced `storetest`'s
  package-main reasoning *and* pointed at it. The pointer is enough.
- **`cmd/organizer/accounts_test.go` calls the helper**, rather than
  `trainer.Provision` with the fixture's password — the last direct call in a test.
- **`internal/archtest`'s doc named two fixtures**, and `trainertest` is now the
  third and the sharpest example: it sits *under* an allowed directory and is
  checked anyway.

### What `/reuse-and-trim` changed afterwards

**Reuse.** Exporting `Account` turned two more places into callers, both of which
had the same four lines written out:

- `cmd/organizer/listing_test.go:110` — `store.TrainerByUsername` plus its own
  `t.Fatalf`.
- `internal/web/auth_test.go` — four `before, _ := store.TrainerByUsername(...)`
  lookups whose error went to `_`. That mattered: a failed lookup left both hashes
  at their zero value, the two compared equal, and the assertion "the hash did not
  change" passed without having looked at a hash. They fail now. `internal/web`
  stopped importing `internal/store` in its auth tests as a result.

**Trim.** Two comments pointed at the package where the rule asks for the
identifier (`trainertest` → `trainertest.Provision`), and `club`'s doc restated its
own signature before pointing; both cut to one sentence.

**One overclaim of the fix's own.** `Provision`'s doc said that with one trainer
provisioned "every act a test could aim at them is refused". Not true — a password
reset is not. It names deactivation and deletion now, which are the two the rule
actually refuses. Same failure mode as the finding it replaced, one round later,
which is the argument for stating the rule and not its consequences.
