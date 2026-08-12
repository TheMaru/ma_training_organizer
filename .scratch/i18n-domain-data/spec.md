# Spec — Localized rank and grading-system names

Status: done

Localize the domain data a trainer reads — rank names and grading-system names —
so a German UI says `Weiß, 2 Streifen` and `BJJ Kinder` where it currently says
`White, 2 stripes` and `BJJ Kids`. Composed in the view from catalog building
blocks; nothing localized is stored and the schema does not change.

Outcome of a triage + grilling + domain-modeling session (2026-08-03), following
[[rank-belt-visual]] shipping. Every decision below is settled; the open
questions on the originating issue are resolved here. The decision itself is
recorded in **ADR-0009**.

## Problem Statement

A trainer who set the UI to German gets a German interface with English data in
it. The nav, the buttons, the column headers and the validation messages are all
translated, but the things the page is actually *about* are not: an athlete's
rank reads `Grey-White, 2 stripes` and their cohort reads `BJJ Kids`. On the
roster that English text is what a screen reader announces for the belt graphic,
since the graphic carries no text of its own. In the promotion form it is every
one of the ~77 options a trainer scrolls through to record a promotion.

The result is a UI that looks half-finished in exactly the places a trainer looks
most, and it is worse for the trainers the bilingual UI exists for: someone who
chose German did so because English is not comfortable, and the rank names are
the vocabulary they need most often.

This was parked deliberately when the UI was made bilingual, on the belief that
localizing rank names meant per-rank translation columns — a model change out of
proportion to UI work. That belief is now known to be wrong.

## Solution

Rank and grading-system names are **composed in the view** from small
translatable pieces, and the composed label replaces the raw English string
everywhere it is shown.

A rank's name is derivable: the rank already carries a colour group and a stripe
degree, and the seed builds its English name from exactly those two. So the
catalog holds colour words, stripe phrases and a split-belt pattern, and the view
assembles `Grey-White` + degree 2 into `Grau-Weiß, 2 Streifen` or
`Grey-White, 2 stripes` depending on the trainer's language.

A grading system's name is a plain display label whose identity moved to its slug
(ADR-0006), so it is one catalog key per slug.

Where the view cannot resolve something — a colour it has no word for, a system
with no key — it renders the stored English name, exactly as the belt graphic
already falls back to text for an unknown colour (ADR-0004). The graphic and the
label degrade together and by the same rule.

Nothing about the data changes: no migration, no new column, no rewritten rows.

## User Stories

