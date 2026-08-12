# Deactivating a trainer is the offboarding act; deleting is the exception

## Context

A trainer who leaves the club keeps working credentials indefinitely: there is no
way to remove access at the account level. Revoking sessions (ADR-0002's
revocable server-side sessions) only buys the time until the next login.

The minimal fix — one command that removes the row — would quietly answer a
domain question that deserves asking: **must a departed trainer remain visible as
the author of anything?** Today the answer is trivially no. ADR-0001 deliberately
records no trainer on a `Promotion`, and nothing else in the schema points at
`trainers` either, so dropping the row is consequence-free. But a parked idea —
authored, timestamped notes per athlete — would make a trainer the author of
records that outlive their membership, and by then the cheap choice would be the
expensive one to undo.

Pulling the other way: the trainer is a data subject in their own right.
ADR-0003 draws the erasure rules around athletes only, so a departed trainer
asking for their account to be erased needs somewhere to go.

## Decision

Two acts, with a rank order between them.

**Deactivation is the offboarding act.** The row stays, login is refused. The
state is carried by a nullable `deactivated_at` timestamp rather than a boolean:
the date is the part a future reader would miss, and it is what a notes history
would need to answer "was this written while they were still here?". Consistent
with ADR-0001, which records dated events instead of state fields. Deactivation
is reversible, as a pure state flip — restoring a password is a separate act with
a separate command.

**Deletion is the exception**, for erasure requests and for freeing a username.
It is available on any trainer without first deactivating them: a two-step
requirement protects nothing, since both steps are one command each.

**A deactivated trainer keeps their `username`.** Recycling a name means nobody
can later tell who was who — the exact damage that makes an authored history
worth having in the first place. Whoever genuinely needs the name back deletes
the account and accepts what that costs.

**A deactivated account is still a `Trainer`.** The glossary defined a Trainer by
a capability ("a person who logs in to…"), which an account that cannot log in
fails; the definition is now about the account, and `Deactivated` is its own
glossary term.

## Considered Options

- **Delete only** — rejected. The obvious minimal version, and free today, but it
  is the choice that gets expensive the moment a trainer authors anything.
- **Deactivate only** — rejected. Leaves a departed trainer no way to have their
  own account data erased, which is the one place their personal data lives.
- **A boolean `deactivated` column** — rejected for the timestamp, which carries
  the same signal plus a date, in the same column.
- **Freeing the username on deactivation** (by scrambling it) — rejected, see
  above.

## Consequences

- "Who can log in" stops being answered by a row's existence and becomes a
  predicate (`deactivated_at IS NULL`) that both the login path and the
  per-request path have to consult. Existing sessions carry only a trainer id and
  know nothing of the state, so the account is re-checked per request rather than
  trusted for the session's lifetime.
- `ErrUsernameTaken` can now name an account that nobody can see anywhere in the
  app, so its message has to say so.
- The club can be left with no active trainer, which the tooling refuses rather
  than allows. Not a permanent lockout — `create-trainer` runs against the volume
  — but a trip to the console at the worst moment.
