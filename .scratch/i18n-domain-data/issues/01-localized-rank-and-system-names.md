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
