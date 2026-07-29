# Spec — CSV athlete import (idempotent)

Status: done

A CLI subcommand that bulk-imports **athlete master data** from a CSV file into
the roster. Idempotent: re-running the same file never creates duplicates. Aimed
at the one-time bootstrap of a club's existing member list before the trainer
takes over editing in the web UI.

Outcome of a grilling + domain-modeling session (2026-07-24). Every decision
below is settled; the originating triage question is resolved here.

## Surface & scope

- **CLI subcommand**, `organizer import-athletes <file.csv>`, mirroring
  `seed-demo`: a testable core (`func importAthletes(db, r io.Reader) (report, error)`)
  plus a thin `cmdImportAthletes` wrapper wired in `main.go`, opening/migrating
  the DB via the existing `withDB` helper. (Rejected: web upload — import is a
  rare setup action, accounts are already provisioned out-of-band via CLI, and a
  web upload would add multipart handling, size limits, a failure UI and CSRF for
  little gain.)
- **Athlete master data only** — the five `store.Athlete` fields. No promotions:
  the 1:n rank history is awkward in flat CSV and would need rank-to-grading-system
  resolution; the trainer adds graduations afterwards in the web UI where the rank
  picker already lives.

## CSV format

- **Header row with named columns**, matched case-insensitively and trimmed.
  Required: `Vorname`, `Nachname`. Optional: `Geburtsdatum`, `Beitritt`,
  `Notizen`. A missing optional column ⇒ that field is empty for every row.
  **Unknown columns are ignored** (so a real club export with extra columns still
  runs). A missing *required* column is a fatal error before any row is read.
- **German header names only** — consistent with the German-first domain data.
  English aliases wait until the parked domain-data i18n work
  ([[i18n-domain-data]]); adding them now would be speculation.
- **Delimiter auto-detected** as `,` or `;` by sniffing the header line (German
  Excel exports `;`). A leading UTF-8 **BOM is stripped**. Encoding is UTF-8;
  cp1252 is deliberately out of scope. (Rejected: comma-only RFC 4180 — the most
  common real failure is a German Excel `;` export.)
- **Dates** accepted as ISO `yyyy-mm-dd` **or** German `dd.mm.yyyy`, normalised
  to ISO for storage. Empty ⇒ SQL NULL (the columns are optional). Any other
  string is a row error.

## Idempotency & duplicates

- **Identity = (Vorname, Nachname)**, the same key `seed-demo` uses
  (`nameKey`). Trimmed before comparison.
- An athlete whose (Vorname, Nachname) **already exists in the DB is skipped**,
  not updated — the web UI is the editing surface and a skip never clobbers a
  hand-edit. Each skip is recorded in the report (name + reason "bereits
  vorhanden"). (Rejected: upsert — would let a stale CSV overwrite later web
  edits.)
- Two rows in the **same file** sharing a (Vorname, Nachname) is a **validation
  error** naming both line numbers — not a skip. Picking a winner would be
  arbitrary and dropping the second is silent data loss; it almost always signals
  a messy source the trainer should see.

## Validation & transaction boundary

- **All-or-nothing.** The whole file is parsed and validated first; if there is
  ≥1 error, **nothing is written**, every error is reported together (line number
  + reason), and the process exits non-zero. Only a fully valid file is inserted,
  in a **single transaction**. (Rejected: best-effort partial import — leaves a
  half-filled DB and a surprising "errored but half-imported" outcome; idempotent
  re-runs make it unnecessary.)
- Skipped **duplicates are not errors** — they never abort the import; they are
  reported separately.
- **Minimal** field validation, no stricter than the web CRUD (so import never
  surprises by rejecting what hand-entry accepts):
  - `Vorname` and `Nachname` non-empty after trim.
  - `Geburtsdatum` / `Beitritt` empty or parseable in one of the two date formats.
  - `Notizen` arbitrary free text.
  - All fields trimmed.
  - No semantic checks (no future-birthdate, no joined-after-born, no min-age).
- **No `--dry-run`** in v1: the abort report already previews a broken file, and
  idempotency makes a real re-run safe. Trivially added later if missed.

## Report (stdout, German)

- Success: `N Athleten importiert, M übersprungen (bereits vorhanden):` followed
  by the list of skipped names.
- Abort: `Import abgebrochen, K Fehler:` followed by the per-line error list;
  nothing committed; exit code ≠ 0.
- Messages are **German** — the CLI is otherwise English, but the headers, data
  and audience here are German, and the user is German-speaking.

## Domain model

- **No new glossary term.** Import introduces no new vocabulary; name-based
  athlete identity is already established by `seed-demo`.
- **No ADR.** Reversible, unsurprising (mirrors `seed-demo`), no locked-in
  trade-off.

## Acceptance

- `organizer import-athletes club.csv` inserts every new athlete and prints
  `N importiert, M übersprungen` with the skipped names listed.
- Re-running the identical file imports 0 and skips all — no duplicates (idempotent).
- A `;`-delimited, BOM-prefixed export from German Excel with `dd.mm.yyyy` dates
  imports correctly; dates are stored as ISO.
- Column order may vary and extra unknown columns are ignored; omitting an
  optional column leaves that field empty.
- A file with any bad row (unparseable date, blank required name, in-file
  duplicate name) imports **nothing**, lists every offending line, and exits
  non-zero.
- Missing a required column (`Vorname`/`Nachname`) aborts with a clear message
  before importing anything.
- The testable core is exercised via `importAthletes(db, reader)` without the CLI
  wrapper, covering: happy path, idempotent re-run, both delimiters, BOM, both
  date formats, each validation-error class, and the transaction rollback on error.
