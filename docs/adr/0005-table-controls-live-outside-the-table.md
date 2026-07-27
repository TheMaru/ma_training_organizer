# Table controls live outside the table in the phone card layout

## Context

At phone widths `app.css`'s `@media (max-width: 600px)` block turns every table
into a stack of labelled cards: `table`, `tbody`, `tr` and `td` all become
`display: block`, `td[data-label]::before` renders the column name inside each
card, and `thead` is pushed off-screen with `position: absolute; left: -9999px`.
The comment on that rule states the intent — keep the headers in the
accessibility tree rather than removing them with `display: none`.

Two things forced us to re-examine that arrangement:

1. The roster gained sortable column headers (`c8ff458`). The sort links live in
   the `<th>`s, i.e. inside the off-screen `thead`, so on a phone the feature has
   no surface at all. The kids trainer is exactly the phone user.
2. The roster's view state is moving into the URL (`?sort=&dir=`, with `?system=`
   to follow for the cohort filter), so *how* a control emits a URL is no longer
   a detail — it decides whether unrelated view state survives an interaction.

This ADR records where controls that belong to a table are allowed to live, and
what they are allowed to be built from.

## Decision

**(a) An off-screen `thead` is not an accessibility affordance under
`display: block`.**

Browsers derive the implicit ARIA roles `table` / `row` / `cell` /
`columnheader` from the corresponding `display` values. Once the card layout sets
`display: block` on the table elements, those roles are gone, and the off-screen
`thead` can no longer associate its headers with any data cell. The card
labelling is done entirely by `td[data-label]::before`. The off-screen `thead`
therefore contributes nothing but duplicate content, and at phone widths it is
hidden with `display: none`.

**(b) A table's controls render as sibling links carrying the full view-state
URL, never as form fields.**

A link's `href` is built server-side from the resolved view state, so every
control automatically carries *all* of it. A `GET` form submits only its own
fields: any view state not mirrored into a hidden input is silently dropped from
the resulting URL. That failure is invisible — no error, no failing test, just a
filter that quietly resets whenever someone sorts. Links keep the view state in
one place; forms create a second enumeration of it that must be maintained by
hand.

The concrete application: the roster's phone sort affordance is a horizontally
scrollable row of chips, one `<a href>` per column, rendered from the same
resolved header view model as the desktop `<th>` links. State that the removed
`aria-sort` used to convey is carried by `aria-current` plus visually hidden
text on the active chip.

## Considered Options

- **Keep `thead` visible at phone widths as a compact sort bar (rejected):**
  under `display: block` it renders as a stack of links above the first card, not
  as a control, and it re-introduces into the card layout the header row the card
  layout exists to remove.
- **A `<select>` + submit button instead of chips (rejected):** more compact and
  it scales past five columns, but without JavaScript it costs two interactions
  per sort (ADR-0002 rules out an `onchange` submit), it needs a second view model
  with different semantics (`selected` rather than an active indicator), and it
  inherits the silent view-state loss described in (b).
- **Replacing the desktop header links with one control for all widths
  (rejected):** fewer surfaces, but it discards both the expected
  click-the-column idiom and the `aria-sort` attributes, which on a real desktop
  table are genuine assistive-technology signal.
- **Scoping the `thead` change to the roster (rejected):** the reasoning in (a) is
  not roster-specific. The promotion history table on the athlete detail page is
  in exactly the same position, and a scoping class would separate two things that
  behave identically.

## Consequences

- Two sort surfaces exist in the roster markup — desktop `<th>` links and phone
  chips — but only one truth: both render from `rosterColumns` and the resolved
  sort state, into the same URLs.
- Every future table control (the parked cohort filter first) has its shape
  decided in advance: a row of links, not a form. Adding a view-state key means
  one entry in one whitelist, not an audit of every control.
- Screen-reader users at phone widths lose the `aria-sort` announcement, which is
  replaced by `aria-current` and visually hidden direction text on the active
  chip. This is a different mechanism, not an equivalent one, and it only applies
  where the table roles were already absent.
- The promotion history table's `thead` also stops rendering at phone widths.
  Intentional: it was in the same position and contributed the same nothing.
