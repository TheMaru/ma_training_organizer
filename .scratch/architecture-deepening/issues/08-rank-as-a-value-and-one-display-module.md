# 08 — Rank is a value, and one module renders it

Status: ready-for-agent
Blocked by: None — Wave B is done and Wave C starts here
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 7 of 10 in the architecture review (2026-08-18)
Grilled: `/grill-with-docs`, three rounds, 2026-09-04

**What to build:** two things that only make sense together. The store stops
spelling a rank out as five loose columns on three different types and publishes
one `Rank` value that carries its grading system. And the rank's rendering — the
belt graphic and the composed name — leaves `internal/web` for `internal/rankview`,
a module of four exported functions that each take that one value.

## The state this starts from

The same five display columns are spelled out on three types:

| type | fields | filled by |
| --- | --- | --- |
| `store.RosterRow` | `RankID`, `RankName`, `SystemName`, `SystemSlug`, `SystemOrder`, `Group`, `Degree` | `listRoster` |
| `store.PromotionRow` | `RankName`, `SystemName`, `SystemSlug`, `Group`, `Degree` | `ListPromotions` |
| `store.Rank` | `ID`, `Name`, `Group`, `Degree`; the system sits on the parent `GradingSystem` | `ListGradingSystems` |

Nothing outside `internal/web` reads them. `cmd/organizer/demo_test.go:32` reads
`RankName`, and the store's own tests assert on them; that is all.

Six template call sites assemble a rank from those scalars:

```
athletes.html:69         belt .Group .Degree (rosterRankLabel .)  |  rankLabel + systemLabel
athlete_detail.html:13   belt (decorative) + rankLabel + "(" systemLabel ", " PromotedOn ")"
athlete_detail.html:31   systemLabel .Slug .Name          (optgroup, system only)
athlete_detail.html:32   rankLabel .Group .Degree .Name   (option, rank only)
athlete_detail.html:62   belt (decorative) + rankLabel
athlete_detail.html:63   systemLabel                      (own column)
```

**The duplication this closes** is in `athletes.html:69`, which builds one
sentence twice. `rosterRankLabel` composes `Grau-Weiß, 2 Streifen (BJJ Kinder)` in
Go for the belt's `aria-label` and `title`; the `{{else}}` branch composes the same
sentence again in the template for the eye. The guard behind the brackets — only
name the system when there is a system name to give — is likewise written twice,
as `if system == ""` in `ranklabel.go:80` and as `{{with systemLabel …}}` in the
template. Change one spelling and the screen reader and the screen disagree.

Two of the six call sites are also the **same rendering**, not two surfaces:
`athlete_detail.html:13` and `:62` both draw a decorative belt followed by the
rank name. Everything that differs is what surrounds them.

## Decisions

Three grilling rounds, 2026-09-04. Every recommendation was taken.

- **The store publishes one `Rank` value, and the value carries its system.**
  `RosterRow` and `PromotionRow` replace their flat display fields with a single
  `Rank Rank`. `CONTEXT.md` defines a **Rank** as "a single named position *within
  a grading system*", so a Rank that does not know its system is incomplete by our
  own glossary — and the roster's bracketed system is the proof, because that label
  cannot be composed without it. `ListGradingSystems` fills `System` on each child
  rank too, so a `Rank` is self-describing wherever it turns up and every display
  function takes exactly one argument. That repeats, on each child, what the parent
  `GradingSystem` already says: duplication inside one read model, which is what
  `PromotionRow`'s own docstring already licenses ("denormalised by
  `ListPromotions` and have no meaning on write"). **Rejected:** two separate
  fields (`Rank` + `System`) on each carrier, which avoids the repeat and costs
  every display function a second parameter; and leaving the store flat, which
  would reduce this ticket to half its name.

