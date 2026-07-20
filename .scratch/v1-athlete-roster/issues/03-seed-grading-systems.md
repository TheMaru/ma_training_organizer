# 03 — Seed grading systems

Status: ready-for-agent
Blocked by: 02

Seed the "BJJ Kids" and "BJJ Adult" grading systems with their ordered, stripe-fine ranks. Idempotent (safe to re-run). Model per ADR-0001, Option B: ranks are atomic promotion targets carrying **optional descriptive** metadata.

## Rank fields (per system)

Each `Rank`: `name`, `order` (display/sort only), plus optional descriptive fields:
- `group` — major-rank label for grouping/queries (belt colour, e.g. "White"); empty where a system has none.
- `degree` — numeric sub-level for display (BJJ stripes; also fits Karate dan). `0` for the plain belt.

Promotions target a single `Rank`; `group`/`degree` are descriptive only, never part of the promotion.

## BJJ Kids

Belts, in order:
`White, Grey-White, Grey, Grey-Black, Yellow-White, Yellow, Yellow-Black, Orange-White, Orange, Orange-Black, Green-White, Green, Green-Black`.

Stripe-fine: **each** belt gets ranks for **0–3 stripes** (`degree` 0..3). `group` = belt name.

## BJJ Adult

Belts, in order: `White, Blue, Purple, Brown, Black`.

Stripe-fine: **each** belt gets ranks for **0–4 stripes** (`degree` 0..4). A rare **5th** stripe is awarded occasionally — the data-driven model makes adding that rank trivial, so seed 0–4 and add a 5th only where the club actually uses it. `group` = belt name.

## Acceptance

- Ranks are queryable in order per system.
- Each rank carries `group` + `degree` (e.g. "White, 2 stripes" → `group=White`, `degree=2`).
- Re-running the seed does not duplicate rows.
