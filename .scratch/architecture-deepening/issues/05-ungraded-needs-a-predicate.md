# 05 — Ungraded needs a predicate

Status: done
Blocked by: None — Wave B depends on nothing
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 3 of 10 in the architecture review (2026-08-18)

**What to build:** one predicate that answers "is this athlete ungraded?", asked
of the field that actually decides it — `RankID` — and used everywhere the
question is currently asked of something else. It closes a latent defect on the
way.

`CONTEXT.md` calls **Ungraded** an athlete who is in no grading system at all,
and `store.RosterRow` says the same in prose: "Rank fields are empty/zero for an
ungraded athlete, who has no rank at all — that is distinct from the lowest rank,
which is a graduation" (`roster.go:41-43`). Three call sites ask that question and
none of them asks `RankID`:

- `internal/store/roster.go:163` — `row.SystemSlug == ""` decides whether the
  filter row offers an Ungraded chip at all.
- `internal/store/roster.go:198-204` — `filterRoster` turns the Ungraded filter
  into `slug = ""` and matches rows on `SystemSlug`.
- `internal/web/ranklabel.go:74` — `line.SystemName == ""` decides whether the
  accessible rank label carries a system in brackets.

**The latent defect.** Migration 00004 added `slug` as `NOT NULL DEFAULT ''`, and
`ensureGradingSystem` (`seed.go:104-122`) fills it only for the systems the seed
itself knows, matched by name. So a grading system that reaches the table any
other way has an empty slug — and its athletes, who hold a real rank, are counted
as Ungraded by both store sites while `rosterRankLabel` still renders their rank
*with the system's name*. They appear under the "Ohne Graduierung" chip showing a
belt. Nothing can produce such a system today: there is no UI and no subcommand
for grading systems, so the seed is the only door. That is why this is latent and
not a bug report — but it is one insert away, and the review found it by reading,
which means nothing else would have.

## Decisions

Not grilled. The domain term already exists in `CONTEXT.md` and needs no
sharpening; what is missing is a predicate in the code, and where it goes is
forced by the types.

- **The predicate is a method on `store.RosterRow`, and it asks `RankID == 0`.**
  `Trainer.Deactivated()` is the precedent in this repo — a one-field question the
  domain has a word for, answered in one place. `RankID` is the field the LEFT JOIN
  leaves zero (`roster.go:306`), so it is the field that decides.
- **`internal/web` needs no new field.** `rosterLine` embeds `store.RosterRow`
  (`web/roster.go:50`), so the predicate is already in hand at `ranklabel.go:74`.
  That is what keeps this ticket out of candidate 2's way: the view model's shape
  is not touched, only the question it asks.
- **`filterRoster` keeps matching on the slug for a *named* system.** The Ungraded
  cell is the only branch that changes: it selects rows by the predicate instead of
  by an empty slug. A system filter still means "this slug", which is ADR-0006's
  identity rule and is correct as it stands.
- **`store.CurrentRank`'s `ok` stays as it is** (`promotion.go:95`, read at
  `web/promotions.go:87`). It reports whether a promotion list is empty, which is a
  statement about a list and not about a roster row — the same *answer* by a
  different route, and collapsing the two would tie the detail page's data to the
  roster's row type for nothing.
- **The template is left alone.** `athletes.html:69` renders an empty cell because
  every rank field is empty, not because it asks the question; there is nothing
  there to redirect. Its comment already says so.
- **No ADR, and no `CONTEXT.md` change.** ADR-0006 (slug identity) and ADR-0007
  (option source) already decided everything this touches; the term is already
  defined. This is the code catching up with both.

## Acceptance

- [x] `store.RosterRow` has one exported predicate for Ungraded, answering
      `RankID == 0`, with a docstring that says why the rank id and not the slug
      decides it.
- [x] `rosterFilterOptions` and `filterRoster` both use it. No site in
      `internal/store` decides Ungraded by an empty `SystemSlug` any more.
- [x] `rosterRankLabel` uses it instead of `line.SystemName == ""`.
- [x] A regression test pins the defect: a grading system inserted with an empty
      slug (`storetest.MustInsert`) and an athlete promoted inside it is **not**
      offered as Ungraded, is **not** returned by the Ungraded filter, and keeps
      its system in the rank label. The test names the migration-00004 default as
      the reason such a row can exist.
- [x] Ungraded athletes still behave exactly as they do today: they get the chip,
      the Ungraded filter returns them, and their rank cell renders blank.
- [x] `store.CurrentRank` and `athletes.html` are unchanged.
- [x] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.

## Comments

Two things came up that the ticket did not foresee. Both were raised before the
edit that would have caused them.

- **The Ungraded branch could not be the only branch that changed.** Once a
  slugless system stops counting as Ungraded, it becomes its own option with
  `Value: ""` — and `""` is already Alle. `rosterFilters` would have rendered two
  chips with the same href, both of them `Active`. So `rosterFilterOptions` now
  skips a row whose `SystemSlug` is empty as well, after the predicate has had its
  say: a system with no slug has no identity in a URL (ADR-0006), so it can carry
  no chip. Its athletes stay on the roster under Alle, with their rank. That is a
  narrower defect than the one the ticket set out to close, but it is a real one,
  and it contradicts ADR-0007(a)'s claim that the offered cells partition the
  roster. ADR-0007 carries an Update saying so.
- **`rosterRankLabel` needed a second guard.** Asking `Ungraded()` instead of
  `SystemName == ""` opens a path the old check closed: a graded athlete whose
  system has an empty name would have rendered `Weiß ()`. It now returns the bare
  rank when the resolved system label is empty. The question that guard asks is
  "is there anything to disambiguate with?", which is a label concern, not a
  second proxy for Ungraded.

The label half of the regression needed a sharper case than the ticket described.
A slugless system still has a *name*, so the old `SystemName == ""` check passes
that case too — the test would not have failed if the predicate were reverted.
`TestRosterRankLabelNamesTheSystemOfAnyRankedAthlete` therefore also asserts the
case that discriminates: a rank in a system with a slug and no stored name, which
the old check silenced and the predicate names.

`go vet`, `gofmt -l`, `go test ./...`, `staticcheck` and `govulncheck` all clean
(`govulncheck` 2026-09-01: 0 reachable, 2 in required modules the code does not
call).
