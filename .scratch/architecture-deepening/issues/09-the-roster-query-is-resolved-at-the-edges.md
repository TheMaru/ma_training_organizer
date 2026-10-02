# 09 — The roster query is resolved at the edges, not in each handler

Status: done
Blocked by: None — `08` is done
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 9 of 10 in the architecture review (2026-08-18)
Grilled: `/grill-with-docs`, two rounds, 2026-10-02

**What to build:** no handler in `internal/web` holds a `rosterQuery` any more.
The two render helpers and one new redirect function read it from the request
themselves, so a handler cannot forget to carry the trainer's sort and filter,
and the tests that today prove each handler remembered collapse into one table.

## The state this starts from

The query is read with `rosterQueryFrom(r)` and passed on by hand:

| Place | Reads the query | Uses it for |
| --- | --- | --- |
| create, update, delete, promote | `rosterQueryFrom(r)` | the redirect — `redirect(w, r, query.path(…))`, 4 sites |
| new, edit, create/update on error | `rosterQueryFrom(r)` → `renderAthleteForm(…, query, …)` | `Action`, `Cancel` |
| detail, promote on error | `rosterQueryFrom(r)` → `renderAthleteDetail(…, query, …)` | `EditHref`, `PromoteAction`, `Back` |
| every page | `returnTarget`, called from `render` | the language switcher's `Return` |
| roster page | `rosterQueryOf(view.Query)` — the **resolved** query | `NewHref`, row links, chips, `Return` |

Five of the nine handlers read the query only to hand it to a render helper or to
`redirect`. Every one of them is a place that can forget it, which is why
`rosterquery_test.go` proves the carry route by route: Create, Update, Delete (a
table over every sort), DeleteHTMX, RecordingAPromotion and MutationRedirectKeepsTheFilter
differ only in which route they post to. The four normalisation tests — Junk,
Normalised, FilterOnly, Malformed — already run through the delete route alone.

## Decisions

Two grilling rounds, 2026-10-02. Every recommendation was taken.

- **Scope: the redirects and the two render helpers.** `renderAthleteForm` and
  `renderAthleteDetail` lose their `query rosterQuery` parameter and call
  `rosterQueryFrom(r)` themselves. The query is a property of the request, and only
  the two edges that turn it into a URL — rendering and redirecting — need it.
  **Rejected:** collecting only the four redirects, which leaves five handlers
  reading the query solely to pass it on, so the "each call site remembers"
  problem the candidate names would survive at most of its sites.

- **Each edge calls `rosterQueryFrom(r)`; no middleware.** The function is pure,
  reads nothing but the URL, and costs nothing to call twice. **Rejected:** a
  middleware that resolves the query once into the request context, as the locale
  and the trainer are. It buys no saved I/O, adds a context key and a router
  ordering rule, and would still not be the only source — the roster page has to
  render from the *resolved* query (ADR-0007b), so a context value would be
  overridden on exactly the page that matters most.

- **A second redirect function beside `redirect`, not a change to it.** Working
  name `redirectWithinRoster(w, r, bare string)`: it appends
  `rosterQueryFrom(r).path(bare)` and then behaves as `redirect`, HTMX included.
  `redirect` stays bare, because it also serves `/login`, `/` and the language
  switcher's `returnPath`, none of which may carry a roster query. **Rejected:**
  making `redirect` always append — logout would land on `/login?sort=…`, and
  `returnPath`'s target, which already carries its own query, would get a second.

- **Template hrefs stay Go-built data fields.** This ticket changes *who* reads
  the query, not where hrefs are built; that belongs to candidate 2 (ticket `10`,
  the view model), which this ticket exists to come before. **Rejected:** a
  request-aware template func such as `{{withRoster "/athletes/5/edit"}}`. The
  renderer parses one template set per locale up front (ADR-0008), so a func that
  needs the request means a `Clone` + `Funcs` on every request — and it would
  render wrong links on the roster page, which must use the resolved query.

