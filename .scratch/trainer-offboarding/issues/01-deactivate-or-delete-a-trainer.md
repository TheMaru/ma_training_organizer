# 01 — Offboarding a trainer: deactivate, or delete?

Status: ready-for-agent
Spec: `.scratch/trainer-offboarding/spec.md`

There is no way to remove a trainer's access. No `delete-trainer`, no way to
deactivate an account. A trainer who leaves the club keeps working credentials
indefinitely.

Revoking their sessions ([[pre-deploy-hardening]] 02) does not solve this — they
simply log in again. The gap is at the account level.

## Why this is a feature, not a fix

Split out of [[pre-deploy-hardening]] 02 at triage on 2026-08-04, deliberately,
because the obvious minimal version — "add a `disable-trainer` CLI command" —
quietly pre-decides a domain question that deserves its own discussion.

The question it pre-decides: **must a departed trainer remain visible as the
author of anything?** Today the answer is trivially no. ADR-0001 keeps no trainer
on a `Promotion` — the model records *that* an athlete reached a rank on a date,
never who awarded it — so nothing in the database points at a trainer, and
deleting the row would be consequence-free.

But that is exactly the assumption that breaks if the parked notes-history idea
ever lands: authored, timestamped notes per athlete would make a trainer the
author of records that outlive their membership. Deciding "delete is fine"
now, and building it, is the kind of choice that is cheap today and expensive
after that feature exists.

## Decisions (grill-with-docs, 2026-08-12)

The root question is answered in **ADR-0010**: deactivation is the offboarding
act, deletion is the exception, the state is a nullable `deactivated_at`, the
username stays reserved, and a deactivated account is still a `Trainer`.
`CONTEXT.md` gained the `Deactivated` term and its `Trainer` entry is now about
the account rather than the capability to log in. What follows is everything
decided that is not ADR material.

**What happens to the athletes they entered: nothing.** `CONTEXT.md` already
settles it — the roster is "one, total, and shared", and "no athlete belongs to a
trainer". No column in `athletes` points at a trainer. Written down here because
it is the first question anyone asks, not because anything has to be built.

**No self-service half.** Only the operator deactivates or deletes, matching how
accounts are created. A trainer stilling their own account would also be the one
way to lock yourself out single-handed.

**Enforcement sits in three places, not one.** This is the part the ticket as
filed got wrong: `requireAuth` only checks `sessionKeyTrainerID != 0` and never
loads the trainer, so today a live session would survive both deactivation *and*
deletion until it expired — and `localeFor` swallows the resulting
`ErrTrainerNotFound`, so there is not even a 500 to notice. So:

1. **Login** refuses a deactivated account.
2. **The act itself** revokes the trainer's sessions, reusing
   `web.RevokeSessions(ctx, sessions, trainerID, "")` — this ticket is a caller
   of the mechanism [[pre-deploy-hardening]] 02 built, not a duplicate of it.
   Deletion revokes too.
3. **Every request** re-checks the account in `requireAuth`, which loads the
   trainer and drops a session whose account is deactivated or gone. ~~This is
   effectively free: `localeFor` already does that read on every authenticated
   request. Once the middleware decides first, `localeFor` only ever sees a
   trainer that exists, and its silent error path should stop pretending
   otherwise.~~ **Both sentences were wrong** (2026-08-12, while building
   [[02]]): `resolveLocale` is registered outside `requireAuth`, so it still runs
   first and the read is a second query, not a free one. See [[02]]'s comments.

**A deactivated trainer sees the ordinary login failure**,
`login.badCredentials` — no distinct "account deactivated" message. A separate
message would confirm the username exists, and the app deliberately spends
argon2 work on a decoy hash to avoid exactly that (see `decoyHash`). For the same
reason the password must still be **verified before** the state is checked, or the
deactivation becomes measurable in the response time.

**Neither act may leave the club without an active trainer.** Refused outright,
with no `--force`: "create the new trainer first, then deactivate the old one" is
always available, so nothing is lost. The rule counts trainers with
`deactivated_at IS NULL`, not rows — otherwise ten deactivated accounts satisfy
it. The lockout would not in fact be permanent (`create-trainer` runs against the
volume), but it is a trip to the console at the worst possible moment.

**Four new CLI subcommands**, in the existing verb-noun style, each taking one
`<username>` except the listing:

- `deactivate-trainer` — sets `deactivated_at`, revokes sessions.
- `reactivate-trainer` — clears it. A pure state flip: the old password works
  again, and the operator can `reset-password` before or after if they want
  otherwise. One command, one effect.
- `delete-trainer` — removes the row, revokes sessions. Available on any trainer,
  deactivated or not, behind a confirmation prompt.
- `list-trainers` — username and state (with the date) per line, no hashes. Not
  optional garnish: the last-active-trainer rule above is unenforceable-looking
  without it, and the operator has no other way to see account state — the web UI
  has no trainer list at all and never has.