1. As a trainer with a German UI, I want an athlete's current rank to read `Weiß, 2 Streifen`, so that the page speaks one language throughout.
2. As a trainer with an English UI, I want rank names in English, so that the language switch is symmetric rather than "German plus raw data".
3. As a trainer, I want the grading-system name localized to `BJJ Kinder`, so that a cohort label does not sit in English beside translated chrome.
4. As a trainer on the roster, I want the belt graphic's accessible label localized, so that a screen reader announces the rank in my language.
5. As a trainer using a screen reader, I want the localized rank and its localized system announced together, so that I can tell a Kids white belt from an Adult white belt.
6. As a trainer, I want the roster's filter chips to carry localized system names, so that the filter matches the rows it filters.
7. As a trainer recording a promotion, I want the rank dropdown localized, so that I pick a rank by the name I actually use.
8. As a trainer recording a promotion, I want the dropdown's system groups localized, so that the grouping reads as the cohorts I know.
9. As a trainer reading an athlete's promotion history, I want both the rank column and the system column localized, so that the history matches the current-rank line above it.
10. As a trainer, I want a split belt named naturally in my language, so that `Grey-White` reads as `Grau-Weiß` rather than as a half-translated hybrid.
11. As a trainer, I want stripe counts phrased naturally, so that one stripe and several stripes each read correctly in my language.
12. As a trainer, I want a plain belt to read as just its colour, so that a white belt is `Weiß` and not `Weiß, 0 Streifen`.
13. As a trainer, I want an ungraded athlete to keep showing the existing "no rank" wording, so that this change does not disturb a case that was already correct.
14. As a trainer, I want the roster's rank sort to be unchanged, so that ranks still sort in progression order and not by the alphabet of whichever language I chose.
15. As a trainer switching language mid-session, I want rank and system names to switch along with everything else, so that no part of the page lags behind the switcher.
16. As a trainer on a phone, I want the card labels to stay consistent with the desktop headers, so that the responsive layout does not diverge from the table.
17. As a trainer whose club added a rank in a colour the app has no word for, I want it to show its stored name, so that I see a truthful rank rather than a blank or a broken key.
18. As a trainer whose club added a grading system with no translation, I want it to show its stored name, so that an untranslated system is usable rather than invisible.
19. As a trainer whose club uses a fifth stripe, I want that rank to be localized too, so that a rank the graphic can already draw is not the one rank left in English.
20. As a trainer, I want a rank with more stripes than the app anticipates to still show a truthful name, so that an extreme value degrades rather than misleads.
21. As a trainer, I want this change to require no migration and touch no stored data, so that upgrading carries no data risk.
22. As a maintainer, I want a forgotten colour word to fail a test, so that a rank cannot silently degrade to English in production.
23. As a maintainer, I want a translation forgotten in one catalog to fail the existing parity test, so that the two languages cannot drift.
24. As a maintainer, I want adding a further locale to remain one catalog file, so that the property ADR-0008 promised still holds.
25. As a maintainer adding a belt colour to the seed, I want a single place that tells the tests which colours exist, so that I cannot update one copy of the list and miss another.
26. As a maintainer, I want the stored English rank name left untouched, so that seeding stays idempotent.
27. As a maintainer, I want the reason the stored name must stay English written down, so that a future contributor does not "finish the job" by translating the column.

## Implementation Decisions

### What gets translated, and how it is composed

- A rank's localized name is composed from its **colour group** and its
  **degree**, never looked up as a whole string.
- The catalog gains **fifteen keys per locale**: nine colour words, five degree
  patterns for degrees 1–5, and one split pattern.
  - Colour words: White, Grey, Yellow, Orange, Green, Blue, Purple, Brown, Black
    — the colours the seed uses. The same nine serve both the body colour and a
    split belt's bar, since a bar is only ever White or Black.
  - Degree patterns take the composed colour phrase and produce the full name,
    e.g. German `%s, 2 Streifen`, English `%s, 2 stripes`.
  - The split pattern joins body and bar, e.g. `%s-%s`. It is a **pattern, not a
    separator**, so a language may reorder the parts, and the joining character
    is a translator's choice rather than a hardcoded hyphen. The `-` that the
    *parser* splits on is unrelated: that belongs to the stored data's format.
- **Degree 0 has no key.** The colour word alone is the whole name, mirroring how
  the seed builds English names.
- **Degrees are enumerated, not interpolated with a number verb.** Pluralization
  therefore lives entirely in the catalog rather than in a branch in Go. The
  enumeration runs 1–5, the same bound the belt graphic already uses for how many
  stripes fit on the friso.
- **Grading-system names are one catalog key per slug**, e.g. `system.bjj-kids`.
  ADR-0006 made the slug the stable identity precisely so the name could be a
  display label.
- **English is composed from the catalog too.** The catalog parity test forces
  both locales to carry the same keys, so the English label comes from the
  English catalog. Consequently the stored name is never displayed on a resolved
  rank.

### Fallback

- **One rule everywhere: a catalog miss renders the stored English name.** This
  is the rule ADR-0004 already set for a colour the belt renderer cannot resolve,
  now extended to the label. Graphic and label degrade together.
- The cases it covers: a colour with no word, a degree outside 1–5, an empty
  colour group (a rank predating the descriptive columns), and a system with no
  key.
- This requires a new lookup in the i18n package that **reports whether it hit**,
  alongside the existing translate call. The existing call returns the key itself
  on a miss, which is right for UI chrome — a visible gap — but wrong here, where
  a missing key would render as an athlete's rank.

