# Architecture deepening

The ten candidates from `/improve-codebase-architecture` (2026-08-18). All ten ship
**before the first deployment**: with no release out there is no live data, no users
and no shipped migration, so every deepening is free now and costs more later. The
`/code-review` since `bb523cb` runs between the waves rather than once at the end.

Tickets in `issues/` are numbered in **build order**, not by candidate number. Each
ticket names its candidate.

## Waves

**A — the offboarding axis.** 5 → 1 → 4 → 6, then `/code-review`. Candidate 5 goes
first so candidate 1 has a session verb to call.

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
   Wants a dated note on ADR-0010; standard SQL, per ADR-0002.
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

## Deliberately left alone

The deletion test said no: `store.Open`, `LoadRoster`/`RosterView`,
`Trainer.Deactivated()`, `TrainerSummary`, the Trainer CRUD near-duplicates,
`internal/auth`, `store/storetest`, `i18n.Lookup` beside `i18n.T`, `CurrentRank`,
`internal/config`.
