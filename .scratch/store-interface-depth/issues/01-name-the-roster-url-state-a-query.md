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

**Status:** ready-for-agent

- [ ] The private URL-state type in `web` is named for a Roster query rather than a Roster view, in its declaration, its default value, its constructor and its methods
- [ ] Its URL-rendering method no longer shares a name with the type
- [ ] Every call site is updated: the athlete handlers, the promotion handlers, the locale return-target helper, the Roster handler, and both affected test files
- [ ] No behaviour change — the full existing suite passes with no assertion edited, only identifiers
- [ ] `go vet ./...` is clean
