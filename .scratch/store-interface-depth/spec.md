# Spec — Store interface depth: roster protocol, database setup, promotion writes

Status: ready-for-agent

Three deepenings in `internal/store`, taken from an architecture review of the
hot spots in recent history (roster sort/filter/mobile, i18n, rank rendering,
CSV import). Grounded in `CONTEXT.md` and ADR-0001, ADR-0004 through ADR-0009.

All three land **before the first deployment** (`README` status: v1 built, not
yet deployed), which is why none of them is ordered by user-facing severity.

## Problem Statement

Two audiences have a problem, and one of them is the Trainer.

**The Trainer.** A Trainer who submits an athlete form with a malformed birth
date — a hand-crafted POST, or any client where the browser's date picker is not
in play — permanently breaks the Roster page for *every* Trainer. The bad value
is accepted, written, and then fails on every subsequent read, so the shared
Roster answers with a server error until someone edits the database by hand. The
same class of failure exists for a Promotion's date; there it is guarded at the
form, but only there. Nothing in the store refuses the value.

**Whoever changes this codebase next, human or agent.** Three interfaces in
`store` are shallow in the specific way that makes change hazardous:

- The Roster is loaded through four functions that must be called in one exact
  sequence, with one exact argument — the *unfiltered* Roster. That sequence
  exists in prose in a doc comment and in exactly one place in the code. Calling
  the functions in the wrong order compiles and returns a plausible, wrong
  answer, and no test can catch it. ADR-0007 claims the resulting invariant
  holds "by construction"; it holds by convention.
- Opening a database means `Open`, then `Migrate`, then `Seed`, in that order.
  That contract is unwritten and is obeyed six different ways across production
  code and test fixtures — three of the six omit the seed, so a Trainer created
  through the CLI on a fresh database ends up with no GradingSystems and an empty
  promotion form.
- `Promotion` serves as both the write and the read type. Five of its nine fields
  are silently ignored on insert, documented only in a comment. Its own current-
  rank derivation justifies its correctness with "ISO dates compare lexically" —
  an invariant the write path does not enforce.

## Solution

**For the Trainer:** a malformed date is refused. The store rejects it on write
with a distinct error, and the athlete form reports it the way the promotion form
already does — a 400 with a message and the entered values preserved, not a
server error, and never a poisoned read.

**For whoever changes the code next:** each of the three interfaces gets one
entry point, with the invariant behind it rather than beside it.

- The Roster loads through a single call that takes a query and returns a view:
  the rows to show, the filter options actually represented, and the resolved
  query. The unfiltered Roster never leaves the module, so the misuse becomes
  unstateable and ADR-0007's "by construction" becomes literally true.
- Opening a database is one call that migrates and seeds. A shared test-fixture
  module gives every test suite the same database in one line.
- A Promotion is exactly what `CONTEXT.md` says it is — an Athlete, a Rank, a
  date — and the denormalised display fields move into a separate row type, the
  same way the Roster's row type already relates to Athlete.

## User Stories

