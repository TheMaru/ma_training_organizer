# Architecture deepening

The ten candidates from `/improve-codebase-architecture` (2026-08-18). All ten ship
**before the first deployment**: with no release out there is no live data, no users
and no shipped migration, so every deepening is free now and costs more later. The
`/code-review` since `bb523cb` runs between the waves rather than once at the end.

Tickets in `issues/` are numbered in **build order**, not by candidate number. Each
ticket names its candidate. The numbers are reserved up front so a `Blocked by:`
line has something to point at: `01` = 5 · `02` = 1 · `03` = 4 · `04` = 6 ·
`05` = 3 · `06` = 10 · `07` = 8 · `08` = 7 · `09` = 9 · `10` = 2.

**Which candidates get a `/grill-with-docs` first (2026-08-18).** The ones with a
decision still open: 1 (moves seam A, so it touches `spec.md:210,251`), 4 (wants a
dated note on ADR-0010), 8 (reopens the struck `spec.md:181-198`, and the login
ordering is a security property), 7 (the display module's cut, ADR-0009 at the
edge), 9 (the redirect seam collects only 4 of the ~14 call sites; the template
hrefs are the other question) and 2 (what belongs in the view model). The rest —
`04`, `05`, `06` — were written straight from this plan, because their Decisions
follow from the code and change no vocabulary.

## Waves

**A — the offboarding axis.** 5 → 1 → 6, then `/code-review`. Candidate 5 goes
first so candidate 1 has a session verb to call. Candidate 4 was in this wave until
its grill closed it without a ticket — see below.

**B — isolated, small.** 3, 10, 8. Nothing depends on these; 3 is a latent defect.

**C — the display axis.** 7 → 9 → 2, then `/code-review`, then deploy. 7 before 2 so
the view model finds a Rank value already there; 9 before 2 so 2 builds its hrefs
once.

## The candidates, as ranked in the review

1. **Offboarding acts sit in `package main`** and are therefore unreachable.
   `internal/web/auth_test.go:81` deactivates with a bare `store.DeactivateTrainer` —
   no revocation, no last-active-trainer refusal — so the enforcement side is verified
   against a weaker definition of *Deactivated* than the one the Operator can invoke.
   Correctness, not shape. (`spec.md:210,251` says "two seams, deliberately no third";
   this relocates seam A, it does not add one.)
2. **The roster view has no seam.** `rosterFilters` / `rosterLines` / `rosterHeaders`
   are unreachable, so rendered HTML is the test surface — ~250 of `roster_test.go`'s
   757 lines are scrapers.
3. **Ungraded has no predicate.** Four questions over three fields (`roster.go:162`
   slug, `ranklabel.go:74` system name, `promotions.go:87` a bool); none asks
   `RankID == 0`. A GradingSystem without a slug (migration 00004 `DEFAULT ''`) lands
   in the Ungraded cell and renders inside it with its name. Latent defect.
4. **The last-active-trainer guard** belongs at the store's write seam, not in the CLI.
   Wants a dated note on ADR-0010; standard SQL, per ADR-0002. **Closed without a
   ticket, 2026-08-19** — candidate 1 already took the guard out of the CLI. See
   "Candidate 4" below; ticket `03` was never written and its number stays unused.
5. **Sessions belong in a module of their own** — see `issues/01`.
6. **One command table** instead of seven `cmdXxx` + a switch + `helpText`;
   `help_test.go:88` parses `main.go` with `go/ast` to recover the list.
7. **Rank as a value** in the store, plus one surface-aware display module. Touches the
   edge of ADR-0009 (one sentence in its last bullet), not its decision.
8. **Authentication has no module.** The login ordering (decoy before the Deactivated
   read) is defended only by a comment, and `trainerMayUseTheApp` throws away the
   Trainer that two further callers then reload. Deliberately reopens the struck
   `spec.md:181-198`.
9. **Resolve the roster query at the redirect seam** instead of at twelve call sites —
   ten handler tests become one.
10. **`store/date.go` publishes the layout, not the question**, so three callers parse
    for themselves. Sharing the predicate is compatible with ADR-0008; sharing the
    message is not.

## Candidate 4, closed without a ticket (2026-08-19)

`/grill-with-docs`, one round, all three recommendations confirmed. **No code
changes.** Ticket `03` was never written; the number stays reserved and unused so
the `Blocked by:` lines that already point at the others keep their meaning.

The candidate's reason was that the guard sat in `package main`, where nothing
could reach it — `internal/web` therefore set its enforcement tests up with a bare
`store.DeactivateTrainer` and verified a weaker definition of *Deactivated* than the
Operator could invoke. Candidate 1 (ticket `02`) fixed exactly that: the guard is in
`internal/trainer/offboarding.go:25`, every caller can reach it, and
`internal/trainer/calls_test.go` fails if any package outside `internal/trainer` and
`internal/store` writes trainer state through the store. What the candidate asked
for is done; only its proposed location is not.

Pushing it the last step into the store was weighed on its two remaining merits and
rejected on both:

- **Unmissable by construction rather than by test.** Rejected. The rule is made of
  three things the store knows nothing about — Offboarding, the Operator, and the
  `create-trainer` the refusal has to name. And it is not a store invariant: a
  freshly migrated database has no active trainer at all, and that is allowed. The
  store would be guarding a property that does not hold for the store. The boundary
  test is also the mechanism this repo already chose deliberately for the scs
  imports; calling it too weak here would devalue it there.
- **One statement instead of count-then-write.** Rejected. The race needs two
  concurrent `organizer` processes — the acts run only on the command line, and
  `store.Open` caps the pool at one connection. Folding the count into the `UPDATE`
  would also collide with `writeOneTrainer`, which reads "no rows affected" as
  `ErrTrainerNotFound`: a refusal would surface as "trainer not found". Untangling
  that costs more than the theoretical race is worth.

The message question ticket `02` deferred here therefore does not arise: the refusal
stays in `internal/trainer` and keeps naming `create-trainer`.

What was written down instead:

- **`CONTEXT.md`, under *Offboarding*:** an act is refused if it would leave the club
  with no trainer who can still log in. The rule had four spellings —
  `ErrLastActiveTrainer`, "the last-active-trainer guard" here, a consequence in
  ADR-0010, two tests — and no definition. Appended to *Offboarding* rather than made
  its own term: it is a boundary of that act, not a thing in the domain.
- **ADR-0010's last consequence** said the club being left without an active trainer
  is what "the tooling" refuses. Since ticket `02` that is `internal/trainer`, with
  `cmd/organizer` only calling it. Corrected in place, not as a dated addendum: the
  decision did not change, one imprecise word in its consequence did.

## Deliberately left alone

The deletion test said no: `store.Open`, `LoadRoster`/`RosterView`,
`Trainer.Deactivated()`, `TrainerSummary`, the Trainer CRUD near-duplicates,
`internal/auth`, `store/storetest`, `i18n.Lookup` beside `i18n.T`, `CurrentRank`,
`internal/config`.
