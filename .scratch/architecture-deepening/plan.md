# Architecture deepening

The ten candidates from `/improve-codebase-architecture` (2026-08-18). All ten ship
**before the first deployment**: with no release out there is no live data, no users
and no shipped migration, so every deepening is free now and costs more later. The
`/code-review` runs between the waves rather than once at the end, each one since
the commit before its wave's first ticket — `7309b22` for A. (An earlier draft of
this line named `bb523cb`, which a history rewrite has since orphaned: the SHA
still resolves but shares no ancestor with `main`, so a three-dot diff against it
fails.)

Tickets in `issues/` are numbered in **build order**, not by candidate number. Each
ticket names its candidate. The numbers are reserved up front so a `Blocked by:`
line has something to point at: `01` = 5 · `02` = 1 · `03` = 4 · `04` = 6 ·
`05` = 3 · `06` = 10 · `07` = 8 · `08` = 7 · `09` = 9 · `10` = 2. `11` and up are
therefore not candidates but tickets this effort turned up on its way through —
the first is `11`, from Wave A's review.

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

**`internal/auth` came back off this list, 2026-08-21.** Candidate 8's grill
reopened it deliberately: the deletion test asked whether the package earns its
keep, which it does, and answered nothing about whether it is the right size.
Ticket `07` grows it from two password primitives into the module that owns
authentication, so that `Verify` can go unexported and the compiler can hold
"only this package checks a password". Everything else on the list stands.

## Wave A's `/code-review` (2026-08-21)

Since `7309b22`, both axes. Nothing was implemented wrongly and nothing the three
tickets asked for is missing; the findings were one latent defect and four places
the same thing was said or done twice. What was applied:

- **`helpListing` printed a heading per *change* of group**, not per group, so a
  table whose entries had drifted out of their blocks would print one twice — the
  one invariant `commands`'s own comment claims and no test held.
  `TestEachGroupGetsOneHeading` now holds it, against a deliberately interleaved
  table. The listing for the real table is byte-identical either way, checked.
- **`TestSubcommandsNeedExactlyOneUsername` still listed the six username
  subcommands by hand** — exactly the second listing ticket `04` set out to
  delete, and it would have skipped a seventh silently. It reads them off
  `commands` by their argument sketch now, and fails if none has one.
- **"A Session outlives the Trainer row" was written out four times.**
  `session.RevokeAll` owns it (it already owned the encoded-values reason it
  follows from); the three others point at it, per `docs/agents/comments.md`.
  `internal/session`'s package doc also pointed at `RevokeAll` "for where the key
  is kept", which is a dozen lines below the pointer.
- **The two boundary tests shared their whole walk.** It is `internal/archtest`
  now, with each test supplying only its allowed directories and what to look for,
  and it finds the module root by `go.mod` rather than by `"../.."`. Both were
  re-checked against planted violations — an aliased import, and a fixture in an
  allowed directory's subtree — because a boundary test that passes proves nothing
  on its own.
- **The two last-active-trainer refusal tests were line-for-line identical** but
  for the act. One table over the two acts, with only "still there" differing;
  re-checked by removing the guard, which fails both subtests.

Left alone, with reasons: `RevokeAll` returning a count while `RevokeOthers` does
not (asymmetric, but each is documented and the count has a caller); the fixture
duplication *between* test packages — `club`/`addTrainers`, `trainerID` /
`trainerIDOf` — which wants a `trainertest` shared by `cmd/organizer` and
`internal/web`, and is a ticket rather than a review fix (`issues/11`, the first
ticket that is not one of the ten candidates, which is why it starts at 11); and
the review's
scope-creep list (the `README` tree lines, ADR-0010, the extra `help_test.go`
tests), all of which are wanted and recorded.

`go vet`, `go test ./...`, `go test -race ./...`, `staticcheck` and `govulncheck`
all clean afterwards (`govulncheck` 2026-08-21: 0 reachable, 1 in a required
module the code does not call — unchanged, and this wave added no dependency).

## Candidate 7, grilled 2026-09-04

`/grill-with-docs`, three rounds, every recommendation taken. Wave C's first
ticket is `issues/08-rank-as-a-value-and-one-display-module.md`, `ready-for-agent`.
The Decisions there are the record; three of them change something at this plan's
level:

- **A new package, `internal/rankview`,** takes the belt graphic and the composed
  rank name out of `internal/web`, with four exported functions — one per distinct
  rendering, which is four and not six, because the detail page's current rank and
  its history rows render identically and differ only in what surrounds them.
- **A third boundary test** joins the two Wave A left behind, and it is the first
  that is a claim about **literals** rather than about imports or calls: no `.go`
  file outside the package may hold a belt hex or a rank-catalog key prefix. That
  is coarser than the other two and can produce a false positive; the ticket says
  so. It is recorded in `docs/agents/analysis.md`, which already anticipated a
  third.
- **ADR-0009 gets a dated `## Update`,** not a rewrite: its last Decision bullet
  describes the scalar signatures this ticket ends, and that sentence was true when
  written. ADR-0004 and `CONTEXT.md` get nothing — the quarantine ADR-0004 decided
  is exactly what the new package is, and the glossary gains no term, because how a
  rank is shown is view vocabulary.

## Candidate 9, grilled 2026-10-02

`/grill-with-docs`, two rounds, every recommendation taken. Ticket
`issues/09-the-roster-query-is-resolved-at-the-edges.md`, `ready-for-agent`. One
decision changes something at this plan's level: the review's "the redirect seam
collects only 4 of the ~14 call sites; the template hrefs are the other question"
is answered by splitting the sites. The four redirects and the two render helpers
move into this ticket, so no handler holds the query any more. The template
hrefs stay Go-built data fields and are left to candidate 2 (`10`), which is the
order Wave C already assumed. No ADR and no glossary term.

## Candidate 2, grilled 2026-10-06

`/grill-with-docs`, two rounds, every recommendation taken. Ticket
`issues/10-the-roster-page-has-a-view-model.md`, `ready-for-agent`, the last of
Wave C. One decision changes something at this plan's level: the view model stays
in `internal/web` and is tested in-package, rather than becoming a package of its
own the way `internal/rankview` did in `08`. Almost every field of the roster page
is a URL, and the URLs belong to the router, which is in `web`; `rankview` could
leave because a rank's display has no URL. The candidate's "unreachable" is
therefore answered by one in-package entry point, `rosterPage`, not by an export.
No ADR and no glossary term.