A single `offboard-trainer` with flags was rejected: it hides which of
the two acts a given line performs, which is the whole distinction this ticket
exists to draw.

**`reset-password` refuses a deactivated account**, with `trainer %q is
deactivated; reactivate first`. On the operator surface a clear message costs
nothing — whoever runs the CLI already holds the database. A reset there is
either a mistake or a homecoming, and the homecoming now has its own verb.
`revoke-sessions` stays permitted: harmless and idempotent.

**`create-trainer` must say why a name is taken.** `ErrUsernameTaken` can now
mean a deactivated account, which is invisible everywhere in the app, so the
message has to name that possibility or the operator hunts for a trainer they
cannot find.

## Ordering

**Pre-deployment scope, decided 2026-08-12** by the maintainer during the grill:
this ships before the first deployment, along with the CLI help ticket below.
Supersedes the original note in this section, which read "not a deployment
blocker" on the reasoning that a club which has not deployed yet has no departed
trainers. That reasoning was sound and the maintainer chose otherwise.

The CLI help listing is a separate ticket, [[cli-help]] 01, in neither direction
blocking: whichever lands first, the other adjusts. If this ticket lands second,
its four subcommands go into the help as part of it.

## Acceptance

- A migration adds `deactivated_at TIMESTAMP` (nullable) to `trainers`, with a
  `Down` that drops it — following `00005_trainer_locale.sql` as the pattern for
  a nullable column added to this table.
- `deactivate-trainer`, `reactivate-trainer`, `delete-trainer` and
  `list-trainers` exist, in English (operator surface, [[cli-language]] 01), with
  the same `usage:` handling as their siblings via `singleUsernameArg`.
- Deactivating the only active trainer is refused, and the refusal counts active
  trainers — a test with several deactivated accounts present proves it.
  `delete-trainer` is refused under the same rule.
- A deactivated trainer cannot log in, and the response is indistinguishable
  from a wrong password: same status, same body, and the password is verified
  first.
- **A live session dies:** log in, deactivate from a second connection to the
  same store, and assert the first client's next request lands on `/login`.
  `cmd/organizer/sessions_test.go` already sets up that two-process arrangement.
  The same test for `delete-trainer`.
- `requireAuth` drops a session whose trainer is missing or deactivated; a test
  covers the missing case directly, since that hole exists today.
- `reactivate-trainer` restores login with the unchanged password.
- `reset-password` on a deactivated trainer fails with the message above, and
  `revoke-sessions` on one still succeeds.
- `create-trainer` against a deactivated trainer's username reports that the name
  belongs to a deactivated account.
- `list-trainers` prints no password hashes. Assert on that, not just on the
  happy path.
- No new UI strings: none of this touches the web layer beyond `requireAuth`, so
  `TestCatalogsHaveIdenticalKeys` and friends are untouched — if that stops being
  true, the new keys go into both catalogs (ADR-0008).
- `go test ./...` passes; the static analysers in `docs/agents/analysis.md` run
  clean.

## Comments

2026-08-04 — > *This was generated by AI during triage.*

Raised while triaging [[pre-deploy-hardening]] 02. The maintainer's instruction
was that this gets discussed properly rather than absorbed into a hardening
ticket, which is why it is filed `needs-triage` with the questions laid out
instead of `ready-for-agent` with an answer guessed at.

Related parked idea, and the reason the timing matters: notes as authored,
timestamped history per athlete. It has no ticket by choice, and it is the
feature that would make a trainer's identity load-bearing in the data.

2026-08-12 — > *This was generated by AI during a `/grill-with-docs` session.*

Grilled over four rounds; every question in the original "What needs deciding"
list is answered above, and that section is now "Decisions". The maintainer
confirmed each round's recommendations without amendment, and added one thing the
grill had not asked about: the CLI has no help listing at all, which became
[[cli-help]] 01.

Three claims in the ticket as filed turned out to be wrong or too strong, all
checked against the code rather than reasoned about:

- Sessions were assumed to be the easy half. They are not: nothing in the request
  path loads the trainer, so a *deleted* account's session keeps full access
  until expiry. That is a hole that exists today, independent of this feature.
- "Locks everyone out of the app permanently" — no. `create-trainer` runs against
  the SQLite file on the volume. Still worth refusing, on different grounds.
- The DSGVO angle was missing entirely. The trainer is a data subject too, and
  ADR-0003 covers only athletes — which is the argument that keeps `delete`
  alive rather than settling on deactivation alone.

The ticket is `ready-for-agent` on the strength of the Acceptance list above. It
is a wide ticket for one context window — a migration, four subcommands, a
middleware change — so splitting it through `/to-spec` and `/to-tickets` before
implementing is reasonable; nothing above presumes it stays one file.
