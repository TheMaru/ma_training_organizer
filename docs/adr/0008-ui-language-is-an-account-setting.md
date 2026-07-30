# The UI language is an account setting, resolved per request

## Context

The app is bilingual (German + English) at the UI level ([[i18n-ui]]): some
trainers are not native German speakers, and the string set is small enough that
building this in now is far cheaper than retrofitting it later. Two questions
had to be settled before any string moved: where the active language comes from,
and what carries the translations.

The app sits entirely behind a login and is never indexed, so the usual reason
for `/de/…`/`/en/…` prefixes — one indexable URL per language — does not apply
here. It also renders HTMX fragments from the same handlers as full pages, so
whatever carries the language has to reach both without a second mechanism.

## Decision

**The language is a property of the trainer's account**, stored in a `locale`
column on `trainers` (migration `00005_trainer_locale.sql`), switched from a
`DE | EN` control in the nav (`POST /account/language`).

**One middleware resolves the request's locale** — the authenticated trainer's
stored value, else `Accept-Language`, else German — and puts it in the request
context. Renderer and handlers read it from there, which is what makes HTMX
fragments inherit it for free and leaves routing untouched. An empty or no
longer supported stored value reads as "never chose" and falls back to
`Accept-Language`, so the login page (where no trainer is known) is the same code
path rather than a special case.

**Translations live in `internal/i18n`**: two embedded JSON catalogs with dotted
keys, reached as `{{t "key"}}` in templates and `i18n.T(locale, key)` in Go. The
renderer parses **one template set per locale**, so the locale is baked into the
func map instead of being threaded through every call site. A missing key falls
back to German and then to the key itself, and a parity test keeps the two
catalogs' key sets identical.

**Scope is the UI**: chrome and user-facing messages. Domain data (rank and
grading-system names) still renders raw — localizing it is a model change
([[i18n-domain-data]], and cheaper since ADR-0006) — and internal errors and CLI
output stay English.

## Considered Options

- **URL prefixes `/de/…` (rejected):** the SEO standard for public sites, and
  the rationale evaporates behind a login. It would mean reworking routing and
  every HTMX URL for a benefit we do not collect.
- **A query parameter (rejected):** same rework, and it collides with the
  roster's own view state, which `rosterView` deliberately keeps closed
  (ADR-0005, ADR-0007).
- **A session value rather than an account column (rejected):** would not follow
  a trainer to a second device, and sessions are already revocable server-side —
  losing the language on logout is a surprise, not a feature.
- **`nicksnyder/go-i18n` (rejected):** the standard choice, but overkill for two
  languages and a small string set with no pluralization to speak of (the
  countable cases are parked domain data). ADR-0002's minimal-dependency rule
  applies, and the call site `{{t "key"}}` is identical if we ever migrate.
- **`golang.org/x/text/message` (rejected):** its `gotext` extraction is a build
  step, which ADR-0002 rules out. We do use `golang.org/x/text/language` — the
  single new direct dependency — for `Accept-Language` matching, which is the one
  part of this worth not hand-rolling.

## Consequences

- Adding a locale is one entry in `i18n.supported` plus a catalog file; the
  matcher and the template sets are both built from that list, so the two halves
  cannot drift.
- The roster's column labels live in Go (`rosterColumns`) as catalog *keys*, and
  `athletes.html` repeats those keys in each cell's `data-label`. That shared key
  is what keeps the phone card labels identical to the desktop headers.
- Resolving the locale costs one `TrainerByID` per authenticated request. At this
  app's scale (one SQLite connection, a handful of trainers) that is cheaper than
  a cache that has to be invalidated when the switcher writes.
- The switcher needs somewhere to return to, and that target travels through the
  client. It is rebuilt from the normalised view state rather than echoed from
  the request, and validated to be an in-app path before it reaches a `Location`
  header.
- The German UI lost its stray English strings (`Athletes` in the nav) on the way
  through, and gained `Abmelden` for `Logout`.