1. As a Trainer, I want a malformed birth date to be refused when I save an athlete, so that I get a correctable error instead of silently storing bad data.
2. As a Trainer, I want a malformed joined-on date to be refused the same way, so that both optional dates behave alike.
3. As a Trainer, I want the entered values preserved when a date is rejected, so that I can fix the one wrong field instead of retyping the form.
4. As a Trainer, I want the Roster page to keep loading no matter what anyone has submitted, so that one bad record cannot take the shared Roster away from the whole club.
5. As a Trainer, I want a malformed promotion date refused by the store and not only by the form, so that the guard cannot be bypassed by a client that skips the form.
6. As a Trainer, I want an Athlete's current Rank to keep deriving from the most recent Promotion by date, so that nothing about my recorded history changes.
7. As a Trainer, I want the Roster's sort, filter and phone chips to behave exactly as they do today, so that this work is invisible to me apart from the fixed defect.
8. As a Trainer, I want a bookmarked filter for a GradingSystem nobody is in any more to still resolve to the unfiltered Roster, so that a stale link never shows me an empty table.
9. As a Trainer, I want the filter row to keep appearing only when there is something to partition, so that a homogeneous Roster is not cluttered with two chips showing the same list.
10. As a Trainer created through the CLI on a fresh database, I want the GradingSystems to be present when I first log in, so that the promotion form is usable immediately.
11. As a maintainer, I want one call that loads the Roster, so that the sequence I have to get right does not exist.
12. As a maintainer, I want the requested and the resolved Roster query to have the same shape, so that building the page's links from the resolved state is obvious rather than remembered.
13. As a maintainer, I want the "options come from the unfiltered Roster" rule enforced by the interface, so that a future simplification cannot silently reintroduce the empty-result state ADR-0007 exists to prevent.
14. As a maintainer, I want the Roster's partition rules testable without a database, so that the cheap cases — a system with no members, a tie in system order — stay cheap to write.
15. As a maintainer, I want ADR-0007's claim to be true as written, so that I can trust the ADR log instead of re-deriving whether a stated guarantee holds.
16. As a maintainer, I want one call that yields a ready database, so that no call site has to know that migrating comes before seeding.
17. As a maintainer, I want one shared test fixture for a database, so that "what a test database looks like" has one answer instead of three.
18. As a maintainer, I want the duplicated insert helper in the store and web test suites collapsed into one, so that the two copies cannot drift.
19. As a maintainer, I want the Promotion type to match the glossary definition, so that reading the type teaches me the domain rather than the storage layout.
20. As a maintainer, I want the write path to refuse a date the read path cannot parse, so that the two halves of the store agree about what a date is.
21. As a maintainer, I want the ISO layout spelled in one place, so that the store and the CSV import cannot disagree about it.
22. As a maintainer, I want the existing HTTP tests to pass unchanged, so that I have evidence the refactor preserved behaviour rather than an assertion that it did.
23. As a maintainer, I want a documented way to measure coverage, so that "fewer tests, same coverage" is a claim I can check rather than assert.
24. As a maintainer, I want to know why the Trainer CRUD functions were deliberately left duplicated, so that I do not spend an afternoon deduplicating something that was decided.

## Implementation Decisions

### Roster: one interface, protocol removed

The seam stays in `store`. The three pure functions over Roster rows and the
web-side resolution of an unrepresented filter move behind a single call. A new
package between `store` and `web` was rejected: the Roster's row type embeds
Athlete, so either that type migrates and `store` loses its central type, or the
new package imports `store` and `web` imports both — a package for one production
caller and a type that still lives elsewhere.

The interface takes a query value and returns a view value, deliberately the same
three fields in and out, because every link on the page is built from the
resolved state:

```
RosterQuery { Sort string; Descending bool; Filter string }
RosterView  { Query RosterQuery; Rows []RosterRow; Options []RosterOption }
```

`Rows` is already filtered, `Options` is derived from the *unfiltered* Roster,
and `Query` is resolved — an unrepresented filter has already fallen back to the
unfiltered Roster. The unfiltered slice never escapes the module.

`LoadRoster`'s three collaborators — the raw listing, the option derivation, the
filtering — become unexported, as does the web-side representation check. A
second exported entry point that skips the partition was rejected for the same
reason the protocol is being removed: it is the door the protocol comes back
through.

The sort-column normaliser **stays exported**. It has three call sites, all in
the URL-state carrier, none in the handler path, and it runs before any database
access — ADR-0007b's two-stage validation depends on that carrier staying
database-free. It is a pure, idempotent whitelist query with no ordering
contract, so it cannot be called wrongly. This means the interface shrinks from
four functions to two, not to one; the *protocol* is what disappears, and that is
the point.

Naming follows the glossary entry added during grilling. There is exactly one
Roster; sorting and filtering produce views of it. So the returned type is a
Roster **view**, and `web`'s existing private view-state type is renamed to a
Roster **query** — the URL asks a query, the store answers with a view. Its
URL-rendering method is renamed so that a type called query does not carry a
method called query. `RosterPage` was rejected: "page" implies pagination, and
ADR-0007 deliberately fetches the whole Roster.

### Database setup: one call, one fixture

`Open` migrates and seeds. The migrate and seed functions become unexported. The
three call sites that omit the seed today are the latent defect, so making the
seed optional is making it forgettable. The consequence accepted: no caller can
observe a migrated-but-unseeded database. Nothing needs that today; a read-only
tool later would reopen this decision.

Two fixture helpers collapse into one, since a seeded database is now the only
kind. A new `storetest` module under `store` holds the database fixture and the
insert helper, because the CLI tests are in `package main` and cannot import a
`store_test` helper — a shared module is the only way to deduplicate across the
three suites. This is explicitly **not** a second adapter at the seam; there is
one adapter, the file-backed SQLite database, and `storetest` is a fixture. The
seam is justified by the ordering contract disappearing, not by adapter count.
It is also not a net removal of lines: roughly thirty lines of duplication are
replaced by a module of comparable size. The gain is one source of truth for what
a test database is.

