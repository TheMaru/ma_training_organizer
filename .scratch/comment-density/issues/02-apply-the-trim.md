# 02 — Apply the trim to the densest files

Status: done
Blocked by: comment-density/01 (writes the rule this applies), i18n-domain-data/01 (rewrites comments in `belt.go` and `roster.go`, two of the files trimmed here) — both landed

Apply the standard from [[comment-density]] 01 to the code. Comment changes only
— no behaviour, no renames, no restructuring.

## Why it waits

Two blockers, for different reasons.

**The rule must exist first**, or the trim is one agent's taste. That is 01.

**The domain-data build must land first**, or part of this work is done twice.
[[i18n-domain-data]] 01 splits the colour parse out of `belt.go` and turns
`rosterLine.RankLabel()` into a function in `roster.go` — both files on the list
below, and both changes rewrite the doc comments attached to exactly the
declarations being trimmed. Trimming first means bidding on comments that are
about to be rewritten. The cost of waiting is that the build produces some
comments this pass then reviews again, which is the cheaper half of the trade.

Ordering constraint from the maintainer: **this ships before the first
deployment.**

## The evidence

Comment lines as a share of each file, measured 2026-07-30:

| File | Comment / total |
| --- | --- |
| `internal/web/rosterview.go` | 56 / 141 — **39 %** |
| `internal/web/belt.go` | 64 / 184 — 34 % |
| `internal/store/roster.go` | 76 / 255 — 29 % |
| `internal/web/roster.go` | 52 / 193 — 26 % |
| `internal/store/athlete.go` | 44 / 206 — 21 % |
| `internal/store/seed.go` | 30 / 148 — 20 % |

A repo-wide pattern, not one bad file. `10552df` made it measurably worse: of the
262 production lines it added across four files, **104 were comment — 39 %**.

Re-measure before starting; these numbers predate the i18n work.

## Known trimmable, found while reviewing `10552df`

- **The ADR-0007 options invariant is written out three times** — in
  `RosterFilterOptions`, in `handleAthletesList`, and in a template comment in
  `athletes.html`. This is the carve-out from 01 in its purest form: two of the
  three are comments about distant code. One place owns it, the others point.
- `internal/web/rosterview.go` — the densest file, and mostly narrative. The
  `rosterView` struct doc, `rosterViewFrom` and `query()` each carry several
  paragraphs with overlap between them.
- `FilterRoster` (`internal/store/roster.go`) — its doc enumerated the three
  branches the body shows. Already shortened in `10552df`; the same shape exists
  elsewhere.
- Template comments in `athletes.html` that re-explain CSS decisions `app.css`
  states at the rule itself.

## Scope

The four files the rule most obviously bites: `internal/web/rosterview.go`,
`internal/web/roster.go`, `internal/web/belt.go` and `internal/store/roster.go`.
`belt.go` is on the list although the original ticket omitted it — at 34 % it is
second-densest, and the domain-data build touches it anyway.

Everything else is trimmed as it is next touched, under the rule from 01. This
ticket is not a repo-wide sweep.

## Not in scope

- ADRs, `CONTEXT.md`, specs and issue files. Prose is wanted there; this is about
  comments in code.
- Commit messages.
- Test names and test doc comments — worth a look, but they document intent for a
  reader deciding whether a failure matters, which is a different trade. Split
  into its own ticket if it turns out to be worth doing.

## Acceptance

- The four files above are trimmed under the rule from 01.
- **No reasoning is lost.** A "why" with no ADR home *stays*, per 01 — it is
  shortened only if the surrounding code already says it. Anything genuinely
  deleted that was the sole record of a decision moves into the ADR it belongs
  to. Deleting reasoning outright is a failure of this ticket, not a success.
- ADR citations survive. They are how a reader gets from code to reasoning.
- `go test ./...` passes and `git diff` shows comment-only changes — no
  behaviour, no renames.

## Comments

2026-08-04 — **Done.** [[i18n-domain-data]] 01 landed (`8d4b0e7`), which unblocked
the deferred half: `internal/web/belt.go` and `internal/web/roster.go` are trimmed,
and all four files in scope are now under the rule.

