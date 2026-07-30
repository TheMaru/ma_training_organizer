# 01 — Trim the code comments back to the non-obvious why

Status: needs-triage

The Go code carries a lot of comment, and a growing share of it explains *what*
the code does rather than *why* it is the way it is. Cut it back, and settle what
the standard actually is so the next feature does not re-inflate it.

## The evidence

Comment lines as a share of each file, today:

| File | Comment / total |
| --- | --- |
| `internal/web/rosterview.go` | 56 / 141 — **39 %** |
| `internal/web/belt.go` | 64 / 184 — 34 % |
| `internal/store/roster.go` | 76 / 255 — 29 % |
| `internal/web/roster.go` | 52 / 193 — 26 % |
| `internal/store/athlete.go` | 44 / 206 — 21 % |
| `internal/store/seed.go` | 30 / 148 — 20 % |

So it is a repo-wide pattern, not one bad file. The most recent commit made it
measurably worse: of the 262 production lines `10552df` added across
`store/roster.go`, `web/roster.go`, `web/rosterview.go` and `store/seed.go`,
**104 are comment — 39 %**, against the 20–34 % of the files it was written into.

## The tension to settle first

There are two standards in play and they point in opposite directions:

- The **global instruction** (`~/.claude/CLAUDE.md`): "default to none. Most code
  needs no comment at all. Write one only for a non-obvious *why* that the code
  itself cannot state — never to restate what the code does. Prefer deleting a
  comment over shortening it."
- The **repo as it stands**: every exported *and* unexported function carries a
  doc comment, usually several sentences, frequently citing an ADR or the spec.
  A new file that followed the global instruction literally would read as foreign
  in this codebase.

That conflict is the actual open question, and it should be answered before
anyone starts deleting. Three candidate positions:

1. **The global instruction wins**; the existing density is legacy to be worked
   off. Cheapest to state, most churn, and it throws away ADR cross-references
   that have genuinely helped — several comments are the only place a piece of
   reasoning is reachable from the code.
2. **Godoc-on-declarations stays, prose-in-bodies goes.** One-sentence doc
   comment per declaration saying what it is for; the multi-paragraph rationale
   moves to the ADR it already cites. Comments inside function bodies survive only
   where they name a non-obvious constraint.
3. **The repo convention wins as-is**, and this ticket becomes about the excess
   only: comments that restate their own body, and cases where the same reasoning
   is written out in two or three places.

## What is already known to be trimmable

Found while reviewing `10552df`, whichever position wins:

- `FilterRoster` (`internal/store/roster.go`) — the doc's first paragraph
  enumerated the three branches the body shows. Already shortened in that commit;
  the same shape exists elsewhere.
- The ADR-0007 reasoning about the options invariant is written out three times:
  in `RosterFilterOptions`, in `handleAthletesList`, and in the template comment
  in `athletes.html`. One of the three is the right number.
- `internal/web/rosterview.go` — the highest-density file in the repo, and mostly
  narrative: the `rosterView` struct doc, `rosterViewFrom` and `query()` each
  carry several paragraphs, with overlap between them.
- Template comments in `athletes.html` re-explain CSS decisions that `app.css`
  states at the rule itself.

## Not in scope

- ADRs, `CONTEXT.md`, specs and issue files. Prose is wanted there; this is about
  comments in code only.
- Commit messages.
- Test names and test doc comments — worth a look, but they document intent for a
  reader deciding whether a failure matters, which is a different trade. Split
  into a second ticket if it turns out to be worth doing.

## Acceptance

- The standard is written down where an agent will actually read it: a section in
  `CLAUDE.md`, or `docs/agents/`, saying what earns a comment in this repo.
- The trim is applied to at least `internal/web/rosterview.go`,
  `internal/store/roster.go` and `internal/web/roster.go`, the three densest
  files touched most recently.
- No reasoning is *lost*: anything deleted that was the only record of a decision
  moves into the ADR it belongs to, rather than disappearing.
- `go test ./...` still passes, which for a comment change means the diff really
  was comment-only — worth confirming with `git diff --stat` against the
  behaviour being untouched.

## Comments

2026-07-30 — Raised by the user straight after `10552df` shipped ("sehr viele und
große Kommentare"), and the numbers above back it up: that commit's production
diff is 39 % comment against a 20–34 % baseline. Filed as `needs-triage` rather
than `ready-for-agent` on purpose — the global "default to none" and the repo's
own thoroughly-commented style genuinely conflict, and an agent turned loose on
this without that decision would either strip out load-bearing ADR pointers or
polish the wording and change nothing. `/grill-me` or `/to-spec` on the three
candidate positions above is the natural next step.
