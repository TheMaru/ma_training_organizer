# 03 — Deactivate and reactivate a trainer

Status: ready-for-agent
Blocked by: 02
Spec: `.scratch/trainer-offboarding/spec.md`
Parent: `01-deactivate-or-delete-a-trainer.md`

**What to build:** the operator can take a departed trainer's access away, and
give it back. `deactivate-trainer <username>` keeps the account and refuses it at
login from that moment on, ending the trainer's sessions as it goes.
`reactivate-trainer <username>` undoes it, with the same password as before.

The state is a recorded date, not a flag (ADR-0010), and a deactivated account is
still a `Trainer` — it is `Deactivated`, the glossary's term.

Neither this ticket's act nor [[05]]'s may leave the club with no active trainer,
so the rule arrives here with the state it depends on. It counts trainers who can
log in, not rows in the table.

- [ ] A migration adds the nullable deactivation timestamp to the trainers table,
      with a `Down` that drops it. `NULL` means active.
- [ ] `deactivate-trainer <username>` records the timestamp and reports what it
      did, in English (operator surface, [[cli-language]] 01).
- [ ] After deactivation the trainer's login fails **indistinguishably** from a
      wrong password: same status, same body, same catalog key. No "account
      deactivated" message anywhere.
- [ ] The password is verified **before** the state is checked, so a deactivated
      account costs the same argon2 work as any other failed login and the state
      is not measurable in the response time.
- [ ] Deactivation revokes the trainer's sessions, through the existing revocation
      mechanism rather than a second copy of it. A live session on another device
      lands on the login page.
- [ ] A colleague's sessions are untouched.
- [ ] `requireAuth` refuses a session whose trainer is deactivated, extending the
      check [[02]] introduced.
- [ ] `reactivate-trainer <username>` clears the timestamp, and the trainer's
      **unchanged** password logs in again.
- [ ] Reactivation does nothing else — no password reset, no session changes.
- [ ] Deactivating the only active trainer is refused, the account is left
      untouched, and the message names the way through (create the replacement
      first). No override flag.
- [ ] The refusal counts **active** trainers: a test with several deactivated
      trainers present fails if the rule counts rows.
- [ ] An unknown username fails as a not-found error, like the sibling
      subcommands.
- [ ] Deactivating an already-deactivated trainer is not an error.
- [ ] CLI-side tests use the `cmd/organizer` seam against a migrated database, as
      the account tests do; the login and session behaviour is tested in the
      `internal/web` black-box seam. No store-level tests are added.
- [ ] `go test ./...` passes; the analysers in `docs/agents/analysis.md` run clean.
