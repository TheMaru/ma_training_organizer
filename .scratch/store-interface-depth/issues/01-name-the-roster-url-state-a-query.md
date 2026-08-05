# 01 — Name the roster's URL state a query, not a view

**What to build:** `CONTEXT.md` now defines the Roster as one complete, shared set
of Athletes, with sorting and filtering producing *views* of it. That makes the
private type in `web` that carries the Roster's URL state misnamed: it is not a
view of the Roster, it is what the URL *asks for*. Rename it to a Roster query,
and rename its URL-rendering method so that a type called query does not also
carry a method called query.

Pure prefactoring. Nothing a Trainer can see changes, and no logic moves. It runs
first because it frees the view/query vocabulary that [[03-load-the-roster-through-one-interface]]
needs, and because doing it separately keeps that ticket to one concern.

Spec: [[spec]].

**Blocked by:** None — can start immediately.

**Status:** done

- [x] The private URL-state type in `web` is named for a Roster query rather than a Roster view, in its declaration, its default value, its constructor and its methods
- [x] Its URL-rendering method no longer shares a name with the type
- [x] Every call site is updated: the athlete handlers, the promotion handlers, the locale return-target helper, the Roster handler, and both affected test files
- [x] No behaviour change — the full existing suite passes with no assertion edited, only identifiers
- [x] `go vet ./...` is clean

## Comments

2026-08-05 — Implemented. `rosterView` → `rosterQuery`, `defaultRosterView` →
`defaultRosterQuery`, `rosterViewFrom` → `rosterQueryFrom`, and the URL-rendering
`query()` → `params()`; the files followed the type (`rosterview.go` →
`rosterquery.go`, same for the test file), and the local variable at every handler
is now `query`. The test helper carrying a URL's sort state is `rosterSortQuery`
with a `params()` method, matching the production vocabulary.

The rename reached further than the ticket's call-site list, in two steps that are
worth separating:

- **Dangling references, not optional:** ADR-0006 and `docs/agents/comments.md`
  pointed at `internal/web/rosterview.go`, and ADR-0007 named `rosterViewFrom`,
  `query()` and `rosterview_test.go`. Those are now wrong file names and wrong
  identifiers, so they moved with the code.
- **Vocabulary, a judgement call:** the prose "view state" in ADR-0005, ADR-0007
  and ADR-0008 and in two template comments now reads "query". Left alone, ADR-0008
  would say "the roster's own query, which `rosterQuery` keeps closed (ADR-0005)"
  while ADR-0005 called the same thing view state. Half a vocabulary across the
  live docs is worse than either whole one.

**Not done, deliberately:** `CONTEXT.md` gains no `RosterQuery` entry. The
glossary is the domain's language and a URL carrier is not a domain concept; the
Roster entry already says sorting and filtering produce views. If the pair earns a
glossary term it is when the store's own `RosterQuery`/`RosterView` land in
[[03-load-the-roster-through-one-interface]].
