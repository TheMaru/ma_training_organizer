# 06 — The store publishes the date question, not the layout

Status: done
Blocked by: None — Wave B depends on nothing
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 10 of 10 in the architecture review (2026-08-18)

**What to build:** the store answers "is this a date I will accept?" itself, so its
three callers stop re-deriving that answer from the layout constant.

`store.ISODate` is exported with a reason attached: "the callers that parse a date
before handing it over — the web forms, the CSV import — have to agree with the
store about what a date is, and a second spelling of the layout is a second thing
to keep in step" (`date.go:9-13`). The layout is shared, and so every caller ends
up writing the parse itself:

- `internal/web/athletes.go:135` — `isoDateOrBlank`, whose comment cites
  `store.ErrMalformedDate` as the thing it is anticipating.
- `internal/web/promotions.go:58` — the same parse inline, folded into the
  `rankID == 0` check, citing the same error for the same reason.
- `cmd/organizer/import.go:204-206` — `parseImportDate`, which tries
  `store.ISODate` *and* `germanDate` and normalises to the former.

All three are guarding against one thing the store already knows: `checkDate`
(`date.go:31`) is the definition, and `ErrMalformedDate` is what it returns. Each
caller reconstructs that predicate from the layout because the predicate itself
was never published. The layout being exported is what makes the reconstruction
possible, and what makes it look correct while it drifts.

## Decisions

Not grilled. The plan already settled the one question that mattered — what may be
shared and what may not — and the callers' own comments settle the rest.

- **The predicate is shared, the message is not.** `plan.md` states it: "das
  Prädikat teilen ist mit ADR-0008 verträglich, die Meldung teilen nicht". The web
  renders `translate(r, "promotion.required")` in the trainer's language, the
  import writes German prose of its own (`import.go:209`), the store's own error is
  English (ADR-0008 puts the operator's half in English). So the store publishes a
  question, not a sentence a caller could print.
- **A boolean question, not a returned `time.Time`.** None of the three callers
  wants the parsed value: two want a yes/no, and the import wants the *normalised
  string*, which it already builds itself and would build the same way from a bool.
  Returning a `time.Time` would hand every caller a value it has to discard, and
  `internal/store` keeps dates as ISO strings on purpose (`athlete.go:206`).
- **`store.ISODate` stays exported.** Two callers format with it rather than parse
  — `accounts.go:220` prints a deactivation date, `athlete.go:206` renders a scanned
  one — and the roster's string comparison of ISO dates is documented as correct
  date comparison (`promotion.go:93-94`). Unexporting it would break those for a
  problem they do not have. What changes is that parsing it is no longer the way to
  ask a question.
- **The blank case belongs to the store too.** `checkOptionalDate` (`date.go:41`)
  already distinguishes "absent" from "malformed" for the nullable columns, and
  `isoDateOrBlank` is `internal/web` restating exactly that. So the store publishes
  both forms — the required question and the optional one — and `isoDateOrBlank`
  disappears rather than being rewritten around the new call.
- **`parseImportDate` keeps the German layout.** `germanDate` is what a German
  spreadsheet exports (`import.go:29-30`), and accepting it is the import's own
  decision about its own input, not something the store should learn. It keeps its
  loop; what it stops doing is spelling the store's half of that loop itself.
- **No ADR.** ADR-0008 already governs which half of the app speaks which language,
  and nothing here reverses it.

## Acceptance

- [x] `internal/store` exports the date question in both forms — required and
      optional-means-blank — sharing its implementation with `checkDate` and
      `checkOptionalDate` so there is exactly one parse in the package.
- [x] Its docstring says the caller gets a yes/no and formats its own message, and
      names why (ADR-0008: the store's own error is English, the web's is
      translated, the import's is German).
- [x] `isoDateOrBlank` is gone from `internal/web`; its caller asks the store.
- [x] `internal/web/promotions.go` no longer parses the date itself, and still
      answers a bad date with `promotion.required` at `400` rather than a 500.
- [x] `parseImportDate` asks the store for the ISO case and keeps `germanDate` and
      its German message. Its behaviour is unchanged: empty stays empty, both
      layouts are accepted, output is ISO.
- [x] No file outside `internal/store` calls `time.Parse(store.ISODate, …)`.
      `store.ISODate` is still exported and still used for formatting.
- [x] `go test ./...` passes — the existing date tests in `internal/store`,
      `internal/web` and the import suite unchanged — and the analysers in
      `docs/agents/analysis.md` run clean.

## Comments

One decision in the ticket was not followed to the letter. **`parseImportDate`
lost its loop.** The ticket said it keeps it — but once the ISO half is a call to
`store.IsDate` rather than a layout, the loop has one element left, and a range
over a one-element slice says less than two branches do. It was raised before the
edit. Behaviour is unchanged and that was checked rather than assumed: for any
string `time.Parse(store.ISODate, s)` accepts, `Format(store.ISODate)` returns `s`
verbatim, so the new `return s` is byte-identical to the old normalisation. Empty
still returns empty first, ISO is still tried before German, and the German
message and layout are untouched.

Two things the review pass added on top of the ticket.

- **`checkOptionalDate` now asks `IsDateOrBlank`.** Writing the exported form as
  `date == "" || IsDate(date)` left the blank-means-absent rule stated twice in
  one file — the ticket's own defect, one level down. `IsDateOrBlank` owns the
  rule and `checkOptionalDate` calls it; the `nullIfEmpty` half stays on
  `checkOptionalDate`, because that is the part about the column rather than the
  question.
- **The comment at the promotion guard shrank to the half the store cannot say.**
  "The date the store will refuse is refused here first, so the trainer gets a
  message instead of a 500" is now `store.IsDate`'s own docstring. What is left is
  the web-local fact: `<input type="date">` guards the UI, this guards a
  hand-crafted POST. The twin guard in `athleteFromForm` carries no comment —
  a function named for form validation does not need one.

`gofmt -l`, `go vet`, `go test ./...`, `staticcheck` and `govulncheck` all clean
(`govulncheck` 2026-09-01: 0 reachable, 2 in required modules the code does not
call).
