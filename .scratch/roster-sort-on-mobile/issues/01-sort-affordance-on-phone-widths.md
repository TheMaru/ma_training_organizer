# 01 — Roster sorting has no affordance on phone widths

Status: needs-triage

The roster's sortable column headers are unreachable on a phone. Found by the
code review of [[roster-sortable-columns]] (shipped in `c8ff458`, 2026-07-27).

## Motivation

`app.css`'s `@media (max-width: 600px)` block turns every table into a stack of
labelled cards and moves `thead` off-screen (`position: absolute; left: -9999px`)
so it stays in the accessibility tree without showing. The sort links live in
those `<th>`s, so on a phone there is visually nothing to click: the roster is
permanently in the default Vorname-ascending order.

Not a regression — the previous single Nachname toggle sat in the same hidden
`thead` — but the feature whose whole point is "click any column to sort" now has
no mobile surface. The kids trainer is exactly the phone user.

## Open questions (triage / grilling material)

- **What is the mobile control?** A `<select>` + direction toggle above the table
  ("Sortieren nach …"), reusing the same `?sort=&dir=` URLs? A horizontally
  scrollable sort chip row? Or keep `thead` visible as a compact sort bar in the
  card layout? Progressive enhancement and the no-build ethos (ADR-0002) rule out
  a JS-only widget.
- **One control or two?** A `<select>` that also renders on desktop would replace
  the header links entirely (one mechanism, less markup) — or it stays a
  phone-only addition and the desktop keeps clickable headers.
- **Does the same gap apply to the detail page's Verlauf table?** That one is not
  sortable today, so probably not.

## Acceptance (provisional, pending triage)

- At ≤600px a trainer can change both the sort column and the direction, and the
  currently active sort is visible.
- Server-rendered, no client-side sorting; the existing `?sort=&dir=` URLs stay
  the single mechanism.
