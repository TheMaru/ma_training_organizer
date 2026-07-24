# Spec — Sortable roster with current-rank column

Status: ready-for-agent

Make the athlete roster (`/athletes`) sortable by any data column, and add the
derived **current rank** as one of those columns. Replaces the current
single-column (last-name only) sort with one generic mechanism.

Outcome of a grilling + domain-modeling session (2026-07-23). Every decision
below is settled; the open triage questions on the originating issue are
resolved here.

## Mechanism & UI

- **Server-side** sorting via `?sort=<col>&dir=<asc|desc>`, consistent with the
  existing `?dir` pattern and the server-rendered/no-build ethos (ADR-0002).
  (Rejected: client-side JS table sort — inconsistent with the rest of the app,
  breaks progressive enhancement, and the cross-system rank key must come from
  the server anyway.)
- **All five data columns are sortable:** Nachname, Vorname, Geburtsdatum,
  Eintritt, Aktueller Rang. The actions column is not sortable.
- **First click on a column = ascending**; clicking the already-active column
  toggles the direction. A direction indicator (▲ asc / ▼ desc) appears **only on
  the active column**. (Rejected: per-column "smart" default directions — kept
  uniform for predictability; cheap to revisit later.)
- **Default view: Vorname ascending** (last name as the secondary key). Changed
  from the previous last-name default because the kids trainer recalls first
  names more readily.
- **Invalid / unknown params fall back silently to the default** (unknown `sort`
  → Vorname; invalid `dir` → asc). No error, no 400 — sorting is a view concern,
  not a data error. Allowed columns are mapped through a fixed **whitelist**, so
  the interpolated `ORDER BY` stays injection-safe (as `dir` already is today).

## "Aktueller Rang" column

- **Display: rank name + muted grading system**, e.g. `Weißgurt` *(BJJ Kids)*.
  The system disambiguates same-named ranks across cohorts (White exists in both
  BJJ Kids and BJJ Adult). Ungraded athletes render blank (or a muted "—").
  The promotion date stays on the detail page; it is noise in the scan view.
- A **visual belt graphic** was considered and **parked** as its own ticket
  ([[rank-belt-visual]]): it must stay data-driven and not re-hardcode BJJ
  conventions (ADR-0001), so the text name remains the accessible baseline and
  the sort target regardless.

## Cross-system rank ordering

- Sort key for the rank column = **(`grading_systems.sort_order`,
  `ranks.sort_order`)**. Ascending = beginners first. Because the kids system
  sorts before the adult system, the roster groups into a kids block then an
  adult block, each internally in progression order.
- This is the only domain-honest ordering: those are the sole defined ordering
  axes, and there is no meaningful cross-system "how advanced" comparison
  (a kids black belt is not an adult black belt). Accepted consequence: an adult
  beginner sorts after an advanced kid. A cohort filter ([[roster-cohort-filter]])
  is the future answer to that, not an invented global rank index.
- **Ungraded athletes always sort last**, in both directions (`NULLS LAST`).
  "Ungraded" is *no* rank (no promotion at all) — distinct from White belt,
  which *is* a graduation. So it is not a lowest rank; it sits after everyone.

## Sort rules (all columns)

- Tie-break: **Vorname, then Nachname, always ascending (A→Z)** — only the
  primary axis reverses with `dir`. Within "all blue belts", alphabetical A→Z is
  more natural than Z→A regardless of the primary direction. (Exception is
  automatic: when Vorname/Nachname is itself the primary column, it reverses as
  the primary axis.)

## Data access

- New `store.ListRoster(sort string, descending bool) ([]RosterRow, error)`,
  replacing the roster's use of `ListAthletes`. Returns each athlete joined with
  their derived current rank (rank name, system name, and the two sort keys).
- Current rank per athlete is selected in SQL with a **window function**:
  `ROW_NUMBER() OVER (PARTITION BY athlete_id ORDER BY promoted_on DESC, id DESC)`
  filtered to `= 1`. This encodes exactly `CurrentRank`'s rule (most recent by
  date, higher id wins a tie). Window functions are standard SQL supported by
  both SQLite (≥3.25) and Postgres — not SQLite-specific, so ADR-0002's
  portable-SQL constraint holds. (Rejected: correlated `NOT EXISTS` — same result
  but less readable; rejected: deriving/sorting in Go — would keep SQL simplest
  but was passed over in favour of keeping the roster on SQL `ORDER BY`.)
- Ranks/grading systems joined via LEFT JOIN so ungraded athletes survive with
  NULL rank keys; `ORDER BY <whitelisted col> <dir>` with `NULLS LAST` and the
  fixed `first_name, last_name` tie-break.

## Single-source-of-truth safeguard (no ADR)

The tie-break "current rank = latest promotion, higher id on a tie" now exists in
two encodings: the pure `CurrentRank` (used by the detail page) and the SQL
window function (used by the roster). A **pinning test** guards against drift: over
fixtures that include a **same-date tie**, assert that the current rank returned by
`ListRoster` equals the one `CurrentRank` derives for the same athlete. As long as
it is green, the two paths cannot diverge. No ADR is warranted — SQL `ORDER BY` is
the expected approach here, not a surprising trade-off.

## Acceptance

- Clicking any of the five data-column headers sorts by it; clicking the active
  column reverses direction; the active column shows a ▲/▼ indicator.
- The roster opens sorted by Vorname ascending by default.
- Each athlete's current rank shows as rank name + muted system; ungraded
  athletes render blank.
- Sorting by "Aktueller Rang" orders by (system sort_order, rank sort_order)
  ascending with ungraded athletes always last, in both directions.
- All sorts tie-break on Vorname then Nachname (A→Z), except where the name is
  the primary column.
- Unknown `sort`/`dir` values render the default view without error.
- A pinning test asserts `ListRoster`'s current rank matches `CurrentRank` on a
  same-date tie fixture.
