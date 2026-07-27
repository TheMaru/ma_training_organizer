# Spec — Belt graphic for a rank (visual rank display)

Status: done

Render an athlete's rank as a **belt graphic** (body colour + split-belt bar +
stripes) alongside or in place of the plain rank name, wherever a rank name is
shown. A small visual-polish feature; the text name stays the accessible
baseline (ADR-0001, ADR-0004).

Outcome of a grilling + domain-modeling session (2026-07-24). Triaged
**accepted for build now** (pre-deploy set). Every decision below is settled;
the open questions on the originating issue are resolved here.

## Rendering technique

- **Self-built inline SVG**, server-rendered from `rank_group` + `degree`. No
  build step, no CDN, no external asset files (ADR-0002). (Rejected: external BJJ
  SVG sets — discipline-specific, still need the same colour/stripe mapping, add
  embedded files + licence/attribution.)

## Where belt-ness and colour data live (ADR-0004)

- A **view-layer convention**, not data-model metadata. A hardcoded
  colour-name→hex table + split-belt parser in the Go/view layer. A rank renders
  as a belt **only when its `rank_group` resolves** in the table; otherwise it
  falls back to the plain rank name. **No schema/model change.**
- The seam for other systems is code-side: a new belt system = extend the table;
  a non-belt system has no entries → text. Recorded in **ADR-0004**.

## Belt appearance (BJJ-faithful)

- Belt **body** in the `rank_group` colour.
- **Split belts** (`rank_group` = `<colour>-White` or `<colour>-Black`): a
  contrasting **longitudinal bar** in white/black over the body colour.
- Black **friso** (end block) carrying `degree` white **stripes** (0–4).
- **Parsing rule:** split `rank_group` on `-`; if two parts and the second is
  `White` or `Black`, that is body-colour + bar; otherwise the whole string is a
  single body colour. `degree` = number of white stripes on the friso.
- Colours needed by the seed today: White, Grey, Yellow, Orange, Green, Blue,
  Purple, Brown, Black (body), plus White/Black bars.

## Surfaces (everywhere a rank name appears)

- **Roster / overview list: graphic only.** The rank name goes into
  `aria-label`/`title` (and `alt` as applicable), including the system to
  disambiguate same-colour ranks across cohorts (White exists in Kids and Adult).
- **Athlete detail — "Aktueller Rang": graphic + name.**
- **Athlete detail — "Verlauf" table: graphic + name** on each promotion row.
- The roster surface **composes on `roster-sortable-columns`**, which introduces
  the roster's "Aktueller Rang" column and `ListRoster`; if that column is not
  yet present, the roster part lands when it is. Filtering by rank/cohort is a
  separate ticket ([[roster-cohort-filter]]) and is **not** built here.

## Data plumbing

- `store.Promotion` currently carries only `RankName` + `SystemName`. Add
  **`Group` (rank_group)** and **`Degree`**, denormalised and populated by
  `ListPromotions` (join already selects the rank), so the current rank and each
  history row can be rendered.
- The roster row type (`RosterRow` from `roster-sortable-columns`) must likewise
  carry `Group` + `Degree` for the current rank.
- A single reusable render helper (e.g. a template function `belt <group>
  <degree>` returning safe inline SVG) is used by all three surfaces, so the
  markup cannot drift between them.

## Accessibility / fallback (ADR-0001)

- The text rank name is always available: shown next to the graphic on the detail
  surfaces, and as `aria-label`/`title` on the roster where only the graphic
  shows.
- **Unknown colour ⇒ no SVG, text only.** The graphic never becomes the sole
  carrier of meaning.

## QA test data (`seed-demo`)

- Rework the demo roster into **~12 curated athletes** whose current ranks and
  histories together give confidence the **bar** and **stripes** render
  correctly — not all 77 ranks, but sure visual coverage:
  - every body colour at least once (White, Grey, Yellow, Orange, Green, Blue,
    Purple, Brown, Black);
  - **both bar variants** (`-White` and `-Black`) and plain (no-bar) belts;
  - **stripe degrees 0, 1, 2, 3, 4** (checks "1 stripe" vs "N stripes" and max);
  - at least one **split + stripes** combination (both features compose);
  - a **white belt with stripes** (friso on a light body);
  - one **ungraded** athlete (text/blank fallback).
- Keep `seed-demo` / `clear-demo` idempotent as they are (identity = first+last
  name). The scan is done via the detail "Verlauf" table (graphic + name, so each
  belt can be checked against its label) and the roster (graphic-only rendering).

## Domain model

- **No new glossary term** — belt / friso / stripe are display details, not new
  domain concepts.
- **ADR-0004** records the view-convention decision (hardcoded colour map + text
  fallback, deliberately not model metadata, despite ADR-0001).

## Acceptance

- A rank with a known colour renders as an inline SVG belt: correct body colour,
  a white/black bar for split belts, and `degree` white stripes on a black friso.
- The roster shows the belt graphic only, with the rank name (+ system) as its
  accessible label; the detail page and history table show belt + name.
- A rank whose colour is unknown renders as plain text, with no broken graphic.
- `seed-demo` produces ~12 athletes covering both bar variants, stripe degrees
  0–4, a split+stripes combo, a striped white belt, and one ungraded athlete.
- One render helper backs all three surfaces (no duplicated SVG markup).
- The model/schema is unchanged; `Promotion`/roster-row types carry `Group` +
  `Degree` for rendering only.
