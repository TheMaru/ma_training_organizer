# Spec — Offboarding a trainer

Status: ready-for-agent

Issue: `.scratch/trainer-offboarding/issues/01-deactivate-or-delete-a-trainer.md`
Decided in: **ADR-0010**, `CONTEXT.md` (`Trainer`, `Deactivated`)
Sibling, not blocking: [[cli-help]] 01 — the CLI help listing, which must gain
the four subcommands below whichever of the two ships second.

## Problem Statement

A trainer who leaves the club keeps working credentials for as long as the club
runs the tool. There is no way to take access away: no way to deactivate an
account, no way to delete one. The only lever an operator has is revoking
sessions, and that buys nothing here — the departed trainer simply logs in again.

Worse, the two people who would notice are the two who cannot act. The remaining
trainers see no list of accounts anywhere in the app, so nobody can even tell who
still has access. And a departed trainer who wants their own account data gone
has nowhere to send that request: the tool has no answer for it.

## Solution

The operator gets four subcommands, and the app learns to enforce what they say.

**Deactivating** a trainer keeps the account and refuses it at login, from a
recorded date onwards. This is the ordinary offboarding act. It is reversible, so
a trainer who comes back — or a mistyped username — costs nothing. **Deleting** a
trainer removes the account entirely; it is the exception, for an erasure request
or to free a username for reuse. Both acts end the trainer's sessions
immediately, and every subsequent request re-checks the account, so a session
cannot outlive the account it belongs to.

**Listing** trainers makes all of this legible: who exists, and which of them are
deactivated since when. Without it the operator is working blind, since the app
itself never shows one trainer to another.

Neither act may leave the club with no active trainer. That is refused outright
rather than warned about, because the alternative — everyone locked out until
someone reaches a console — is not a trade worth offering.

Nothing happens to the athletes. The roster is one, total, and shared, and no
athlete belongs to a trainer, so there is nothing to reassign and nothing to
delete alongside.

## User Stories

1. As an operator, I want to deactivate a trainer who has left the club, so that
   their credentials stop working without me having to delete anything.
2. As an operator, I want a deactivated trainer's sessions to end the moment I
   deactivate them, so that a browser already logged in does not keep the access I
   just took away.
3. As an operator, I want a deactivated trainer's account to stay in the
   database, so that a future record authored by them still has something to point
   at.
4. As an operator, I want to see when a trainer was deactivated, so that I can
   answer "since when did they no longer have access?" without guessing.
5. As an operator, I want to reactivate a trainer who has come back, so that a
   return does not mean creating a second account for the same person.
6. As an operator, I want reactivation to restore the account exactly as it was,
   including the old password, so that I do not have to coordinate a password
   handover for a routine return.
7. As an operator, I want to delete a trainer entirely, so that I can honour an
   erasure request for the one place their personal data lives.
8. As an operator, I want to delete a trainer without deactivating them first, so
   that an erasure request is one act and not a procedure.
9. As an operator, I want deletion to ask me to confirm, so that a mistyped
   username does not silently destroy an account.
10. As an operator, I want to list every trainer with their state, so that I can
    see who has access before I change anything.
11. As an operator, I want the listing to show no password material, so that I
    can run it on a shared screen or paste its output into a note.
12. As an operator, I want to be refused when I would leave the club with no
    active trainer, so that I cannot lock every trainer out of the app by
    offboarding in the wrong order.
13. As an operator, I want that refusal to count active trainers rather than rows,
    so that a club with a long history of departed trainers is still protected.
14. As an operator, I want the refusal to tell me what to do instead, so that I
    know creating the replacement first is the way through.
15. As an operator, I want `reset-password` to refuse a deactivated trainer, so
    that I notice I am working on an account that cannot log in either way.
16. As an operator, I want the refusal to name reactivation, so that if a return
    is what I actually meant, I know the verb for it.
17. As an operator, I want `create-trainer` to tell me when a username belongs to
    a deactivated trainer, so that I am not hunting for an account the app never
    shows me.
18. As an operator, I want `revoke-sessions` to keep working on a deactivated
    trainer, so that I do not have to reason about command order in an incident.
19. As a departed trainer, I want my login to fail like any other failed login, so
    that nothing about the attempt tells an onlooker whether my username still
    exists.
20. As a trainer still at the club, I want a departed colleague's access to end
    without my roster changing, so that offboarding a person never costs the club
    its data.
21. As a trainer still at the club, I want the athletes a departed colleague
    entered to stay exactly as they are, so that the roster remains the club's
    record rather than a per-trainer one.
22. As a remaining trainer, I want my own session untouched when a colleague is
    offboarded, so that an operator's action does not sign me out mid-training.
23. As the club, I want a departed trainer's session to die even if the operator
    deleted the account and forgot everything else, so that no path leaves a live
    session behind an account that no longer exists.

## Implementation Decisions

