# 01 — Sortable roster with current-rank column

Status: ready-for-agent

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

## Resolved design questions

Settled in a grilling + domain-modeling session (2026-07-23). Full detail in
[`../spec.md`](../spec.md); summary:

- **Cross-system rank ordering.** ✅ Sort by (`grading_systems.sort_order`,
  `ranks.sort_order`), ascending = beginners first — kids block then adult block,
  each in progression. The only domain-honest ordering; no invented global rank
  index. A cohort filter ([[roster-cohort-filter]]) is parked as the future
  answer to mixed-cohort scanning.
- **Ungraded athletes.** ✅ Always last, in both directions (`NULLS LAST`).
  Ungraded is *no* rank, distinct from White belt (which is a graduation).
- **Data access.** ✅ New `store.ListRoster(sort, descending)`, replacing
  `ListAthletes` in the roster. Current rank selected via a **window function**
  (`ROW_NUMBER() OVER (PARTITION BY athlete_id ORDER BY promoted_on DESC, id
  DESC) = 1`) — standard SQL, portable to Postgres (ADR-0002), chosen over a
  correlated `NOT EXISTS`. A pinning test asserts it matches the pure
  `CurrentRank` on a same-date tie.

## Further decisions (see spec)

- Default view: **Vorname ascending** (was last name). Tie-break Vorname →
  Nachname, always A→Z; only the primary axis reverses.
- All five data columns sortable; first click ascending; ▲/▼ on the active
  column only; unknown params fall back silently via a column whitelist.
- Current-rank display: **rank name + muted system**; ungraded blank. Belt
  graphic parked ([[rank-belt-visual]]).

## Acceptance

- Clicking a column header sorts by it; clicking again reverses direction.
- The roster shows each athlete's current rank; ungraded athletes render blank.
- Sorting by the current-rank column orders athletes by graduation with a
  documented, agreed cross-system ordering.
