# 05 — Cut the Promotion type back to the domain

**What to build:** `CONTEXT.md` defines a Promotion as "the event of an athlete
reaching a rank on a date" — three things. The type carries nine fields, five of
which are silently ignored when written and documented only in a comment. Cut the
write type back to the glossary definition, and move the denormalised display
fields onto a separate row type that embeds it.

This is the pattern the codebase already uses: the Roster's row type embeds
Athlete and adds what a view needs to render it. A promotion row is the Promotion
plus the Rank name, the GradingSystem name and slug, and the Rank's descriptive
group and degree. The agreed field shapes are in the spec.

The current-rank derivation stays as it is, retyped to the row type. Collapsing it
with the history query into a single call was proposed in the architecture review
and is wrong: the only caller needs the full graduation history *and* the current
Rank, and already holds the slice, so a combined call would issue a second query
for data in hand. There is also no protocol to remove — the function is
order-independent by construction and documented as such. Depth is for invariants
that need an enforceable home, not for every pair of calls.

Nothing a Trainer can see changes. This is a type change; its proof is that the
existing suites compile and stay green. Writing tests for a rename would be
testing the compiler.

Distinct identifier types — so that an Athlete id and a Rank id cannot be
swapped — are **out of scope** and scheduled separately. Named struct fields
already protect the call site this ticket creates; distinct types protect values
carried through a scope, which is a different problem living mostly in the demo
seeder, and applying them is an all-or-nothing policy across the whole store.

Spec: [[spec]].

**Blocked by:** [[02-open-a-database-in-one-call]] (shared fixture) and
[[04-refuse-a-malformed-date-on-write]] — not a hard dependency, but both edit the
promotion create path and the same two test files, so they are sequenced rather
than run in parallel.

**Status:** done

- [x] The Promotion write type carries only its id, the Athlete, the Rank and the date
- [x] Reads return a row type that embeds the Promotion and adds the Rank name, the GradingSystem name and slug, and the Rank's group and degree
- [x] The current-rank derivation operates on the row type and its behaviour is otherwise untouched, including the tie-break on a shared date
- [x] The athlete detail page renders the current Rank, the graduation history and the belt graphics exactly as before
- [x] No new tests are added for the type change; the existing suites compile and stay green
- [x] The test pinning the SQL recency rule against the Go derivation still ties the two encodings together
- [x] `go vet ./...` is clean
- [x] Coverage recorded before and after as absolute statement counts

## Comments

Shipped 2026-08-07.

**How it landed.** `Promotion` is now `{ID, AthleteID, RankID, PromotedOn}` and the
five display fields moved to `PromotionRow`, which embeds it — the shape the spec
gives at line 169, field for field. `ListPromotions` returns `[]PromotionRow` and
`CurrentRank` takes and returns one; both bodies are otherwise unchanged, so the
tie-break on a shared date is the same code it was.

**Nothing outside `store` needed editing.** Go promotes an embedded struct's
fields, and `html/template` resolves them the same way, so `{{.Current.PromotedOn}}`
and `{{range .Promotions}}` in `athlete_detail.html` keep working untouched, as do
`web/promotions.go` and the demo seeder. The only test edits are three composite
literals in `promotion_test.go` that construct rows by hand; the HTTP suites are
byte-identical and green, which is the evidence the ticket asked for.

**Coverage:** 820 of 1108 statements before, 820 of 1108 after. A pure retyping
adds and removes no statements.

**Left undone, deliberately.** The review flagged that `PromotionRow` and
`RosterRow` now carry the same five display fields, and that the consumers already
read them as two sub-tuples — `rankLabel(locale, Group, Degree, RankName)` and
`systemLabel(locale, SystemSlug, SystemName)`. A `RankDisplay`/`SystemDisplay` pair
would be born here. It is a real data clump and it is not this ticket, which is the
type split only; it belongs with the deferred "rank display spread across six
files" work the spec lists under Out of Scope.