- **The six route-only tests become one black-box table.** Rows: create, update,
  delete and promote, each as a plain POST (expect a 303 `Location`) and as HTMX
  (expect `HX-Redirect`), all with one non-default query that includes a filter,
  e.g. `?sort=rank&dir=desc&system=bjj-adult`. One further row posts
  `/logout?sort=rank&dir=desc` and expects a bare `/login`: that row is the only
  thing that defends the decision to keep two redirect functions, and without it
  a later merge of the two passes every test. The four normalisation tests stay
  as they are. **Rejected:** replacing all ten with an in-package test of
  `rosterQuery`. "No user-controlled string reaches the Location header" is a
  security property and stays proved at the header, and an in-package test file
  needs a written reason in this repo — `locale_test.go` is the only one.
  `TestDeleteReturnsToSortedRoster`'s table over every sort column is folded into
  the normalisation side if it still proves something there (every column
  round-trips, the default as a bare `/athletes`); otherwise it goes.

- **No enforcement test.** Nothing checks that only the edges call
  `rosterQueryFrom`. A new handler that calls it directly produces correct links,
  so the failure is duplication, not a defect; review and the function's doc
  comment hold the rule. **Rejected:** an `internal/archtest` rule, which would be
  a call-graph check for a style problem and would teach archtest to read function
  bodies. The boundary tests of Waves A and C guard correctness or a quarantine.

- **No glossary term, no ADR.** The roster query is view state, not domain
  language — the same reasoning that kept rank display out of `CONTEXT.md` in
  ticket `08`. An ADR fails "hard to reverse".

## Acceptance

- [x] No handler in `internal/web` declares or passes a `rosterQuery`.
      `rosterQueryFrom` is called only by `renderAthleteForm`,
      `renderAthleteDetail`, the new redirect function, `returnTarget` and the
      roster handler.
- [x] `renderAthleteForm` and `renderAthleteDetail` take no query parameter.
- [x] Create, update, delete and promote redirect through the new function;
      `redirect` itself is unchanged and appends nothing.
- [x] The roster page still renders every link from `rosterQueryOf(view.Query)`,
      and `TestRosterUnrepresentedFilterFallsBackToAll` still passes.
- [x] `rosterquery_test.go` has one table test over the four mutation routes ×
      {POST, HTMX} with a filtered, sorted query, plus the bare-`/login` row for
      `/logout`. The six route-only tests it replaces are gone.
- [x] The four normalisation tests and the link tests (`RosterLinksCarry…`,
      `EditFormReturns…`, `AthleteDetailOffers…`, `DefaultRosterLinksCarryNoQuery`)
      pass unchanged.
- [x] The doc comment on `rosterQueryFrom` names the edges that call it, in place
      of "every handler reads it".
- [x] No new in-package test file, no new archtest rule, `CONTEXT.md` and
      `docs/adr/` unchanged.
- [x] `go vet`, `gofmt -l`, `go test ./...`, `go test -race ./...` and
      `staticcheck ./...` are clean.

## Comments

**2026-10-02 — built.** Every Acceptance box holds. Where the build went past or
beside the Decisions above:

- **`redirectWithinRoster` lives in `rosterquery.go`, not beside `redirect` in
  `server.go`.** Its only work is the roster query's, so it sits with the
  function whose doc names it as an edge.
- **The table is `TestOnlyRosterRedirectsKeepTheQuery`.** The logout row asserts a
  bare `/login`, so a name about mutations alone would contradict one of its
  rows. Logout runs as POST and as HTMX, like the four mutation routes.
- **`TestDeleteReturnsToSortedRoster` is folded, as
  `TestEverySortRoundTripsThroughARedirect`.** The new table uses one query, so it
  proves neither that every column passes the whitelist nor that the default
  comes back as a bare `/athletes`; the folded test still does.
- **The logout row is checked against a merge.** With `redirect` made to append
  and `redirectWithinRoster` reduced to a call of it, only the two logout rows
  fail — the row defends the decision it was written for.
