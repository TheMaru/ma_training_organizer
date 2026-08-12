# 05 — Delete a trainer

Status: ready-for-agent
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

- [ ] `delete-trainer <username>` removes the account, and the username becomes
      available to `create-trainer` again.
- [ ] It asks for confirmation on the terminal before acting, and does nothing if
      the answer is not a clear yes. The read happens in the subcommand wrapper so
      the core stays non-interactive and testable.
- [ ] Deletion revokes the trainer's sessions, through the same mechanism
      [[03]] uses. A live session on another device lands on the login page — with
      [[02]] in place this holds even if revocation were skipped, and a test covers
      the account-is-gone case regardless.
- [ ] Deleting the only active trainer is refused by the same rule as [[03]], with
      the account left untouched. The rule is one predicate serving both acts, not
      two copies.
- [ ] A colleague's sessions and the roster are untouched: no athlete and no
      promotion changes, because no athlete belongs to a trainer (`CONTEXT.md`).
      A test asserts the roster survives.
- [ ] An unknown username fails as a not-found error.
- [ ] Tests use the `cmd/organizer` seam against a migrated database; the
      session-dies behaviour is asserted in the `internal/web` seam as in [[02]].
- [ ] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.
