# 07 — Authentication needs a module

Status: ready-for-agent
Blocked by: None — Wave B depends on nothing
Plan: `.scratch/architecture-deepening/plan.md`
Candidate: 8 of 10 in the architecture review (2026-08-18)

**What to build:** grow `internal/auth` from a pair of password primitives into the
module that owns authentication. Two decisions move into it — turning a username and
a password into a Trainer who may sign in, and turning a Session's trainer id into a
Trainer who may use the app — and with them the third caller of `auth.Verify`, the
current-password check in the account area. `internal/web` keeps the HTTP: the
middlewares, the redirects, the rendering. One new middleware there loads the Trainer
once per request and puts it in the request context, where `resolveLocale`,
`requireAuth` and the account handlers read it instead of asking the store again.

## The evidence

**Two mutations, both leaving `go test ./...` green** (run 2026-08-21, during the
grill):

| Mutation | Result |
| --- | --- |
| `tr.Deactivated()` with an early return placed *before* `auth.Verify` in `handleLogin` | green |
| the decoy verification at `auth.go:108` deleted | green |

So both halves of the login ordering are defended by a comment and by nothing else.
`TestDeactivatedTrainerLoginIsIndistinguishable` compares status and body, which is
the axis on which the two answers already agree; they differ in the argon2 work
behind them, and nothing looks there. This is a security property with no test, which
is what makes the candidate correctness rather than shape.

**The doubled read is a tripled one, and it runs the other way round.**
`resolveLocale` is registered at `server.go:60` and `requireAuth` at `:66`, so
`localeFor` reads the Trainer first and takes `Locale`; `trainerMayUseTheApp` reads it
again and takes `Deactivated`, throwing the rest away; and on `POST /account/password`
`handleChangePassword` reads it a third time and takes `PasswordHash`. Three
`TrainerByID` calls for one row, one field each.

**`internal/auth` was on the plan's "Deliberately left alone" list.** This ticket
reopens that deliberately. The plan carries a dated note saying so, because a list of
things a deletion test said no to should not silently stop being true.

## Decisions

Settled in `/grill-with-docs`, 2026-08-21. Three rounds, every recommendation
confirmed.

- **The module is `internal/auth` grown, not a new package beside it.** `auth.Verify`
  has exactly three callers today, all in `internal/web/auth.go` — the decoy at
  `:108`, the login at `:116`, the current-password check at `:169`. Grow the package
  that already holds `Verify` and all three callers end up inside it, so `Verify`
  becomes `verify` and the claim "checking a password is not something a handler
  does" is held by the compiler. The alternative — rename the primitives to
  `internal/password` and give the new module the `internal/auth` name — keeps
  `Verify` exported, because the new module has to import it, and buys back a test
  where a compile error was available. Rejected on that.
- **No third `archtest` boundary.** `analysis.md` says a third one now costs a
  predicate rather than another copy of the walk, and that reads as an invitation.
  Declined here, because the decision above makes the same claim unmissable by
  construction. That is the merit the candidate 4 grill named and could not use
  there; here it is available, and a test that restates a compiler rule is a test
  that can be deleted without anything noticing. `analysis.md` is untouched.
- **The self-service password change's credential half moves in.** It is the third
  caller of `Verify`, so leaving it out would keep `Verify` exported and cost the
  decision above its whole point. `auth.ChangePassword` verifies the current
  password, applies the shared length policy, hashes and writes. What stays in the
  handler is `next != confirm`: that compares two form fields, which is web's
  business, not a rule about credentials.
  - Ticket 02 kept `store.UpdateTrainerPassword` off the trainer boundary list
    because "the self-service change in the account area is the Trainer's own act
    with its own rules and stays in `internal/web`". That was drawn against
    `internal/trainer`, the Operator's module, and it still holds: the Operator's
    `ResetPassword` and this act stay in different packages. It says nothing about
    `internal/auth`.
  - **One behaviour changes.** With the confirm check moved to the front of the
    handler, a submission that gets *both* the current password and the confirmation
    wrong now reports the confirmation, where today it reports the current password.
    No test holds the old order — `TestChangePasswordWrongCurrentIsRejected` sends a
    matching confirmation and `TestChangePasswordMismatchIsRejected` sends the correct
    current password, so each isolates one fault. The combined case has no test today
    and gains none: there is nothing to prefer about either answer.
