# Localized rank and system names are composed in the view, not stored

Status: accepted — supersedes the scope paragraph of ADR-0008 in part

## Context

ADR-0008 made the UI bilingual and drew its scope at UI chrome, leaving rank and
grading-system names to render as their raw English seed strings. It gave a
reason: localizing domain data would be a model change. That reason no longer
holds, and three developments since are why.

**ADR-0006** moved a grading system's identity onto its `slug`, leaving `name` as
a pure display label. A translatable name is no longer a name that anything
depends on.

**The belt visual** (ADR-0004, shipped in `5a182c1`) established that BJJ display
conventions live in the view layer, and built the parse this needs:
`resolveBelt` already splits a `rank_group` into a body colour and an optional
white/black bar.

**`seed.rankName` is a pure function** of `(rank_group, degree)` — `White`,
`White, 1 stripe`, `White, 2 stripes` — so a rank's English name is fully
reconstructible from two columns the rank already carries. And **only the seed
writes ranks and grading systems**: no HTTP route creates either, so there are no
hand-authored names a reconstruction could overwrite.

Together these mean a localized rank name needs no per-rank translation columns
and no schema change at all.

## Decision

**Localized rank and system names are composed in the view from catalog building
blocks.** Nothing localized is ever stored.

- The catalog holds **nine colour words**, **five degree patterns** for degrees
  1–5 (`%s, 2 Streifen`), and **one split pattern** (`%s-%s`) — fifteen keys per
  locale. Degree 0 is the colour word alone. `Grey-White` at degree 2 composes to
  `Grau-Weiß, 2 Streifen`.
- Degrees are **enumerated rather than interpolated with `%d`**, so pluralization
  lives entirely in the catalog. The enumeration stops at 5, the same bound as
  `beltMaxStripes`, because `seed.go` omits the rare 5th BJJ stripe but expects
  clubs to add it.
- **Grading-system names are catalog keys on the slug** (`system.bjj-kids`),
  which ADR-0006 made the stable identity.
- **This applies to English too.** `TestCatalogsHaveIdenticalKeys` forces both
  catalogs to carry the same keys, so the English label is composed from
  `en.json` as well. The stored name is consequently never displayed.
- **One fallback rule everywhere:** a catalog miss renders the stored English
  name — the same rule ADR-0004 already set for an unresolvable colour. This
  needs `i18n.Lookup(locale, key) (string, bool)`, because `i18n.T` returns the
  key itself on a miss and `rank.degree.7` as an athlete's rank would be worse
  than `White, 7 stripes`.
- Composition happens in **template functions whose logic lives in Go**.
  `templateFuncs(locale)` already closes over the locale, so nothing new is
  threaded through handlers or store types. Their signatures mirror `belt`, which
  takes the colour group and the degree as plain arguments: three call sites carry
  a rank on three different types (a roster row, a promotion, a rank), and only the
  roster's label — which has a conditional, the parenthesised system an ungraded
  athlete does not get — takes the whole struct.

The line this draws: **hex values stay in Go, human-readable text goes to the
catalog.** `beltColours` maps a colour name to a fill, which is not language and
belongs in the view code. The colour *words* and the system names are text, and
text lives where this project keeps text.

## Considered Options

- **Per-rank translation columns or a translation table (rejected):** what
  ADR-0008 assumed would be necessary. Unnecessary once the name is derivable,
  and it would put a second, hand-maintained truth beside `seed.rankName`.
- **One catalog key per finished rank name (rejected):** ~73 distinct names, so
  ~146 hand-maintained entries, and raising a system's `maxDegree` by one would
  add thirteen more. It also rebuilds `seed.rankName`'s composition in JSON where
  the two can drift silently.
- **A hardcoded Go table for system names, beside `beltColours` (rejected):** it
  would make system names the sole text not in the catalog, forgo
  `TestCatalogsHaveIdenticalKeys`, and turn "adding a locale is one catalog file"
  (ADR-0008) into two places.
- **Leaving system names untranslated (rejected):** the argument for it was that
  the name was load-bearing, which ADR-0006 ended.
- **Amending ADR-0008 in place, or superseding it wholly (rejected):** its core
  decision — locale as an account column, resolved per request, one template set
  per locale — is entirely intact. Only its scope boundary moved, and rewriting
  the paragraph would erase the fact that domain data was deliberately parked
  first and why the judgment reversed.

## Consequences

*The first two have since been overtaken — see the Update below.*

- **The stored English rank name must never be localized.** `ensureRank` looks a
  rank up by `(grading_system_id, name)`, so the name is the seed's idempotency
  key. A unique index on that pair (migration `00002`) stops two rows sharing a
  name, but it does not stop the failure that matters: respelling `rankName`
  makes the lookup miss, the insert succeeds under the *new* name, and the system
  ends up with two rows for one rank while existing promotions still point at the
  old one. Moving that lookup onto the natural key
  `(grading_system_id, rank_group, degree)` is filed separately
  ([[rank-seed-identity]]) and deliberately not done here, so this stays a
  view-layer change.
- **After this, the stored name has two jobs left:** the seed's lookup key, and
  the fallback for a rank whose colour the view cannot resolve. It is not a
  display string.
- **The derivation rests on two facts that could change.** If `seed.rankName`
  stops being a pure function of group and degree, or if anything other than the
  seed starts writing ranks, this decision needs revisiting rather than patching.
- A missing colour word degrades a real rank to its English name silently, so a
  guard test covers every seeded group across degrees 1–5 in both locales.
  `store.SeededRankGroups()` is exported for it, and
  `TestBeltSVGCoversEverySeededColour` derives from the same source instead of
  keeping its own copy of the list.
- Composition is fixed to "colour phrase, then degree phrase". German and
  English both fit; a language that inflects the colour or leads with the numeral
  would not, and would need a different seam.

## Update, 2026-08-10

The decision stands untouched; two of its consequences do not. They are left
above as written, the same way this ADR left ADR-0008's scope paragraph standing,
and corrected here.

- **The seed no longer keys on the name.** `ensureRank` looks a rank up by
  `(grading_system_id, rank_group, degree)` and corrects a stored name it
  disagrees with ([[rank-seed-identity]] 01). The duplicate-row failure described
  above is gone; the reasoning behind the key that replaced it lives on
  `ensureRank`.
- **"Two jobs" was wrong when written, and still counts two.** The seed's lookup
  was one of them and has dropped away, but `cmd/organizer/demo.go` addresses its
  target ranks by name through `rankIndex` and was missed at the time. So the
  stored name is still the fallback for a rank whose colour the view cannot
  resolve, and still a lookup key — just for the demo fixture rather than the
  seed. The decision's own conclusion is unaffected: it must stay English.
