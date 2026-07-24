# 01 — CSV athlete import (idempotent)

Status: ready-for-agent

Add a CLI subcommand that bulk-imports **athlete master data** from a CSV file
into the roster, idempotently. Bootstraps a club's existing member list so the
trainer starts from real data rather than hand-entering everyone.

## Motivation

Onboarding a club means getting its existing members into the roster. Typing
each athlete by hand is tedious; trainers already keep their members in a
spreadsheet. A CSV import turns that spreadsheet into the initial roster, and
being idempotent means a re-run (after fixing or extending the file) is always
safe.

## Scope

- New subcommand `organizer import-athletes <file.csv>`, wired like `seed-demo`:
  a testable core plus a thin CLI wrapper using the existing `withDB` helper.
- Imports the five `store.Athlete` fields only. **No promotions.**
- Idempotent on (Vorname, Nachname): existing athletes are skipped, not updated.

## Resolved design questions

Settled in a grilling + domain-modeling session (2026-07-24). Full detail in
[`../spec.md`](../spec.md); summary:

- **Surface.** ✅ CLI subcommand, not web upload (rare setup action; accounts are
  already provisioned via CLI; avoids multipart/CSRF/failure-UI cost).
- **Format.** ✅ Header row, named **German** columns (`Vorname`, `Nachname`
  required; `Geburtsdatum`, `Beitritt`, `Notizen` optional), case-insensitive,
  unknown columns ignored. Delimiter `,`/`;` auto-detected, UTF-8 BOM stripped.
  Dates ISO or `dd.mm.yyyy`, normalised to ISO; empty ⇒ NULL.
- **Idempotency.** ✅ Identity (Vorname, Nachname) like `seed-demo`; existing ⇒
  **skip** (reported, not updated). In-file duplicate names ⇒ validation error
  (both line numbers).
- **Errors.** ✅ **All-or-nothing** in one transaction: validate whole file,
  report all errors together, write nothing and exit non-zero on any error.
  Duplicates are not errors. Minimal validation, no stricter than web CRUD. No
  `--dry-run`.
- **Report.** ✅ German stdout: `N importiert, M übersprungen` (+ skipped names)
  on success; `Import abgebrochen, K Fehler:` (+ line list) on abort.

## Domain / ADR

- No new glossary term (name-based identity already exists via `seed-demo`).
- No ADR (reversible, mirrors `seed-demo`, no locked-in trade-off).

## Acceptance

- Importing a file adds new athletes and prints the count imported + the skipped
  names.
- Re-running the identical file imports 0, skips all — no duplicates.
- A `;`-delimited, BOM-prefixed German-Excel export with `dd.mm.yyyy` dates
  imports correctly (stored as ISO); column order may vary; extra columns ignored.
- Any bad row (bad date, blank required name, in-file duplicate) imports nothing,
  lists every offending line, and exits non-zero.
- The core is tested without the CLI wrapper across happy path, idempotent
  re-run, both delimiters, BOM, both date formats, each error class, and rollback.
