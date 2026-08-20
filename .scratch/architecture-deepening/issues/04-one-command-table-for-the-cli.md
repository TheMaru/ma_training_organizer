# 04 — One command table for the CLI

Status: done
Blocked by: 02 (done) — not logically, but it rewrites `cmd/organizer`. `03` was
listed here too and was closed without ever being written (see the plan).
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 6 of 10 in the architecture review (2026-08-18)

**What to build:** one table of the binary's subcommands, and `run`, the help
listing and the completeness test all read it instead of each holding their own
copy of the list.

A subcommand is stated in three places today. It is a `cmdXxx` function
(`accounts.go:242,262,282,313,328,349,374`, `demo.go:251,265`,
`import.go:328` — ten of them, not the seven the plan's one-line summary says);
it is a `case` in the switch in `run` (`main.go:52-73`); and it is a line under a
group heading in `helpText` (`help.go:9-40`). Nothing ties the three together, so
the eleventh subcommand can be dispatched and never listed.

`help_test.go:88` is the evidence that this was already felt. It parses
`main.go` with `go/ast` to recover the dispatched names, because — in its own
words — "the expectation is read off the dispatcher itself rather than written
down here a second time". That test is a workaround for the missing table: it
recovers by parsing what the code could simply state.

## Decisions

Not grilled. Nothing here touches `CONTEXT.md`, an ADR, or the domain: the
subcommands and their wording stay exactly as they are, and only where the list
lives changes. What follows is either forced by the code or already settled in
`plan.md`.

- **The table is the one source, and `helpText` is rendered from it.** Generating
  the listing is the whole point — a table that the listing is still maintained
  beside would leave two lists and only move one of them. Completeness stops
  being something a test recovers and becomes structural.
- **Every `cmdXxx` already has the same signature**, `func(dbPath string, args
  []string) error`, verified across all ten. So the table's run field is that
  type and no adapter is needed anywhere.
- **A slice, not a map.** The listing's order is deliberate (`help.go:6-8`: "a
  flat list of every subcommand is a wall") and a map has none. Dispatch by a
  linear scan over ten entries needs no defending.
- **The group is a field on the entry.** Accounts · Data · Development, in the
  order `helpText` has them today, with the group headings rendered from the
  entries rather than written out again.
- **`help`, `-h` and `--help` stay out of the table.** They are answered before
  the configuration is read (`main.go:39-42`), which is a property the listing's
  own test pins (`TestHelpSurvivesABrokenEnvironment`), and `isHelpRequest` is
  not a `cmdXxx`. The `Help:` group in the listing stays a written line.
- **The usage strings stay where they are.** Each `cmdXxx` already rejects its own
  argument shape with its own `usage:` message (`singleUsernameArg`,
  `import.go:329-331`, `demo.go:252-254`). Moving argument validation into the
  table is a second change to the same file and belongs to nobody's ticket — the
  table carries the *argument sketch* the listing prints (`<username>`,
  `<file.csv>`, nothing), not the validation.
- **`TestHelpListsEveryDispatchedCommand` and `dispatchedCommands` go.** Once the
  listing is rendered from the table they assert that a thing equals itself. What
  replaces them is a test at the table: every entry renders a listing line under
  its group, and the dispatcher answers every entry's name. `go/ast` leaves the
  package.
- **No ADR.** Where a list of subcommands lives is trivially reversible and
  surprises nobody (`docs/agents/analysis.md` states this repo's ADR bar).

## Acceptance

- [x] One table in `cmd/organizer` holds every subcommand: its name, its group,
      the argument sketch the listing prints, its one-line description, and the
      function that runs it.
- [x] `run` dispatches from the table. No `switch` over subcommand names remains,
      and an unknown command still errors with the name and a pointer to
      `organizer help` (`TestUnknownCommandPointsAtHelp` keeps passing unchanged).
- [x] `helpText` is rendered from the table, groups and order preserved. The
      printed listing is byte-identical to today's for the ten subcommands — the
      `Usage:` preamble and the `Help:` group may stay written out.
- [x] `help`, `-h` and `--help` are still answered before `config.Load`, and
      `TestHelpSurvivesABrokenEnvironment` still passes.
- [x] `dispatchedCommands` and its `go/ast` walk are gone from `help_test.go`, and
      so is the `go/ast`, `go/parser`, `go/token` import block. What replaces them
      tests the table: every entry appears in the listing under its group, and
      every entry's name dispatches.
- [x] Each `cmdXxx` keeps its own argument validation and its own `usage:`
      message; none of them changes behaviour.
- [x] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.
