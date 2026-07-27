# 01 — UI internationalization (German + English)

Status: ready-for-agent

Make the app bilingual at the UI level, chosen per trainer account. Full detail
and rationale in [`../spec.md`](../spec.md).

## Motivation

Some trainers are not native German speakers; the app is currently German UI with
a few stray English strings (`Athletes` in the nav) and English domain data
(belt names). Building multilingual support in now is far cheaper than
retrofitting later.

## Summary of decisions (see spec)

- **Locale source:** stored per-trainer preference (new `locale` column on
  `trainers`), `Accept-Language` as first-visit/pre-login default, fallback
  German. **No URL prefixes** (behind login, never indexed → SEO rationale gone;
  HTMX inherits locale for free).
- **Scope:** UI chrome + user-facing Go messages only. Domain data (rank/system
  names) parked ([[i18n-domain-data]], likely subsumed by [[rank-belt-visual]]).
  Internal errors + CLI stay English (already clean).
- **Implementation:** homegrown `internal/i18n` + embedded JSON catalogs
  (`de.json`/`en.json`), `{{t "key"}}` template func + Go lookup; no library
  (ADR-0002). Locale-resolution middleware → request context; `<html lang>`
  dynamic; `Accept-Language` via `golang.org/x/text/language`.
- **Switcher:** `DE | EN` toggle in the nav next to the theme toggle,
  `POST /account/language` → update trainer → redirect back.
- **Robustness:** missing key → German fallback → visible key; parity test
  enforces identical key sets across catalogs.

## Acceptance

See spec. Key points: per-account persistent language, `Accept-Language`
first-visit default, all chrome + user messages translated, internal/CLI stay
English, dynamic `<html lang>`, catalog parity test.

## Comments

2026-07-27: [[roster-sortable-columns]] (`c8ff458`) introduced the first
user-facing German UI strings that live in **Go code rather than a template** —
the roster column labels in `rosterColumns` (`internal/web/roster.go`), which the
template renders via `{{range .Headers}}`. The i18n pass must translate those
through the Go-side lookup, not only the `{{t "key"}}` template func; the roster
`<td data-label="…">` attributes carry the same labels a second time (mobile card
layout) and have to stay in sync with them.
