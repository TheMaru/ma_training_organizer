# A grading system's identity is a slug, not its display name

## Context

`grading_systems` is seeded reference data (ADR-0001), and until now a system was
identified by whatever was convenient at the call site: its `name` in every
rendered view, its `id` in the promotion form. Nothing needed a *stable* handle,
because nothing outside the database referred to a system.

Two things changed that at once:

1. The roster cohort filter ([[roster-cohort-filter]]) puts a system into the
   roster's URL as `?system=`, alongside `?sort=`/`?dir=`. A URL is a handle that
   trainers bookmark, share and — for the operator of a single-club instance —
   edit by hand.
2. [[i18n-domain-data]] wants `BJJ Kids` → `BJJ Kinder`. That makes `name` a
   *translatable display string*, and a translatable string cannot also be an
   identity: the handle would either change with the trainer's locale or freeze
   the English seed string into the URL forever.

So we must decide what identifies a grading system outside the database.

## Decision

`grading_systems` gains a **`slug`**: a short, stable, hand-authored technical
identifier (`bjj-kids`, `bjj-adult`), written in the seed next to the name.

The slug is what leaves the database — URLs, and any future config or export.
`name` becomes **purely a display label**, free to be renamed or localized
without breaking a single link.

`none` is **reserved** as the URL value meaning "no grading system at all" (the
ungraded athletes, see ADR-0007). Because slugs are hand-authored in the seed
rather than derived, a collision with the reserved word cannot arise by accident.

## Considered Options

- **The `id` (rejected):** free, opaque, and already the DB's join key. Rejected
  because ids are autoincrement values assigned in seed order — equal across
  instances only by coincidence, not by guarantee — and because `?system=2` tells
  a human editing the URL nothing. On a single-club instance the operator *is* a
  URL reader; that is a real requirement here, not a hypothetical one.
- **The URL-encoded `name` (rejected):** readable, no migration. Rejected twice
  over. It breaks the invariant documented at `internal/web/rosterquery.go` that
  every rendered query value comes from a fixed set and therefore never needs
  escaping — `BJJ Kids` contains a space. And it makes the identity translatable,
  which is precisely the coupling this ADR exists to cut.
- **A slug derived from the name at read time (rejected):** readable and free of
  a migration, but derivation from a translatable name re-introduces the same
  coupling through the back door, and a *derived* slug could silently collide
  with the reserved `none` sentinel (a system named "None").

## Consequences

- Migration `00004_grading_system_slug.sql` follows `00003_grading_system_sort_order.sql`
  exactly: add the column with a default, and let the seed fill it. Existing rows
  are corrected on the next boot, which is already what `ensureGradingSystem`
  does for `sort_order` — no backfill script, no separate step.
- No unique index is required: the seed is the only writer of `grading_systems`
  and slugs are hand-authored, so uniqueness is a property of the reference data
  rather than something the schema has to defend.
- [[i18n-domain-data]]'s open question — translation column on `grading_systems`,
  or leave system names untranslated? — gets substantially cheaper. With identity
  moved to the slug, putting a localized name next to `name` is a display-layer
  change, not a change to what a system *is*.
- Renaming a seeded system is now a safe, link-preserving operation. Changing a
  slug is not, and should be treated like changing a URL.
- The same pattern is available to `ranks` if they ever need to be addressable
  from outside, but this ADR does not introduce it there — ranks are currently
  only ever referenced by id, inside a form.