- **`store.System{Name, Slug}`, embedded by `GradingSystem`.**

  ```go
  type System struct{ Name, Slug string }
  type GradingSystem struct { ID int64; System; Ranks []Rank }
  type Rank struct { ID int64; Name, Group string; Degree int; System System }
  ```

  One type for "the system's own two facts" — the slug that identifies it
  (ADR-0006) and the name that displays it — and `GradingSystem` stays the whole
  seeded set, related to it by embedding rather than by coincidence. The glossary
  keeps one term, **GradingSystem**, and `CONTEXT.md` is unchanged. Because `Name`
  is promoted through the embedded field, `cmd/organizer/demo.go`'s `rankIndex`
  (`s.Name`, `r.Name`) compiles untouched. **Rejected:** `SystemRef` (programmer
  vocabulary, not domain vocabulary); reusing `GradingSystem` on a rank with a
  permanently nil `Ranks`; flat `Rank.SystemName`/`Rank.SystemSlug`, which puts the
  shotgun argument list back.

- **`SystemOrder` stays a flat field on `RosterRow`.** It is a sort key the SQL
  produces, not something any view shows, so it has no business on a value that
  exists for display.

- **`store.RosterOption` is unchanged.** It is a filter chip, not a system, and
  its Ungraded cell is in no system at all. `filterLabel` (`web/roster.go:124`)
  therefore builds a `store.System{Slug: option.Value, Name: option.Name}` on its
  one non-ungraded path. That one awkward line is cheaper than changing what a chip
  is.

- **`store.Rank` gets `IsZero()`, and `RosterRow.Ungraded()` returns
  `r.Rank.IsZero()`.** `RosterRank` takes only a `Rank`, so it cannot ask the row
  whether the athlete is ungraded and needs a "no rank here" test of its own — one
  test with two homes, which is the duplication this ticket exists to remove. Two
  names, one definition: `IsZero` is a statement about a value (`time.Time` is the
  precedent), `Ungraded` is the domain word, and `CONTEXT.md` defines Ungraded as a
  property of an **athlete**, so it stays on the row and is defined in terms of the
  value. Ticket `05`'s docstring — why the rank id and not the slug decides —
  moves onto `IsZero` with it. **Rejected:** putting `Ungraded()` on `Rank`; a rank
  cannot be ungraded, a rank *is* a grading.

- **`internal/rankview` is a package, not two files in `internal/web`.** 288 lines
  of code with 419 lines of tests is a module, and it is the only place in the repo
  where BJJ convention lives. ADR-0004 says that knowledge is "quarantined in the
  view"; today that is a sentence in a document, and a package with a boundary test
  makes it something the suite fails on. The name echoes `store.RosterView`'s
  different sense of *view*, which is accepted: one is a package, the other a type.
  **Rejected:** `internal/display` (says nothing about ranks), `internal/beltview`
  (names the graphic and not the text it also owns).

- **Four exported functions, one per distinct rendering.** Not one function with a
  `Surface` parameter, which would force four return shapes into one type and add a
  value to switch on for nothing.

  ```
  rankview.RosterRank(locale, rank)    → {{rosterRank .Rank}}          template.HTML
  rankview.RankWithBelt(locale, rank)  → {{rankWithBelt .Rank}}        template.HTML
  rankview.RankName(locale, rank)      → {{rankName .Rank}}            string
  rankview.SystemName(locale, system)  → {{systemName .Rank.System}}   string
  ```

  `RosterRank` rather than `RosterCell`: on a phone the roster renders as cards,
  not table cells (ADR-0005), so *Cell* would name a shape that surface does not
  always have. `RankWithBelt` is named for what it renders rather than for a
  surface, because two call sites share it and naming it `DetailRank` or
  `HistoryRank` would make one of them read wrong. Four `FuncMap` keys replace four:
  `belt`, `rankLabel`, `systemLabel` and `rosterRankLabel` all go. `SystemName` is
  called from Go as well as from templates (`filterLabel`).

  One tension, to be named in both docstrings: `rankview.RankName` returns the
  *localized* name while `store.Rank.Name` holds the *stored English* one.
  `rankview.Name` would avoid the echo but greps badly.

