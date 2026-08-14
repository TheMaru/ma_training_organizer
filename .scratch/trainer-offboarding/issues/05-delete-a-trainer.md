# 05 — Delete a trainer

Status: done
Blocked by: 03
Spec: `.scratch/trainer-offboarding/spec.md`
Parent: `01-deactivate-or-delete-a-trainer.md`

**What to build:** `delete-trainer <username>` removes an account entirely, after
asking the operator to confirm. This is the exception, not the ordinary
offboarding act (ADR-0010): it exists for an erasure request — the trainer is a
data subject too, and their account is the one place their personal data lives —
and to free a username for reuse.

It works on any trainer, deactivated or not. Requiring deactivation first would
protect nothing, since both acts are one command each.

- [x] `delete-trainer <username>` removes the account, and the username becomes
      available to `create-trainer` again.
- [x] It asks for confirmation on the terminal before acting, and does nothing if
      the answer is not a clear yes. The read happens in the subcommand wrapper so
      the core stays non-interactive and testable.
- [x] Deletion revokes the trainer's sessions, through the same mechanism
      [[03]] uses. A live session on another device lands on the login page — with
      [[02]] in place this holds even if revocation were skipped, and a test covers
      the account-is-gone case regardless.
- [x] Deleting the only active trainer is refused by the same rule as [[03]], with
      the account left untouched. The rule is one predicate serving both acts, not
      two copies.
- [x] A colleague's sessions and the roster are untouched: no athlete and no
      promotion changes, because no athlete belongs to a trainer (`CONTEXT.md`).
      A test asserts the roster survives.
- [x] An unknown username fails as a not-found error.
- [x] Tests use the `cmd/organizer` seam against a migrated database; the
      session-dies behaviour is asserted in the `internal/web` seam as in [[02]].
- [x] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.

## Comments

**2026-08-14 — built.**

`store.DeleteTrainer` is one more caller of the shared rows-affected tail, which is
why that helper is now `writeOneTrainer` rather than `updateOneTrainer`: a deletion
is not an update, and leaving the old name would have made the file's comment about
the shared tail read as if it covered only the `SET` statements. Three existing call
sites renamed with it — the only change here beyond the ticket.

The refusal is literally [[03]]'s predicate, called with `"delete"` as the act, so
the two acts cannot drift apart. Its already-deactivated shortcut turns out to be
what an erasure request for somebody long departed needs: such a trainer is not the
last active one whatever the count.

Revocation runs after the row is gone. That is not a formality — the trainer id
lives inside each session's encoded values, so the sessions genuinely outlive the
row, and the test counts them before any request is made. Deleting the revocation
call fails it at 2 when it wants 1; the same mutation check was run against the
last-active rule and against a `confirm` that always says yes.

**The confirmation is in the wrapper, reading `os.Stdin` directly like
`readHidden`.** The test swaps that global rather than passing a reader in, which
is what buys the end-to-end assertion "a `no` leaves the account standing" instead
of a weaker one about the prompt function alone. The price is named where the
helper is: those tests may not call `t.Parallel`. The prompt asks before the
username is looked up — an operator answering no has said no to whatever they
typed, and a name that does not exist is reported the same way either way.

No new `internal/web` test. [[02]]'s `TestSessionDiesWithItsTrainer` and
`TestOnlyTheDeletedTrainersSessionDies` already assert the deleted-account case,
and they now reach it through `store.DeleteTrainer` instead of the raw `DELETE`
they used while this ticket was still ahead of them.

`go test ./...`, `go vet`, `staticcheck` and `govulncheck` all clean
(`govulncheck`: no vulnerabilities, 2026-08-14).
