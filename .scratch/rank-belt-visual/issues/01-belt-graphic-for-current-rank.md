# 01 — Belt graphic for a rank (visual rank display)

Status: ready-for-agent
Blocked by: roster-sortable-columns/01 (introduces `ListRoster` + `RosterRow` and the roster's "Aktueller Rang" column, which the roster belt surface and `Group`/`Degree` plumbing build on)

Render an athlete's rank as a **belt graphic** (colour + stripes) instead of, or
alongside, the plain rank name — on the roster's "Aktueller Rang" column first,
and plausibly on the athlete detail page and promotion history too.

Parked deliberately out of the sortable-roster feature
([[roster-sortable-columns]]): that feature ships the **text** variant (rank name
+ muted system), which stays the accessible, universal baseline. The belt visual
is a display enhancement on top and needs its own design round before build.

## Why this is not a trivial add

- The data is already there: each rank carries `rank_group` (colour) and
  `degree` (stripe count). See `internal/store/seed.go`.
- But drawing a faithful belt means **parsing `rank_group`** and encoding BJJ
  belt conventions: the kids seed uses *split-belt* names like `Grey-White`,
  `Yellow-Black` (body colour + a white/black bar), plus a black friso carrying
  `degree` white stripes. That needs a colour-name→hex table and split-belt logic.
- **Tension with ADR-0001.** The model is deliberately discipline-agnostic and
  data-driven; `rank_group` is today just a free-text string. A belt renderer
  hard-wires BJJ belt conventions into the view, and a future non-belt grading
  system would have no meaningful graphic. So the **text name must remain** as
  the fallback and as `alt`/`aria-label` — the belt is pure enhancement.

## Open questions to settle before building (grill these)

- **Self-built inline SVG vs. an existing asset set/lib.** First pass leaned
  self-built inline SVG from `rank_group`+`degree` (licence-free, data-driven,
  crisp at any size), over sourcing external BJJ SVG sets (some MIT/CC on
  GitHub) which are BJJ-specific, still need the same colour/stripe mapping to
  pick an asset, and add licence/attribution + embedded asset files. **User
  wants to challenge this** — re-evaluate the named libs and whether one is worth
  it before committing to hand-rolled SVG.
- **A "text or SVG" switch that generalises to other belt systems.** Rather than
  hard-coding one renderer, design a small seam: a rank renders as text by
  default, and *opts into* a visual when its grading system provides the needed
  descriptive data (colour + stripe convention). How is that data expressed so a
  new belt system (or a non-belt system) can plug in — extra columns/metadata on
  the grading system or rank, a small per-system renderer registry, a
  colour-map lookup? Keep it data-driven, don't re-hardcode a discipline.
- **Split belts / colour mapping.** Where does the colour-name→hex table live,
  and how are compound names (`Grey-White`) parsed without baking BJJ assumptions
  everywhere?
- **Scope of surfaces.** Roster only, or also detail page + promotion history?

## Acceptance (provisional — to be firmed up at triage)

- Ranks render as a belt graphic where a system supplies the needed data, and
  fall back cleanly to the text name otherwise.
- The rank name remains available for screen readers / as the accessible label.
- No discipline hard-coded in a way that blocks a future non-BJJ belt system.

## Comments

2026-07-24 — Triaged: **accepted for build now** (pulled into the pre-deploy set). A small, high-delight visual-polish feature. Next step: a grill-with-docs session to settle the four open design questions, then spec + build. Text rank name stays the accessible baseline (ADR-0001).

2026-07-24 — Grilled + spec'd (grill-with-docs). All four open questions resolved; full detail in [`../spec.md`](../spec.md). Summary: self-built inline SVG; belt-ness/colour = **view-layer convention** (hardcoded colour map + text fallback, no schema change) recorded in **ADR-0004**; BJJ-faithful (body + split-belt bar + friso with degree stripes); surfaces = roster (graphic only, name in aria-label), detail current-rank + Verlauf table (graphic + name); `Promotion`/roster-row gain denormalised `Group`+`Degree`; one shared render helper; unknown colour → text. QA: rework `seed-demo` to ~12 curated athletes covering both bar variants, stripe degrees 0–4, a split+stripes combo, a striped white belt, and one ungraded. Status → ready-for-agent.