- **A surface function returns the finished rank; the template keeps what is not
  the rank.** `RosterRank` owns its whole cell, including the `<span class="muted">`
  brackets and the "draw the belt if you can, otherwise write the words" rule, so
  that rule and the accessible label are composed once. The promotion date is not a
  rank fact, so `athlete_detail.html:13` keeps it:
  `{{rankWithBelt .Current.Rank}} <span class="muted">({{systemName
  .Current.Rank.System}}, {{.Current.PromotedOn}})</span>`. The table cell, the
  `<optgroup>` and where anything sits stay in the template.

  This moves `<span class="muted">(…)</span>` out of HTML and into Go, which is
  accepted deliberately: markup is already built in Go by `beltSVG`, so this widens
  a door ADR-0004 opened rather than opening a new one, and it is the only option
  that actually deletes the double sentence. **Rejected:** returning a struct of
  parts for the template to assemble, which removes the scalar arguments but leaves
  the rule stated per surface; and re-typing today's three helpers to take a `Rank`,
  which reduces the ticket to a rename.

- **Anything the module builds must escape what came from the database.** The
  stored English rank and system names are the fallback text, they reach the module
  as data, and they are now wrapped in Go-built markup. `beltSVG` already escapes
  its label; the new functions follow it. Everything else in the output is a
  constant or arithmetic on one.

- **The roster's accessible label stays a named function**, unexported:
  `rosterLabel(locale, rank) string`, called by `RosterRank`. Its tests — including
  the two guards ticket `05` added — move across nearly unchanged. **Rejected:**
  inlining it, which would leave those tests pulling an `aria-label` out of an SVG
  string, exactly the markup scraping candidate 2 exists to stop.

- **A third boundary test, `internal/rankview/literals_test.go`.** No `.go` file
  outside the package may contain a belt hex or a rank-catalog key prefix
  (`rank.colour.`, `rank.degree.`, `rank.split`, `system.`). Verified clean today:
  no hex literal exists in any Go file outside `belt.go`, and the key prefixes
  appear only in `ranklabel.go` and one explanatory comment at
  `internal/i18n/i18n.go:98`, which the allowed list must therefore account for.
  `app.css` is full of hexes and cannot trip it, because `archtest` walks `.go`
  files only. The name follows the two boundaries already there — `imports_test.go`
  for a claim about imports, `calls_test.go` for a claim about calls, so a claim
  about literals is `literals_test.go`. The failure mode is not hypothetical:
  ADR-0009 records that `TestBeltSVGCoversEverySeededColour` once kept its own copy
  of the colour list, which is why `store.SeededRankGroups()` was exported.
  **The cost, stated so it is not discovered later:** a string-literal check is
  coarser than an import or call check, so it is the first of the three that can
  produce a false positive. A Go file that legitimately mentions a hex would have to
  join the allowed list, and whoever adds it has to judge whether that is a real
  exception or the boundary being worked around.

- **Three test files in the new package.** `belt_internal_test.go` and
  `ranklabel_internal_test.go` keep the exhaustive matrices — every seeded colour
  across degrees 1–5 in both locales, the geometry, `splitRankGroup`, `resolveBelt`,
  `rosterLabel`. A new `rankview_test.go` in `package rankview_test` tests the four
  exported functions through the public API only, which is what proves the module is
  usable from outside by its exported names alone. The `_internal_test.go` suffix is
  this repo's newer convention (`internal/auth`, `store/roster_internal_test.go`);
  `belt_test.go` and `ranklabel_test.go` predate it and are renamed on the way.
  `internal/web/i18n_domain_test.go` stays where it is: it tests rendered pages.

- **ADR-0009 gets a dated `## Update`; ADR-0004 and `CONTEXT.md` get nothing.**
  The sentence that stops being true is in ADR-0009's last Decision bullet: "Their
  signatures mirror `belt` … three call sites carry a rank on three different types
  … and only the roster's label … takes the whole struct." The decision itself —
  composed in the view, from catalog pieces, never stored — is untouched, so this
  is an Update and not a new ADR. It is appended rather than corrected in place
  because the sentence was *true when written* and describes a deliberate choice;
  ADR-0010 was corrected in place because one word had been imprecise from the
  start. That is the same reasoning ADR-0009 gave for not rewriting ADR-0008's scope
  paragraph. ADR-0004 gets nothing: its decision was "view layer, not data-model
  metadata", `internal/rankview` *is* the view layer, and every consequence it lists
  stays literally true — writing a package name into it would put a pointer far from
  the code it points at. `CONTEXT.md` gets nothing: **Rank** already says its
  identity is separate from its localizable display name, and **Ungraded** and
  **GradingSystem** are unchanged. How a rank is *shown* is view vocabulary and does
  not belong in the glossary.

