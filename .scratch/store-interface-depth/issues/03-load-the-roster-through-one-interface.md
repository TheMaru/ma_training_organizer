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

**Status:** ready-for-agent

- [ ] One store call takes a Roster query and returns the rows to show, the represented filter options and the resolved query
- [ ] The rows are already filtered; the options are derived from the unfiltered Roster; the returned query has an unrepresented filter already resolved away
- [ ] The raw listing, the option derivation and the filtering are no longer callable from outside the store
- [ ] The resolution of an unrepresented filter happens inside the call, not in the handler
- [ ] The sort-column normaliser is still available to `web`
- [ ] The Roster handler builds every link on the page from the resolved query
- [ ] The existing Roster HTTP tests pass **unchanged** — no assertion edited
- [ ] A store-level test asserts that an unrepresented filter resolves to the unfiltered Roster, and that the options come from the unfiltered set. This is ADR-0007b's guarantee, asserted at the store seam for the first time
- [ ] A store-level test asserts through the new interface that every offered option matches at least one Athlete
- [ ] The partition mechanics — which cells are non-empty, their order, the fallback when two GradingSystems share a sort order, and filtering preserving the order it was given — are tested in an internal store test file, constructing rows directly and using no database. Prior art: the existing internal test file in the i18n module
- [ ] ADR-0007's sentence about where representation is resolved names the Roster module rather than the handler
- [ ] Coverage recorded before and after as absolute statement counts
