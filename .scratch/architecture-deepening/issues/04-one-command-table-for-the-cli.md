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

## Comments

**2026-08-20 — implemented.** One commit, `8425b01`. `command` and `commands` in
`cmd/organizer/commands.go`; `run` scans the table, `helpListing` renders it,
`go/ast` is gone from `help_test.go`. The printed listing is byte-identical to
`7309b22`'s `helpText`, diffed rather than eyeballed. `go vet`, `go test ./...`,
`staticcheck` clean; `govulncheck` not re-run for this ticket — it answers a
question about the world and no dependency changed (`docs/agents/analysis.md`).

Three things went beyond the ticket, all of them the table paying for itself:
`TestCommandNamesAreUnique` (a duplicate name takes a listing line and is never
dispatched, and nothing else notices),
`TestEveryDescriptionStartsInTheSameColumn` (the hand-aligned preamble and the
rendered entries share `descriptionColumn` and nothing else holds them together)
and `TestRunPassesTheDatabaseAndTheRemainingArguments` with `swapCommands`, which
tests the dispatch against a table none of the real subcommands are in.

**2026-08-21 — Wave A's `/code-review`.** Two findings here, both applied; the
wave's full record is in `plan.md`.

- **`helpListing` emitted a heading whenever the group differed from the
  *previous* entry.** With the table's groups in blocks that is the same thing,
  which is why it shipped — but `commands`'s own comment claims "entries of a
  group are kept together", and an entry inserted out of its block would have
  printed that group's heading twice. It now renders group by group, and
  `TestEachGroupGetsOneHeading` pins it against an interleaved table, since the
  real one cannot tell the two rules apart. The listing for the real table is
  unchanged, checked again. The table's comment now says the blocks are for the
  reader, not for the renderer.
- **`TestSubcommandsNeedExactlyOneUsername` in `accounts_test.go` still listed
  the six username subcommands by hand** — the second listing this ticket set out
  to delete, missed because the ticket's inventory named `run` and `helpText` and
  not this test. It reads them off `commands` by the `<username>` sketch now, and
  fails if the table has none, so a sketch nobody spells that way any more cannot
  leave it passing over nothing.

`go vet`, `go test ./...`, `go test -race ./...`, `staticcheck` and `govulncheck`
all clean after both (`govulncheck` 2026-08-21: 0 reachable, 1 in a required
module the code does not call).

