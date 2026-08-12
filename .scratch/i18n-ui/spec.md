# Spec — UI internationalization (German + English)

Status: done

Make the app bilingual (German + English) at the **UI level**: a trainer can use
the tool in either language, chosen per account. Motivated by non-native-German
trainers, and by building multilingual support in early (retrofitting later is
much more expensive).

Outcome of a grilling + domain-modeling session (2026-07-24). Every decision
below is settled.

## Locale source & persistence

- **Stored preference, not URL prefixes.** URL-per-language is the SEO standard
  for publicly indexed sites; this app is entirely behind login and never
  indexed, so that rationale does not apply. A stored preference also lets HTMX
  fragments inherit the locale for free (they run through the same handlers),
  keeping routing untouched. (Rejected: `/de/…`/`/en/…` prefixes — large routing
  + HTMX-URL rework for a benefit we don't need; rejected: query param.)
- **New `locale` column on `trainers`** (goose migration), holding the account's
  chosen language. Set via the switcher (below).
- **First-visit / pre-login default from `Accept-Language`**, fallback **German**.
  The login page and its errors are pre-auth (trainer unknown), so they resolve
  locale from `Accept-Language`/default only. (The edge case "trainer switches
  language on the login page and expects it to stick" is deliberately out of
  scope for v1.)

## Scope: what gets translated

- **In scope:** UI chrome (nav, buttons, labels, headings) + user-facing Go
  messages (form validation, login/flash errors).
- **Out of scope (parked):** domain data — rank/belt names (`White`,
  `White, 1 stripe`) and grading-system names (`BJJ Kids`), which live as seed
  data and render raw (English) today. Localizing them is a model change
  (per-locale translations) that touches ADR-0001; see [[i18n-domain-data]]. It
  is likely subsumed by the belt visual ([[rank-belt-visual]]) via a locale-aware
  colour map. Accepted interim state: translated UI, raw English rank names.
- **Stays English (internal/operator-facing):** `serverError`/500s, wrapped
  store/auth/config errors, and the operator-facing CLI subcommands — trainer
  creation, password reset, session revocation, demo seed/clear.
  `store`/`auth`/`config` contain no German strings; `ErrPasswordTooShort` is
  English and is mapped to a German user message in the web layer.
- **Stays German, outside the catalogs (trainer-facing CLI):** the
  `import-athletes` subcommand reports to the trainer in German — the success
  report, the skipped rows, and the abort report with its per-line problems. It
  reports on German CSV column headers (`Vorname`, `Nachname`, `Geburtsdatum`,
  `Beitritt`, `Notizen`), which double as the display names in those messages —
  ADR-0008's update of 2026-08-12 carries the reasoning. It is not resolved
  through `internal/i18n`: ADR-0008 binds the locale to the trainer's account and
  the CLI has no account. Its `usage:` line stays English like any other
  operator-facing failure. (The German prose in the `seed-demo` fixtures is
  sample content, not UI.)

## Implementation

- A small **`internal/i18n` package** with **embedded JSON catalogs**
  (`de.json`, `en.json`, dotted keys like `nav.logout`, `athletes.new`),
  exposed to templates via a `{{t "key"}}` function and to Go handlers via an
  equivalent lookup. No i18n library (fits ADR-0002: minimal deps, no build
  step, single binary). Migrating to `nicksnyder/go-i18n` later stays cheap —
  the call site `{{t "key"}}` is unchanged. (Rejected: `go-i18n` now — overkill
  for two languages + a small string set with little pluralization need, since
  the countable cases like "1 stripe" are parked domain data; rejected:
  `golang.org/x/text/message` — its `gotext` extraction is a build step.)
- **Locale-resolution middleware** determines the request locale (authenticated
  trainer's `locale` → `Accept-Language` → German) and puts it in the request
  context; the renderer and handlers read it there.
- **HTMX needs no special handling** — fragments run through the same handlers
  and inherit the context locale.
- **`<html lang>` is set dynamically** from the active locale (was hard-coded
  `de`).
- **`Accept-Language` matching via `golang.org/x/text/language`** (already in the
  module graph; using it directly adds only a `require` line, no new download).

## Language switcher

- A **`DE | EN` toggle in the nav header, next to the theme toggle** — most
  discoverable, and consistent with the existing theme control. Unlike the theme
  toggle (client-side localStorage), language is a server round-trip:
  `POST /account/language` → update the trainer's `locale` → redirect back.
  (Rejected: a setting buried on the account page — less discoverable.)

## Go strings to move into catalogs

All user-facing German lives in `internal/web` handlers (+ templates):

- `athletes.go`: `Neuer Athlet`, `Athlet bearbeiten`, `Vor- und Nachname sind
  erforderlich.`
- `auth.go`: `Das aktuelle Passwort ist falsch.`, `Die neuen Passwörter stimmen
  nicht überein.`, `Das neue Passwort ist zu kurz.`, `Benutzername oder Passwort
  ist falsch.`
- `promotions.go`: `Bitte Rang und Datum wählen.`

(The `"Athletes"`/`"Athlete"` occurrences in `athletes.go` are template data keys,
not display text.)

## Robustness

- Missing key → fall back to the **default locale (German)** value; if missing
  there too, render the **key itself** so gaps are visible in the UI.
- A **parity test** asserts `de.json` and `en.json` have identical key sets, so a
  forgotten translation fails in tests, not in the browser.

## Acceptance

- A trainer can switch between German and English from the nav; the choice
  persists on their account across sessions and devices.
- First visit / pre-login honours `Accept-Language`, defaulting to German.
- All UI chrome and user-facing messages render in the active language; internal
  errors and the operator-facing CLI subcommands remain English, and the
  trainer-facing `import-athletes` report stays German outside the catalogs.
- `<html lang>` reflects the active locale.
- A parity test enforces matching keys across `de.json` and `en.json`.
