# 01 — Sessions are a module of their own

Status: ready-for-agent
Blocked by: None — can start immediately
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 5 of 10 in the architecture review (2026-08-18)

**What to build:** a new package `internal/session` that owns everything about a
Trainer's Session, and is the only package that knows scs. `internal/web` and
`cmd/organizer` become its callers.

The seam sits in `internal/web` today for one reason: revoking a Trainer's sessions
needs the trainer id, which lives inside each session's encoded values under the
unexported key `sessionKeyTrainerID` (`auth.go:19` → `:85`). The cost is paid by the
Operator's side. To revoke, `cmd/organizer` must build a *second* session manager over
the same database, know to disable its cleanup interval, know that an empty
`exceptToken` means "spare nothing", and supply a background context
(`accounts.go:97-102`) — four things that are all implementation detail, to say one
thing: end this Trainer's sessions. `NewStoredSessions` is a five-line constructor with
one production caller.

The interface is also missing the question its own callers ask most. `offboarding_test.go:127`
counts rows with `SELECT COUNT(*) FROM sessions`, and its comment says why: "the trainer
id lives inside each session's encoded values, so there is nothing to count by trainer".
That is the *interface is the test surface* rule failing — an assertion about revocation
written against the schema instead of the behaviour.

## Decisions

Settled in `/grill-with-docs`, 2026-08-18. Four rounds, every recommendation confirmed.

- **The Trainer identity moves too, not just revocation.** The key is the only reason
  the seam is in `web`; leaving it there leaves revocation there. So `internal/session`
  owns the encoding, and `internal/web` asks it "which Trainer is this request".
- **The cookie policy and `sessionKeyNotice` stay in `internal/web`.** A notice is
  display, not access. It happens to share the store; that is not a reason to share the
  module.
- **scs stays the adapter.** ADR-0002 chose revocable server-side sessions, not a
  session implementation of our own. The walk over the whole session store is therefore
  a property of the interface and is documented as one — performance is part of an
  interface, not of an implementation.
- **A type that hides scs, not free functions taking a manager.** Moving the functions
  as they are would keep every caller beholden to obtaining a manager, which is the
  complaint. After the change, `internal/session` is the only importer of scs; today it
  is `web/auth.go`, `web/server.go` and, through them, `cmd/organizer`.
- **A generic flash pair, prefixed.** `internal/web` needs `Put`/`Pop` for its own keys.
  The pair takes a caller's key and prefixes it, so caller keys live in a space of their
  own and cannot collide with the identity key however `web` names them. Today that
  collision is impossible because both keys sit in one file; the flash pair is what makes
  it possible, so the prefix is what closes it again. Pending flash values in a
  development database are invalidated — they are ephemeral by design.
- **Two constructors, not one with a policy the CLI must null out.** A server (cookie
  policy, background cleanup) and a one-shot command (a database, nothing else) are two
  different things. The Operator should not have to state a null policy — that is exactly
  what `NewStoredSessions` is today.
- **The policy is one named value, and `internal/session` does not import `internal/config`.**
  `main` builds it. Three positional durations-and-a-bool is a leak every test has to
  learn (`auth_test.go:41`); importing config would make it worse by tying the module to
  where configuration comes from.
- **`exceptToken` disappears entirely.** Two verbs — revoke all of a Trainer's sessions,
  revoke all but the one this request is on — and the second needs no token argument,
  because "this one" *is* the request context it already has. The Operator path has no
  request context and calls the other verb.
- **The verbs are named for the domain.** `CONTEXT.md` now says a Session ends by signing
  out, by running out of time, or by being **revoked**. So: revoke all · revoke the others ·
  count the ones a Trainer holds.
- **The stored key stays `"trainerID"`.** It is a data format, not an identifier;
  renaming it costs sign-ins and buys nothing.
- **Redirect the seven reads, do not tidy them.** The doubled `TrainerByID` in
  `resolveLocale` and the `bool`-returning `trainerMayUseTheApp` are candidate 8's whole
  content. Taking them here would overlap two tickets and make neither diff reviewable
  on its own.
- **One ticket, two commits.** First the package plus the identity move; then revocation,
  counting and the CLI. Not two tickets: between them would sit a state where two
  packages know scs, which is worse than either end.
- **No ADR.** ADR-0002 already made the decision that matters. Where the code for it
  lives is a consequence, and this repo carries that kind of reasoning in package
  docstrings (`store.Open`, `storetest`).
- **`CONTEXT.md` gained the term *Session*** during the grill. *Operator* was already
  there — the 2026-08-12 note that it was missing is out of date.

## Acceptance

- [ ] `internal/session` exists and is the only package importing
      `github.com/alexedwards/scs/...`. A test or an analyser check makes that checkable,
      not just true.
- [ ] Two constructors: one for the server taking the database and a named policy value,
      one for a one-shot command taking only the database. `internal/session` does not
      import `internal/config`.
- [ ] The interface carries: the session middleware, read and set the request's Trainer,
      renew, destroy, the prefixed flash pair, revoke all of a Trainer's sessions, revoke
      the others, and count the ones a Trainer holds.
- [ ] The revocation verbs' docstring states that they walk the whole session store, and
      why SQL cannot narrow it.
- [ ] `NewSessionManager`, `NewStoredSessions`, `RevokeSessions` and `exceptToken` are gone
      from `internal/web`.
- [ ] `sessionKeyNotice` still lives in `internal/web`, and its stored key is prefixed by
      the flash pair rather than written raw.
- [ ] `cmd/organizer`'s revoke path names a database and a Trainer, and nothing else. Its
      output says how many Sessions were revoked.
- [ ] `internal/session` has its own tests for what its interface newly claims: revoke all,
      revoke the others, count per Trainer, and another Trainer's Sessions left untouched.
- [ ] `offboarding_test.go` loses `storedSessions` and its `SELECT COUNT(*)`; what it
      asserts is behaviour ("the phone lands on the login page") and, where it wants a
      count, the module's own question. No test asserts the same mechanics at two levels —
      tests are replaced, not layered.
- [ ] The seven reads of `sessionKeyTrainerID` go through the module. The doubled
      `TrainerByID` in `resolveLocale` and the shape of `trainerMayUseTheApp` are
      untouched, left to candidate 8.
- [ ] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.
