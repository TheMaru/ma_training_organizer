# Belt graphic is a view-layer convention, not data-model metadata

## Context

ADR-0001 makes the domain model deliberately discipline-agnostic and
data-driven: a `Rank` carries only optional, descriptive `group` (belt colour
name, free text) and `degree` (stripe count) fields, and the model bakes in no
belt/stripe conventions. `rank_group` is just a string like `White`,
`Grey-White` or `Yellow-Black`.

We now want to render an athlete's rank as a **belt graphic** (body colour,
split-belt bar, black friso with `degree` white stripes) — a small visual-polish
feature. Drawing a faithful belt inevitably encodes BJJ conventions: a
colour-name→hex table and split-belt parsing (`<colour>-White|Black`). That sits
in tension with ADR-0001's "no discipline hardcoded" stance, so we must decide
*where* that BJJ knowledge lives.

## Decision

The belt graphic is a **view-layer convention**, not new data-model metadata.

- A hardcoded colour-name→hex table and split-belt parser live in the Go/view
  layer. A rank renders as an inline SVG belt **only when its `rank_group`
  resolves** in that table; otherwise it falls back to the plain rank name.
- **No schema or model change.** The grading system does not *declare* itself
  belt-renderable and stores no colour convention. Renderability is *derived*
  from whether the view knows the colours.
- The text rank name **remains the accessible baseline** (ADR-0001): it is always
  present as the label (and as `aria-label`/`title` where the roster shows the
  graphic alone). The belt is pure enhancement.

The "seam" for other grading systems is therefore cheap and code-side: a new
belt system is added by extending the colour table; a non-belt system (e.g.
numbered dan grades with no colour) simply has no entries and renders as text —
no migration, no per-system renderer registry.

## Considered Options

- **Data-model metadata (rejected):** columns/table so a system declares
  `belt_renderable` and supplies its palette. More "correct" per ADR-0001's
  data-driven spirit, but over-engineered for two BJJ systems — a migration, more
  schema, and speculative generality with no second consumer today.
- **External BJJ SVG asset set (rejected):** discipline-specific, still needs the
  same colour/stripe mapping to pick an asset, and adds embedded files +
  licence/attribution.

## Consequences

- The model stays exactly as ADR-0001 left it: `rank_group` is still free text,
  agnostic. The BJJ-specific knowledge is quarantined in the view and never
  reaches the domain.
- A future non-BJJ belt system renders as text until someone extends the colour
  table — an intentional, graceful degradation, not a bug.
- If a real second belt system with a genuinely different palette convention
  arrives, migrating the hardcoded table to per-system metadata is rework; that
  cost is accepted now in exchange for shipping the feature without a schema
  change. This ADR records why the table is hardcoded despite ADR-0001, so that
  rework is a deliberate revisit rather than a surprise.