Comment share: `belt.go` 34 % → 31 %, `web/roster.go` 26 % → 26 %. Flat again, and
for the same reason as the first half — both files carry a lot of genuine "why", so
what went was restatement rather than length. Three kinds again: an enumeration of
the body (`splitRankGroup`'s parse rule, which `strings.Cut` and
`slices.Contains` already state; `rosterLines` and `writeBeltStripes`, whose docs
said exactly what their four lines said), an invariant stated more than once (the
unknown-colour fallback, now owned by `beltSVG` where the templates meet it; the
degree clamp, owned by `beltMaxStripes`), and a comment about another file
(`rosterHeaders` restating what `rosterView.sortedBy` decides, `handleAthletesList`
restating ADR-0007).

**The ADR-0007 invariant is two thirds resolved and stops there by design.**
`RosterFilterOptions` owns it; `FilterRoster` and now `handleAthletesList` point at
it. The third copy is the `athletes.html` template comment, which was never in this
ticket's four-file scope — it gets trimmed as the template is next touched.

Review found three things, all fixed before the commit. Two are the same failure
mode the first half hit, from the opposite direction: trimming the belt geometry
doc deleted the only statement that a split belt's bar reappears in the tail past
the friso, which left the drawing-order comment below it ("the friso covers its
end") the sole account — and wrong, since the friso covers the bar's middle. The
fact moved down to the rect that enacts it and the wrong clause went. Second,
`representedFilter` was left pointing at `NormalizeRosterSort` for a
view-concern-not-a-data-error trade that lives on `ListRoster`, not there; the
pointer went rather than being redirected, because the local "why" stands on its
own. Third, `resolveBelt`'s doc was redundant three ways after the trim and was
deleted outright. **Check the target before you point at it** — that is now twice.

2026-08-04 — **Half done.** `internal/web/rosterview.go` and
`internal/store/roster.go` are trimmed. The other two files in scope wait, because
the blocker did not move: [[i18n-domain-data]] 01 is still `ready-for-agent`, and it
is the one that rewrites the doc comments on exactly the declarations `belt.go` and
`internal/web/roster.go` would be trimmed at. Maintainer's call to ship the
unaffected half now rather than pay for those comments twice. Status stays
`ready-for-agent`; what remains is the two files plus the two remaining copies of
the ADR-0007 invariant.

Comment share: `rosterview.go` 39 % → 34 %, `store/roster.go` 29 % → 27 %. Small,
and that is the rule working as written — the trim is against restatement and
distant-code coupling, not against length, and both files carry a lot of genuine
"why" with no ADR home. What actually went was of one of three kinds: restatement
of the line below (`dir()`'s entire doc, `rosterTieBreak` re-listing its own
literal), an invariant stated in three places at once (the URL-safety guarantee,
now owned by `rosterViewFrom` alone), or a comment describing another file
(`NormalizeRosterSort` on the web layer's header indicator, `FilterRoster` on what
the roster handler does — which now points at `RosterFilterOptions` instead).

**The ADR-0007 invariant is one third resolved.** `RosterFilterOptions` keeps
ownership and `FilterRoster` points at it. `handleAthletesList` still restates it,
but it sits in the deferred file; the `athletes.html` template comment was never in
this ticket's four-file scope and gets trimmed as it is next touched.

Review found three things worth fixing, all fixed before the commit: the trim had
left `query()` pointing at `rosterViewFrom` for a shape guarantee that the trim had
just deleted from it (the pointer is now the enumeration again — `query()` is where
the escaping decision is made, so it owns it); a shortened sentence in `path()` had
started claiming the function redirects, which it does not; and `RosterRow` had lost
its ADR-0001 citation. The first is the failure mode the rule's carve-out invites if
you point without checking the target — worth remembering for the second half.

2026-08-03 — > *This was generated by AI during triage.*

Split out of [[comment-density]] 01 at triage. The original ticket carried both
the rule and the trim, but the two have opposite ordering constraints against the
domain-data build — the rule must precede it, the trim must follow it. A single
ticket would have to pause in the middle and wait, which is not an
implementable unit of work.

The evidence table and the known-trimmable list moved here from 01; the reasoning
behind the rule stayed there.
