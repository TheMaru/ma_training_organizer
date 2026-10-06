# 10 — The roster page has a view model

Status: ready-for-agent
Blocked by: None — `08` and `09` are done
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 2 of 10 in the architecture review (2026-08-18)
Grilled: `/grill-with-docs`, two rounds, 2026-10-06

**What to build:** one pure function, `rosterPage(locale, store.RosterView)`, that
turns the store's answer into everything `athletes.html` renders, as one typed
value. The handler shrinks to load, build, render. The roster tests whose claim is
about data stop scraping HTML and assert on that value instead; only the tests
whose claim is about markup keep going through HTTP.

## The state this starts from

`handleAthletesList` assembles a `map[string]any` by hand from three unexported
builders and two inline paths:

| Key | Built by |
| --- | --- |
| `Athletes` | `rosterLines(view.Rows, query)` |
| `Headers` | `rosterHeaders(locale, query)` |
| `Filters` | `rosterFilters(locale, query, view.Options)` |
| `NewHref` | `query.path(rosterPath + "/new")`, inline |
| `Return` | `query.path(rosterPath)`, inline |

No test reaches any of it. Every claim about the roster's links, active states and
chips is proved by rendering the page and scraping it: `roster_test.go` is 758
lines, and its fifteen helpers (`headerCell`, `sortChip`, `filterChip`,
`attrValue`, `rosterRow` and the rest) exist only to recover from markup what the
builders had in hand as plain values. Three of its tests also re-prove row order
and filter narrowing, which `internal/store/roster_test.go` already proves
(`TestListRosterSortsByEachColumn`, `TestListRosterFallsBackToFirstNameAscending`,
`TestLoadRosterResolvesAnUnrepresentedFilter`, the `roster_internal_test.go`
partition tests).

## Decisions

Two grilling rounds, 2026-10-06. Every recommendation was taken; the first
question was asked twice, because the first answer argued from ticket size rather
than from the design.

- **The view model stays in `internal/web`, tested in-package.** Nearly every
  field it holds is a URL — `NewHref`, `Return`, each header's and chip's `Href`,
  each row's `Href` and `DeleteAction` — and those URLs are the router's:
  `server.go` defines the routes and `athletes.go` spells them once
  (`rosterPath`, `athletePath`, `deletePath`). The page data belongs with the
  package that owns the URL space. **Rejected:** a new package
  `internal/rosterview`, tested black-box. It would have to own `rosterQuery` and
  the path helpers, because the page is made of them, while the router stays in
  `web` — two packages spelling one URL space, kept in step only by tests.
  `internal/rankview` (ticket `08`) could be its own package because a rank's
  display needs only store data and a locale and has no URL; the roster page has
  no such cut. What this costs, said plainly: the test is in-package rather than
  black-box, and the compiler does not hold the query whitelist — any code in
  `web` could write `rosterQuery{sort: "junk"}`. Today only `rosterQueryFrom` and
  `rosterQueryOf` build one, and the normalisation tests hold the `Location`
  header. What would make the other package right: a second consumer of the
  roster page outside `web`, such as an export or a JSON API. There is none.

- **One entry point that returns the whole page, as a typed struct.**
  `rosterPage(locale i18n.Locale, view store.RosterView) rosterPage`, with fields
  `Athletes`, `Headers`, `Filters`, `NewHref` and `Return`. It derives the query
  from `view.Query` itself (`rosterQueryOf`), so the resolved query of ADR-0007b
  is the only one it can see. The handler is `LoadRoster` → `rosterPage` →
  `render`, and passes the value under one key, e.g.
  `{"Authenticated": true, "Roster": page, "Return": page.Return}`, with
  `athletes.html` reading `.Roster.Headers` and so on. `Return` stays at the top
  level as well, because `base.html`'s language switcher reads it there and
  `render` fills it in only when absent. The three builders stay as they are, as
  helpers behind the entry point; the test calls `rosterPage` only. **Rejected:**
  keeping the map and testing the three builders directly. `NewHref` and `Return`
  would stay reachable only through HTML, and `Return` is the one link only the
  resolved query knows; the "seam" would be three functions rather than one.
  **Out of scope:** typed data for the other five pages and any change to
  `render`'s map. Those are a later ticket if wanted.

