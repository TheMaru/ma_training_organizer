# 03 — Load the Roster through one interface

**What to build:** One store call takes a Roster query — sort column, direction,
filter — and returns everything the page needs: the rows to show, the filter
options that are actually represented, and the resolved query. Today that is four
functions which must be called in one exact sequence, passing the *unfiltered*
Roster to two of them. That sequence lives in prose in a doc comment and in
exactly one place in the code; calling them in the wrong order compiles and
returns a plausible wrong answer that no test can catch.

After this, the unfiltered Roster never leaves the module and the misuse is not
expressible. ADR-0007 already claims the resulting invariant — every offered
filter matches at least one Athlete, so no filter can produce an empty Roster —
holds "by construction". Today it holds by convention. This makes the claim
literally true, so the ADR is earned rather than corrected.

The field shapes for the query and the returned view were agreed during grilling
and are recorded in the spec; follow them there rather than re-deriving them.

The sort-column normaliser **stays available to `web`**: it runs when the URL is
parsed and when a column header is clicked, both before any database access, and
ADR-0007b's two-stage validation depends on the URL carrier staying
database-free. It is a pure idempotent whitelist query with no ordering contract,
so it cannot be misused. The interface therefore goes from four functions to two,
not to one — the *protocol* is what disappears, and that was the defect.

Nothing a Trainer can see changes. Sorting, the filter chips, the phone sort
chips and the fallback for a bookmarked filter nobody is in any more all behave
exactly as they do now.

Spec: [[spec]].

**Blocked by:** [[01-name-the-roster-url-state-a-query]] (the view/query vocabulary this
builds on) and [[02-open-a-database-in-one-call]] (its new tests should be written
against the shared fixture, not written twice).

**Status:** done

- [x] One store call takes a Roster query and returns the rows to show, the represented filter options and the resolved query
- [x] The rows are already filtered; the options are derived from the unfiltered Roster; the returned query has an unrepresented filter already resolved away
- [x] The raw listing, the option derivation and the filtering are no longer callable from outside the store
- [x] The resolution of an unrepresented filter happens inside the call, not in the handler
- [x] The sort-column normaliser is still available to `web`
- [x] The Roster handler builds every link on the page from the resolved query
- [x] The existing Roster HTTP tests pass **unchanged** — no assertion edited
- [x] A store-level test asserts that an unrepresented filter resolves to the unfiltered Roster, and that the options come from the unfiltered set. This is ADR-0007b's guarantee, asserted at the store seam for the first time
- [x] A store-level test asserts through the new interface that every offered option matches at least one Athlete
- [x] The partition mechanics — which cells are non-empty, their order, the fallback when two GradingSystems share a sort order, and filtering preserving the order it was given — are tested in an internal store test file, constructing rows directly and using no database. Prior art: the existing internal test file in the i18n module
- [x] ADR-0007's sentence about where representation is resolved names the Roster module rather than the handler
- [x] Coverage recorded before and after as absolute statement counts

## Comments

**Done.** `store.LoadRoster(db, RosterQuery) (RosterView, error)` is the only way
into the Roster. `ListRoster`, `RosterFilterOptions` and `FilterRoster` are
unexported, and `web`'s `representedFilter` moved into the store — so the
unfiltered Roster no longer leaves the module and the sequence that had to be
obeyed no longer exists. `NormalizeRosterSort` stays exported, as the spec
requires.

`internal/web/roster_test.go` was not touched. The whole HTTP suite passes
unchanged, which is the evidence that the refactor preserved behaviour.

**Coverage.** Before: **793 of 1081** (73.4%). After: **799 of 1087** (73.5%).

**Two things the code review caught, both fixed.**

The first was a real defect I had introduced. `rosterQueryOf` ran the store's
resolved filter back through `normalizeSystemFilter`, on the argument that
"no string reaches a rendered URL unchecked" should stay a property of
`rosterquery.go` alone. But the store filters the rows by the value it was given,
so if the two checks ever disagreed the table would show one cell while every
link on the page dropped the filter — reintroducing exactly the drift this ticket
removes, in the one place it had been eliminated. The fix is the other direction:
`RosterQuery` now documents that resolving *narrows but never substitutes* — the
resolved filter is empty or exactly the one asked — and `web` relies on that
instead of re-checking. The form check happens once, on the way in.

The second was the comment carve-out in `docs/agents/comments.md`: the handler
doc and `rosterQueryOf`'s doc each restated what `RosterView` already owns. Both
now point at it.

**One deviation from the plan, recorded.** The spec said ADR-0007 needs *one*
sentence adjusted and no rewrite. Three passages changed instead, because two
others became false rather than merely dated: the paragraph naming the handler as
what loads the unfiltered Roster, and a Consequences sentence claiming the
`ListGradingSystems` regression would pass without a failing test — the new
store-level test is exactly that test. The ADR is still not superseded and the
"true by construction" claim is unchanged.

Also, the internal file's sort-order-tie test is new rather than moved. The spec
said "five moved tests"; there were four, and the tie case the ticket asks for
had never been tested.
