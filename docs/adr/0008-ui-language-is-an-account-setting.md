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

*The clause about CLI output was already wrong when written — see the Update of
2026-08-12 below.*

> **Superseded in part by ADR-0009.** The claim that localizing rank and
> grading-system names is a model change turned out to be wrong: a rank's name is
> derivable from `rank_group` + `degree`, and a system's name has been a mere
> display label since ADR-0006, so both are localized in the view with no schema
> change. Everything else in this ADR stands.

## Considered Options

- **URL prefixes `/de/…` (rejected):** the SEO standard for public sites, and
  the rationale evaporates behind a login. It would mean reworking routing and
  every HTMX URL for a benefit we do not collect.
- **A query parameter (rejected):** same rework, and it collides with the
  roster's own query, which `rosterQuery` deliberately keeps closed
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
  client. It is rebuilt from the normalised query rather than echoed from
  the request, and validated to be an in-app path before it reaches a `Location`
  header.
- The German UI lost its stray English strings (`Athletes` in the nav) on the way
  through, and gained `Abmelden` for `Logout`.

## Update, 2026-08-12

The decision stands untouched; the scope paragraph's last clause does not. "CLI
output stays English" was wrong about the CLI on the day it was written, and is
left above as written rather than repaired in place — the same way ADR-0009 left
this ADR's rank-name reasoning standing. (ADR-0009's note above closes
"everything else in this ADR stands"; it was written before this correction and
did not look at the scope paragraph.) `import-athletes` had landed the day before
([[csv-athlete-import]], 2026-07-29) and reports to the trainer in German; this
ADR repeated a verification the i18n spec had made on 2026-07-24, when it was
still correct.

The CLI is two surfaces, not one:

- **Trainer-facing.** Everything `import-athletes` writes for the trainer to read
  is German: the success report, the skipped rows, and the abort report with its
  per-line problems. (Its `usage:` line and a bare `read csv:` wrapper are
  English, like any other operator-facing failure.) The German follows from the
  CSV column headers it reports on — `Vorname`, `Nachname`, `Geburtsdatum`,
  `Beitritt`, `Notizen` — a file format clubs have already prepared spreadsheets
  against, and one the messages use as their display names, so the two cannot
  diverge. Three constructions exist only to serve it:
  `countAthletesDE` for the German noun forms of a count, `germanDate` accepted
  alongside ISO on input, and `errAlreadyReported` in `main.go`, whose only job is
  to suppress the generic English `error:` line that would otherwise follow a
  German report.
- **Operator-facing.** Trainer creation, password reset, session revocation and
  the demo seed/clear subcommands are English and stay so, as do `serverError`/
  500s and wrapped store/auth/config errors. This is the half the original clause
  described, and nothing about it changes.

Routing the import report through `internal/i18n` instead was considered and
rejected: the decision above binds the locale to the trainer's account, the CLI
has no login and no account, and resolving from `LANG`/`LC_ALL` would be exactly
the second mechanism this ADR set out to avoid. Nobody has asked for an English
import report.