- **The third boundary is recorded in `docs/agents/analysis.md`**, in the list that
  already carries the other two. That file already anticipates it: "A third boundary
  therefore costs a predicate rather than another copy of the walk." Its entry names
  what the test forbids and that it is a claim about **literals**, as the other two
  name imports and calls.

- **Scope: candidates 2 and 9 are not touched.** This ticket changes the three
  store types, creates `internal/rankview`, rewrites the six template call sites and
  `filterLabel`. `rosterLine` keeps embedding `store.RosterRow`, so `.Rank` reaches
  the template through it unchanged; the handlers are unchanged; `rosterLines`,
  `rosterHeaders` and `rosterFilters` stay unreachable, which is candidate 2's
  problem and stays candidate 2's. Wave C is ordered 7 → 9 → 2 so that 2 finds a
  `Rank` value already in place, with `/code-review` after the wave. The cost:
  `roster_test.go`'s ~250 lines of HTML scraping stay as they are for two more
  tickets, and until 2 lands the new package's own tests are the only direct test of
  a rank's rendering.

## Acceptance

- [ ] `store.System{Name, Slug}` exists, `store.GradingSystem` embeds it, and
      `store.Rank` carries one. `ListGradingSystems` fills it on every child rank.
- [ ] `store.RosterRow` and `store.PromotionRow` each carry a single `Rank Rank`
      and no loose rank or system columns. `RosterRow.SystemOrder` remains.
- [ ] `store.Rank.IsZero()` exists, carrying ticket `05`'s reasoning for why the
      rank id decides, and `RosterRow.Ungraded()` is defined in terms of it. No site
      anywhere tests a zero rank by any other field.
- [ ] `internal/rankview` exists and exports exactly `RosterRank`, `RankWithBelt`,
      `RankName` and `SystemName`. `internal/web` contains no belt colour, no belt
      geometry and no rank-name composition.
- [ ] `internal/web/templates.go` binds four keys — `rosterRank`, `rankWithBelt`,
      `rankName`, `systemName` — and `belt`, `rankLabel`, `systemLabel` and
      `rosterRankLabel` are gone.
- [ ] All six template call sites render through one function each. No template
      contains a fallback branch between a belt and a name.
- [ ] Text that came from the database is escaped in every string the module
      builds. A rank whose stored name contains `<` renders it as text, pinned by a
      test.
- [ ] `rosterLabel` is a named unexported function and its existing tests — the
      Ungraded branch and the empty-system-name branch from ticket `05` — still test
      it directly rather than through rendered markup.
- [ ] `internal/rankview/literals_test.go` fails for a planted belt hex and for a
      planted catalog key prefix in a package outside it. Checked by planting both,
      as Wave A's boundary tests were.
- [ ] The package has `belt_internal_test.go`, `ranklabel_internal_test.go` and an
      external `rankview_test.go`, and every test that moved still asserts what it
      asserted before.
- [ ] `internal/web/i18n_domain_test.go` is unchanged in intent and still passes:
      the rendered pages say the same thing in both locales.
- [ ] ADR-0009 carries a dated `## Update` correcting its last Decision bullet, and
      nothing above that Update is rewritten.
- [ ] `docs/agents/analysis.md` lists the third boundary test beside the other two.
- [ ] ADR-0004 and `CONTEXT.md` are unchanged.
- [ ] `internal/web`'s handlers, `rosterLine`, `rosterLines`, `rosterHeaders` and
      `rosterFilters` are unchanged.
- [ ] `go vet`, `gofmt -l`, `go test ./...`, `go test -race ./...` and
      `staticcheck` are clean.