- **One read per request, carried in the request context.** A new
  `resolveTrainer` middleware in `internal/web` sits between the session middleware
  and `resolveLocale`. It reads the id from the Session, and for a non-zero id asks
  `auth.TrainerMayUseTheApp`. A Trainer who may use the app goes into the context; a
  refusal leaves it empty; a database that could not answer is a 500. `localeFor`,
  `requireAuth` and `handleChangePassword` all read from there, so an authenticated
  request makes exactly one `TrainerByID` call.
- **The context carries a `store.Trainer`, with the zero value meaning nobody.** No
  marker and no reason code. `requireAuth` still has to tell its two refusals apart —
  no Session at all is a plain redirect, a Session naming nobody usable is a destroy
  and then a redirect — and it can, by asking the Session for its id, which costs no
  query. A marker in the context would be a second answer to a question the Session
  already answers.
- **What the struck `spec.md:181-198` gets back, and what it does not.** The strike
  of 2026-08-12 was right: both claims were false, because `resolveLocale` runs
  first. After this ticket:
  - Claim A, that the re-check is free, becomes true **in substance and not in
    wording**. There is one `TrainerByID` per authenticated request and all three
    consumers read it. But it is no longer the locale middleware that loads it, which
    is what the sentence actually said.
  - Claim B, that `resolveLocale`'s `ErrTrainerNotFound` fallback becomes dead code,
    does **not** become true — it becomes moot. `store.TrainerByID` leaves `localeFor`
    altogether, so there is no `ErrTrainerNotFound` there to be alive or dead. The
    fallback that remains reads "no Trainer in the context, so `Accept-Language`", and
    it stays load-bearing for the login page exactly as the strike says.
- **The ordering and the decoy are held by a table test over a pure function.**
  `Authenticate` does the store read and hands the result to an unexported
  `authenticate(tr store.Trainer, found bool, password string, check func(plain, hash string) (bool, error))`.
  The test drives that function over the three outcomes — unknown username, wrong
  password, deactivated account — with a counting `check`, and asserts one password
  check for each. A timing test is rejected: it would be flaky, and it would measure
  argon2 rather than the intent. A package-level variable swapped by the test is
  rejected as the same seam with worse blast radius.
  - Both mutations from the evidence section must fail against the new test.
    Swapping the order gives zero checks for the deactivated case; deleting the decoy
    gives zero for the unknown username. Re-run both while building and record the
    result in `## Comments`, the way ticket 02 did.
- **The names.** `internal/auth` exports `MinPasswordLength`, `ErrPasswordTooShort`,
  `ValidatePassword` and `Hash` unchanged, plus:

  ```go
  var ErrBadCredentials       // username unknown, password wrong, or account deactivated
  var ErrCurrentPasswordWrong

  func Authenticate(db *sql.DB, username, password string) (store.Trainer, error)
  func TrainerMayUseTheApp(db *sql.DB, id int64) (store.Trainer, bool, error)
  func ChangePassword(db *sql.DB, tr store.Trainer, current, next string) error
  ```

  `Verify` becomes `verify`. `SignIn` was rejected for `Authenticate`: the function
  starts no Session, `internal/web` does that through `session.SetTrainerID`, and a
  name that promised otherwise would be the kind of inferred intent
  `docs/agents/comments.md` warns about. `TrainerMayUseTheApp` keeps the name it has
  in `internal/web` today, so a reader who knew it finds it. Its three return values
  are deliberate: the current docstring argues that an account that is gone and one
  that is deactivated are both a plain "no" rather than an error, and only a database
  that could not answer is one. Folding that into a sentinel would throw the argument
  away. `ChangePassword` takes the Trainer rather than an id, because the caller has
  it from the context and a fourth read would undo half this ticket.
- **`CONTEXT.md` gains a sentence under *Deactivated*.** The glossary says the account
  is "refused at login" but never that the refusal has to be indistinguishable from a
  wrong password. That property lives in a code comment and a test name only, and it
  is domain language, not implementation: whoever reads *Deactivated* should know the
  state is invisible from outside. Appended to the existing term rather than made a
  new one, the same call the candidate 4 grill made for the last-active-trainer rule
  under *Offboarding* — it is a boundary of the state, not a thing in the domain. A
  term *Authentication* was rejected: the project does not speak that word, and
  `docs/agents/domain.md` warns against inventing vocabulary.
- **No new ADR.** Where code lives is reversible, and this repo carries that reasoning
  in package docs — ticket 02 decided the same way. ADR-0010 already owns why a
  deactivated account is refused the way it is; its three consequences were re-read
  during the grill and all three still hold.