### Where composition happens

- Composition lives in **template functions that close over the request's
  locale**, added to the existing per-locale function map. The renderer already
  parses one template set per locale, so the locale needs no threading through
  handlers or store types.
- Their **signatures mirror the existing belt function**, which takes the colour
  group, the degree and a label as plain arguments. Three functions:
  - a rank function taking group, degree and the stored name as fallback;
  - a system function taking the slug and the stored name as fallback;
  - a roster-label function taking the roster line, which composes rank and
    system into the belt graphic's accessible label.
- The roster-label function is the one that takes a struct rather than scalars,
  because it carries a **conditional**: an ungraded athlete gets no parenthesised
  system. That conditional is Go's job, not the template's. It replaces the
  method that does this today, which cannot reach the locale.
- The **logic lives in Go**, in the web package, beside the belt renderer.

### Data plumbing

Two display-only fields, denormalised exactly as the belt work denormalised the
colour group and degree:

- **`GradingSystem` gains its slug**, so the promotion form's system grouping can
  be keyed. Its query already joins the systems table.
- **`Promotion` gains its system slug**, so the history table's system column can
  be keyed. Its query already joins the systems table too.
- The roster row already carries the system slug, and the roster's filter options
  already carry it as their value — the chips need no new data.
- The rank type used by the promotion form already carries colour group and
  degree.
- **No schema change, no migration, no new table.**

### Surfaces

Nine places, all of which must show localized text:

Rank names — the roster's rank cell (both the graphic's accessible label and the
text shown when a colour does not resolve); the athlete detail page's current
rank; every option in the promotion form's rank dropdown; the rank column of the
promotion history table.

System names — the roster's accessible label and text fallback; the athlete
detail page's current-rank line; the promotion form's option groups; the system
column of the promotion history table; the roster's filter chips.

### Seed and shared test data

- **The stored English rank name is not touched.** It remains the seed's
  idempotency key. ADR-0009 records why it must never be localized, and moving
  that lookup onto a stable key is filed separately.
- The seeded colour groups are **exported from the store package** as the single
  source for tests. The existing belt coverage test currently keeps its own
  hand-copied list of those colours; it moves onto the exported source, so this
  change removes a duplicate rather than adding a third copy.

## Testing Decisions

A good test here asserts what a trainer sees, not how it was assembled. It should
survive renaming a helper, changing a key's spelling, or moving the composition
between functions; it should fail if a German trainer sees an English rank.
Assertions go against rendered output and public behaviour, never against the
composition's internals.

Three seams, **all of them already in use** in this codebase.

### Seam 1 — the rendered page (primary)

The external test package for the web package, driving a real server over HTTP,
logging in, switching language, and asserting on returned HTML. Prior art: the
existing i18n handler tests, which already assert that pages render in the chosen
language and that the switcher persists on the account. Existing helpers cover
server setup, login, the language switch and body reading.

This seam carries the **behaviour** for all nine surfaces, in both languages:

- The roster's rank cell, its accessible label, and the filter chips.
- The athlete detail page's current rank, promotion form options and option
  groups, and the promotion history table's rank and system columns.
- An ungraded athlete still showing the existing "no rank" wording.
- A language switch mid-session changing rank and system names along with the
  rest of the page.

**The shared roster fixture must be extended.** It currently promotes only to
plain white and plain blue belts, both at degree 0, so at this seam no degree
pattern and no split belt would ever be exercised. Add a striped rank and a
split-belt rank so the interesting compositions are covered by a test that goes
through the real templates.

### Seam 2 — the composition functions (in-package)

The web package's single in-package test file, which already unit-tests the belt
renderer and its coverage of every seeded colour. This seam exists **only for
exhaustiveness and for the fallback cases**, not for behaviour already covered
above: driving the full matrix through HTTP would mean creating one athlete per
rank.