Rider, since the function is being opened anyway: `Open` appends its pragma query
to the path by string concatenation and therefore assumes the path contains no
query separator.

### Promotion: the domain type, and the date invariant

The write type is cut back to the glossary definition — an Athlete, a Rank, a
date — and the denormalised display fields move to a separate row type that
embeds it. This is the pattern the codebase already uses for the Roster's row
type, which embeds Athlete: a row is the domain value plus what a view needs to
render it.

```
Promotion    { ID, AthleteID, RankID int64; PromotedOn string }
PromotionRow { Promotion; RankName, SystemName, SystemSlug, Group string; Degree int }
```

Named fields are what protect the two adjacent identifiers here, not distinct
types; making the identifier types distinct is a codebase-wide typing policy and
is out of scope (see below).

The ISO date invariant is enforced **in the store on write** — the athlete create
and update paths and the promotion create path reject a malformed date with a
distinct sentinel error — **and additionally at the athlete form**, which today
has no date validation at all while the promotion form does. Store-side is the
backstop that closes the seam; form-side is what turns the refusal into a
message a Trainer can act on rather than a server error. A dedicated date type
that makes a malformed value unconstructable was considered and rejected for now:
it is the stronger form, but it is not a net removal of code — it replaces
roughly as much null-and-format plumbing as it adds — and it ripples through both
Athlete and Promotion, both web forms, the templates and the CSV import.

The current-rank derivation stays as it is, retyped to the new row type. The
architecture review proposed collapsing it with the history query into one call;
that was wrong. The only caller needs the full graduation history *and* the
current Rank, and already holds the slice, so a combined call would issue a
second query for data in hand. There is also no protocol to remove: the function
is order-independent by construction and documented as such. Depth is for
invariants that need an enforceable home, not for every pair of calls.

The test that pins the SQL recency rule against the Go derivation stays and gains
importance: once the raw listing is unexported, it is the only thing tying the two
encodings of "current Rank" together.

### Documentation and measurement

- `CONTEXT.md` gains a `Roster` entry — the complete, shared set of Athletes,
  with sorting and filtering producing views of it. **Already done during
  grilling.** It closes a real gap: two ADRs are largely about the Roster and the
  glossary never defined it.
- ADR-0007 needs one sentence adjusted: representation resolves inside the Roster
  module rather than in the handler. Its "true by construction" claim is not
  weakened but earned — no rewrite, no superseding ADR.
- The coverage command is documented in the existing development block of the
  README as bare `go` commands, matching the three already there. A build tool is
  not the Go standard and would be a second convention for one command. The
  profile output is git-ignored. Baseline at the time of writing: **787 of 1061
  statements, 74.2%**. Percentage alone is the wrong measure for this work — a
  refactor that deletes untested code raises it without a new test — so the two
  absolute numbers are what get recorded.
- The deliberate duplication in the Trainer CRUD functions gets a code comment
  explaining why it stays: deleting those functions would move SQL into the
  handlers rather than concentrate it. An ADR was considered and rejected — the
  decision is trivially reversible and so fails the ADR bar, the reader who needs
  it has the file open anyway, and this repo has recorded evidence that its
  documentation drifts in proportion to its distance from the code.

## Testing Decisions

A good test here asserts external behaviour at the highest available seam and
says nothing about how the module reaches its answer. For this work the strongest
tests are the ones that **do not change**: the existing HTTP suite covering the
Roster's sorting, filter chips, phone chips and stale-filter fallback must pass
untouched. That is evidence the refactor preserved behaviour; an assertion in a
commit message is not.

Three seams, two of them already in use, one new.

**HTTP seam (existing).** Prior art: the Roster, athlete and promotion HTTP
suites, which drive a real server with a logged-in Trainer and read the rendered
HTML. Used here for the Trainer-visible promise: an athlete form with a malformed
date answers 400 with a message and preserved input, and the Roster page still
loads afterwards. Also the untouched-suite check described above.

**Store package seam (existing).** Prior art: the existing external store suite
against a temporary SQLite database. Used here for:

- Opening a fresh database yields one that is migrated *and* seeded; opening it
  twice is idempotent; a path containing a query separator does not break the
  pragma.