**Schema.** `trainers` gains a nullable `deactivated_at TIMESTAMP`. `NULL` means
active; a value means deactivated, and is the date the account was deactivated.
A boolean was rejected: the timestamp carries the same signal plus the date, in
one column (ADR-0010). The migration follows the existing pattern for a nullable
column added to this table, and its `Down` drops the column.

**`internal/store` gains four trainer operations and one predicate**, in the
style of the existing `TrainerByUsername` / `UpdateTrainerPassword` pair —
`*sql.DB` in, domain values out, `ErrTrainerNotFound` for a miss:

- deactivate by id (sets the timestamp; a no-op on an already-deactivated trainer
  is not an error)
- reactivate by id (clears it)
- delete by id
- list trainers, returning username, id and the deactivation timestamp, ordered
  by username — never the password hash
- count active trainers, or a list that makes the count derivable

`Trainer` gains the deactivation timestamp as a field. Every existing lookup
selects it, so a caller that has a `Trainer` can answer "is this account
deactivated?" without a second query. That matters for the login path, which
already holds the row.

**`cmd/organizer` gains four subcommands**, dispatched from the same `switch` as
their siblings, each with a testable core taking `(*sql.DB, username)` and a thin
`cmdXxx` wrapper that opens the database through `withDB` and prints:

- `deactivate-trainer <username>`
- `reactivate-trainer <username>`
- `delete-trainer <username>`
- `list-trainers`

Names follow the existing verb-noun convention. A single `offboard-trainer` with
flags was rejected in the grill: it hides which of the two acts a given line
performs. Output is English — the operator-facing half of the CLI, per
[[cli-language]] 01.

`list-trainers` takes no username, so it does not go through the
single-username-argument helper; it rejects extra arguments the way the demo
subcommands do. Its core **returns data and prints nothing**, mirroring how the
athlete import returns a report value that its wrapper renders. That is what makes
the listing assertable without capturing stdout.

`delete-trainer` prompts for confirmation on the terminal before acting, in the
same spirit as the hidden password prompt: the read happens in the wrapper, so
the core stays non-interactive and testable.

**Both acts revoke sessions**, by calling the existing
`web.RevokeSessions(ctx, sessions, trainerID, "")` over `web.NewStoredSessions`
— exactly what the existing `revoke-sessions` core does. This feature is a caller
of the mechanism [[pre-deploy-hardening]] 02 built, not a second copy of it.
Revocation happens after the state change succeeds.

**The last-active-trainer rule** is one predicate, checked by both
`deactivate-trainer` and `delete-trainer` before they act: the act is refused if
it would leave zero trainers with a `NULL` deactivation timestamp. It counts
active trainers, not rows. There is no override flag. The error names the way
through — create the replacement first.

**`reset-password` refuses a deactivated trainer**, with a message naming
reactivation. `create-trainer`'s existing `ErrUsernameTaken` path gains wording
for the case where the name belongs to a deactivated account, which is invisible
everywhere in the app. `revoke-sessions` is unchanged and stays permitted.

**The login handler refuses a deactivated trainer** with the ordinary bad-
credentials response — same status, same body, same catalog key. A distinct
message would confirm the username exists, which the deliberate decoy-hash work
in that handler exists to prevent. **The password is verified before the state is
checked**, so the refusal costs the same argon2 work as any other failed login and
the deactivation does not become measurable in the response time.

**`requireAuth` re-checks the account on every request.** It loads the session's
trainer and, if the account is missing or deactivated, destroys the session and
redirects to the login page. This closes a hole that exists today independently of
this feature: the middleware checks only that a trainer id is present, so a
session whose account was deleted would keep full access until expiry. ~~The read
is effectively free — the locale middleware already loads the same row on every
authenticated request.~~

~~Once the middleware decides first, the locale resolution only ever sees a trainer
that exists. Its silent `ErrTrainerNotFound` fallback stops carrying weight and
should stop pretending to: the ordering between the two middlewares becomes the
thing that guarantees it, and that is what wants recording where the code says it.~~

**Struck 2026-08-12, while building the ticket that shipped this** — both claims
are false, and the reasoning now lives in
`issues/02-a-session-cannot-outlive-its-trainer.md`'s comments. In short:
`resolveLocale` is registered outside `requireAuth` (`Handler`), so it runs
*first*, the fallback stays reachable and stays load-bearing for the login page,
and the re-check is a second `TrainerByID` query rather than a free one. The
middleware still re-checks the account on every request, which is the part that
matters; only the two claims about what that costs and what it makes dead are
withdrawn.

**No web UI changes.** The app has no trainer list and shows no trainer to
another, so there is no surface to add a state to and no new catalog keys. If that
turns out false during the build, keys go into both catalogs per ADR-0008.

## Testing Decisions

A good test here asserts what the operator or the trainer observes: a login that
fails, a request that lands on the login page, a refusal with a reason, a listing
that contains what it should and omits what it must. It does not assert on the
shape of a SQL statement, on which function called which, or on the presence of a
column. Two seams, both already in the repo, and deliberately no third.

