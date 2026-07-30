# Roster filter options come from the roster, not from the seed

## Context

The roster gains a filter that narrows `/athletes` to one grading system — the
system of each athlete's *current* rank, since an athlete is not bound to a
single system (ADR-0001). ADR-0005 already fixed its shape: a row of links, not a
form. Its identity in the URL is a slug (ADR-0006). What remained open was where
the filter's **options** come from, and that turns out to decide considerably
more than which chips get rendered.

Both seeded systems (`BJJ Kids`, `BJJ Adult`) exist in the database regardless of
who is on the roster, but v1 manages kids only. Offering "every seeded system" as
the options would therefore render, in v1's *default* state, a `BJJ Adult` chip
that is guaranteed to match nobody — manufacturing an empty-result state that no
user error produced, on a page whose empty-state markup (`{{if .Athletes}}` in
`athletes.html`) wraps the filter row itself and so would take the trainer's way
back out with it.

## Decision

**(a) The options are the non-empty cells of the roster's own partition.**

Every athlete falls under exactly one of: a grading system (the system of their
current rank) or "ungraded" (no promotions at all — a distinct category, not the
lowest rank). These partition the roster; `Alle` is their union. The options
offered are those cells that actually contain someone, derived from the
**unfiltered** roster. Deriving them from the filtered roster would leave only
the active option standing and delete the trainer's route back.

Three properties follow, and they are the point of the decision:

- A requested option that is not among them falls back to `Alle`, in the spirit
  of `NormalizeRosterSort` — filtering is a view concern, not a data error.
- Therefore **every offered option matches at least one athlete, and no filter
  can yield an empty roster.** There is no zero-hit state and no zero-hit UI;
  `athletes.html` is not restructured. An empty roster means no athletes exist.
- The row renders only from **two** options upward. With one, `Alle` and that
  option show the same list, and two chips for one view is noise — so in a
  homogeneous roster the filter appears not at all, and surfaces by itself when
  there is something to partition.

**(b) Validation is two-stage, and one query feeds both the options and the
filter.**

Whether a slug is *well-formed* is a property of the URL; whether it is
*represented* is a property of the data. Only one handler needs the second.

- **Form**, in `rosterViewFrom`, pure and without a database: a slug shape,
  `none`, or empty; anything else reads as empty. This keeps the file's guarantee
  that no user-controlled string reaches a rendered URL or a `Location` header,
  and keeps `query()` free of escaping — though the guarantee now rests on the
  value having a fixed *shape* rather than coming from a fixed *set*.
- **Representation**, in the roster handler alone: the eight other call sites
  carry the filter through untouched and never ask the database about it.

The roster handler loads the unfiltered roster **once** and derives both the
options and the filtered rows from that same slice, through pure functions over
`[]RosterRow` in `store`. The invariant in (a) is then true by construction: with
one source there is nothing for a second one to drift from.

## Considered Options

- **The seeded grading systems as options (rejected):** the obvious reading of
  "data-driven", and it keeps the chips stable as data changes. Rejected because
  it creates the permanently-empty `BJJ Adult` chip described above. Both designs
  are data-driven; this one is merely less derived.
- **Ungraded athletes visible under every filter (rejected):** a filter reading
  `BJJ Kids` that shows athletes who are not in BJJ Kids no longer tells the
  truth, and under the rank sort their NULL rank keys go to the end via
  `NULLS LAST` in both directions — reproducing exactly the cross-system blocking
  the filter exists to remove.
- **No representation check, plus a zero-hit UI (rejected, but the way back):**
  filter in SQL, let an unrepresented slug return no rows, and build the "no
  matches" message with a reset link. Genuinely simpler in code and the coherent
  alternative should this invariant ever feel too clever. Rejected because it
  trades logic for template surface that only exists to apologise for a state the
  other design cannot enter.
- **`rosterViewFrom` as a method on `*Server` (rejected):** it would reach the
  database without polluting any signature, since every call site is already a
  handler method, and eight extra `DISTINCT`s over a club-sized roster on local
  SQLite cost nothing. Rejected on coupling, not cost: it gives *parsing the view
  state* a database error path in all nine handlers — the delete handler would
  have to decide what to do when it could not determine which chips are populated
  — it makes a URL's meaning depend on who was deleted a second ago, everywhere,
  and it costs `rosterview_test.go` its freedom from a database fixture for a
  property that has nothing to do with URL grammar.
- **A separate store query for the options, plus a test pinning the invariant
  (rejected):** the same behaviour, but with two sources that must agree. It
  makes the invariant enforced by evidence rather than by construction, which is
  the weaker of the two for a guarantee that licenses *absent* UI.

## Consequences

- The chip row changes with the data. The first athlete promoted into `BJJ Adult`
  makes a chip appear; the last one to leave makes it vanish, and a bookmarked
  `?system=bjj-adult` then resolves to `Alle`. This is intended behaviour, and it
  is documented here so it does not later read as a bug.
- `athletes.html` keeps both chip rows inside `{{if .Athletes}}`, which is
  correct rather than merely tolerable: no athletes, no controls. The filter row
  sits above the sort row — narrow first, then order — and carries a visible
  muted `Filtern` label, parallel to `Sortieren`, as ADR-0005 requires.
- The comment in `internal/web/rosterview.go` about values coming from a fixed
  set must be rewritten to say fixed *shape*. Security-wise the two are
  equivalent — `[a-z0-9-]` rules out escaping and header injection just as a
  closed set does — but a future reader has to understand the difference.
- The whole roster is fetched to display a subset. Irrelevant at club size, and
  sorting stays in SQL. Pushing the filter into a `WHERE` clause later would
  require a separate options query and would reintroduce the drift this avoids;
  that is a deliberately deferred trade, not an oversight.
- If the whitelist is ever rebuilt on `store.ListGradingSystems` "for
  simplicity", the zero-hit state becomes reachable and the trainer's reset
  disappears without an error or a failing test. That is the invisible failure
  mode ADR-0005(b) is about, which is why the derivation lives next to the
  filtering rather than in a query of its own.
- "Cohort" deliberately does **not** enter the domain model. The filter is a
  grading-*system* filter; kids/adults coincide with systems only while there is
  one discipline. A real cohort axis spanning disciplines would be a second
  filter, not a rename of this one.
