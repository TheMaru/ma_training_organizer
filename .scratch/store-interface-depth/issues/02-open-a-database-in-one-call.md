# 02 — Open a database in one call

**What to build:** Opening a database becomes one call that also migrates and
seeds it. Today that is three calls in a required order, the order is written
down nowhere, and it is obeyed six different ways — three of which skip the seed.
The Trainer-visible consequence of that gap: a Trainer created through the CLI on
a fresh database logs in to a promotion form with no GradingSystems in it,
because nothing seeded them.

The separate migrate and seed steps stop being callable from outside the store,
so the order cannot be got wrong. Every test suite then obtains its database from
one shared fixture module rather than three near-identical local copies — the CLI
tests live in `package main` and cannot borrow the store suite's helper, so a
shared module is the only way to have one answer to "what does a test database
look like".

Accepted consequence: no caller can observe a migrated-but-unseeded database.
Nothing needs that today. Reference data is idempotent and transactional, so
seeding on every open is safe.

Two riders travel with this ticket as their own commits, because they touch
neighbouring ground and are too small to be tickets:

- The coverage command, documented in the README's existing development block as
  bare `go` commands alongside the three already there. Not a build tool — Go's
  tooling is self-sufficient and a Makefile would be a second convention for one
  command. Profile output is git-ignored. Baseline at the time of writing: **787
  of 1061 statements, 74.2%**. Record the two absolute numbers, not the
  percentage: a refactor that deletes untested code raises the percentage without
  adding a test.
- A comment on the Trainer CRUD functions explaining why their duplication stays.
  Deleting them would move SQL into the handlers rather than concentrate it
  anywhere — the complexity moves, it does not reduce. An ADR was considered and
  rejected: trivially reversible, so it fails the ADR bar, and this repo has
  recorded evidence that its documentation drifts in proportion to its distance
  from the code.

Spec: [[spec]].

**Blocked by:** None — can start immediately.

**Status:** done

- [x] Opening a fresh database yields one that is both migrated and seeded, in one call
- [x] Opening an already-prepared database again changes nothing
- [x] The migrate and seed steps are no longer callable from outside the store
- [x] A database path containing a query separator does not corrupt the connection settings
- [x] A Trainer created through the CLI on a fresh database finds the seeded GradingSystems available when recording a Promotion
- [x] All three test suites — store, web and CLI — get their database from one shared fixture module
- [x] The two verbatim copies of the insert helper are reduced to one
- [x] The two database fixtures that differed only by filename are reduced to one, and the seeded-versus-unseeded distinction between fixtures is gone
- [x] The coverage command is in the README's development block; the profile output is git-ignored
- [x] The Trainer CRUD duplication carries its reasoning as a comment
- [x] Coverage recorded before and after as absolute statement counts

## Comments

**Coverage, before and after.** Before: **787 of 1061** statements (74.2%). After:
**793 of 1060** statements (74.8%), counting the packages that existed before —
covered up six from the new `Open` tests, total down one because the deleted
duplicate helpers were statements too. Measured over everything `./...` reports,
the after figure is **793 of 1081** (73.4%): the new `storetest` package
contributes 21 statements that no test targets directly, since it is the fixture
the other suites run *through* rather than a package with tests of its own. That
gap between 74.8% and 73.4% is the reason the absolute counts are what get
recorded.

**One deviation from the plan, recorded.** `storetest` also holds a seeded-rank
lookup, which the ticket did not list. Once the seed stopped being callable, the
store and web copies of that lookup became byte-identical to each other — the same
condition that put the insert helper in the module. The grading-system lookup in
`internal/web/i18n_domain_test.go` stayed local: one caller, and moving it would
be a shared helper for nothing.