> **2026-08-19.** Seam A moved: the acts now live in `internal/trainer` and their
> tests with them (`.scratch/architecture-deepening/issues/02`). `cmd/organizer`
> keeps the wiring — the prompts, the confirmation, the listing's rendering — and
> its tests cover that only. Still two seams, and still deliberately no third:
> `internal/web` sets its state up by calling the acts, which is what the move was
> for. Everything below reads the same with `internal/trainer` in place of
> `cmd/organizer`, except that the login assertions listed under seam A belong to
> seam B now, where a login can be observed.

**Seam A — the `cmd/organizer` package, against a real migrated database.** Prior
art: the account tests, which build a database with the `storetest` helper and
call `createTrainer` directly, and the CLI session test, which drives revocation
through a second session manager over the same store. What lives here:

- deactivating a trainer, then asserting the login that used to work no longer
  does
- reactivating, then asserting the same password works again
- deleting, then asserting the account is gone
- the last-active-trainer refusal, for both acts, **with several deactivated
  trainers present**, so that a rule counting rows instead of active trainers
  fails the test
- the refusal leaving the account untouched — refused, not half-applied
- both acts revoking the trainer's sessions, and leaving another trainer's
  sessions alone
- `reset-password` refused on a deactivated trainer; `revoke-sessions` still
  permitted
- `create-trainer` reporting the deactivated-account case
- the listing: both states present, the deactivation date carried, and **no
  password hash anywhere in the returned data** — asserted directly, not implied
  by the happy path
- an unknown username failing as `ErrTrainerNotFound` for each new subcommand

**Seam B — `internal/web`, black-box through the existing auth test server.** It
already hands back the server, a cookie-carrying client and the database, so a
test can log in, change the account state through the store, and make the next
request. Prior art: the session and login tests, and the two-client dance the
locale test performs. What lives here:

- a deactivated trainer's login fails **indistinguishably** from a wrong password:
  same status and same body as the existing bad-credentials case
- a live session whose trainer is deactivated lands on the login page on its next
  request
- the same for a trainer whose account was deleted — the hole that exists today,
  so this test fails before the change
- a second trainer's session is unaffected by either
- the surviving negative: an active trainer's session keeps working, so the new
  middleware check cannot pass by signing everyone out

**No store-level tests are added.** Both seams above drive the new store
operations end to end, and a third set asserting the same guarantees at a lower
level would be a seam to maintain for no new information.

The migration needs no test of its own: every test database is built by the same
open-and-migrate path the application uses.

## Out of Scope

- **Any self-service half.** A trainer cannot deactivate or delete their own
  account. Accounts are provisioned out-of-band, and removal belongs in the same
  hands.
- **A web UI for any of this**, including a trainer list, a state badge, or an
  admin area. The app has no concept of one trainer seeing another and this spec
  does not introduce one.
- **In-app roles or permissions.** All trainers remain equal (`CONTEXT.md`).
- **Anything about athletes.** No reassignment, no cascade, no "entered by"
  column. The roster is unchanged, and confirming that is the whole of the work
  here.
- **Notes as authored, timestamped history.** The parked idea that makes a
  trainer's identity load-bearing. This spec exists so that idea stays possible;
  it does not begin it.
- **The CLI help listing.** [[cli-help]] 01, its own ticket. This spec's
  subcommands appear there, but the listing itself is not built here.
- **Bulk or scheduled offboarding**, and any notification to the departed trainer.
- **Audit logging of who offboarded whom.** There is one operator and no audit
  surface; the deactivation date is the only record kept.

## Further Notes

**`Operator` entered the glossary while this spec was written.** The word was
already in use across several tickets, ADR-0008's scope and ADR-0010's
consequences, without ever being defined. Nothing about it needed deciding — the
usage was consistent — so it was recorded rather than ticketed: not an account,
a hat, usually worn by one of the trainers, and the reason these acts live on the
command line instead of behind a login.

**Two claims in the originating ticket were wrong**, both checked against the code
during the grill and both already corrected there. Repeated here because they
change what gets built: the request path never loads the trainer, so sessions
outliving a deleted account is a live defect rather than a new risk; and the
last-trainer lockout is not permanent, since trainer creation runs against the
volume — which is why the rule is a refusal on ergonomic grounds rather than a
safety interlock.

**Splitting this into tickets:** the natural seam is the schema-and-store layer
first (migration plus the store operations, which nothing yet calls), then the
enforcement in the web layer (login refusal and the middleware re-check, which is
the part that closes the existing hole and is independently valuable), then the
four subcommands, then the guard rule and the two neighbouring-command refusals.
Each of the middle two stands on its own; the first is a tracer bullet with no
observable behaviour, so it wants folding into whichever comes next rather than
shipping alone.