- **The coverage guard:** every seeded colour group, across degrees 1–5, in both
  locales, must compose rather than fall back. This is the analogue of the
  existing belt coverage test and catches a forgotten colour word.
- **The fallback cases:** an unmapped colour, a degree beyond the enumeration, an
  empty colour group, and a system with no catalog key each render the stored
  name.
- **The composition rules:** degree 0 yields the colour word alone; a split belt
  composes body and bar; the roster label appends the system for a graded athlete
  and omits it for an ungraded one.

Both this guard and the existing belt coverage test derive their colour list from
the newly exported seeded groups, so a colour added to the seed cannot be missed
by either.

The functions stay unexported; this test file is inside the package.

### Seam 3 — the i18n package

The i18n package's own tests, including the internal test file that already
exercises the fallback chain against a **synthetic catalog pair**. One test: the
new lookup reports a miss where the existing translate call would return the key
itself.

This needs the synthetic pair because the real catalogs cannot express the case —
the parity test forbids a key present in one catalog and absent from the other.
That is exactly why the synthetic pair exists already.

### Unchanged guards that must keep passing

- Catalog key parity across locales — it now also guards the fifteen new keys.
- Non-empty catalog values.
- Every page rendering in English.
- Roster column labels staying in sync with the phone card labels.
- The belt renderer's own tests: the graphic is untouched by this work.

## Out of Scope

- **Moving the seed's rank lookup off the name.** A latent duplicate-rank bug
  found during this grilling, filed as [[rank-seed-identity]] 01. It predates
  i18n entirely and can duplicate reference data that promotions hang off, so it
  gets its own tests and its own review rather than riding along with a display
  change. ADR-0009 records the constraint in the meantime.
- **Giving ranks a slug** for symmetry with ADR-0006. Considered in that ticket,
  not here.
- **A UNIQUE constraint on ranks.** Same ticket.
- **Any schema change, migration or data rewrite.**
- **Locales beyond German and English.** The design keeps adding one to a single
  catalog file, but no third locale is added here.
- **CLI output and internal error messages**, which this does not revisit. What
  the CLI speaks is settled by [[cli-language]] 01 and recorded in ADR-0008's
  update of 2026-08-12: the operator-facing subcommands are English, the
  trainer-facing `import-athletes` report is German outside the catalogs.
- **Free-text domain data** — athlete notes, names — which is trainer-entered
  content, not reference data, and is not translatable in principle.
- **Runtime-editable translations.** Systems and ranks are seed-only; adding
  either is already a code change plus a deploy, which is the same act as adding
  a catalog key.
- **Changing sort behaviour.** Ranks sort by their stored progression order, not
  by name, and localization does not touch that.
- **The belt graphic itself**, which is complete and unchanged.

## Further Notes

- **ADR-0009** records the decision, the four rejected alternatives, and two
  facts the derivation rests on: the seed's rank-name builder is a pure function
  of colour group and degree, and only the seed writes ranks. If either stops
  being true, the decision needs revisiting rather than patching.
- **ADR-0008** keeps its core decision — locale as an account column, resolved
  per request, one template set per locale — and gains a pointer noting that its
  scope paragraph is superseded in part.
- **`CONTEXT.md`** now says a rank's identity is separate from its name, which is
  a display label and may be localized — the same wording grading systems already
  carried. No new glossary term was added for the composed label: like belt,
  friso and stripe before it, it is a display detail rather than a domain
  concept.
- **Composition order is fixed** to colour phrase then degree phrase. German and
  English both fit. A language that inflects the colour word or leads with the
  numeral would not, and would need a different seam — worth knowing before a
  third locale is promised to anyone.
- **Degree 5 is exercised by unit test only.** No seeded rank uses it, because
  the seed deliberately leaves the rare fifth stripe to the clubs that need it,
  but the graphic already draws it and the catalog now names it.
- **The demo seed needs no change.** Its twelve curated athletes already cover
  every colour, both bar variants and degrees 0–4, which is the coverage a manual
  pass over the localized labels wants.
