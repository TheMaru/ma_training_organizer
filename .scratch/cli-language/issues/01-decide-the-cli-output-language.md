# 01 — Decide what language the CLI speaks

Status: needs-triage

The i18n spec states, as settled scope, that internal and developer-facing
output "stays English": `serverError`/500s, wrapped store/auth/config errors, and
**CLI output** — "Verified already clean". That verification was true when it was
written, and it is not true now.

`cmd/organizer/import.go`, added later by [[csv-athlete-import]], reports to the
trainer in German by design: `"%s importiert, %d übersprungen"`, `"… ist kein
gültiges Datum (JJJJ-MM-TT oder TT.MM.JJJJ)"`, `"Datei enthält keine
Kopfzeile"`, `"CSV-Format ungültig: %v"`. `main.go` even gained an
`errAlreadyReported` so that German report is not followed by an English
`error:` line. The `seed-demo` fixtures carry German prose too, though that is
sample content rather than UI.

So there are three positions and nobody has picked one:

1. **The CLI is a trainer-facing surface and stays German.** It is run by the
   same person who uses the web UI, on their own club's data; the import report
   is something they read, not a log line. Then the i18n spec's scope sentence is
   simply wrong and should be corrected — and the CSV column headers
   (`Vorname`/`Nachname`) already commit the file format to German anyway.
2. **The CLI is developer-facing and becomes English**, matching the spec. Means
   translating `import.go`'s report and error strings, and deciding whether the
   `-athletes` CSV headers follow (they should not — that is a file format, and
   changing it breaks every spreadsheet a club has already prepared).
3. **The CLI is bilingual too**, resolving from `LANG`/`LC_ALL` through the same
   `internal/i18n` catalogs. Technically cheap now that the catalogs exist, but it
   puts trainer-facing strings into a package whose scope ADR-0008 deliberately
   drew around the web UI.

## Why it matters at all

Not much, on its own — a German report in a German club is fine. It matters
because the spec now contains a false statement about the codebase, and the next
person to read it will either trust it or spend an hour finding out it is stale.

## Acceptance

Whichever position wins, `.scratch/i18n-ui/spec.md` gets corrected so its scope
section describes the code as it actually is.

## Comments

2026-08-03: Found while implementing [[i18n-ui]] (`bb523cb`); recorded in that
ticket's comments and split out here so it does not get lost with a closed issue.
