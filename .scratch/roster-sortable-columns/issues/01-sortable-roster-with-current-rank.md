# 01 — Sortable roster with current-rank column

Status: needs-triage

Introduce sortable columns on the athlete roster (`/athletes`), and add the
derived **current rank** (graduation) as one of the sortable columns. Deferred
out of the issue-06 follow-up so sorting is solved once, generically, rather
than bolted on per column.

## Motivation

The roster currently sorts only by last name (asc/desc toggle on that one
header). Trainers want to scan athletes by their current graduation, and more
generally to sort by any column. Doing this ad-hoc per column would duplicate
the toggle logic; a single generic mechanism is cleaner.

## Scope

- A generic per-column sort mechanism for the roster table: each sortable
  header links to `?sort=<col>&dir=<asc|desc>` and toggles direction when it is
  already the active column; the active column shows a direction indicator.
- Add a **current rank** column showing the derived current rank (most recent
  promotion by date, cross-system — ADR-0001), empty for athletes with no
  promotions.
- Make the current-rank column one of the sortable columns.

## Open design questions (triage)

- **Cross-system rank ordering.** Ranks carry a per-system `sort_order`, but
  order across systems is not defined. Proposal: sort by (grading system
  `sort_order`, rank `sort_order`) so kids ranks group before adult ones in
  progression order. Confirm this is the desired semantics.
- **Ungraded athletes.** Where do athletes with no promotion sort? Proposal:
  always last, regardless of direction.
- **Data access.** Likely a new `store.ListRoster(sort, descending)` returning
  athletes joined with their derived current rank (one correlated-subquery
  LEFT JOIN, portable SQL per ADR-0002), replacing the roster's current use of
  `ListAthletes`.

## Acceptance

- Clicking a column header sorts by it; clicking again reverses direction.
- The roster shows each athlete's current rank; ungraded athletes render blank.
- Sorting by the current-rank column orders athletes by graduation with a
  documented, agreed cross-system ordering.