- **Tests split by what they claim: data moves, markup stays.** The new
  in-package test builds `store.RosterView` values directly, with no database and
  no server, as `internal/store/roster_internal_test.go` builds rows. The split,
  test by test:

  | Test | Goes to |
  | --- | --- |
  | `RosterDefaultsToFirstNameAscending` | view model: header active, ▲, aria-sort value, hrefs. Its order check goes — store proves it |
  | `RosterSortsByRequestedColumn` | view model, same split |
  | `RosterUnknownSortParamsFallBackToDefault` | stays on HTTP as the smoke test: junk params → 200, Vorname active. Its order check goes |
  | `RosterSortChipsLinkWhereTheHeadersDo` | stays — both surfaces render `.Headers`; the claim is the template's |
  | `RosterSortChipMarksTheActiveColumn` | stays — `aria-current` and the `sr-only` text are markup |
  | `RosterSortChipRowIsALabelledNav` | stays (the column-order half may move) |
  | `RosterSortTargetIsTheWholeHeaderCell` | stays |
  | `RosterSortIndicatorIsHiddenFromAssistiveTech` | stays |
  | `RosterShowsCurrentRank`, `…UnknownColour`, `BeltMarkupIsIdenticalAcrossSurfaces` | stay — `rankview` markup |
  | `RosterFilterNarrowsToOneSystem` | goes — store proves it |
  | `RosterFilterChipsOfferEveryNonEmptyCell` | view model: labels, order, active, the bare Alle href. One `aria-current` check stays on HTTP |
  | `RosterFilterChipsKeepTheSort` | view model |
  | `RosterUnrepresentedFilterFallsBackToAll` | stays — it is the only test that the *handler* renders from the resolved query rather than from the request, which the view model cannot see |
  | `NoOfferedFilterYieldsAnEmptyRoster` | goes — `store.TestEveryOfferedOptionHasAthletes` plus the view model's "each chip links to its option" hold it together |
  | `RosterFilterRowIsAbsentWithFewerThanTwoOptions` | view model: no chips below two options. "An empty `Filters` renders no row" stays on HTTP |
  | `RosterFilterRowIsALabelledNavAboveTheSortRow` | stays |
  | `RosterLinksCarryTheQuery`, `RosterLinksCarryTheFilter`, `DefaultRosterLinksCarryNoQuery` (in `rosterquery_test.go`) | view model — row `Href`, `DeleteAction` and `NewHref` are fields now |

  The link tests for other pages (`EditFormReturns…`, `AthleteDetailOffers…`)
  and the redirect and normalisation tests from ticket `09` are untouched.
  Scraper helpers that no remaining test calls are deleted. **Rejected:** keeping
  the three order tests on HTTP as end-to-end cover; they prove what the store
  tests already prove, through the slowest surface.

- **The in-package test file follows the repo's naming and carries its reason.**
  `internal/web/roster_internal_test.go`, after
  `internal/store/roster_internal_test.go` and
  `internal/i18n/fallback_internal_test.go`. Its header says why it is
  in-package: `rosterPage` is unexported because the URLs it builds belong to this
  package's router, and the property worth pinning is about values, not markup.

- **No glossary term, no ADR.** The view model is view vocabulary, not domain
  language — the reasoning tickets `08` and `09` gave. An ADR fails "hard to
  reverse".

## Acceptance

- [ ] `rosterPage(locale, store.RosterView) rosterPage` exists in `internal/web`,
      is pure, and derives its query from `view.Query` only.
- [ ] `handleAthletesList` is `LoadRoster` → `rosterPage` → `render`, and builds
      no link or chip itself.
- [ ] `athletes.html` reads the roster's data from the one key; `Return` still
      reaches the language switcher.
- [ ] `internal/web/roster_internal_test.go` exists with a header that gives its
      reason, builds `store.RosterView` values directly, and calls only
      `rosterPage`.
- [ ] The tests in the table above are moved, kept or deleted as it says; no
      scraper helper is left without a caller.
- [ ] `TestRosterUnrepresentedFilterFallsBackToAll` and the ticket `09` redirect
      and normalisation tests pass unchanged.
- [ ] Rendered roster HTML is byte-identical before and after for at least the
      default view and one sorted, filtered view (checked once by hand or by a
      throwaway golden diff, not a kept test).
- [ ] The other five pages still render from `map[string]any`; `render` is
      unchanged.
- [ ] `CONTEXT.md` and `docs/adr/` unchanged.
- [ ] `go vet`, `gofmt -l`, `go test ./...`, `go test -race ./...` and
      `staticcheck ./...` are clean.

## Comments
