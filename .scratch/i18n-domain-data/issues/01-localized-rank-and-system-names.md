# 01 — Localized rank and grading-system names

Status: needs-triage

Localize the **domain data** shown to trainers — rank/belt names (`White`,
`White, 1 stripe`) and grading-system names (`BJJ Kids`) — so a German UI shows
`Weiß` / `Weiß, 1 Streifen` / `BJJ Kinder` rather than the raw English seed
strings.

Parked deliberately out of the UI-i18n feature ([[i18n-ui]]), which covers only
UI chrome + Go messages. Accepted interim state: UI is translated, but rank and
system names render as their raw (English) seed data.

## Why it is its own (bigger) concern

- Rank names are **data-driven** and are what a promotion targets (ADR-0001).
  Localizing them means per-locale translations in the model — `name_de`/`name_en`
  columns or a translation table on ranks and grading systems — a real model
  change, disproportionate to bundle into UI i18n.

## Likely subsumed by the belt visual

The belt graphic ([[rank-belt-visual]]) already plans to render a rank from
`rank_group` (colour) + `degree` (stripes) via a colour-name→something map. If
that map is **locale-aware** (colour → localized label), it produces a localized
rank *label* for free — so rank-name i18n may fall out of the belt-visual work
rather than needing per-rank translation columns. System names (`BJJ Kids`) are
not covered by that and would still need their own translation.

## Open questions (triage)

- Per-rank translation columns/table vs. a locale-aware colour+degree renderer
  (coordinate with [[rank-belt-visual]]).
- How to localize system names (`BJJ Kids` → `BJJ Kinder`) — translation column
  on `grading_systems`, or leave proper-noun-ish system names untranslated?
- Fallback when a translation is missing for the active locale.

## Comments

2026-07-30 — Triage of [[roster-cohort-filter]] 01 touched this ticket twice, and
both are good news for it.

**The system-name question got easier.** That triage produced
[ADR-0006](../../../docs/adr/0006-grading-system-identity-is-a-slug.md): a
`grading_systems.slug` (`bjj-kids`, `bjj-adult`) becomes a system's stable
identity, and `name` becomes *purely a display label*. The reason it did was this
ticket — a translatable name cannot also be a URL key. The consequence is that
the open question above shrinks: with identity moved off the name, adding a
localized name beside `name` is a display-layer change rather than a change to
what a system *is*. That also weakens the "leave them untranslated" option, since
the argument for it was largely that the name was load-bearing.

**The call-site inventory is now five, not four.** For whichever mechanism wins,
the places a raw system name reaches the German UI are:

1. `athletes.html` — the rank cell's accessible label / muted fallback text
2. `athlete_detail.html` — the current rank line
3. `athlete_detail.html` — the system grouping in the promotion form
4. `athlete_detail.html` — the "System" column of the promotion history table
5. `athletes.html` — the roster filter chips (new, [[roster-cohort-filter]] 01)

The chips render whatever `SystemName` yields, so they need no special handling —
they are listed so the inventory is complete, not because they add work. That
ticket deliberately ships with raw seed names: translating only the chips would
put `BJJ Kinder` next to `BJJ Kids` in one view, which is worse than uniform
English. Its build order is ahead of this one (it is fully specified; this is
still `needs-triage` with a question waiting on [[rank-belt-visual]]), and nothing
is duplicated either way.
