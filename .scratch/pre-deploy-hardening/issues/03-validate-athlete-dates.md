# 03 — Validate the athlete form's dates like every other date path does

Status: done

`athleteFromForm` (`internal/web/athletes.go`) trims `birthDate` and `joinedOn`
and hands them straight to `store.CreateAthlete`/`UpdateAthlete`, which bind them
into `DATE` columns. Nothing parses them. It is the only date path in the
codebase without that guard: `handleAthletePromote` parses with `time.Parse` and
its comment names this exact hazard ("the DATE column would otherwise take the
garbage string and every later read would fail its NullTime scan (500).
`<input type="date">` guards the UI; this guards a hand-crafted POST"), and the
CSV importer does the same through `parseImportDate`.

The consequence of one bad row is disproportionate: `AthleteByID`, `ListRoster`
and `ListAthletes` all fail their `sql.NullTime` scan, so `GET /athletes`,
`GET /athletes/{id}` and `GET /athletes/{id}/edit` return 500 **for every
trainer**, not just the one who typed it.

## Not a security finding

The pre-deployment security review (2026-08-03) dropped it: the trigger requires
an authenticated trainer bypassing their own `<input type="date">`, all trainers
are trusted equals, and any of them can already hard-delete any athlete — strictly
more destructive and fully authorised. It is a correctness bug and an
inconsistency against the codebase's own established pattern.

Also worth recording, because the first analysis got it wrong: recovery does
**not** need hand-editing the SQLite file. `handleAthleteUpdate` never reads the
row before writing, so a `POST /athletes/{id}` with valid dates repairs it with
no data loss.

## Fix

- Parse both non-empty values in `athleteFromForm` with
  `time.Parse("2006-01-02", …)`, returning a field-level message that re-renders
  the form with a 400, exactly as the missing-name case already does. The message
  needs a catalog key in both `de.json` and `en.json` (the parity test enforces
  it).
- Consider additionally making the read side resilient — scanning the date
  columns as `sql.NullString` and parsing in Go — so that one bad row can never
  take the whole roster down again, whatever writes it. That is the deeper fix
  and touches `store`, so it may be worth splitting.

## Acceptance

- A `POST /athletes` with `birthDate=x` re-renders the form with a 400 and the
  athlete is not stored.
- The roster still renders after the attempt.
- Same for `POST /athletes/{id}` (update), which shares `athleteFromForm`.

## Comments

2026-08-03: Filed from the pre-deployment security review, which verified the
break end to end against the real schema (`modernc.org/sqlite` returns the raw
string with `ok=false` for an unparseable `DATE`, so the `NullTime` scan errors).

2026-08-07: **Superseded — shipped as
`.scratch/store-interface-depth/issues/04-refuse-a-malformed-date-on-write.md`,
commit `a0115a4`.** That ticket was written from the same defect and closed this
one's whole Fix section, so nothing is left to build here.

All three acceptance criteria are pinned by tests:
`TestCreateAthleteRefusesAMalformedDate` and `TestUpdateAthleteRefusesAMalformedDate`
in `internal/web/athletes_test.go` — 400, error message, the trainer's input still
in the fields, nothing stored — and the first of those asserts `GET /athletes`
answers 200 afterwards, which is the reproduction.

**The deeper fix landed from the other direction, and the difference matters.**
This ticket suggested making the *read* side resilient (scan as `sql.NullString`,
parse in Go). What shipped guards the *write* side instead: `internal/store/date.go`
holds `ISODate`, an `ErrMalformedDate` sentinel and the checks behind it, and
athlete create, athlete update and promotion create all refuse before anything
reaches SQL. So no new bad row can be written by any path, form or not — but a row
that is *already* malformed would still fail its `NullTime` scan and take the
roster down. That gap is empty in practice: the app has never been deployed and the
only writers are the guarded ones. If a resilient read is still wanted, it is a new
ticket, not this one.

The form-side check this ticket asked for is in `athleteFromForm` via
`isoDateOrBlank`, parsing against `store.ISODate` rather than its own literal — the
CSV importer's private copy of the layout went the same way.
