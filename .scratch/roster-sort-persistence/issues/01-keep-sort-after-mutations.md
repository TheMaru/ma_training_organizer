# 01 — Mutations drop the trainer's active roster sort

Status: needs-triage

Deleting, creating or editing an athlete redirects to a bare `/athletes`, so the
trainer lands back in the default Vorname-ascending view. Found by the code review
of [[roster-sortable-columns]] (shipped in `c8ff458`, 2026-07-27).

## Motivation

`handleAthleteDelete`, `handleAthleteCreate` and `handleAthleteUpdate` all call
`redirect(w, r, "/athletes")`. Before sorting was generic this cost at most a
direction flip; now a trainer who sorted by "Aktueller Rang" to work through a
grading list loses their view on every delete. The delete button sits in the
roster itself, so it is the common case.

## Scope

Carry the active `?sort=&dir=` through the mutation and back into the redirect for
all three handlers.

## Open questions (triage material)

- **Where does the sort come from on a POST?** Cleanest is to render it into the
  form action / a hidden field from the same header state the roster already
  builds (`rosterHeaders`), rather than parsing `Referer`, which is spoofable and
  absent on some clients. The create/edit forms would need to carry it too.
- **Does the edit form's "Zurück" link get the same treatment?** Presumably yes,
  for consistency.
- **Whitelisting on the way back.** The redirect target must be built from
  `store.NormalizeRosterSort` plus the two direction literals, never from the raw
  parameter, so no user-controlled string reaches a `Location` header.

## Acceptance (provisional, pending triage)

- Deleting an athlete from a roster sorted by any column and direction returns to
  that same sorted view.
- Creating and updating an athlete likewise return to the sort the trainer came
  from.
- A junk `sort`/`dir` on the way in still redirects to the valid default view.