- **ADR-0008 gets a dated update, not an in-place fix.** Its consequence "Resolving
  the locale costs one `TrainerByID` per authenticated request" was true when written
  and this ticket makes it false, and the cache it weighs itself against stops being
  a live alternative. That is not the case candidate 4 had, where one imprecise word
  in ADR-0010's consequence was corrected in place because the decision had not
  changed. Here the world changes under a correct sentence, so the sentence stays and
  a dated note says what happened — which is what ADR-0008 has already done twice for
  itself.
- **The struck passage in `spec.md` keeps its strike and gains a second dated note.**
  The strike records that both claims were false on 2026-08-12, which stays true. The
  new note records which of them this ticket makes true, in what sense, and which one
  it merely retires.
- **Two commits.** First the module: `internal/auth` grows, `verify` goes unexported,
  `internal/web` becomes a caller, the pure-function test arrives. Then the single
  read: `resolveTrainer`, the context, and the three readers. The first commit changes
  no query count and the second changes no security decision, so split they are each
  reviewable; together nobody could tell which half was which.

## Acceptance

- [ ] `internal/auth` exports `Authenticate`, `TrainerMayUseTheApp` and
      `ChangePassword` with the signatures above, plus `ErrBadCredentials` and
      `ErrCurrentPasswordWrong`. `MinPasswordLength`, `ErrPasswordTooShort`,
      `ValidatePassword` and `Hash` are unchanged.
- [ ] `Verify` is gone from the package's API. No file outside `internal/auth`
      mentions it, and nothing was added to `docs/agents/analysis.md` to make that
      true.
- [ ] The decoy hash lives in `internal/auth`, unexported, with the reasoning that is
      in `internal/web/auth.go:24-27` today.
- [ ] `internal/auth`'s package doc says what the module owns, that it is the only
      place a password is checked, and why the login checks the password before it
      reads the Deactivated state.
- [ ] An unexported `authenticate` takes the looked-up Trainer, whether it was found,
      the submitted password and the check function, and returns the Trainer to sign
      in or `ErrBadCredentials`.
- [ ] A table test in `internal/auth` drives `authenticate` over unknown username,
      wrong password and deactivated account, and asserts exactly one password check
      in each. It fails when the Deactivated read is moved ahead of the check, and it
      fails when the decoy check is removed — both verified by planting the mutation,
      both recorded in `## Comments`.
- [ ] `internal/web` gained a `resolveTrainer` middleware, registered after the
      session middleware and before `resolveLocale`. It puts a `store.Trainer` in the
      request context, empty when nobody may use the app, and answers a store failure
      with a 500.
- [ ] `internal/web/locale.go` no longer reads the store: `localeFor` takes the
      Trainer from the context, and falls back to `Accept-Language` when there is
      none.
- [ ] `requireAuth` reads the context rather than the store. It redirects a request
      with no Session, and destroys the Session before redirecting when the Session
      names a Trainer who may not use the app. `trainerMayUseTheApp` is gone from
      `internal/web`.
- [ ] `handleChangePassword` checks `next != confirm` first, then calls
      `auth.ChangePassword` with the Trainer from the context. It keeps the
      `session.Renew` and the comment explaining why the other Sessions are left
      alone.
- [ ] An authenticated request makes exactly one `TrainerByID` call, including
      `POST /account/password`. Verified rather than asserted by eye — a counting
      `sql.DB` wrapper in a test, or a temporary counter checked by hand and removed;
      say which in `## Comments`.
- [ ] `internal/web`'s existing tests still pass unchanged, and still assert what the
      trainer observes: the indistinguishable refusal, the session that dies with its
      trainer, the login after reactivation, the three password-change rejections.
- [ ] `CONTEXT.md`'s *Deactivated* entry says the refusal is indistinguishable from a
      wrong password.
- [ ] ADR-0008 carries a dated update, 2026-08-21, correcting the per-request
      `TrainerByID` consequence. Its text is left as written.
- [ ] `.scratch/trainer-offboarding/spec.md` carries a second dated note under the
      struck passage: claim A true in substance and not in wording, claim B moot
      rather than true.
- [ ] `.scratch/architecture-deepening/plan.md`'s "Deliberately left alone" list notes
      that `internal/auth` was reopened by this ticket.
- [ ] Two commits, in the order above; each one leaves `go test ./...` passing.
- [ ] `go test ./...`, `go test -race ./...`, `go vet` and `staticcheck` are clean.
      `govulncheck` is not re-run — this change adds no dependency, and the deploy
      ticket owns the fresh scan (`docs/agents/analysis.md`).
