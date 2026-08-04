# Code Comments

What earns a comment in this repo.

## The rule

**A comment should only contain what the surrounding code does not already clearly
say.**

That is the primary test. It constrains *what* a comment says — not how many
comments a declaration gets, and not how long any one of them runs. A doc comment
on a small unexported helper is fine if it carries reasoning the signature and body
cannot; a comment that restates the line beneath it is not, however short.

## The carve-out: reasoning about distant code

A comment describing code in *another* file — "the caller in `rosterview.go` relies
on this ordering" — passes the primary test, because the surrounding code genuinely
does not say it. It still doesn't belong. Comments like that couple two files, and
when one changes, only the other gets read.

- **One place owns a piece of reasoning.** Everywhere else points at it rather than
  restating it.
- Where the reasoning lives with code, point at the file or identifier.
- Where it lives in an ADR, cite the ADR.

Pointing is allowed and wanted. Restating is what drifts.

## Two clarifications

- **A "why" with no ADR home stays in the comment, at whatever length it needs.**
  Most reasoning does not qualify for an ADR — that bar is hard-to-reverse *and*
  surprising *and* carrying a real trade-off. Pushing everything else into ADRs
  would either inflate the log with non-decisions or lose reasoning that is
  genuinely load-bearing.
- **ADR citations in comments are navigation, not decoration.** They are how a
  reader gets from a function to the reasoning behind it without already knowing
  which ADR to open. Keep them.

## The failure mode to watch for

Not staleness. Comments sitting next to the code they describe have tracked it here:
the belt follow-ups corrected their own prose as the code changed, including the
highest-risk case there is — prose asserting a computed value (`beltMaxStripes`,
corrected from "four" to "five" when the friso widened). What went stale was the
documentation *furthest* from the code: a README claiming the app was not yet built,
and a wrong claim in ADR-0008. **Drift tracks distance from the code, not the
medium.** An agent editing a function reads that function's comment by construction;
nobody opens the README to change `belt.go`.

What does get worse when agents write the comments: **stating inferred intent as
though it were decided.** Not stale — confidently wrong from the day it is written,
and it reads exactly as authoritative as a correct comment. If you are inferring why
something is the way it is, verify it or say you are inferring.

## Why this repo is looser than the global default, deliberately

The default an agent brings in is stricter: no comments unless forced, prefer
deleting over shortening. This repo takes a documented exception because it is
worked deliberately by agents — stated intent and ADR pointers in code are how a
reader who has never opened `docs/adr/` finds the reasoning at all.

Going by Go's exported/unexported line would not substitute: this is an
application, not a library, nothing consumes `internal/web` from outside, and the
densest files there (`rosterview.go`, `belt.go`) export nothing at all — so that
line would delete hardest exactly where the non-obvious reasoning sits.

An exception with a reason, not an oversight.