- `LoadRoster` resolves an unrepresented filter to the unfiltered Roster and
  derives its options from the unfiltered set. This is ADR-0007b's guarantee,
  asserted for the first time at the store seam instead of three layers up.
- Every offered option matches at least one Athlete — moved down from the pure
  functions to the interface, because after the change it is a statement about
  the module rather than about two functions agreeing.
- The date invariant, including the regression case reproduced during grilling:
  creating an Athlete with a malformed birth date is refused, and the Roster
  listing runs cleanly afterwards.

**Store internal seam (new, precedented).** The repo already has one internal
test file, in the i18n module, for exactly this reason: reaching mechanics the
external interface deliberately hides. The Roster's partition rules move here —
which cells are non-empty, their order, the fallback when two GradingSystems
share a sort order, and the filter preserving the order it was given. These
construct row values directly and need no database, which is why they stay cheap.
The five moved tests keep their assertions; only the package and the called
identifiers change.

The type-only changes — the Promotion split, the current-rank retyping, the
fixture consolidation — get **no new tests**. They are proven by the existing
suites compiling and staying green. Adding tests for a type rename would be
testing the compiler.

## Out of Scope

- **The other architecture-review candidates.** Athlete identity living in the
  demo seeder, rank display spread across six files, and the untested CLI command
  shells are all real and all deferred to a second round.
- **Distinct identifier types** for Athlete, Rank, Trainer, Promotion and
  GradingSystem. Genuinely worthwhile, but it is an all-or-nothing policy across
  the whole store — half-applied, a bare integer identifier stops meaning
  anything — and its best justification lives in the demo seeder, which the
  deferred athlete-identity work rewrites anyway. Scheduled after that.
- **A dedicated date type.** Rejected for now, with reasons recorded above. This
  is a deferral, not a rejection on principle.
- **Pushing the Roster filter into SQL.** ADR-0007 records this as a deliberately
  deferred trade; nothing here revisits it.
- **A coverage threshold or CI gate.** There is no CI in this repo, and a
  threshold would punish deleting untested code, which is one of the outcomes this
  work is aiming for.
- **Deployment**, which is the remaining v1 issue and unaffected by any of this.
- **Any change to Trainer, GradingSystem or Rank storage**, to the schema, or to
  the migrations. No migration is written for this work.

## Further Notes

**Ordering.** Five tickets. Two can start immediately: the naming prefactor in
`web`, and the database setup. The database setup goes early because it touches
every test fixture, and the tickets after it write tests that would otherwise be
written twice. The Roster work is gated on both. The date defect and the Promotion
type split are separate tickets, sequenced rather than parallel because both edit
the promotion create path and the same two test files.

Two things changed while drafting the tickets, and this paragraph is the
correction:

- The naming prefactor was not in the original three-ticket plan. It is
  mechanical, green on its own, and makes the Roster ticket smaller — "make the
  change easy, then make the easy change".
- The Promotion work was going to be one ticket, on the claim that the defect fix
  and the type split "edit the same three write functions". Counting the edges
  showed that overstated: the date work sits mostly in the Athlete path, the type
  split in the Promotion path, and they genuinely overlap only at promotion
  creation and two test files. Split.

Splitting the defect fix out to *ship* it early was considered and dropped once
the not-yet-deployed status was confirmed: there is no production to ship it to,
so severity does not order this set. Everything lands before the first
deployment.

**Corrections to the architecture review that produced this spec**, recorded so
the report is not read as authoritative where it was wrong:

- "The interface shrinks four to one" — it shrinks to two. The sort normaliser
  stays exported for good reason.
- "Two adapters justify the seam" for the database setup — there is one adapter
  and one test fixture. The ordering contract justifies the seam.
- "One call for the current Rank instead of two" — the two-call shape is correct
  at the only call site, which needs both values from one fetch.
- The review understated the date defect. It reported the promotion date, which
  is guarded at the form. The athlete dates are guarded nowhere, and the failure
  takes down the shared Roster rather than one Athlete's detail page.

**What this work does not buy.** With one production caller, the Roster deepening
gains no leverage — there are no N call sites paying back one implementation. The
gain is locality: an invariant that ADR-0007 rests on gets an enforceable home,
and the interface stops describing a sequence. That is the honest case for it, and
it is the reason the hot-spot weighting mattered: the Roster is where recent
change keeps landing.
